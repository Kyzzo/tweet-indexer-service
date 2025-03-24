package util

import (
	"net"
	"strconv"
	"time"
)

// NetworkingService -
type NetworkingService interface {
	ResolveDNS(record string) ([]net.IP, error)
	CheckIPConnection(host string, port string) (bool, error)
	ExtractIPListFromDNS(host string, port string) ([]string, error)
	CheckIPConnectionTimeout(host string, port string, timeout time.Duration) (bool, error)
	GetBoundIPs() []string
	CheckIfBound(ip string) bool
	ExtractHost(address string) string
	ExtractPort(host string) int
}

// ResolveDNS - Resolves the DNS record to IP list
func ResolveDNS(record string) ([]net.IP, error) {
	ips, err := net.LookupIP(record)
	if err != nil {
		return nil, err
	}
	return ips, nil
}

// CheckIPConnection - Checks if IP is available
func CheckIPConnection(host string, port string) (bool, error) {
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	return true, nil
}

// ExtractIPListFromDNS - Extracts an IP list from a DNS resolve
func ExtractIPListFromDNS(host string, port string) ([]string, error) {
	ipList := []string{}
	ips, err := ResolveDNS(host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if check, err := CheckIPConnection(ip.String(), port); check && err == nil {

			ipList = append(ipList, ip.String()+":"+port)
		}
	}
	return ipList, nil
}

// CheckIPConnectionTimeout - Checks if IP is available
func CheckIPConnectionTimeout(host string, port string, timeout time.Duration) (bool, error) {
	conn, err := net.DialTimeout("tcp", host+":"+port, timeout)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	return true, nil
}

// GetBoundIPs - Returns all bound IPs on the machine
func GetBoundIPs() []string {
	addrs, _ := net.InterfaceAddrs()
	res := make([]string, 0)
	for _, addr := range addrs {
		res = append(res, addr.String())
	}
	return res
}

// CheckIfBound - Checks if the given IP is bound on the machine
func CheckIfBound(ip string) bool {
	for _, i := range GetBoundIPs() {
		if i == ip {
			return true
		}
	}
	return false
}

// ExtractHost - Returns the hostname out of a string
func ExtractHost(address string) string {
	host := SplitStrings(address, ":")[0]
	if host == "" {
		return "localhost"
	}
	return host
}

// ExtractPort - Returns the port name out of a string
func ExtractPort(host string) int {
	portStr := SplitStrings(host, ":")[1]
	port, _ := strconv.Atoi(portStr)
	return port
}

// DefaultNetworkResolver -
type DefaultNetworkResolver struct{}

// ResolveDNS -
func (r *DefaultNetworkResolver) ResolveDNS(record string) ([]net.IP, error) {
	return ResolveDNS(record)
}

// CheckIPConnection -
func (r *DefaultNetworkResolver) CheckIPConnection(host string, port string) (bool, error) {
	return CheckIPConnection(host, port)
}

// ExtractIPListFromDNS -
func (r *DefaultNetworkResolver) ExtractIPListFromDNS(host string, port string) ([]string, error) {
	return ExtractIPListFromDNS(host, port)
}

// CheckIPConnectionTimeout -
func (r *DefaultNetworkResolver) CheckIPConnectionTimeout(host string, port string, timeout time.Duration) (bool, error) {
	return CheckIPConnectionTimeout(host, port, timeout)
}

// GetBoundIPs -
func (r *DefaultNetworkResolver) GetBoundIPs() []string {
	return GetBoundIPs()
}

// CheckIfBound -
func (r *DefaultNetworkResolver) CheckIfBound(ip string) bool {
	return CheckIfBound(ip)
}

// ExtractHost -
func (r *DefaultNetworkResolver) ExtractHost(address string) string {
	return ExtractHost(address)
}

// ExtractPort -
func (r *DefaultNetworkResolver) ExtractPort(host string) int {
	return ExtractPort(host)
}
