package irc

// Capability represents an IRC capability supported by the server
type Capability struct {
	Name        string // The name of the capability as sent to the client
	Description string // Description of what the capability does
	Value       string // Optional value for capabilities that have a value parameter
}

// GetCapabilityString returns the full capability string including optional value
func (c *Capability) GetCapabilityString() string {
	if c.Value != "" {
		return c.Name + "=" + c.Value
	}
	return c.Name
}

// ServerCapabilities defines all the capabilities supported by this server
var ServerCapabilities = map[string]*Capability{
	"multi-prefix": {
		Name:        "multi-prefix",
		Description: "Enables multiple prefix modes in NAMES/WHO replies (@+nick)",
	},
	"away-notify": {
		Name:        "away-notify",
		Description: "Sends automatic AWAY notifications when users change away status",
	},
	"account-notify": {
		Name:        "account-notify",
		Description: "Sends notifications when users identify with services",
	},
	"account-tag": {
		Name:        "account-tag",
		Description: "Adds account information to message tags",
	},
	"extended-join": {
		Name:        "extended-join",
		Description: "Provides account name and real name in JOIN messages",
	},
	"userhost-in-names": {
		Name:        "userhost-in-names",
		Description: "Includes full user@host in NAMES replies",
	},
	"echo-message": {
		Name:        "echo-message",
		Description: "Echoes a user's own messages back to them",
	},
	"invite-notify": {
		Name:        "invite-notify",
		Description: "Notifies channel members when someone is invited",
	},
	"cap-notify": {
		Name:        "cap-notify",
		Description: "Notifies about capability changes without reconnecting",
	},
}

// ClientCapabilities represents the capabilities negotiated and activated for a client
type ClientCapabilities struct {
	Negotiating   bool                // Whether the client is currently negotiating capabilities
	Enabled       map[string]struct{} // Set of enabled capabilities for this client
	RequestedCaps []string            // Capabilities requested in the current negotiation
}

// NewClientCapabilities creates a new client capabilities tracker
func NewClientCapabilities() *ClientCapabilities {
	return &ClientCapabilities{
		Negotiating: false,
		Enabled:     make(map[string]struct{}),
	}
}

// HasCapability checks if a client has a specific capability enabled
func (cc *ClientCapabilities) HasCapability(name string) bool {
	_, has := cc.Enabled[name]
	return has
}

// EnableCapability enables a capability for this client
func (cc *ClientCapabilities) EnableCapability(name string) {
	cc.Enabled[name] = struct{}{}
}

// DisableCapability disables a capability for this client
func (cc *ClientCapabilities) DisableCapability(name string) {
	delete(cc.Enabled, name)
}
