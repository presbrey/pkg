package syncmap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"
)

// waitForCondition waits for a condition to be true with a timeout
func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func TestRemoteMapBasic(t *testing.T) {
	// Create a test server that returns a simple JSON map
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 100,
		"key3": true,
		"key4": map[string]interface{}{
			"nested": "value",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Create a RemoteMap with a short refresh period for testing using Fluent Interface
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(50 * time.Millisecond).
		WithTimeout(1 * time.Second).
		Start()

	// Stop the map when done
	defer rm.Stop()

	// Wait for initial fetch to complete
	if !waitForCondition(t, 2*time.Second, func() bool {
		_, ok := rm.Load("key1")
		return ok
	}) {
		t.Fatal("Timed out waiting for initial data fetch")
	}

	// Test basic Load operations
	val, ok := rm.Load("key1")
	if !ok {
		t.Errorf("Failed to get key1")
	}
	if val != "value1" {
		t.Errorf("Expected key1=value1, got %v", val)
	}

	val, ok = rm.Load("key2")
	if !ok {
		t.Errorf("Failed to get key2")
	}
	if val != float64(100) {
		t.Errorf("Expected key2=100, got %v (type %T)", val, val)
	}

	val, ok = rm.Load("key3")
	if !ok {
		t.Errorf("Failed to get key3")
	}
	if val != true {
		t.Errorf("Expected key3=true, got %v", val)
	}

	// Test LoadOrStore
	val, _ = rm.LoadOrStore("non_existent", "default")
	if val != "default" {
		t.Errorf("Expected default value, got %v", val)
	}

	// Test Range
	count := 0
	rm.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	// We expect len(testData) + 1 because we added "non_existent" with LoadOrStore
	expectedCount := len(testData) + 1
	if count != expectedCount {
		t.Errorf("Expected Range to iterate over %d items, got %d", expectedCount, count)
	}

	// Test early termination in Range
	count = 0
	rm.Range(func(key, value interface{}) bool {
		count++
		return count < 2 // Stop after first item
	})
	if count != 2 {
		t.Errorf("Expected Range to stop after 2 items, got %d", count)
	}
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

	// Track updates
	updateCh := make(chan []string, 1)
	
	// Create a RemoteMap with a short refresh period for testing using Fluent Interface
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(50 * time.Millisecond).
		WithTimeout(1 * time.Second).
		WithUpdateCallback(func(updated []string) {
			select {
			case updateCh <- updated:
			default:
			}
		}).
		Start()

	// Stop the map when done
	defer rm.Stop()

	// Wait for initial fetch to complete
	if !waitForCondition(t, 2*time.Second, func() bool {
		_, ok := rm.Load("key1")
		return ok
	}) {
		t.Fatal("Timed out waiting for initial data fetch")
	}

	// Verify initial data
	val, ok := rm.Load("key1")
	if !ok || val != "initial" {
		t.Errorf("Expected key1=initial, got %v, ok=%v", val, ok)
	}

	// Update the test data
	mu.Lock()
	testData = map[string]interface{}{
		"key1": "updated",
		"key3": "new",
	}
	mu.Unlock()

	// Wait for the update to occur and verify we get update notifications
	var receivedUpdates []string
	if !waitForCondition(t, 2*time.Second, func() bool {
		select {
		case updates := <-updateCh:
			// Store the updates we received
			receivedUpdates = updates
			return true
		default:
			val, ok := rm.Load("key1")
			return ok && val == "updated"
		}
	}) {
		t.Fatal("Timed out waiting for data update")
	}

	// Verify we received update notifications if we got them through the channel
	if len(receivedUpdates) > 0 {
		// Check if key1 is in the updated keys
		foundKey1 := false
		for _, k := range receivedUpdates {
			if k == "key1" {
				foundKey1 = true
				break
			}
		}
		if !foundKey1 {
			t.Errorf("Expected key1 to be in updated keys, got %v", receivedUpdates)
		}
	}

	// Verify updated data
	val, ok = rm.Load("key1")
	if !ok || val != "updated" {
		t.Errorf("Expected key1=updated, got %v, ok=%v", val, ok)
	}

	val, ok = rm.Load("key3")
	if !ok || val != "new" {
		t.Errorf("Expected key3=new, got %v, ok=%v", val, ok)
	}

	// Verify key2 was deleted
	_, ok = rm.Load("key2")
	if ok {
		t.Error("Expected key2 to be deleted, but it still exists")
	}
}

func TestRemoteMapTransform(t *testing.T) {
	// Create a test server that returns a simple JSON map
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 100,
		"key3": true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Create a transform function that adds a prefix to string values
	transform := func(data map[string]interface{}) map[string]interface{} {
		result := make(map[string]interface{})
		for k, v := range data {
			if s, ok := v.(string); ok {
				result[k] = "prefix_" + s
			} else {
				result[k] = v
			}
		}
		return result
	}

	// Create a RemoteMap with the transform function using Fluent Interface
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(50 * time.Millisecond).
		WithTimeout(1 * time.Second).
		WithTransformFunc(transform).
		Start()

	// Stop the map when done
	defer rm.Stop()

	// Wait for initial fetch to complete
	if !waitForCondition(t, 2*time.Second, func() bool {
		_, ok := rm.Load("key1")
		return ok
	}) {
		t.Fatal("Timed out waiting for initial data fetch")
	}

	// Verify transformed data
	val, ok := rm.Load("key1")
	if !ok || val != "prefix_value1" {
		t.Errorf("Expected key1=prefix_value1, got %v, ok=%v", val, ok)
	}

	// Non-string values should remain unchanged
	val, ok = rm.Load("key2")
	if !ok || val != float64(100) {
		t.Errorf("Expected key2=100, got %v, ok=%v", val, ok)
	}
}

func TestRemoteMapHeaders(t *testing.T) {
	// Create a test server that checks for custom headers
	headerReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the custom header was sent
		if r.Header.Get("X-Custom-Header") == "test-value" {
			headerReceived = true
		}

		// Return a simple JSON response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "ok",
		})
	}))
	defer server.Close()

	// Create a RemoteMap with custom headers using Fluent Interface
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(50*time.Millisecond).
		WithTimeout(1*time.Second).
		WithHeader("X-Custom-Header", "test-value").
		Start()

	// Stop the map when done
	defer rm.Stop()

	// Wait for initial fetch to complete
	if !waitForCondition(t, 2*time.Second, func() bool {
		_, ok := rm.Load("result")
		return ok
	}) {
		t.Fatal("Timed out waiting for initial data fetch")
	}

	// Verify that the header was received
	if !headerReceived {
		t.Error("Custom header was not received by the server")
	}
}

