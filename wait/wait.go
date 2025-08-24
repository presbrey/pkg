// Package wait provides utilities for waiting on various conditions with configurable strategies
package wait

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Common errors
var (
	ErrTimeout          = errors.New("wait: timeout exceeded")
	ErrMaxRetriesReached = errors.New("wait: maximum retries reached")
	ErrCanceled         = errors.New("wait: operation canceled")
)

// ConditionFunc represents a function that returns true when a condition is met
type ConditionFunc func() (bool, error)

// ConditionWithResultFunc represents a function that returns a result when condition is met
type ConditionWithResultFunc func() (interface{}, bool, error)

// Strategy defines the interface for wait strategies
type Strategy interface {
	Next() (time.Duration, bool)
	Reset()
}

// Options configures wait behavior
type Options struct {
	MaxRetries int
	Timeout    time.Duration
	Strategy   Strategy
	Context    context.Context
}

// DefaultOptions returns default wait options
func DefaultOptions() *Options {
	return &Options{
		MaxRetries: 10,
		Timeout:    30 * time.Second,
		Strategy:   NewFixedStrategy(1 * time.Second),
		Context:    context.Background(),
	}
}

// WithMaxRetries sets the maximum number of retries
func (o *Options) WithMaxRetries(n int) *Options {
	o.MaxRetries = n
	return o
}

// WithTimeout sets the overall timeout
func (o *Options) WithTimeout(d time.Duration) *Options {
	o.Timeout = d
	return o
}

// WithStrategy sets the wait strategy
func (o *Options) WithStrategy(s Strategy) *Options {
	o.Strategy = s
	return o
}

// WithContext sets the context for cancellation
func (o *Options) WithContext(ctx context.Context) *Options {
	o.Context = ctx
	return o
}

// Until waits until the condition returns true or an error occurs
func Until(condition ConditionFunc, opts ...*Options) error {
	options := mergeOptions(opts...)
	
	ctx, cancel := context.WithTimeout(options.Context, options.Timeout)
	defer cancel()
	
	options.Strategy.Reset()
	attempts := 0
	
	for {
		// Check condition
		ok, err := condition()
		if err != nil {
			return fmt.Errorf("wait: condition error: %w", err)
		}
		if ok {
			return nil
		}
		
		// Check retry limit
		attempts++
		if options.MaxRetries > 0 && attempts >= options.MaxRetries {
			return ErrMaxRetriesReached
		}
		
		// Get next wait duration
		waitDuration, ok := options.Strategy.Next()
		if !ok {
			return ErrMaxRetriesReached
		}
		
		// Wait with context
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return ErrTimeout
			}
			return ErrCanceled
		case <-time.After(waitDuration):
			// Continue to next iteration
		}
	}
}

// UntilWithResult waits until condition is met and returns the result
func UntilWithResult(condition ConditionWithResultFunc, opts ...*Options) (interface{}, error) {
	options := mergeOptions(opts...)
	
	ctx, cancel := context.WithTimeout(options.Context, options.Timeout)
	defer cancel()
	
	options.Strategy.Reset()
	attempts := 0
	
	for {
		// Check condition
		result, ok, err := condition()
		if err != nil {
			return nil, fmt.Errorf("wait: condition error: %w", err)
		}
		if ok {
			return result, nil
		}
		
		// Check retry limit
		attempts++
		if options.MaxRetries > 0 && attempts >= options.MaxRetries {
			return nil, ErrMaxRetriesReached
		}
		
		// Get next wait duration
		waitDuration, ok := options.Strategy.Next()
		if !ok {
			return nil, ErrMaxRetriesReached
		}
		
		// Wait with context
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return nil, ErrTimeout
			}
			return nil, ErrCanceled
		case <-time.After(waitDuration):
			// Continue to next iteration
		}
	}
}

// All waits until all conditions are met
func All(conditions []ConditionFunc, opts ...*Options) error {
	for i, condition := range conditions {
		if err := Until(condition, opts...); err != nil {
			return fmt.Errorf("wait: condition %d failed: %w", i, err)
		}
	}
	return nil
}

// Any waits until any condition is met
func Any(conditions []ConditionFunc, opts ...*Options) (int, error) {
	options := mergeOptions(opts...)
	
	ctx, cancel := context.WithTimeout(options.Context, options.Timeout)
	defer cancel()
	
	options.Strategy.Reset()
	attempts := 0
	
	for {
		// Check all conditions
		for i, condition := range conditions {
			ok, err := condition()
			if err != nil {
				return -1, fmt.Errorf("wait: condition %d error: %w", i, err)
			}
			if ok {
				return i, nil
			}
		}
		
		// Check retry limit
		attempts++
		if options.MaxRetries > 0 && attempts >= options.MaxRetries {
			return -1, ErrMaxRetriesReached
		}
		
		// Get next wait duration
		waitDuration, ok := options.Strategy.Next()
		if !ok {
			return -1, ErrMaxRetriesReached
		}
		
		// Wait with context
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return -1, ErrTimeout
			}
			return -1, ErrCanceled
		case <-time.After(waitDuration):
			// Continue to next iteration
		}
	}
}

// Poll executes a function repeatedly until it succeeds or timeout
func Poll(fn func() error, opts ...*Options) error {
	return Until(func() (bool, error) {
		err := fn()
		return err == nil, nil
	}, opts...)
}

// mergeOptions merges provided options with defaults
func mergeOptions(opts ...*Options) *Options {
	if len(opts) == 0 {
		return DefaultOptions()
	}
	return opts[0]
}