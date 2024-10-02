package uibackend

import (
	"net"
	"net/netip"
	"testing"
)

func TestIPInRange(t *testing.T) {
	tests := []struct {
		name        string
		ip          string
		dhcpStartIP string
		dhcpEndIP   string
		expected    bool
	}{
		{
			name:        "IP within range",
			ip:          "192.168.1.10",
			dhcpStartIP: "192.168.1.1",
			dhcpEndIP:   "192.168.1.100",
			expected:    true,
		},
		{
			name:        "IP equal to start of range",
			ip:          "192.168.1.1",
			dhcpStartIP: "192.168.1.1",
			dhcpEndIP:   "192.168.1.100",
			expected:    true,
		},
		{
			name:        "IP equal to end of range",
			ip:          "192.168.1.100",
			dhcpStartIP: "192.168.1.1",
			dhcpEndIP:   "192.168.1.100",
			expected:    true,
		},
		{
			name:        "IP outside range (too low)",
			ip:          "192.168.1.0",
			dhcpStartIP: "192.168.1.1",
			dhcpEndIP:   "192.168.1.100",
			expected:    false,
		},
		{
			name:        "IP outside range (too high)",
			ip:          "192.168.1.101",
			dhcpStartIP: "192.168.1.1",
			dhcpEndIP:   "192.168.1.100",
			expected:    false,
		},
		{
			name:        "IPv6 IP within range",
			ip:          "2001:db8::2",
			dhcpStartIP: "2001:db8::1",
			dhcpEndIP:   "2001:db8::ff",
			expected:    true,
		},
		{
			name:        "IPv6 IP outside range",
			ip:          "2001:db8::100",
			dhcpStartIP: "2001:db8::1",
			dhcpEndIP:   "2001:db8::ff",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := netip.MustParseAddr(tt.ip)
			dhcpStartIP := net.ParseIP(tt.dhcpStartIP)
			dhcpEndIP := net.ParseIP(tt.dhcpEndIP)

			got := IpInRange(ip, dhcpStartIP, dhcpEndIP)
			if got != tt.expected {
				t.Errorf("ipInRange() = %v, expected %v", got, tt.expected)
			}
		})
	}
}
