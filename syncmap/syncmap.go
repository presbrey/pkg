package syncmap

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sync"
	"time"
)

// DefaultRefreshPeriod is the default time between refreshes of the remote data
const DefaultRefreshPeriod = 5 * time.Minute

// DefaultTimeout is the default timeout for HTTP requests
const DefaultTimeout = 30 * time.Second

// RemoteMap extends sync.Map to synchronize with a remote JSON endpoint
type RemoteMap struct {
	sync.Map
	url             string
	refreshPeriod   time.Duration
	timeout         time.Duration
	ignoreTLSVerify bool
	headers         map[string]string
	errorHandler    func(error)
	updateCallback  func([]string)
	deleteCallback  func([]string)
	refreshCallback func()
	transformFunc   func(map[string]interface{}) map[string]interface{}
	httpClient      *http.Client
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	started         bool
	mu              sync.Mutex
}

// NewRemoteMap creates a new RemoteMap that synchronizes with the provided URL
func NewRemoteMap(url string) *RemoteMap {
	rm := &RemoteMap{
		url:             url,
		refreshPeriod:   DefaultRefreshPeriod,
		timeout:         DefaultTimeout,
		ignoreTLSVerify: false,
		headers:         make(map[string]string),
	}

	// Initialize HTTP client with default settings
	rm.initHTTPClient()

	return rm
}

// initHTTPClient initializes the HTTP client with current settings
func (rm *RemoteMap) initHTTPClient() {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: rm.ignoreTLSVerify}

	rm.httpClient = &http.Client{
		Timeout:   rm.timeout,
		Transport: transport,
	}
}

// WithRefreshPeriod sets the time between refreshes of the remote data
func (rm *RemoteMap) WithRefreshPeriod(period time.Duration) *RemoteMap {
	if period > 0 {
		rm.refreshPeriod = period
	}
	return rm
}

// WithTimeout sets the timeout for HTTP requests
func (rm *RemoteMap) WithTimeout(timeout time.Duration) *RemoteMap {
	if timeout > 0 {
		rm.timeout = timeout
		rm.initHTTPClient() // Reinitialize HTTP client with new timeout
	}
	return rm
}

// WithIgnoreTLSVerify sets whether to disable TLS certificate verification
func (rm *RemoteMap) WithIgnoreTLSVerify(ignore bool) *RemoteMap {
	rm.ignoreTLSVerify = ignore
	rm.initHTTPClient() // Reinitialize HTTP client with new TLS settings
	return rm
}

// WithHeader adds an HTTP header to include in requests
func (rm *RemoteMap) WithHeader(key, value string) *RemoteMap {
	rm.headers[key] = value
	return rm
}

// WithHeaders sets all HTTP headers to include in requests
func (rm *RemoteMap) WithHeaders(headers map[string]string) *RemoteMap {
	rm.headers = headers
	return rm
}

// WithErrorHandler sets a function to be called when an error occurs during refresh
func (rm *RemoteMap) WithErrorHandler(handler func(error)) *RemoteMap {
	rm.errorHandler = handler
	return rm
}

// WithUpdateCallback sets a function to be called when keys are updated in the map
func (rm *RemoteMap) WithUpdateCallback(callback func([]string)) *RemoteMap {
	rm.updateCallback = callback
	return rm
}

// WithDeleteCallback sets a function to be called when keys are deleted from the map
func (rm *RemoteMap) WithDeleteCallback(callback func([]string)) *RemoteMap {
	rm.deleteCallback = callback
	return rm
}

// WithRefreshCallback sets a function to be called after each refresh operation
func (rm *RemoteMap) WithRefreshCallback(callback func()) *RemoteMap {
	rm.refreshCallback = callback
	return rm
}

// WithTransformFunc sets a function to transform the fetched data before storing
func (rm *RemoteMap) WithTransformFunc(transform func(map[string]interface{}) map[string]interface{}) *RemoteMap {
	rm.transformFunc = transform
	return rm
}

