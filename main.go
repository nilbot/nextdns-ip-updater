package main

import (
	"net/http"
	"net/url"
	"os"
	"strconv"
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

// updateNextDNS updates NextDNS with the current WAN IP by calling the endpoint directly
func updateNextDNS(endpoint string) bool {
	// Validate endpoint
	parsedURL, err := url.Parse(endpoint)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		logger.WithFields(logrus.Fields{
			"endpoint": endpoint,
			"error":    "invalid NextDNS endpoint",
		}).Error("Invalid NextDNS endpoint")
		return false
	}

	// Create HTTP client with reasonable timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(endpoint)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"endpoint": endpoint,
			"error":    err.Error(),
		}).Error("Error updating NextDNS")
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		logger.WithFields(logrus.Fields{
			"endpoint": endpoint,
		}).Info("Successfully updated NextDNS")
		return true
	}

	logger.WithFields(logrus.Fields{
		"endpoint":    endpoint,
		"status_code": resp.StatusCode,
		"status":      resp.Status,
	}).Error("Failed to update NextDNS")
	return false
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

	logger.WithFields(logrus.Fields{
		"endpoint":         nextdnsEndpoint,
		"interval_seconds": interval,
		"version":          version,
		"build_time":       buildTime,
	}).Info("Starting NextDNS IP updater")

	// Main update loop
	for {
		success := updateNextDNS(nextdnsEndpoint)
		logger.WithFields(logrus.Fields{
			"success": success,
		}).Info("Update cycle completed")

		time.Sleep(time.Duration(interval) * time.Second)
	}
}
