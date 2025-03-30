package uibackend

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIpNetworkInfo_HasValidIPs(t *testing.T) {
	tests := []struct {
		netInfo  IpNetworkInfo
		expected bool
	}{

		// IPv4
		{
			netInfo: IpNetworkInfo{
				Start:   net.IP{192, 168, 3, 4},     // a private IP
				End:     net.IP{192, 168, 7, 8},     // a private IP
				Gateway: net.IP{192, 168, 11, 12},   // a private IP
				Netmask: net.IPMask{255, 255, 0, 0}, // equivalent to /16
			},
			expected: true,
		},
		{
			netInfo: IpNetworkInfo{
				Start:   net.IP{192, 168, 3, 50},      // a private IP
				End:     net.IP{192, 168, 3, 150},     // a private IP
				Gateway: net.IP{192, 168, 3, 254},     // a private IP
				Netmask: net.IPMask{255, 255, 255, 0}, // equivalent to /24
			},
			expected: true,
		},
		{
			netInfo: IpNetworkInfo{
				Start:   net.IP{192, 168, 3, 150},     // a private IP
				End:     net.IP{192, 168, 3, 50},      // a private IP
				Gateway: net.IP{192, 168, 3, 254},     // a private IP
				Netmask: net.IPMask{255, 255, 255, 0}, // equivalent to /24
			},
			expected: true, // start and end are swapped, but the network is still valid
		},
		{
			netInfo: IpNetworkInfo{
				Start:   net.IP{192, 170, 0, 1},       // not a private IP
				End:     net.IP{192, 170, 0, 100},     // not a private IP
				Gateway: net.IP{192, 170, 0, 2},       // no a private IP but inside the network
				Netmask: net.IPMask{255, 255, 255, 0}, // equivalent to /24
			},
			expected: false,
		},
		{
			netInfo: IpNetworkInfo{
				Start:   net.IP{1, 2, 3, 4},             // not a private IP
				End:     net.IP{5, 6, 7, 8},             // not a private IP
				Gateway: net.IP{192, 168, 3, 254},       // a private IP - not inside the network
				Netmask: net.IPMask{255, 255, 255, 255}, // equivalent to /32
			},
			expected: false,
		},

		// IPv6
		{
			netInfo: IpNetworkInfo{
				Start:   net.ParseIP("fd12:3456:789a:1::10"),  // a private IP
				End:     net.ParseIP("fd12:3456:789a:1::100"), // a private IP
				Gateway: net.ParseIP("fd12:3456:789a:1::1"),   // a private IP
				Netmask: net.CIDRMask(64, 128),                // this is a /64 IPv6 netmask
			},
			expected: true,
		},
		{
			netInfo: IpNetworkInfo{
				Start:   net.ParseIP("fd12:3456:789a:1::100"), // a private IP - start is greater than end
				End:     net.ParseIP("fd12:3456:789a:1::10"),  // a private IP
				Gateway: net.ParseIP("fd12:3456:789a:1::1"),   // a private IP
				Netmask: net.CIDRMask(64, 128),                // this is a /64 IPv6 netmask
			},
			expected: true, // start and end are swapped, but the network is still valid
		},
		{
			netInfo: IpNetworkInfo{
				Start:   net.ParseIP("fd12:3456:789a:1::10"),  // a private IP
				End:     net.ParseIP("fd12:3456:789a:1::100"), // a private IP
				Gateway: net.ParseIP("fd12:3456:789a:1::1"),   // a private IP
				Netmask: net.IPMask{255, 255, 255, 255},       // ipv4 netmask with an IPv6 network
			},
			expected: false,
		},

		// corner cases
		{
			netInfo: IpNetworkInfo{
				Start:   nil,                            // test nil entry
				End:     nil,                            // test nil entry
				Gateway: net.IP{1, 2, 3, 4},             // valid IPv4
				Netmask: net.IPMask{255, 255, 255, 255}, // equivalent to /32
			},
			expected: false,
		},
	}

	for _, test := range tests {
		actual := test.netInfo.HasValidIPs()
		assert.Equal(t, test.expected, actual, fmt.Sprintf("for test %v", test.netInfo))
	}
}

func TestIpNetworkInfo_HasValidGateway(t *testing.T) {
	tests := []struct {
		netInfo  IpNetworkInfo
		expected bool
	}{
		// IPv4
		{
			netInfo: IpNetworkInfo{
				Start:   net.IP{192, 168, 3, 4},
				End:     net.IP{192, 168, 7, 8},
				Gateway: net.IP{192, 168, 11, 12},   // gateway is within the (large) network /16
				Netmask: net.IPMask{255, 255, 0, 0}, // equivalent to /16
			},
			expected: true,
		},
		{
			netInfo: IpNetworkInfo{
				Start:   net.IP{192, 168, 3, 50},
				End:     net.IP{192, 168, 3, 150},
				Gateway: net.IP{192, 168, 3, 254},     // gateway is within the network /24
				Netmask: net.IPMask{255, 255, 255, 0}, // equivalent to /24
			},
			expected: true, // it's fine to have the gateway outside the range defined by start/end IPs, but it must be within the network defined by start/netmask
		},
		{
			netInfo: IpNetworkInfo{
				Start:   net.IP{192, 168, 3, 50},
				End:     net.IP{192, 168, 3, 150},
				Gateway: net.IP{192, 168, 4, 254},     // invalid gateway, since it belongs to another network
				Netmask: net.IPMask{255, 255, 255, 0}, // equivalent to /24
			},
			expected: false,
		},

		// IPv6
		{
			netInfo: IpNetworkInfo{
				Start:   net.ParseIP("fd12:3456:789a:1::10"),  // a private IP
				End:     net.ParseIP("fd12:3456:789a:1::100"), // a private IP
				Gateway: net.ParseIP("fd12:3456:789a:1::1"),   // a private IP
				Netmask: net.CIDRMask(64, 128),                // this is a /64 IPv6 netmask
			},
			expected: true, // it's fine to have the gateway outside the range defined by start/end IPs, but it must be within the network defined by start/netmask
		},
		{
			netInfo: IpNetworkInfo{
				Start:   net.ParseIP("fd12:3456:789a:1::10"),  // a private IP
				End:     net.ParseIP("fd12:3456:789a:1::100"), // a private IP
				Gateway: net.ParseIP("fd12:3456:789a:1::15"),  // a private IP
				Netmask: net.CIDRMask(64, 128),                // this is a /64 IPv6 netmask
			},
			expected: true,
		},
		{
			netInfo: IpNetworkInfo{
				Start:   net.ParseIP("fd12:3456:789a:1::10"),  // a private IP
				End:     net.ParseIP("fd12:3456:789a:1::100"), // a private IP
				Gateway: net.ParseIP("fd12:ffff:ffff:1::15"),  // a private IP outside the IPv6 network
				Netmask: net.CIDRMask(64, 128),                // this is a /64 IPv6 netmask
			},
			expected: false,
		},

		// corner cases
		{
			netInfo: IpNetworkInfo{
				Start:   nil,                            // test nil entry
				End:     nil,                            // test nil entry
				Gateway: net.IP{1, 2, 3, 4},             // valid IPv4
				Netmask: net.IPMask{255, 255, 255, 255}, // equivalent to /32
			},
			expected: false,
		},
	}

	for _, test := range tests {
		actual := test.netInfo.HasValidGateway()
		assert.Equal(t, test.expected, actual)
	}
}
