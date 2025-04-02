package syncmap

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

	// Create a RemoteMap with a short refresh period for testing
	options := &Options{
		RefreshPeriod: 100 * time.Millisecond,
		Timeout:       5 * time.Second,
	}
	rm := NewRemoteMap(server.URL, options)

	// Start the map and wait for the first refresh
	rm.Start()
	defer rm.Stop()

	// Wait for initial fetch to complete
	time.Sleep(200 * time.Millisecond)

	// Test that the data was loaded correctly
	val1, ok := rm.GetString("key1")
	if !ok || val1 != "value1" {
		t.Errorf("Expected key1=value1, got %v, ok=%v", val1, ok)
	}

	val2, ok := rm.GetInt("key2")
	if !ok || val2 != 42 {
		t.Errorf("Expected key2=42, got %v, ok=%v", val2, ok)
	}

	val3, ok := rm.GetBool("key3")
	if !ok || val3 != true {
		t.Errorf("Expected key3=true, got %v, ok=%v", val3, ok)
	}

	val4, ok := rm.GetMap("key4")
	if !ok || val4["nested"] != "value" {
		t.Errorf("Expected key4.nested=value, got %v, ok=%v", val4, ok)
	}

	// Test the new GetInt64 method
	val5, ok := rm.GetInt64("key5")
	expectedVal := int64(1234567890123456)
	if !ok || val5 != expectedVal {
		t.Errorf("Expected key5=%v, got %v, ok=%v", expectedVal, val5, ok)
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

	// Create a RemoteMap with a short refresh period
	options := &Options{
		RefreshPeriod: 100 * time.Millisecond,
		Timeout:       5 * time.Second,
	}
	rm := NewRemoteMap(server.URL, options)

	// Start the map and wait for the first refresh
	rm.Start()
	defer rm.Stop()

	// Wait for initial fetch to complete
	time.Sleep(200 * time.Millisecond)

	// Verify initial data
	val1, ok := rm.GetString("key1")
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
	val1, ok = rm.GetString("key1")
	if !ok || val1 != "updated" {
		t.Errorf("Expected key1=updated, got %v, ok=%v", val1, ok)
	}

	val3, ok := rm.GetString("key3")
	if !ok || val3 != "new" {
		t.Errorf("Expected key3=new, got %v, ok=%v", val3, ok)
	}

	// key2 should be gone
	_, ok = rm.Load("key2")
	if ok {
		t.Errorf("Expected key2 to be removed")
	}
}

func TestRemoteMapTransform(t *testing.T) {
	// Create a test server
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Create a transform function that adds a prefix to string values
	transformFunc := func(data map[string]interface{}) map[string]interface{} {
		result := make(map[string]interface{})
		for k, v := range data {
			if str, ok := v.(string); ok {
				result[k] = "prefix_" + str
			} else {
				result[k] = v
			}
		}
		return result
	}

	// Create a RemoteMap with the transform function
	options := &Options{
		RefreshPeriod: 100 * time.Millisecond,
		TransformFunc: transformFunc,
	}
	rm := NewRemoteMap(server.URL, options)

	// Start the map and wait for the first refresh
	rm.Start()
	defer rm.Stop()

	// Wait for initial fetch to complete
	time.Sleep(200 * time.Millisecond)

	// Test that the data was transformed correctly
	val1, ok := rm.GetString("key1")
	if !ok || val1 != "prefix_value1" {
		t.Errorf("Expected key1=prefix_value1, got %v, ok=%v", val1, ok)
	}

	val2, ok := rm.GetInt("key2")
	if !ok || val2 != 42 {
		t.Errorf("Expected key2=42, got %v, ok=%v", val2, ok)
	}
}

func TestRemoteMapHeaders(t *testing.T) {
	// Create a test server that checks for custom headers
	headerChecked := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for the custom header
		if r.Header.Get("X-Custom-Header") == "test-value" {
			headerChecked = true
		}

		// Return a simple response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})
	}))
	defer server.Close()

	// Create a RemoteMap with custom headers
	headers := map[string]string{
		"X-Custom-Header": "test-value",
	}

	options := &Options{
		RefreshPeriod: 100 * time.Millisecond,
		Headers:       headers,
	}
	rm := NewRemoteMap(server.URL, options)

	// Start the map and wait for the refresh
	rm.Start()
	defer rm.Stop()

	// Wait for fetch to complete
	time.Sleep(200 * time.Millisecond)

	// Verify the header was sent
	if !headerChecked {
		t.Errorf("Custom header was not sent or not received by the server")
	}
}

func TestRemoteMapErrorHandler(t *testing.T) {
	// Create a server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Track if the error handler was called
	errorCalled := false
	errorHandler := func(err error) {
		errorCalled = true
	}

	// Create a RemoteMap with the error handler
	options := &Options{
		RefreshPeriod: 100 * time.Millisecond,
		ErrorHandler:  errorHandler,
	}
	rm := NewRemoteMap(server.URL, options)

	// Start the map and wait for the refresh
	rm.Start()
	defer rm.Stop()

	// Wait for fetch to complete
	time.Sleep(200 * time.Millisecond)

	// Verify the error handler was called
	if !errorCalled {
		t.Errorf("Error handler was not called")
	}
}

