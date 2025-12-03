package main

import (
	"testing"
	"time"
)

func TestNewExponentialBackoff(t *testing.T) {
	tests := []struct {
		name         string
		initialDelay time.Duration
		maxDelay     time.Duration
		multiplier   float64
		wantPanic    bool
	}{
		{
			name:         "valid configuration",
			initialDelay: 1 * time.Second,
			maxDelay:     60 * time.Second,
			multiplier:   2.0,
			wantPanic:    false,
		},
		{
			name:         "minimum values",
			initialDelay: 1 * time.Millisecond,
			maxDelay:     1 * time.Millisecond,
			multiplier:   1.0,
			wantPanic:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("Expected panic but didn't get one")
					}
				}()
			}

			eb := NewExponentialBackoff(tt.initialDelay, tt.maxDelay, tt.multiplier)

			if !tt.wantPanic {
				if eb == nil {
					t.Fatal("NewExponentialBackoff returned nil")
				}
				if eb.InitialDelay != tt.initialDelay {
					t.Errorf("InitialDelay = %v, want %v", eb.InitialDelay, tt.initialDelay)
				}
				if eb.MaxDelay != tt.maxDelay {
					t.Errorf("MaxDelay = %v, want %v", eb.MaxDelay, tt.maxDelay)
				}
				if eb.Multiplier != tt.multiplier {
					t.Errorf("Multiplier = %v, want %v", eb.Multiplier, tt.multiplier)
				}
				if eb.currentDelay != 0 {
					t.Errorf("currentDelay should start at 0, got %v", eb.currentDelay)
				}
			}
		})
	}
}

func TestExponentialBackoff_Sequence(t *testing.T) {
	// Test the standard exponential backoff sequence: 1s, 2s, 4s, 8s, 16s, 32s, 60s, 60s...
	eb := NewExponentialBackoff(1*time.Second, 60*time.Second, 2.0)

	expectedSequence := []time.Duration{
		1 * time.Second,  // First call
		2 * time.Second,  // 1 * 2
		4 * time.Second,  // 2 * 2
		8 * time.Second,  // 4 * 2
		16 * time.Second, // 8 * 2
		32 * time.Second, // 16 * 2
		60 * time.Second, // 32 * 2 = 64, but capped at 60
		60 * time.Second, // Stays at max
		60 * time.Second, // Stays at max
	}

	for i, expected := range expectedSequence {
		got := eb.Next()
		if got != expected {
			t.Errorf("Next() call %d = %v, want %v", i+1, got, expected)
		}
	}
}

func TestExponentialBackoff_MaxDelay(t *testing.T) {
	// Verify that delay never exceeds MaxDelay
	eb := NewExponentialBackoff(1*time.Second, 10*time.Second, 2.0)

	// Call Next() many times to ensure we hit and stay at max
	var lastDelay time.Duration
	for i := 0; i < 20; i++ {
		delay := eb.Next()
		if delay > eb.MaxDelay {
			t.Errorf("Next() returned %v, which exceeds MaxDelay %v", delay, eb.MaxDelay)
		}
		if delay < lastDelay {
			t.Errorf("Next() returned %v, which is less than previous %v (should be monotonically increasing until max)", delay, lastDelay)
		}
		lastDelay = delay
	}

	// After many calls, should be at MaxDelay
	if lastDelay != eb.MaxDelay {
		t.Errorf("After many calls, delay = %v, want MaxDelay %v", lastDelay, eb.MaxDelay)
	}
}

func TestExponentialBackoff_Reset(t *testing.T) {
	eb := NewExponentialBackoff(1*time.Second, 60*time.Second, 2.0)

	// Advance to some later state
	eb.Next()                 // 1s
	eb.Next()                 // 2s
	eb.Next()                 // 4s
	currentDelay := eb.Next() // 8s

	if currentDelay != 8*time.Second {
		t.Errorf("Before reset, delay = %v, want 8s", currentDelay)
	}

	// Reset should go back to initial state
	eb.Reset()

	// Next call after reset should return InitialDelay
	firstAfterReset := eb.Next()
	if firstAfterReset != eb.InitialDelay {
		t.Errorf("After Reset(), Next() = %v, want InitialDelay %v", firstAfterReset, eb.InitialDelay)
	}

	// Verify sequence starts over
	secondAfterReset := eb.Next()
	expected := time.Duration(float64(eb.InitialDelay) * eb.Multiplier)
	if secondAfterReset != expected {
		t.Errorf("Second call after Reset() = %v, want %v", secondAfterReset, expected)
	}
}

