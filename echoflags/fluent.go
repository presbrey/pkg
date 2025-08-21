package echoflags

import "github.com/labstack/echo/v4"

// FlagSet is a helper that binds an SDK and context for fluent API access.
type FlagSet struct {
	sdk *SDK
	c   echo.Context
}

// WithContext creates a FlagSet for a specific context, allowing for a fluent API.
func (s *SDK) WithContext(c echo.Context) *FlagSet {
	return &FlagSet{
		sdk: s,
		c:   c,
	}
}

// GetString retrieves a string value for the given key.
func (fs *FlagSet) GetString(key string) (string, error) {
	return fs.sdk.GetString(fs.c, key)
}

// GetStringWithDefault retrieves a string value for the given key, with a default value.
func (fs *FlagSet) GetStringWithDefault(key string, defaultValue string) string {
	return fs.sdk.GetStringWithDefault(fs.c, key, defaultValue)
}

// GetBool retrieves a boolean value for the given key.
func (fs *FlagSet) GetBool(key string) (bool, error) {
	return fs.sdk.GetBool(fs.c, key)
}

// GetBoolWithDefault retrieves a boolean value for the given key, with a default value.
func (fs *FlagSet) GetBoolWithDefault(key string, defaultValue bool) bool {
	return fs.sdk.GetBoolWithDefault(fs.c, key, defaultValue)
}


// GetInt retrieves an integer value for the given key.
func (fs *FlagSet) GetInt(key string) (int, error) {
	return fs.sdk.GetInt(fs.c, key)
}

// GetIntWithDefault retrieves an integer value for the given key, with a default value.
func (fs *FlagSet) GetIntWithDefault(key string, defaultValue int) int {
	return fs.sdk.GetIntWithDefault(fs.c, key, defaultValue)
}

// GetFloat64 retrieves a float64 value for the given key.
func (fs *FlagSet) GetFloat64(key string) (float64, error) {
	return fs.sdk.GetFloat64(fs.c, key)
}

// GetFloat64WithDefault retrieves a float64 value for the given key, with a default value.
func (fs *FlagSet) GetFloat64WithDefault(key string, defaultValue float64) float64 {
	return fs.sdk.GetFloat64WithDefault(fs.c, key, defaultValue)
}

// GetStringSlice retrieves a string slice value for the given key.
func (fs *FlagSet) GetStringSlice(key string) ([]string, error) {
	return fs.sdk.GetStringSlice(fs.c, key)
}

// GetStringSliceWithDefault retrieves a string slice value for the given key, with a default value.
func (fs *FlagSet) GetStringSliceWithDefault(key string, defaultValue []string) []string {
	return fs.sdk.GetStringSliceWithDefault(fs.c, key, defaultValue)
}

// GetMap retrieves a map value for the given key.
func (fs *FlagSet) GetMap(key string) (map[string]interface{}, error) {
	return fs.sdk.GetMap(fs.c, key)
}

// GetMapWithDefault retrieves a map value for the given key, with a default value.
func (fs *FlagSet) GetMapWithDefault(key string, defaultValue map[string]interface{}) map[string]interface{} {
	return fs.sdk.GetMapWithDefault(fs.c, key, defaultValue)
}

// IsEnabled is a convenience method to check if a feature is enabled (boolean true).
func (fs *FlagSet) IsEnabled(key string) bool {
	return fs.sdk.IsEnabled(fs.c, key)
}
