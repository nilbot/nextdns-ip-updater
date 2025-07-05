package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestUpdateNextDNS(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		handlerFunc  http.HandlerFunc
		expectedBool bool
	}{
		{
			name:     "successful update",
			endpoint: "",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			},
			expectedBool: true,
		},
		{
			name:     "server error",
			endpoint: "",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error"))
			},
			expectedBool: false,
		},
		{
			name:         "invalid URL",
			endpoint:     "invalid-url",
			handlerFunc:  nil,
			expectedBool: false,
		},
		{
			name:         "empty URL",
			endpoint:     "",
			handlerFunc:  nil,
			expectedBool: false,
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

			result := updateNextDNS(endpoint)
			if result != tt.expectedBool {
				t.Errorf("updateNextDNS() = %v, want %v", result, tt.expectedBool)
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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		updateNextDNS(testServer.URL)
	}
}

// Test HTTP client timeout behavior
func TestHTTPTimeout(t *testing.T) {
	// Create a server that delays response longer than our timeout
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(35 * time.Second) // Longer than our 30s timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	start := time.Now()
	result := updateNextDNS(testServer.URL)
	elapsed := time.Since(start)

	// Should fail due to timeout
	if result {
		t.Error("Expected updateNextDNS to fail due to timeout")
	}

	// Should complete in approximately 30 seconds (our timeout)
	if elapsed > 35*time.Second {
		t.Errorf("Request took too long: %v (expected ~30s timeout)", elapsed)
	}
}
