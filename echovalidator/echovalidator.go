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

// CustomValidator holds an instance of the go-playground validator.
// Use this when you want to manage validator instances explicitly.
type CustomValidator struct {
	validator *validator.Validate
}

// New creates and returns a new CustomValidator instance,
// initializing the underlying go-playground validator.
func New() *CustomValidator {
	v := validator.New()

	// In the Go Playground validator library, when validation errors occur, the error messages
	// normally use the struct field name (like Name or Email). However, in API responses, you
	// typically want to use the JSON field names instead (like name or email).
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {

		// Split the JSON tag at the first comma
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]

		// If the JSON tag is "-" (which means "don't include this field in JSON"),
		// return an empty string
		if name == "-" {
			return ""
		}

		// Otherwise, return the JSON field name
		return name
	})

	return &CustomValidator{validator: v}
}

// Validate implements the echo.Validator interface for CustomValidator.
// It uses the go-playground validator to validate the struct 'i'.
// If validation fails, it returns an HTTPError with status 400
// and the validation errors. Otherwise, it returns nil.
func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

// Setup registers a new CustomValidator instance (created via New())
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
// from a CustomValidator. This allows for further customization, such as
// registering custom validation functions on this specific instance.
// Example:
//
//	customValidator := echovalidator.New()
//	v := customValidator.Validator()
//	v.RegisterValidation(...) // Register custom validation
//	e.Validator = customValidator // Then assign to Echo
func (cv *CustomValidator) Validator() *validator.Validate {
	return cv.validator
}

// --- Singleton Validator ---

// defaultValidator holds the singleton validator instance.
type defaultValidator struct {
	validator *validator.Validate
}

var (
	singletonInstance *defaultValidator
	initOnce          sync.Once
)

// initializeDefault creates the singleton validator instance.
// This function is called exactly once by initOnce.Do.
func initializeDefault() {
	singletonInstance = &defaultValidator{
		validator: validator.New(),
	}
}

// DefaultInstance returns the package-level singleton validator instance.
// It initializes the instance thread-safely on the first call.
// The returned instance implements the echo.Validator interface.
// Use this for simple setups where a single global validator is sufficient.
func DefaultInstance() *defaultValidator {
	initOnce.Do(initializeDefault)
	return singletonInstance
}

// Validate implements the echo.Validator interface for the singleton.
func (dv *defaultValidator) Validate(i interface{}) error {
	if err := dv.validator.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

// Validator returns the underlying go-playground/validator instance
// held by the singleton. Use this *carefully* early in your application's
// setup if you need to register custom validations globally for the singleton.
// Example:
//
//	// In your main or init function:
//	v := echovalidator.Instance().Validator()
//	err := v.RegisterValidation("custom_tag", myCustomFunc)
//	if err != nil { log.Fatal(err) }
//	// Now the singleton validator has the custom tag registered.
func (dv *defaultValidator) Validator() *validator.Validate {
	return dv.validator
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
	e.Validator = DefaultInstance() // Assign the singleton
}
