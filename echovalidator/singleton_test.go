package echovalidator

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// --- Singleton-Based Tests ---

// Helper to reset singleton state for tests
func resetSingleton() {
	initOnce = sync.Once{}  // Reset the sync.Once
	singletonInstance = nil // Nil out the instance
}

func TestDefaultInstance(t *testing.T) {
	resetSingleton() // Ensure clean state

	instance1 := DefaultInstance() // Use the function directly
	assert.NotNil(t, instance1, "DefaultInstance() should return a non-nil validator")
	assert.NotNil(t, instance1.validator, "Singleton's internal validator should not be nil")

	instance2 := DefaultInstance() // Use the function directly
	assert.Same(t, instance1, instance2, "Multiple calls to Instance() should return the same singleton instance")

	// Check default tag name function IS NOT the JSON one by default
	v := instance1.Validator()
	invalidData := struct {
		JSONName   string `json:"json_name" validate:"required"`
		StructName string `validate:"required"`
	}{}
	err := v.Struct(invalidData)
	assert.Error(t, err)
	valErrors, ok := err.(validator.ValidationErrors)
	assert.True(t, ok, "Error should be validator.ValidationErrors")

	// Check if error refers to the Struct Field Name, not the JSON tag
	foundStructNameError := false
	for _, fieldError := range valErrors {
		if fieldError.StructField() == "StructName" {
			foundStructNameError = true
			break
		}
	}
	assert.True(t, foundStructNameError, "Default singleton error should use Struct Field 'StructName'")

	foundJSONNameError := false
	for _, fieldError := range valErrors {
		// Namespace includes struct name, field name etc. We just check the field part.
		if strings.HasSuffix(fieldError.Namespace(), ".JSONName") || fieldError.StructField() == "JSONName" {
			foundJSONNameError = true
			break
		}
	}
	assert.True(t, foundJSONNameError, "Default singleton error should use Struct Field 'JSONName', not 'json_name'")

	resetSingleton() // Clean up
}

func TestDefaultValidator_Validate_Valid(t *testing.T) {
	resetSingleton()        // Ensure clean state
	dv := DefaultInstance() // Use the function directly

	validData := TestValidStruct{
		Name:  "Jane Doe",
		Email: "jane.doe@example.com",
		Age:   25,
	}

	err := dv.Validate(validData)
	assert.Nil(t, err, "Validation should pass for valid data using singleton")
	resetSingleton() // Clean up
}

func TestDefaultValidator_Validate_Invalid(t *testing.T) {
	resetSingleton()        // Ensure clean state
	dv := DefaultInstance() // Use the function directly

	invalidData := TestInvalidStruct{
		Name:  "Test",
		Email: "invalid-email",
		Age:   10,
	}

	err := dv.Validate(invalidData)
	assert.NotNil(t, err, "Validation should fail for invalid data using singleton")

	httpErr, ok := err.(*echo.HTTPError)
	assert.True(t, ok, "Error should be an echo.HTTPError")
	assert.Equal(t, http.StatusBadRequest, httpErr.Code, "HTTP status code should be 400")
	// NOTE: Default validator uses struct field names (Name, Email, Age) not JSON tags
	errMsg := httpErr.Message.(string)
	// assert.Contains(t, errMsg, "Key: 'TestInvalidStruct.Name' Error:Field validation for 'Name' failed on the 'required' tag", "Error message should mention struct field 'Name' and 'required' tag") // Name is valid in test data
	assert.Contains(t, errMsg, "Key: 'TestInvalidStruct.Email' Error:Field validation for 'Email' failed on the 'email' tag", "Error message should mention struct field 'Email' and 'email' tag")
	assert.Contains(t, errMsg, "Key: 'TestInvalidStruct.Age' Error:Field validation for 'Age' failed on the 'min' tag", "Error message should mention struct field 'Age' and 'min' tag")
	// Ensure it's NOT using the JSON tag 'name' for the Name field error (if Name were invalid)
	assert.NotContains(t, errMsg, "validation failed for 'name'", "Error message should NOT use JSON tag 'name' by default")
	resetSingleton() // Clean up
}

func TestDefaultValidator_Validator(t *testing.T) {
	resetSingleton()        // Ensure clean state
	dv := DefaultInstance() // Use the function directly

	vInstance := dv.Validator()
	assert.NotNil(t, vInstance, "Validator() should return a non-nil validator.Validate")
	assert.Equal(t, dv.validator, vInstance, "Returned instance should be the same as the internal singleton one")
	// Check usability instead
	err := vInstance.Struct(TestValidStruct{Name: "Test", Email: "test@example.com"})
	assert.NoError(t, err, "Returned validator instance should be usable")
	resetSingleton() // Clean up
}

func TestSetupDefault(t *testing.T) {
	resetSingleton() // Ensure clean state

	e := echo.New()
	assert.Nil(t, e.Validator, "Validator should be nil initially")

	SetupDefault(e) // Use the function directly
	assert.NotNil(t, e.Validator, "Validator should be set after SetupDefault()")

	// Check if the type is the internal defaultValidator (which is unexported)
	// We can check if it implements the Validate method and if it's the same instance
	_, ok := e.Validator.(echo.Validator) // Check if it implements the interface
	assert.True(t, ok, "Assigned validator should implement echo.Validator")
	assert.Same(t, DefaultInstance(), e.Validator, "Echo's validator should be the singleton instance") // Check instance equality
	resetSingleton()                                                                                    // Clean up
}

func TestSetupDefault_NilEcho(t *testing.T) {
	assert.PanicsWithValue(t, "echovalidator.SetupDefault: received nil Echo instance", func() {
		SetupDefault(nil) // Use the function directly
	}, "SetupDefault(nil) should panic")
}

// --- Test Tag Name Function Registration (Singleton Example) ---

// Demonstrates how a user might register the JSON tag name func globally
func TestSingleton_RegisterTagNameFunc_Manually(t *testing.T) {
	resetSingleton()        // Ensure clean state
	dv := DefaultInstance() // Get the singleton
	v := dv.Validator()     // Get the underlying validator

	// Manually register the tag name function (like New() does)
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// Now validate with the modified singleton
	invalidData := TestInvalidStruct{

		Name:  "", // Required field missing
		Email: "bad",
		Age:   5,
	}

	err := dv.Validate(invalidData)
	assert.NotNil(t, err)
	httpErr, ok := err.(*echo.HTTPError)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)

	// Crucially, the error message should now use JSON tags because we registered the func
	errMsg := httpErr.Message.(string)
	fmt.Println("Error Message after manual registration:", errMsg) // Debug print
	assert.Contains(t, errMsg, "Key: 'TestInvalidStruct.name' Error:Field validation for 'name' failed on the 'required' tag", "Error message should match validator format for JSON tag 'name'")
	assert.Contains(t, errMsg, "Key: 'TestInvalidStruct.email' Error:Field validation for 'email' failed on the 'email' tag", "Error message should match validator format for JSON tag 'email'")
	assert.Contains(t, errMsg, "Key: 'TestInvalidStruct.age' Error:Field validation for 'age' failed on the 'min' tag", "Error message should match validator format for JSON tag 'age'")

	resetSingleton() // Clean up
}