// Start begins the periodic refresh of the map from the remote URL and returns the RemoteMap for chaining
func (rm *RemoteMap) Start() *RemoteMap {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	// Don't start if already running
	if rm.started {
		return rm
	}
	
	// Immediately fetch data once
	if err := rm.Refresh(); err != nil && rm.errorHandler != nil {
		rm.errorHandler(err)
	}

	// Set up periodic refresh
	ctx, cancel := context.WithCancel(context.Background())
	rm.cancel = cancel

	rm.wg.Add(1)
	go func() {
		defer rm.wg.Done()
		ticker := time.NewTicker(rm.refreshPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := rm.Refresh(); err != nil && rm.errorHandler != nil {
					rm.errorHandler(err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	rm.started = true
	return rm
}

// Stop halts the periodic refresh of the map and returns the RemoteMap for chaining
func (rm *RemoteMap) Stop() *RemoteMap {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	if !rm.started {
		return rm
	}
	
	if rm.cancel != nil {
		rm.cancel()
		rm.wg.Wait()
		rm.cancel = nil
	}
	
	rm.started = false
	return rm
}

// Started returns whether the RemoteMap is currently running
func (rm *RemoteMap) Started() bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.started
}

// Refresh immediately updates the map from the remote URL and returns any error
func (rm *RemoteMap) Refresh() error {
	data, err := rm.fetchData()
	if err != nil {
		return err
	}

	// Apply transform function if provided
	if rm.transformFunc != nil {
		data = rm.transformFunc(data)
	}

	// Update the map with the new data and track changes
	_, updated, deleted := rm.updateMap(data)

	// Call the update callback if set and if there are changes
	if rm.updateCallback != nil && len(updated) > 0 {
		rm.updateCallback(updated)
	}

	// Call the delete callback if set and if there are deletions
	if rm.deleteCallback != nil && len(deleted) > 0 {
		rm.deleteCallback(deleted)
	}

	// Call the refresh callback if set
	if rm.refreshCallback != nil {
		rm.refreshCallback()
	}

	return nil
}

// fetchData retrieves the JSON data from the remote URL
func (rm *RemoteMap) fetchData() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rm.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rm.url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range rm.headers {
		req.Header.Add(key, value)
	}

	resp, err := rm.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-OK response: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return data, nil
}

// updateMap updates the internal sync.Map with the fetched data
// Returns slices of added, updated, and deleted keys
func (rm *RemoteMap) updateMap(data map[string]interface{}) ([]string, []string, []string) {
	// Track existing keys and their values to detect changed and deleted entries
	existingKeys := make(map[string]interface{})

	// First, collect all existing keys and values
	rm.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			existingKeys[k] = value
		}
		return true
	})

	// Track added, changed, and deleted keys
	added := make([]string, 0)
	updated := make([]string, 0)

	// Process new data
	for key, value := range data {
		if oldValue, exists := existingKeys[key]; !exists {
			// This is a new key
			added = append(added, key)
		} else {
			// This key already exists, check if value has changed
			// Simple equality check might not work for complex types
			// For maps and slices, we need to do a deep comparison
			if !reflect.DeepEqual(oldValue, value) {
				updated = append(updated, key)
			}
			// Mark as processed
			delete(existingKeys, key)
		}
		// Store the value
		rm.Store(key, value)
	}

	// Any keys left in existingKeys are no longer in the data (deleted)
	deleted := make([]string, 0, len(existingKeys))
	for key := range existingKeys {
		deleted = append(deleted, key)
		rm.Delete(key)
	}

	return added, updated, deleted
}

// Keys returns all keys in the map as a slice of strings
func (rm *RemoteMap) Keys() []string {
	var keys []string
	rm.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			keys = append(keys, k)
		}
		return true
	})
	return keys
}

// Load retrieves a value from the map
// This is an override of sync.Map's Load method to handle type conversions
func (rm *RemoteMap) Load(key interface{}) (interface{}, bool) {
	value, ok := rm.Map.Load(key)
	if !ok {
		return nil, false
	}
	return value, true
}

