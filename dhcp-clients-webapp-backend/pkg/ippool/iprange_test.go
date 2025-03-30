package ippool

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		startIP  string
		endIP    string
		expected bool
	}{
		// IPv4 tests
		{
			name:     "IP within range",
			ip:       "192.168.1.10",
			startIP:  "192.168.1.1",
			endIP:    "192.168.1.100",
			expected: true,
		},
		{
			name:     "IP equal to start of range",
			ip:       "192.168.1.1",
			startIP:  "192.168.1.1",
			endIP:    "192.168.1.100",
			expected: true,
		},
		{
			name:     "IP equal to end of range",
			ip:       "192.168.1.100",
			startIP:  "192.168.1.1",
			endIP:    "192.168.1.100",
			expected: true,
		},
		{
			name:     "IP outside range (too low)",
			ip:       "192.168.1.0",
			startIP:  "192.168.1.1",
			endIP:    "192.168.1.100",
			expected: false,
		},
		{
			name:     "IP outside range (too high)",
			ip:       "192.168.1.101",
			startIP:  "192.168.1.1",
			endIP:    "192.168.1.100",
			expected: false,
		},

		// IPv6 tests

		{
			name:     "IPv6 IP within range",
			ip:       "2001:db8::2",
			startIP:  "2001:db8::1",
			endIP:    "2001:db8::ff",
			expected: true,
		},
		{
			name:     "IPv6 IP outside range",
			ip:       "2001:db8::100",
			startIP:  "2001:db8::1",
			endIP:    "2001:db8::ff",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := netip.MustParseAddr(tt.ip)
			assert.NotNil(t, ip)

			r := NewRangeFromString(tt.startIP, tt.endIP)
			assert.True(t, r.IsValid())

			got := r.Contains(ip)
			if got != tt.expected {
				t.Errorf("ipInRange() = %v, expected %v", got, tt.expected)
			}
		})
	}
}
