package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
	"github.com/presbrey/pkg/glinet"
)

var (
	flagRouterURL = flag.String("router-url", "", "Router URL")
	flagAuthToken = flag.String("auth-token", "", "Router authentication token")

	flagImportCSV = flag.String("import-csv", "", "CSV file containing MAC addresses and IP addresses")
	flagImportARP = flag.String("import-arp", "", "ARP table file from Linux containing IP and MAC addresses")
	flagClientList = flag.String("client-list", "", "CSV file containing known client hostnames for MAC addresses")
	flagDryRun    = flag.Bool("dry-run", false, "Parse the input without making changes to the router")
)

// loadClientList loads a client list CSV file and returns a map of MAC addresses to hostnames
func loadClientList(clientListPath string) (map[string]string, error) {
	// Open the client list file
	file, err := os.Open(clientListPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open client list file: %w", err)
	}
	defer file.Close()

	// Create a new CSV reader
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields per record

	// Read the header row
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read client list header: %w", err)
	}

	// Find the indices of the columns we need
	nameIdx := -1
	macIdx := -1
	for i, col := range header {
		switch col {
		case "USERNAME":
			nameIdx = i
		case "MAC ADDRESS":
			macIdx = i
		}
	}

	// Check if we found all required columns
	if nameIdx == -1 || macIdx == -1 {
		return nil, fmt.Errorf("client list CSV file missing required columns: USERNAME and/or MAC ADDRESS")
	}

	// Create a map to store MAC to hostname mappings
	macToHostname := make(map[string]string)

	// Process each row
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading client list row: %w", err)
		}

		// Skip rows that don't have enough columns
		if len(row) <= macIdx || len(row) <= nameIdx {
			continue
		}

		// Extract hostname and MAC address
		hostname := strings.Trim(row[nameIdx], "\"")
		macAddress := strings.Trim(row[macIdx], "\"")

		// Normalize MAC address format (convert to lowercase, remove hyphens)
		macAddress = normalizeMACAddress(macAddress)

		// Add to map if hostname is not empty and not the same as MAC
		if hostname != "" && hostname != macAddress {
			macToHostname[macAddress] = hostname
		}
	}

	return macToHostname, nil
}

// normalizeMACAddress standardizes MAC address format for consistent comparison
func normalizeMACAddress(mac string) string {
	// Convert to lowercase
	mac = strings.ToLower(mac)
	// Replace hyphens with colons if present
	mac = strings.ReplaceAll(mac, "-", ":")
	return mac
}

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Failed to load environment variables: %v", err)
	}
	flag.Parse()

	// Use environment variables if flags are not provided
	if *flagAuthToken == "" {
		*flagAuthToken = os.Getenv("GLINET_AUTH_TOKEN")
	}
	if *flagRouterURL == "" {
		*flagRouterURL = os.Getenv("GLINET_ROUTER_URL")
	}
	if *flagAuthToken == "" || *flagRouterURL == "" {
		log.Fatal("Router authentication token and URL are required")
	}

	// Create a new router client
	client := glinet.NewClient(*flagRouterURL, *flagAuthToken)

	// Load client list if specified
	var clientList map[string]string
	if *flagClientList != "" {
		log.Printf("Loading client list from %s", *flagClientList)
		var err error
		clientList, err = loadClientList(*flagClientList)
		if err != nil {
			log.Fatalf("Error loading client list: %v", err)
		}
		log.Printf("Loaded %d client hostnames", len(clientList))
	}

	switch {
	case *flagImportCSV != "":
		// Import static IP reservations from CSV
		if err := importCSV(*flagImportCSV, client, *flagDryRun, clientList); err != nil {
			log.Fatalf("Error importing CSV: %v", err)
		}
	case *flagImportARP != "":
		// Import static IP reservations from Linux ARP table
		if err := importARP(*flagImportARP, client, *flagDryRun, clientList); err != nil {
			log.Fatalf("Error importing ARP table: %v", err)
		}
	}
}

