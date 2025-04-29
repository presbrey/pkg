package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Base92 encoding/decoding implementation
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

// CLI implementation
func main() {
	var rootCmd = &cobra.Command{
		Use:   "base92",
		Short: "Base92 encoding and decoding utility",
		Long:  `A command-line utility for encoding and decoding data using the URL-safe Base92 encoding scheme.`,
	}

	var encodeCmd = &cobra.Command{
		Use:   "encode [file]",
		Short: "Encode data to Base92",
		Long:  `Encode data from stdin or a file to Base92 format.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var input []byte
			var err error

			if len(args) == 0 {
				// Read from stdin if no file is specified
				input, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("error reading from stdin: %w", err)
				}
			} else {
				// Read from specified file
				input, err = os.ReadFile(args[0])
				if err != nil {
					return fmt.Errorf("error reading file %s: %w", args[0], err)
				}
			}

			encoded := Encode(input)
			fmt.Println(encoded)
			return nil
		},
	}

	var decodeCmd = &cobra.Command{
		Use:   "decode [file]",
		Short: "Decode Base92 data",
		Long:  `Decode Base92 data from stdin or a file to its original format.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var input []byte
			var err error

			if len(args) == 0 {
				// Read from stdin if no file is specified
				input, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("error reading from stdin: %w", err)
				}
			} else {
				// Read from specified file
				input, err = os.ReadFile(args[0])
				if err != nil {
					return fmt.Errorf("error reading file %s: %w", args[0], err)
				}
			}

			// Trim any newlines that might have been added when reading files
			inputStr := string(input)
			inputStr = trimNewlines(inputStr)

			decoded, err := Decode(inputStr)
			if err != nil {
				return fmt.Errorf("error decoding Base92 data: %w", err)
			}

			os.Stdout.Write(decoded)
			return nil
		},
	}

	rootCmd.AddCommand(encodeCmd, decodeCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

// trimNewlines removes trailing newlines from a string
func trimNewlines(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
