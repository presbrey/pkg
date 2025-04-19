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
	// Sort hooks by priority (lowest first) after each registration
	sort.Slice(r.hooks, func(i, j int) bool {
		return r.hooks[i].Priority < r.hooks[j].Priority
	})
}

// runHooksWithFilter is a helper to execute hooks matching a filter, in priority order.
func (r *Registry[T]) runHooksWithFilter(context T, filter func(HookInfo[T]) bool) map[string]error {
	r.mu.RLock()
	hooks := make([]HookInfo[T], 0, len(r.hooks))
	for _, hi := range r.hooks {
		if filter == nil || filter(hi) {
			hooks = append(hooks, hi)
		}
	}
	r.mu.RUnlock()

	hookErrors := make(map[string]error)

	for _, hookInfo := range hooks {
		err := func() error {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("PANIC in hook %s: %v", hookInfo.Name, r)
					err := fmt.Errorf("panic in hook %s: %v", hookInfo.Name, r)
					hookErrors[hookInfo.Name] = err
				}
			}()
			return hookInfo.Hook(context)
		}()
		if err != nil && hookErrors[hookInfo.Name] == nil {
			hookErrors[hookInfo.Name] = err
			log.Printf("ERROR in hook %s: %v", hookInfo.Name, err)
		}
	}

	if len(hookErrors) == 0 {
		return nil
	}
	return hookErrors
}

// RunEarly executes hooks with priority < 0
func (r *Registry[T]) RunEarly(context T) map[string]error {
	return r.runHooksWithFilter(context, func(hi HookInfo[T]) bool { return hi.Priority < 0 })
}

// RunMiddle executes hooks with priority == 0
func (r *Registry[T]) RunMiddle(context T) map[string]error {
	return r.runHooksWithFilter(context, func(hi HookInfo[T]) bool { return hi.Priority == 0 })
}

// RunLate executes hooks with priority > 0
func (r *Registry[T]) RunLate(context T) map[string]error {
	return r.runHooksWithFilter(context, func(hi HookInfo[T]) bool { return hi.Priority > 0 })
}

// RunAll executes all hooks in order: Early, Middle, Late
// Returns a map of hook names to errors for any hooks that failed
func (r *Registry[T]) RunAll(context T) map[string]error {
	allErrs := make(map[string]error)
	for _, run := range []func(T) map[string]error{r.RunEarly, r.RunMiddle, r.RunLate} {
		errMap := run(context)
		for k, v := range errMap {
			allErrs[k] = v
		}
	}
	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
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
