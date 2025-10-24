package echogoog

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Config holds the configuration for the Google OpenID middleware
type Config struct {
	// ClientID is the Google OAuth2 client ID
	ClientID string

	// ClientSecret is the Google OAuth2 client secret
	ClientSecret string

	// RedirectURL is the callback URL for OAuth2 flow
	// Either RedirectURL or RedirectPath must be provided
	RedirectURL string

	// RedirectPath is the callback path for OAuth2 flow (alternative to RedirectURL)
	// When set, the absolute URL is generated dynamically from the request's scheme and host
	// Example: "/auth/google/callback" becomes "https://example.com/auth/google/callback"
	RedirectPath string

	// TrustForwardedHeaders controls whether to trust X-Forwarded-* and Forwarded headers
	// SECURITY: Only enable when behind a trusted proxy/load balancer that sets these headers
	// Default: false (for security)
	TrustForwardedHeaders bool

	// AllowedRedirectHosts is an optional list of allowed hostnames for redirect URL generation
	// When set, only these hosts are allowed when using RedirectPath
	// Example: ["example.com", "staging.example.com", "localhost:8080"]
	// Default: empty (allows any host - use with caution)
	AllowedRedirectHosts []string

	// AllowedHostedDomains is a list of Google Workspace domains allowed to authenticate
	// Example: ["example.com", "company.org"]
	AllowedHostedDomains []string

	// Scopes are the OAuth2 scopes to request (default: openid, email, profile)
	Scopes []string

	// SessionCookieName is the name of the session cookie (default: "google_openid_session")
	SessionCookieName string

	// SessionMaxAge is the max age of the session cookie in seconds (default: 86400 = 24 hours)
	SessionMaxAge int

	// CookieSecure sets the Secure flag on cookies (should be true in production)
	CookieSecure bool

	// CookieHTTPOnly sets the HttpOnly flag on cookies (default: true)
	CookieHTTPOnly bool

	// CookieSameSite sets the SameSite attribute for cookies (default: Lax)
	CookieSameSite http.SameSite

	// LoginPath is the path where users initiate login (default: "/auth/google/login")
	LoginPath string

	// CallbackPath is the path for the OAuth2 callback (default: "/auth/google/callback")
	CallbackPath string

	// LogoutPath is the path for logout (default: "/auth/google/logout")
	LogoutPath string

	// UnauthorizedHandler is called when authentication fails
	UnauthorizedHandler echo.HandlerFunc

	// SuccessRedirect is the URL to redirect to after successful authentication
	SuccessRedirect string
}

// UserInfo represents the authenticated user's information
type UserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	HostedDomain  string `json:"hd"` // Google Workspace domain
}

// Middleware manages Google OpenID authentication
type Middleware struct {
	config       *Config
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
	provider     *oidc.Provider
}

const (
	contextKeyUser = "google_openid_user"
	stateKey       = "google_openid_state"
)

// New creates a new Google OpenID middleware with the given configuration
func New(config *Config) (*Middleware, error) {
	if config.ClientID == "" {
		return nil, errors.New("ClientID is required")
	}
	if config.ClientSecret == "" {
		return nil, errors.New("ClientSecret is required")
	}
	if config.RedirectURL == "" && config.RedirectPath == "" {
		return nil, errors.New("either RedirectURL or RedirectPath is required")
	}
	if config.RedirectURL != "" && config.RedirectPath != "" {
		return nil, errors.New("cannot specify both RedirectURL and RedirectPath")
	}

	// Normalize RedirectPath to ensure it starts with exactly one leading "/"
	if config.RedirectPath != "" {
		config.RedirectPath = strings.TrimSpace(config.RedirectPath)
		config.RedirectPath = "/" + strings.TrimLeft(config.RedirectPath, "/")
	}

	// Set defaults
	if config.SessionCookieName == "" {
		config.SessionCookieName = "google_openid_session"
	}
	if config.SessionMaxAge == 0 {
		config.SessionMaxAge = 86400 // 24 hours
	}
	if config.CookieSameSite == 0 {
		config.CookieSameSite = http.SameSiteLaxMode
	}
	if config.LoginPath == "" {
		config.LoginPath = "/auth/google/login"
	}
	if config.CallbackPath == "" {
		config.CallbackPath = "/auth/google/callback"
	}
	if config.LogoutPath == "" {
		config.LogoutPath = "/auth/google/logout"
	}
	if len(config.Scopes) == 0 {
		config.Scopes = []string{oidc.ScopeOpenID, "email", "profile"}
	}
	config.CookieHTTPOnly = true // Always set HttpOnly for security

	// Initialize OIDC provider
	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	// Configure OAuth2
	// When using RedirectPath, we'll set a placeholder here and update it dynamically
	redirectURL := config.RedirectURL
	if redirectURL == "" {
		redirectURL = "http://placeholder" // Will be overridden dynamically
	}

	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     google.Endpoint,
		Scopes:       config.Scopes,
	}

	// Create ID token verifier
	verifier := provider.Verifier(&oidc.Config{
		ClientID: config.ClientID,
	})

	return &Middleware{
		config:       config,
		oauth2Config: oauth2Config,
		verifier:     verifier,
		provider:     provider,
	}, nil
}

