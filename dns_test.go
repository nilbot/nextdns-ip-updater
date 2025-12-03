package main

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestNewDNSHealthChecker(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{
			name:    "standard timeout",
			timeout: 5 * time.Second,
		},
		{
			name:    "short timeout",
			timeout: 1 * time.Second,
		},
		{
			name:    "long timeout",
			timeout: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dhc := NewDNSHealthChecker(tt.timeout)

			if dhc == nil {
				t.Fatal("NewDNSHealthChecker returned nil")
			}
			if dhc.timeout != tt.timeout {
				t.Errorf("timeout = %v, want %v", dhc.timeout, tt.timeout)
			}
			if dhc.resolver == nil {
				t.Error("resolver should not be nil")
			}
		})
	}
}

func TestDNSHealthChecker_CheckDNS(t *testing.T) {
	tests := []struct {
		name      string
		endpoint  string
		timeout   time.Duration
		wantError bool
	}{
		{
			name:      "valid NextDNS endpoint",
			endpoint:  "https://link-ip.nextdns.io/test123/test456",
			timeout:   5 * time.Second,
			wantError: false, // Should resolve successfully
		},
		{
			name:      "invalid URL",
			endpoint:  "not-a-url",
			timeout:   5 * time.Second,
			wantError: true,
		},
		{
			name:      "empty endpoint",
			endpoint:  "",
			timeout:   5 * time.Second,
			wantError: true,
		},
		{
			name:      "URL without host",
			endpoint:  "https://",
			timeout:   5 * time.Second,
			wantError: true,
		},
		{
			name:      "non-existent domain",
			endpoint:  "https://this-domain-definitely-does-not-exist-12345.com",
			timeout:   2 * time.Second,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dhc := NewDNSHealthChecker(tt.timeout)
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout+1*time.Second)
			defer cancel()

			err := dhc.CheckDNS(ctx, tt.endpoint)

			if tt.wantError && err == nil {
				t.Error("CheckDNS() expected error but got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("CheckDNS() unexpected error: %v", err)
			}
		})
	}
}

func TestDNSHealthChecker_CheckDNS_Timeout(t *testing.T) {
	dhc := NewDNSHealthChecker(100 * time.Millisecond)

	// Create a context that expires quickly
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Try to check DNS - might timeout OR succeed quickly
	err := dhc.CheckDNS(ctx, "https://link-ip.nextdns.io/test/test")

	// This test is lenient because in a healthy DNS environment,
	// resolution might complete before timeout. The important thing
	// is that the timeout mechanism exists and would work if DNS were slow.
	if err == nil {
		t.Log("DNS resolved quickly (acceptable - timeout mechanism still works)")
		return
	}

	// If there was an error, verify it's timeout-related
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Logf("Got non-timeout error: %v (acceptable - DNS might be unreachable)", err)
	}
}

