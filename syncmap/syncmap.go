package syncmap

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// DefaultRefreshPeriod is the default time between refreshes of the remote data
const DefaultRefreshPeriod = 5 * time.Minute

// DefaultTimeout is the default timeout for HTTP requests
const DefaultTimeout = 30 * time.Second

// Options contains configuration options for RemoteMap
type Options struct {
	// RefreshPeriod is the time between refreshes of the remote data
	RefreshPeriod time.Duration

	// Timeout is the timeout for HTTP requests
	Timeout time.Duration

	// IgnoreTLSVerify disables TLS certificate verification when true
	IgnoreTLSVerify bool

	// Headers are additional HTTP headers to include in requests
	Headers map[string]string

	// ErrorHandler is called when an error occurs during refresh
	// If nil, errors are ignored
	ErrorHandler func(error)

	// TransformFunc allows transforming the fetched data before storing
	// If nil, data is stored as-is
	TransformFunc func(map[string]any) map[string]any
}

// RemoteMap extends sync.Map to synchronize with a remote JSON endpoint
type RemoteMap struct {
	sync.Map
	url        string
	options    Options
	httpClient *http.Client
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewRemoteMap creates a new RemoteMap that synchronizes with the provided URL
func NewRemoteMap(url string, options *Options) *RemoteMap {
	opts := getDefaultOptions()
	if options != nil {
		if options.RefreshPeriod > 0 {
			opts.RefreshPeriod = options.RefreshPeriod
		}
		if options.Timeout > 0 {
			opts.Timeout = options.Timeout
		}
		opts.IgnoreTLSVerify = options.IgnoreTLSVerify
		if options.Headers != nil {
			opts.Headers = options.Headers
		}
		if options.ErrorHandler != nil {
			opts.ErrorHandler = options.ErrorHandler
		}
		if options.TransformFunc != nil {
			opts.TransformFunc = options.TransformFunc
		}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: opts.IgnoreTLSVerify}

	client := &http.Client{
		Timeout:   opts.Timeout,
		Transport: transport,
	}

	return &RemoteMap{
		url:        url,
		options:    opts,
		httpClient: client,
	}
}

// getDefaultOptions returns the default options
func getDefaultOptions() Options {
	return Options{
		RefreshPeriod:   DefaultRefreshPeriod,
		Timeout:         DefaultTimeout,
		IgnoreTLSVerify: false,
		Headers:         make(map[string]string),
		ErrorHandler:    nil,
		TransformFunc:   nil,
	}
}

// Start begins the periodic refresh of the map from the remote URL
func (rm *RemoteMap) Start() {
	// Immediately fetch data once
	if err := rm.Refresh(); err != nil && rm.options.ErrorHandler != nil {
		rm.options.ErrorHandler(err)
	}

	// Set up periodic refresh
	ctx, cancel := context.WithCancel(context.Background())
	rm.cancel = cancel

	rm.wg.Add(1)
	go func() {
		defer rm.wg.Done()
		ticker := time.NewTicker(rm.options.RefreshPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := rm.Refresh(); err != nil && rm.options.ErrorHandler != nil {
					rm.options.ErrorHandler(err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop halts the periodic refresh of the map
func (rm *RemoteMap) Stop() {
	if rm.cancel != nil {
		rm.cancel()
		rm.wg.Wait()
		rm.cancel = nil
	}
}

// Refresh immediately updates the map from the remote URL
func (rm *RemoteMap) Refresh() error {
	data, err := rm.fetchData()
	if err != nil {
		return err
	}

	// Apply transform function if provided
	if rm.options.TransformFunc != nil {
		data = rm.options.TransformFunc(data)
	}

	// Update the map with the new data
	rm.updateMap(data)
	return nil
}

// fetchData retrieves the JSON data from the remote URL
func (rm *RemoteMap) fetchData() (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, rm.url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range rm.options.Headers {
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

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return data, nil
}

// updateMap updates the internal sync.Map with the fetched data
func (rm *RemoteMap) updateMap(data map[string]any) {
	// Track keys to detect deleted entries
	currentKeys := make(map[string]bool)

	// First, collect all current keys
	rm.Range(func(key, value any) bool {
		if k, ok := key.(string); ok {
			currentKeys[k] = true
		}
		return true
	})

	// Update with new data
	for key, value := range data {
		rm.Store(key, value)
		delete(currentKeys, key)
	}

	// Remove keys that no longer exist in the remote data
	for key := range currentKeys {
		rm.Delete(key)
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

// GetInt retrieves an int value from the map
func (rm *RemoteMap) GetInt(key string) (int, bool) {
	value, ok := rm.Load(key)
	if !ok {
		return 0, false
	}

	// Handle different numeric types in JSON
	switch v := value.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	case int64:
		return int(v), true
	default:
		return 0, false
	}
}

// GetFloat retrieves a float64 value from the map
func (rm *RemoteMap) GetFloat(key string) (float64, bool) {
	value, ok := rm.Load(key)
	if !ok {
		return 0, false
	}

	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
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
func (rm *RemoteMap) GetMap(key string) (map[string]any, bool) {
	value, ok := rm.Load(key)
	if !ok {
		return nil, false
	}

	m, ok := value.(map[string]any)
	return m, ok
}

// GetInt64 retrieves an int64 value from the map
func (rm *RemoteMap) GetInt64(key string) (int64, bool) {
	value, ok := rm.Load(key)
	if !ok {
		return 0, false
	}

	// Handle different numeric types in JSON
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}
