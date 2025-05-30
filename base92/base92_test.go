package base92

import (
	"bytes"
	"crypto/rand"
	"io"
	"strings"
	"testing"
)

func TestBase92EncodeDecode(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"Empty", []byte{}},
		{"Single Byte", []byte{65}},
		{"ASCII", []byte("Hello, World!")},
		{"Binary", []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{"URL Chars", []byte("https://example.com/test?param=value")},
		{"UTF-8", []byte("こんにちは世界")},
		{"Emoji", []byte("🚀🌟🌈")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := Encode(tt.data)
			decoded, err := Decode(encoded)
			if err != nil {
				t.Errorf("Failed to decode: %v", err)
			}
			if !bytes.Equal(decoded, tt.data) {
				t.Errorf("Decode(Encode(%v)) = %v, want %v", tt.data, decoded, tt.data)
			}
		})
	}
}

func TestEncodedStringContainsOnlyURLSafeChars(t *testing.T) {
	// Generate random data of different sizes
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		t.Run("Random data of size", func(t *testing.T) {
			data := make([]byte, size)
			_, err := io.ReadFull(rand.Reader, data)
			if err != nil {
				t.Fatalf("Failed to generate random data: %v", err)
			}

			encoded := Encode(data)

			// Verify all characters are URL-safe
			for i := 0; i < len(encoded); i++ {
				c := encoded[i]
				if !isURLSafeChar(c) {
					t.Errorf("Character '%c' at position %d is not URL-safe", c, i)
				}
			}
		})
	}
}

func isURLSafeChar(c byte) bool {
	return strings.IndexByte(charset, c) >= 0
}

func TestDecodeInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		wantErr error
	}{
		{"Invalid Character", "ABC#DEF", ErrInvalidChar},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(tt.encoded)
			if err != tt.wantErr {
				t.Errorf("Decode(%q) error = %v, wantErr %v", tt.encoded, err, tt.wantErr)
			}
		})
	}
}

func TestEncodingRoundtrip(t *testing.T) {
	// Test with different input sizes to ensure proper bit handling
	sizes := []int{1, 2, 3, 4, 5, 10, 16, 20, 32, 64, 100, 127, 128, 129, 255, 256, 257, 1000}

	for _, size := range sizes {
		t.Run("Size", func(t *testing.T) {
			data := make([]byte, size)
			_, err := io.ReadFull(rand.Reader, data)
			if err != nil {
				t.Fatalf("Failed to generate random data: %v", err)
			}

			encoded := Encode(data)
			decoded, err := Decode(encoded)
			if err != nil {
				t.Errorf("Failed to decode: %v", err)
			}

			if !bytes.Equal(decoded, data) {
				t.Errorf("Decode(Encode(data)) != data for size %d", size)
				t.Errorf("First few bytes: Original=%v, Decoded=%v", data[:min(10, len(data))], decoded[:min(10, len(decoded))])
			}
		})
	}
}

// Helper function for Go versions that might not have min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func BenchmarkEncode(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		data := make([]byte, size)
		_, err := io.ReadFull(rand.Reader, data)
		if err != nil {
			b.Fatalf("Failed to generate random data: %v", err)
		}

		b.Run("Size", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Encode(data)
			}
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		data := make([]byte, size)
		_, err := io.ReadFull(rand.Reader, data)
		if err != nil {
			b.Fatalf("Failed to generate random data: %v", err)
		}

		encoded := Encode(data)

		b.Run("Size", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = Decode(encoded)
			}
		})
	}
}

// TestDecodeIgnoresWhitespace verifies that Decode ignores whitespace characters.
func TestDecodeIgnoresWhitespace(t *testing.T) {
	originalData := []byte("Test data with spaces and newlines")
	encoded := Encode(originalData)

	tests := []struct {
		name          string
		encodedInput  string
		expectedData  []byte
		expectErr     bool
	}{
		{
			name:         "Spaces only",
			encodedInput: insertWhitespace(encoded, ' '),
			expectedData: originalData,
		},
		{
			name:         "Tabs only",
			encodedInput: insertWhitespace(encoded, '\t'),
			expectedData: originalData,
		},
		{
			name:         "Newlines only (LF)",
			encodedInput: insertWhitespace(encoded, '\n'),
			expectedData: originalData,
		},
		{
			name:         "Carriage returns only (CR)",
			encodedInput: insertWhitespace(encoded, '\r'),
			expectedData: originalData,
		},
		{
			name:         "Mixed whitespace (Space, Tab, LF, CR)",
			encodedInput: insertMixedWhitespace(encoded),
			expectedData: originalData,
		},
		{
			name:         "Invalid char with whitespace",
			encodedInput: encoded[:5] + " # " + encoded[5:], // '#' is invalid
			expectErr:    true,
		},
		{
			name: "Empty input",
			encodedInput: "",
			expectedData: []byte{},
		},
		{
			name: "Whitespace only input",
			encodedInput: " \t\n\r ",
			expectedData: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := Decode(tt.encodedInput)
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
				// Optionally check for specific error type if needed, e.g., assert.ErrorIs(t, err, ErrInvalidChar)
			} else {
				if err != nil {
					t.Errorf("Decode failed: %v", err)
				}
				if !bytes.Equal(decoded, tt.expectedData) {
					t.Errorf("Decode() got = %v, want %v", decoded, tt.expectedData)
				}
			}
		})
	}
}

// Helper function to insert a specific whitespace character periodically
func insertWhitespace(s string, whitespaceChar byte) string {
	var builder strings.Builder
	for i, r := range s {
		builder.WriteRune(r)
		if i%5 == 4 { // Insert whitespace every 5 chars
			builder.WriteByte(whitespaceChar)
		}
	}
	return builder.String()
}

// Helper function to insert mixed whitespace characters periodically
func insertMixedWhitespace(s string) string {
	var builder strings.Builder
	whitespace := []byte{' ', '\t', '\n', '\r'}
	for i, r := range s {
		builder.WriteRune(r)
		if i%3 == 2 { // Insert mixed whitespace every 3 chars
			builder.WriteByte(whitespace[i%len(whitespace)])
		}
	}
	return builder.String()
}
