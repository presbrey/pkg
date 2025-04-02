package main

import (
	"fmt"
	"log"
	"time"

	"github.com/presbrey/pkg/syncmap"
)

func main() {
	// Create a RemoteMap with custom options
	options := &syncmap.Options{
		RefreshPeriod:   30 * time.Second,
		Timeout:         10 * time.Second,
		IgnoreTLSVerify: false,
		Headers: map[string]string{
			"User-Agent": "SyncMap/1.0",
		},
		ErrorHandler: func(err error) {
			log.Printf("Error refreshing map: %v", err)
		},
	}

	// Replace with your actual JSON endpoint
	rm := syncmap.NewRemoteMap("https://api.example.com/data", options)

	// Start the automatic refresh
	rm.Start()
	defer rm.Stop()

	// Use the map
	for {
		// Access values using type-specific getters
		if name, ok := rm.GetString("name"); ok {
			fmt.Printf("Name: %s\n", name)
		}

		if count, ok := rm.GetInt("count"); ok {
			fmt.Printf("Count: %d\n", count)
		}

		if bigNum, ok := rm.GetInt64("big_number"); ok {
			fmt.Printf("Big Number: %d\n", bigNum)
		}

		if enabled, ok := rm.GetBool("enabled"); ok {
			fmt.Printf("Enabled: %t\n", enabled)
		}

		// Or use the standard sync.Map methods
		rm.Range(func(key, value interface{}) bool {
			fmt.Printf("%v: %v\n", key, value)
			return true
		})

		time.Sleep(5 * time.Second)
	}
}
