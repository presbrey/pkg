package main

import (
	"fmt"
	"log"

	"github.com/presbrey/pkg/hooks"
)

// SiteContext represents the application context for site initialization
type SiteContext struct {
	Name        string
	Environment string
	Config      map[string]string
	Components  []string
}

func main() {
	// Create a new hook registry for site initialization
	siteInitRegistry := hooks.NewRegistry[*SiteContext]()

	// Register hooks with different priorities

	// Database initialization (high priority - runs first)
	siteInitRegistry.RegisterWithPriority(func(ctx *SiteContext) error {
		fmt.Println("Initializing database connection...")
		ctx.Components = append(ctx.Components, "database")
		return nil
	}, -10)

	// User authentication setup (normal priority)
	siteInitRegistry.Register(func(ctx *SiteContext) error {
		fmt.Println("Setting up authentication...")
		ctx.Components = append(ctx.Components, "auth")
		return nil
	})

	// Analytics tracking (low priority - runs later)
	siteInitRegistry.RegisterWithPriority(func(ctx *SiteContext) error {
		fmt.Println("Configuring analytics...")
		ctx.Components = append(ctx.Components, "analytics")
		return nil
	}, 10)

	// Cache warming (can fail without breaking site)
	siteInitRegistry.RegisterWithPriority(func(ctx *SiteContext) error {
		fmt.Println("Warming cache...")
		if ctx.Environment == "development" {
			fmt.Println("Skipping cache warming in development mode")
			return fmt.Errorf("cache warming skipped")
		}
		ctx.Components = append(ctx.Components, "cache")
		return nil
	}, 20)

	// Initialize site context
	site := &SiteContext{
		Name:        "My Website",
		Environment: "development",
		Config:      make(map[string]string),
		Components:  make([]string, 0),
	}

	// Run all site initialization hooks
	errors := siteInitRegistry.RunAll(site)

	// Log any errors that occurred during initialization
	if errors != nil {
		fmt.Println("Site initialization completed with errors:")
		for hookName, err := range errors {
			log.Printf("  - %s: %v", hookName, err)
		}
	} else {
		fmt.Println("Site initialization completed successfully!")
	}

	// Display initialized components
	fmt.Println("Initialized components:")
	for _, component := range site.Components {
		fmt.Printf("  - %s\n", component)
	}
}
