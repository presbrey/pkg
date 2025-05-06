package fly

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Machine represents the fly machine data structure
type Machine struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	State    string    `json:"state"`
	Region   string    `json:"region"`
	ImageRef ImageRef  `json:"image_ref"`
	Created  time.Time `json:"created_at"`
	Updated  time.Time `json:"updated_at"`
	Config   Config    `json:"config"`
	Events   []Event   `json:"events"`
}

// ImageRef contains image reference information
type ImageRef struct {
	Registry   string            `json:"registry"`
	Repository string            `json:"repository"`
	Tag        string            `json:"tag"`
	Digest     string            `json:"digest"`
	Labels     map[string]string `json:"labels"`
}

// Config contains machine configuration
type Config struct {
	Env      map[string]string `json:"env"`
	Guest    Guest             `json:"guest"`
	Metadata map[string]string `json:"metadata"`
	Services []interface{}     `json:"services"` // Using interface{} as we don't need to parse this
}

// Guest contains guest VM configuration
type Guest struct {
	CPUKind  string `json:"cpu_kind"`
	CPUs     int    `json:"cpus"`
	MemoryMB int    `json:"memory_mb"`
}

// Event represents machine events
type Event struct {
	Type      string                 `json:"type"`
	Status    string                 `json:"status"`
	Source    string                 `json:"source"`
	Timestamp int64                  `json:"timestamp"`
	Request   map[string]interface{} `json:"request"` // Using interface{} as structure may vary
}

// getEnvironmentStringSlice reads a comma-separated string from an environment variable
// and returns it as a slice of strings. If the environment variable is not set or empty,
// returns the default values.
func getEnvironmentStringSlice(envName string, defaultValues []string) []string {
	envValue := os.Getenv(envName)
	if envValue == "" {
		return defaultValues
	}

	// Split by comma and trim spaces
	values := strings.Split(envValue, ",")
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}

	return values
}

var (
	usRegions []string
	euRegions []string
	appNames  []string

	// Global counter for flyctl calls
	flyctlCallCount int32

	// ANSI color codes for terminal output
	colors = []string{
		"\033[38;5;160m", // Red
		"\033[38;5;208m", // Orange
		"\033[38;5;220m", // Yellow
		"\033[38;5;34m",  // Green
		"\033[38;5;27m",  // Blue
		"\033[38;5;93m",  // Purple
		"\033[38;5;199m", // Pink
		"\033[38;5;46m",  // Bright Green
		"\033[38;5;51m",  // Cyan
		"\033[38;5;201m", // Magenta
		"\033[38;5;202m", // Dark Orange
		"\033[38;5;33m",  // Medium Blue
		"\033[38;5;118m", // Light Green
		"\033[38;5;226m", // Bright Yellow
	}
	colorReset = "\033[0m"
)

func init() {
	usRegions = getEnvironmentStringSlice("US_REGIONS", []string{"us-east-1", "us-east-2", "us-east-3", "us-east-4"})
	euRegions = getEnvironmentStringSlice("EU_REGIONS", []string{"eu-west-1", "eu-west-2", "eu-west-3", "eu-west-4"})
	appNames = getEnvironmentStringSlice("APP_NAMES", []string{"portal", "websocket"})
}

// GetMachineList gets the list of machines for a specific app
func GetMachineList(appName string) ([]Machine, error) {
	// Increment the global flyctl call counter
	IncrementFlyctlCallCount()

	cmd := exec.Command("flyctl", "machine", "list", "--json", "-a", appName)
	var out bytes.Buffer
	cmd.Stdout = &out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error listing machines: %v - %s", err, stderr.String())
	}

	var machines []Machine
	err = json.Unmarshal(out.Bytes(), &machines)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	return machines, nil
}

// GetMachineLogs gets the logs for a specific machine
func GetMachineLogs(appName, machineID string, followFlag bool) (string, error) {
	// Increment the global flyctl call counter
	IncrementFlyctlCallCount()

	args := []string{"logs", "-a", appName, "--machine", machineID}
	if !followFlag {
		args = append(args, "--no-tail")
	}

	cmd := exec.Command("flyctl", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// For follow mode, we need to handle the command differently
	if followFlag {
		// When following, pipe the output directly to stdout
		cmd.Stdout = nil // Reset the buffer since we'll be streaming
		cmd.Stderr = nil

		// Set up pipes to capture and prefix the output
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			return "", fmt.Errorf("error creating stdout pipe: %v", err)
		}

		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			return "", fmt.Errorf("error creating stderr pipe: %v", err)
		}

		// Start the command
		err = cmd.Start()
		if err != nil {
			return "", fmt.Errorf("error starting command: %v", err)
		}

		// Create a WaitGroup to wait for both pipes to be processed
		var wg sync.WaitGroup
		wg.Add(2)

		// Get colorized prefix for this app
		prefix := ColorizedAppPrefix(appName)

		// Process stdout in a goroutine with proper prefixing
		go func() {
			defer wg.Done()

			// Create a scanner to read line by line
			buf := make([]byte, 1024)
			for {
				n, err := stdoutPipe.Read(buf)
				if err != nil {
					break
				}
				if n > 0 {
					lines := strings.Split(string(buf[:n]), "\n")
					for _, line := range lines {
						if line != "" {
							fmt.Printf("%s %s\n", prefix, line)
						}
					}
				}
			}
		}()

		// Process stderr in a goroutine with proper prefixing
		go func() {
			defer wg.Done()

			// Create a scanner to read line by line
			buf := make([]byte, 1024)
			for {
				n, err := stderrPipe.Read(buf)
				if err != nil {
					break
				}
				if n > 0 {
					lines := strings.Split(string(buf[:n]), "\n")
					for _, line := range lines {
						if line != "" {
							fmt.Printf("%s ERROR: %s\n", prefix, line)
						}
					}
				}
			}
		}()

		// Wait for the command to complete
		err = cmd.Wait()
		if err != nil {
			return "", fmt.Errorf("error running command: %v", err)
		}

		// Wait for both pipes to be processed
		wg.Wait()

		// In follow mode, we directly output to stdout so we return empty string for logs
		return "", nil
	} else {
		// In non-follow mode, we capture and return the logs
		err := cmd.Run()
		if err != nil {
			return "", fmt.Errorf("error running command: %v - %s", err, stderr.String())
		}

		return out.String(), nil
	}
}

// GetColorForApp returns a consistent color for a given app name
func GetColorForApp(appName string) string {
	// Use a hash function to consistently map app names to colors
	h := fnv.New32a()
	h.Write([]byte(appName))
	index := int(h.Sum32()) % len(colors)
	return colors[index]
}

// ColorizedAppPrefix returns a colorized prefix for the app name
func ColorizedAppPrefix(appName string) string {
	color := GetColorForApp(appName)
	return fmt.Sprintf("%s[%s]%s", color, appName, colorReset)
}

// GetUSRegions returns the list of US regions
func GetUSRegions() []string {
	return usRegions
}

// GetEURegions returns the list of EU regions
func GetEURegions() []string {
	return euRegions
}

// GetAppNames returns the list of application names
func GetAppNames() []string {
	return appNames
}

// GetFlyctlCallCount returns the current count of flyctl calls
func GetFlyctlCallCount() int32 {
	return atomic.LoadInt32(&flyctlCallCount)
}

// IncrementFlyctlCallCount increments the flyctl call counter and returns the new value
func IncrementFlyctlCallCount() int32 {
	return atomic.AddInt32(&flyctlCallCount, 1)
}
