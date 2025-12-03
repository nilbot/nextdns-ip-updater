package main

import (
	"time"
)

// ExponentialBackoff manages retry timing with exponential backoff strategy.
// It provides a simple way to implement retry logic with increasing delays
// between attempts, capped at a maximum delay.
type ExponentialBackoff struct {
	// InitialDelay is the delay returned on the first call to Next()
	InitialDelay time.Duration

	// MaxDelay is the maximum delay that Next() will return
	MaxDelay time.Duration

	// Multiplier is the factor by which the delay increases on each call
	// For example, 2.0 means the delay doubles each time
	Multiplier float64

	// currentDelay tracks the current state of the backoff
	currentDelay time.Duration
}

// NewExponentialBackoff creates a new ExponentialBackoff with the specified parameters.
//
// Parameters:
//   - initialDelay: The delay to return on the first call to Next()
//   - maxDelay: The maximum delay that will be returned
//   - multiplier: The factor by which to multiply the delay on each call
//
// Example:
//
//	backoff := NewExponentialBackoff(1*time.Second, 60*time.Second, 2.0)
//	// Will produce delays: 1s, 2s, 4s, 8s, 16s, 32s, 60s, 60s, ...
func NewExponentialBackoff(initialDelay, maxDelay time.Duration, multiplier float64) *ExponentialBackoff {
	return &ExponentialBackoff{
		InitialDelay: initialDelay,
		MaxDelay:     maxDelay,
		Multiplier:   multiplier,
		currentDelay: 0, // Start at 0 so first call to Next() returns InitialDelay
	}
}

// Next returns the next delay duration and advances the internal state.
// The delay starts at InitialDelay and grows by Multiplier on each call,
// until it reaches MaxDelay where it stays.
//
// Example usage:
//
//	backoff := NewExponentialBackoff(1*time.Second, 60*time.Second, 2.0)
//	for {
//	    delay := backoff.Next()
//	    time.Sleep(delay)
//	    // ... retry operation ...
//	    if success {
//	        backoff.Reset()
//	        break
//	    }
//	}
func (eb *ExponentialBackoff) Next() time.Duration {
	// First call: return InitialDelay (capped at MaxDelay)
	if eb.currentDelay == 0 {
		eb.currentDelay = eb.InitialDelay
		if eb.currentDelay > eb.MaxDelay {
			eb.currentDelay = eb.MaxDelay
		}
		return eb.currentDelay
	}

	// Calculate next delay: current * multiplier
	nextDelay := time.Duration(float64(eb.currentDelay) * eb.Multiplier)

	// Cap at MaxDelay
	if nextDelay > eb.MaxDelay {
		nextDelay = eb.MaxDelay
	}

	eb.currentDelay = nextDelay
	return eb.currentDelay
}

// Reset resets the backoff to its initial state.
// The next call to Next() will return InitialDelay.
//
// This is typically called after a successful operation to reset
// the backoff for future retry sequences.
func (eb *ExponentialBackoff) Reset() {
	eb.currentDelay = 0
}