// addStaticBinding is a helper function to add a static IP binding to the router
// It checks if the binding already exists and skips it if it does
func addStaticBinding(client *glinet.Client, deviceName, macAddress, ipAddress string, dryRun bool, existingBindings map[string]glinet.StaticBindInfo) error {
	// Check if the MAC address already has a static binding
	if existingBind, exists := existingBindings[macAddress]; exists {
		log.Printf("SKIPPING: Static IP reservation already exists for MAC %s (%s) with IP %s",
			macAddress, existingBind.Name, existingBind.IP)
		return nil
	}

	if dryRun {
		log.Printf("DRY RUN: Would add static IP reservation for %s (%s) to IP %s",
			deviceName, macAddress, ipAddress)
		return nil
	}

	log.Printf("Adding static IP reservation for %s (%s) to IP %s",
		deviceName, macAddress, ipAddress)

	err := client.AddStaticBind(deviceName, macAddress, ipAddress)
	if err != nil {
		return fmt.Errorf("error adding static IP reservation for %s: %w", deviceName, err)
	}

	log.Printf("Successfully added static IP reservation for %s (%s) to IP %s",
		deviceName, macAddress, ipAddress)
	return nil
}

func importCSV(csvPath string, client *glinet.Client, dryRun bool, clientList map[string]string) error {
	if dryRun {
		log.Printf("DRY RUN: Parsing CSV file %s without making changes", csvPath)
	} else {
		log.Printf("Importing static IP reservations from %s", csvPath)
	}

	// Get existing static bindings
	log.Printf("Fetching existing static bindings from router...")
	existingBindings := make(map[string]glinet.StaticBindInfo)
	if !dryRun {
		bindings, err := client.GetStaticBindings()
		if err != nil {
			return fmt.Errorf("failed to get static bindings from router: %w", err)
		}
		log.Printf("Found %d existing static bindings", len(bindings))

		// Create a map of MAC to binding info for quick lookups
		for _, binding := range bindings {
			existingBindings[binding.MAC] = binding
		}
	}

	// Open the CSV file
	file, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	// Create a new CSV reader with flexible field count
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields per record

	// Read the header row
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	log.Printf("Found CSV headers: %v", header)

	// Find the indices of the columns we need
	nameIdx := -1
	ipIdx := -1
	statusIdx := -1
	for i, col := range header {
		switch col {
		case "USERNAME":
			nameIdx = i
		case "IP ADDRESS":
			ipIdx = i
		case "STATUS":
			statusIdx = i
		}
	}

	// Check if we found all required columns
	if nameIdx == -1 || ipIdx == -1 {
		return fmt.Errorf("CSV file missing required columns: USERNAME and/or IP ADDRESS")
	}

	// Map to store IP to MAC mappings we've seen
	ipToMac := make(map[string]string)

	// Only fetch clients from router if not in dry-run mode
	if !dryRun {
		// First, get all current clients to have their MAC addresses
		log.Printf("Fetching current clients from router...")
		allClients, err := client.GetClients()
		if err != nil {
			return fmt.Errorf("failed to get clients from router: %w", err)
		}
		log.Printf("Found %d clients connected to the router", len(allClients))

		// Create a map of IP to client info for quick lookups
		for _, c := range allClients {
			ipToMac[c.IP] = c.MAC
		}
	}

	// Process each row
	var successCount, failCount, skippedCount int
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading CSV row: %w", err)
		}

		// Skip rows that aren't connected if status column exists
		if statusIdx != -1 && row[statusIdx] != "CONNECTED" {
			log.Printf("Skipping device with status: %s", row[statusIdx])
			skippedCount++
			continue
		}

		// Extract device name and IP address
		csvDeviceName := strings.Trim(row[nameIdx], "\"")
		ipAddress := strings.Trim(row[ipIdx], "\"")

		log.Printf("Processing device: %s with IP: %s", csvDeviceName, ipAddress)

		// In dry-run mode, generate a fake MAC address based on the IP
		var macAddress string
		if dryRun {
			// Generate a deterministic MAC address from the IP for testing
			ipParts := strings.Split(ipAddress, ".")
			if len(ipParts) != 4 {
				log.Printf("Warning: Invalid IP format: %s", ipAddress)
				failCount++
				continue
			}
			// Create a MAC address using 00:00:00: prefix and the last 3 octets of the IP
			// Ensure each octet is properly formatted with leading zeros if needed
			macAddress = fmt.Sprintf("00:00:00:%02s:%02s:%02s", ipParts[1], ipParts[2], ipParts[3])
			log.Printf("DRY RUN: Generated MAC address %s for IP %s", macAddress, ipAddress)
		} else {
			// Look up the client by IP in our map
			mac, found := ipToMac[ipAddress]
			if !found {
				log.Printf("Warning: Could not find device with IP %s in router clients", ipAddress)
				failCount++
				continue
			}
			macAddress = mac
		}

		// Determine the device name to use
		// Start with the name from the CSV
		deviceName := csvDeviceName
		
		// Check if we have a better name in the client list
		if clientList != nil {
			normalizedMAC := normalizeMACAddress(macAddress)
			if hostname, exists := clientList[normalizedMAC]; exists {
				deviceName = hostname
				log.Printf("Using hostname '%s' from client list for MAC %s", deviceName, macAddress)
			}
		}

		// Add static binding using the MAC address
		err = addStaticBinding(client, deviceName, macAddress, ipAddress, dryRun, existingBindings)
		if err != nil {
			log.Printf("%v", err)
			failCount++
		} else {
			successCount++
		}
	}

	if dryRun {
		log.Printf("DRY RUN complete: %d would succeed, %d would fail, %d skipped",
			successCount, failCount, skippedCount)
	} else {
		log.Printf("Import complete: %d successful, %d failed, %d skipped",
			successCount, failCount, skippedCount)
	}
	return nil
}

