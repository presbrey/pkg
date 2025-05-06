package admind

import (
	"regexp"
	"strconv"
	"time"

	"github.com/presbrey/pkg/irc"
)

// BanEntry is a reference to the main irc.BanEntry struct
type BanEntry = irc.BanEntry

// isValidHostmask checks if a hostmask is valid
func isValidHostmask(mask string) bool {
	// Simple validation - a more comprehensive implementation would check
	// for proper format like nick!user@host with wildcards
	return len(mask) > 0 && len(mask) < 256 &&
		regexp.MustCompile(`^[^\s]+$`).MatchString(mask)
}

// parseDuration parses a duration string used for bans
// It accepts IRC-like formats like "1d", "2h", "30m" etc.
func parseDuration(s string) (time.Duration, error) {
	// Handle IRC-style durations
	if len(s) > 0 {
		last := s[len(s)-1]
		value := s[:len(s)-1]

		if len(value) > 0 {
			val, err := strconv.Atoi(value)
			if err != nil {
				return 0, err
			}

			switch last {
			case 's':
				return time.Duration(val) * time.Second, nil
			case 'm':
				return time.Duration(val) * time.Minute, nil
			case 'h':
				return time.Duration(val) * time.Hour, nil
			case 'd':
				return time.Duration(val) * time.Hour * 24, nil
			case 'w':
				return time.Duration(val) * time.Hour * 24 * 7, nil
			case 'y':
				return time.Duration(val) * time.Hour * 24 * 365, nil
			}
		}
	}

	// Fallback to standard Go duration parsing
	return time.ParseDuration(s)
}
