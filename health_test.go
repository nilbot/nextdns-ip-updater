package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewHealthServer(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		interval time.Duration
	}{
		{
			name:     "standard configuration",
			port:     8080,
			interval: 5 * time.Minute,
		},
		{
			name:     "custom port",
			port:     9090,
			interval: 1 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hs := NewHealthServer(tt.port, tt.interval)

			if hs == nil {
				t.Fatal("NewHealthServer returned nil")
			}
			if hs.port != tt.port {
				t.Errorf("port = %v, want %v", hs.port, tt.port)
			}
			if hs.interval != tt.interval {
				t.Errorf("interval = %v, want %v", hs.interval, tt.interval)
			}
			if hs.isReady {
				t.Error("isReady should start as false")
			}
			if !hs.isHealthy {
				t.Error("isHealthy should start as true")
			}
			if hs.updateCount != 0 {
				t.Error("updateCount should start at 0")
			}
			if hs.errorCount != 0 {
				t.Error("errorCount should start at 0")
			}
		})
	}
}

func TestHealthServer_StartAndShutdown(t *testing.T) {
	hs := NewHealthServer(8888, 5*time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- hs.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is responding
	resp, err := http.Get("http://localhost:8888/health")
	if err != nil {
		t.Fatalf("Failed to connect to health server: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health endpoint returned %d, want 200", resp.StatusCode)
	}

	// Shutdown server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	if err := hs.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Verify server stopped
	_, err = http.Get("http://localhost:8888/health")
	if err == nil {
		t.Error("Server still responding after shutdown")
	}
}

func TestHealthServer_HealthEndpoint(t *testing.T) {
	hs := NewHealthServer(8889, 5*time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hs.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	defer hs.Shutdown(context.Background())

	tests := []struct {
		name        string
		setupFn     func()
		wantStatus  int
		wantHealthy bool
	}{
		{
			name: "healthy with recent success",
			setupFn: func() {
				hs.SetHealthy(true)
				hs.RecordSuccess()
			},
			wantStatus:  http.StatusOK,
			wantHealthy: true,
		},
		{
			name: "unhealthy - no recent success",
			setupFn: func() {
				hs.SetHealthy(false)
			},
			wantStatus:  http.StatusServiceUnavailable,
			wantHealthy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn()

			resp, err := http.Get("http://localhost:8889/health")
			if err != nil {
				t.Fatalf("GET /health failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			var healthResp HealthResponse
			if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if healthResp.Healthy != tt.wantHealthy {
				t.Errorf("Healthy = %v, want %v", healthResp.Healthy, tt.wantHealthy)
			}
		})
	}
}

func TestHealthServer_ReadyEndpoint(t *testing.T) {
	hs := NewHealthServer(8890, 5*time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hs.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	defer hs.Shutdown(context.Background())

	// Initially not ready
	resp, err := http.Get("http://localhost:8890/ready")
	if err != nil {
		t.Fatalf("GET /ready failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Initial status = %d, want 503", resp.StatusCode)
	}

	var readyResp ReadyResponse
	if err := json.NewDecoder(resp.Body).Decode(&readyResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if readyResp.Ready {
		t.Error("Should not be ready initially")
	}

	// Mark as ready
	hs.SetReady(true)

	resp2, err := http.Get("http://localhost:8890/ready")
	if err != nil {
		t.Fatalf("GET /ready failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("After SetReady status = %d, want 200", resp2.StatusCode)
	}

	var readyResp2 ReadyResponse
	if err := json.NewDecoder(resp2.Body).Decode(&readyResp2); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !readyResp2.Ready {
		t.Error("Should be ready after SetReady(true)")
	}
}

func TestHealthServer_MetricsEndpoint(t *testing.T) {
	hs := NewHealthServer(8891, 5*time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hs.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	defer hs.Shutdown(context.Background())

	// Record some activity
	hs.RecordSuccess()
	hs.RecordSuccess()
	hs.RecordError(fmt.Errorf("test error"))

	resp, err := http.Get("http://localhost:8891/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	bodyStr := string(body)

	// Check for expected metrics
	expectedMetrics := []string{
		"nextdns_updates_total 2",
		"nextdns_errors_total 1",
		"nextdns_ready 0", // Not ready yet
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(bodyStr, metric) {
			t.Errorf("Metrics missing expected line: %q\nGot:\n%s", metric, bodyStr)
		}
	}

	// Mark ready and check again
	hs.SetReady(true)

	resp2, err := http.Get("http://localhost:8891/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body2), "nextdns_ready 1") {
		t.Error("Metrics should show ready=1 after SetReady(true)")
	}
}

func TestHealthServer_SetReady(t *testing.T) {
	hs := NewHealthServer(8080, 5*time.Minute)

	if hs.isReady {
		t.Error("Should not be ready initially")
	}

	hs.SetReady(true)
	hs.mu.RLock()
	ready := hs.isReady
	hs.mu.RUnlock()

	if !ready {
		t.Error("SetReady(true) failed")
	}

	hs.SetReady(false)
	hs.mu.RLock()
	ready = hs.isReady
	hs.mu.RUnlock()

	if ready {
		t.Error("SetReady(false) failed")
	}
}

func TestHealthServer_SetHealthy(t *testing.T) {
	hs := NewHealthServer(8080, 5*time.Minute)

	if !hs.isHealthy {
		t.Error("Should be healthy initially")
	}

	hs.SetHealthy(false)
	hs.mu.RLock()
	healthy := hs.isHealthy
	hs.mu.RUnlock()

	if healthy {
		t.Error("SetHealthy(false) failed")
	}

	hs.SetHealthy(true)
	hs.mu.RLock()
	healthy = hs.isHealthy
	hs.mu.RUnlock()

	if !healthy {
		t.Error("SetHealthy(true) failed")
	}
}

func TestHealthServer_RecordSuccess(t *testing.T) {
	hs := NewHealthServer(8080, 5*time.Minute)

	if hs.updateCount != 0 {
		t.Error("updateCount should start at 0")
	}

	hs.RecordSuccess()

	hs.mu.RLock()
	count := hs.updateCount
	lastSuccess := hs.lastSuccess
	hs.mu.RUnlock()

	if count != 1 {
		t.Errorf("After RecordSuccess, updateCount = %d, want 1", count)
	}

	if lastSuccess.IsZero() {
		t.Error("lastSuccess should be set")
	}

	// Record multiple successes
	for i := 0; i < 5; i++ {
		hs.RecordSuccess()
	}

	hs.mu.RLock()
	count = hs.updateCount
	hs.mu.RUnlock()

	if count != 6 {
		t.Errorf("After 6 total successes, updateCount = %d, want 6", count)
	}
}

func TestHealthServer_RecordError(t *testing.T) {
	hs := NewHealthServer(8080, 5*time.Minute)

	if hs.errorCount != 0 {
		t.Error("errorCount should start at 0")
	}

	testErr := fmt.Errorf("test error")
	hs.RecordError(testErr)

	hs.mu.RLock()
	count := hs.errorCount
	lastErr := hs.lastError
	hs.mu.RUnlock()

	if count != 1 {
		t.Errorf("After RecordError, errorCount = %d, want 1", count)
	}

	if lastErr == nil {
		t.Fatal("lastError should be set")
	}

	if lastErr.Error() != testErr.Error() {
		t.Errorf("lastError = %q, want %q", lastErr.Error(), testErr.Error())
	}

	// Record multiple errors
	for i := 0; i < 3; i++ {
		hs.RecordError(fmt.Errorf("error %d", i))
	}

	hs.mu.RLock()
	count = hs.errorCount
	lastErr = hs.lastError
	hs.mu.RUnlock()

	if count != 4 {
		t.Errorf("After 4 total errors, errorCount = %d, want 4", count)
	}

	if lastErr.Error() != "error 2" {
		t.Errorf("lastError should be most recent: %q", lastErr.Error())
	}
}

func TestHealthServer_Concurrency(t *testing.T) {
	hs := NewHealthServer(8080, 5*time.Minute)

	// Hammer the health server with concurrent operations
	var wg sync.WaitGroup
	numGoroutines := 10
	operationsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				switch j % 4 {
				case 0:
					hs.RecordSuccess()
				case 1:
					hs.RecordError(fmt.Errorf("error from goroutine %d", id))
				case 2:
					hs.SetReady(j%2 == 0)
				case 3:
					hs.SetHealthy(j%2 == 0)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify counts are consistent
	hs.mu.RLock()
	updateCount := hs.updateCount
	errorCount := hs.errorCount
	hs.mu.RUnlock()

	expectedSuccesses := int64(numGoroutines * (operationsPerGoroutine / 4))
	expectedErrors := int64(numGoroutines * (operationsPerGoroutine / 4))

	if updateCount != expectedSuccesses {
		t.Errorf("updateCount = %d, want %d", updateCount, expectedSuccesses)
	}

	if errorCount != expectedErrors {
		t.Errorf("errorCount = %d, want %d", errorCount, expectedErrors)
	}
}

func TestHealthServer_HealthCheck_StaleData(t *testing.T) {
	hs := NewHealthServer(8892, 1*time.Second) // Very short interval for testing

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hs.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	defer hs.Shutdown(context.Background())

	// Record a success, then wait for it to become stale
	hs.RecordSuccess()
	hs.SetHealthy(true)

	// Check immediately - should be healthy
	resp, err := http.Get("http://localhost:8892/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Fresh data: status = %d, want 200", resp.StatusCode)
	}

	// Wait for data to become stale (2x interval + buffer)
	time.Sleep(3 * time.Second)

	// Check again - should be unhealthy due to stale data
	resp2, err := http.Get("http://localhost:8892/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Stale data: status = %d, want 503", resp2.StatusCode)
	}
}

func TestHealthServer_InvalidRoutes(t *testing.T) {
	hs := NewHealthServer(8893, 5*time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hs.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	defer hs.Shutdown(context.Background())

	tests := []struct {
		name string
		path string
	}{
		{"root", "/"},
		{"invalid path", "/invalid"},
		{"typo in health", "/helth"},
		{"typo in ready", "/redy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get("http://localhost:8893" + tt.path)
			if err != nil {
				t.Fatalf("GET %s failed: %v", tt.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("Path %s returned %d, want 404", tt.path, resp.StatusCode)
			}
		})
	}
}

// BenchmarkHealthServer_RecordSuccess measures the performance of RecordSuccess
func BenchmarkHealthServer_RecordSuccess(b *testing.B) {
	hs := NewHealthServer(8080, 5*time.Minute)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		hs.RecordSuccess()
	}
}

// BenchmarkHealthServer_ConcurrentAccess measures concurrent access performance
func BenchmarkHealthServer_ConcurrentAccess(b *testing.B) {
	hs := NewHealthServer(8080, 5*time.Minute)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hs.RecordSuccess()
			hs.RecordError(fmt.Errorf("test"))
			hs.SetReady(true)
			hs.SetHealthy(true)
		}
	})
}
