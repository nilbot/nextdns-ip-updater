package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// Build-time variables
var (
	version   = "dev"
	buildTime = "unknown"
)

// Logger holds our structured logger instance
var logger *logrus.Logger

func init() {
	// Configure structured logging similar to Python's structlog
	logger = logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})
	logger.SetOutput(os.Stdout)
}

// createHTTPClient creates an HTTP client with custom timeouts for DNS and connections.
// This ensures we have fine-grained control over timeout behavior.
func createHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second, // Total request timeout (unchanged from original)
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second, // Connection timeout
				KeepAlive: 30 * time.Second, // TCP keepalive
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second, // TLS handshake timeout
			ResponseHeaderTimeout: 20 * time.Second, // Response header timeout
			ExpectContinueTimeout: 1 * time.Second,  // 100-continue timeout
			MaxIdleConns:          2,                // Connection pooling
			MaxIdleConnsPerHost:   2,
			IdleConnTimeout:       90 * time.Second,
		},
	}
}

// updateNextDNS updates NextDNS with the current WAN IP by calling the endpoint directly.
// Returns error if update fails, nil on success.
func updateNextDNS(ctx context.Context, endpoint string, client *http.Client) error {
	// Validate endpoint
	parsedURL, err := url.Parse(endpoint)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		logger.WithFields(logrus.Fields{
			"endpoint": endpoint,
			"error":    "invalid NextDNS endpoint",
		}).Error("Invalid NextDNS endpoint")
		if err != nil {
			return err
		}
		return fmt.Errorf("invalid NextDNS endpoint: missing scheme or host")
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.WithFields(logrus.Fields{
			"status_code": resp.StatusCode,
			"status":      resp.Status,
		}).Error("NextDNS returned non-OK status")
		return fmt.Errorf("NextDNS returned status %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// runUpdateLoop executes the main update loop with retry logic and health tracking.
func runUpdateLoop(ctx context.Context, endpoint string, interval time.Duration, healthServer *HealthServer) {
	client := createHTTPClient()
	hasHadFirstSuccess := false

	for {
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			logger.Info("Update loop stopping (context cancelled)")
			return
		default:
		}

		// Attempt update
		err := updateNextDNS(ctx, endpoint, client)

		if err == nil {
			// Success!
			logger.WithField("endpoint", endpoint).Info("Successfully updated NextDNS")
			healthServer.RecordSuccess()
			healthServer.SetHealthy(true)

			// Mark as ready after first success
			if !hasHadFirstSuccess {
				healthServer.SetReady(true)
				logger.Info("Service marked as ready after first successful update")
				hasHadFirstSuccess = true
			}

			// Sleep for the configured interval before next update
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
				// Continue to next update
			}
		} else {
			// Update failed
			logger.WithFields(logrus.Fields{
				"endpoint": endpoint,
				"error":    err.Error(),
			}).Error("Error updating NextDNS")
			healthServer.RecordError(err)

			// Fast retry with shorter interval (30 seconds)
			logger.Info("Retrying in 30 seconds...")
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
				// Retry
			}
		}

		logger.WithField("success", err == nil).Info("Update cycle completed")
	}
}

func main() {
	// Get NextDNS endpoint from environment variable
	nextdnsEndpoint := os.Getenv("NEXTDNS_ENDPOINT")
	if nextdnsEndpoint == "" {
		logger.Error("NEXTDNS_ENDPOINT environment variable is not set")
		os.Exit(1)
	}

	// Get update interval from environment variable or use default (5 minutes)
	intervalStr := os.Getenv("UPDATE_INTERVAL_SECONDS")
	if intervalStr == "" {
		intervalStr = "300"
	}

	interval, err := strconv.Atoi(intervalStr)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"interval": intervalStr,
			"error":    err.Error(),
		}).Error("Invalid UPDATE_INTERVAL_SECONDS value")
		os.Exit(1)
	}

	// Validate interval
	if interval <= 0 {
		logger.WithField("interval", interval).Fatal("UPDATE_INTERVAL_SECONDS must be positive")
	}
	if interval < 60 {
		logger.WithField("interval", interval).Warn("UPDATE_INTERVAL_SECONDS is very short, consider 60+ seconds")
	}

	// Get health server port (default 8080)
	healthPort := 8080
	if portStr := os.Getenv("HEALTH_SERVER_PORT"); portStr != "" {
		healthPort, err = strconv.Atoi(portStr)
		if err != nil {
			logger.WithError(err).Warn("Invalid HEALTH_SERVER_PORT, using default 8080")
			healthPort = 8080
		}
	}

	logger.WithFields(logrus.Fields{
		"endpoint":         nextdnsEndpoint,
		"interval_seconds": interval,
		"health_port":      healthPort,
		"version":          version,
		"build_time":       buildTime,
	}).Info("Starting NextDNS IP updater")

	// Create root context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// WaitGroup for goroutine tracking
	var wg sync.WaitGroup

	// 1. Start health server
	healthServer := NewHealthServer(healthPort, time.Duration(interval)*time.Second)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := healthServer.Start(ctx); err != nil {
			logger.WithError(err).Error("Health server failed")
		}
	}()

	// Give health server time to start
	time.Sleep(100 * time.Millisecond)

	// 2. Wait for DNS to be ready (blocking)
	dnsChecker := NewDNSHealthChecker(5 * time.Second)
	dnsBackoff := NewExponentialBackoff(1*time.Second, 60*time.Second, 2.0)

	logger.Info("Waiting for DNS to be ready...")
	if err := dnsChecker.WaitForDNS(ctx, nextdnsEndpoint, dnsBackoff); err != nil {
		logger.WithError(err).Fatal("DNS check cancelled")
	}
	logger.Info("DNS is ready")

	// 3. Start update loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		runUpdateLoop(ctx, nextdnsEndpoint, time.Duration(interval)*time.Second, healthServer)
	}()

	// 4. Wait for shutdown signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-signalChan:
		logger.WithField("signal", sig.String()).Info("Received shutdown signal")
	case <-ctx.Done():
		logger.Info("Context cancelled")
	}

	// 5. Graceful shutdown
	logger.Info("Shutting down gracefully...")
	cancel() // Cancel context (stops update loop)

	// Stop health server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Warn("Health server shutdown error")
	}

	// Wait for all goroutines to finish
	wg.Wait()
	logger.Info("Shutdown complete")
}
