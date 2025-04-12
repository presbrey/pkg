// Package hooks provides a flexible hook registration and execution system with priority support.
package hooks

import (
	"fmt"
	"log"
	"reflect"
	"runtime"
	"sort"
	"sync"
)

// Hook defines a generic hook function that returns an error if it fails
type Hook[T any] func(context T) error

// HookInfo stores information about a registered hook including its priority
type HookInfo[T any] struct {
	Name     string  // Name of the hook function
	Hook     Hook[T] // The hook function itself
	Priority int64   // Priority value (lower values run first, like Unix nice)
}

// Registry manages hook registration and execution for a specific context type
type Registry[T any] struct {
	mu    sync.RWMutex
	hooks []HookInfo[T]
}

// NewRegistry creates a new hook registry for the given context type
func NewRegistry[T any]() *Registry[T] {
	return &Registry[T]{
		hooks: make([]HookInfo[T], 0),
	}
}

// Register adds a new hook to the registry with default priority (0)
func (r *Registry[T]) Register(hook Hook[T]) {
	r.RegisterWithPriority(hook, 0)
}

// RegisterWithPriority adds a new hook to the registry with the specified priority
// Hooks with lower priority values run first (like Unix nice)
func (r *Registry[T]) RegisterWithPriority(hook Hook[T], priority int64) {
	name := runtime.FuncForPC(reflect.ValueOf(hook).Pointer()).Name()

	r.mu.Lock()
	defer r.mu.Unlock()

	r.hooks = append(r.hooks, HookInfo[T]{
		Name:     name,
		Hook:     hook,
		Priority: priority,
	})
}

// RunHooks executes all registered hooks with the provided context
// Hooks are executed in priority order (lower values first)
// It returns a map of hook names to errors for any hooks that failed
func (r *Registry[T]) RunHooks(context T) map[string]error {
	r.mu.RLock()
	// Create a copy of the hooks slice to avoid holding the lock during execution
	hooks := make([]HookInfo[T], len(r.hooks))
	copy(hooks, r.hooks)
	r.mu.RUnlock()

	// Sort hooks by priority (lower values first)
	sort.Slice(hooks, func(i, j int) bool {
		return hooks[i].Priority < hooks[j].Priority
	})

	hookErrors := make(map[string]error)

	for _, hookInfo := range hooks {
		// Recover from panics in hooks to prevent one hook from breaking everything
		err := func() error {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("PANIC in hook %s: %v", hookInfo.Name, r)

					// Convert panic to error
					err := fmt.Errorf("panic in hook %s: %v", hookInfo.Name, r)
					hookErrors[hookInfo.Name] = err
				}
			}()
			return hookInfo.Hook(context)
		}()

		if err != nil && hookErrors[hookInfo.Name] == nil {
			// Only store the error if it's not already set by panic recovery
			hookErrors[hookInfo.Name] = err
			log.Printf("ERROR in hook %s: %v", hookInfo.Name, err)
		}
	}

	if len(hookErrors) == 0 {
		return nil
	}
	return hookErrors
}

// Clear removes all hooks from the registry
func (r *Registry[T]) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.hooks = make([]HookInfo[T], 0)
}

// Count returns the number of registered hooks
func (r *Registry[T]) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.hooks)
}
