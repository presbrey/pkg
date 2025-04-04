package main

import (
	"fmt"
	"log"
	"time"

	"github.com/presbrey/pkg/syncmap"
)

func main() {
	// Create a RemoteMap with fluent interface
	rm := syncmap.NewRemoteMap("https://api.example.com/data").
		WithRefreshPeriod(30 * time.Second).
		WithTimeout(10 * time.Second).
		WithIgnoreTLSVerify(false).
		WithHeader("User-Agent", "SyncMap/1.0").
		WithErrorHandler(func(err error) {
			log.Printf("Error refreshing map: %v", err)
		}).
		WithUpdateCallback(func(updated []string) {
			log.Printf("Updated keys: %v", updated)
		}).
		WithDeleteCallback(func(deleted []string) {
			log.Printf("Deleted keys: %v", deleted)
		}).
		WithRefreshCallback(func() {
			log.Printf("Map refreshed at %v", time.Now())
		}).
		Start()

	// Stop the automatic refresh when done
	defer rm.Stop()

	// Use the map
	for {
		// Access values using type-specific getters
		if name, ok := rm.GetString("name"); ok {
			fmt.Printf("Name: %s\n", name)
		} else {
			// Use default value if key doesn't exist
			name = rm.GetStringWithDefault("name", "Unknown")
			fmt.Printf("Name (with default): %s\n", name)
		}

		if count, ok := rm.GetInt("count"); ok {
			fmt.Printf("Count: %d\n", count)
		} else {
			// Use default value if key doesn't exist
			count = rm.GetIntWithDefault("count", 0)
			fmt.Printf("Count (with default): %d\n", count)
		}

		if bigNum, ok := rm.GetInt64("big_number"); ok {
			fmt.Printf("Big Number: %d\n", bigNum)
		} else {
			// Use default value if key doesn't exist
			bigNum = rm.GetInt64WithDefault("big_number", 0)
			fmt.Printf("Big Number (with default): %d\n", bigNum)
		}

		if enabled, ok := rm.GetBool("enabled"); ok {
			fmt.Printf("Enabled: %t\n", enabled)
		} else {
			// Use default value if key doesn't exist
			enabled = rm.GetBoolWithDefault("enabled", false)
			fmt.Printf("Enabled (with default): %t\n", enabled)
		}

		// Get all keys
		keys := rm.Keys()
		fmt.Println("All keys:", keys)

		// Or use the standard sync.Map methods
		rm.Range(func(key, value interface{}) bool {
			fmt.Printf("%v: %v\n", key, value)
			return true
		})

		time.Sleep(5 * time.Second)
	}
}
