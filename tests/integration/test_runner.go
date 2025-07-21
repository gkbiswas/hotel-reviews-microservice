package integration

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// TestRunner provides utilities for running integration tests
type TestRunner struct {
	containers    []testcontainers.Container
	cleanupFuncs  []func() error
	timeout       time.Duration
	skipOnCICD    bool
	requirements  []string
}

// NewTestRunner creates a new test runner with default configuration
func NewTestRunner() *TestRunner {
	return &TestRunner{
		containers:   make([]testcontainers.Container, 0),
		cleanupFuncs: make([]func() error, 0),
		timeout:      5 * time.Minute,
		skipOnCICD:   false,
		requirements: []string{"docker"},
	}
}

// SetTimeout sets the maximum time for integration tests to run
func (tr *TestRunner) SetTimeout(timeout time.Duration) *TestRunner {
	tr.timeout = timeout
	return tr
}

// SkipOnCICD configures whether to skip tests in CI/CD environments
func (tr *TestRunner) SkipOnCICD(skip bool) *TestRunner {
	tr.skipOnCICD = skip
	return tr
}

// AddRequirement adds a system requirement for running tests
func (tr *TestRunner) AddRequirement(requirement string) *TestRunner {
	tr.requirements = append(tr.requirements, requirement)
	return tr
}

// CheckRequirements verifies that all system requirements are met
func (tr *TestRunner) CheckRequirements(t *testing.T) {
	// Check if running in CI/CD and should skip
	if tr.skipOnCICD && (os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "") {
		t.Skip("Skipping integration tests in CI/CD environment")
	}

	// Check Docker availability
	for _, req := range tr.requirements {
		switch req {
		case "docker":
			if !tr.isDockerAvailable() {
				t.Skip("Docker is not available, skipping integration tests")
			}
		}
	}
}

// isDockerAvailable checks if Docker is available and running
func (tr *TestRunner) isDockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to get Docker provider
	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		return false
	}

	// Try to create a minimal container to test Docker connectivity
	req := testcontainers.ContainerRequest{
		Image: "alpine:latest",
		Cmd:   []string{"echo", "test"},
	}

	container, err := provider.CreateContainer(ctx, req)
	if err != nil {
		return false
	}

	// Clean up test container
	container.Terminate(ctx)
	return true
}

// RegisterContainer adds a container to be managed by the test runner
func (tr *TestRunner) RegisterContainer(container testcontainers.Container) {
	tr.containers = append(tr.containers, container)
}

// RegisterCleanup adds a cleanup function to be called when tests complete
func (tr *TestRunner) RegisterCleanup(cleanup func() error) {
	tr.cleanupFuncs = append(tr.cleanupFuncs, cleanup)
}

// Cleanup performs cleanup of all registered resources
func (tr *TestRunner) Cleanup() {
	ctx := context.Background()

	// Stop all containers
	for _, container := range tr.containers {
		if err := container.Terminate(ctx); err != nil {
			log.Printf("Warning: Failed to terminate container: %v", err)
		}
	}

	// Run cleanup functions
	for _, cleanup := range tr.cleanupFuncs {
		if err := cleanup(); err != nil {
			log.Printf("Warning: Cleanup function failed: %v", err)
		}
	}
}

// RunWithTimeout runs a test function with the configured timeout
func (tr *TestRunner) RunWithTimeout(t *testing.T, testFunc func(t *testing.T)) {
	done := make(chan bool, 1)
	var testErr error

	go func() {
		defer func() {
			if r := recover(); r != nil {
				testErr = fmt.Errorf("test panicked: %v", r)
			}
			done <- true
		}()
		testFunc(t)
	}()

	select {
	case <-done:
		if testErr != nil {
			t.Fatal(testErr)
		}
	case <-time.After(tr.timeout):
		t.Fatalf("Test timed out after %v", tr.timeout)
	}
}

// TestEnvironmentInfo provides information about the test environment
type TestEnvironmentInfo struct {
	DockerVersion     string
	AvailableMemory   string
	AvailableDisk     string
	NetworkMode       string
	Platform          string
	TestTimeout       time.Duration
	ContainersRunning int
}

// GetEnvironmentInfo returns information about the current test environment
func (tr *TestRunner) GetEnvironmentInfo() TestEnvironmentInfo {
	return TestEnvironmentInfo{
		TestTimeout:       tr.timeout,
		ContainersRunning: len(tr.containers),
		Platform:          "docker",
		NetworkMode:       "bridge",
	}
}

// TestSuiteRunner manages multiple test suites
type TestSuiteRunner struct {
	suites []TestSuite
	runner *TestRunner
}

// TestSuite represents a collection of related tests
type TestSuite struct {
	Name        string
	Description string
	TestFunc    func(t *testing.T)
	SetupFunc   func(t *testing.T) interface{}
	TeardownFunc func(t *testing.T, suite interface{})
	Parallel    bool
	Tags        []string
}

// NewTestSuiteRunner creates a new test suite runner
func NewTestSuiteRunner() *TestSuiteRunner {
	return &TestSuiteRunner{
		suites: make([]TestSuite, 0),
		runner: NewTestRunner(),
	}
}