// LoadOrStore loads the value for a key or stores the default value if it doesn't exist
// Returns the actual value and a boolean indicating whether the value was loaded
func (rm *RemoteMap) LoadOrStore(key string, defaultValue interface{}) (interface{}, bool) {
	// First try to load the value
	if value, ok := rm.Load(key); ok {
		// If the types match, return the loaded value
		if reflect.TypeOf(value) == reflect.TypeOf(defaultValue) {
			return value, true
		}
		
		// Handle type conversions based on the default value type
		switch defaultValue.(type) {
		case string:
			if strVal, ok := value.(string); ok {
				return strVal, true
			}
		case int:
			if floatVal, ok := value.(float64); ok {
				if float64(int(floatVal)) == floatVal {
					return int(floatVal), true
				}
			}
		case int64:
			if floatVal, ok := value.(float64); ok {
				if float64(int64(floatVal)) == floatVal {
					return int64(floatVal), true
				}
			}
		case float64:
			if floatVal, ok := value.(float64); ok {
				return floatVal, true
			}
		case bool:
			if boolVal, ok := value.(bool); ok {
				return boolVal, true
			}
		case []string:
			if strSlice, ok := getStringSlice(value); ok {
				return strSlice, true
			}
		case map[string]string:
			if strMap, ok := getStringMap(value); ok {
				return strMap, true
			}
		case map[string]bool:
			if boolMap, ok := getBoolMap(value); ok {
				return boolMap, true
			}
		case map[string][]string:
			if strSliceMap, ok := getStringSliceMap(value); ok {
				return strSliceMap, true
			}
		}
		
		// If we get here, the type conversion failed
		return defaultValue, false
	}

	// Store the default value
	rm.Store(key, defaultValue)
	return defaultValue, false
}

// GetFloat retrieves a float64 value from the map
func (rm *RemoteMap) GetFloat(key string) (float64, bool) {
	value, ok := rm.Load(key)
	if !ok {
		return 0, false
	}

	// Try to convert to float64
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	}

	// If we get here, the value is not a number
	return 0, false
}

// GetInt retrieves an int value from the map
func (rm *RemoteMap) GetInt(key string) (int, bool) {
	value, ok := rm.Load(key)
	if !ok {
		return 0, false
	}

	// Try to convert to int
	switch v := value.(type) {
	case int:
		return v, true
	case float64:
		// JSON numbers are decoded as float64, so we need to check if it's a whole number
		if float64(int(v)) == v {
			return int(v), true
		}
	case int64:
		// Check if the int64 value can fit in an int without overflow
		if int64(int(v)) == v {
			return int(v), true
		}
	}

	// If we get here, the value is not a valid int
	return 0, false
}

// GetBool retrieves a bool value from the map
func (rm *RemoteMap) GetBool(key string) (bool, bool) {
	value, ok := rm.Load(key)
	if !ok {
		return false, false
	}
	
	b, ok := value.(bool)
	return b, ok
}

// GetMap retrieves a nested map from the map
func (rm *RemoteMap) GetMap(key string) (map[string]interface{}, bool) {
	value, ok := rm.Load(key)
	if !ok {
		return nil, false
	}
	
	m, ok := value.(map[string]interface{})
	return m, ok
}