// RegisterRoutes registers the authentication routes on the Echo instance
func (m *Middleware) RegisterRoutes(e *echo.Echo) {
	e.GET(m.config.LoginPath, m.handleLogin)
	e.GET(m.config.CallbackPath, m.handleCallback)
	e.GET(m.config.LogoutPath, m.handleLogout)
}

// Protect returns an Echo middleware that requires authentication
func (m *Middleware) Protect() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, err := m.getUserFromSession(c)
			if err != nil || user == nil {
				if m.config.UnauthorizedHandler != nil {
					return m.config.UnauthorizedHandler(c)
				}
				return c.Redirect(http.StatusTemporaryRedirect, m.config.LoginPath)
			}

			// Store user in context
			c.Set(contextKeyUser, user)
			return next(c)
		}
	}
}

// GetUser retrieves the authenticated user from the request context
func GetUser(c echo.Context) (*UserInfo, error) {
	user := c.Get(contextKeyUser)
	if user == nil {
		return nil, errors.New("user not found in context")
	}
	userInfo, ok := user.(*UserInfo)
	if !ok {
		return nil, errors.New("invalid user info in context")
	}
	return userInfo, nil
}

// handleLogin initiates the OAuth2 flow
func (m *Middleware) handleLogin(c echo.Context) error {
	state, err := generateRandomState()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to generate state")
	}

	// Store state in session cookie
	m.setSessionCookie(c, stateKey, state, 600) // 10 minutes

	// Get per-request oauth2 config (avoids data race on shared config)
	oauth2Cfg := m.getOAuth2Config(c)

	// Build authorization URL with hd parameter if hosted domains are specified
	authURL := oauth2Cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)

	// Add hosted domain hint if only one domain is allowed
	if len(m.config.AllowedHostedDomains) == 1 {
		authURL += "&hd=" + m.config.AllowedHostedDomains[0]
	}

	return c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// handleCallback processes the OAuth2 callback
func (m *Middleware) handleCallback(c echo.Context) error {
	// Verify state
	stateCookie, err := c.Cookie(stateKey)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "State cookie not found")
	}

	state := c.QueryParam("state")
	if state != stateCookie.Value {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid state parameter")
	}

	// Clear state cookie
	m.clearCookie(c, stateKey)

	// Get per-request oauth2 config (avoids data race on shared config)
	oauth2Cfg := m.getOAuth2Config(c)

	// Exchange code for token
	code := c.QueryParam("code")
	oauth2Token, err := oauth2Cfg.Exchange(c.Request().Context(), code)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to exchange token")
	}

	// Extract ID token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "No id_token in token response")
	}

	// Verify ID token
	idToken, err := m.verifier.Verify(c.Request().Context(), rawIDToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to verify ID token")
	}

	// Extract user info
	var userInfo UserInfo
	if err := idToken.Claims(&userInfo); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to parse claims")
	}

	// Validate hosted domain
	if len(m.config.AllowedHostedDomains) > 0 {
		if !m.isHostedDomainAllowed(userInfo.HostedDomain) {
			return echo.NewHTTPError(http.StatusForbidden,
				fmt.Sprintf("Domain '%s' is not allowed", userInfo.HostedDomain))
		}
	}

	// Store user in session
	userJSON, err := json.Marshal(userInfo)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to serialize user info")
	}

	m.setSessionCookie(c, m.config.SessionCookieName,
		base64.StdEncoding.EncodeToString(userJSON),
		m.config.SessionMaxAge)

	// Redirect to success page
	redirectURL := m.config.SuccessRedirect
	if redirectURL == "" {
		redirectURL = "/"
	}

	return c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// handleLogout clears the session
func (m *Middleware) handleLogout(c echo.Context) error {
	m.clearCookie(c, m.config.SessionCookieName)
	return c.Redirect(http.StatusTemporaryRedirect, "/")
}

// isHostedDomainAllowed checks if the hosted domain is in the allowed list
func (m *Middleware) isHostedDomainAllowed(domain string) bool {
	if domain == "" {
		return false
	}

	for _, allowed := range m.config.AllowedHostedDomains {
		if strings.EqualFold(domain, allowed) {
			return true
		}
	}
	return false
}

// getUserFromSession retrieves user info from the session cookie
func (m *Middleware) getUserFromSession(c echo.Context) (*UserInfo, error) {
	cookie, err := c.Cookie(m.config.SessionCookieName)
	if err != nil {
		return nil, err
	}

	userJSON, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil, err
	}

	var userInfo UserInfo
	if err := json.Unmarshal(userJSON, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// setSessionCookie sets a session cookie
func (m *Middleware) setSessionCookie(c echo.Context, name, value string, maxAge int) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: m.config.CookieHTTPOnly,
		Secure:   m.config.CookieSecure,
		SameSite: m.config.CookieSameSite,
	}
	c.SetCookie(cookie)
}

