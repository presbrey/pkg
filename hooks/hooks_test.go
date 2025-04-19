package hooks

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// TestContext is a simple context type for testing
type TestContext struct {
	Value string
	Order []string
	Mutex sync.Mutex
}

// AddToOrder adds a value to the order slice in a thread-safe manner
func (tc *TestContext) AddToOrder(value string) {
	tc.Mutex.Lock()
	defer tc.Mutex.Unlock()
	tc.Order = append(tc.Order, value)
}

func TestRegistryBasic(t *testing.T) {
	registry := NewRegistry[*TestContext]()

	if registry.Count() != 0 {
		t.Errorf("Expected empty registry, got %d hooks", registry.Count())
	}

	// Register a hook
	registry.Register(func(ctx *TestContext) error {
		ctx.Value = "modified"
		return nil
	})

	if registry.Count() != 1 {
		t.Errorf("Expected 1 hook, got %d hooks", registry.Count())
	}

	// Run hooks
	ctx := &TestContext{Value: "original"}
	errors := registry.RunAll(ctx)

	if errors != nil {
		t.Errorf("Expected no errors, got %v", errors)
	}

	if ctx.Value != "modified" {
		t.Errorf("Expected context value to be 'modified', got '%s'", ctx.Value)
	}

	// Clear registry
	registry.Clear()

	if registry.Count() != 0 {
		t.Errorf("Expected empty registry after clear, got %d hooks", registry.Count())
	}
}

func TestRegistryPriority(t *testing.T) {
	registry := NewRegistry[*TestContext]()

	// Register hooks with different priorities
	registry.RegisterWithPriority(func(ctx *TestContext) error {
		ctx.AddToOrder("third")
		return nil
	}, 5)

	registry.RegisterWithPriority(func(ctx *TestContext) error {
		ctx.AddToOrder("first")
		return nil
	}, -5)

	registry.RegisterWithPriority(func(ctx *TestContext) error {
		ctx.AddToOrder("second")
		return nil
	}, 0)

	// Run hooks
	ctx := &TestContext{Order: make([]string, 0)}
	registry.RunAll(ctx)

	// Check execution order
	expected := []string{"first", "second", "third"}
	for i, v := range expected {
		if i >= len(ctx.Order) || ctx.Order[i] != v {
			t.Errorf("Expected execution order %v, got %v", expected, ctx.Order)
			break
		}
	}
}

func TestRegistryErrors(t *testing.T) {
	registry := NewRegistry[*TestContext]()

	// Register a hook that returns an error
	expectedError := errors.New("hook error")
	registry.Register(func(ctx *TestContext) error {
		return expectedError
	})

	// Run hooks
	ctx := &TestContext{}
	errors := registry.RunAll(ctx)

	if errors == nil {
		t.Errorf("Expected errors, got nil")
	}

	if len(errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(errors))
	}
}

func TestRegistryPanic(t *testing.T) {
	registry := NewRegistry[*TestContext]()

	// Register a hook that panics
	registry.Register(func(ctx *TestContext) error {
		panic("hook panic")
	})

	// Run hooks
	ctx := &TestContext{}
	errors := registry.RunAll(ctx)

	if errors == nil {
		t.Errorf("Expected errors, got nil")
	}

	if len(errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(errors))
	}
}

func TestRegistryConcurrency(t *testing.T) {
	registry := NewRegistry[*TestContext]()

	// Register hooks
	for i := 0; i < 10; i++ {
		priority := int64(i - 5) // Priorities from -5 to 4
		registry.RegisterWithPriority(func(ctx *TestContext) error {
			time.Sleep(1 * time.Millisecond) // Simulate work
			return nil
		}, priority)
	}

	// Run hooks concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := &TestContext{}
			registry.RunAll(ctx)
		}()
	}

	wg.Wait()

	if registry.Count() != 10 {
		t.Errorf("Expected 10 hooks, got %d", registry.Count())
	}
}

