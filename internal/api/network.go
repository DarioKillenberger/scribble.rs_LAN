package api

import (
	"net"
	"net/http"
	"sort"
	"strings"
)

type lanNetworkAddressesResponse struct {
	Origins          []string `json:"origins"`
	Addresses        []string `json:"addresses"`
	UsingRequestHost bool     `json:"usingRequestHost"`
}

func (handler *V1Handler) getLanNetworkAddresses(writer http.ResponseWriter, request *http.Request) {
	scheme := requestScheme(request)
	host := request.Host
	if host == "" {
		host = request.URL.Host
	}

	response := lanNetworkAddressesResponse{}
	if host != "" && !isLocalhostHost(host) {
		response.Origins = append(response.Origins, scheme+"://"+host)
		response.UsingRequestHost = true
	}

	hostPort := hostPort(host, scheme)
	for _, address := range discoverLanIPv4Addresses() {
		response.Addresses = append(response.Addresses, address)
		response.Origins = append(response.Origins, scheme+"://"+net.JoinHostPort(address, hostPort))
	}
	response.Origins = compactStringsPreservingOrder(response.Origins)
	response.Addresses = compactStringsPreservingOrder(response.Addresses)

	_, _ = marshalToHTTPWriter(response, writer)
}

func requestScheme(request *http.Request) string {
	if forwardedProto := strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		return strings.Split(forwardedProto, ",")[0]
	}
	if request.TLS != nil {
		return "https"
	}
	return "http"
}

func hostPort(host string, scheme string) string {
	_, port, err := net.SplitHostPort(host)
	if err == nil && port != "" {
		return port
	}
	if scheme == "https" {
		return "443"
	}
	return "80"
}

func isLocalhostHost(host string) bool {
	name, _, err := net.SplitHostPort(host)
	if err == nil {
		host = name
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func discoverLanIPv4Addresses() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	type candidate struct {
		address string
		score   int
	}
	candidates := make([]candidate, 0)
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := networkInterface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := ipFromAddr(addr)
			if ip == nil {
				continue
			}
			score := lanAddressScore(networkInterface.Name, ip)
			if score < 0 {
				continue
			}
			candidates = append(candidates, candidate{address: ip.String(), score: score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].address < candidates[j].address
		}
		return candidates[i].score > candidates[j].score
	})

	addresses := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		addresses = append(addresses, candidate.address)
	}
	return compactStringsPreservingOrder(addresses)
}

func ipFromAddr(addr net.Addr) net.IP {
	var ip net.IP
	switch typed := addr.(type) {
	case *net.IPNet:
		ip = typed.IP
	case *net.IPAddr:
		ip = typed.IP
	default:
		return nil
	}
	ip = ip.To4()
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return nil
	}
	return ip
}

func lanAddressScore(interfaceName string, ip net.IP) int {
	score := 0
	if isPrivateIPv4(ip) {
		score += 100
	}
	if isLikelyVirtualInterface(interfaceName) {
		score -= 50
	}
	if strings.HasPrefix(ip.String(), "172.") {
		score -= 5
	}
	return score
}

func isPrivateIPv4(ip net.IP) bool {
	return ip[0] == 10 ||
		(ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31) ||
		(ip[0] == 192 && ip[1] == 168)
}

func isLikelyVirtualInterface(name string) bool {
	name = strings.ToLower(name)
	virtualMarkers := []string{"docker", "veth", "br-", "bridge", "virtualbox", "vmware", "hyper-v", "wsl", "vethernet"}
	for _, marker := range virtualMarkers {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func compactStringsPreservingOrder(values []string) []string {
	seen := make(map[string]bool, len(values))
	compacted := values[:0]
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		compacted = append(compacted, value)
	}
	return compacted
}