// GetInt64 retrieves an int64 value from the map
func (rm *RemoteMap) GetInt64(key string) (int64, bool) {
	value, ok := rm.Load(key)
	if !ok {
		return 0, false
	}

	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

// GetString retrieves a string value from the map
func (rm *RemoteMap) GetString(key string) (string, bool) {
	value, ok := rm.Load(key)
	if !ok {
		return "", false
	}
	
	str, ok := value.(string)
	return str, ok
}

// GetStringWithDefault retrieves a string value from the map or returns a default value if not found
func (rm *RemoteMap) GetStringWithDefault(key string, defaultValue string) string {
	value, ok := rm.GetString(key)
	if !ok {
		return defaultValue
	}
	return value
}

// GetIntWithDefault retrieves an int value from the map or returns a default value if not found
func (rm *RemoteMap) GetIntWithDefault(key string, defaultValue int) int {
	value, ok := rm.GetInt(key)
	if !ok {
		return defaultValue
	}
	return value
}

// GetFloatWithDefault retrieves a float64 value from the map or returns a default value if not found
func (rm *RemoteMap) GetFloatWithDefault(key string, defaultValue float64) float64 {
	value, ok := rm.GetFloat(key)
	if !ok {
		return defaultValue
	}
	return value
}

// GetBoolWithDefault retrieves a bool value from the map or returns a default value if not found
func (rm *RemoteMap) GetBoolWithDefault(key string, defaultValue bool) bool {
	value, ok := rm.GetBool(key)
	if !ok {
		return defaultValue
	}
	return value
}

// GetMapWithDefault retrieves a nested map from the map or returns a default value if not found
func (rm *RemoteMap) GetMapWithDefault(key string, defaultValue map[string]interface{}) map[string]interface{} {
	value, ok := rm.GetMap(key)
	if !ok {
		return defaultValue
	}
	return value
}

// GetInt64WithDefault retrieves an int64 value from the map or returns a default value if not found
func (rm *RemoteMap) GetInt64WithDefault(key string, defaultValue int64) int64 {
	value, ok := rm.GetInt64(key)
	if !ok {
		return defaultValue
	}
	return value
}

// GetStringSlice retrieves a slice of strings from the map
func (rm *RemoteMap) GetStringSlice(key string) ([]string, bool) {
	value, ok := rm.Load(key)
	if !ok {
		return nil, false
	}
	
	return getStringSlice(value)
}

// GetStringMap retrieves a map of string values from the map
func (rm *RemoteMap) GetStringMap(key string) (map[string]string, bool) {
	value, ok := rm.Load(key)
	if !ok {
		return nil, false
	}
	
	return getStringMap(value)
}

// GetBoolMap retrieves a map of boolean values from the map
func (rm *RemoteMap) GetBoolMap(key string) (map[string]bool, bool) {
	value, ok := rm.Load(key)
	if !ok {
		return nil, false
	}
	
	return getBoolMap(value)
}

// GetStringSliceMap retrieves a map of string slice values from the map
func (rm *RemoteMap) GetStringSliceMap(key string) (map[string][]string, bool) {
	value, ok := rm.Load(key)
	if !ok {
		return nil, false
	}
	
	return getStringSliceMap(value)
}

// GetStringSliceWithDefault retrieves a slice of strings from the map or returns a default value if not found
func (rm *RemoteMap) GetStringSliceWithDefault(key string, defaultValue []string) []string {
	value, ok := rm.GetStringSlice(key)
	if !ok {
		return defaultValue
	}
	return value
}

// GetStringMapWithDefault retrieves a map of string values from the map or returns a default value if not found
func (rm *RemoteMap) GetStringMapWithDefault(key string, defaultValue map[string]string) map[string]string {
	value, ok := rm.GetStringMap(key)
	if !ok {
		return defaultValue
	}
	return value
}

// GetBoolMapWithDefault retrieves a map of boolean values from the map or returns a default value if not found
func (rm *RemoteMap) GetBoolMapWithDefault(key string, defaultValue map[string]bool) map[string]bool {
	value, ok := rm.GetBoolMap(key)
	if !ok {
		return defaultValue
	}
	return value
}

// GetStringSliceMapWithDefault retrieves a map of string slice values from the map or returns a default value if not found
func (rm *RemoteMap) GetStringSliceMapWithDefault(key string, defaultValue map[string][]string) map[string][]string {
	value, ok := rm.GetStringSliceMap(key)
	if !ok {
		return defaultValue
	}
	return value
}

// Helper function to convert a value to a string slice
func getStringSlice(value interface{}) ([]string, bool) {
	switch v := value.(type) {
	case []string:
		return v, true
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			// Try to convert to string
			if str, ok := item.(string); ok {
				result = append(result, str)
			} else {
				// Skip non-string items
				continue
			}
		}
		return result, true
	}
	return nil, false
}

// Helper function to convert a value to a string map
func getStringMap(value interface{}) (map[string]string, bool) {
	switch v := value.(type) {
	case map[string]string:
		return v, true
	case map[string]interface{}:
		result := make(map[string]string)
		for k, val := range v {
			// Try to convert to string
			if strVal, ok := val.(string); ok {
				result[k] = strVal
			}
			// Skip non-string values
		}
		return result, true
	}
	return nil, false
}

// Helper function to convert a value to a boolean map
func getBoolMap(value interface{}) (map[string]bool, bool) {
	switch v := value.(type) {
	case map[string]bool:
		return v, true
	case map[string]interface{}:
		result := make(map[string]bool)
		for k, val := range v {
			// Try to convert to bool
			if boolVal, ok := val.(bool); ok {
				result[k] = boolVal
			}
			// Skip non-bool values
		}
		return result, true
	}
	return nil, false
}

// Helper function to convert a value to a string slice map
func getStringSliceMap(value interface{}) (map[string][]string, bool) {
	switch v := value.(type) {
	case map[string][]string:
		return v, true
	case map[string]interface{}:
		result := make(map[string][]string)
		for k, val := range v {
			// Try to convert to []string
			if sliceVal, ok := val.([]interface{}); ok {
				strSlice := make([]string, 0, len(sliceVal))
				for _, item := range sliceVal {
					if strItem, ok := item.(string); ok {
						strSlice = append(strSlice, strItem)
					}
					// Skip non-string items
				}
				if len(strSlice) > 0 {
					result[k] = strSlice
				}
			}
			// Skip non-slice values
		}
		return result, true
	}
	return nil, false
}
