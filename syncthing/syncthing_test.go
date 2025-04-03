package syncthing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRemoteMapBasic(t *testing.T) {
	// Create a test server that returns a simple JSON map
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
		"key4": map[string]interface{}{
			"nested": "value",
		},
		"key5": float64(1234567890123456), // Large number that should be handled by int64
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Common refresh period for testing
	refreshPeriod := 100 * time.Millisecond

	// Test with string values
	t.Run("string values", func(t *testing.T) {
		rm := NewMapString[string](server.URL).
			WithRefreshPeriod(refreshPeriod).
			WithTimeout(5 * time.Second).
			Start()
		defer rm.Stop()

		// Wait for initial fetch to complete
		time.Sleep(200 * time.Millisecond)

		// Test that the string data was loaded correctly
		val1, ok := rm.Get("key1")
		if !ok || val1 != "value1" {
			t.Errorf("Expected key1=value1, got %v, ok=%v", val1, ok)
		}

		// Test with default value
		val := rm.GetWithDefault("nonexistent", "default")
		if val != "default" {
			t.Errorf("Expected default value, got %v", val)
		}
	})

	// Test with int values
	t.Run("int values", func(t *testing.T) {
		rm := NewMapString[int](server.URL).
			WithRefreshPeriod(refreshPeriod).
			WithTimeout(5 * time.Second).
			Start()
		defer rm.Stop()

		// Wait for initial fetch to complete
		time.Sleep(200 * time.Millisecond)

		// Test that the int data was loaded correctly
		val2, ok := rm.Get("key2")
		if !ok || val2 != 42 {
			t.Errorf("Expected key2=42, got %v, ok=%v", val2, ok)
		}
	})

	// Test with bool values
	t.Run("bool values", func(t *testing.T) {
		rm := NewMapString[bool](server.URL).
			WithRefreshPeriod(refreshPeriod).
			WithTimeout(5 * time.Second).
			Start()
		defer rm.Stop()

		// Wait for initial fetch to complete
		time.Sleep(200 * time.Millisecond)

		// Test that the bool data was loaded correctly
		val3, ok := rm.Get("key3")
		if !ok || val3 != true {
			t.Errorf("Expected key3=true, got %v, ok=%v", val3, ok)
		}
	})

	// Test with int64 values
	t.Run("int64 values", func(t *testing.T) {
		rm := NewMapString[int64](server.URL).
			WithRefreshPeriod(refreshPeriod).
			WithTimeout(5 * time.Second).
			Start()
		defer rm.Stop()

		// Wait for initial fetch to complete
		time.Sleep(200 * time.Millisecond)

		// Test that the int64 data was loaded correctly
		val5, ok := rm.Get("key5")
		expectedVal := int64(1234567890123456)
		if !ok || val5 != expectedVal {
			t.Errorf("Expected key5=%v, got %v, ok=%v", expectedVal, val5, ok)
		}
	})

	// Test with any values
	t.Run("any values", func(t *testing.T) {
		rm := NewMapString[any](server.URL).
			WithRefreshPeriod(refreshPeriod).
			WithTimeout(5 * time.Second).
			Start()
		defer rm.Stop()

		// Wait for initial fetch to complete
		time.Sleep(200 * time.Millisecond)

		// Test that all data was loaded correctly
		val1, ok := rm.Get("key1")
		if !ok || val1 != "value1" {
			t.Errorf("Expected key1=value1, got %v, ok=%v", val1, ok)
		}

		val2, ok := rm.Get("key2")
		if !ok || val2 != float64(42) { // JSON numbers are float64 by default
			t.Errorf("Expected key2=42, got %v, ok=%v", val2, ok)
		}

		val3, ok := rm.Get("key3")
		if !ok || val3 != true {
			t.Errorf("Expected key3=true, got %v, ok=%v", val3, ok)
		}

		// Test nested map
		val4, ok := rm.GetMap("key4")
		if !ok {
			t.Errorf("Expected to get key4 map, got ok=%v", ok)
		} else if nestedVal, ok := val4["nested"]; !ok || nestedVal != "value" {
			t.Errorf("Expected key4.nested=value, got %v", val4)
		}
	})
}

