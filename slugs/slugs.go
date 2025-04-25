// Package slugs provides functionality for generating URL-safe slugs.
package slugs

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// SlugGenerator is the main struct for configuring and generating slugs.
type SlugGenerator struct {
	maxLength       int
	delimiter       string
	lowercase       bool
	removeStopWords bool
	stopWords       map[string]bool
	slugType        slugType
	prefix          string
	suffix          string
	randomLength    int
	safePattern     *regexp.Regexp
	multiPattern    *regexp.Regexp
}

type slugType int

const (
	textSlug slugType = iota
	uuidV4Slug
	uuidV7Slug
	nanoidSlug
	randomSlug
)

// New creates a new SlugGenerator with default settings.
func New() *SlugGenerator {
	sg := &SlugGenerator{
		maxLength:       100,
		delimiter:       "-",
		lowercase:       true,
		removeStopWords: false,
		stopWords:       defaultStopWords(),
		slugType:        textSlug,
		randomLength:    8,
	}
	sg.compileRegex()
	return sg
}

// MaxLength sets the maximum length of the generated slug.
func (sg *SlugGenerator) MaxLength(length int) *SlugGenerator {
	sg.maxLength = length
	return sg
}

// Delimiter sets the character used to separate words in the slug.
func (sg *SlugGenerator) Delimiter(delimiter string) *SlugGenerator {
	sg.delimiter = delimiter
	sg.compileRegex()
	return sg
}

// Lowercase sets whether the slug should be converted to lowercase.
func (sg *SlugGenerator) Lowercase(lowercase bool) *SlugGenerator {
	sg.lowercase = lowercase
	return sg
}

// RemoveStopWords sets whether common stop words should be removed from the slug.
func (sg *SlugGenerator) RemoveStopWords(remove bool) *SlugGenerator {
	sg.removeStopWords = remove
	return sg
}

// AddStopWords adds custom stop words to be removed during slug generation.
func (sg *SlugGenerator) AddStopWords(words ...string) *SlugGenerator {
	if sg.stopWords == nil {
		sg.stopWords = make(map[string]bool)
	}
	for _, word := range words {
		sg.stopWords[strings.ToLower(word)] = true
	}
	return sg
}

// WithPrefix adds a prefix to the generated slug.
func (sg *SlugGenerator) WithPrefix(prefix string) *SlugGenerator {
	sg.prefix = prefix
	return sg
}

// WithSuffix adds a suffix to the generated slug.
func (sg *SlugGenerator) WithSuffix(suffix string) *SlugGenerator {
	sg.suffix = suffix
	return sg
}

// UUID sets the generator to create UUID v4-based slugs (for backward compatibility).
func (sg *SlugGenerator) UUID() *SlugGenerator {
	sg.slugType = uuidV4Slug
	return sg
}

// UUIDv4 sets the generator to create UUID v4-based slugs.
func (sg *SlugGenerator) UUIDv4() *SlugGenerator {
	sg.slugType = uuidV4Slug
	return sg
}

// UUIDv7 sets the generator to create UUID v7-based slugs.
func (sg *SlugGenerator) UUIDv7() *SlugGenerator {
	sg.slugType = uuidV7Slug
	return sg
}

// NanoID sets the generator to create NanoID-style slugs.
func (sg *SlugGenerator) NanoID() *SlugGenerator {
	sg.slugType = nanoidSlug
	return sg
}

// Random sets the generator to create random string slugs.
func (sg *SlugGenerator) Random() *SlugGenerator {
	sg.slugType = randomSlug
	return sg
}

// RandomLength sets the length of random slugs.
func (sg *SlugGenerator) RandomLength(length int) *SlugGenerator {
	sg.randomLength = length
	return sg
}

// Generate creates a slug from the given text based on the configured options.
func (sg *SlugGenerator) Generate(text string) string {
	var result string

	switch sg.slugType {
	case textSlug:
		result = sg.generateTextSlug(text)
	case uuidV4Slug:
		result = sg.generateUUIDv4()
	case uuidV7Slug:
		result = sg.generateUUIDv7()
	case nanoidSlug:
		result = sg.generateNanoID()
	case randomSlug:
		result = sg.generateRandomSlug()
	}

	// Apply prefix and suffix
	if sg.prefix != "" {
		result = sg.prefix + sg.delimiter + result
	}
	if sg.suffix != "" {
		result = result + sg.delimiter + sg.suffix
	}

	return result
}

