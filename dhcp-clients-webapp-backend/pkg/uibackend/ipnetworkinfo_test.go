package uibackend

import (
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
				Start:   net.IP{192, 168, 3, 4},
				End:     net.IP{192, 168, 7, 8},
				Gateway: net.IP{192, 168, 11, 12},
				Netmask: net.IPMask{255, 255, 0, 0}, // equivalent to /16
			},
			expected: true,
		},
		{
			netInfo: IpNetworkInfo{
				Start:   net.IP{192, 168, 3, 50},
				End:     net.IP{192, 168, 3, 150},
				Gateway: net.IP{192, 168, 3, 254},
				Netmask: net.IPMask{255, 255, 255, 0}, // equivalent to /24
			},
			expected: true,
		},

		// IPv6
		{
			netInfo: IpNetworkInfo{
				Start:   net.ParseIP("fe80::1"),         // not a private IP
				End:     net.ParseIP("fe80::64"),        // not a private IP
				Gateway: net.ParseIP("fe80::63"),        // gateway is IPv6
				Netmask: net.IPMask{255, 255, 255, 255}, // equivalent to /32
			},
			expected: true,
		},
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
		assert.Equal(t, test.expected, actual)
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
				Gateway: net.IP{192, 168, 11, 12},
				Netmask: net.IPMask{255, 255, 0, 0}, // equivalent to /16
			},
			expected: true,
		},
		{
			netInfo: IpNetworkInfo{
				Start:   net.IP{192, 168, 3, 50},
				End:     net.IP{192, 168, 3, 150},
				Gateway: net.IP{192, 168, 3, 254},
				Netmask: net.IPMask{255, 255, 255, 0}, // equivalent to /24
			},
			expected: true,
		},

		// IPv6
		{
			netInfo: IpNetworkInfo{
				Start:   net.ParseIP("fe80::1"),         // not a private IP
				End:     net.ParseIP("fe80::64"),        // not a private IP
				Gateway: net.ParseIP("fe80::63"),        // gateway is IPv6
				Netmask: net.IPMask{255, 255, 255, 255}, // equivalent to /32
			},
			expected: false,
		},
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
