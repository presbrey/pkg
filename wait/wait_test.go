package wait_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/presbrey/pkg/wait"
)

func ExampleUntil() {
	// Wait until a custom condition is met
	counter := 0
	err := wait.Until(func() (bool, error) {
		counter++
		return counter >= 3, nil
	}, wait.DefaultOptions().WithMaxRetries(5))

	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Condition met after", counter, "attempts")
}

func ExampleForNetwork() {
	// Wait for network connectivity with timeout
	err := wait.ForNetwork(
		wait.DefaultOptions().
			WithTimeout(30 * time.Second).
			WithStrategy(wait.NewFixedStrategy(2 * time.Second)),
	)

	if err != nil {
		log.Printf("Network not available: %v", err)
	} else {
		fmt.Println("Network is available")
	}
}

func ExampleForHTTP() {
	// Wait for a service to be healthy
	err := wait.ForHTTPSHealthy(
		"http://localhost:8080/health",
		wait.DefaultOptions().
			WithMaxRetries(10).
			WithStrategy(wait.NewExponentialBackoffStrategy(
				1*time.Second,  // initial delay
				2.0,            // multiplier
				30*time.Second, // max delay
				true,           // add jitter
			)),
	)

	if err != nil {
		log.Printf("Service not healthy: %v", err)
	}
}

func ExampleForMultiplePorts() {
	// Wait for multiple services to be ready
	ports := []int{8080, 8081, 8082}

	err := wait.ForMultiplePorts(
		ports,
		wait.DefaultOptions().
			WithTimeout(2*time.Minute).
			WithStrategy(wait.NewLinearStrategy(
				1*time.Second,        // initial
				500*time.Millisecond, // increment
				5*time.Second,        // max
			)),
	)

	if err != nil {
		log.Printf("Not all ports are available: %v", err)
	} else {
		fmt.Println("All services are ready")
	}
}

func ExampleRetryWithBackoff() {
	// Retry a function with exponential backoff
	attempts := 0
	err := wait.RetryWithBackoff(func() error {
		attempts++
		fmt.Printf("Attempt %d\n", attempts)

		if attempts < 3 {
			return fmt.Errorf("not ready yet")
		}
		return nil
	}, 100*time.Millisecond, 5)

	if err != nil {
		log.Printf("Failed after retries: %v", err)
	}
}

func ExampleGroup() {
	// Wait for multiple conditions
	group := wait.NewGroup()

	// Add various conditions
	group.Add(func() (bool, error) {
		// Check if database is ready
		return true, nil // simplified
	})

	group.Add(func() (bool, error) {
		// Check if cache is ready
		return true, nil // simplified
	})

	group.AddFunc(func() error {
		// Check if config is loaded
		return nil // simplified
	})

	// Wait for all conditions
	err := group.Wait(wait.DefaultOptions().WithTimeout(30 * time.Second))
	if err != nil {
		log.Printf("Not all services ready: %v", err)
	}
}

func ExampleAny() {
	// Wait for any of multiple conditions
	conditions := []wait.ConditionFunc{
		func() (bool, error) {
			// Check primary server
			return false, nil // simplified
		},
		func() (bool, error) {
			// Check backup server
			return true, nil // simplified
		},
	}

	index, err := wait.Any(
		conditions,
		wait.DefaultOptions().WithTimeout(10*time.Second),
	)

	if err != nil {
		log.Printf("No server available: %v", err)
	} else {
		fmt.Printf("Server %d is available\n", index)
	}
}

func ExampleOptions_WithContext() {
	// Use context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a goroutine that cancels after some condition
	go func() {
		time.Sleep(5 * time.Second)
		cancel()
	}()

	err := wait.Until(
		func() (bool, error) {
			// Some long-running check
			time.Sleep(1 * time.Second)
			return false, nil
		},
		wait.DefaultOptions().WithContext(ctx),
	)

	if err == wait.ErrCanceled {
		fmt.Println("Operation was canceled")
	}
}

func ExampleCustomStrategy() {
	// Use custom wait durations
	durations := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
	}

	strategy := wait.NewCustomStrategy(durations, false)

	err := wait.Until(
		func() (bool, error) {
			// Your condition here
			return false, nil
		},
		wait.DefaultOptions().WithStrategy(strategy),
	)

	if err != nil {
		fmt.Printf("Condition not met: %v\n", err)
	}
}

func ExampleNewDecorrelatedJitterStrategy() {
	// Use AWS-style decorrelated jitter for better distributed systems behavior
	strategy := wait.NewDecorrelatedJitterStrategy(
		100*time.Millisecond, // base
		10*time.Second,       // max
	)

	err := wait.ForTCP(
		"database:5432",
		wait.DefaultOptions().
			WithStrategy(strategy).
			WithMaxRetries(20),
	)

	if err != nil {
		log.Printf("Database connection failed: %v", err)
	}
}
