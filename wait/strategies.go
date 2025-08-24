package wait

import (
	"math"
	"math/rand"
	"time"
)

// FixedStrategy waits for a fixed duration between attempts
type FixedStrategy struct {
	duration time.Duration
}

// NewFixedStrategy creates a new fixed wait strategy
func NewFixedStrategy(duration time.Duration) *FixedStrategy {
	return &FixedStrategy{duration: duration}
}

// Next returns the next wait duration
func (s *FixedStrategy) Next() (time.Duration, bool) {
	return s.duration, true
}

// Reset resets the strategy
func (s *FixedStrategy) Reset() {}

// LinearStrategy increases wait time linearly
type LinearStrategy struct {
	initial   time.Duration
	increment time.Duration
	max       time.Duration
	current   time.Duration
}

// NewLinearStrategy creates a new linear wait strategy
func NewLinearStrategy(initial, increment, max time.Duration) *LinearStrategy {
	return &LinearStrategy{
		initial:   initial,
		increment: increment,
		max:       max,
		current:   initial,
	}
}

// Next returns the next wait duration
func (s *LinearStrategy) Next() (time.Duration, bool) {
	duration := s.current
	s.current += s.increment
	if s.max > 0 && s.current > s.max {
		s.current = s.max
	}
	return duration, true
}

// Reset resets the strategy
func (s *LinearStrategy) Reset() {
	s.current = s.initial
}

// ExponentialBackoffStrategy implements exponential backoff with optional jitter
type ExponentialBackoffStrategy struct {
	initial    time.Duration
	multiplier float64
	max        time.Duration
	jitter     bool
	attempt    int
}

// NewExponentialBackoffStrategy creates a new exponential backoff strategy
func NewExponentialBackoffStrategy(initial time.Duration, multiplier float64, max time.Duration, jitter bool) *ExponentialBackoffStrategy {
	return &ExponentialBackoffStrategy{
		initial:    initial,
		multiplier: multiplier,
		max:        max,
		jitter:     jitter,
		attempt:    0,
	}
}

// Next returns the next wait duration
func (s *ExponentialBackoffStrategy) Next() (time.Duration, bool) {
	duration := time.Duration(float64(s.initial) * math.Pow(s.multiplier, float64(s.attempt)))
	
	if s.max > 0 && duration > s.max {
		duration = s.max
	}
	
	if s.jitter {
		// Add random jitter (±25% of duration)
		jitterRange := float64(duration) * 0.25
		jitter := (rand.Float64() - 0.5) * 2 * jitterRange
		duration = time.Duration(float64(duration) + jitter)
		if duration < 0 {
			duration = 0
		}
	}
	
	s.attempt++
	return duration, true
}

// Reset resets the strategy
func (s *ExponentialBackoffStrategy) Reset() {
	s.attempt = 0
}

// FibonacciStrategy implements Fibonacci sequence based backoff
type FibonacciStrategy struct {
	unit    time.Duration
	max     time.Duration
	prev    time.Duration
	current time.Duration
}

// NewFibonacciStrategy creates a new Fibonacci wait strategy
func NewFibonacciStrategy(unit, max time.Duration) *FibonacciStrategy {
	return &FibonacciStrategy{
		unit:    unit,
		max:     max,
		prev:    0,
		current: unit,
	}
}

// Next returns the next wait duration
func (s *FibonacciStrategy) Next() (time.Duration, bool) {
	duration := s.current
	
	next := s.prev + s.current
	s.prev = s.current
	s.current = next
	
	if s.max > 0 && duration > s.max {
		duration = s.max
	}
	
	return duration, true
}

// Reset resets the strategy
func (s *FibonacciStrategy) Reset() {
	s.prev = 0
	s.current = s.unit
}

// CustomStrategy allows custom wait durations
type CustomStrategy struct {
	durations []time.Duration
	index     int
	repeat    bool
}

// NewCustomStrategy creates a new custom wait strategy
func NewCustomStrategy(durations []time.Duration, repeat bool) *CustomStrategy {
	return &CustomStrategy{
		durations: durations,
		repeat:    repeat,
		index:     0,
	}
}

// Next returns the next wait duration
func (s *CustomStrategy) Next() (time.Duration, bool) {
	if s.index >= len(s.durations) {
		if s.repeat && len(s.durations) > 0 {
			s.index = 0
		} else {
			return 0, false
		}
	}
	
	duration := s.durations[s.index]
	s.index++
	return duration, true
}

// Reset resets the strategy
func (s *CustomStrategy) Reset() {
	s.index = 0
}

// DecorrelatedJitterStrategy implements AWS-style decorrelated jitter
type DecorrelatedJitterStrategy struct {
	base    time.Duration
	max     time.Duration
	current time.Duration
}

// NewDecorrelatedJitterStrategy creates a new decorrelated jitter strategy
func NewDecorrelatedJitterStrategy(base, max time.Duration) *DecorrelatedJitterStrategy {
	return &DecorrelatedJitterStrategy{
		base:    base,
		max:     max,
		current: base,
	}
}

// Next returns the next wait duration
func (s *DecorrelatedJitterStrategy) Next() (time.Duration, bool) {
	// Decorrelated jitter: sleep = min(max, random_between(base, sleep * 3))
	min := float64(s.base)
	max := float64(s.current) * 3
	
	if s.max > 0 && time.Duration(max) > s.max {
		max = float64(s.max)
	}
	
	duration := time.Duration(min + rand.Float64()*(max-min))
	s.current = duration
	
	return duration, true
}

// Reset resets the strategy
func (s *DecorrelatedJitterStrategy) Reset() {
	s.current = s.base
}