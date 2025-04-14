# slugs

A Go package for generating URL-safe slugs with a fluent API pattern.

## Features

- Create slugs from text with customizable options
- Generate UUID-style, NanoID-style, or random slugs
- Fluent interface for easy configuration
- Customizable maximum length, delimiter, case preservation
- Stop word removal (with customizable stop word list)
- Add prefixes and suffixes to generated slugs

## Installation

```bash
go get github.com/yourusername/slugs
```

## Usage

### Basic Examples

```go
import (
    "fmt"
    "github.com/yourusername/slugs"
)

// Basic text slug
slug := slugs.New().Generate("Hello World")
fmt.Println(slug) // Output: hello-world

// Custom delimiter
slug = slugs.New().Delimiter("_").Generate("Hello World")
fmt.Println(slug) // Output: hello_world

// Max length
slug = slugs.New().MaxLength(10).Generate("This is a very long title")
fmt.Println(slug) // Output: this-is-a

// Remove stop words
slug = slugs.New().RemoveStopWords(true).Generate("The quick brown fox")
fmt.Println(slug) // Output: quick-brown-fox

// UUID-style slug
slug = slugs.New().UUID().Generate("")
fmt.Println(slug) // Output: random UUID-style string

// NanoID-style slug
slug = slugs.New().NanoID().RandomLength(10).Generate("")
fmt.Println(slug) // Output: 10-character NanoID-style string
```

### Advanced Configuration

```go
// Chain multiple configurations
slug := slugs.New().
    MaxLength(30).
    Delimiter("-").
    RemoveStopWords(true).
    AddStopWords("custom", "words").
    WithPrefix("article").
    WithSuffix("2025").
    Generate("The Latest and Greatest Development in Go Programming")

fmt.Println(slug) // Output: article-latest-greatest-development-go-2025
```

## API Reference

### Creating a Generator

```go
// Create a new generator with default settings
generator := slugs.New()
```

### Configuration Methods

All configuration methods return the generator itself, allowing for method chaining:

- `MaxLength(length int)` - Set maximum length of the generated slug
- `Delimiter(delimiter string)` - Set character used to separate words (default: "-")
- `Lowercase(lowercase bool)` - Set whether to convert the slug to lowercase (default: true)
- `RemoveStopWords(remove bool)` - Set whether to remove common stop words (default: false)
- `AddStopWords(words ...string)` - Add custom stop words to be removed
- `WithPrefix(prefix string)` - Add a prefix to the generated slug
- `WithSuffix(suffix string)` - Add a suffix to the generated slug
- `UUID()` - Set generator to create UUID-style slugs
- `NanoID()` - Set generator to create NanoID-style slugs
- `Random()` - Set generator to create random string slugs
- `RandomLength(length int)` - Set length of random slugs (default: 8)

### Generating Slugs

```go
// Generate a slug from text
slug := generator.Generate("Your text here")
```

## License

MIT License