func TestRemoteMapManualRefresh(t *testing.T) {
	// Create a test server with changing data
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		data := map[string]interface{}{
			"count": callCount,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	// Create a RemoteMap with a long refresh period
	options := &Options{
		RefreshPeriod: 1 * time.Hour, // Long period so auto-refresh doesn't interfere
	}
	rm := NewRemoteMap(server.URL, options)

	// Don't start the automatic refresh
	// Just call Refresh manually
	err := rm.Refresh()
	if err != nil {
		t.Errorf("Manual refresh failed: %v", err)
	}

	// Check the data
	count, ok := rm.GetInt("count")
	if !ok || count != 1 {
		t.Errorf("Expected count=1, got %v, ok=%v", count, ok)
	}

	// Refresh again
	err = rm.Refresh()
	if err != nil {
		t.Errorf("Second manual refresh failed: %v", err)
	}

	// Check the updated data
	count, ok = rm.GetInt("count")
	if !ok || count != 2 {
		t.Errorf("Expected count=2, got %v, ok=%v", count, ok)
	}
}

func TestGetFloat(t *testing.T) {
	// Create a test server with different numeric types
	testData := map[string]interface{}{
		"float_value":  3.14159,
		"int_value":    42,
		"int64_value":  int64(9876543210),
		"string_value": "not a number",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Create a RemoteMap
	options := &Options{
		RefreshPeriod: 100 * time.Millisecond,
	}
	rm := NewRemoteMap(server.URL, options)

	// Start the map and wait for the first refresh
	rm.Start()
	defer rm.Stop()

	// Wait for initial fetch to complete
	time.Sleep(200 * time.Millisecond)

	// Test GetFloat with float value
	floatVal, ok := rm.GetFloat("float_value")
	if !ok || floatVal != 3.14159 {
		t.Errorf("Expected float_value=3.14159, got %v, ok=%v", floatVal, ok)
	}

	// Test GetFloat with int value
	intAsFloat, ok := rm.GetFloat("int_value")
	if !ok || intAsFloat != 42.0 {
		t.Errorf("Expected int_value as float=42.0, got %v, ok=%v", intAsFloat, ok)
	}

	// Test GetFloat with int64 value
	int64AsFloat, ok := rm.GetFloat("int64_value")
	if !ok || int64AsFloat != 9876543210.0 {
		t.Errorf("Expected int64_value as float=9876543210.0, got %v, ok=%v", int64AsFloat, ok)
	}

	// Test GetFloat with non-numeric value
	_, ok = rm.GetFloat("string_value")
	if ok {
		t.Errorf("Expected GetFloat to fail with string value, but it succeeded")
	}

	// Test GetFloat with non-existent key
	_, ok = rm.GetFloat("non_existent_key")
	if ok {
		t.Errorf("Expected GetFloat to fail with non-existent key, but it succeeded")
	}
}

func TestGetInt(t *testing.T) {
	// Create a test server with different numeric types
	testData := map[string]interface{}{
		"int_value":         42,
		"float_value":       3.14159,
		"float_int_value":   100.0, // Float that is actually an integer
		"int64_value":       int64(9876543210),
		"int64_small_value": int64(123), // Small enough to fit in an int
		"string_value":      "not a number",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	// Create a RemoteMap
	options := &Options{
		RefreshPeriod: 100 * time.Millisecond,
	}
	rm := NewRemoteMap(server.URL, options)

	// Start the map and wait for the first refresh
	rm.Start()
	defer rm.Stop()

	// Wait for initial fetch to complete
	time.Sleep(200 * time.Millisecond)

	// Test GetInt with int value
	intVal, ok := rm.GetInt("int_value")
	if !ok || intVal != 42 {
		t.Errorf("Expected int_value=42, got %v, ok=%v", intVal, ok)
	}

	// Test GetInt with float value that's an integer
	floatIntVal, ok := rm.GetInt("float_int_value")
	if !ok || floatIntVal != 100 {
		t.Errorf("Expected float_int_value as int=100, got %v, ok=%v", floatIntVal, ok)
	}

	// Test GetInt with float value that's not an integer
	// Should still work but will be truncated
	floatVal, ok := rm.GetInt("float_value")
	if !ok || floatVal != 3 {
		t.Errorf("Expected float_value as int=3, got %v, ok=%v", floatVal, ok)
	}

	// Test GetInt with small int64 value that fits in int
	smallInt64Val, ok := rm.GetInt("int64_small_value")
	if !ok || smallInt64Val != 123 {
		t.Errorf("Expected int64_small_value as int=123, got %v, ok=%v", smallInt64Val, ok)
	}

	// Test GetInt with large int64 value
	// This might overflow on 32-bit platforms, but we'll test it anyway
	_, ok = rm.GetInt("int64_value")
	if !ok {
		t.Errorf("Expected GetInt to succeed with large int64 value, but it failed")
	}
	// We don't check the exact value as it might be platform-dependent

	// Test GetInt with non-numeric value
	_, ok = rm.GetInt("string_value")
	if ok {
		t.Errorf("Expected GetInt to fail with string value, but it succeeded")
	}

	// Test GetInt with non-existent key
	_, ok = rm.GetInt("non_existent_key")
	if ok {
		t.Errorf("Expected GetInt to fail with non-existent key, but it succeeded")
	}
}