func TestRemoteMapUpdate(t *testing.T) {
	// Create a test server with changing data
	var mu sync.Mutex
	testData := map[string]interface{}{
		"key1": "initial",
		"key2": 100,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Create a RemoteMap with a short refresh period
	rm := NewMapString[string](server.URL).
		WithRefreshPeriod(100 * time.Millisecond).
		WithTimeout(5 * time.Second).
		Start()

	// Wait for initial fetch to complete
	time.Sleep(200 * time.Millisecond)

	// Verify initial data
	val1, ok := rm.Get("key1")
	if !ok || val1 != "initial" {
		t.Errorf("Expected key1=initial, got %v, ok=%v", val1, ok)
	}

	// Update the test data
	mu.Lock()
	testData = map[string]interface{}{
		"key1": "updated",
		"key3": "new",
	}
	mu.Unlock()

	// Wait for the next refresh
	time.Sleep(200 * time.Millisecond)

	// Verify the data was updated
	val1, ok = rm.Get("key1")
	if !ok || val1 != "updated" {
		t.Errorf("Expected key1=updated, got %v, ok=%v", val1, ok)
	}

	val3, ok := rm.Get("key3")
	if !ok || val3 != "new" {
		t.Errorf("Expected key3=new, got %v, ok=%v", val3, ok)
	}

	// key2 should be gone
	_, ok = rm.Get("key2")
	if ok {
		t.Errorf("Expected key2 to be removed")
	}

	// Clean up
	rm.Stop()
}

func TestRemoteMapUpdateCallback(t *testing.T) {
	// Create a test server with pre-transformed data
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": "42", // Changed to string to work with string RemoteMap
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Track updated keys
	var updatedKeys []string
	var callbackCalled bool
	var mu sync.Mutex

	// Create a RemoteMap with update callback
	rm := NewMapString[string](server.URL).
		WithRefreshPeriod(100 * time.Millisecond).
		WithUpdateCallback(func(updated []string) {
			mu.Lock()
			defer mu.Unlock()
			t.Logf("Update callback called with updated=%v", updated)
			updatedKeys = updated
			callbackCalled = true
		}).
		Start()
	defer rm.Stop()

	// Wait for initial fetch to complete
	time.Sleep(200 * time.Millisecond)

	// Verify callback was called
	mu.Lock()
	t.Logf("After sleep: callbackCalled=%v, updatedKeys=%v", callbackCalled, updatedKeys)
	if !callbackCalled {
		t.Errorf("Expected update callback to be called")
	}
	mu.Unlock()
}

func TestRemoteMapDeleteCallback(t *testing.T) {
	// Create a test server with changing data
	var mu sync.Mutex
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Track deleted keys
	var deletedKeys []string
	var deleteCallbackCalled bool
	var callbackMu sync.Mutex

	// Create a RemoteMap with delete callback
	rm := NewMapString[string](server.URL).
		WithRefreshPeriod(100 * time.Millisecond).
		WithDeleteCallback(func(deleted []string) {
			callbackMu.Lock()
			defer callbackMu.Unlock()
			t.Logf("Delete callback called with deleted=%v", deleted)
			deletedKeys = deleted
			deleteCallbackCalled = true
		}).
		Start()
	defer rm.Stop()

	// Wait for initial fetch to complete
	time.Sleep(200 * time.Millisecond)

	// Verify initial data
	val1, ok := rm.Get("key1")
	if !ok || val1 != "value1" {
		t.Errorf("Expected key1=value1, got %v, ok=%v", val1, ok)
	}

	// Update the test data by removing a key
	mu.Lock()
	testData = map[string]interface{}{
		"key1": "value1",
		// key2 is removed
		"key3": "updated",
	}
	mu.Unlock()

	// Wait for the next refresh
	time.Sleep(200 * time.Millisecond)

	// Verify the delete callback was called
	callbackMu.Lock()
	if !deleteCallbackCalled {
		t.Errorf("Expected delete callback to be called")
	}

	// Verify the correct key was reported as deleted
	if len(deletedKeys) != 1 || deletedKeys[0] != "key2" {
		t.Errorf("Expected [key2] to be deleted, got %v", deletedKeys)
	}
	callbackMu.Unlock()

	// Verify key2 is gone
	_, ok = rm.Get("key2")
	if ok {
		t.Errorf("Expected key2 to be removed")
	}

	// Verify key3 was updated
	val3, ok := rm.Get("key3")
	if !ok || val3 != "updated" {
		t.Errorf("Expected key3=updated, got %v, ok=%v", val3, ok)
	}
}

func TestRemoteMapRefreshCallback(t *testing.T) {
	// Create a test server with static data
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Track refresh callback calls
	var refreshCallCount int
	var callbackMu sync.Mutex

	// Create a RemoteMap with refresh callback
	rm := NewMapString[string](server.URL).
		WithRefreshPeriod(100 * time.Millisecond).
		WithRefreshCallback(func() {
			callbackMu.Lock()
			defer callbackMu.Unlock()
			refreshCallCount++
			t.Logf("Refresh callback called, count: %d", refreshCallCount)
		}).
		Start()
	defer rm.Stop()

	// Wait for initial fetch and a couple of refresh cycles
	time.Sleep(250 * time.Millisecond)

	// Verify refresh callback was called
	callbackMu.Lock()
	if refreshCallCount < 1 {
		t.Errorf("Expected refresh callback to be called at least once, got %d", refreshCallCount)
	}
	callbackMu.Unlock()

	// Manually trigger a refresh and verify callback is called again
	initialCount := refreshCallCount
	err := rm.Refresh()
	if err != nil {
		t.Errorf("Unexpected error during manual refresh: %v", err)
	}

	callbackMu.Lock()
	if refreshCallCount <= initialCount {
		t.Errorf("Expected refresh callback to be called after manual refresh")
	}
	callbackMu.Unlock()
}

func TestGetTypedMaps(t *testing.T) {
	// Create a test server
	testData := map[string]interface{}{
		"stringMap": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
		"boolMap": map[string]interface{}{
			"key1": true,
			"key2": false,
		},
		"intMap": map[string]interface{}{
			"key1": 1,
			"key2": 2,
		},
		"floatMap": map[string]interface{}{
			"key1": 1.1,
			"key2": 2.2,
		},
		"mixedMap": map[string]interface{}{
			"key1": "value1",
			"key2": 2,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Common refresh period for testing
	refreshPeriod := 100 * time.Millisecond

	// Test with string values
	t.Run("string map", func(t *testing.T) {
		// Create a RemoteMap with map[string]string type
		rm := NewMapString[map[string]string](server.URL).
			WithRefreshPeriod(refreshPeriod).
			Start()
		defer rm.Stop()

		// Wait for initial fetch to complete
		time.Sleep(200 * time.Millisecond)

		// Test getting a map of strings
		stringMap, ok := rm.Get("stringMap")
		if !ok {
			t.Errorf("Expected to get stringMap")
		} else {
			if stringMap["key1"] != "value1" || stringMap["key2"] != "value2" {
				t.Errorf("Expected stringMap to have correct values, got %v", stringMap)
			}
		}
	})

	// Test with bool values
	t.Run("bool map", func(t *testing.T) {
		// Create a RemoteMap with map[string]bool type
		rm := NewMapString[map[string]bool](server.URL).
			WithRefreshPeriod(refreshPeriod).
			Start()
		defer rm.Stop()

		// Wait for initial fetch to complete
		time.Sleep(200 * time.Millisecond)

		// Test getting a map of booleans
		boolMap, ok := rm.Get("boolMap")
		if !ok {
			t.Errorf("Expected to get boolMap")
		} else {
			if boolMap["key1"] != true || boolMap["key2"] != false {
				t.Errorf("Expected boolMap to have correct values, got %v", boolMap)
			}
		}
	})

	// Test with int values
	t.Run("int map", func(t *testing.T) {
		// Create a RemoteMap with map[string]int type
		rm := NewMapString[map[string]int](server.URL).
			WithRefreshPeriod(refreshPeriod).
			Start()
		defer rm.Stop()

		// Wait for initial fetch to complete
		time.Sleep(200 * time.Millisecond)

		// Test getting a map of ints
		intMap, ok := rm.Get("intMap")
		if !ok {
			t.Errorf("Expected to get intMap")
		} else {
			if intMap["key1"] != 1 || intMap["key2"] != 2 {
				t.Errorf("Expected intMap to have correct values, got %v", intMap)
			}
		}
	})

	// Test with float values
	t.Run("float map", func(t *testing.T) {
		// Create a RemoteMap with map[string]float64 type
		rm := NewMapString[map[string]float64](server.URL).
			WithRefreshPeriod(refreshPeriod).
			Start()
		defer rm.Stop()

		// Wait for initial fetch to complete
		time.Sleep(200 * time.Millisecond)

		// Test getting a map of floats
		floatMap, ok := rm.Get("floatMap")
		if !ok {
			t.Errorf("Expected to get floatMap")
		} else {
			if floatMap["key1"] != 1.1 || floatMap["key2"] != 2.2 {
				t.Errorf("Expected floatMap to have correct values, got %v", floatMap)
			}
		}
	})

	// Test with mixed values (should fail type assertion)
	t.Run("mixed map", func(t *testing.T) {
		// Create a RemoteMap with map[string]string type
		rm := NewMapString[map[string]string](server.URL).
			WithRefreshPeriod(refreshPeriod).
			Start()
		defer rm.Stop()

		// Wait for initial fetch to complete
		time.Sleep(200 * time.Millisecond)

		// Test getting a mixed map as string map (should fail)
		_, ok := rm.Get("mixedMap")
		if ok {
			t.Errorf("Expected Get to fail for mixed types")
		}
	})
}

func TestKeys(t *testing.T) {
	// Create a test server
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Create a RemoteMap
	rm := NewMapString[any](server.URL).
		WithRefreshPeriod(100 * time.Millisecond).
		Start()
	defer rm.Stop()

	// Wait for initial fetch to complete
	time.Sleep(200 * time.Millisecond)

	// Test getting all keys
	keys := rm.Keys()
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Check that all expected keys are present
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}

	expectedKeys := []string{"key1", "key2", "key3"}
	for _, k := range expectedKeys {
		if !keyMap[k] {
			t.Errorf("Expected key %s to be present", k)
		}
	}
}
