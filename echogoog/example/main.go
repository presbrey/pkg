package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	googleopenid "github.com/presbrey/pkg/echogoog"
)

func main() {
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Configure Google OpenID middleware
	authMiddleware, err := googleopenid.New(&googleopenid.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),

		// Option 1: Use static RedirectURL
		// RedirectURL:  "http://localhost:8080/auth/google/callback",

		// Option 2: Use dynamic RedirectPath (generates full URL from request context)
		// This allows the app to work across different domains/schemes automatically
		RedirectPath: "/auth/google/callback",

		// Only allow users from these Google Workspace domains
		AllowedHostedDomains: []string{
			"example.com",
			"company.org",
		},

		// Cookie configuration
		SessionCookieName: "auth_session",
		SessionMaxAge:     86400, // 24 hours
		CookieSecure:      false, // Set to true in production with HTTPS
		CookieSameSite:    http.SameSiteLaxMode,

		// Custom paths (optional - these are the defaults)
		LoginPath:    "/auth/google/login",
		CallbackPath: "/auth/google/callback",
		LogoutPath:   "/auth/google/logout",

		// Redirect after successful login
		SuccessRedirect: "/dashboard",

		// Custom unauthorized handler (optional)
		UnauthorizedHandler: func(c echo.Context) error {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Authentication required",
			})
		},
	})
	if err != nil {
		e.Logger.Fatal(err)
	}

	// Register authentication routes
	authMiddleware.RegisterRoutes(e)

	// Public routes
	e.GET("/", func(c echo.Context) error {
		return c.HTML(http.StatusOK, `
			<html>
				<body>
					<h1>Welcome to the App</h1>
					<p><a href="/auth/google/login">Login with Google</a></p>
					<p><a href="/dashboard">Go to Dashboard (requires auth)</a></p>
				</body>
			</html>
		`)
	})

	// Protected routes - require authentication
	protected := e.Group("")
	protected.Use(authMiddleware.Protect())

	protected.GET("/dashboard", func(c echo.Context) error {
		// Get authenticated user information
		user, err := googleopenid.GetUser(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get user")
		}

		return c.HTML(http.StatusOK, fmt.Sprintf(`
			<html>
				<body>
					<h1>Dashboard</h1>
					<div>
						<img src="%s" alt="Profile Picture" style="border-radius: 50%%; width: 100px;">
						<h2>Welcome, %s!</h2>
						<p><strong>Email:</strong> %s</p>
						<p><strong>Domain:</strong> %s</p>
						<p><strong>Email Verified:</strong> %t</p>
					</div>
					<p><a href="/profile">View Profile</a></p>
					<p><a href="/auth/google/logout">Logout</a></p>
				</body>
			</html>
		`, user.Picture, user.Name, user.Email, user.HostedDomain, user.EmailVerified))
	})

	protected.GET("/profile", func(c echo.Context) error {
		user, err := googleopenid.GetUser(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get user")
		}

		return c.JSON(http.StatusOK, user)
	})

	protected.GET("/api/data", func(c echo.Context) error {
		user, err := googleopenid.GetUser(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get user")
		}

		// Business logic here - user is authenticated and from allowed domain
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "This is protected data",
			"user":    user.Email,
			"domain":  user.HostedDomain,
		})
	})

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}
