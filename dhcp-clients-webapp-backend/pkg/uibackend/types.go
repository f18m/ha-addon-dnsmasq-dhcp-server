package uibackend

import (
	"dhcp-clients-webapp-backend/pkg/trackerdb"
	"encoding/json"
	"net"

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
}

// WebSocketMessage defines which contents get transmitted over the websocket in the
// BACKEND -> UI direction.
// Any structure contained here should have a sensible JSON marshalling helper.
type WebSocketMessage struct {
	// CurrentClients contains the list of clients currently "connected" to the dnsmasq server.
	// In this context "connected" means: that sent DHCP traffic since the dnsmasq server was started.
	CurrentClients []DhcpClientData `json:"current_clients"`

	// PastClients contains the list of clients that were connected in the past, but never
	// obtained a DHCP lease since the last dnsmasq server restart.
	PastClients []trackerdb.DhcpClient `json:"past_clients"`

	// DHCPServerStartTime indicates the point in time where
	// the dnsmasq server was last started, expressed as Unix epoch.
	DHCPServerStartTime int64 `json:"dhcp_server_starttime"`
}
