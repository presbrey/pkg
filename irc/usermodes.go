package irc

import (
	"fmt"
	"log"
	"reflect"
	"strings"
)

// UserMode represents the user modes for an IRC client
type UserMode struct {
	// Basic user modes
	Away       bool `mode:"a" desc:"away"`
	Invisible  bool `mode:"i" desc:"invisible - hidden from /WHO and /NAMES if queried by someone outside the channel"`
	Wallops    bool `mode:"w" desc:"can listen to wallops messages"`
	Registered bool `mode:"r" desc:"registered nick - set by services"`
	Operator   bool `mode:"o" desc:"IRC Operator - set by server"`
	Notice     bool `mode:"s" desc:"server notices for IRCOps"`

	// Unreal user modes
	Bot            bool `mode:"B" desc:"marks you as being a bot"`
	Deaf           bool `mode:"d" desc:"cannot receive channel PRIVMSG's except for messages with command prefix"`
	PrivDeaf       bool `mode:"D" desc:"cannot receive private messages except from IRCOps, servers and services"`
	Censor         bool `mode:"G" desc:"swear filter - filters out bad words configured in Badword block"`
	HideOper       bool `mode:"H" desc:"hide IRCOp status - IRCOp-only"`
	HideIdle       bool `mode:"I" desc:"hide idle time in /WHOIS"`
	Mute           bool `mode:"M" desc:"mute - cannot send to channels"`
	Privacy        bool `mode:"p" desc:"hide channels you are in from /WHOIS"`
	NoKick         bool `mode:"q" desc:"unkickable (only by U-lines) - IRCOp-only"`
	RegOnlyMsg     bool `mode:"R" desc:"only receive private messages from registered users"`
	ServiceBot     bool `mode:"S" desc:"user is a services bot - services-only"`
	VHost          bool `mode:"t" desc:"indicates you are using a /VHOST - set by server"`
	NoCTCP         bool `mode:"T" desc:"prevents you from receiving CTCP's"`
	ShowWhois      bool `mode:"W" desc:"lets you see when people do a /WHOIS on you - IRCOp-only"`
	HostHiding     bool `mode:"x" desc:"gives you a hidden/cloaked hostname"`
	SecuredOnlyMsg bool `mode:"Z" desc:"allows only users on a secure connection to send you private messages"`
	SSL            bool `mode:"z" desc:"indicates you are connected via SSL/TLS - set by server"`

	// Additional modes
	ServerOnly     bool `mode:"O" desc:"server-only operator"`
	Admin          bool `mode:"A" desc:"server administrator"`
	BotDetect      bool `mode:"b" desc:"bot-detection notices"`
	CommonChans    bool `mode:"c" desc:"only allow PRIVMSG from shared channels"`
	CoAdmin        bool `mode:"C" desc:"service co-administrator"`
	External       bool `mode:"e" desc:"server connect/disconnect notices"`
	Floods         bool `mode:"f" desc:"I-line/full notices"`
	Globops        bool `mode:"g" desc:"global operator notices"`
	Helper         bool `mode:"h" desc:"helper/service specialist flag"`
	RejectedClient bool `mode:"j" desc:"rejected client notices"`
	Service        bool `mode:"k" desc:"protected service flag"`
	Locops         bool `mode:"l" desc:"local oper notices"`
	SpamBots       bool `mode:"m" desc:"spam-bot notices"`
	NickChanges    bool `mode:"n" desc:"nick changes"`
	Unauth         bool `mode:"u" desc:"unauthorized client notices"`
	HostHidingAlt  bool `mode:"v" desc:"hide your host (alternative)"`
	WebTV          bool `mode:"V" desc:"connected via WebTV client"`
	StatsLinks     bool `mode:"y" desc:"stats/links notices"`
}

// ParseModeString parses an IRC mode string (e.g., "+aw-i") and applies it to the UserMode struct
func (m *UserMode) ParseModeString(modeString string) error {
	if modeString == "" {
		return nil
	}

	// Parse mode string character by character
	var add bool = true // Default to adding modes

	for _, ch := range modeString {
		if ch == '+' {
			add = true
			continue
		} else if ch == '-' {
			add = false
			continue
		}

		// Try to set the mode
		if err := m.setModeByChar(rune(ch), add); err != nil {
			// Just log a warning and continue if an unsupported mode is encountered
			log.Printf("Warning: Unsupported mode '%c' in string '%s'", ch, modeString)
		}
	}

	return nil
}

// setModeByChar sets a specific mode character on the UserMode struct
func (m *UserMode) setModeByChar(mode rune, value bool) error {
	val := reflect.ValueOf(m).Elem()
	typ := val.Type()

	// Look through all fields in the struct
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		modeTag := fieldType.Tag.Get("mode")

		// If this field has the matching mode tag
		if modeTag == string(mode) {
			if field.Kind() == reflect.Bool {
				field.SetBool(value)
				return nil
			}
		}
	}

	return fmt.Errorf("no field found for mode %c", mode)
}

// String returns the compact mode string representation (e.g., "+awi")
func (m *UserMode) String() string {
	modeStr := "+"
	val := reflect.ValueOf(m).Elem()
	typ := val.Type()

	// Process fields in the struct
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if !field.Bool() {
			continue // Skip if the mode is not set
		}

		fieldType := typ.Field(i)
		modeTag := fieldType.Tag.Get("mode")

		// Only include primary mode flags
		if modeTag != "" {
			modeStr += modeTag
		}
	}

	// If only + was added, return empty string
	if modeStr == "+" {
		return ""
	}

	return modeStr
}

// GetModeDescription returns a human-readable description of all set modes
func (m *UserMode) GetModeDescription() string {
	var descriptions []string
	val := reflect.ValueOf(m).Elem()
	typ := val.Type()

	// Process fields in the struct
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if !field.Bool() {
			continue // Skip if the mode is not set
		}

		fieldType := typ.Field(i)
		modeTag := fieldType.Tag.Get("mode")
		descTag := fieldType.Tag.Get("desc")

		// Only include primary mode flags
		if modeTag != "" && descTag != "" {
			descriptions = append(descriptions, fmt.Sprintf("+%s (%s)", modeTag, descTag))
		}
	}

	if len(descriptions) == 0 {
		return "No modes set"
	}

	return strings.Join(descriptions, ", ")
}

// ApplyMode applies a single mode change (char with + or - prefix)
func (m *UserMode) ApplyMode(modeChar rune, add bool) error {
	return m.setModeByChar(modeChar, add)
}

// HasMode checks if a specific mode is set
func (m *UserMode) HasMode(modeChar rune) bool {
	val := reflect.ValueOf(m).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		modeTag := fieldType.Tag.Get("mode")

		// If this field has the matching mode tag
		if modeTag == string(modeChar) {
			return field.Bool()
		}
	}

	return false
}
