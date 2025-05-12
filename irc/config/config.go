package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// Config represents the server configuration
type Config struct {
	// Server settings
	Server struct {
		Name     string `yaml:"name" toml:"name" json:"name" env:"IRCD_SERVER_NAME"`
		Network  string `yaml:"network" toml:"network" json:"network" env:"IRCD_NETWORK"`
		Host     string `yaml:"host" toml:"host" json:"host" env:"IRCD_HOST"`
		Port     int    `yaml:"port" toml:"port" json:"port" env:"IRCD_PORT"`
		Password string `yaml:"password" toml:"password" json:"password" env:"IRCD_PASSWORD"`
	} `yaml:"server" toml:"server" json:"server"`

	// TLS settings
	TLS struct {
		Enabled    bool   `yaml:"enabled" toml:"enabled" json:"enabled" env:"IRCD_TLS_ENABLED"`
		Cert       string `yaml:"cert" toml:"cert" json:"cert" env:"IRCD_TLS_CERT"`
		Key        string `yaml:"key" toml:"key" json:"key" env:"IRCD_TLS_KEY"`
		Generation bool   `yaml:"auto_generate" toml:"auto_generate" json:"auto_generate" env:"IRCD_TLS_AUTO_GENERATE"`
	} `yaml:"tls" toml:"tls" json:"tls"`

	// Web portal settings
	WebPortal struct {
		Enabled bool   `yaml:"enabled" toml:"enabled" json:"enabled" env:"IRCD_WEB_ENABLED"`
		Host    string `yaml:"host" toml:"host" json:"host" env:"IRCD_WEB_HOST"`
		Port    int    `yaml:"port" toml:"port" json:"port" env:"IRCD_WEB_PORT"`
		TLS     bool   `yaml:"tls" toml:"tls" json:"tls" env:"IRCD_WEB_TLS"`
	} `yaml:"web_portal" toml:"web_portal" json:"web_portal"`

	// Bot API settings
	Bots struct {
		Enabled      bool     `yaml:"enabled" toml:"enabled" json:"enabled" env:"IRCD_BOTS_ENABLED"`
		Host         string   `yaml:"host" toml:"host" json:"host" env:"IRCD_BOTS_HOST"`
		Port         int      `yaml:"port" toml:"port" json:"port" env:"IRCD_BOTS_PORT"`
		BearerTokens []string `yaml:"bearer_tokens" toml:"bearer_tokens" json:"bearer_tokens" env:"IRCD_BOTS_TOKENS"`
	} `yaml:"bots" toml:"bots" json:"bots"`

	// Operator definitions
	Operators []struct {
		Username string `yaml:"username" toml:"username" json:"username"`
		Password string `yaml:"password" toml:"password" json:"password"`
		Email    string `yaml:"email" toml:"email" json:"email"`
		Mask     string `yaml:"mask" toml:"mask" json:"mask"`
	} `yaml:"operators" toml:"operators" json:"operators"`

	// Plugins/Extensions
	Plugins []struct {
		Name    string                 `yaml:"name" toml:"name" json:"name"`
		Enabled bool                   `yaml:"enabled" toml:"enabled" json:"enabled"`
		Config  map[string]interface{} `yaml:"config" toml:"config" json:"config"`
	} `yaml:"plugins" toml:"plugins" json:"plugins"`

	// Configuration source for rehashing
	Source string
}

// Load loads configuration from a file or URL
func Load(source string) (*Config, error) {
	cfg := &Config{
		Source: source,
	}

	// Set defaults
	cfg.Server.Name = "goircd.local"
	cfg.Server.Network = "GoIRCd"
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 6667

	// Load configuration from file or URL
	err := cfg.loadFromSource(source)
	if err != nil {
		return nil, err
	}

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	return cfg, nil
}

