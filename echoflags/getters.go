package echoflags

import (
	"fmt"
	"strconv"

	"github.com/labstack/echo/v4"
)

// GetString retrieves a string value for the given key
func (s *SDK) GetString(c echo.Context, key string) (string, error) {
	value, err := s.getValue(c, key)
	if err != nil {
		return "", err
	}

	switch v := value.(type) {
	case string:
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// GetStringWithDefault retrieves a string value for the given key, with a default value.
func (s *SDK) GetStringWithDefault(c echo.Context, key string, defaultValue string) string {
	value, err := s.GetString(c, key)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetBool retrieves a boolean value for the given key
func (s *SDK) GetBool(c echo.Context, key string) (bool, error) {
	value, err := s.getValue(c, key)
	if err != nil {
		return false, err
	}

	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case int, int32, int64, float32, float64:
		return v != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", value)
	}
}

// GetBoolWithDefault retrieves a boolean value for the given key, with a default value.
func (s *SDK) GetBoolWithDefault(c echo.Context, key string, defaultValue bool) bool {
	value, err := s.GetBool(c, key)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetInt retrieves an integer value for the given key
func (s *SDK) GetInt(c echo.Context, key string) (int, error) {
	value, err := s.getValue(c, key)
	if err != nil {
		return 0, err
	}

	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

// GetIntWithDefault retrieves an integer value for the given key, with a default value.
func (s *SDK) GetIntWithDefault(c echo.Context, key string, defaultValue int) int {
	value, err := s.GetInt(c, key)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetFloat64 retrieves a float64 value for the given key
func (s *SDK) GetFloat64(c echo.Context, key string) (float64, error) {
	value, err := s.getValue(c, key)
	if err != nil {
		return 0, err
	}

	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

// GetFloat64WithDefault retrieves a float64 value for the given key, with a default value.
func (s *SDK) GetFloat64WithDefault(c echo.Context, key string, defaultValue float64) float64 {
	value, err := s.GetFloat64(c, key)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetStringSlice retrieves a string slice value for the given key
func (s *SDK) GetStringSlice(c echo.Context, key string) ([]string, error) {
	value, err := s.getValue(c, key)
	if err != nil {
		return nil, err
	}

	switch v := value.(type) {
	case []string:
		return v, nil
	case []interface{}:
		result := make([]string, len(v))
		for i, item := range v {
			result[i] = fmt.Sprintf("%v", item)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to []string", value)
	}
}

// GetStringSliceWithDefault retrieves a string slice value for the given key, with a default value.
func (s *SDK) GetStringSliceWithDefault(c echo.Context, key string, defaultValue []string) []string {
	value, err := s.GetStringSlice(c, key)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetMap retrieves a map value for the given key
func (s *SDK) GetMap(c echo.Context, key string) (map[string]interface{}, error) {
	value, err := s.getValue(c, key)
	if err != nil {
		return nil, err
	}

	switch v := value.(type) {
	case map[string]interface{}:
		return v, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to map[string]interface{}", value)
	}
}

// GetMapWithDefault retrieves a map value for the given key, with a default value.
func (s *SDK) GetMapWithDefault(c echo.Context, key string, defaultValue map[string]interface{}) map[string]interface{} {
	value, err := s.GetMap(c, key)
	if err != nil {
		return defaultValue
	}
	return value
}

// IsEnabled is a convenience method to check if a feature is enabled (boolean true)
func (s *SDK) IsEnabled(c echo.Context, key string) bool {
	enabled, err := s.GetBool(c, key)
	if err != nil {
		return false
	}
	return enabled
}
