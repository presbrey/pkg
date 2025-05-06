package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/presbrey/pkg/fly"
)

// Command-line flags for logs command
type LogsFlags struct {
	follow   bool
	usOnly   bool
	euOnly   bool
	numLines int
	appName  string
}

// LogResult contains the logs and metadata for a machine
type LogResult struct {
	AppName     string
	MachineID   string
	MachineName string
	Logs        string
	Error       error
}

// Command-line flags for list command
type ListFlags struct {
	usOnly  bool
	euOnly  bool
	quiet   bool
	appName string
}

// MachineResult holds the result of a machine query
type MachineResult struct {
	AppName      string
	Region       string
	Output       string
	MachineCount int
	Error        error
}

// printHorizontalRule prints a horizontal rule
func printHorizontalRule() {
	fmt.Println(strings.Repeat("-", 120))
}

// formatTimestamp formats a Unix timestamp from fly.io into a readable string
func formatTimestamp(ts int64) string {
	return fmt.Sprintf("%d-%03d::%02d:%02d",
		ts/1000000,
		(ts/1000)%1000,
		(ts/10)%100,
		ts%10)
}

// padToWidth ensures a string is exactly the specified width by padding or truncating
func padToWidth(s string, width int) string {
	if len(s) > width {
		return s[:width-3] + "..."
	}
	return s + strings.Repeat(" ", width-len(s))
}

// centerText centers text in a field of given width
func centerText(text string, width int) string {
	if len(text) >= width {
		return text
	}

	leftPad := (width - len(text)) / 2
	rightPad := width - len(text) - leftPad

	return strings.Repeat(" ", leftPad) + text + strings.Repeat(" ", rightPad)
}

// prefixLogLines adds the app name as a prefix to each line of logs
func prefixLogLines(appName, logs string) string {
	prefix := fly.ColorizedAppPrefix(appName)
	var result strings.Builder

	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		if line != "" {
			result.WriteString(prefix)
			result.WriteString(" ")
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	return result.String()
}

// processMachineLogs processes logs for all machines of a specific app
func processMachineLogs(appName string, resultChan chan<- LogResult, wg *sync.WaitGroup, followFlag bool) {
	defer wg.Done()

	// Get list of machines for this app
	machines, err := fly.GetMachineList(appName)
	if err != nil {
		resultChan <- LogResult{
			AppName: appName,
			Error:   err,
		}
		return
	}

	// If no machines found, send empty result
	if len(machines) == 0 {
		resultChan <- LogResult{
			AppName: appName,
			Logs:    "No machines found for this app.",
		}
		return
	}

	// For each machine, fetch logs
	for _, machine := range machines {
		// Only process started machines
		if machine.State != "started" {
			continue
		}

		// Get logs for this machine
		logs, err := fly.GetMachineLogs(appName, machine.ID, followFlag)
		if err != nil {
			resultChan <- LogResult{
				AppName:   appName,
				MachineID: machine.ID,
				Error:     err,
			}
			continue
		}

		// Send the result
		resultChan <- LogResult{
			AppName:     appName,
			MachineID:   machine.ID,
			MachineName: machine.Name,
			Logs:        logs,
		}

		// When following, only process one machine per app to avoid flooding the terminal
		if followFlag {
			break
		}
	}
}

// getMachineDetails gets the machine details for a specific app
func getMachineDetails(appName string) (string, int, error) {
	// Increment the global flyctl call counter
	fly.IncrementFlyctlCallCount()

	cmd := exec.Command("flyctl", "machine", "list", "--json", "-a", appName)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return "Not found or error", 0, nil
	}

	var machines []fly.Machine
	err = json.Unmarshal(out.Bytes(), &machines)
	if err != nil {
		return fmt.Sprintf("Error parsing JSON: %v", err), 0, nil
	}

	if len(machines) == 0 {
		return "No machines", 0, nil
	}

	// Format the output
	var result strings.Builder
	for _, m := range machines {
		// Format the timestamp
		var timeStr string
		if len(m.Events) > 0 {
			timeStr = formatTimestamp(m.Events[0].Timestamp)
		} else {
			timeStr = "N/A"
		}

		// Extract deployment ID (truncated)
		deployID := m.ImageRef.Tag
		if len(deployID) > 14 && strings.HasPrefix(deployID, "deployment-") {
			deployID = deployID[11:25]
		}

		// Format the machine details
		fmt.Fprintf(&result, "%s [%s] %s in %s • %dCPU/%dMB • %s\n",
			m.Name,
			m.ID[:8],
			m.State,
			m.Region,
			m.Config.Guest.CPUs,
			m.Config.Guest.MemoryMB,
			deployID)

		if len(m.Events) > 0 {
			fmt.Fprintf(&result, "  Event: %s/%s @ %s\n",
				m.Events[0].Type,
				m.Events[0].Status,
				timeStr)
		}
	}

	return result.String(), len(machines), nil
}

