package uibackend

import (
	"bytes"
	"fmt"
	"net"
	"net/netip"
	"time"
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

func LeaseTimeToString(t time.Time) string {

	if t.IsZero() {
		return "Never expires"
	}

	now := time.Now()
	duration := t.Sub(now)
	if duration < 0 {
		return "Expired"
	}

	// compute hours, min, secs
	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%02dd, %02dh, %02dm, %02ds", days, hours, minutes, seconds)
	} else if hours > 0 {
		return fmt.Sprintf("%02dh, %02dm, %02ds", hours, minutes, seconds)
	} else {
		return fmt.Sprintf("%02dm, %02ds", minutes, seconds)
	}
}
