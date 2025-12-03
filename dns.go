package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
)

// DNSHealthChecker validates DNS resolution with custom timeouts and retry logic.
// It's designed to verify that DNS is working before attempting HTTP requests.
type DNSHealthChecker struct {
	resolver *net.Resolver
	timeout  time.Duration
}

// NewDNSHealthChecker creates a DNS health checker with a custom resolver.
// The resolver uses a pure Go implementation with the specified timeout for DNS queries.
//
// Parameters:
//   - timeout: Maximum time to wait for a DNS query to complete
//
// Example:
//
//	checker := NewDNSHealthChecker(5 * time.Second)
//	err := checker.CheckDNS(ctx, "https://link-ip.nextdns.io/id/token")
func NewDNSHealthChecker(timeout time.Duration) *DNSHealthChecker {
	// Create a custom resolver with PreferGo=true for pure Go DNS resolution
	// This gives us more control over timeouts and behavior
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: timeout,
			}
			return d.DialContext(ctx, network, address)
		},
	}

	return &DNSHealthChecker{
		resolver: resolver,
		timeout:  timeout,
	}
}

// CheckDNS verifies that DNS resolution works for the given endpoint URL.
// It extracts the hostname from the URL and attempts to resolve it.
//
// Returns nil if DNS resolution succeeds, error otherwise.
func (dhc *DNSHealthChecker) CheckDNS(ctx context.Context, endpoint string) error {
	// Extract hostname from endpoint URL
	hostname, err := dhc.extractHostname(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	// Attempt DNS resolution
	_, err = dhc.resolveDNS(ctx, hostname)
	if err != nil {
		return fmt.Errorf("DNS resolution failed for %s: %w", hostname, err)
	}

	return nil
}

// WaitForDNS blocks until DNS resolution succeeds for the given endpoint,
// using exponential backoff for retries. It will keep retrying until either
// DNS works or the context is cancelled.
//
// Parameters:
//   - ctx: Context for cancellation
//   - endpoint: The URL endpoint to check (hostname will be extracted)
//   - backoff: Exponential backoff strategy for retry delays
//
// Returns:
//   - nil if DNS eventually succeeds
//   - error if context is cancelled before DNS succeeds
//
// Example:
//
//	backoff := NewExponentialBackoff(1*time.Second, 60*time.Second, 2.0)
//	err := checker.WaitForDNS(ctx, endpoint, backoff)
func (dhc *DNSHealthChecker) WaitForDNS(ctx context.Context, endpoint string, backoff *ExponentialBackoff) error {
	attempt := 0

	for {
		attempt++

		err := dhc.CheckDNS(ctx, endpoint)
		if err == nil {
			// DNS check succeeded!
			if attempt > 1 {
				logger.WithFields(logrus.Fields{
					"attempts": attempt,
				}).Info("DNS is now ready")
			}
			return nil
		}

		// Check if context was cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Calculate next retry delay
		delay := backoff.Next()

		logger.WithFields(logrus.Fields{
			"attempt":    attempt,
			"error":      err.Error(),
			"next_retry": delay.String(),
		}).Warn("DNS check failed, retrying")

		// Wait for delay or context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}
}

// extractHostname extracts the hostname from a URL string.
// Returns error if the URL is invalid or has no host.
func (dhc *DNSHealthChecker) extractHostname(endpoint string) (string, error) {
	if endpoint == "" {
		return "", fmt.Errorf("endpoint is empty")
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	if parsedURL.Scheme == "" {
		return "", fmt.Errorf("URL has no scheme")
	}

	if parsedURL.Host == "" {
		return "", fmt.Errorf("URL has no host")
	}

	// Remove port if present (e.g., "example.com:8080" -> "example.com")
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return "", fmt.Errorf("could not extract hostname from URL")
	}

	return hostname, nil
}

// resolveDNS attempts to resolve a hostname to IP addresses using the custom resolver.
// Returns the list of IP addresses if successful, error otherwise.
func (dhc *DNSHealthChecker) resolveDNS(ctx context.Context, hostname string) ([]string, error) {
	// Create a context with timeout for the DNS query
	queryCtx, cancel := context.WithTimeout(ctx, dhc.timeout)
	defer cancel()

	// Use LookupHost to resolve the hostname
	addrs, err := dhc.resolver.LookupHost(queryCtx, hostname)
	if err != nil {
		return nil, err
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("no addresses found for hostname %s", hostname)
	}

	return addrs, nil
}
