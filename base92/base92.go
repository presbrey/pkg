// Package base92 provides encoding and decoding for a URL-safe Base92 encoding scheme.
package base92

import (
	"errors"
	"strings"
)

// The character set for Base92 encoding using only URL-safe characters
// Excludes non-URL safe characters like: `"'\/<>?#{}|^~[]`;,
const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_.$!*()+=@&%:;"

var (
	ErrInvalidLength = errors.New("base92: invalid input length")
	ErrInvalidChar   = errors.New("base92: invalid character in input")
	charToIndexMap   map[byte]int
)

func init() {
	// Initialize the character-to-index map
	charToIndexMap = make(map[byte]int, len(charset))
	for i := 0; i < len(charset); i++ {
		charToIndexMap[charset[i]] = i
	}
}

// Encode converts a byte slice to a Base92 encoded string
func Encode(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	var output strings.Builder
	bitBuffer := uint(0)
	bitsInBuffer := uint(0)

	for _, b := range data {
		// Add 8 bits to buffer
		bitBuffer = (bitBuffer << 8) | uint(b)
		bitsInBuffer += 8

		// Extract 6.5 bits (base92 uses ~6.5 bits per character)
		for bitsInBuffer >= 6 {
			bitsInBuffer -= 6
			index := (bitBuffer >> bitsInBuffer) & 0x3F // 63 (2^6 - 1)
			output.WriteByte(charset[index])
		}
	}

	// Handle remaining bits if any
	if bitsInBuffer > 0 {
		index := (bitBuffer & ((1 << bitsInBuffer) - 1)) << (6 - bitsInBuffer)
		output.WriteByte(charset[index])
	}

	return output.String()
}

// Decode converts a Base92 encoded string back to the original byte slice
func Decode(encoded string) ([]byte, error) {
	if len(encoded) == 0 {
		return []byte{}, nil
	}

	bitBuffer := uint(0)
	bitsInBuffer := uint(0)
	result := make([]byte, 0, len(encoded)*6/8) // Approximate size

	for i := 0; i < len(encoded); i++ {
		c := encoded[i]

		// Ignore whitespace characters
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			continue
		}

		index, ok := charToIndexMap[c]
		if !ok {
			return nil, ErrInvalidChar
		}

		// Add 6 bits to buffer
		bitBuffer = (bitBuffer << 6) | uint(index)
		bitsInBuffer += 6

		// Extract 8 bits (1 byte) when available
		for bitsInBuffer >= 8 {
			bitsInBuffer -= 8
			result = append(result, byte(bitBuffer>>bitsInBuffer))
		}
	}

	return result, nil
}
