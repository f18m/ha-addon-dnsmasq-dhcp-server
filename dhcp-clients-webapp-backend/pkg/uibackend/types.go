package uibackend

import (
	"dhcp-clients-webapp-backend/pkg/trackerdb"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"

	"github.com/b0ch3nski/go-dnsmasq-utils/dnsmasq"
)

// DhcpClientData holds all the information the backend has about a particular DHCP client,
// currently "connected" to the dnsmasq server.
// In this context "connected" means: that sent DHCP traffic since the dnsmasq server was started.
type DhcpClientData struct {
	// the lease as it is parsed from dnsmasq LEASE file:
	Lease dnsmasq.Lease

	// metadata associated with the DHCP client (obtained from configuration):

	// HasStaticIP indicates whether the DHCP server is configured to provide a specific IP address
	// (i.e. has an IP address reservation) for this client.
	// Note that static IP addresses do not need to be inside the DHCP range; indeed quite often the
	// static IP address reserved lies outside the DHCP range
	HasStaticIP bool

	// IsInsideDHCPPool indicates whether this DHCP client has an IP that lies within the DHCP pool
	// range and thus is consuming an IP address from that pool
	// (note that this DHCP client might be a client with a static reservation or not)
	IsInsideDHCPPool bool

	// Sometimes the hostname provided by the DHCP client to the DHCP server is really awkward and
	// non-informative, so we allow users to override that.
	FriendlyName string
}

// MarshalJSON customizes the JSON serialization for DhcpClientData
func (d DhcpClientData) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Lease struct {
			Expires  int64  `json:"expires"`
			MacAddr  string `json:"mac_addr"`
			IPAddr   string `json:"ip_addr"`
			Hostname string `json:"hostname"`
		} `json:"lease"`
		HasStaticIP      bool   `json:"has_static_ip"`
		IsInsideDHCPPool bool   `json:"is_inside_dhcp_pool"`
		FriendlyName     string `json:"friendly_name"`
	}{
		Lease: struct {
			Expires  int64  `json:"expires"`
			MacAddr  string `json:"mac_addr"`
			IPAddr   string `json:"ip_addr"`
			Hostname string `json:"hostname"`
		}{
			Expires:  d.Lease.Expires.Unix(), // unix time, the number of seconds elapsed since January 1, 1970 UTC
			MacAddr:  d.Lease.MacAddr.String(),
			IPAddr:   d.Lease.IPAddr.String(),
			Hostname: d.Lease.Hostname,
		},
		HasStaticIP:      d.HasStaticIP,
		IsInsideDHCPPool: d.IsInsideDHCPPool,
		FriendlyName:     d.FriendlyName,
	})
}

// PastDhcpClientData identifies a DHCP client that was connected in the past, but not anymore
type PastDhcpClientData struct {
	PastInfo     trackerdb.DhcpClient `json:"past_info"`
	HasStaticIP  bool                 `json:"has_static_ip"`
	FriendlyName string               `json:"friendly_name"`
	Notes        string               `json:"notes"`
}

// DhcpClientFriendlyName is the 1:1 binding between a MAC address and a human-friendly name
type DhcpClientFriendlyName struct {
	MacAddress   net.HardwareAddr
	FriendlyName string
}

// IpAddressReservation represents a static IP configuration loaded from the addon configuration file
type IpAddressReservation struct {
	Name string `json:"name"`
	Mac  string `json:"mac"`
	IP   string `json:"ip"`
}

// AddonConfig is used to unmarshal HomeAssistant option file correctly
// This must be updated every time the config.yaml of the addon is changed
type AddonConfig struct {
	// Static IP addresses, as read from the configuration
	ipAddressReservationsByIP  map[netip.Addr]IpAddressReservation
	ipAddressReservationsByMAC map[string]IpAddressReservation

	// DHCP client friendly names, as read from the configuration
	// The key of this map is the MAC address formatted as string (since net.HardwareAddr is not a valid map key type)
	friendlyNames map[string]DhcpClientFriendlyName

	// DHCP range
	// NOTE: if in the future we want to support more complex DHCP configurations where the DHCP pool
	//       consists of multiple disjoint IP address ranges, then we should consider using:
	//       the iprange.Pool object to represent that
	dhcpStartIP net.IP
	dhcpEndIP   net.IP

	// Log this backend activities?
	logDHCP  bool
	logWebUI bool

	webUIPort int

	// Lease times
	defaultLease            string
	addressReservationLease string
}

