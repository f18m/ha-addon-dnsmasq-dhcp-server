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
		{
			netInfo: IpNetworkInfo{
				Start:   net.IP{1, 2, 3, 4},                                        // not a private IP
				End:     net.IP{5, 6, 7, 8},                                        // not a private IP
				Gateway: net.IP{0xfc, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4}, // gateway is IPv6
				Netmask: net.IPMask{255, 255, 255, 255},                            // equivalent to /32
			},
			expected: false,
		},
		{
			netInfo: IpNetworkInfo{
				Start:   nil,
				End:     nil,
				Gateway: net.IP{1, 2, 3, 4},
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

/*
func TestIpNetworkInfo_HasValidGateway(t *testing.T) {
	nw := IpNetworkInfo{
		start:   net.IP{1, 2, 3, 4},
		end:     net.IP{5, 6, 7, 8},
		gateway: net.IP{9, 10, 11, 12},
		Netmask: net.IPMask(0xffffffff00), // equivalent to /32
	}
	if !nw.HasValidGateway() {
		t.Errorf("expected HasValidGateway() to return true")
	}

	nw = IpNetworkInfo{
		start:   net.IP{1, 2, 3, 4},
		end:     net.IP{5, 6, 7, 8},
		gateway: net.IP{9, 10, 11, 13},    // invalid gateway
		Netmask: net.IPMask(0xffffffff00), // equivalent to /32
	}
	if nw.HasValidGateway() {
		t.Errorf("expected HasValidGateway() to return false")
	}

	nw = IpNetworkInfo{
		start:   nil,
		end:     nil,
		gateway: net.IP{1, 2, 3, 4},
		Netmask: net.IPMask(0xffffffff00), // equivalent to /32
	}
	if !nw.HasValidGateway() {
		t.Errorf("expected HasValidGateway() to return false")
	}

	nw = IpNetworkInfo{
		start:   nil,
		end:     nil,
		gateway: nil,
		Netmask: net.IPMask(0xffffffff00), // equivalent to /32
	}
	if nw.HasValidGateway() {
		t.Errorf("expected HasValidGateway() to return false")
	}

	nw = IpNetworkInfo{
		start:   net.IP{1, 2, 3, 4},
		end:     nil,
		gateway: net.IP{9, 10, 11, 12},
		Netmask: net.IPMask(0xffffffff00), // equivalent to /32
	}
	if !nw.HasValidGateway() {
		t.Errorf("expected HasValidGateway() to return false")
	}

	nw = IpNetworkInfo{
		start:   nil,
		end:     nil,
		gateway: net.IP{9, 10, 11, 12},
		Netmask: net.IPMask(0xffffffff00), // equivalent to /32
	}
	if !nw.HasValidGateway() {
		t.Errorf("expected HasValidGateway() to return false")
	}

	nw = IpNetworkInfo{
		start:   nil,
		end:     nil,
		gateway: net.IP{9, 10, 11, 12},
		Netmask: net.IPMask(0xffffffff00), // equivalent to /32
	}
	if !nw.HasValidGateway() {
		t.Errorf("expected HasValidGateway() to return false")
	}

	nw = IpNetworkInfo{
		start:   nil,
		end:     nil,
		gateway: net.IP{1, 2, 3, 4},
		Netmask: nil,
	}
	if !nw.HasValidGateway() {
		t.Errorf("expected HasValidGateway() to return false")
	}

	nw = IpNetworkInfo{
		start:   nil,
		end:     nil,
		gateway: net.IP{1, 2, 3, 4},
		Netmask: net.IPMask(0xffffffff00), // equivalent to /32
	}
	if !nw.HasValidGateway() {
		t.Errorf("expected HasValidGateway() to return false")
	}

	nw = IpNetworkInfo{
		start:   nil,
		end:     nil,
		gateway: net.IP{1, 2, 3, 4},
		Netmask: net.IPMask(0xffffffff00), // equivalent to /32
	}
	if nw.HasValidGateway() {
		t.Errorf("expected HasValidGateway() to return false")
	}

	nw = IpNetworkInfo{
		start:   nil,
		end:     nil,
		gateway: nil,
		Netmask: net.IPMask(0xffffffff00), // equivalent to /32
	}
	if nw.HasValidGateway() {
		t.Errorf("expected HasValidGateway() to return false")
	}

	nw = IpNetworkInfo{
		start:   nil,
		end:     nil,
		gateway: nil,
		Netmask: nil,
	}
	if !nw.HasValidGateway() {
		t.Errorf("expected HasValidGateway() to return true")
	}
}
*/