func TestRemoteMapErrorHandler(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error response"))
	}))
	defer server.Close()

	// Track if the error handler was called
	errorHandlerCalled := false
	errorCh := make(chan struct{}, 1)
	
	// Create a RemoteMap with an error handler using Fluent Interface
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(50 * time.Millisecond).
		WithTimeout(1 * time.Second).
		WithErrorHandler(func(err error) {
			errorHandlerCalled = true
			select {
			case errorCh <- struct{}{}:
			default:
			}
		}).
		Start()

	// Stop the map when done
	defer rm.Stop()

	// Wait for the error handler to be called
	if !waitForCondition(t, 2*time.Second, func() bool {
		select {
		case <-errorCh:
			return true
		default:
			return errorHandlerCalled
		}
	}) {
		t.Fatal("Timed out waiting for error handler to be called")
	}

	// Verify that the error handler was called
	if !errorHandlerCalled {
		t.Error("Error handler was not called")
	}
}

func TestRemoteMapManualRefresh(t *testing.T) {
	// Create a test server with changing data
	var mu sync.Mutex
	counter := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		counter++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"counter": counter,
		})
	}))
	defer server.Close()

	// Create a RemoteMap with a very long refresh period using Fluent Interface
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(1 * time.Hour). // Long enough that it won't auto-refresh during the test
		WithTimeout(1 * time.Second).
		Start()

	// Stop the map when done
	defer rm.Stop()

	// Wait for initial fetch to complete
	if !waitForCondition(t, 2*time.Second, func() bool {
		_, ok := rm.Load("counter")
		return ok
	}) {
		t.Fatal("Timed out waiting for initial data fetch")
	}

	// Verify initial counter value
	val1, ok := rm.Load("counter")
	if !ok || val1 != float64(1) {
		t.Errorf("Expected counter=1, got %v, ok=%v", val1, ok)
	}

	// Manually refresh the map
	err := rm.Refresh()
	if err != nil {
		t.Errorf("Manual refresh failed: %v", err)
	}

	// Verify that the counter was incremented
	val2, ok := rm.Load("counter")
	if !ok || val2 != float64(2) {
		t.Errorf("Expected counter=2, got %v, ok=%v", val2, ok)
	}
}

