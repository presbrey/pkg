# Google OpenID Echo Middleware - Complete Implementation

A comprehensive Echo middleware for Google OpenID Connect authentication with Google Workspace hosted domain restrictions.

## 📦 Package Structure

```
google-openid-middleware/
├── middleware.go      # Main implementation
├── go.mod            # Module dependencies
└── example/
    └── main.go       # Usage example
```

---

## 🔑 Key Features

1. **Hosted Domain Restriction** - Restrict authentication to specific Google Workspace domains
2. **Secure by Default** - HTTP-only cookies, CSRF protection, ID token verification
3. **Easy Integration** - Simple Echo middleware pattern
4. **Fully Configurable** - Customize paths, cookies, redirects, and handlers
5. **Type-Safe** - Access user info with strongly-typed structs
6. **Production Ready** - Comprehensive error handling and validation

---

## 📝 Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `ClientID` | string | *required* | Google OAuth2 client ID |
| `ClientSecret` | string | *required* | Google OAuth2 client secret |
| `RedirectURL` | string | *required*\* | OAuth2 callback URL (static) |
| `RedirectPath` | string | *required*\* | OAuth2 callback path (dynamic) - generates full URL from request context |
| `AllowedHostedDomains` | []string | `nil` | List of allowed Google Workspace domains |
| `Scopes` | []string | `["openid", "email", "profile"]` | OAuth2 scopes to request |
| `SessionCookieName` | string | `"google_openid_session"` | Session cookie name |
| `SessionMaxAge` | int | `86400` | Session cookie max age (seconds) |
| `CookieSecure` | bool | `false` | Set Secure flag on cookies |
| `CookieHTTPOnly` | bool | `true` | Set HttpOnly flag (always true) |
| `CookieSameSite` | http.SameSite | `Lax` | SameSite cookie attribute |
| `LoginPath` | string | `"/auth/google/login"` | Login initiation path |
| `CallbackPath` | string | `"/auth/google/callback"` | OAuth2 callback path |
| `LogoutPath` | string | `"/auth/google/logout"` | Logout path |
| `SuccessRedirect` | string | `"/"` | Redirect URL after successful auth |
| `UnauthorizedHandler` | echo.HandlerFunc | `nil` | Custom unauthorized handler |

\* Either `RedirectURL` or `RedirectPath` is required, but not both.

---

## 🔄 Dynamic vs Static Redirect URLs

The middleware supports two ways to configure the OAuth2 callback URL:

### Static RedirectURL
Use a fixed, absolute URL that doesn't change:
```go
RedirectURL: "https://example.com/auth/google/callback"
```
Best for single-domain applications with a known URL.

### Dynamic RedirectPath
Use a relative path that generates the full URL from the incoming request:
```go
RedirectPath: "/auth/google/callback"
```
The middleware automatically detects:
- **Scheme**: `http` or `https` (from TLS or X-Forwarded-Proto header)
- **Host**: From the request's Host header
- **Path**: Your configured path

**Benefits:**
- Works seamlessly with multiple domains (e.g., production, staging, development)
- Automatically adapts to proxy/load balancer schemes (via X-Forwarded-Proto)
- No hardcoded URLs - perfect for containerized/cloud deployments
- Simplifies configuration across different environments

**Example:** When using `RedirectPath: "/auth/google/callback"`:
- Request to `http://localhost:8080` → Callback: `http://localhost:8080/auth/google/callback`
- Request to `https://example.com` → Callback: `https://example.com/auth/google/callback`
- Behind proxy with X-Forwarded-Proto → Uses forwarded scheme automatically

---

## 🔐 Security Features

- ✅ CSRF protection with cryptographically random state parameter
- ✅ ID token cryptographic verification using Google's public keys
- ✅ HTTP-only secure cookies to prevent XSS attacks
- ✅ Hosted domain validation from ID token claims
- ✅ SameSite cookie protection against CSRF
- ✅ Automatic token expiration handling

---

## 📊 User Information Structure

```go
type UserInfo struct {
    Sub           string `json:"sub"`            // Google user ID (unique)
    Email         string `json:"email"`          // Email address
    EmailVerified bool   `json:"email_verified"` // Email verification status
    Name          string `json:"name"`           // Full name
    Picture       string `json:"picture"`        // Profile picture URL
    GivenName     string `json:"given_name"`     // First name
    FamilyName    string `json:"family_name"`    // Last name
    HostedDomain  string `json:"hd"`             // Google Workspace domain
}
```

---

## 🎯 Domain Restriction Details

The middleware validates the `hd` (hosted domain) claim in the Google ID token:

- Only Google Workspace accounts have a hosted domain
- Personal Gmail accounts (@gmail.com) don't have this claim
- Domain comparison is case-insensitive
- If `AllowedHostedDomains` is empty, any Google account can authenticate
- Users from unauthorized domains receive a 403 Forbidden error

**Important:** The domain check happens server-side using the verified ID token, not just the email address, ensuring it cannot be spoofed.

---

## 💡 Best Practices

1. **Use HTTPS in production** - Set `CookieSecure: true`
2. **Rotate session secrets** - Implement session key rotation for long-running apps
3. **Set appropriate session timeout** - Balance security and user experience
4. **Monitor failed auth attempts** - Log and alert on authorization failures
5. **Implement rate limiting** - Prevent brute force attacks on auth endpoints
6. **Use environment variables** - Never commit credentials to source control
