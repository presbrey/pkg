package admind

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/labstack/echo/v4"
	"github.com/presbrey/pkg/irc"
	"golang.org/x/oauth2"
)

type Server struct {
	*irc.Server

	echoServer   *echo.Echo
	oidcProvider *oidc.Provider
	oidcVerifier *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	onceSetup    sync.Once
}

func (s *Server) setup() {
	s.onceSetup.Do(func() {
		s.echoServer = echo.New()
		s.route(s.echoServer)
		s.initOIDC()
	})
}

// startAdminServer starts the HTTP server for admin analytics
func (s *Server) StartAdminServer() error {
	s.setup()
	return s.echoServer.Start(s.Config.AdminBindAddr)
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

// RelayPrivmsgToChannel sends a message to all clients in a specified IRC channel.
func (s *Server) RelayPrivmsgToChannel(channelName string, message string) error {
	// s.Server is the embedded irc.Server instance from the admind.Server struct
	if s.Server == nil {
		return fmt.Errorf("IRC server component is not initialized in admind.Server")
	}

	s.Server.RLock() // Lock the IRC server for reading its channels map
	channel, exists := s.Server.GetChannels()[channelName]
	s.Server.RUnlock()

	if !exists {
		return fmt.Errorf("channel %s does not exist", channelName)
	}

	// Determine the sender's prefix. Using the IRC server's configured name.
	var senderName string
	if s.Server.Config != nil && s.Server.Config.ServerName != "" {
		senderName = s.Server.Config.ServerName
	} else {
		senderName = "AdminService" // Fallback name if server name is not configured
		log.Printf("Warning: irc.Server.Config.ServerName is not set. Using default sender '%s' for relayed message.", senderName)
	}

	// Construct the raw PRIVMSG command string
	// Format: :<sender_name> PRIVMSG <target_channel> :<message_content>
	privmsgString := fmt.Sprintf(":%s PRIVMSG %s :%s", senderName, channelName, message)

	channel.RLock()                          // Lock the channel for reading its clients map
	clientsInChannel := channel.GetClients() // Get a copy of the client map
	channel.RUnlock()

	if len(clientsInChannel) == 0 {
		log.Printf("No clients in channel %s to relay message to.", channelName)
		return nil // Not an error, just no one to send to
	}

	for _, client := range clientsInChannel {
		if client != nil { // Safety check
			// Corrected to use new exported SendRawMessage method
			// SendRawMessage (and its underlying sendRaw) handles errors internally.
			client.SendRawMessage(privmsgString)
			// Potential error logging for sendRaw is handled within sendRaw itself.
		}
	}

	log.Printf("Relayed message to %d clients in channel %s: %s", len(clientsInChannel), channelName, message)
	return nil
}
