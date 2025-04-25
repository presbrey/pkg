package slugs

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestTextSlugGeneration(t *testing.T) {
	testCases := []struct {
		name     string
		text     string
		options  func(*SlugGenerator) *SlugGenerator
		expected string
	}{
		{
			name: "Basic text slug",
			text: "Hello World",
			options: func(sg *SlugGenerator) *SlugGenerator {
				return sg
			},
			expected: "hello-world",
		},
		{
			name: "With special characters",
			text: "Hello, World! How are you?",
			options: func(sg *SlugGenerator) *SlugGenerator {
				return sg
			},
			expected: "hello-world-how-are-you",
		},
		{
			name: "Custom delimiter",
			text: "Hello World",
			options: func(sg *SlugGenerator) *SlugGenerator {
				return sg.Delimiter("_")
			},
			expected: "hello_world",
		},
		{
			name: "Max length",
			text: "This is a very long title that should be truncated",
			options: func(sg *SlugGenerator) *SlugGenerator {
				return sg.MaxLength(20)
			},
			expected: "this-is-a-very-long",
		},
		{
			name: "Keep uppercase",
			text: "Hello World",
			options: func(sg *SlugGenerator) *SlugGenerator {
				return sg.Lowercase(false)
			},
			expected: "Hello-World",
		},
		{
			name: "Remove stop words",
			text: "The quick brown fox jumps over the lazy dog",
			options: func(sg *SlugGenerator) *SlugGenerator {
				return sg.RemoveStopWords(true)
			},
			expected: "quick-brown-fox-jumps-over-lazy-dog",
		},
		{
			name: "With prefix",
			text: "Hello World",
			options: func(sg *SlugGenerator) *SlugGenerator {
				return sg.WithPrefix("prefix")
			},
			expected: "prefix-hello-world",
		},
		{
			name: "With suffix",
			text: "Hello World",
			options: func(sg *SlugGenerator) *SlugGenerator {
				return sg.WithSuffix("suffix")
			},
			expected: "hello-world-suffix",
		},
		{
			name: "With prefix and suffix",
			text: "Hello World",
			options: func(sg *SlugGenerator) *SlugGenerator {
				return sg.WithPrefix("prefix").WithSuffix("suffix")
			},
			expected: "prefix-hello-world-suffix",
		},
		{
			name: "Custom stop words",
			text: "Hello World Custom Stop Word",
			options: func(sg *SlugGenerator) *SlugGenerator {
				return sg.RemoveStopWords(true).AddStopWords("custom", "hello")
			},
			expected: "world-stop-word",
		},
		{
			name: "Empty text",
			text: "",
			options: func(sg *SlugGenerator) *SlugGenerator {
				return sg
			},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			generator := tc.options(New())
			result := generator.Generate(tc.text)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestRandomSlugTypes(t *testing.T) {
	t.Run("UUID v4 slug (legacy method)", func(t *testing.T) {
		generator := New().UUID()
		slug := generator.Generate("ignored text")
		if len(slug) < 10 {
			t.Errorf("UUID v4 slug too short: %s", slug)
		}
		urlSafePattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
		if !urlSafePattern.MatchString(slug) {
			t.Errorf("UUID v4 slug is not URL-safe: %s", slug)
		}
	})

	t.Run("UUID v4 slug", func(t *testing.T) {
		generator := New().UUIDv4()
		slug := generator.Generate("ignored text")
		if len(slug) < 10 {
			t.Errorf("UUID v4 slug too short: %s", slug)
		}
		urlSafePattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
		if !urlSafePattern.MatchString(slug) {
			t.Errorf("UUID v4 slug is not URL-safe: %s", slug)
		}
	})

	t.Run("UUID v7 slug", func(t *testing.T) {
		generator := New().UUIDv7()
		slug := generator.Generate("ignored text")
		if len(slug) < 10 {
			t.Errorf("UUID v7 slug too short: %s", slug)
		}
		urlSafePattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
		if !urlSafePattern.MatchString(slug) {
			t.Errorf("UUID v7 slug is not URL-safe: %s", slug)
		}

		// Generate another UUID v7 after a short delay to verify timestamps are different
		time.Sleep(10 * time.Millisecond)
		slug2 := generator.Generate("ignored text")
		if slug == slug2 {
			t.Errorf("UUID v7 slugs should be different due to timestamp component, got: %s and %s", slug, slug2)
		}
	})

	t.Run("NanoID slug", func(t *testing.T) {
		generator := New().NanoID().RandomLength(15)
		slug := generator.Generate("ignored text")
		if len(slug) != 15 {
			t.Errorf("NanoID slug length is incorrect, expected 15, got %d", len(slug))
		}
		urlSafePattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
		if !urlSafePattern.MatchString(slug) {
			t.Errorf("NanoID slug is not URL-safe: %s", slug)
		}
	})

	t.Run("Random slug", func(t *testing.T) {
		generator := New().Random().RandomLength(10)
		slug := generator.Generate("ignored text")
		if len(slug) != 10 {
			t.Errorf("Random slug length is incorrect, expected 10, got %d", len(slug))
		}
		urlSafePattern := regexp.MustCompile(`^[a-z0-9]+$`)
		if !urlSafePattern.MatchString(slug) {
			t.Errorf("Random slug is not URL-safe: %s", slug)
		}
	})
}

func TestFluentInterface(t *testing.T) {
	// Test that the fluent interface works as expected
	generator := New().
		MaxLength(20).
		Delimiter("_").
		Lowercase(false).
		RemoveStopWords(true).
		AddStopWords("custom").
		WithPrefix("pre").
		WithSuffix("post").
		RandomLength(10)

	if generator.maxLength != 20 ||
		generator.delimiter != "_" ||
		generator.lowercase != false ||
		generator.removeStopWords != true ||
		!generator.stopWords["custom"] ||
		generator.prefix != "pre" ||
		generator.suffix != "post" ||
		generator.randomLength != 10 {
		t.Errorf("Fluent interface did not properly set values")
	}
}

func TestConsistency(t *testing.T) {
	// Test that the same input produces the same output
	generator := New()
	slug1 := generator.Generate("Hello World")
	slug2 := generator.Generate("Hello World")

	if slug1 != slug2 {
		t.Errorf("Inconsistent results for same input: %s vs %s", slug1, slug2)
	}
}

func TestRandomness(t *testing.T) {
	// Test that random generators produce different values
	generator := New().Random().RandomLength(20)
	slugs := make(map[string]bool)

	for i := 0; i < 100; i++ {
		slug := generator.Generate("")
		if slugs[slug] {
			t.Errorf("Duplicate slug detected: %s", slug)
		}
		slugs[slug] = true
	}
}

func TestEdgeCases(t *testing.T) {
	t.Run("Very long input", func(t *testing.T) {
		generator := New().MaxLength(100)
		longText := strings.Repeat("a ", 1000)
		slug := generator.Generate(longText)
		if len(slug) > 100 {
			t.Errorf("Slug exceeds max length: %d > 100", len(slug))
		}
	})

	t.Run("Input with only special characters", func(t *testing.T) {
		generator := New()
		specialText := "!@#$%^&*()"
		slug := generator.Generate(specialText)
		if slug != "" {
			t.Errorf("Expected empty slug for special characters only, got: %s", slug)
		}
	})

	t.Run("Input with mixed content", func(t *testing.T) {
		generator := New()
		mixedText := "Hello123!@#World"
		slug := generator.Generate(mixedText)
		expected := "hello123-world"
		if slug != expected {
			t.Errorf("Expected %q, got %q", expected, slug)
		}
	})
}

func TestReusability(t *testing.T) {
	// Test that a single generator can be reused with different settings
	generator := New()

	slug1 := generator.Delimiter("-").Generate("Hello World")
	if slug1 != "hello-world" {
		t.Errorf("Expected 'hello-world', got %q", slug1)
	}

	slug2 := generator.Delimiter("_").Generate("Hello World")
	if slug2 != "hello_world" {
		t.Errorf("Expected 'hello_world', got %q", slug2)
	}
}

func BenchmarkSlugGeneration(b *testing.B) {
	generator := New()
	text := "This is a benchmark test for the slug generation package"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		generator.Generate(text)
	}
}

func BenchmarkRandomSlugGeneration(b *testing.B) {
	generator := New().Random()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		generator.Generate("")
	}
}
