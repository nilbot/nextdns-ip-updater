package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// HealthServer manages HTTP endpoints for Kubernetes health probes and metrics.
// It provides /health (liveness), /ready (readiness), and /metrics (Prometheus) endpoints.
type HealthServer struct {
	port   int
	server *http.Server

	// State management
	mu          sync.RWMutex
	isReady     bool
	isHealthy   bool
	lastSuccess time.Time
	lastError   error
	startTime   time.Time
	updateCount int64
	errorCount  int64
	interval    time.Duration
}

// HealthResponse is the JSON response for /health endpoint
type HealthResponse struct {
	Healthy     bool      `json:"healthy"`
	LastSuccess time.Time `json:"last_success,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	Uptime      float64   `json:"uptime_seconds"`
	UpdateCount int64     `json:"update_count"`
	ErrorCount  int64     `json:"error_count"`
}

// ReadyResponse is the JSON response for /ready endpoint
type ReadyResponse struct {
	Ready   bool   `json:"ready"`
	Message string `json:"message"`
}

// NewHealthServer creates a new health server that listens on the specified port.
//
// Parameters:
//   - port: The port to listen on (e.g., 8080)
//   - interval: The expected update interval (used to detect stale data)
//
// Example:
//
//	hs := NewHealthServer(8080, 5*time.Minute)
//	go hs.Start(ctx)
func NewHealthServer(port int, interval time.Duration) *HealthServer {
	hs := &HealthServer{
		port:      port,
		isReady:   false,
		isHealthy: true, // Start healthy, becomes unhealthy if updates fail
		startTime: time.Now(),
		interval:  interval,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", hs.handleHealth)
	mux.HandleFunc("/ready", hs.handleReady)
	mux.HandleFunc("/metrics", hs.handleMetrics)

	hs.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return hs
}

// Start begins serving HTTP requests on the configured port.
// This method blocks until the server is shutdown or encounters an error.
// It should be called in a goroutine.
//
// Example:
//
//	go func() {
//	    if err := hs.Start(ctx); err != nil {
//	        log.Fatalf("Health server failed: %v", err)
//	    }
//	}()
func (hs *HealthServer) Start(ctx context.Context) error {
	logger.WithField("port", hs.port).Info("Health server starting")

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := hs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		logger.Info("Health server stopping (context cancelled)")
		return nil
	case err := <-errChan:
		logger.WithError(err).Error("Health server failed")
		return err
	}
}

// Shutdown gracefully stops the health server with a timeout.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	if err := hs.Shutdown(ctx); err != nil {
//	    log.Errorf("Shutdown error: %v", err)
//	}
func (hs *HealthServer) Shutdown(ctx context.Context) error {
	logger.Info("Shutting down health server")
	return hs.server.Shutdown(ctx)
}

// SetReady updates the readiness state.
// Once set to true, the /ready endpoint will return 200 OK.
func (hs *HealthServer) SetReady(ready bool) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.isReady = ready

	logger.WithField("ready", ready).Debug("Readiness state updated")
}

// SetHealthy updates the health state.
// When set to false, the /health endpoint will return 503.
func (hs *HealthServer) SetHealthy(healthy bool) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.isHealthy = healthy

	logger.WithField("healthy", healthy).Debug("Health state updated")
}

// RecordSuccess records a successful update operation.
// This updates metrics and the lastSuccess timestamp.
func (hs *HealthServer) RecordSuccess() {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.updateCount++
	hs.lastSuccess = time.Now()

	logger.WithFields(logrus.Fields{
		"update_count": hs.updateCount,
	}).Debug("Success recorded")
}

// RecordError records a failed update operation.
// This updates metrics and stores the error for diagnostics.
func (hs *HealthServer) RecordError(err error) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.errorCount++
	hs.lastError = err

	logger.WithFields(logrus.Fields{
		"error_count": hs.errorCount,
		"error":       err.Error(),
	}).Debug("Error recorded")
}

// handleHealth implements the /health endpoint (liveness probe).
// Returns 200 OK if healthy and data is fresh, 503 otherwise.
func (hs *HealthServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	// Check if we're healthy
	healthy := hs.isHealthy

	// Check if data is stale (no successful update in 2x the expected interval)
	if !hs.lastSuccess.IsZero() {
		timeSinceLastSuccess := time.Since(hs.lastSuccess)
		if timeSinceLastSuccess > 2*hs.interval {
			healthy = false
		}
	}

	uptime := time.Since(hs.startTime).Seconds()

	response := HealthResponse{
		Healthy:     healthy,
		LastSuccess: hs.lastSuccess,
		Uptime:      uptime,
		UpdateCount: hs.updateCount,
		ErrorCount:  hs.errorCount,
	}

	if hs.lastError != nil {
		response.LastError = hs.lastError.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	if !healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(response)
}

// handleReady implements the /ready endpoint (readiness probe).
// Returns 200 OK only after first successful update, 503 otherwise.
func (hs *HealthServer) handleReady(w http.ResponseWriter, r *http.Request) {
	hs.mu.RLock()
	ready := hs.isReady
	hs.mu.RUnlock()

	response := ReadyResponse{
		Ready: ready,
	}

	if ready {
		response.Message = "Service is ready"
	} else {
		response.Message = "Service is not ready (waiting for first successful update)"
	}

	w.Header().Set("Content-Type", "application/json")
	if ready {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(response)
}

// handleMetrics implements the /metrics endpoint (Prometheus format).
// Returns metrics in plain text format compatible with Prometheus.
func (hs *HealthServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	uptime := time.Since(hs.startTime).Seconds()

	// Prometheus text format
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)

	// Write metrics
	fmt.Fprintf(w, "# HELP nextdns_updates_total Total number of successful NextDNS updates\n")
	fmt.Fprintf(w, "# TYPE nextdns_updates_total counter\n")
	fmt.Fprintf(w, "nextdns_updates_total %d\n", hs.updateCount)

	fmt.Fprintf(w, "# HELP nextdns_errors_total Total number of failed update attempts\n")
	fmt.Fprintf(w, "# TYPE nextdns_errors_total counter\n")
	fmt.Fprintf(w, "nextdns_errors_total %d\n", hs.errorCount)

	fmt.Fprintf(w, "# HELP nextdns_ready Readiness status (1=ready, 0=not ready)\n")
	fmt.Fprintf(w, "# TYPE nextdns_ready gauge\n")
	if hs.isReady {
		fmt.Fprintf(w, "nextdns_ready 1\n")
	} else {
		fmt.Fprintf(w, "nextdns_ready 0\n")
	}

	fmt.Fprintf(w, "# HELP nextdns_healthy Health status (1=healthy, 0=unhealthy)\n")
	fmt.Fprintf(w, "# TYPE nextdns_healthy gauge\n")
	if hs.isHealthy {
		fmt.Fprintf(w, "nextdns_healthy 1\n")
	} else {
		fmt.Fprintf(w, "nextdns_healthy 0\n")
	}

	fmt.Fprintf(w, "# HELP nextdns_uptime_seconds Uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE nextdns_uptime_seconds gauge\n")
	fmt.Fprintf(w, "nextdns_uptime_seconds %.2f\n", uptime)

	if !hs.lastSuccess.IsZero() {
		timeSinceLastSuccess := time.Since(hs.lastSuccess).Seconds()
		fmt.Fprintf(w, "# HELP nextdns_last_success_seconds Time since last successful update\n")
		fmt.Fprintf(w, "# TYPE nextdns_last_success_seconds gauge\n")
		fmt.Fprintf(w, "nextdns_last_success_seconds %.2f\n", timeSinceLastSuccess)
	}
}
