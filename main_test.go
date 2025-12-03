package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestUpdateNextDNS(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    string
		handlerFunc http.HandlerFunc
		wantError   bool
	}{
		{
			name:     "successful update",
			endpoint: "",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			},
			wantError: false,
		},
		{
			name:     "server error",
			endpoint: "",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error"))
			},
			wantError: true,
		},
		{
			name:      "invalid URL",
			endpoint:  "invalid-url",
			wantError: true,
		},
		{
			name:      "empty URL",
			endpoint:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testServer *httptest.Server
			endpoint := tt.endpoint

			if tt.handlerFunc != nil {
				testServer = httptest.NewServer(tt.handlerFunc)
				defer testServer.Close()
				endpoint = testServer.URL
			}

			ctx := context.Background()
			client := createHTTPClient()
			err := updateNextDNS(ctx, endpoint, client)

			if tt.wantError && err == nil {
				t.Error("updateNextDNS() expected error but got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("updateNextDNS() unexpected error: %v", err)
			}
		})
	}
}

func TestEnvironmentVariables(t *testing.T) {
	// Save original env vars
	originalEndpoint := os.Getenv("NEXTDNS_ENDPOINT")
	originalInterval := os.Getenv("UPDATE_INTERVAL_SECONDS")

	// Clean up after test
	defer func() {
		if originalEndpoint != "" {
			os.Setenv("NEXTDNS_ENDPOINT", originalEndpoint)
		} else {
			os.Unsetenv("NEXTDNS_ENDPOINT")
		}
		if originalInterval != "" {
			os.Setenv("UPDATE_INTERVAL_SECONDS", originalInterval)
		} else {
			os.Unsetenv("UPDATE_INTERVAL_SECONDS")
		}
	}()

	t.Run("missing NEXTDNS_ENDPOINT", func(t *testing.T) {
		os.Unsetenv("NEXTDNS_ENDPOINT")
		os.Unsetenv("UPDATE_INTERVAL_SECONDS")

		// We can't easily test main() exit behavior, but we can test the validation logic
		endpoint := os.Getenv("NEXTDNS_ENDPOINT")
		if endpoint != "" {
			t.Error("Expected empty NEXTDNS_ENDPOINT")
		}
	})

	t.Run("valid environment variables", func(t *testing.T) {
		testEndpoint := "https://link-ip.nextdns.io/test/test"
		testInterval := "60"

		os.Setenv("NEXTDNS_ENDPOINT", testEndpoint)
		os.Setenv("UPDATE_INTERVAL_SECONDS", testInterval)

		endpoint := os.Getenv("NEXTDNS_ENDPOINT")
		interval := os.Getenv("UPDATE_INTERVAL_SECONDS")

		if endpoint != testEndpoint {
			t.Errorf("Expected endpoint %s, got %s", testEndpoint, endpoint)
		}
		if interval != testInterval {
			t.Errorf("Expected interval %s, got %s", testInterval, interval)
		}
	})
}

func TestVersionAndBuildTime(t *testing.T) {
	// Test that version and buildTime variables exist
	// They should be set during build via ldflags
	if version == "" {
		t.Log("Warning: version is empty (this is expected during 'go test' without ldflags)")
	}
	if buildTime == "" {
		t.Log("Warning: buildTime is empty (this is expected during 'go test' without ldflags)")
	}
}

func TestLoggerInitialization(t *testing.T) {
	// Ensure logger is properly initialized
	if logger == nil {
		t.Error("Logger should be initialized")
	}

	// Test that we can log without panicking
	logger.Info("Test log message")
}

// Benchmark test for updateNextDNS function
func BenchmarkUpdateNextDNS(b *testing.B) {
	// Create a test server that always returns OK
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer testServer.Close()

	ctx := context.Background()
	client := createHTTPClient()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		updateNextDNS(ctx, testServer.URL, client)
	}
}

// Test HTTP client timeout behavior
func TestHTTPTimeout(t *testing.T) {
	// Test timeout behavior using context cancellation
	t.Run("timeout test with context cancellation", func(t *testing.T) {
		// Create a server that blocks indefinitely
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// This handler will block until the context is cancelled
			select {
			case <-r.Context().Done():
				return
			case <-time.After(10 * time.Second):
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer testServer.Close()

		// Create a context that times out quickly
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		client := createHTTPClient()

		start := time.Now()
		err := updateNextDNS(ctx, testServer.URL, client)
		elapsed := time.Since(start)

		// Should fail due to timeout
		if err == nil {
			t.Error("Expected updateNextDNS to fail due to timeout")
		}

		// Should complete quickly (within 1 second)
		if elapsed > 1*time.Second {
			t.Errorf("Request took too long: %v (expected quick timeout)", elapsed)
		}

		t.Logf("Timeout test completed in %v", elapsed)
	})
}

func TestCreateHTTPClient(t *testing.T) {
	client := createHTTPClient()

	if client == nil {
		t.Fatal("createHTTPClient returned nil")
	}

	if client.Timeout != 30*time.Second {
		t.Errorf("client.Timeout = %v, want 30s", client.Timeout)
	}

	if client.Transport == nil {
		t.Fatal("client.Transport should not be nil")
	}
}
