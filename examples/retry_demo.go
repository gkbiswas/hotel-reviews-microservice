package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

// Standalone demo of the retry mechanism functionality
// This demonstrates the key concepts without external dependencies

func main() {
	fmt.Println("=== Retry Mechanism Demo ===")

	// Demo 1: Basic retry with exponential backoff
	fmt.Println("\n1. Basic Retry with Exponential Backoff:")
	demoBasicRetry()

	// Demo 2: Different retry strategies
	fmt.Println("\n2. Different Retry Strategies:")
	demoRetryStrategies()

	// Demo 3: Error classification
	fmt.Println("\n3. Error Classification:")
	demoErrorClassification()

	// Demo 4: Context cancellation
	fmt.Println("\n4. Context Cancellation:")
	demoContextCancellation()

	// Demo 5: Custom retry conditions
	fmt.Println("\n5. Custom Retry Conditions:")
	demoCustomConditions()

	fmt.Println("\n=== Demo Complete ===")
}

// Demo 1: Basic retry with exponential backoff
func demoBasicRetry() {
	maxAttempts := 3
	baseDelay := 100 * time.Millisecond
	multiplier := 2.0

	attempt := 0
	operation := func() error {
		attempt++
		fmt.Printf("  Attempt %d/%d\n", attempt, maxAttempts)

		if attempt < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}

	start := time.Now()
	var err error

	for i := 1; i <= maxAttempts; i++ {
		err = operation()
		if err == nil {
			break
		}

		if i < maxAttempts {
			delay := time.Duration(float64(baseDelay) * (multiplier * float64(i-1)))
			fmt.Printf("  Waiting %v before retry...\n", delay)
			time.Sleep(delay)
		}
	}

	duration := time.Since(start)
	if err != nil {
		fmt.Printf("  Failed after %d attempts in %v: %v\n", maxAttempts, duration, err)
	} else {
		fmt.Printf("  Succeeded after %d attempts in %v\n", attempt, duration)
	}
}

// Demo 2: Different retry strategies
func demoRetryStrategies() {
	strategies := []struct {
		name string
		calc func(attempt int, base time.Duration) time.Duration
	}{
		{
			name: "Fixed Delay",
			calc: func(attempt int, base time.Duration) time.Duration {
				return base
			},
		},
		{
			name: "Linear Backoff",
			calc: func(attempt int, base time.Duration) time.Duration {
				return time.Duration(attempt) * base
			},
		},
		{
			name: "Exponential Backoff",
			calc: func(attempt int, base time.Duration) time.Duration {
				return time.Duration(float64(base) * (2.0 * float64(attempt-1)))
			},
		},
		{
			name: "Fibonacci Backoff",
			calc: func(attempt int, base time.Duration) time.Duration {
				fib := []int{1, 1, 2, 3, 5, 8, 13, 21, 34, 55}
				if attempt-1 >= len(fib) {
					return time.Duration(fib[len(fib)-1]) * base
				}
				return time.Duration(fib[attempt-1]) * base
			},
		},
	}

	for _, strategy := range strategies {
		fmt.Printf("  %s:\n", strategy.name)
		for i := 1; i <= 4; i++ {
			delay := strategy.calc(i, 100*time.Millisecond)
			fmt.Printf("    Attempt %d: %v delay\n", i, delay)
		}
	}
}

// Demo 3: Error classification
func demoErrorClassification() {
	errors := []struct {
		name  string
		error error
		retry bool
	}{
		{"Network Error", errors.New("connection refused"), true},
		{"Timeout Error", errors.New("request timeout"), true},
		{"Server Error", errors.New("500 internal server error"), true},
		{"Rate Limit", errors.New("rate limit exceeded"), true},
		{"Client Error", errors.New("400 bad request"), false},
		{"Auth Error", errors.New("401 unauthorized"), false},
		{"Not Found", errors.New("404 not found"), false},
		{"Invalid Input", errors.New("invalid input format"), false},
	}

	for _, e := range errors {
		retryable := classifyError(e.error)
		status := "Non-retryable"
		if retryable {
			status = "Retryable"
		}
		fmt.Printf("  %s: %s - %s\n", e.name, e.error.Error(), status)
	}
}

// Demo 4: Context cancellation
func demoContextCancellation() {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	attempt := 0
	operation := func() error {
		attempt++
		fmt.Printf("  Attempt %d\n", attempt)

		// Simulate work
		select {
		case <-time.After(100 * time.Millisecond):
			return errors.New("temporary failure")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	start := time.Now()
	var err error

	for i := 1; i <= 3; i++ {
		err = operation()
		if err == context.DeadlineExceeded || err == context.Canceled {
			fmt.Printf("  Context cancelled after %v\n", time.Since(start))
			break
		}

		if err == nil {
			break
		}

		if i < 3 {
			delay := 50 * time.Millisecond
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				fmt.Printf("  Context cancelled during backoff after %v\n", time.Since(start))
				return
			}
		}
	}

	duration := time.Since(start)
	fmt.Printf("  Total duration: %v, Final error: %v\n", duration, err)
}

// Demo 5: Custom retry conditions
func demoCustomConditions() {
	// Custom condition: only retry network errors, and only twice
	customCondition := func(err error, attempt int) bool {
		if attempt >= 3 {
			return false // Don't retry after 3 attempts
		}

		// Only retry network-related errors
		errStr := err.Error()
		return contains(errStr, "network") || contains(errStr, "connection") || contains(errStr, "timeout")
	}

	testErrors := []error{
		errors.New("network connection failed"),
		errors.New("timeout occurred"),
		errors.New("invalid input"),
		errors.New("500 server error"),
	}

	for _, testErr := range testErrors {
		fmt.Printf("  Testing: %s\n", testErr.Error())
		attempt := 0

		for i := 1; i <= 5; i++ {
			attempt++
			fmt.Printf("    Attempt %d: %s\n", attempt, testErr.Error())

			if !customCondition(testErr, attempt) {
				fmt.Printf("    Custom condition says: Do not retry\n")
				break
			}

			if i < 5 {
				fmt.Printf("    Custom condition says: Retry allowed\n")
			}
		}
		fmt.Println()
	}
}

// Helper functions

func classifyError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Retryable errors
	if contains(errStr, "network") || contains(errStr, "connection") ||
		contains(errStr, "timeout") || contains(errStr, "500") ||
		contains(errStr, "502") || contains(errStr, "503") ||
		contains(errStr, "504") || contains(errStr, "rate limit") {
		return true
	}

	// Non-retryable errors
	if contains(errStr, "400") || contains(errStr, "401") ||
		contains(errStr, "403") || contains(errStr, "404") ||
		contains(errStr, "invalid") || contains(errStr, "unauthorized") {
		return false
	}

	// Default to retryable for unknown errors
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				indexOf(s, substr) >= 0)))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
