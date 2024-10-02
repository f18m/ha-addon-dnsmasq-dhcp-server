package uibackend

import (
	"bytes"
	"net"
	"net/netip"
)

// ipInRange checks if the IP address is between dhcpStartIP and dhcpEndIP.
func IpInRange(ipOrig netip.Addr, dhcpStartIP, dhcpEndIP net.IP) bool {
	// Ensure that all IP addresses are in a consistent IPv4 or IPv6 form
	ip := net.IP(ipOrig.AsSlice()).To16()
	dhcpStartIP = dhcpStartIP.To16()
	dhcpEndIP = dhcpEndIP.To16()

	if ip == nil || dhcpStartIP == nil || dhcpEndIP == nil {
		return false
	}

	// Check if the IP address is between dhcpStartIP and dhcpEndIP
	return bytes.Compare(ip, dhcpStartIP) >= 0 && bytes.Compare(ip, dhcpEndIP) <= 0
}
