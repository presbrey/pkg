package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/presbrey/pkg/slugs"
)

func main() {
	maxLength := flag.Int("maxlen", 100, "maximum length of the generated slug")
	delimiter := flag.String("delim", "-", "character used to separate words in the slug")
	lowercase := flag.Bool("lower", true, "convert slug to lowercase")
	removeStopWords := flag.Bool("stop", false, "remove common stop words")
	addStopWords := flag.String("stopwords", "", "additional stop words to remove (comma-separated)")
	prefix := flag.String("prefix", "", "prefix to add to the slug")
	suffix := flag.String("suffix", "", "suffix to add to the slug")

	uuidv4Mode := flag.Bool("u4", false, "generate UUID v4-based slug")
	uuidv7Mode := flag.Bool("u7", false, "generate UUID v7-based slug")
	nanoLength := flag.Int("n", 0, "length of NanoID slugs")
	randomLength := flag.Int("r", 0, "length of random slugs")
	count := flag.Int("c", 1, "number of slugs to generate")

	flag.Parse()

	// Check if any text was provided
	args := flag.Args()
	if len(args) == 0 && !*uuidv4Mode && !*uuidv7Mode && *nanoLength == 0 && *randomLength == 0 {
		fmt.Println("Error: No input text provided and no random slug mode selected")
		fmt.Println("Usage: slug [options] text")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Create and configure the slug generator
	sg := slugs.New().
		MaxLength(*maxLength).
		Delimiter(*delimiter).
		Lowercase(*lowercase).
		RemoveStopWords(*removeStopWords)

	// Add custom stop words if provided
	if *addStopWords != "" {
		words := strings.Split(*addStopWords, ",")
		sg.AddStopWords(words...)
	}

	// Add prefix and suffix if provided
	if *prefix != "" {
		sg.WithPrefix(*prefix)
	}
	if *suffix != "" {
		sg.WithSuffix(*suffix)
	}

	// Set the slug type based on flags
	if *uuidv4Mode {
		sg.UUIDv4()
	} else if *uuidv7Mode {
		sg.UUIDv7()
	} else if *nanoLength > 0 {
		sg.NanoID().RandomLength(*nanoLength)
	} else if *randomLength > 0 {
		sg.Random().RandomLength(*randomLength)
	}

	// Generate the slugs
	var text string
	if len(args) > 0 {
		text = strings.Join(args, " ")
	}

	// Ensure count is at least 1
	if *count < 1 {
		*count = 1
	}

	// Generate and output the requested number of slugs
	for i := 0; i < *count; i++ {
		slug := sg.Generate(text)
		fmt.Println(slug)
	}
}
