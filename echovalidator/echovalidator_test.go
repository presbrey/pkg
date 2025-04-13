package echovalidator_test

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/presbrey/pkg/echovalidator"
	"github.com/stretchr/testify/assert"
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

// --- Instance-Based Tests ---

func TestNew(t *testing.T) {
	cv := echovalidator.New()
	assert.NotNil(t, cv, "New() should return a non-nil CustomValidator")
	assert.NotNil(t, cv.Validator(), "CustomValidator's internal validator should not be nil")

	// Check the behavior of the tag name function indirectly via Validate
	invalidData := struct {
		BadJSONName string `json:"bad-json-name" validate:"required"`
	}{}
	err := cv.Validate(invalidData)
	assert.NotNil(t, err)
	httpErr, ok := err.(*echo.HTTPError)
	assert.True(t, ok)
	// Ensure the error message uses the JSON tag name
	assert.Contains(t, httpErr.Message.(string), "Key: 'bad-json-name' Error:Field validation for 'bad-json-name' failed on the 'required' tag", "Error message should match validator format for JSON tag 'bad-json-name'")

	// Test ignored field
	fieldIgnored := TestIgnoredField{
		PublicData: "data", // This is valid
	}
	err = cv.Validate(fieldIgnored) // Should pass as InternalID is ignored
	assert.Nil(t, err, "Validation should pass when only the ignored field is present but not required")

	missingData := TestIgnoredField{
		InternalID: "id",
		// PublicData is missing but required
	}
	err = cv.Validate(missingData)
	assert.NotNil(t, err)
	httpErr, ok = err.(*echo.HTTPError)
	assert.True(t, ok)
	assert.Contains(t, httpErr.Message.(string), "Key: 'TestIgnoredField.public_data' Error:Field validation for 'public_data' failed on the 'required' tag", "Error message should match validator format for JSON tag 'public_data'")
	assert.NotContains(t, httpErr.Message.(string), "InternalID")
}

func TestCustomValidator_Validate_Valid(t *testing.T) {
	cv := echovalidator.New()
	validData := TestValidStruct{
		Name:  "John Doe",
		Email: "john.doe@example.com",
		Age:   30,
	}

	err := cv.Validate(validData)
	assert.Nil(t, err, "Validation should pass for valid data")
}

func TestCustomValidator_Validate_Invalid(t *testing.T) {
	cv := echovalidator.New()
	invalidData := TestInvalidStruct{
		Name:  "", // Required field missing
		Email: "not-an-email",
		Age:   15, // Below min age
	}

	err := cv.Validate(invalidData)
	assert.NotNil(t, err, "Validation should fail for invalid data")

	httpErr, ok := err.(*echo.HTTPError)
	assert.True(t, ok, "Error should be an echo.HTTPError")
	assert.Equal(t, http.StatusBadRequest, httpErr.Code, "HTTP status code should be 400")
	errMsg := httpErr.Message.(string)
	assert.Contains(t, errMsg, "Key: 'TestInvalidStruct.name' Error:Field validation for 'name' failed on the 'required' tag", "Error message should match validator format for JSON tag 'name'")
	assert.Contains(t, errMsg, "Key: 'TestInvalidStruct.email' Error:Field validation for 'email' failed on the 'email' tag", "Error message should match validator format for JSON tag 'email'")
	assert.Contains(t, errMsg, "Key: 'TestInvalidStruct.age' Error:Field validation for 'age' failed on the 'min' tag", "Error message should match validator format for JSON tag 'age'")
}

func TestSetup(t *testing.T) {
	e := echo.New()
	assert.Nil(t, e.Validator, "Validator should be nil initially")

	echovalidator.Setup(e)
	assert.NotNil(t, e.Validator, "Validator should be set after Setup()")
	_, ok := e.Validator.(*echovalidator.Wrapper)
	assert.True(t, ok, "Validator should be of type *Wrapper")
}

func TestSetup_NilEcho(t *testing.T) {
	assert.PanicsWithValue(t, "echovalidator.Setup: received nil Echo instance", func() {
		echovalidator.Setup(nil)
	}, "Setup(nil) should panic")
}

func TestCustomValidator_Validator(t *testing.T) {
	cv := echovalidator.New()
	vInstance := cv.Validator()
	assert.NotNil(t, vInstance, "Validator() should return a non-nil validator.Validate")
	assert.Equal(t, cv.Validator(), vInstance, "Returned instance should be the same as the internal one")
	// Check if we can use the returned instance for validation
	err := vInstance.Struct(TestValidStruct{Name: "Test", Email: "test@example.com"})
	assert.NoError(t, err, "Returned validator instance should be usable")
}
