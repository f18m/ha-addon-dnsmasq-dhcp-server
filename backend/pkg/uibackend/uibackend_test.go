package uibackend

import (
	"dnsmasq-dhcp-backend/pkg/config"
	"dnsmasq-dhcp-backend/pkg/ippool"
	"dnsmasq-dhcp-backend/pkg/logger"
	"dnsmasq-dhcp-backend/pkg/trackerdb"
	"net"
	"net/netip"
	"os"
	"testing"
	"text/template"
	"time"

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
	backendopts := config.AddonOptions{
		DnsDomain: "lan",
		DhcpClientSettingsByMAC: map[string]config.DhcpClientSettings{
			"00:11:22:33:44:55": { // this is the MAC of 'client1'
				MacAddress: MustParseMAC("00:11:22:33:44:55"),
				Name:       "FriendlyClient1",
				Link:       MustParseTemplate("https://{{ .ip }}/client1-page"),
			},
			"aa:bb:cc:dd:ee:ff": { // this is the MAC of 'client4'
				MacAddress: MustParseMAC("aa:bb:CC:DD:ee:FF"),
				Name:       "FriendlyClient4",
				Link:       MustParseTemplate("https://{{ .hostname }}/client4-page"),
			},
		},
		DhcpClientSettingsByIP: map[netip.Addr]config.DhcpClientSettings{
			netip.MustParseAddr("192.168.0.3"): {
				Name:       "test-friendly-name",
				MacAddress: MustParseMAC("00:11:22:33:44:56"), // this is the MAC of 'client2'
				IP:         netip.MustParseAddr("192.168.0.3"),
				Link:       MustParseTemplate("https://{{ .ip }}"),
			},
		},
		DhcpPool: ippool.NewPoolFromString("192.168.0.1", "192.168.0.100"),
	}
	return &UIBackend{
		logger:    logger.NewNopCustomLogger("unit tests"),
		options:   backendopts,
		trackerDB: trackerdb.NewTestDB(),
	}
}