// collectMachineData collects data for all machines in parallel
func collectMachineData(regions []string) (map[string]map[string]MachineResult, int) {
	results := make(map[string]map[string]MachineResult)
	totalMachines := 0
	var mutex sync.Mutex
	var wg sync.WaitGroup

	// Initialize results map
	for _, region := range regions {
		results[region] = make(map[string]MachineResult)
	}

	// Launch goroutines to query fly.io
	for _, region := range regions {
		for _, appType := range fly.GetAppNames() {
			wg.Add(1)
			go func(r, appType string) {
				defer wg.Done()

				appName := r + "-" + appType
				output, count, err := getMachineDetails(appName)

				mutex.Lock()
				results[r][appType] = MachineResult{
					AppName:      appName,
					Region:       r,
					Output:       output,
					MachineCount: count,
					Error:        err,
				}
				totalMachines += count
				mutex.Unlock()
			}(region, appType)
		}
	}

	// Wait for all queries to complete
	wg.Wait()

	return results, totalMachines
}

// displayRegionData displays data for a set of regions
func displayRegionData(regions []string, regionName string, results map[string]map[string]MachineResult, quiet bool) {
	// Print section header
	fmt.Printf("\n%s REGIONS:\n", strings.ToUpper(regionName))

	if !quiet {
		// Print column headers
		const colWidth = 80

		// Create centered headers with dynamic app names
		appNames := fly.GetAppNames()
		appHeaders := make([]string, len(appNames))
		for i, appName := range appNames {
			appHeaders[i] = centerText(strings.ToUpper(appName), colWidth)
		}

		// Build header format string dynamically based on number of apps
		headerFormat := "%-10s"
		for range appNames {
			headerFormat += " | %s"
		}
		headerFormat += "\n"

		// Create args for Printf, starting with "REGION" and followed by app headers
		args := make([]interface{}, len(appNames)+1)
		args[0] = "REGION"
		for i, header := range appHeaders {
			args[i+1] = header
		}

		fmt.Printf(headerFormat, args...)
		printHorizontalRule()
	}

	// Calculate total machines for this region set
	regionTotal := 0
	for _, r := range regions {
		for _, appType := range fly.GetAppNames() {
			if result, ok := results[r][appType]; ok {
				regionTotal += result.MachineCount
			}
		}
	}

	// If no machines in this region set and quiet mode, skip it
	if regionTotal == 0 && quiet {
		fmt.Printf("No machines found in %s regions.\n", regionName)
		return
	}

	// Print data for each region
	for _, r := range regions {
		if quiet {
			// In quiet mode, just print the machine counts
			appNames := fly.GetAppNames()
			counts := make([]int, len(appNames))
			hasAnyMachines := false

			// Get counts for each app type
			for i, appName := range appNames {
				if result, ok := results[r][appName]; ok {
					counts[i] = result.MachineCount
					if result.MachineCount > 0 {
						hasAnyMachines = true
					}
				}
			}

			// Only print if there are any machines
			if hasAnyMachines {
				output := fmt.Sprintf("%s:", r)
				for i, appName := range appNames {
					output += fmt.Sprintf(" %d %s,", counts[i], appName)
				}
				// Remove trailing comma
				output = output[:len(output)-1]
				fmt.Println(output)
			}
		} else {
			// Get outputs for all application types
			appNames := fly.GetAppNames()
			outputs := make([]string, len(appNames))

			// Get output for each app type
			for i, appName := range appNames {
				outputs[i] = "No data"
				if result, ok := results[r][appName]; ok {
					outputs[i] = result.Output
				}
			}

			// Build format string dynamically based on number of apps
			rowFormat := "%-10s"
			for range appNames {
				rowFormat += " | %s"
			}
			rowFormat += "\n"

			// Create args for Printf, starting with region and followed by outputs
			args := make([]interface{}, len(appNames)+1)
			args[0] = r
			for i, output := range outputs {
				args[i+1] = output
			}

			// Print the row
			fmt.Printf(rowFormat, args...)
			printHorizontalRule()
		}
	}
}