func (sg *SlugGenerator) generateTextSlug(text string) string {
	if text == "" {
		return ""
	}

	// Convert to lowercase if needed
	if sg.lowercase {
		text = strings.ToLower(text)
	}

	// Split into words
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	// Remove stop words if configured
	if sg.removeStopWords {
		filteredWords := make([]string, 0, len(words))
		for _, word := range words {
			if !sg.stopWords[strings.ToLower(word)] {
				filteredWords = append(filteredWords, word)
			}
		}
		words = filteredWords
	}

	// Join words with delimiter
	slug := strings.Join(words, sg.delimiter)

	// Ensure URL-safety using pre-compiled regex
	slug = sg.safePattern.ReplaceAllString(slug, "")

	// Handle consecutive delimiters using pre-compiled regex
	slug = sg.multiPattern.ReplaceAllString(slug, sg.delimiter)

	// Trim delimiters from start and end
	slug = strings.Trim(slug, sg.delimiter)

	// Enforce max length, being careful not to cut in the middle of a word
	if len(slug) > sg.maxLength {
		parts := strings.Split(slug, sg.delimiter)
		result := ""
		for _, part := range parts {
			if len(result)+len(part)+len(sg.delimiter) <= sg.maxLength {
				if result != "" {
					result += sg.delimiter
				}
				result += part
			} else {
				break
			}
		}
		slug = result
	}

	return slug
}

func (sg *SlugGenerator) generateUUIDv4() string {
	// Generate a UUID v4 (random UUID)
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "error-generating-uuid"
	}

	// Set version (4) and variant bits according to RFC 4122
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 1

	// Use RawURLEncoding to drop padding without replacements
	uuid := strings.ToLower(base64.RawURLEncoding.EncodeToString(b))

	if len(uuid) > sg.maxLength {
		uuid = uuid[:sg.maxLength]
	}

	return uuid
}

func (sg *SlugGenerator) generateUUIDv7() string {
	// Generate a UUID v7 (time-based UUID with random component)
	b := make([]byte, 16)

	// Get current Unix timestamp in milliseconds for the first 48 bits
	timestamp := time.Now().UnixMilli()
	binary.BigEndian.PutUint32(b[0:4], uint32(timestamp>>16))
	binary.BigEndian.PutUint16(b[4:6], uint16(timestamp&0xFFFF))

	// Fill the remaining bytes with random data
	_, err := rand.Read(b[6:])
	if err != nil {
		return "error-generating-uuid"
	}

	// Set version (7) and variant bits according to RFC 4122
	b[6] = (b[6] & 0x0f) | 0x70 // Version 7
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 1

	// Use RawURLEncoding to drop padding without replacements
	uuid := strings.ToLower(base64.RawURLEncoding.EncodeToString(b))

	if len(uuid) > sg.maxLength {
		uuid = uuid[:sg.maxLength]
	}

	return uuid
}

func (sg *SlugGenerator) generateNanoID() string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-"
	length := sg.randomLength
	if length <= 0 {
		length = 21 // Default NanoID length
	}

	bytes := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "error-generating-nanoid"
		}
		bytes[i] = alphabet[num.Int64()]
	}

	return string(bytes)
}

func (sg *SlugGenerator) generateRandomSlug() string {
	// Use a mix of lowercase letters and numbers
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	length := sg.randomLength
	if length <= 0 {
		length = 8
	}

	bytes := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "error-generating-random-slug"
		}
		bytes[i] = alphabet[num.Int64()]
	}

	return string(bytes)
}

// compileRegex compiles regex patterns based on the current delimiter.
func (sg *SlugGenerator) compileRegex() {
	d := regexp.QuoteMeta(sg.delimiter)
	sg.safePattern = regexp.MustCompile("[^a-zA-Z0-9" + d + "]+")
	sg.multiPattern = regexp.MustCompile(d + "+")
}

// Common English stop words that can be removed from slugs
func defaultStopWords() map[string]bool {
	return map[string]bool{
		"a": true, "an": true, "the": true, "and": true, "or": true,
		"but": true, "if": true, "then": true, "else": true, "when": true,
		"at": true, "from": true, "by": true, "for": true, "with": true,
		"about": true, "to": true, "in": true, "on": true, "of": true,
	}
}
