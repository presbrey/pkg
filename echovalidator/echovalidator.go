// Package echovalidator provides convenient ways to set up
// github.com/go-playground/validator/v10 as the default validator
// for the Echo web framework (github.com/labstack/echo/v4).
//
// It offers two main approaches:
//
//  1. Instance-based: Create validator instances using New() and manage them yourself.
//     This is useful if you need different validator configurations or want explicit control.
//     Use echovalidator.Setup(e) or set e.Validator = echovalidator.New() manually.
//
//  2. Singleton-based: Use a package-level singleton instance for simple setups.
//     Access the singleton via echovalidator.Instance() and register it using
//     echovalidator.SetupDefault(e). Modifications to the singleton (e.g., adding
//     custom validations via Instance().Validator().RegisterValidation(...)) affect
//     the entire application using this singleton.
package echovalidator

import (
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// --- Test Structs ---

type TestValidStruct struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age,omitempty" validate:"gte=0,lte=130"`
}

type TestInvalidStruct struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"` // Invalid email format
	Age   int    `json:"age" validate:"min=18"`           // Below minimum age
}

type TestIgnoredField struct {
	InternalID string `json:"-"` // Should be ignored by tag name func
	PublicData string `json:"public_data" validate:"required"`
}

// --- Instance-Based Validator ---

// Wrapper wraps the validator.Validate instance
type Wrapper struct {
	validator *validator.Validate
}

// Configurator provides a fluent interface for configuring the validator.
type Configurator struct {
	validator *validator.Validate
}

// NewConfigurator creates a new Configurator.
func NewConfigurator() *Configurator {
	return &Configurator{
		validator: validator.New(),
	}
}

// RegisterJSONTagNameFunc registers a function to use JSON field names in validation errors.
// This allows API error messages to refer to 'email' instead of 'Email'.
func (c *Configurator) RegisterJSONTagNameFunc() *Configurator {
	c.validator.RegisterTagNameFunc(func(fld reflect.StructField) string {
		// Split the JSON tag at the first comma
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]

		// If the JSON tag is "-" (meaning "don't include this field in JSON"),
		// return an empty string so the validator uses the original field name
		// or potentially skips it based on other rules.
		if name == "-" {
			// Returning an empty string tells the validator to skip this tag name transformation
			// and potentially fall back to the default field name or other registered tag names.
			// For the specific goal of using JSON names, returning empty for '-' is appropriate.
			return ""
		}

		// Otherwise, return the JSON field name
		return name
	})
	return c // Return self for chaining
}

// Validator returns the configured validator instance.
func (c *Configurator) Validator() *validator.Validate {
	return c.validator
}

// New creates a new Wrapper instance with default configuration.
// It specifically configures the validator to use JSON tag names in error messages.
func New() *Wrapper {
	// Create and configure the validator using the fluent configurator
	v := NewConfigurator().
		RegisterJSONTagNameFunc(). // Use JSON tags for field names in errors
		Validator()                // Get the configured validator instance

	// Return the Wrapper instance which wraps the configured validator
	return &Wrapper{
		validator: v,
	}
}

// Validate implements the echo.Validator interface for EchoValidator.
// It uses the go-playground validator to validate the struct 'i'.
// If validation fails, it returns an HTTPError with status 400
// and the validation errors. Otherwise, it returns nil.
func (cv *Wrapper) Validate(i any) error {
	if err := cv.validator.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

// Setup registers a new EchoValidator instance (created via New())
// with the provided Echo app.
// This is a convenience function for the instance-based approach.
// Example:
//
//	e := echo.New()
//	echovalidator.Setup(e)
//
// Equivalent to:
//
//	e := echo.New()
//	e.Validator = echovalidator.New()
func Setup(e *echo.Echo) {
	if e == nil {
		panic("echovalidator.Setup: received nil Echo instance")
	}
	e.Validator = New()
}

// ValidatorInstance returns the underlying validator.Validate instance
// from a EchoValidator. This allows for further customization, such as
// registering custom validation functions on this specific instance.
// Example:
//
//	customValidator := echovalidator.New()
//	v := customValidator.Validator()
//	v.RegisterValidation(...) // Register custom validation
//	e.Validator = customValidator // Then assign to Echo
func (cv *Wrapper) Validator() *validator.Validate {
	return cv.validator
}

// --- Singleton Validator ---

var (
	singletonInstance *Wrapper
	initOnce          sync.Once
)

// initializeDefault creates the singleton validator instance.
// This function is called exactly once by initOnce.Do.
func initializeDefault() {
	singletonInstance = &Wrapper{
		validator: NewConfigurator().
			RegisterJSONTagNameFunc().
			Validator(),
	}
}

// DefaultInstance returns the package-level singleton validator instance.
// It initializes the instance thread-safely on the first call.
// The returned instance implements the echo.Validator interface.
// Use this for simple setups where a single global validator is sufficient.
func Default() *Wrapper {
	initOnce.Do(initializeDefault)
	return singletonInstance
}

// SetupDefault registers the package-level singleton validator (obtained via Instance())
// with the provided Echo instance.
// This is the convenience function for the singleton approach.
// Example:
//
//	e := echo.New()
//	echovalidator.SetupDefault(e)
//
// Equivalent to:
//
//	e := echo.New()
//	e.Validator = echovalidator.Instance()
func SetupDefault(e *echo.Echo) {
	if e == nil {
		panic("echovalidator.SetupDefault: received nil Echo instance")
	}
	e.Validator = Default() // Assign the singleton
}