// Reload reloads the configuration from the original source or a new source
func (c *Config) Reload(newSource string) error {
	if newSource != "" {
		c.Source = newSource
	}

	// Create a new configuration with defaults
	newCfg := &Config{}
	newCfg.Server.Name = "goircd.local"
	newCfg.Server.Network = "GoIRCd"
	newCfg.Server.Host = "0.0.0.0"
	newCfg.Server.Port = 6667

	// Load configuration
	err := newCfg.loadFromSource(c.Source)
	if err != nil {
		return err
	}

	// Apply environment variable overrides
	applyEnvOverrides(newCfg)

	// Copy the new configuration to the current one
	*c = *newCfg
	return nil
}

// loadFromSource loads configuration from a file or URL
func (c *Config) loadFromSource(source string) error {
	var data []byte
	var err error

	// Check if the source is a URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		// Load from URL
		resp, err := http.Get(source)
		if err != nil {
			return fmt.Errorf("failed to load config from URL: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to load config from URL, status: %s", resp.Status)
		}

		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read config from URL: %v", err)
		}
	} else {
		// Load from file
		data, err = os.ReadFile(source)
		if err != nil {
			return fmt.Errorf("failed to read config file: %v", err)
		}
	}

	// Determine the format based on file extension
	switch {
	case strings.HasSuffix(source, ".yaml") || strings.HasSuffix(source, ".yml"):
		err = yaml.Unmarshal(data, c)
	case strings.HasSuffix(source, ".toml"):
		err = toml.Unmarshal(data, c)
	case strings.HasSuffix(source, ".json"):
		err = json.Unmarshal(data, c)
	default:
		// Default to YAML
		err = yaml.Unmarshal(data, c)
	}

	if err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	c.Source = source
	return nil
}

// applyEnvOverrides applies environment variable overrides to the configuration
func applyEnvOverrides(cfg *Config) {
	applyEnvOverridesRecursive(reflect.ValueOf(cfg).Elem(), "")
}

// applyEnvOverridesRecursive recursively applies environment variable overrides
func applyEnvOverridesRecursive(v reflect.Value, prefix string) {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		envTag := field.Tag.Get("env")

		if envTag != "" {
			// Field has an env tag, check if the environment variable exists
			if envValue, exists := os.LookupEnv(envTag); exists {
				// Try to set the field value from the environment variable
				setFieldFromEnv(fieldValue, envValue)
			}
		} else if field.Type.Kind() == reflect.Struct {
			// Recursively process nested structs
			newPrefix := prefix
			if prefix != "" {
				newPrefix += "_"
			}
			newPrefix += field.Name
			applyEnvOverridesRecursive(fieldValue, newPrefix)
		}
	}
}

// setFieldFromEnv sets a field's value from an environment variable
func setFieldFromEnv(field reflect.Value, envValue string) {
	switch field.Kind() {
	case reflect.String:
		field.SetString(envValue)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v, err := parseInt(envValue); err == nil {
			field.SetInt(v)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v, err := parseUint(envValue); err == nil {
			field.SetUint(v)
		}
	case reflect.Bool:
		if v, err := parseBool(envValue); err == nil {
			field.SetBool(v)
		}
	case reflect.Slice:
		// Handle string slices
		if field.Type().Elem().Kind() == reflect.String {
			values := strings.Split(envValue, ",")
			slice := reflect.MakeSlice(field.Type(), len(values), len(values))
			for i, v := range values {
				slice.Index(i).SetString(strings.TrimSpace(v))
			}
			field.Set(slice)
		}
	}
}

// Helper functions for parsing different types
func parseInt(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

func parseUint(s string) (uint64, error) {
	var v uint64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

func parseBool(s string) (bool, error) {
	s = strings.ToLower(s)
	return s == "true" || s == "1" || s == "yes" || s == "y", nil
}

// GetListenAddress returns the formatted listen address for the server
func (c *Config) GetListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetWebListenAddress returns the formatted listen address for the web portal
func (c *Config) GetWebListenAddress() string {
	return fmt.Sprintf("%s:%d", c.WebPortal.Host, c.WebPortal.Port)
}

// GetBotAPIListenAddress returns the formatted listen address for the bot API
func (c *Config) GetBotAPIListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Bots.Host, c.Bots.Port)
}