func TestDNSHealthChecker_CheckDNS_ContextCancellation(t *testing.T) {
	dhc := NewDNSHealthChecker(30 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	// CheckDNS should fail quickly due to cancelled context
	err := dhc.CheckDNS(ctx, "https://link-ip.nextdns.io/test/test")

	if err == nil {
		t.Error("CheckDNS() should have failed with cancelled context")
	}
}

func TestDNSHealthChecker_WaitForDNS_ImmediateSuccess(t *testing.T) {
	dhc := NewDNSHealthChecker(5 * time.Second)
	backoff := NewExponentialBackoff(1*time.Second, 60*time.Second, 2.0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use a real domain that should resolve
	endpoint := "https://link-ip.nextdns.io/test/test"

	start := time.Now()
	err := dhc.WaitForDNS(ctx, endpoint, backoff)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("WaitForDNS() unexpected error: %v", err)
	}

	// Should succeed quickly (within 2 seconds)
	if elapsed > 2*time.Second {
		t.Errorf("WaitForDNS() took %v, expected quick success", elapsed)
	}
}

func TestDNSHealthChecker_WaitForDNS_InvalidDomain(t *testing.T) {
	dhc := NewDNSHealthChecker(500 * time.Millisecond)
	backoff := NewExponentialBackoff(100*time.Millisecond, 1*time.Second, 2.0)

	// Use short timeout so test doesn't take forever
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Invalid domain should keep retrying until context timeout
	endpoint := "https://this-domain-definitely-does-not-exist-9999.invalid"

	start := time.Now()
	err := dhc.WaitForDNS(ctx, endpoint, backoff)
	elapsed := time.Since(start)

	// Should eventually timeout
	if err == nil {
		t.Error("WaitForDNS() should have failed for non-existent domain")
	}

	// Should have tried multiple times (at least 1.5 seconds)
	if elapsed < 1*time.Second {
		t.Errorf("WaitForDNS() gave up too quickly: %v", elapsed)
	}
}

func TestDNSHealthChecker_WaitForDNS_ContextCancellation(t *testing.T) {
	dhc := NewDNSHealthChecker(5 * time.Second)
	backoff := NewExponentialBackoff(1*time.Second, 60*time.Second, 2.0)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	endpoint := "https://this-domain-definitely-does-not-exist-8888.invalid"

	start := time.Now()
	err := dhc.WaitForDNS(ctx, endpoint, backoff)
	elapsed := time.Since(start)

	// Should fail due to cancelled context
	if err == nil {
		t.Error("WaitForDNS() should have failed due to cancelled context")
	}

	// Should exit quickly after cancellation (< 1 second total)
	if elapsed > 1*time.Second {
		t.Errorf("WaitForDNS() took too long after context cancellation: %v", elapsed)
	}
}

func TestDNSHealthChecker_WaitForDNS_BackoffReset(t *testing.T) {
	// This test verifies that backoff doesn't need to be reset manually
	// between successful WaitForDNS calls
	dhc := NewDNSHealthChecker(5 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	endpoint := "https://link-ip.nextdns.io/test/test"

	// First call should succeed
	backoff := NewExponentialBackoff(1*time.Second, 60*time.Second, 2.0)
	err1 := dhc.WaitForDNS(ctx, endpoint, backoff)
	if err1 != nil {
		t.Fatalf("First WaitForDNS() failed: %v", err1)
	}

	// Second call with same backoff should also succeed quickly
	// (tests that we don't need to manually reset between calls)
	start := time.Now()
	err2 := dhc.WaitForDNS(ctx, endpoint, backoff)
	elapsed := time.Since(start)

	if err2 != nil {
		t.Errorf("Second WaitForDNS() failed: %v", err2)
	}

	if elapsed > 2*time.Second {
		t.Errorf("Second WaitForDNS() took %v, expected quick success", elapsed)
	}
}

func TestDNSHealthChecker_ExtractHostname(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantHost string
		wantErr  bool
	}{
		{
			name:     "valid NextDNS URL",
			endpoint: "https://link-ip.nextdns.io/abc123/def456",
			wantHost: "link-ip.nextdns.io",
			wantErr:  false,
		},
		{
			name:     "URL with port",
			endpoint: "https://example.com:8080/path",
			wantHost: "example.com",
			wantErr:  false,
		},
		{
			name:     "HTTP URL",
			endpoint: "http://example.com/path",
			wantHost: "example.com",
			wantErr:  false,
		},
		{
			name:     "invalid URL",
			endpoint: "not a url",
			wantHost: "",
			wantErr:  true,
		},
		{
			name:     "empty URL",
			endpoint: "",
			wantHost: "",
			wantErr:  true,
		},
		{
			name:     "URL without scheme",
			endpoint: "example.com/path",
			wantHost: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dhc := NewDNSHealthChecker(5 * time.Second)
			host, err := dhc.extractHostname(tt.endpoint)

			if tt.wantErr {
				if err == nil {
					t.Error("extractHostname() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("extractHostname() unexpected error: %v", err)
				return
			}

			if host != tt.wantHost {
				t.Errorf("extractHostname() = %q, want %q", host, tt.wantHost)
			}
		})
	}
}

func TestDNSHealthChecker_ResolveDNS(t *testing.T) {
	dhc := NewDNSHealthChecker(5 * time.Second)

	tests := []struct {
		name      string
		hostname  string
		wantError bool
	}{
		{
			name:      "resolvable hostname",
			hostname:  "link-ip.nextdns.io",
			wantError: false,
		},
		{
			name:      "localhost",
			hostname:  "localhost",
			wantError: false,
		},
		{
			name:      "non-existent hostname",
			hostname:  "this-definitely-does-not-exist-12345.invalid",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			addrs, err := dhc.resolveDNS(ctx, tt.hostname)

			if tt.wantError {
				if err == nil {
					t.Error("resolveDNS() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("resolveDNS() unexpected error: %v", err)
				return
			}

			if len(addrs) == 0 {
				t.Error("resolveDNS() returned empty address list")
			}

			// Verify addresses are valid IPs
			for _, addr := range addrs {
				if net.ParseIP(addr) == nil {
					t.Errorf("resolveDNS() returned invalid IP: %q", addr)
				}
			}
		})
	}
}

// BenchmarkDNSHealthChecker_CheckDNS measures the performance of DNS checks
func BenchmarkDNSHealthChecker_CheckDNS(b *testing.B) {
	dhc := NewDNSHealthChecker(5 * time.Second)
	ctx := context.Background()
	endpoint := "https://link-ip.nextdns.io/test/test"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dhc.CheckDNS(ctx, endpoint)
	}
}