// clearCookie removes a cookie
func (m *Middleware) clearCookie(c echo.Context, name string) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.config.CookieSecure,
		SameSite: m.config.CookieSameSite,
	}
	c.SetCookie(cookie)
}

// generateRandomState generates a random state string for CSRF protection
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// getRedirectURL generates the redirect URL from echo.Context when RedirectPath is set
// Otherwise returns the static RedirectURL
func (m *Middleware) getRedirectURL(c echo.Context) string {
	if m.config.RedirectPath != "" {
		scheme := m.getScheme(c)
		host := m.getHost(c)

		// Validate host if AllowedRedirectHosts is configured
		if len(m.config.AllowedRedirectHosts) > 0 {
			if !m.isHostAllowed(host) {
				// Fall back to first allowed host if current host is not allowed
				host = m.config.AllowedRedirectHosts[0]
			}
		}

		return fmt.Sprintf("%s://%s%s", scheme, host, m.config.RedirectPath)
	}
	return m.config.RedirectURL
}

// getOAuth2Config returns a per-request copy of the OAuth2 config
// This avoids data races when using dynamic RedirectPath
func (m *Middleware) getOAuth2Config(c echo.Context) *oauth2.Config {
	// Create a shallow copy of the oauth2 config
	cfg := *m.oauth2Config

	// Update redirect URL if using dynamic RedirectPath
	if m.config.RedirectPath != "" {
		cfg.RedirectURL = m.getRedirectURL(c)
	}

	return &cfg
}

// getScheme determines the scheme (http/https) from the request
func (m *Middleware) getScheme(c echo.Context) string {
	// Only trust forwarded headers if explicitly configured
	if m.config.TrustForwardedHeaders {
		// Try RFC7239 Forwarded header first
		if proto := m.parseForwardedProto(c.Request().Header.Get("Forwarded")); proto != "" {
			return proto
		}

		// Fall back to X-Forwarded-Proto (Echo's c.Scheme() handles this)
		return c.Scheme()
	}

	// When not trusting forwarded headers, check TLS directly
	if c.Request().TLS != nil {
		return "https"
	}
	return "http"
}

// getHost determines the host from the request
func (m *Middleware) getHost(c echo.Context) string {
	// Only trust forwarded headers if explicitly configured
	if m.config.TrustForwardedHeaders {
		// Try X-Forwarded-Host first
		if host := m.getFirstHeaderValue(c.Request().Header.Get("X-Forwarded-Host")); host != "" {
			return m.sanitizeHost(host)
		}

		// Try RFC7239 Forwarded header
		if host := m.parseForwardedHost(c.Request().Header.Get("Forwarded")); host != "" {
			return m.sanitizeHost(host)
		}
	}

	// Fall back to Request.Host
	return m.sanitizeHost(c.Request().Host)
}

// getFirstHeaderValue splits a comma-separated header value and returns the first non-empty token
func (m *Middleware) getFirstHeaderValue(header string) string {
	if header == "" {
		return ""
	}

	parts := strings.Split(header, ",")
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// parseForwardedProto extracts the proto parameter from RFC7239 Forwarded header
func (m *Middleware) parseForwardedProto(forwarded string) string {
	if forwarded == "" {
		return ""
	}

	// Split by comma for multiple proxies, use first
	parts := strings.Split(forwarded, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Look for proto= parameter
		for _, param := range strings.Split(part, ";") {
			param = strings.TrimSpace(param)
			if strings.HasPrefix(strings.ToLower(param), "proto=") {
				proto := strings.TrimPrefix(param[6:], "\"")
				proto = strings.TrimSuffix(proto, "\"")
				proto = strings.ToLower(strings.TrimSpace(proto))
				if proto == "http" || proto == "https" {
					return proto
				}
			}
		}
	}
	return ""
}

// parseForwardedHost extracts the host parameter from RFC7239 Forwarded header
func (m *Middleware) parseForwardedHost(forwarded string) string {
	if forwarded == "" {
		return ""
	}

	// Split by comma for multiple proxies, use first
	parts := strings.Split(forwarded, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Look for host= parameter
		for _, param := range strings.Split(part, ";") {
			param = strings.TrimSpace(param)
			if strings.HasPrefix(strings.ToLower(param), "host=") {
				host := strings.TrimPrefix(param[5:], "\"")
				host = strings.TrimSuffix(host, "\"")
				return strings.TrimSpace(host)
			}
		}
	}
	return ""
}

// sanitizeHost performs basic validation and sanitization on the host
func (m *Middleware) sanitizeHost(host string) string {
	// Remove any whitespace
	host = strings.TrimSpace(host)

	// Basic validation: ensure no control characters or invalid URL characters
	for _, ch := range host {
		if ch < 32 || ch == 127 || strings.ContainsRune("<>\"\\{}|^`", ch) {
			return "localhost" // Return safe default on invalid host
		}
	}

	return host
}

// isHostAllowed checks if the host is in the AllowedRedirectHosts list
func (m *Middleware) isHostAllowed(host string) bool {
	for _, allowed := range m.config.AllowedRedirectHosts {
		if strings.EqualFold(host, allowed) {
			return true
		}
	}
	return false
}