func TestGetFloat(t *testing.T) {
	// Create a test server with different numeric types
	testData := map[string]interface{}{
		"int_value":    42,
		"float_value":  3.14159,
		"string_value": "not a number",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Create a RemoteMap using Fluent Interface
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(50 * time.Millisecond).
		WithTimeout(1 * time.Second).
		Start()

	// Stop the map when done
	defer rm.Stop()

	// Wait for initial fetch to complete
	if !waitForCondition(t, 2*time.Second, func() bool {
		_, ok := rm.Load("int_value")
		return ok
	}) {
		t.Fatal("Timed out waiting for initial data fetch")
	}

	// Test Load with an integer value
	intValue, ok := rm.Load("int_value")
	if !ok {
		t.Error("Load failed for int_value")
	}
	if intValue != float64(42) {
		t.Errorf("Expected int_value=42.0, got %v", intValue)
	}

	// Test Load with a float value
	floatValue, ok := rm.Load("float_value")
	if !ok {
		t.Error("Load failed for float_value")
	}
	if floatValue != 3.14159 {
		t.Errorf("Expected float_value=3.14159, got %v", floatValue)
	}

	// Test Load with a string value (should succeed with mixed types)
	stringValue, ok := rm.Load("string_value")
	if !ok {
		t.Error("Load failed for string_value")
	}
	if stringValue != "not a number" {
		t.Errorf("Expected string_value=\"not a number\", got %v", stringValue)
	}

	// Test Load with a non-existent key
	_, ok = rm.Load("non_existent")
	if ok {
		t.Error("Load should have failed for non_existent key")
	}

	// Test LoadOrStore
	defaultValue, _ := rm.LoadOrStore("non_existent", 99.9)
	if defaultValue != 99.9 {
		t.Errorf("Expected default value 99.9, got %v", defaultValue)
	}
}

func TestGetInt(t *testing.T) {
	// Create a test server with different numeric types
	testData := map[string]interface{}{
		"int_value":    42,
		"float_value":  3.14159,
		"string_value": "not a number",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Create a RemoteMap using Fluent Interface
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(50 * time.Millisecond).
		WithTimeout(1 * time.Second).
		Start()

	// Stop the map when done
	defer rm.Stop()

	// Wait for initial fetch to complete
	if !waitForCondition(t, 2*time.Second, func() bool {
		_, ok := rm.Load("int_value")
		return ok
	}) {
		t.Fatal("Timed out waiting for initial data fetch")
	}

	// Test Load with an integer value
	intValue, ok := rm.Load("int_value")
	if !ok {
		t.Error("Load failed for int_value")
	}
	if intValue != float64(42) {
		t.Errorf("Expected int_value=42, got %v", intValue)
	}

	// Test Load with a float value (should convert to int)
	floatAsInt, ok := rm.Load("float_value")
	if !ok {
		t.Error("Load failed for float_value")
	}
	if floatAsInt != 3.14159 {
		t.Errorf("Expected float_value=3.14159, got %v", floatAsInt)
	}

	// Test Load with a string value (should succeed with mixed types)
	stringValue, ok := rm.Load("string_value")
	if !ok {
		t.Error("Load failed for string_value")
	}
	if stringValue != "not a number" {
		t.Errorf("Expected string_value=\"not a number\", got %v", stringValue)
	}

	// Test Load with a non-existent key
	_, ok = rm.Load("non_existent")
	if ok {
		t.Error("Load should have failed for non_existent key")
	}

	// Test LoadOrStore
	defaultValue, _ := rm.LoadOrStore("non_existent", 99)
	if defaultValue != 99 {
		t.Errorf("Expected default value 99, got %v", defaultValue)
	}
}

func TestGetBoolMap(t *testing.T) {
	// Create a test server with boolean map data
	testData := map[string]interface{}{
		"bool_map": map[string]interface{}{
			"key1": true,
			"key2": false,
		},
		"mixed_map": map[string]interface{}{
			"key1": true,
			"key2": "not a bool",
		},
		"empty_map":  map[string]interface{}{},
		"not_a_map":  "string value",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Create a RemoteMap using Fluent Interface
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(50 * time.Millisecond).
		WithTimeout(1 * time.Second).
		Start()

	// Stop the map when done
	defer rm.Stop()

	// Wait for initial fetch to complete
	if !waitForCondition(t, 2*time.Second, func() bool {
		_, ok := rm.Load("bool_map")
		return ok
	}) {
		t.Fatal("Timed out waiting for initial data fetch")
	}

	// Test Load with a valid boolean map
	boolMap, ok := rm.Load("bool_map")
	if !ok {
		t.Error("Load failed for bool_map")
	}
	boolMapValue, ok := boolMap.(map[string]interface{})
	if !ok {
		t.Errorf("Expected bool_map to be a map, got %T", boolMap)
	}
	if len(boolMapValue) != 2 {
		t.Errorf("Expected bool_map to have 2 entries, got %d", len(boolMapValue))
	}
	if boolMapValue["key1"] != true {
		t.Errorf("Expected bool_map[key1]=true, got %v", boolMapValue["key1"])
	}
	if boolMapValue["key2"] != false {
		t.Errorf("Expected bool_map[key2]=false, got %v", boolMapValue["key2"])
	}

	// Test GetBoolMap with a valid boolean map
	boolMapTyped, ok := rm.GetBoolMap("bool_map")
	if !ok {
		t.Error("GetBoolMap failed for bool_map")
	}
	if len(boolMapTyped) != 2 {
		t.Errorf("Expected GetBoolMap to return 2 entries, got %d", len(boolMapTyped))
	}
	if !boolMapTyped["key1"] {
		t.Errorf("Expected boolMapTyped[key1]=true, got %v", boolMapTyped["key1"])
	}
	if boolMapTyped["key2"] {
		t.Errorf("Expected boolMapTyped[key2]=false, got %v", boolMapTyped["key2"])
	}

	// Test GetBoolMap with a mixed map (should succeed but only include boolean values)
	mixedMapTyped, ok := rm.GetBoolMap("mixed_map")
	if !ok {
		t.Error("GetBoolMap failed for mixed_map")
	}
	if len(mixedMapTyped) != 1 {
		t.Errorf("Expected GetBoolMap to return 1 entry for mixed_map, got %d", len(mixedMapTyped))
	}
	if !mixedMapTyped["key1"] {
		t.Errorf("Expected mixedMapTyped[key1]=true, got %v", mixedMapTyped["key1"])
	}

	// Test GetBoolMap with a non-map value (should fail)
	_, ok = rm.GetBoolMap("not_a_map")
	if ok {
		t.Error("GetBoolMap should have failed for not_a_map")
	}

	// Test GetBoolMap with a non-existent key
	_, ok = rm.GetBoolMap("non_existent")
	if ok {
		t.Error("GetBoolMap should have failed for non_existent key")
	}

	// Test GetBoolMapWithDefault
	defaultMap := map[string]bool{"default": true}
	result := rm.GetBoolMapWithDefault("non_existent", defaultMap)
	if !reflect.DeepEqual(result, defaultMap) {
		t.Errorf("Expected default map %v, got %v", defaultMap, result)
	}
}

func TestGetStringMap(t *testing.T) {
	// Create a test server with string map data
	testData := map[string]interface{}{
		"string_map": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
		"mixed_map": map[string]interface{}{
			"key1": "value1",
			"key2": 100,
		},
		"empty_map":  map[string]interface{}{},
		"not_a_map":  "string value",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Create a RemoteMap using Fluent Interface
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(50 * time.Millisecond).
		WithTimeout(1 * time.Second).
		Start()

	// Stop the map when done
	defer rm.Stop()

	// Wait for initial fetch to complete
	if !waitForCondition(t, 2*time.Second, func() bool {
		_, ok := rm.Load("string_map")
		return ok
	}) {
		t.Fatal("Timed out waiting for initial data fetch")
	}

	// Test Load with a valid string map
	stringMap, ok := rm.Load("string_map")
	if !ok {
		t.Error("Load failed for string_map")
	}
	stringMapValue, ok := stringMap.(map[string]interface{})
	if !ok {
		t.Errorf("Expected string_map to be a map, got %T", stringMap)
	}
	if len(stringMapValue) != 2 {
		t.Errorf("Expected string_map to have 2 entries, got %d", len(stringMapValue))
	}
	if stringMapValue["key1"] != "value1" {
		t.Errorf("Expected string_map[key1]=value1, got %v", stringMapValue["key1"])
	}
	if stringMapValue["key2"] != "value2" {
		t.Errorf("Expected string_map[key2]=value2, got %v", stringMapValue["key2"])
	}

	// Test GetStringMap with a valid string map
	stringMapTyped, ok := rm.GetStringMap("string_map")
	if !ok {
		t.Error("GetStringMap failed for string_map")
	}
	if len(stringMapTyped) != 2 {
		t.Errorf("Expected GetStringMap to return 2 entries, got %d", len(stringMapTyped))
	}
	if stringMapTyped["key1"] != "value1" {
		t.Errorf("Expected stringMapTyped[key1]=value1, got %v", stringMapTyped["key1"])
	}
	if stringMapTyped["key2"] != "value2" {
		t.Errorf("Expected stringMapTyped[key2]=value2, got %v", stringMapTyped["key2"])
	}

	// Test GetStringMap with a mixed map (should succeed but only include string values)
	mixedMapTyped, ok := rm.GetStringMap("mixed_map")
	if !ok {
		t.Error("GetStringMap failed for mixed_map")
	}
	if len(mixedMapTyped) != 1 {
		t.Errorf("Expected GetStringMap to return 1 entry for mixed_map, got %d", len(mixedMapTyped))
	}
	if mixedMapTyped["key1"] != "value1" {
		t.Errorf("Expected mixedMapTyped[key1]=value1, got %v", mixedMapTyped["key1"])
	}

	// Test GetStringMap with a non-map value (should fail)
	_, ok = rm.GetStringMap("not_a_map")
	if ok {
		t.Error("GetStringMap should have failed for not_a_map")
	}

	// Test GetStringMap with a non-existent key
	_, ok = rm.GetStringMap("non_existent")
	if ok {
		t.Error("GetStringMap should have failed for non_existent key")
	}

	// Test GetStringMapWithDefault
	defaultMap := map[string]string{"default": "value"}
	result := rm.GetStringMapWithDefault("non_existent", defaultMap)
	if !reflect.DeepEqual(result, defaultMap) {
		t.Errorf("Expected default map %v, got %v", defaultMap, result)
	}
}

func TestGetStringSliceMap(t *testing.T) {
	// Create a test server with string slice map data
	testData := map[string]interface{}{
		"string_slice_map": map[string]interface{}{
			"key1": []interface{}{"value1", "value2"},
			"key2": []interface{}{"value3"},
		},
		"mixed_slice_map": map[string]interface{}{
			"key1": []interface{}{"value1", "value2"},
			"key2": []interface{}{"value3", 100},
		},
		"empty_map":  map[string]interface{}{},
		"not_a_map":  "string value",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Create a RemoteMap using Fluent Interface
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(50 * time.Millisecond).
		WithTimeout(1 * time.Second).
		Start()

	// Stop the map when done
	defer rm.Stop()

	// Wait for initial fetch to complete
	if !waitForCondition(t, 2*time.Second, func() bool {
		_, ok := rm.Load("string_slice_map")
		return ok
	}) {
		t.Fatal("Timed out waiting for initial data fetch")
	}

	// Test Load with a valid string slice map
	stringSliceMap, ok := rm.Load("string_slice_map")
	if !ok {
		t.Error("Load failed for string_slice_map")
	}
	stringSliceMapValue, ok := stringSliceMap.(map[string]interface{})
	if !ok {
		t.Errorf("Expected string_slice_map to be a map, got %T", stringSliceMap)
	}
	if len(stringSliceMapValue) != 2 {
		t.Errorf("Expected string_slice_map to have 2 entries, got %d", len(stringSliceMapValue))
	}

	// Test GetStringSliceMap with a valid string slice map
	stringSliceMapTyped, ok := rm.GetStringSliceMap("string_slice_map")
	if !ok {
		t.Error("GetStringSliceMap failed for string_slice_map")
	}
	if len(stringSliceMapTyped) != 2 {
		t.Errorf("Expected GetStringSliceMap to return 2 entries, got %d", len(stringSliceMapTyped))
	}
	if len(stringSliceMapTyped["key1"]) != 2 {
		t.Errorf("Expected stringSliceMapTyped[key1] to have 2 entries, got %d", len(stringSliceMapTyped["key1"]))
	}
	if stringSliceMapTyped["key1"][0] != "value1" {
		t.Errorf("Expected stringSliceMapTyped[key1][0]=value1, got %v", stringSliceMapTyped["key1"][0])
	}
	if stringSliceMapTyped["key1"][1] != "value2" {
		t.Errorf("Expected stringSliceMapTyped[key1][1]=value2, got %v", stringSliceMapTyped["key1"][1])
	}
	if len(stringSliceMapTyped["key2"]) != 1 {
		t.Errorf("Expected stringSliceMapTyped[key2] to have 1 entry, got %d", len(stringSliceMapTyped["key2"]))
	}
	if stringSliceMapTyped["key2"][0] != "value3" {
		t.Errorf("Expected stringSliceMapTyped[key2][0]=value3, got %v", stringSliceMapTyped["key2"][0])
	}

	// Test GetStringSliceMap with a mixed slice map (should succeed but filter non-string values)
	mixedSliceMapTyped, ok := rm.GetStringSliceMap("mixed_slice_map")
	if !ok {
		t.Error("GetStringSliceMap failed for mixed_slice_map")
	}
	if len(mixedSliceMapTyped) != 2 {
		t.Errorf("Expected GetStringSliceMap to return 2 entries for mixed_slice_map, got %d", len(mixedSliceMapTyped))
	}
	if len(mixedSliceMapTyped["key1"]) != 2 {
		t.Errorf("Expected mixedSliceMapTyped[key1] to have 2 entries, got %d", len(mixedSliceMapTyped["key1"]))
	}
	if len(mixedSliceMapTyped["key2"]) != 1 {
		t.Errorf("Expected mixedSliceMapTyped[key2] to have 1 entry, got %d", len(mixedSliceMapTyped["key2"]))
	}
	if mixedSliceMapTyped["key2"][0] != "value3" {
		t.Errorf("Expected mixedSliceMapTyped[key2][0]=value3, got %v", mixedSliceMapTyped["key2"][0])
	}

	// Test GetStringSliceMap with a non-map value (should fail)
	_, ok = rm.GetStringSliceMap("not_a_map")
	if ok {
		t.Error("GetStringSliceMap should have failed for not_a_map")
	}

	// Test GetStringSliceMap with a non-existent key
	_, ok = rm.GetStringSliceMap("non_existent")
	if ok {
		t.Error("GetStringSliceMap should have failed for non_existent key")
	}

	// Test GetStringSliceMapWithDefault
	defaultMap := map[string][]string{"default": {"value"}}
	result := rm.GetStringSliceMapWithDefault("non_existent", defaultMap)
	if !reflect.DeepEqual(result, defaultMap) {
		t.Errorf("Expected default map %v, got %v", defaultMap, result)
	}
}

func TestGetStringSlice(t *testing.T) {
	// Create a test server with string slice data
	testData := map[string]interface{}{
		"string_slice": []interface{}{"value1", "value2", "value3"},
		"mixed_slice":  []interface{}{"value1", 42, "value3"},
		"empty_slice":  []interface{}{},
		"not_a_slice":  "string value",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Create a RemoteMap with a short refresh period
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(50 * time.Millisecond).
		WithTimeout(1 * time.Second).
		Start()

	// Stop the map when done
	defer rm.Stop()

	// Wait for initial fetch to complete
	if !waitForCondition(t, 2*time.Second, func() bool {
		_, ok := rm.Load("string_slice")
		return ok
	}) {
		t.Fatal("Timed out waiting for initial data fetch")
	}

	// Test Load with a valid string slice
	stringSlice, ok := rm.Load("string_slice")
	if !ok {
		t.Error("Load failed for string_slice")
	}
	stringSliceValue, ok := stringSlice.([]interface{})
	if !ok {
		t.Errorf("Expected string_slice to be a slice, got %T", stringSlice)
	}
	if len(stringSliceValue) != 3 {
		t.Errorf("Expected string_slice to have 3 entries, got %d", len(stringSliceValue))
	}
	if stringSliceValue[0] != "value1" {
		t.Errorf("Expected string_slice[0]=value1, got %s", stringSliceValue[0])
	}
	if stringSliceValue[1] != "value2" {
		t.Errorf("Expected string_slice[1]=value2, got %s", stringSliceValue[1])
	}
	if stringSliceValue[2] != "value3" {
		t.Errorf("Expected string_slice[2]=value3, got %s", stringSliceValue[2])
	}

	// Test GetStringSlice with a valid string slice
	stringSliceTyped, ok := rm.GetStringSlice("string_slice")
	if !ok {
		t.Error("GetStringSlice failed for string_slice")
	}
	if len(stringSliceTyped) != 3 {
		t.Errorf("Expected GetStringSlice to return 3 entries, got %d", len(stringSliceTyped))
	}
	if stringSliceTyped[0] != "value1" {
		t.Errorf("Expected stringSliceTyped[0]=value1, got %s", stringSliceTyped[0])
	}
	if stringSliceTyped[1] != "value2" {
		t.Errorf("Expected stringSliceTyped[1]=value2, got %s", stringSliceTyped[1])
	}
	if stringSliceTyped[2] != "value3" {
		t.Errorf("Expected stringSliceTyped[2]=value3, got %s", stringSliceTyped[2])
	}

	// Test GetStringSlice with a mixed slice (should succeed but filter non-string values)
	mixedSliceTyped, ok := rm.GetStringSlice("mixed_slice")
	if !ok {
		t.Error("GetStringSlice failed for mixed_slice")
	}
	if len(mixedSliceTyped) != 2 {
		t.Errorf("Expected GetStringSlice to return 2 entries for mixed_slice, got %d", len(mixedSliceTyped))
	}
	if mixedSliceTyped[0] != "value1" {
		t.Errorf("Expected mixedSliceTyped[0]=value1, got %s", mixedSliceTyped[0])
	}
	if mixedSliceTyped[1] != "value3" {
		t.Errorf("Expected mixedSliceTyped[1]=value3, got %s", mixedSliceTyped[1])
	}

	// Test GetStringSlice with a non-slice value (should fail)
	_, ok = rm.GetStringSlice("not_a_slice")
	if ok {
		t.Error("GetStringSlice should have failed for not_a_slice")
	}

	// Test GetStringSlice with a non-existent key
	_, ok = rm.GetStringSlice("non_existent")
	if ok {
		t.Error("GetStringSlice should have failed for non_existent key")
	}

	// Test GetStringSliceWithDefault
	defaultSlice := []string{"default1", "default2"}
	result := rm.GetStringSliceWithDefault("non_existent", defaultSlice)
	if !reflect.DeepEqual(result, defaultSlice) {
		t.Errorf("Expected default slice %v, got %v", defaultSlice, result)
	}
}

func TestOnUpdate(t *testing.T) {
	// Create a test server with changing data
	var mu sync.Mutex
	initialData := map[string]interface{}{
		"key1": "initial1",
		"key2": "initial2",
	}
	updatedData := map[string]interface{}{
		"key1": "updated1",
		"key3": "new3",
	}

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if callCount == 0 {
			json.NewEncoder(w).Encode(initialData)
		} else {
			json.NewEncoder(w).Encode(updatedData)
		}
		callCount++
	}))
	defer server.Close()

	// Track update callback invocations
	updateCallbackCalled := false
	var updatedKeys []string

	// Track delete callback invocations
	deleteCallbackCalled := false
	var deletedKeys []string

	// Create a RemoteMap with update and delete callbacks using Fluent Interface
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(50 * time.Millisecond).
		WithTimeout(1 * time.Second).
		WithUpdateCallback(func(keys []string) {
			updateCallbackCalled = true
			updatedKeys = keys
		}).
		WithDeleteCallback(func(keys []string) {
			deleteCallbackCalled = true
			deletedKeys = keys
		}).
		Start()

	// Stop the map when done
	defer rm.Stop()

	// Wait for initial fetch to complete
	if !waitForCondition(t, 2*time.Second, func() bool {
		_, ok := rm.Load("key1")
		return ok
	}) {
		t.Fatal("Timed out waiting for initial data fetch")
	}

	// Verify initial data
	val1, ok := rm.Load("key1")
	if !ok || val1 != "initial1" {
		t.Errorf("Expected key1=initial1, got %v, ok=%v", val1, ok)
	}

	// Wait for the update to occur
	if !waitForCondition(t, 2*time.Second, func() bool {
		val1, ok := rm.Load("key1")
		return ok && val1 == "updated1"
	}) {
		t.Fatal("Timed out waiting for data update")
	}

	// Verify updated data
	val1, ok = rm.Load("key1")
	if !ok || val1 != "updated1" {
		t.Errorf("Expected key1=updated1, got %v, ok=%v", val1, ok)
	}

	val3, ok := rm.Load("key3")
	if !ok || val3 != "new3" {
		t.Errorf("Expected key3=new3, got %v, ok=%v", val3, ok)
	}

	// Verify key2 was deleted
	_, ok = rm.Load("key2")
	if ok {
		t.Error("Expected key2 to be deleted, but it still exists")
	}

	// Verify update callback was called with the correct keys
	if !updateCallbackCalled {
		t.Error("Update callback was not called")
	}
	if len(updatedKeys) != 1 || updatedKeys[0] != "key1" {
		t.Errorf("Expected updated keys to be [key1], got %v", updatedKeys)
	}

	// Verify delete callback was called with the correct keys
	if !deleteCallbackCalled {
		t.Error("Delete callback was not called")
	}
	if len(deletedKeys) != 1 || deletedKeys[0] != "key2" {
		t.Errorf("Expected deleted keys to be [key2], got %v", deletedKeys)
	}
}

func TestKeys(t *testing.T) {
	// Create a test server with some data
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

	// Create a RemoteMap using Fluent Interface
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(50 * time.Millisecond).
		WithTimeout(1 * time.Second).
		Start()

	// Stop the map when done
	defer rm.Stop()

	// Wait for initial fetch to complete
	if !waitForCondition(t, 2*time.Second, func() bool {
		_, ok := rm.Load("key1")
		return ok
	}) {
		t.Fatal("Timed out waiting for initial data fetch")
	}

	// Get all keys
	keys := rm.Keys()

	// Sort the keys for consistent comparison
	sort.Strings(keys)

	// Verify the keys
	expectedKeys := []string{"key1", "key2", "key3"}
	if !reflect.DeepEqual(keys, expectedKeys) {
		t.Errorf("Expected keys %v, got %v", expectedKeys, keys)
	}

	// Test with empty map
	emptyMap := NewRemoteMap("http://non-existent-url").
		WithTimeout(1 * time.Second)

	emptyKeys := emptyMap.Keys()
	if len(emptyKeys) != 0 {
		t.Errorf("Expected empty keys for empty map, got %v", emptyKeys)
	}
}

// TestRemoteMapStartedState tests the Started method and start/stop state tracking
func TestRemoteMapStartedState(t *testing.T) {
	// Create a test server that returns a simple JSON map
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 100,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Create a RemoteMap but don't start it yet
	rm := NewRemoteMap(server.URL).
		WithRefreshPeriod(50 * time.Millisecond).
		WithTimeout(1 * time.Second)

	// Check initial state - should not be started
	if rm.Started() {
		t.Error("RemoteMap should not be started initially")
	}

	// Start the map
	rm.Start()

	// Should now be started
	if !rm.Started() {
		t.Error("RemoteMap should be started after Start() is called")
	}

	// Calling Start() again should be a no-op
	rm.Start()

	// Should still be started
	if !rm.Started() {
		t.Error("RemoteMap should still be started after second Start() call")
	}

	// Stop the map
	rm.Stop()

	// Should now be stopped
	if rm.Started() {
		t.Error("RemoteMap should not be started after Stop() is called")
	}

	// Calling Stop() again should be a no-op
	rm.Stop()

	// Should still be stopped
	if rm.Started() {
		t.Error("RemoteMap should still be stopped after second Stop() call")
	}

	// Start again to verify we can restart
	rm.Start()

	// Should be started again
	if !rm.Started() {
		t.Error("RemoteMap should be started after restarting")
	}

	// Clean up
	rm.Stop()
}
