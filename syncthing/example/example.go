package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/presbrey/pkg/syncthing"
)

func main() {
	// Start a simple HTTP server that serves JSON data
	go startTestServer()

	// Example 1: String values
	fmt.Println("\n=== String Map Example ===")
	stringMap := syncthing.NewMapString[string]("http://localhost:8080/string-data").
		WithRefreshPeriod(5 * time.Second).
		WithTimeout(10 * time.Second).
		WithErrorHandler(func(err error) {
			log.Printf("Error: %v", err)
		}).
		WithUpdateCallback(func(updated []string) {
			log.Printf("String map updated: %d keys changed", len(updated))
		}).
		WithDeleteCallback(func(deleted []string) {
			log.Printf("String map deleted: %d keys removed", len(deleted))
		}).
		Start()
	defer stringMap.Stop()

	// Wait for initial data to load
	time.Sleep(1 * time.Second)

	// Get string values
	if name, ok := stringMap.Get("name"); ok {
		fmt.Printf("Name: %s\n", name)
	}

	// Get with default
	description := stringMap.GetWithDefault("description", "No description available")
	fmt.Printf("Description: %s\n", description)

	// Example 2: Integer values
	fmt.Println("\n=== Integer Map Example ===")
	intMap := syncthing.NewMapString[int]("http://localhost:8080/int-data").
		WithRefreshPeriod(5 * time.Second).
		WithTimeout(10 * time.Second).
		WithErrorHandler(func(err error) {
			log.Printf("Error: %v", err)
		}).
		Start()
	defer intMap.Stop()

	// Wait for initial data to load
	time.Sleep(1 * time.Second)

	// Get integer values
	if count, ok := intMap.Get("count"); ok {
		fmt.Printf("Count: %d\n", count)
	}

	// Get with default
	limit := intMap.GetWithDefault("limit", 100)
	fmt.Printf("Limit: %d\n", limit)

	// Example 3: Any values
	fmt.Println("\n=== Any Map Example ===")
	anyMap := syncthing.NewMapString[any]("http://localhost:8080/mixed-data").
		WithRefreshPeriod(5 * time.Second).
		WithTimeout(10 * time.Second).
		Start()
	defer anyMap.Stop()

	// Wait for initial data to load
	time.Sleep(1 * time.Second)

	// Get values of different types
	if name, ok := anyMap.Get("name"); ok {
		fmt.Printf("Name: %v (type: %T)\n", name, name)
	}

	if age, ok := anyMap.Get("age"); ok {
		fmt.Printf("Age: %v (type: %T)\n", age, age)
	}

	if active, ok := anyMap.Get("active"); ok {
		fmt.Printf("Active: %v (type: %T)\n", active, active)
	}

	// Create a typed map MapString for settings
	settingsMap := syncthing.NewMapString[map[string]bool]("http://localhost:8080/mixed-data").
		WithRefreshPeriod(5 * time.Second).
		Start()
	defer settingsMap.Stop()

	// Wait for initial data to load
	time.Sleep(1 * time.Second)

	// Get settings as a typed map
	if settings, ok := settingsMap.Get("settings"); ok {
		fmt.Println("Settings:")
		for k, v := range settings {
			fmt.Printf("  %s: %t\n", k, v)
		}
	}

	// Get all keys
	fmt.Println("All keys:", anyMap.Keys())

	// Keep the program running
	fmt.Println("\nPress Ctrl+C to exit")
	select {}
}

func startTestServer() {
	// String data handler
	http.HandleFunc("/string-data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"name": "Example Service",
			"version": "1.0.0",
			"status": "running"
		}`)
	})

	// Integer data handler
	http.HandleFunc("/int-data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"count": 42,
			"max": 100,
			"min": 0
		}`)
	})

	// Mixed data handler
	http.HandleFunc("/mixed-data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"name": "John Doe",
			"age": 30,
			"active": true,
			"settings": {
				"notifications": true,
				"darkMode": false,
				"autoSave": true
			}
		}`)
	})

	log.Println("Starting test server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
