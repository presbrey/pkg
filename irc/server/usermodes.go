package server

import (
	"sync"
)

// UserModes represents the modes of a user
type UserModes struct {
	// Basic modes
	Invisible     bool // i - Invisible (+i)
	Operator      bool // o - IRC Operator (+o)
	LocalOperator bool // O - Local Operator (+O)
	ServerNotices bool // s - Receives server notices (+s)
	Wallops       bool // w - Receives wallops (+w)

	// Extended modes (UnrealIRCd compatible)
	Services       bool // z - Connected via services (+z)
	Protected      bool // S - User is protected (+S)
	NoKnock        bool // k - User doesn't receive KNOCK notices (+k)
	NoWhois        bool // p - Blocks details for whois requests (+p)
	RegisteredNick bool // r - Registered user (+r)
	WebIrc         bool // W - WebIRC user (+W)
	HideIdle       bool // I - Hides idle time in WHOIS (+I)
	AllowFilter    bool // G - Allow filter bypass (+G)
	NoCtcp         bool // C - No CTCPs (+C)

	// Custom modes
	customModes map[rune]bool
	mu          sync.RWMutex
}

// NewUserModes creates a new user modes instance
func NewUserModes() UserModes {
	return UserModes{
		customModes: make(map[rune]bool),
	}
}

// SetMode sets a mode
func (m *UserModes) SetMode(mode rune) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch mode {
	case 'i':
		m.Invisible = true
	case 'o':
		m.Operator = true
	case 'O':
		m.LocalOperator = true
	case 's':
		m.ServerNotices = true
	case 'w':
		m.Wallops = true
	case 'z':
		m.Services = true
	case 'S':
		m.Protected = true
	case 'k':
		m.NoKnock = true
	case 'p':
		m.NoWhois = true
	case 'r':
		m.RegisteredNick = true
	case 'W':
		m.WebIrc = true
	case 'I':
		m.HideIdle = true
	case 'G':
		m.AllowFilter = true
	case 'C':
		m.NoCtcp = true
	default:
		m.customModes[mode] = true
	}
}

// UnsetMode unsets a mode
func (m *UserModes) UnsetMode(mode rune) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch mode {
	case 'i':
		m.Invisible = false
	case 'o':
		m.Operator = false
	case 'O':
		m.LocalOperator = false
	case 's':
		m.ServerNotices = false
	case 'w':
		m.Wallops = false
	case 'z':
		m.Services = false
	case 'S':
		m.Protected = false
	case 'k':
		m.NoKnock = false
	case 'p':
		m.NoWhois = false
	case 'r':
		m.RegisteredNick = false
	case 'W':
		m.WebIrc = false
	case 'I':
		m.HideIdle = false
	case 'G':
		m.AllowFilter = false
	case 'C':
		m.NoCtcp = false
	default:
		delete(m.customModes, mode)
	}
}

// HasMode checks if a mode is set
func (m *UserModes) HasMode(mode rune) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch mode {
	case 'i':
		return m.Invisible
	case 'o':
		return m.Operator
	case 'O':
		return m.LocalOperator
	case 's':
		return m.ServerNotices
	case 'w':
		return m.Wallops
	case 'z':
		return m.Services
	case 'S':
		return m.Protected
	case 'k':
		return m.NoKnock
	case 'p':
		return m.NoWhois
	case 'r':
		return m.RegisteredNick
	case 'W':
		return m.WebIrc
	case 'I':
		return m.HideIdle
	case 'G':
		return m.AllowFilter
	case 'C':
		return m.NoCtcp
	default:
		return m.customModes[mode]
	}
}

// GetModeString returns the mode string
func (m *UserModes) GetModeString() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	modeStr := "+"

	// Basic modes
	if m.Invisible {
		modeStr += "i"
	}
	if m.Operator {
		modeStr += "o"
	}
	if m.LocalOperator {
		modeStr += "O"
	}
	if m.ServerNotices {
		modeStr += "s"
	}
	if m.Wallops {
		modeStr += "w"
	}

	// Extended modes
	if m.Services {
		modeStr += "z"
	}
	if m.Protected {
		modeStr += "S"
	}
	if m.NoKnock {
		modeStr += "k"
	}
	if m.NoWhois {
		modeStr += "p"
	}
	if m.RegisteredNick {
		modeStr += "r"
	}
	if m.WebIrc {
		modeStr += "W"
	}
	if m.HideIdle {
		modeStr += "I"
	}
	if m.AllowFilter {
		modeStr += "G"
	}
	if m.NoCtcp {
		modeStr += "C"
	}

	// Custom modes
	for mode := range m.customModes {
		modeStr += string(mode)
	}

	return modeStr
}
