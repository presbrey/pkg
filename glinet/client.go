package glinet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Client represents a router client connection
type Client struct {
	RouterURL  string
	AuthToken  string
	HTTPClient *http.Client

	clientCache     []ClientInfo
	clientCacheMu   sync.RWMutex
	clientCacheTime time.Time
}

// RouterClient creates a new client for connecting to the router
func NewClient(routerURL, authToken string) *Client {
	return &Client{
		RouterURL:  routerURL,
		AuthToken:  authToken,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// ClientInfo represents information about a connected client device
type ClientInfo struct {
	IP             string   `json:"ip"`
	MAC            string   `json:"mac"`
	Name           string   `json:"name"`
	IPv6           []string `json:"ipv6"`
	Online         bool     `json:"online"`
	Blocked        bool     `json:"blocked"`
	Type           int      `json:"type"`
	Interface      string   `json:"iface"`
	TotalRx        int64    `json:"total_rx"`
	TotalTx        int64    `json:"total_tx"`
	TotalRxInit    int64    `json:"total_rx_init"`
	TotalTxInit    int64    `json:"total_tx_init"`
	Rx             int      `json:"rx"`
	Tx             int      `json:"tx"`
	LimitRx        int      `json:"limit_rx"`
	LimitTx        int      `json:"limit_tx"`
	LastUpdateRate int64    `json:"last_update_rate"`
	OnlineTime     int64    `json:"online_time"`
	Remote         bool     `json:"remote"`
	LastRx         []int64  `json:"last_rx"`
	LastTx         []int64  `json:"last_tx"`
}

// ClientListResponse represents the response structure from the router
type ClientListResponse struct {
	ID      int    `json:"id"`
	JSONRPC string `json:"jsonrpc"`
	Result  struct {
		Clients []ClientInfo `json:"clients"`
	} `json:"result"`
}

// GenericResponse represents a generic response from the router API
type GenericResponse struct {
	ID      int           `json:"id"`
	JSONRPC string        `json:"jsonrpc"`
	Result  []interface{} `json:"result"`
}

// Request represents the request structure to the router
type Request struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// StaticBindParams represents the parameters for adding a static IP reservation
type StaticBindParams struct {
	Name string `json:"name"`
	MAC  string `json:"mac"`
	IP   string `json:"ip"`
}

// StaticBindInfo represents information about a static IP binding
type StaticBindInfo struct {
	Name string `json:"name"`
	MAC  string `json:"mac"`
	IP   string `json:"ip"`
}

// StaticBindListResponse represents the response structure for static bindings
type StaticBindListResponse struct {
	ID      int    `json:"id"`
	JSONRPC string `json:"jsonrpc"`
	Result  struct {
		StaticBindList []StaticBindInfo `json:"static_bind_list"`
	} `json:"result"`
}

// GetClients retrieves the list of clients from the router
// Results are cached for 30 seconds to improve performance
func (c *Client) GetClients() ([]ClientInfo, error) {
	// Check cache first
	c.clientCacheMu.RLock()
	if len(c.clientCache) > 0 && time.Since(c.clientCacheTime) < 30*time.Second {
		result := make([]ClientInfo, len(c.clientCache))
		copy(result, c.clientCache)
		c.clientCacheMu.RUnlock()
		return result, nil
	}
	c.clientCacheMu.RUnlock()

	// Create request payload
	req := Request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "call",
		Params:  []interface{}{c.AuthToken, "clients", "get_list", map[string]interface{}{}},
	}

	// Marshal the request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest(http.MethodPost, c.RouterURL+"/rpc", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/plain, */*")

	// Add cookie
	cookie := &http.Cookie{
		Name:  "Admin-Token",
		Value: c.AuthToken,
	}
	httpReq.AddCookie(cookie)

	// Make the request
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Decode response
	var clientResp ClientListResponse
	if err := json.NewDecoder(resp.Body).Decode(&clientResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	// Update cache
	c.clientCacheMu.Lock()
	c.clientCache = make([]ClientInfo, len(clientResp.Result.Clients))
	copy(c.clientCache, clientResp.Result.Clients)
	c.clientCacheTime = time.Now()
	c.clientCacheMu.Unlock()

	return clientResp.Result.Clients, nil
}

// GetOnlineClients returns only the online clients
func (c *Client) GetOnlineClients() ([]ClientInfo, error) {
	clients, err := c.GetClients()
	if err != nil {
		return nil, err
	}

	var onlineClients []ClientInfo
	for _, client := range clients {
		if client.Online {
			onlineClients = append(onlineClients, client)
		}
	}

	return onlineClients, nil
}

// GetClientByMAC returns a specific client by its MAC address
func (c *Client) GetClientByMAC(mac string) (*ClientInfo, error) {
	clients, err := c.GetClients()
	if err != nil {
		return nil, err
	}

	for _, client := range clients {
		if client.MAC == mac {
			return &client, nil
		}
	}

	return nil, fmt.Errorf("client with MAC %s not found", mac)
}

// GetClientByIP returns a specific client by its IP address
func (c *Client) GetClientByIP(ip string) (*ClientInfo, error) {
	clients, err := c.GetClients()
	if err != nil {
		return nil, err
	}

	for _, client := range clients {
		if client.IP == ip {
			return &client, nil
		}
	}

	return nil, fmt.Errorf("client with IP %s not found", ip)
}

// GetClientByName returns a specific client by its name
func (c *Client) GetClientByName(name string) (*ClientInfo, error) {
	clients, err := c.GetClients()
	if err != nil {
		return nil, err
	}

	for _, client := range clients {
		if client.Name == name {
			return &client, nil
		}
	}

	return nil, fmt.Errorf("client with name %s not found", name)
}

// AddStaticBind adds a static IP address reservation for a MAC address
func (c *Client) AddStaticBind(name, mac, ip string) error {
	// Create the parameters for the reservation
	bindParams := StaticBindParams{
		Name: name,
		MAC:  mac,
		IP:   ip,
	}

	// Create request payload
	req := Request{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "call",
		Params:  []interface{}{c.AuthToken, "lan", "add_static_bind", bindParams},
	}

	// Marshal the request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("error marshaling request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest(http.MethodPost, c.RouterURL+"/rpc", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/plain, */*")

	// Add cookie
	cookie := &http.Cookie{
		Name:  "Admin-Token",
		Value: c.AuthToken,
	}
	httpReq.AddCookie(cookie)

	// Make the request
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Decode response
	var genericResp GenericResponse
	if err := json.NewDecoder(resp.Body).Decode(&genericResp); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	// The response should be {"id":4,"jsonrpc":"2.0","result":[]}
	// If result is not an empty array, something went wrong
	if genericResp.JSONRPC != "2.0" || len(genericResp.Result) != 0 {
		return fmt.Errorf("unexpected response: %+v", genericResp)
	}

	return nil
}

// GetStaticBindings retrieves the list of static IP bindings from the router
func (c *Client) GetStaticBindings() ([]StaticBindInfo, error) {
	// Create request payload
	req := Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "call",
		Params:  []interface{}{c.AuthToken, "lan", "get_static_bind_list", map[string]interface{}{}},
	}

	// Marshal the request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest(http.MethodPost, c.RouterURL+"/rpc", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/plain, */*")

	// Add cookie
	cookie := &http.Cookie{
		Name:  "Admin-Token",
		Value: c.AuthToken,
	}
	httpReq.AddCookie(cookie)

	// Make the request
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Decode response
	var bindResp StaticBindListResponse
	if err := json.NewDecoder(resp.Body).Decode(&bindResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return bindResp.Result.StaticBindList, nil
}
