package uibackend

import (
	"net"
	"net/netip"
	"testing"

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

// Test function
func TestProcessLeaseUpdatesFromArray(t *testing.T) {
	// Prepare mock leases
	leases := []*dnsmasq.Lease{
		{
			MacAddr:  MustParseMAC("00:11:22:33:44:55"),
			IPAddr:   netip.MustParseAddr("192.168.0.2"),
			Hostname: "client1",
		},
		{
			MacAddr:  MustParseMAC("00:11:22:33:44:56"),
			IPAddr:   netip.MustParseAddr("192.168.0.3"),
			Hostname: "client2",
		},
		{
			MacAddr:  MustParseMAC("00:11:22:33:44:57"),
			IPAddr:   netip.MustParseAddr("192.168.0.101"),
			Hostname: "client3",
		},
	}

	// Prepare UIBackend with mock data
	backend := &UIBackend{
		friendlyNames: map[string]DhcpClientFriendlyName{
			"00:11:22:33:44:55": { // this is the MAC of 'client1'
				MacAddress:   MustParseMAC("00:11:22:33:44:55"),
				FriendlyName: "FriendlyClient1",
			},
		},
		ipAddressReservations: map[netip.Addr]IpAddressReservation{
			netip.MustParseAddr("192.168.0.3"): {
				Name: "test-friendly-name",
				Mac:  "00:11:22:33:44:56", // this is the MAC of 'client2'
				IP:   "192.168.0.3",
			},
		},
		dhcpStartIP: net.IPv4(192, 168, 0, 1),
		dhcpEndIP:   net.IPv4(192, 168, 0, 100),
	}

	// Call the method being tested
	backend.processLeaseUpdatesFromArray(leases)

	// Expected output after processing the leases
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
		},
	}

	// Validate that the state is updated as expected
	if diff := cmp.Diff(backend.dhcpClientData, expectedClientData, cmpopts.EquateComparable(netip.Addr{})); diff != "" {
		t.Errorf("Mismatch (-want +got):\n%s", diff)
	}
}