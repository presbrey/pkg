package admind

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/presbrey/pkg/irc"
	"golang.org/x/oauth2"
)

type Server struct {
	*irc.Server

	httpServer   *http.Server
	oidcProvider *oidc.Provider
	oidcVerifier *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
}

// startAdminServer starts the HTTP server for admin analytics
func (s *Server) StartAdminServer() error {
	// Set up routes
	mux := http.NewServeMux()

	// Add admin routes
	mux.HandleFunc("/", s.handleAdminHome)
	mux.HandleFunc("/stats", s.handleAdminStats)
	mux.HandleFunc("/channels", s.handleAdminChannels)
	mux.HandleFunc("/clients", s.handleAdminClients)
	mux.HandleFunc("/login", s.handleAdminLogin)
	mux.HandleFunc("/callback", s.handleOIDCCallback)
	mux.HandleFunc("/api/stats", s.handleAPIStats)

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:    s.Config.AdminBindAddr,
		Handler: s.authMiddleware(mux),
	}

	// Start HTTP server in a goroutine
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return nil
}

// initOIDC initializes the OIDC provider and verifier
func (s *Server) initOIDC() error {
	ctx := context.Background()
	config := s.Config

	// Initialize OIDC provider
	provider, err := oidc.NewProvider(ctx, config.OIDCIssuer)
	if err != nil {
		return fmt.Errorf("failed to initialize OIDC provider: %w", err)
	}

	s.oidcProvider = provider
	s.oidcVerifier = provider.Verifier(&oidc.Config{
		ClientID: config.OIDCClientID,
	})

	// Set up OAuth2 config
	s.oauth2Config = &oauth2.Config{
		ClientID:     config.OIDCClientID,
		ClientSecret: config.OIDCClientSecret,
		RedirectURL:  config.OIDCRedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	return nil
}
