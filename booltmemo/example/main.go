package main

import (
	"fmt"
	"time"

	"github.com/presbrey/pkg/booltmemo"
)

// A sample function that we want to memoize
// It simulates a function that might be expensive to compute
func isEven(val interface{}) bool {
	// Simulate expensive computation
	time.Sleep(100 * time.Millisecond)

	num, ok := val.(int)
	if !ok {
		return false
	}
	return num%2 == 0
}

func main() {
	// Create a new memoizer:
	// - Cache "true" results for 10 seconds
	// - Cache "false" results for only 5 seconds
	memo := booltmemo.New(isEven, 10*time.Second, 5*time.Second)
	defer memo.Stop() // Ensure cleanup timer is stopped when done

	// First call to check even number (will compute)
	start := time.Now()
	result := memo.Get(42)
	fmt.Printf("First call for 42 took %v: %v\n", time.Since(start), result)

	// Second call to same value (should be instant from cache)
	start = time.Now()
	result = memo.Get(42)
	fmt.Printf("Second call for 42 took %v: %v\n", time.Since(start), result)

	// Check an odd number (will compute)
	start = time.Now()
	result = memo.Get(43)
	fmt.Printf("First call for 43 took %v: %v\n", time.Since(start), result)

	// Wait 6 seconds - this will expire the "false" result for 43
	// but keep the "true" result for 42
	fmt.Println("Waiting 6 seconds...")
	time.Sleep(6 * time.Second)

	// This should be from cache (not expired yet)
	start = time.Now()
	result = memo.Get(42)
	fmt.Printf("Call for 42 after 6s took %v: %v\n", time.Since(start), result)

	// This should recompute (false value expired after 5s)
	start = time.Now()
	result = memo.Get(43)
	fmt.Printf("Call for 43 after 6s took %v: %v\n", time.Since(start), result)

	// We can manually invalidate a cache entry
	memo.Invalidate(42)
	start = time.Now()
	result = memo.Get(42)
	fmt.Printf("Call for 42 after invalidation took %v: %v\n", time.Since(start), result)

	// Wait for the remaining true TTL to expire
	fmt.Println("Waiting 5 more seconds...")
	time.Sleep(5 * time.Second)

	// This should recompute (true value expired after 10s total)
	start = time.Now()
	result = memo.Get(42)
	fmt.Printf("Call for 42 after 11s took %v: %v\n", time.Since(start), result)

	// Clear the entire cache
	memo.Clear()
	fmt.Println("Cache cleared")
}
