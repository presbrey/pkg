package server

import (
	"fmt"
	"sync"
	"time"
)

// Channel represents an IRC channel
type Channel struct {
	Name          string
	Topic         string
	TopicSetBy    string
	TopicSetAt    time.Time
	Members       map[string]*Client
	Operators     map[string]bool
	Voices        map[string]bool
	Halfops       map[string]bool
	Owners        map[string]bool
	Admins        map[string]bool
	Modes         ChannelModes
	BanList       []string
	InviteList    []string
	ExceptionList []string
	Server        *Server
	mu            sync.RWMutex
}

// ChannelModes represents the modes of a channel
type ChannelModes struct {
	// Basic modes
	Private                bool // p - Private channel (+p)
	Secret                 bool // s - Secret channel (+s)
	InviteOnly             bool // i - Invite-only channel (+i)
	NoExternalMsgs         bool // n - No messages from outside (+n)
	Moderated              bool // m - Moderated channel (+m)
	TopicSettableByOpsOnly bool // t - Topic settable by channel operators only (+t)

	// Extended modes (UnrealIRCd compatible)
	NoColors        bool // c - No colors allowed (+c)
	NoCtcp          bool // C - No CTCPs allowed (+C)
	DelayJoin       bool // D - Delayed /JOIN show (users are not shown to be in the channel until they speak) (+D)
	FloodProtection bool // f - Channel flood protection (+f)
	Permanent       bool // P - Permanent channel (+P)
	RegOnly         bool // R - Only registered users can join (+R)
	NoKnock         bool // K - No /KNOCK allowed (+K)
	NoNickChange    bool // N - No nickname changes while in channel (+N)
	StripColors     bool // S - Strip colors from channel messages (+S)

	// Limit
	UserLimit int // l - User limit (+l)

	// Keys
	Key string // k - Channel key (password) (+k)

	// Custom setable mode parameters (used for flood protection, etc.)
	ModeParams map[string]string
}

// NewChannel creates a new channel
type ChannelMember struct {
	Client *Client
	Modes  map[rune]bool // op, voice, etc.
}

// NewChannel creates a new channel
func NewChannel(server *Server, name string) *Channel {
	c := &Channel{
		Name:          name,
		Server:        server,
		Members:       make(map[string]*Client),
		Operators:     make(map[string]bool),
		Voices:        make(map[string]bool),
		Halfops:       make(map[string]bool),
		Owners:        make(map[string]bool),
		Admins:        make(map[string]bool),
		BanList:       make([]string, 0),
		InviteList:    make([]string, 0),
		ExceptionList: make([]string, 0),
		Modes:         DefaultChannelModes(),
	}
	return c
}

// DefaultChannelModes returns the default channel modes
func DefaultChannelModes() ChannelModes {
	return ChannelModes{
		NoExternalMsgs:         true, // +n by default
		TopicSettableByOpsOnly: true, // +t by default
		ModeParams:             make(map[string]string),
	}
}

// AddMember adds a client to the channel
func (c *Channel) AddMember(client *Client) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Members[client.Nickname] = client
}

// RemoveMember removes a client from the channel
func (c *Channel) RemoveMember(client *Client) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.Members, client.Nickname)
}

// GetMember gets a client by nickname
func (c *Channel) GetMember(nickname string) *Client {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Members[nickname]
}

// MemberCount returns the number of members in the channel
func (c *Channel) MemberCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.Members)
}

// SendToAll sends a message to all members of the channel
func (c *Channel) SendToAll(message string, except *Client) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, member := range c.Members {
		if except != nil && member.ID == except.ID {
			continue
		}
		member.SendRaw(message)
	}
}

// SetTopic sets the channel topic
func (c *Channel) SetTopic(topic, setBy string) {
	c.mu.Lock()
	c.Topic = topic
	c.TopicSetBy = setBy
	c.TopicSetAt = time.Now()
	c.mu.Unlock()
}

// GetTopic gets the channel topic
func (c *Channel) GetTopic() (string, string, time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Topic, c.TopicSetBy, c.TopicSetAt
}

// SendNames sends the names list to a client
func (c *Channel) SendNames(client *Client) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Build the names list
	var names string
	for _, member := range c.Members {
		prefix := ""
		// Add prefix for operators
		if member.IsOper {
			prefix = "@"
		}
		names += prefix + member.Nickname + " "
	}

	// Send the names list
	client.SendMessage(c.Server.GetConfig().Server.Name, "353", client.Nickname, "=", c.Name, names)
	client.SendMessage(c.Server.GetConfig().Server.Name, "366", client.Nickname, c.Name, "End of /NAMES list")
}