// importARP imports static IP reservations from a Linux ARP table file
func importARP(arpPath string, client *glinet.Client, dryRun bool, clientList map[string]string) error {
	if dryRun {
		log.Printf("DRY RUN: Parsing ARP table file %s without making changes", arpPath)
	} else {
		log.Printf("Importing static IP reservations from ARP table %s", arpPath)
	}

	// Get existing static bindings
	log.Printf("Fetching existing static bindings from router...")
	existingBindings := make(map[string]glinet.StaticBindInfo)
	if !dryRun {
		bindings, err := client.GetStaticBindings()
		if err != nil {
			return fmt.Errorf("failed to get static bindings from router: %w", err)
		}
		log.Printf("Found %d existing static bindings", len(bindings))

		// Create a map of MAC to binding info for quick lookups
		for _, binding := range bindings {
			existingBindings[binding.MAC] = binding
		}
	}

	// Open the ARP file
	file, err := os.Open(arpPath)
	if err != nil {
		return fmt.Errorf("failed to open ARP file: %w", err)
	}
	defer file.Close()

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	var successCount, failCount int

	// Regular expression to parse ARP table entries
	// Format: ? (10.72.6.120) at 94:a6:7e:3c:d1:0f [ether] on enp3s0
	arpRegex := regexp.MustCompile(`\? \(([0-9.]+)\) at ([0-9a-f:]+) \[ether\] on (\S+)`)

	// Process each line
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse the line using regex
		matches := arpRegex.FindStringSubmatch(line)
		if matches == nil || len(matches) < 3 {
			log.Printf("Warning: Could not parse ARP entry: %s", line)
			failCount++
			continue
		}

		// Extract IP and MAC address
		ipAddress := matches[1]
		macAddress := matches[2]
		interface_ := ""
		if len(matches) > 3 {
			interface_ = matches[3]
		}

		// Determine the device name to use
		deviceName := ""
		
		// First check if we have a hostname in the client list
		if clientList != nil {
			normalizedMAC := normalizeMACAddress(macAddress)
			if hostname, exists := clientList[normalizedMAC]; exists {
				deviceName = hostname
				log.Printf("Using hostname '%s' from client list for MAC %s", deviceName, macAddress)
			}
		}
		
		// If no hostname found, use MAC address with hyphens as the device name
		if deviceName == "" {
			deviceName = strings.ReplaceAll(macAddress, ":", "-")
		}

		log.Printf("Processing ARP entry: IP=%s, MAC=%s, Interface=%s",
			ipAddress, macAddress, interface_)

		// Add static binding
		err = addStaticBinding(client, deviceName, macAddress, ipAddress, dryRun, existingBindings)
		if err != nil {
			log.Printf("%v", err)
			failCount++
		} else {
			successCount++
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading ARP file: %w", err)
	}

	if dryRun {
		log.Printf("DRY RUN complete: %d would succeed, %d would fail",
			successCount, failCount)
	} else {
		log.Printf("Import complete: %d successful, %d failed",
			successCount, failCount)
	}

	return nil
}
