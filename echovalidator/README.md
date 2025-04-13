# echovalidator

`echovalidator` provides a simple integration of the [`go-playground/validator/v10`](https://github.com/go-playground/validator) library with the [Echo](https://github.com/labstack/echo) web framework (`v4`). It allows you to easily validate incoming request data based on struct tags.

## Features

*   Seamless integration with Echo's `Validator` interface.
*   Supports both instance-based and singleton validator approaches.
*   Automatically uses `json` struct tags for validation field names when using the instance-based `New()` constructor.
*   Provides access to the underlying `validator.Validate` instance for custom validation registration.

## Installation

```bash
go get github.com/presbrey/pkg/echovalidator
```

*(Replace `github.com/presbrey/pkg` with the actual path if it's different)*

## Usage

There are two main ways to use `echovalidator`:

### 1. Instance-Based Validator (Recommended for JSON tags)

This approach creates a specific validator instance. The `New()` constructor automatically configures the validator to use `json` struct tags when reporting validation errors, which is often desirable for APIs.

```go
package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/presbrey/pkg/echovalidator" // Adjust import path if needed
)

type User struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"omitempty,gte=0,lte=130"`
}

func main() {
	e := echo.New()

	// Create and assign a new validator instance
	e.Validator = echovalidator.New()

	e.POST("/users", func(c echo.Context) error {
		u := new(User)
		if err := c.Bind(u); err != nil {
			// Handle binding error (e.g., invalid JSON format)
			// Note: This error is from Echo's binder, not the validator yet.
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
		}
		if err := c.Validate(u); err != nil {
			// Handle validation error (returned as *echo.HTTPError)
			// The error message will use JSON tags (e.g., "'name' failed...")
			return err // Directly return the HTTPError from the validator
		}
		return c.JSON(http.StatusOK, u)
	})

	e.Logger.Fatal(e.Start(":1323"))
}

```

### 2. Singleton Validator (Global Instance)

This approach uses a package-level singleton instance. It's simpler to set up globally but **does not** automatically use JSON tags for error messages by default. Error messages will refer to the original struct field names (e.g., `Name`, `Email`).

```go
package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/presbrey/pkg/echovalidator" // Adjust import path if needed
)

type User struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"omitempty,gte=0,lte=130"`
}

func main() {
	e := echo.New()

	// Setup the default singleton validator globally for the Echo instance
	echovalidator.SetupDefault(e)
	// Alternatively: e.Validator = echovalidator.DefaultInstance()

	e.POST("/users", func(c echo.Context) error {
		u := new(User)
		if err := c.Bind(u); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
		}
		if err := c.Validate(u); err != nil {
			// Handle validation error (returned as *echo.HTTPError)
			// The error message will use Struct field names (e.g., "'Name' failed...")
			return err
		}
		return c.JSON(http.StatusOK, u)
	})

	e.Logger.Fatal(e.Start(":1323"))
}
```

### Advanced: Custom Validations

You can access the underlying `validator.Validate` instance to register custom validation functions:

**Instance-Based:**

```go
customValidator := echovalidator.New()
v := customValidator.Validator() // Get underlying validator
err := v.RegisterValidation("custom_tag", myCustomValidationFunc)
// handle err
e.Validator = customValidator
```

**Singleton:**

```go
// Do this early, e.g., in an init() or main()
v := echovalidator.DefaultInstance().Validator() // Get underlying validator
err := v.RegisterValidation("custom_tag", myCustomValidationFunc)
// handle err
echovalidator.SetupDefault(e) // Or assign later
```

## Testing

Tests are included in the package. Run them using:

```bash
go test ./...
```