// TestGetDescriptionFor tests that getDescriptionFor() correctly returns descriptions for a given MAC address.
func TestGetDescriptionFor(t *testing.T) {
	backendopts := config.AddonOptions{
		DhcpClientSettingsByMAC: map[string]config.DhcpClientSettings{
			"00:11:22:33:44:55": {
				MacAddress:  MustParseMAC("00:11:22:33:44:55"),
				Name:        "FriendlyClient1",
				Description: "My laptop",
			},
			"00:11:22:33:44:56": {
				Name:        "client2",
				MacAddress:  MustParseMAC("00:11:22:33:44:56"),
				IP:          netip.MustParseAddr("192.168.0.3"),
				Description: "My server",
			},
		},
		DhcpClientSettingsByIP: map[netip.Addr]config.DhcpClientSettings{
			netip.MustParseAddr("192.168.0.3"): {
				Name:        "client2",
				MacAddress:  MustParseMAC("00:11:22:33:44:56"),
				IP:          netip.MustParseAddr("192.168.0.3"),
				Description: "My server",
			},
		},
		DhcpPool: ippool.NewPoolFromString("192.168.0.1", "192.168.0.100"),
	}
	backend := &UIBackend{
		logger:    logger.NewNopCustomLogger("unit tests"),
		options:   backendopts,
		trackerDB: trackerdb.NewTestDB(),
	}

	tests := []struct {
		name     string
		mac      net.HardwareAddr
		expected string
	}{
		{
			name:     "description from friendly name",
			mac:      MustParseMAC("00:11:22:33:44:55"),
			expected: "My laptop",
		},
		{
			name:     "description from IP reservation (by MAC)",
			mac:      MustParseMAC("00:11:22:33:44:56"),
			expected: "My server",
		},
		{
			name:     "no description for unknown client",
			mac:      MustParseMAC("aa:bb:cc:dd:ee:ff"),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := backend.getDescriptionFor(tt.mac)
			if result != tt.expected {
				t.Errorf("getDescriptionFor() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestGetTagsFor tests that getTagsFor() correctly returns tags for a given MAC address.
func TestGetTagsFor(t *testing.T) {
	backendopts := config.AddonOptions{
		DhcpClientSettingsByMAC: map[string]config.DhcpClientSettings{
			"00:11:22:33:44:55": {
				MacAddress: MustParseMAC("00:11:22:33:44:55"),
				Name:       "FriendlyClient1",
				Tags:       []string{"server", "production"},
			},
			"00:11:22:33:44:56": {
				Name:       "client2",
				MacAddress: MustParseMAC("00:11:22:33:44:56"),
				IP:         netip.MustParseAddr("192.168.0.3"),
				Tags:       []string{"iot"},
			},
		},
		DhcpClientSettingsByIP: map[netip.Addr]config.DhcpClientSettings{
			netip.MustParseAddr("192.168.0.3"): {
				Name:       "client2",
				MacAddress: MustParseMAC("00:11:22:33:44:56"),
				IP:         netip.MustParseAddr("192.168.0.3"),
				Tags:       []string{"iot"},
			},
		},
		DhcpPool: ippool.NewPoolFromString("192.168.0.1", "192.168.0.100"),
	}
	backend := &UIBackend{
		logger:    logger.NewNopCustomLogger("unit tests"),
		options:   backendopts,
		trackerDB: trackerdb.NewTestDB(),
	}

	tests := []struct {
		name     string
		mac      net.HardwareAddr
		expected []string
	}{
		{
			name:     "tags from friendly name",
			mac:      MustParseMAC("00:11:22:33:44:55"),
			expected: []string{"server", "production"},
		},
		{
			name:     "tags from IP reservation (by MAC)",
			mac:      MustParseMAC("00:11:22:33:44:56"),
			expected: []string{"iot"},
		},
		{
			name:     "no tags for unknown client",
			mac:      MustParseMAC("aa:bb:cc:dd:ee:ff"),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := backend.getTagsFor(tt.mac)
			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("getTagsFor() mismatch (-want +got):\n%s", diff)
			}
		})
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
			DnsNames:         []string{"FriendlyClient1.lan"}, // config name + DNS domain
		},
		{
			Lease: dnsmasq.Lease{
				MacAddr:  MustParseMAC("00:11:22:33:44:56"),
				IPAddr:   netip.MustParseAddr("192.168.0.3"),
				Hostname: "client2",
			},
			FriendlyName:     "",   // no friendly name configured
			HasStaticIP:      true, // check the IP address reservation has been recognized successfully
			IsInsideDHCPPool: true,
			EvaluatedLink:    "https://192.168.0.3",
			DnsNames:         []string{"client2.lan"}, // DHCP hostname + DNS domain (no MAC config entry)
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
			DnsNames:         []string{"FriendlyClient4.lan"}, // config name + DNS domain
		},
		{
			Lease: dnsmasq.Lease{
				MacAddr:  MustParseMAC("00:11:22:33:44:57"),
				IPAddr:   netip.MustParseAddr("192.168.0.101"),
				Hostname: "client3",
			},
			FriendlyName:     "", // no friendly name configured
			HasStaticIP:      false,
			IsInsideDHCPPool: false,                   // check if the condition "outside DHCP pool" has been recognized successfully
			EvaluatedLink:    "",                      // no "link" can be rendered since this client is not in configuration
			DnsNames:         []string{"client3.lan"}, // DHCP hostname + DNS domain (no config)
		},
	}

	// Validate that the state is updated as expected
	if diff := cmp.Diff(backend.dhcpClientData, expectedClientData, cmpopts.EquateComparable(netip.Addr{})); diff != "" {
		t.Errorf("Mismatch (-want +got):\n%s", diff)
	}
}

// TestGetDnsNamesFor tests that getDnsNamesFor() builds the correct list of DNS-resolvable names.
func TestGetDnsNamesFor(t *testing.T) {
	backendopts := config.AddonOptions{
		DnsDomain: "lan",
		DhcpClientSettingsByMAC: map[string]config.DhcpClientSettings{
			"00:11:22:33:44:55": {
				MacAddress: MustParseMAC("00:11:22:33:44:55"),
				Name:       "myserver",
				DnsAliases: []string{"alias1.lan", "alias2.lan"},
			},
			"00:11:22:33:44:56": {
				MacAddress: MustParseMAC("00:11:22:33:44:56"),
				Name:       "printer",
			},
		},
		DhcpPool: ippool.NewPoolFromString("192.168.0.1", "192.168.0.100"),
	}
	backend := &UIBackend{
		logger:    logger.NewNopCustomLogger("unit tests"),
		options:   backendopts,
		trackerDB: trackerdb.NewTestDB(),
	}

	tests := []struct {
		name     string
		mac      net.HardwareAddr
		hostname string
		expected []string
	}{
		{
			name:     "config entry with aliases",
			mac:      MustParseMAC("00:11:22:33:44:55"),
			hostname: "myserver",
			expected: []string{"myserver.lan", "alias1.lan", "alias2.lan"},
		},
		{
			name:     "config entry without aliases",
			mac:      MustParseMAC("00:11:22:33:44:56"),
			hostname: "printer",
			expected: []string{"printer.lan"},
		},
		{
			name:     "no config entry, uses DHCP hostname",
			mac:      MustParseMAC("aa:bb:cc:dd:ee:ff"),
			hostname: "laptop",
			expected: []string{"laptop.lan"},
		},
		{
			name:     "no config entry, missing DHCP hostname",
			mac:      MustParseMAC("aa:bb:cc:dd:ee:ff"),
			hostname: dnsmasqMarkerForMissingHostname,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := backend.getDnsNamesFor(tt.mac, tt.hostname)
			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("getDnsNamesFor() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestProcessLeaseUpdatesDebouncesBursts(t *testing.T) {
	backend := getMockUIBackend()
	backend.broadcastCh = make(chan struct{}, 8)
	backend.leasesCh = make(chan []*dnsmasq.Lease, 8)
	backend.options.LogWebUI = false

	oldDebounce := leaseUpdatesDebounceInterval
	leaseUpdatesDebounceInterval = 15 * time.Millisecond
	defer func() {
		leaseUpdatesDebounceInterval = oldDebounce
	}()

	go backend.processLeaseUpdates()

	first := []*dnsmasq.Lease{
		{
			MacAddr:  MustParseMAC("00:11:22:33:44:55"),
			IPAddr:   netip.MustParseAddr("192.168.0.2"),
			Hostname: "client1",
		},
	}
	second := []*dnsmasq.Lease{
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
	}

	backend.leasesCh <- first
	backend.leasesCh <- second

	select {
	case <-backend.broadcastCh:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("timed out waiting for debounced update")
	}

	select {
	case <-backend.broadcastCh:
		t.Fatalf("expected burst events to be coalesced into one update")
	case <-time.After(60 * time.Millisecond):
	}

	if got := len(backend.dhcpClientData); got != len(second) {
		t.Fatalf("expected latest lease snapshot to win after debounce: got %d, want %d", got, len(second))
	}
}

func TestProcessLeaseUpdatesProcessesSeparatedEvents(t *testing.T) {
	backend := getMockUIBackend()
	backend.broadcastCh = make(chan struct{}, 8)
	backend.leasesCh = make(chan []*dnsmasq.Lease, 8)

	oldDebounce := leaseUpdatesDebounceInterval
	leaseUpdatesDebounceInterval = 15 * time.Millisecond
	defer func() {
		leaseUpdatesDebounceInterval = oldDebounce
	}()

	go backend.processLeaseUpdates()

	first := []*dnsmasq.Lease{
		{
			MacAddr:  MustParseMAC("00:11:22:33:44:55"),
			IPAddr:   netip.MustParseAddr("192.168.0.2"),
			Hostname: "client1",
		},
	}
	second := []*dnsmasq.Lease{
		{
			MacAddr:  MustParseMAC("00:11:22:33:44:56"),
			IPAddr:   netip.MustParseAddr("192.168.0.3"),
			Hostname: "client2",
		},
	}

	backend.leasesCh <- first
	select {
	case <-backend.broadcastCh:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("timed out waiting for first update")
	}

	backend.leasesCh <- second
	select {
	case <-backend.broadcastCh:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("timed out waiting for second update")
	}
}

func TestRecoverUnexpectedEmptyLeaseUpdate(t *testing.T) {
	backend := getMockUIBackend()
	tmpFile, err := os.CreateTemp("", "leases-*.leases")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	content := "1790195704 aa:bb:cc:dd:ee:03 192.168.1.57 dynamic2 *\n1790195704 aa:bb:cc:dd:ee:02 192.168.1.56 dynamic1 *\n"
	if err := os.WriteFile(tmpFile.Name(), []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp lease file: %v", err)
	}

	oldLeaseFile := defaultDnsmasqLeasesFile
	defaultDnsmasqLeasesFile = tmpFile.Name()
	defer func() {
		defaultDnsmasqLeasesFile = oldLeaseFile
	}()

	recovered := backend.recoverUnexpectedEmptyLeaseUpdate([]*dnsmasq.Lease{}, 1)
	if len(recovered) != 2 {
		t.Fatalf("expected to recover 2 leases, got %d", len(recovered))
	}
}

func TestRecoverUnexpectedEmptyLeaseUpdateNoRecoveryWhenNotNeeded(t *testing.T) {
	backend := getMockUIBackend()
	nonEmpty := []*dnsmasq.Lease{
		{
			MacAddr:  MustParseMAC("00:11:22:33:44:55"),
			IPAddr:   netip.MustParseAddr("192.168.0.2"),
			Hostname: "client1",
		},
	}

	got := backend.recoverUnexpectedEmptyLeaseUpdate(nonEmpty, 3)
	if len(got) != 1 {
		t.Fatalf("expected non-empty updates to pass through unchanged, got len=%d", len(got))
	}

	got = backend.recoverUnexpectedEmptyLeaseUpdate([]*dnsmasq.Lease{}, 0)
	if len(got) != 0 {
		t.Fatalf("expected no recovery when previous count is zero, got len=%d", len(got))
	}
}