// readAddonConfig reads the configuration of this Home Assistant addon and converts it
// into maps and slices that get stored into the UIBackend instance
func (b *AddonConfig) UnmarshalJSON(data []byte) error {

	// JSON parse
	var cfg struct {
		IpAddressReservations []IpAddressReservation `json:"ip_address_reservations"`

		DhcpClientsFriendlyNames []struct {
			Name string `json:"name"`
			Mac  string `json:"mac"`
		} `json:"dhcp_clients_friendly_names"`

		DhcpRange struct {
			StartIP string `json:"start_ip"`
			EndIP   string `json:"end_ip"`
		} `json:"dhcp_range"`

		LogDHCP  bool `json:"log_dhcp"`
		LogWebUI bool `json:"log_web_ui"`

		WebUIPort int `json:"web_ui_port"`

		DefaultLease            string `json:"default_lease"`
		AddressReservationLease string `json:"address_reservation_lease"`
	}
	err := json.Unmarshal(data, &cfg)
	if err != nil {
		return err
	}

	// convert DHCP IP strings to net.IP
	b.dhcpStartIP = net.ParseIP(cfg.DhcpRange.StartIP)
	b.dhcpEndIP = net.ParseIP(cfg.DhcpRange.EndIP)
	if b.dhcpStartIP == nil || b.dhcpEndIP == nil {
		return fmt.Errorf("invalid DHCP range found in addon config file")
	}

	// ensure we have a valid port for web UI
	if cfg.WebUIPort <= 0 || cfg.WebUIPort > 32768 {
		return fmt.Errorf("invalid web UI port number: %d", cfg.WebUIPort)
	}

	// convert IP address reservations to a map indexed by IP
	for _, r := range cfg.IpAddressReservations {
		ipAddr, err := netip.ParseAddr(r.IP)
		if err != nil {
			return fmt.Errorf("invalid IP address found inside 'ip_address_reservations': %s", r.IP)
		}
		macAddr, err := net.ParseMAC(r.Mac)
		if err != nil {
			return fmt.Errorf("invalid MAC address found inside 'ip_address_reservations': %s", r.Mac)
		}

		// normalize the IP and MAC address format (e.g. to lowercase)
		r.IP = ipAddr.String()
		r.Mac = macAddr.String()

		b.ipAddressReservationsByIP[ipAddr] = r
		b.ipAddressReservationsByMAC[macAddr.String()] = r
	}

	// convert friendly names to a map of DhcpClientFriendlyName instances indexed by MAC address
	for _, client := range cfg.DhcpClientsFriendlyNames {
		macAddr, err := net.ParseMAC(client.Mac)
		if err != nil {
			return fmt.Errorf("invalid MAC address found inside 'dhcp_clients_friendly_names': %s", client.Mac)
		}

		b.friendlyNames[macAddr.String()] = DhcpClientFriendlyName{
			MacAddress:   macAddr,
			FriendlyName: client.Name,
		}
	}

	// copy basic settings
	b.logDHCP = cfg.LogDHCP
	b.logWebUI = cfg.LogWebUI
	b.webUIPort = cfg.WebUIPort
	b.defaultLease = cfg.DefaultLease
	b.addressReservationLease = cfg.AddressReservationLease

	return nil
}

// WebSocketMessage defines which contents get transmitted over the websocket in the
// BACKEND -> UI direction.
// Any structure contained here should have a sensible JSON marshalling helper.
// This structure should contain only dynamic data, that will be updated live on the webpage;
// everything else that is "static" will be rendered as a template variable when the page is
// served to the client.
type WebSocketMessage struct {
	// CurrentClients contains the list of clients currently "connected" to the dnsmasq server.
	// In this context "connected" means: that sent DHCP traffic since the dnsmasq server was started.
	CurrentClients []DhcpClientData `json:"current_clients"`

	// PastClients contains the list of clients that were connected in the past, but never
	// obtained a DHCP lease since the last dnsmasq server restart.
	PastClients []PastDhcpClientData `json:"past_clients"`
}