func TestRegistryPriorityFilters(t *testing.T) {
	registry := NewRegistry[*TestContext]()

	// Register hooks with different priorities
	hooks := map[int64]string{
		-10: "p-10",
		-5:  "p-5",
		0:   "p0",
		5:   "p5",
		10:  "p10",
	}

	for priority, name := range hooks {
		localName := name // capture loop variable
		registry.RegisterWithPriority(func(ctx *TestContext) error {
			ctx.AddToOrder(localName)
			return nil
		}, priority)
	}

	assertOrder := func(t *testing.T, ctx *TestContext, expected []string) {
		t.Helper()
		ctx.Mutex.Lock()
		defer ctx.Mutex.Unlock()
		if len(ctx.Order) != len(expected) {
			t.Errorf("Expected order length %d, got %d. Expected: %v, Got: %v", len(expected), len(ctx.Order), expected, ctx.Order)
			return
		}
		for i, v := range expected {
			if ctx.Order[i] != v {
				t.Errorf("Expected execution order %v, got %v at index %d", expected, ctx.Order, i)
				break
			}
		}
	}

	resetContext := func() *TestContext {
		return &TestContext{Order: make([]string, 0)}
	}

	t.Run("RunPriorityRange", func(t *testing.T) {
		ctx := resetContext()
		registry.RunPriorityRange(ctx, -5, 5) // Includes -5, 0, 5
		assertOrder(t, ctx, []string{"p-5", "p0", "p5"})

		ctx = resetContext()
		registry.RunPriorityRange(ctx, -100, -6) // Includes -10
		assertOrder(t, ctx, []string{"p-10"})

		ctx = resetContext()
		registry.RunPriorityRange(ctx, 6, 100) // Includes 10
		assertOrder(t, ctx, []string{"p10"})

		ctx = resetContext()
		registry.RunPriorityRange(ctx, 0, 0) // Includes 0
		assertOrder(t, ctx, []string{"p0"})

		ctx = resetContext()
		registry.RunPriorityRange(ctx, 20, 30) // Includes none
		assertOrder(t, ctx, []string{})
	})

	t.Run("RunPriorityLessThan", func(t *testing.T) {
		ctx := resetContext()
		registry.RunPriorityLessThan(ctx, 0) // Includes -10, -5
		assertOrder(t, ctx, []string{"p-10", "p-5"})

		ctx = resetContext()
		registry.RunPriorityLessThan(ctx, 6) // Includes -10, -5, 0, 5
		assertOrder(t, ctx, []string{"p-10", "p-5", "p0", "p5"})

		ctx = resetContext()
		registry.RunPriorityLessThan(ctx, -10) // Includes none
		assertOrder(t, ctx, []string{})
	})

	t.Run("RunPriorityGreaterThan", func(t *testing.T) {
		ctx := resetContext()
		registry.RunPriorityGreaterThan(ctx, 0) // Includes 5, 10
		assertOrder(t, ctx, []string{"p5", "p10"})

		ctx = resetContext()
		registry.RunPriorityGreaterThan(ctx, -6) // Includes -5, 0, 5, 10
		assertOrder(t, ctx, []string{"p-5", "p0", "p5", "p10"})

		ctx = resetContext()
		registry.RunPriorityGreaterThan(ctx, 10) // Includes none
		assertOrder(t, ctx, []string{})
	})

	t.Run("RunPriorityLevel", func(t *testing.T) {
		ctx := resetContext()
		registry.RunPriorityLevel(ctx, 0) // Includes p0
		assertOrder(t, ctx, []string{"p0"})

		ctx = resetContext()
		registry.RunPriorityLevel(ctx, -10) // Includes p-10
		assertOrder(t, ctx, []string{"p-10"})

		ctx = resetContext()
		registry.RunPriorityLevel(ctx, 7) // Includes none
		assertOrder(t, ctx, []string{})
	})

	t.Run("RunEarly", func(t *testing.T) {
		ctx := resetContext()
		registry.RunEarly(ctx) // Should run p-10, p-5
		assertOrder(t, ctx, []string{"p-10", "p-5"})
	})

	t.Run("RunMiddle", func(t *testing.T) {
		ctx := resetContext()
		registry.RunMiddle(ctx) // Should run p0
		assertOrder(t, ctx, []string{"p0"})
	})

	t.Run("RunLate", func(t *testing.T) {
		ctx := resetContext()
		registry.RunLate(ctx) // Should run p5, p10
		assertOrder(t, ctx, []string{"p5", "p10"})
	})
}

func BenchmarkRegistryExecution(b *testing.B) {
	registry := NewRegistry[*TestContext]()

	// Register 100 hooks with random priorities
	for i := 0; i < 100; i++ {
		priority := int64(i % 10) // Priorities from 0 to 9
		registry.RegisterWithPriority(func(ctx *TestContext) error {
			return nil
		}, priority)
	}

	ctx := &TestContext{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.RunAll(ctx)
	}
}
