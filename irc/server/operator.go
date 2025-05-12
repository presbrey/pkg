package server

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// Operator represents an IRC operator
type Operator struct {
	Username    string
	Password    string
	Email       string
	Mask        string
	LastLogin   time.Time
	MagicTokens map[string]time.Time
	mu          sync.RWMutex
}

// NewOperator creates a new operator
func NewOperator(username, password, email, mask string) *Operator {
	return &Operator{
		Username:    username,
		Password:    password,
		Email:       email,
		Mask:        mask,
		MagicTokens: make(map[string]time.Time),
	}
}

// CreateMagicToken creates a magic token for web authentication
func (o *Operator) CreateMagicToken() (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Generate a random token
	tokenBytes := make([]byte, 32)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %v", err)
	}

	// Encode the token as base64
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Store the token with expiration time (24 hours)
	o.MagicTokens[token] = time.Now().Add(24 * time.Hour)

	// Clean up expired tokens
	o.cleanupExpiredTokens()

	return token, nil
}

// ValidateMagicToken validates a magic token
func (o *Operator) ValidateMagicToken(token string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Check if the token exists and is not expired
	expirationTime, exists := o.MagicTokens[token]
	if !exists || time.Now().After(expirationTime) {
		return false
	}

	// Remove the token after use
	delete(o.MagicTokens, token)

	return true
}

// cleanupExpiredTokens removes expired tokens
func (o *Operator) cleanupExpiredTokens() {
	now := time.Now()
	for token, expiration := range o.MagicTokens {
		if now.After(expiration) {
			delete(o.MagicTokens, token)
		}
	}
}

// UpdateLastLogin updates the last login time
func (o *Operator) UpdateLastLogin() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.LastLogin = time.Now()
}

// CheckPassword checks if the password is correct
func (o *Operator) CheckPassword(password string) bool {
	return o.Password == password
}

// SendMagicLink sends a magic link to the operator via IRC
func (o *Operator) SendMagicLink(client *Client, webPortalURL string) (string, error) {
	// Create a magic token
	token, err := o.CreateMagicToken()
	if err != nil {
		return "", err
	}

	// Construct the magic link
	magicLink := fmt.Sprintf("%s/login?token=%s&username=%s", webPortalURL, token, o.Username)

	// Send the link to the client
	client.SendMessage(client.Server.GetConfig().Server.Name, "NOTICE", client.Nickname, fmt.Sprintf("Your magic login link: %s (valid for 24 hours)", magicLink))

	return token, nil
}