// SetMode sets a mode for the channel
func (c *Channel) SetMode(mode rune, enable bool, param string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Handle the mode
	switch mode {
	case 'p':
		c.Modes.Private = enable
	case 's':
		c.Modes.Secret = enable
	case 'i':
		c.Modes.InviteOnly = enable
	case 'n':
		c.Modes.NoExternalMsgs = enable
	case 'm':
		c.Modes.Moderated = enable
	case 't':
		c.Modes.TopicSettableByOpsOnly = enable
	case 'c':
		c.Modes.NoColors = enable
	case 'C':
		c.Modes.NoCtcp = enable
	case 'D':
		c.Modes.DelayJoin = enable
	case 'f':
		c.Modes.FloodProtection = enable
		if enable && param != "" {
			c.Modes.ModeParams["f"] = param
		} else {
			delete(c.Modes.ModeParams, "f")
		}
	case 'P':
		c.Modes.Permanent = enable
	case 'R':
		c.Modes.RegOnly = enable
	case 'K':
		c.Modes.NoKnock = enable
	case 'N':
		c.Modes.NoNickChange = enable
	case 'S':
		c.Modes.StripColors = enable
	case 'l':
		if enable && param != "" {
			var limit int
			fmt.Sscanf(param, "%d", &limit)
			c.Modes.UserLimit = limit
		} else {
			c.Modes.UserLimit = 0
		}
	case 'k':
		if enable && param != "" {
			c.Modes.Key = param
		} else {
			c.Modes.Key = ""
		}
	}
}

// GetModeString returns the mode string for the channel
func (c *Channel) GetModeString() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Start with a plus sign for set modes
	modeStr := "+"
	modeParams := ""

	// Basic modes
	if c.Modes.Private {
		modeStr += "p"
	}
	if c.Modes.Secret {
		modeStr += "s"
	}
	if c.Modes.InviteOnly {
		modeStr += "i"
	}
	if c.Modes.NoExternalMsgs {
		modeStr += "n"
	}
	if c.Modes.Moderated {
		modeStr += "m"
	}
	if c.Modes.TopicSettableByOpsOnly {
		modeStr += "t"
	}

	// Extended modes
	if c.Modes.NoColors {
		modeStr += "c"
	}
	if c.Modes.NoCtcp {
		modeStr += "C"
	}
	if c.Modes.DelayJoin {
		modeStr += "D"
	}
	if c.Modes.FloodProtection {
		modeStr += "f"
		modeParams += " " + c.Modes.ModeParams["f"]
	}
	if c.Modes.Permanent {
		modeStr += "P"
	}
	if c.Modes.RegOnly {
		modeStr += "R"
	}
	if c.Modes.NoKnock {
		modeStr += "K"
	}
	if c.Modes.NoNickChange {
		modeStr += "N"
	}
	if c.Modes.StripColors {
		modeStr += "S"
	}

	// Limit
	if c.Modes.UserLimit > 0 {
		modeStr += "l"
		modeParams += fmt.Sprintf(" %d", c.Modes.UserLimit)
	}

	// Key
	if c.Modes.Key != "" {
		modeStr += "k"
		modeParams += " " + c.Modes.Key
	}

	if len(modeStr) <= 1 {
		// No modes set, just return empty string
		return ""
	}

	// Return the complete mode string
	return modeStr + modeParams
}

// IsMember checks if a client is a member of the channel
func (c *Channel) IsMember(client *Client) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.Members[client.Nickname]
	return ok
}

// AddBan adds a ban to the ban list
func (c *Channel) AddBan(mask string, setBy string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.BanList = append(c.BanList, mask)
}

// RemoveBan removes a ban from the ban list
func (c *Channel) RemoveBan(mask string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, ban := range c.BanList {
		if ban == mask {
			c.BanList = append(c.BanList[:i], c.BanList[i+1:]...)
			break
		}
	}
}

// IsBanned checks if a client is banned from the channel
func (c *Channel) IsBanned(client *Client) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// TODO: Implement mask matching
	return false
}

// AddInvite adds a client to the invite list
func (c *Channel) AddInvite(nickname string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.InviteList = append(c.InviteList, nickname)
}

// RemoveInvite removes a client from the invite list
func (c *Channel) RemoveInvite(nickname string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, nick := range c.InviteList {
		if nick == nickname {
			c.InviteList = append(c.InviteList[:i], c.InviteList[i+1:]...)
			break
		}
	}
}

// IsInvited checks if a client is invited to the channel
func (c *Channel) IsInvited(client *Client) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, nick := range c.InviteList {
		if nick == client.Nickname {
			return true
		}
	}
	return false
}

// Kick kicks a client from the channel
func (c *Channel) Kick(client *Client, target *Client, reason string) {
	// Send kick message to all members
	c.SendToAll(fmt.Sprintf(":%s!%s@%s KICK %s %s :%s", client.Nickname, client.Username, client.Hostname, c.Name, target.Nickname, reason), nil)

	// Remove the target from the channel
	c.RemoveMember(target)

	// Remove the channel from the target's channel list
	target.mu.Lock()
	delete(target.Channels, c.Name)
	target.mu.Unlock()
}
