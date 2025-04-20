package booltmemo

import (
	"sync"
	"testing"
	"time"
)

// TestMemoizer verifies the basic functionality of the Memoizer
func TestMemoizer(t *testing.T) {
	// A counter to track function calls
	callCount := 0
	var counterMutex sync.Mutex

	// Function that returns true for even numbers and false otherwise
	isEven := func(val interface{}) bool {
		counterMutex.Lock()
		callCount++
		counterMutex.Unlock()

		num, ok := val.(int)
		if !ok {
			return false
		}
		return num%2 == 0
	}

	// Create a memoizer with different TTLs
	trueTTL := 200 * time.Millisecond
	falseTTL := 100 * time.Millisecond
	memo := New(isEven, trueTTL, falseTTL)
	defer memo.Stop()

	// Test initial function calls
	if !memo.Get(2) {
		t.Error("Expected true for 2")
	}
	if memo.Get(3) {
		t.Error("Expected false for 3")
	}

	// At this point, we should have made exactly 2 function calls
	if callCount != 2 {
		t.Errorf("Expected 2 function calls, got %d", callCount)
	}

	// Calling again immediately should use cached values
	memo.Get(2)
	memo.Get(3)

	// Should still be 2 calls
	if callCount != 2 {
		t.Errorf("Expected still 2 function calls, got %d", callCount)
	}

	// Wait for false TTL to expire (100ms + buffer)
	time.Sleep(falseTTL + 10*time.Millisecond)

	// Check after false TTL expiration
	if !memo.Get(2) { // True should still be cached
		t.Error("Expected true for 2 (after false expiration)")
	}
	if callCount != 2 {
		t.Errorf("Expected 2 function calls before false recompute, got %d", callCount)
	}
	if memo.Get(3) { // False should be recomputed
		t.Error("Expected false for 3 (after false expiration)")
	}
	if callCount != 3 {
		t.Fatalf("Expected 3 function calls after false recompute, got %d", callCount)
	}

	// Wait for true TTL to expire (additional 100ms + buffer, total > 200ms)
	time.Sleep(trueTTL - falseTTL + 10*time.Millisecond) // Wait remaining time for true TTL + buffer

	// Check after true TTL expiration - BOTH should have expired by now

	// Check 3 first - should recompute (false)
	if memo.Get(3) { 
		t.Error("Expected false for 3 (after true expiration + recompute)")
	}
	if callCount != 4 { // This is the 4th call
		t.Fatalf("Expected 4 function calls after false recompute (post long sleep), got %d", callCount)
	}

	// Check 2 second - should recompute (true)
	if !memo.Get(2) { 
		t.Error("Expected true for 2 (after true expiration + recompute)")
	}
	if callCount != 5 { // This is the 5th call
		t.Fatalf("Expected 5 function calls after true recompute (post long sleep), got %d", callCount)
	}

	// --- Test repeated expirations ---
	// Current state: 2=true (expires in 200ms), 3=false (expires in 100ms)

	// Wait for false TTL to expire again (100ms + buffer)
	time.Sleep(falseTTL + 10*time.Millisecond)
	if !memo.Get(2) { // True still cached
		t.Error("Expected true for 2 (after 2nd false expiration)")
	}
	if callCount != 5 {
		t.Errorf("Expected 5 function calls before 2nd false recompute, got %d", callCount)
	}
	if memo.Get(3) { // False recomputed
		t.Error("Expected false for 3 (after 2nd false expiration)")
	}
	if callCount != 6 {
		t.Fatalf("Expected 6 function calls after 2nd false recompute, got %d", callCount)
	}

	// Wait for true TTL to expire again (100ms + buffer)
	time.Sleep(trueTTL - falseTTL + 10*time.Millisecond) // ~110ms sleep
	// At this point, BOTH 2 and 3 should have expired

	// Check 3 first - should recompute (false)
	if memo.Get(3) { 
		t.Error("Expected false for 3 (after 2nd expiration + recompute)")
	}
	if callCount != 7 { // This is the 7th call
		t.Errorf("Expected 7 function calls after 2nd false recompute (post long sleep), got %d", callCount)
	}
	
	// Check 2 second - should recompute (true)
	if !memo.Get(2) { 
		t.Error("Expected true for 2 (after 2nd expiration + recompute)")
	}
	if callCount != 8 { // This is the 8th call
		t.Fatalf("Expected 8 function calls after 2nd true recompute (post long sleep), got %d", callCount)
	}

	// Wait for false TTL again (100ms + buffer)
	time.Sleep(falseTTL + 10*time.Millisecond) // ~110ms sleep
	// Key 3 expires, Key 2 still cached
	if !memo.Get(2) { // True still cached
		t.Error("Expected true for 2 (after 3rd false expiration)")
	}
	if callCount != 8 { // No new call yet
		t.Errorf("Expected 8 function calls before 3rd false recompute, got %d", callCount)
	}
	if memo.Get(3) { // False recomputed
		t.Error("Expected false for 3 (after 3rd false expiration)")
	}
	if callCount != 9 { // 9th call
		t.Fatalf("Expected 9 function calls after 3rd false recompute, got %d", callCount)
	}

	// Wait for true TTL again (100ms + buffer)
	time.Sleep(trueTTL - falseTTL + 10*time.Millisecond) // ~110ms sleep
	// Both 2 and 3 should have expired again
	
	// Check 3 first - should recompute (false)
	if memo.Get(3) { 
		t.Error("Expected false for 3 (after 3rd expiration + recompute)")
	}
	if callCount != 10 { // 10th call
		t.Errorf("Expected 10 function calls after 3rd false recompute (post long sleep), got %d", callCount)
	}

	// Check 2 second - should recompute (true)
	if !memo.Get(2) { 
		t.Error("Expected true for 2 (after 3rd expiration + recompute)")
	}
	if callCount != 11 { // 11th call
		t.Fatalf("Expected 11 function calls after 3rd true recompute (post long sleep), got %d", callCount)
	}

	// Test Invalidate
	// Current state: 2=true (expires T+200), 3=false (expires T+100)
	// Invalidate before testing to ensure the function is called
	memo.Invalidate(2)
	memo.Get(2) // This should now call the function
	if callCount != 12 {
		t.Errorf("Expected 12 function calls, got %d", callCount)
	}

	memo.Invalidate(2)
	memo.Get(2) // Should recompute after invalidation
	if callCount != 13 {
		t.Errorf("Expected 13 function calls, got %d", callCount)
	}

	// Test Clear
	memo.Get(4) // Cache a new value
	if callCount != 14 {
		t.Errorf("Expected 14 function calls, got %d", callCount)
	}

	memo.Clear()
	memo.Get(4) // Should recompute after clear
	if callCount != 15 {
		t.Errorf("Expected 15 function calls, got %d", callCount)
	}

	// Check again before expiration
	if !memo.Get(2) {
		t.Error("Expected true for 2")
	}
	if memo.Get(3) {
		t.Error("Expected false for 3")
	}

	// Check again before expiration
	if !memo.Get(2) {
		t.Error("Expected true for 2")
	}
	if memo.Get(3) {
		t.Error("Expected false for 3")
	}

	// Wait for false TTL to expire (100ms + buffer)
	time.Sleep(falseTTL + 10*time.Millisecond)

	// Check after false TTL expiration
	if !memo.Get(2) {
		t.Error("Expected true for 2")
	}
	if memo.Get(3) {
		t.Error("Expected false for 3")
	}

	// Wait for true TTL to expire (additional 100ms + buffer, total > 200ms)
	time.Sleep(trueTTL - falseTTL + 10*time.Millisecond) // Wait remaining time for true TTL + buffer

	// Check after true TTL expiration
	if !memo.Get(2) {
		t.Error("Expected true for 2")
	}
	if memo.Get(3) {
		t.Error("Expected false for 3")
	}

	// --- Test repeated expirations ---

	// Wait for false TTL to expire again (100ms + buffer)
	time.Sleep(falseTTL + 10*time.Millisecond)
	if !memo.Get(2) {
		t.Error("Expected true for 2")
	}
	if memo.Get(3) {
		t.Error("Expected false for 3")
	}

	// Wait for true TTL to expire again (100ms + buffer)
	time.Sleep(trueTTL - falseTTL + 10*time.Millisecond)
	if !memo.Get(2) {
		t.Error("Expected true for 2")
	}
	if memo.Get(3) {
		t.Error("Expected false for 3")
	}

	// Wait for false TTL again (100ms + buffer)
	time.Sleep(falseTTL + 10*time.Millisecond)
	if !memo.Get(2) {
		t.Error("Expected true for 2")
	}
	if memo.Get(3) {
		t.Error("Expected false for 3")
	}

	// Wait for true TTL again (100ms + buffer)
	time.Sleep(trueTTL - falseTTL + 10*time.Millisecond)
	if !memo.Get(2) {
		t.Error("Expected true for 2")
	}
	if memo.Get(3) {
		t.Error("Expected false for 3")
	}
}

// TestConcurrency checks that the memoizer works correctly under concurrent access
func TestConcurrency(t *testing.T) {
	// A counter to track function calls
	callCount := 0
	var counterMutex sync.Mutex

	testFunc := func(val interface{}) bool {
		counterMutex.Lock()
		callCount++
		count := callCount
		counterMutex.Unlock()

		// Simulate work
		time.Sleep(10 * time.Millisecond)
		return count%2 == 0
	}

	memo := New(testFunc, 100*time.Millisecond, 50*time.Millisecond)
	defer memo.Stop()

	// Run concurrent accesses
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each goroutine accesses the same key several times
			for j := 0; j < 5; j++ {
				memo.Get("test-key")
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// We should have far fewer calls than 10 goroutines * 5 calls = 50
	// The exact number depends on timing, but should be small
	if callCount > 5 {
		t.Errorf("Expected fewer function calls with caching, got %d", callCount)
	}
}