// runLogsCommand runs the logs subcommand
func runLogsCommand(args []string) {
	// Parse flags for the logs command
	logsFlags := LogsFlags{}
	logsCmd := flag.NewFlagSet("logs", flag.ExitOnError)
	logsCmd.BoolVar(&logsFlags.follow, "f", false, "Follow logs")
	logsCmd.BoolVar(&logsFlags.usOnly, "us", false, "Show only US regions")
	logsCmd.BoolVar(&logsFlags.euOnly, "eu", false, "Show only EU regions")
	logsCmd.IntVar(&logsFlags.numLines, "n", 100, "Number of lines to show")
	logsCmd.StringVar(&logsFlags.appName, "a", "", "Specific app name to target")

	logsCmd.Parse(args)

	// Determine regions based on flags
	regions := append(fly.GetUSRegions(), fly.GetEURegions()...)
	if logsFlags.usOnly && !logsFlags.euOnly {
		regions = fly.GetUSRegions()
	} else if logsFlags.euOnly && !logsFlags.usOnly {
		regions = fly.GetEURegions()
	}

	// Generate app names (e.g., "us-east-1-portal", "eu-west-2-websocket", etc.)
	var fullAppNames []string

	// If a specific app name is provided, use it directly
	if logsFlags.appName != "" {
		fullAppNames = []string{logsFlags.appName}
	} else {
		// Otherwise, generate all combinations
		for _, region := range regions {
			for _, appName := range fly.GetAppNames() {
				fullAppNames = append(fullAppNames, region+"-"+appName)
			}
		}
	}

	// Create a channel for results and a WaitGroup to synchronize goroutines
	resultChan := make(chan LogResult, len(fullAppNames))
	var wg sync.WaitGroup

	// Start processing each app's logs
	fmt.Println("Fetching logs for all machines...")
	fmt.Printf("Regions: %s\n", strings.Join(regions, ", "))
	printHorizontalRule()

	for _, appName := range fullAppNames {
		wg.Add(1)
		go processMachineLogs(appName, resultChan, &wg, logsFlags.follow)
	}

	// Create a separate goroutine to close the channel when all processing is done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results as they come in
	for result := range resultChan {
		if result.Error != nil {
			log.Printf("Error processing %s: %v\n", result.AppName, result.Error)
			continue
		}

		if !logsFlags.follow && result.Logs != "" {
			// Print logs with proper prefixing
			output := prefixLogLines(result.AppName, result.Logs)
			fmt.Print(output)
			printHorizontalRule()
		}
	}

	fmt.Printf("Processed %d flyctl calls.\n", fly.GetFlyctlCallCount())
}

