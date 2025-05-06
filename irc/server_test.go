package irc

import (
	"fmt"
	"net"
)

// GrantOperatorForTest is a special method used only in integration tests
// to grant operator status to a user without going through the OIDC flow
// This should never be used in production code
func (s *Server) GrantOperatorForTest(nickname string) error {
	s.Lock()
	defer s.Unlock()

	client, exists := s.clients[nickname]
	if !exists {
		return fmt.Errorf("client with nickname %s not found", nickname)
	}

	client.Lock()
	client.Modes.Operator = true
	client.email = "test@example.com" // Test email
	client.Unlock()

	return nil
}

func (s *Server) TestingGetListener() net.Listener {
	return s.listener
}
