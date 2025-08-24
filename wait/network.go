package wait

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

// ForNetwork waits for network connectivity (global unicast IP)
func ForNetwork(opts ...*Options) error {
	return Until(func() (bool, error) {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return false, nil // Ignore error and retry
		}
		
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.IsGlobalUnicast() {
				return true, nil
			}
		}
		return false, nil
	}, opts...)
}

// ForLocalNetwork waits for any network connectivity (including local)
func ForLocalNetwork(opts ...*Options) error {
	return Until(func() (bool, error) {
		interfaces, err := net.Interfaces()
		if err != nil {
			return false, nil
		}
		
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
				addrs, err := iface.Addrs()
				if err != nil {
					continue
				}
				if len(addrs) > 0 {
					return true, nil
				}
			}
		}
		return false, nil
	}, opts...)
}

// ForTCP waits until a TCP connection can be established
func ForTCP(address string, opts ...*Options) error {
	return Until(func() (bool, error) {
		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			return false, nil // Ignore error and retry
		}
		conn.Close()
		return true, nil
	}, opts...)
}

// ForUDP waits until a UDP connection can be established
func ForUDP(address string, opts ...*Options) error {
	return Until(func() (bool, error) {
		conn, err := net.DialTimeout("udp", address, 5*time.Second)
		if err != nil {
			return false, nil
		}
		conn.Close()
		return true, nil
	}, opts...)
}

// ForHTTP waits until an HTTP endpoint returns a successful response
func ForHTTP(url string, opts ...*Options) error {
	return ForHTTPStatus(url, []int{200}, opts...)
}

// ForHTTPStatus waits until an HTTP endpoint returns one of the expected status codes
func ForHTTPStatus(url string, expectedStatuses []int, opts ...*Options) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	return Until(func() (bool, error) {
		resp, err := client.Get(url)
		if err != nil {
			return false, nil // Ignore error and retry
		}
		defer resp.Body.Close()
		
		for _, status := range expectedStatuses {
			if resp.StatusCode == status {
				return true, nil
			}
		}
		return false, nil
	}, opts...)
}

// ForHTTPSHealthy waits until an HTTP endpoint returns any 2xx status
func ForHTTPSHealthy(url string, opts ...*Options) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	return Until(func() (bool, error) {
		resp, err := client.Get(url)
		if err != nil {
			return false, nil
		}
		defer resp.Body.Close()
		
		return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
	}, opts...)
}

// ForDNS waits until a hostname can be resolved
func ForDNS(hostname string, opts ...*Options) error {
	return Until(func() (bool, error) {
		_, err := net.LookupHost(hostname)
		return err == nil, nil
	}, opts...)
}

// ForPort waits until a port is open on localhost
func ForPort(port int, opts ...*Options) error {
	return ForTCP(fmt.Sprintf("localhost:%d", port), opts...)
}

// ForMultiplePorts waits until all specified ports are open on localhost
func ForMultiplePorts(ports []int, opts ...*Options) error {
	conditions := make([]ConditionFunc, len(ports))
	for i, port := range ports {
		p := port // Capture loop variable
		conditions[i] = func() (bool, error) {
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", p), 5*time.Second)
			if err != nil {
				return false, nil
			}
			conn.Close()
			return true, nil
		}
	}
	return All(conditions, opts...)
}

// ForAnyPort waits until any of the specified ports is open on localhost
func ForAnyPort(ports []int, opts ...*Options) (int, error) {
	conditions := make([]ConditionFunc, len(ports))
	for i, port := range ports {
		p := port // Capture loop variable
		conditions[i] = func() (bool, error) {
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", p), 5*time.Second)
			if err != nil {
				return false, nil
			}
			conn.Close()
			return true, nil
		}
	}
	
	index, err := Any(conditions, opts...)
	if err != nil {
		return -1, err
	}
	return ports[index], nil
}

// NetworkInfo contains information about network interfaces
type NetworkInfo struct {
	Interfaces []InterfaceInfo
	HasIPv4    bool
	HasIPv6    bool
	HasGlobal  bool
}

// InterfaceInfo contains information about a network interface
type InterfaceInfo struct {
	Name      string
	Addresses []net.Addr
	IsUp      bool
	IsLoopback bool
}

// ForNetworkWithInfo waits for network and returns network information
func ForNetworkWithInfo(opts ...*Options) (*NetworkInfo, error) {
	result, err := UntilWithResult(func() (interface{}, bool, error) {
		info := &NetworkInfo{
			Interfaces: []InterfaceInfo{},
		}
		
		interfaces, err := net.Interfaces()
		if err != nil {
			return nil, false, nil
		}
		
		for _, iface := range interfaces {
			ifaceInfo := InterfaceInfo{
				Name:       iface.Name,
				IsUp:       iface.Flags&net.FlagUp != 0,
				IsLoopback: iface.Flags&net.FlagLoopback != 0,
			}
			
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			
			ifaceInfo.Addresses = addrs
			
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok {
					if ipnet.IP.To4() != nil {
						info.HasIPv4 = true
					} else {
						info.HasIPv6 = true
					}
					
					if ipnet.IP.IsGlobalUnicast() {
						info.HasGlobal = true
					}
				}
			}
			
			info.Interfaces = append(info.Interfaces, ifaceInfo)
		}
		
		return info, info.HasGlobal, nil
	}, opts...)
	
	if err != nil {
		return nil, err
	}
	
	return result.(*NetworkInfo), nil
}