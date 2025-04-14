package main

import (
	"fmt"

	"github.com/presbrey/pkg/slugs"
)

func main() {
	// Basic text slug
	basicSlug := slugs.New().Generate("Hello World")
	fmt.Printf("Basic slug: %s\n", basicSlug)

	// Customize delimiter
	customDelimiterSlug := slugs.New().Delimiter("_").Generate("Hello World")
	fmt.Printf("Custom delimiter: %s\n", customDelimiterSlug)

	// Max length with stop word removal
	shortSlug := slugs.New().
		MaxLength(15).
		RemoveStopWords(true).
		Generate("The quick brown fox jumps over the lazy dog")
	fmt.Printf("Short slug with stop words removed: %s\n", shortSlug)

	// Preserve case
	preserveCaseSlug := slugs.New().Lowercase(false).Generate("Hello World")
	fmt.Printf("Preserved case: %s\n", preserveCaseSlug)

	// With prefix and suffix
	taggedSlug := slugs.New().
		WithPrefix("blog").
		WithSuffix("2025").
		Generate("My Latest Article")
	fmt.Printf("Tagged slug: %s\n", taggedSlug)

	// Generate a UUID-style slug
	uuidSlug := slugs.New().UUID().Generate("")
	fmt.Printf("UUID slug: %s\n", uuidSlug)

	// Generate a NanoID-style slug with custom length
	nanoIDSlug := slugs.New().NanoID().RandomLength(10).Generate("")
	fmt.Printf("NanoID slug (10 chars): %s\n", nanoIDSlug)

	// Generate a random slug
	randomSlug := slugs.New().Random().RandomLength(8).Generate("")
	fmt.Printf("Random slug (8 chars): %s\n", randomSlug)

	// Chain multiple configurations
	complexSlug := slugs.New().
		MaxLength(30).
		Delimiter("-").
		RemoveStopWords(true).
		AddStopWords("latest").
		WithPrefix("article").
		Generate("The Latest and Greatest Development in Go Programming")
	fmt.Printf("Complex slug: %s\n", complexSlug)
}
