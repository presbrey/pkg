package wait

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// ForFile waits until a file exists
func ForFile(path string, opts ...*Options) error {
	return Until(func() (bool, error) {
		_, err := os.Stat(path)
		return err == nil, nil
	}, opts...)
}

// ForFileContent waits until a file exists and has content
func ForFileContent(path string, minSize int64, opts ...*Options) error {
	return Until(func() (bool, error) {
		info, err := os.Stat(path)
		if err != nil {
			return false, nil
		}
		return info.Size() >= minSize, nil
	}, opts...)
}

// ForFileRemoval waits until a file is removed
func ForFileRemoval(path string, opts ...*Options) error {
	return Until(func() (bool, error) {
		_, err := os.Stat(path)
		return os.IsNotExist(err), nil
	}, opts...)
}

// ForDirectory waits until a directory exists
func ForDirectory(path string, opts ...*Options) error {
	return Until(func() (bool, error) {
		info, err := os.Stat(path)
		if err != nil {
			return false, nil
		}
		return info.IsDir(), nil
	}, opts...)
}

// ForProcess waits until a process with the given PID exists
func ForProcess(pid int, opts ...*Options) error {
	return Until(func() (bool, error) {
		process, err := os.FindProcess(pid)
		if err != nil {
			return false, nil
		}
		
		// Check if process is actually running
		err = process.Signal(syscall.Signal(0))
		return err == nil, nil
	}, opts...)
}

// ForProcessExit waits until a process exits
func ForProcessExit(pid int, opts ...*Options) error {
	return Until(func() (bool, error) {
		process, err := os.FindProcess(pid)
		if err != nil {
			return true, nil // Process not found, consider it exited
		}
		
		err = process.Signal(syscall.Signal(0))
		return err != nil, nil
	}, opts...)
}

// ForCommand waits until a command executes successfully
func ForCommand(name string, args []string, opts ...*Options) error {
	return Until(func() (bool, error) {
		cmd := exec.Command(name, args...)
		err := cmd.Run()
		return err == nil, nil
	}, opts...)
}

// ForEnvironmentVariable waits until an environment variable is set
func ForEnvironmentVariable(key string, opts ...*Options) error {
	return Until(func() (bool, error) {
		_, exists := os.LookupEnv(key)
		return exists, nil
	}, opts...)
}

// ForEnvironmentVariableValue waits until an environment variable has a specific value
func ForEnvironmentVariableValue(key, expectedValue string, opts ...*Options) error {
	return Until(func() (bool, error) {
		value, exists := os.LookupEnv(key)
		return exists && value == expectedValue, nil
	}, opts...)
}

// ForTime waits until a specific time is reached
func ForTime(target time.Time, opts ...*Options) error {
	return Until(func() (bool, error) {
		return time.Now().After(target), nil
	}, opts...)
}

// ForDuration waits for a specific duration from now
func ForDuration(duration time.Duration) error {
	time.Sleep(duration)
	return nil
}

// ForConditionWithTimeout is a convenience function that waits with a simple timeout
func ForConditionWithTimeout(condition ConditionFunc, timeout time.Duration) error {
	return Until(condition, DefaultOptions().WithTimeout(timeout))
}

// Retry executes a function with retries
func Retry(fn func() error, maxRetries int) error {
	return Poll(fn, DefaultOptions().WithMaxRetries(maxRetries))
}

// RetryWithBackoff executes a function with exponential backoff
func RetryWithBackoff(fn func() error, initial time.Duration, maxRetries int) error {
	strategy := NewExponentialBackoffStrategy(initial, 2.0, 30*time.Second, true)
	return Poll(fn, DefaultOptions().
		WithStrategy(strategy).
		WithMaxRetries(maxRetries))
}

// Forever executes a function forever until it succeeds or context is canceled
func Forever(fn func() error, interval time.Duration) error {
	return Poll(fn, &Options{
		MaxRetries: 0, // No retry limit
		Timeout:    0, // No timeout
		Strategy:   NewFixedStrategy(interval),
		Context:    context.Background(),
	})
}

// Group manages multiple wait operations
type Group struct {
	conditions []ConditionFunc
	errors     []error
}

// NewGroup creates a new wait group
func NewGroup() *Group {
	return &Group{
		conditions: []ConditionFunc{},
		errors:     []error{},
	}
}

// Add adds a condition to the group
func (g *Group) Add(condition ConditionFunc) {
	g.conditions = append(g.conditions, condition)
}

// AddFunc adds a function-based condition to the group
func (g *Group) AddFunc(fn func() error) {
	g.Add(func() (bool, error) {
		err := fn()
		return err == nil, err
	})
}

// Wait waits for all conditions in the group
func (g *Group) Wait(opts ...*Options) error {
	g.errors = make([]error, len(g.conditions))
	
	for i, condition := range g.conditions {
		if err := Until(condition, opts...); err != nil {
			g.errors[i] = err
			return fmt.Errorf("wait group: condition %d failed: %w", i, err)
		}
	}
	
	return nil
}

// WaitAny waits for any condition in the group
func (g *Group) WaitAny(opts ...*Options) (int, error) {
	return Any(g.conditions, opts...)
}

// Errors returns any errors from the last Wait operation
func (g *Group) Errors() []error {
	return g.errors
}