func TestExponentialBackoff_DifferentMultipliers(t *testing.T) {
	tests := []struct {
		name       string
		initial    time.Duration
		max        time.Duration
		multiplier float64
		calls      int
		wantLast   time.Duration
	}{
		{
			name:       "multiplier 1.5",
			initial:    1 * time.Second,
			max:        100 * time.Second,
			multiplier: 1.5,
			calls:      5,
			wantLast:   time.Duration(float64(1*time.Second) * 1.5 * 1.5 * 1.5 * 1.5),
		},
		{
			name:       "multiplier 3.0",
			initial:    1 * time.Second,
			max:        100 * time.Second,
			multiplier: 3.0,
			calls:      3,
			wantLast:   time.Duration(float64(1*time.Second) * 3.0 * 3.0),
		},
		{
			name:       "multiplier 1.0 (no growth)",
			initial:    5 * time.Second,
			max:        100 * time.Second,
			multiplier: 1.0,
			calls:      10,
			wantLast:   5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := NewExponentialBackoff(tt.initial, tt.max, tt.multiplier)

			var lastDelay time.Duration
			for i := 0; i < tt.calls; i++ {
				lastDelay = eb.Next()
			}

			if lastDelay != tt.wantLast {
				t.Errorf("After %d calls, delay = %v, want %v", tt.calls, lastDelay, tt.wantLast)
			}
		})
	}
}

func TestExponentialBackoff_EdgeCases(t *testing.T) {
	t.Run("initial equals max", func(t *testing.T) {
		eb := NewExponentialBackoff(10*time.Second, 10*time.Second, 2.0)

		// Should always return the same value
		for i := 0; i < 5; i++ {
			delay := eb.Next()
			if delay != 10*time.Second {
				t.Errorf("Next() call %d = %v, want 10s", i+1, delay)
			}
		}
	})

	t.Run("initial greater than max", func(t *testing.T) {
		// This is an edge case - the backoff should cap at MaxDelay
		eb := NewExponentialBackoff(100*time.Second, 10*time.Second, 2.0)

		delay := eb.Next()
		if delay > eb.MaxDelay {
			t.Errorf("Next() = %v, should not exceed MaxDelay %v", delay, eb.MaxDelay)
		}
	})

	t.Run("very small delays", func(t *testing.T) {
		eb := NewExponentialBackoff(1*time.Nanosecond, 1*time.Microsecond, 2.0)

		// Should work with very small time durations
		delay := eb.Next()
		if delay != 1*time.Nanosecond {
			t.Errorf("Next() = %v, want 1ns", delay)
		}

		// Eventually caps at max
		for i := 0; i < 30; i++ {
			delay = eb.Next()
		}
		if delay != eb.MaxDelay {
			t.Errorf("After many calls with small delays, got %v, want MaxDelay %v", delay, eb.MaxDelay)
		}
	})
}

func TestExponentialBackoff_Concurrent(t *testing.T) {
	// Test that multiple goroutines can safely use separate backoff instances
	// (This tests that the type itself doesn't have hidden shared state)
	eb1 := NewExponentialBackoff(1*time.Second, 60*time.Second, 2.0)
	eb2 := NewExponentialBackoff(1*time.Second, 60*time.Second, 2.0)

	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 5; i++ {
			eb1.Next()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 5; i++ {
			eb2.Next()
		}
		done <- true
	}()

	<-done
	<-done

	// Both should have advanced independently
	delay1 := eb1.Next()
	delay2 := eb2.Next()

	if delay1 != delay2 {
		t.Errorf("Independent backoffs diverged: eb1.Next() = %v, eb2.Next() = %v", delay1, delay2)
	}
}

// BenchmarkExponentialBackoff measures the performance of Next()
func BenchmarkExponentialBackoff(b *testing.B) {
	eb := NewExponentialBackoff(1*time.Second, 60*time.Second, 2.0)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		eb.Next()
		if i%100 == 0 {
			eb.Reset() // Prevent staying at max the whole time
		}
	}
}
