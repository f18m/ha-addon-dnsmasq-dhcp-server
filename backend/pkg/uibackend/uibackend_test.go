package uibackend

import (
	"dnsmasq-dhcp-backend/pkg/ippool"
	"dnsmasq-dhcp-backend/pkg/logger"
	"dnsmasq-dhcp-backend/pkg/trackerdb"
	"net"
	"net/netip"
	"testing"
	"text/template"

	"github.com/b0ch3nski/go-dnsmasq-utils/dnsmasq"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// MustParseMAC acts like ParseMAC but panics if in case of an error
func MustParseMAC(s string) net.HardwareAddr {
	mac, err := net.ParseMAC(s)
	if err != nil {
		panic(err)
	}
	return mac
}

func MustParseTemplate(s string) *template.Template {
	return template.Must(template.New("test").Parse(s))
}

func getMockLeases() []*dnsmasq.Lease {
	return []*dnsmasq.Lease{
		{
			// client1
			MacAddr:  MustParseMAC("00:11:22:33:44:55"),
			IPAddr:   netip.MustParseAddr("192.168.0.2"),
			Hostname: "client1",
		},
		{
			// client2
			MacAddr:  MustParseMAC("00:11:22:33:44:56"),
			IPAddr:   netip.MustParseAddr("192.168.0.3"),
			Hostname: "client2",
		},
		{
			// client3
			MacAddr:  MustParseMAC("00:11:22:33:44:57"),
			IPAddr:   netip.MustParseAddr("192.168.0.101"),
			Hostname: "client3",
		},
		{
			// client4
			MacAddr:  MustParseMAC("aa:bb:CC:DD:ee:FF"), // mixed case MAC address
			IPAddr:   netip.MustParseAddr("192.168.0.66"),
			Hostname: "client4",
		},
	}
}

func getMockUIBackend() *UIBackend {
	// simulate configurations for:
	//  * IP address reservations
	//  * friendly names for dynamic clients
	//  * DHCP range
	backendopts := AddonOptions{
		friendlyNames: map[string]DhcpClientFriendlyName{
			"00:11:22:33:44:55": { // this is the MAC of 'client1'
				MacAddress:   MustParseMAC("00:11:22:33:44:55"),
				FriendlyName: "FriendlyClient1",
				Link:         MustParseTemplate("https://{{ .ip }}/client1-page"),
			},
			"aa:bb:cc:dd:ee:ff": { // this is the MAC of 'client4'
				MacAddress:   MustParseMAC("aa:bb:CC:DD:ee:FF"),
				FriendlyName: "FriendlyClient4",
				Link:         MustParseTemplate("https://{{ .hostname }}/client4-page"),
			},
		},
		ipAddressReservationsByIP: map[netip.Addr]IpAddressReservation{
			netip.MustParseAddr("192.168.0.3"): {
				Name: "test-friendly-name",
				Mac:  MustParseMAC("00:11:22:33:44:56"), // this is the MAC of 'client2'
				IP:   netip.MustParseAddr("192.168.0.3"),
				Link: MustParseTemplate("https://{{ .ip }}"),
			},
		},
		dhcpPool: ippool.NewPoolFromString("192.168.0.1", "192.168.0.100"),
	}
	return &UIBackend{
		logger:    logger.NewCustomLogger("unit tests"),
		options:   backendopts,
		trackerDB: trackerdb.NewTestDB(),
	}
}

// TestEvaluateLink tests evaluateLink() helper with valid template data.
func TestEvaluateLink(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		ip       netip.Addr
		mac      net.HardwareAddr
		expected string
	}{
		{
			name:     "link from IP address reservation",
			hostname: "test-friendly-name",
			ip:       netip.MustParseAddr("192.168.0.3"),
			mac:      MustParseMAC("00:11:22:33:44:56"), // this is the MAC of 'client2'
			expected: "https://192.168.0.3",
		},
		{
			name:     "link from friendly name",
			hostname: "FriendlyClient1",
			ip:       netip.MustParseAddr("192.168.100.200"), // simulate a dynamic IP
			mac:      MustParseMAC("00:11:22:33:44:55"),      // this is the MAC of 'client1'
			expected: "https://192.168.100.200/client1-page",
		},
		{
			name:     "link from friendly name",
			hostname: "FriendlyClient4",
			ip:       netip.MustParseAddr("192.168.100.200"), // simulate a dynamic IP
			mac:      MustParseMAC("aa:bb:CC:DD:ee:FF"),      // this is the MAC of 'client4'
			expected: "https://FriendlyClient4/client4-page",
		},
	}

	backend := getMockUIBackend()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := backend.evaluateLink(tt.hostname, tt.ip, tt.mac)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Test function
func TestProcessLeaseUpdatesFromArray(t *testing.T) {
	// Prepare mock data
	leases := getMockLeases()
	backend := getMockUIBackend()

	// Call the method being tested
	backend.processLeaseUpdatesFromArray(leases)

	// Expected output after processing the leases
	// NOTE: the expected data must be sorted by IP address
	expectedClientData := []DhcpClientData{
		{
			Lease: dnsmasq.Lease{
				MacAddr:  MustParseMAC("00:11:22:33:44:55"),
				IPAddr:   netip.MustParseAddr("192.168.0.2"),
				Hostname: "client1",
			},
			FriendlyName:     "FriendlyClient1", // check friendly name has been associated successfully
			HasStaticIP:      false,
			IsInsideDHCPPool: true,
			EvaluatedLink:    "https://192.168.0.2/client1-page",
		},
		{
			Lease: dnsmasq.Lease{
				MacAddr:  MustParseMAC("00:11:22:33:44:56"),
				IPAddr:   netip.MustParseAddr("192.168.0.3"),
				Hostname: "client2",
			},
			FriendlyName:     "client2",
			HasStaticIP:      true, // check the IP address reservation has been recognized successfully
			IsInsideDHCPPool: true,
			EvaluatedLink:    "https://192.168.0.3",
		},
		{
			Lease: dnsmasq.Lease{
				MacAddr:  MustParseMAC("aa:bb:cc:dd:ee:ff"),
				IPAddr:   netip.MustParseAddr("192.168.0.66"),
				Hostname: "client4",
			},
			FriendlyName:     "FriendlyClient4",
			HasStaticIP:      false,
			IsInsideDHCPPool: true,
			EvaluatedLink:    "https://client4/client4-page",
		},
		{
			Lease: dnsmasq.Lease{
				MacAddr:  MustParseMAC("00:11:22:33:44:57"),
				IPAddr:   netip.MustParseAddr("192.168.0.101"),
				Hostname: "client3",
			},
			FriendlyName:     "client3",
			HasStaticIP:      false,
			IsInsideDHCPPool: false, // check if the condition "outside DHCP pool" has been recognized successfully
			EvaluatedLink:    "",    // no "link" can be rendered since this client is not in configuration
		},
	}

	// Validate that the state is updated as expected
	if diff := cmp.Diff(backend.dhcpClientData, expectedClientData, cmpopts.EquateComparable(netip.Addr{})); diff != "" {
		t.Errorf("Mismatch (-want +got):\n%s", diff)
	}
}