// AddSuite adds a test suite to the runner
func (tsr *TestSuiteRunner) AddSuite(suite TestSuite) {
	tsr.suites = append(tsr.suites, suite)
}

// RunAllSuites runs all registered test suites
func (tsr *TestSuiteRunner) RunAllSuites(t *testing.T) {
	tsr.runner.CheckRequirements(t)
	defer tsr.runner.Cleanup()

	for _, suite := range tsr.suites {
		t.Run(suite.Name, func(t *testing.T) {
			if suite.Parallel {
				t.Parallel()
			}

			var suiteInstance interface{}
			if suite.SetupFunc != nil {
				suiteInstance = suite.SetupFunc(t)
			}

			defer func() {
				if suite.TeardownFunc != nil {
					suite.TeardownFunc(t, suiteInstance)
				}
			}()

			tsr.runner.RunWithTimeout(t, suite.TestFunc)
		})
	}
}

// RunSuitesByTag runs test suites that match any of the given tags
func (tsr *TestSuiteRunner) RunSuitesByTag(t *testing.T, tags ...string) {
	tsr.runner.CheckRequirements(t)
	defer tsr.runner.Cleanup()

	tagMap := make(map[string]bool)
	for _, tag := range tags {
		tagMap[tag] = true
	}

	for _, suite := range tsr.suites {
		// Check if suite has any of the requested tags
		hasTag := false
		for _, suiteTag := range suite.Tags {
			if tagMap[suiteTag] {
				hasTag = true
				break
			}
		}

		if !hasTag && len(tags) > 0 {
			continue
		}

		t.Run(suite.Name, func(t *testing.T) {
			if suite.Parallel {
				t.Parallel()
			}

			var suiteInstance interface{}
			if suite.SetupFunc != nil {
				suiteInstance = suite.SetupFunc(t)
			}

			defer func() {
				if suite.TeardownFunc != nil {
					suite.TeardownFunc(t, suiteInstance)
				}
			}()

			tsr.runner.RunWithTimeout(t, suite.TestFunc)
		})
	}
}

// TestMetrics tracks test execution metrics
type TestMetrics struct {
	TotalTests       int
	PassedTests      int
	FailedTests      int
	SkippedTests     int
	TotalDuration    time.Duration
	AverageDuration  time.Duration
	ContainersUsed   int
	MemoryUsed       int64
}

// MetricsCollector collects metrics during test execution
type MetricsCollector struct {
	metrics   TestMetrics
	startTime time.Time
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		startTime: time.Now(),
	}
}

// Start begins metrics collection
func (mc *MetricsCollector) Start() {
	mc.startTime = time.Now()
}

// RecordTest records the result of a test
func (mc *MetricsCollector) RecordTest(passed bool, skipped bool, duration time.Duration) {
	mc.metrics.TotalTests++
	mc.metrics.TotalDuration += duration

	if skipped {
		mc.metrics.SkippedTests++
	} else if passed {
		mc.metrics.PassedTests++
	} else {
		mc.metrics.FailedTests++
	}

	if mc.metrics.TotalTests > 0 {
		mc.metrics.AverageDuration = mc.metrics.TotalDuration / time.Duration(mc.metrics.TotalTests)
	}
}

// GetMetrics returns the collected metrics
func (mc *MetricsCollector) GetMetrics() TestMetrics {
	return mc.metrics
}

// PrintSummary prints a summary of test execution
func (mc *MetricsCollector) PrintSummary() {
	fmt.Printf("\n=== Integration Test Summary ===\n")
	fmt.Printf("Total Tests: %d\n", mc.metrics.TotalTests)
	fmt.Printf("Passed: %d\n", mc.metrics.PassedTests)
	fmt.Printf("Failed: %d\n", mc.metrics.FailedTests)
	fmt.Printf("Skipped: %d\n", mc.metrics.SkippedTests)
	fmt.Printf("Total Duration: %v\n", mc.metrics.TotalDuration)
	fmt.Printf("Average Duration: %v\n", mc.metrics.AverageDuration)
	fmt.Printf("Containers Used: %d\n", mc.metrics.ContainersUsed)
	
	if mc.metrics.FailedTests == 0 {
		fmt.Printf("✅ All tests passed!\n")
	} else {
		fmt.Printf("❌ %d tests failed\n", mc.metrics.FailedTests)
	}
	fmt.Printf("===============================\n")
}

// Helper functions for common test patterns

// WaitFor waits for a condition to be true with timeout
func WaitFor(condition func() bool, timeout time.Duration, interval time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(interval)
	}
	return false
}

// RetryOperation retries an operation up to maxRetries times
func RetryOperation(operation func() error, maxRetries int, delay time.Duration) error {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			time.Sleep(delay)
		}
		
		if err := operation(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

// ExpectEventually waits for a condition to eventually be true
func ExpectEventually(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	if !WaitFor(condition, timeout, 100*time.Millisecond) {
		t.Fatalf("Condition was not met within %v: %s", timeout, message)
	}
}