// runListCommand runs the list subcommand
func runListCommand(args []string) {
	// Parse flags for the list command
	listFlags := ListFlags{}
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	listCmd.BoolVar(&listFlags.usOnly, "us", false, "Show only US regions")
	listCmd.BoolVar(&listFlags.euOnly, "eu", false, "Show only EU regions")
	listCmd.BoolVar(&listFlags.quiet, "q", false, "Quiet mode (show only counts)")
	listCmd.StringVar(&listFlags.appName, "a", "", "Specific app name to target")

	listCmd.Parse(args)

	// Start collecting data in parallel
	startTime := time.Now()
	fmt.Println("Fetching machine data from fly.io...")

	// Handle the case of a specific app name
	if listFlags.appName != "" {
		// For a specific app, we don't need to collect data for all regions
		fmt.Printf("Fetching data for app: %s\n", listFlags.appName)

		// Direct call to get machine details for the specific app
		output, count, err := getMachineDetails(listFlags.appName)

		if err != nil {
			fmt.Printf("Error fetching data for %s: %v\n", listFlags.appName, err)
		} else {
			fmt.Printf("\nFound %d machines for app %s (in %.2f seconds).\n",
				count,
				listFlags.appName,
				time.Since(startTime).Seconds())

			if count > 0 {
				fmt.Println("\nAPP DETAILS:")
				fmt.Println(output)
			}
		}

		fmt.Printf("\nProcessed %d flyctl calls.\n", fly.GetFlyctlCallCount())
		return
	}

	// Determine which regions to query for the normal case (no specific app)
	var regionsToQuery []string
	if !listFlags.usOnly && !listFlags.euOnly {
		// Default: query all regions
		regionsToQuery = append(regionsToQuery, fly.GetUSRegions()...)
		regionsToQuery = append(regionsToQuery, fly.GetEURegions()...)
	} else {
		// Query based on flags
		if listFlags.usOnly {
			regionsToQuery = append(regionsToQuery, fly.GetUSRegions()...)
		}
		if listFlags.euOnly {
			regionsToQuery = append(regionsToQuery, fly.GetEURegions()...)
		}
	}

	// Collect data for all regions
	results, totalMachines := collectMachineData(regionsToQuery)

	// Print results
	fmt.Printf("\nFound %d machines across %d regions (in %.2f seconds).\n",
		totalMachines,
		len(regionsToQuery),
		time.Since(startTime).Seconds())

	// Display US regions data
	if listFlags.usOnly || !listFlags.euOnly {
		displayRegionData(fly.GetUSRegions(), "US", results, listFlags.quiet)
	}

	// Display EU regions data
	if listFlags.euOnly || !listFlags.usOnly {
		displayRegionData(fly.GetEURegions(), "EU", results, listFlags.quiet)
	}

	fmt.Printf("\nProcessed %d flyctl calls.\n", fly.GetFlyctlCallCount())
}

func main() {
	// Check if we have at least one argument (the subcommand)
	if len(os.Args) < 2 {
		fmt.Println("Usage: flysu <command> [options]")
		fmt.Println("Commands:")
		fmt.Println("  list    List all fly machines across regions")
		fmt.Println("  logs    Show logs from fly machines across regions")
		os.Exit(1)
	}

	// Get the subcommand
	command := os.Args[1]

	// Parse the remaining arguments
	args := os.Args[2:]

	// Run the appropriate command
	switch command {
	case "list":
		runListCommand(args)
	case "logs":
		runLogsCommand(args)
	case "help":
		fmt.Println("Usage: flysu <command> [options]")
		fmt.Println("Commands:")
		fmt.Println("  list    List all fly machines across regions")
		fmt.Println("    -us   Show only US regions")
		fmt.Println("    -eu   Show only EU regions")
		fmt.Println("    -q    Quiet mode (show only counts)")
		fmt.Println("    -a    Specific app name to target")
		fmt.Println("")
		fmt.Println("  logs    Show logs from fly machines across regions")
		fmt.Println("    -f    Follow logs (tail)")
		fmt.Println("    -us   Show only US regions")
		fmt.Println("    -eu   Show only EU regions")
		fmt.Println("    -n N  Number of lines to show (default: 100)")
		fmt.Println("    -a    Specific app name to target")
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Run 'flysu help' for usage information")
		os.Exit(1)
	}
}
