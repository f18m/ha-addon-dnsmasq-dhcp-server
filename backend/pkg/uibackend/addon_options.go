package uibackend

import (
	"dnsmasq-dhcp-backend/pkg/ippool"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	texttemplate "text/template"
)

// AddonOptions contains the configuration provided by the user to the Home Assistant addon
// in the HomeAssistant YAML editor
type AddonOptions struct {
	// Static IP addresses, as read from the configuration
	ipAddressReservationsByIP  map[netip.Addr]IpAddressReservation
	ipAddressReservationsByMAC map[string]IpAddressReservation

	// DHCP client friendly names, as read from the configuration
	// The key of this map is the MAC address formatted as string (since net.HardwareAddr is not a valid map key type)
	friendlyNames map[string]DhcpClientFriendlyName

	// Multiple IP ranges all together form the DHCP pool
	dhcpPool   ippool.Pool     // this type provide the Size() and Contains() methods
	dhcpRanges []IpNetworkInfo // this type stores additional metadata for each network

	// Log this backend activities?
	logDHCP  bool
	logWebUI bool

	webUIPort int

	// Lease times
	defaultLease            string
	addressReservationLease string

	// DNS
	dnsEnable bool
	dnsDomain string
	dnsPort   int
}

// UnmarshalJSON reads the configuration of this Home Assistant addon and converts it
// into maps and slices that get stored into the UIBackend instance
func (b *AddonOptions) UnmarshalJSON(data []byte) error {
	// JSON structure.
	// This must be updated every time the config.yaml of the addon is changed;
	// however this structure contains only fields that are relevant to the
	// UI backend behavior. In other words the addon config.yaml might contain
	// more settings than those listed here.
	var cfg struct {
		DhcpIpAddressReservations []struct {
			Name string `json:"name"`
			Mac  string `json:"mac"`
			IP   string `json:"ip"`
			Link string `json:"link"`
		} `json:"dhcp_ip_address_reservations"`

		DhcpClientsFriendlyNames []struct {
			Name string `json:"name"`
			Mac  string `json:"mac"`
			Link string `json:"link"`
		} `json:"dhcp_clients_friendly_names"`

		DhcpServer struct {
			LogDHCP                 bool   `json:"log_requests"`
			DefaultLease            string `json:"default_lease"`
			AddressReservationLease string `json:"address_reservation_lease"`
		} `json:"dhcp_server"`

		DhcpPool []struct {
			Interface string `json:"interface"`
			Start     string `json:"start"`
			End       string `json:"end"`
			Gateway   string `json:"gateway"`
			Netmask   string `json:"netmask"`
		} `json:"dhcp_pools"`

		DnsServer struct {
			Enable    bool   `json:"enable"`
			DnsDomain string `json:"dns_domain"`
			Port      int    `json:"port"`
		} `json:"dns_server"`

		WebUI struct {
			Log  bool `json:"log_activity"`
			Port int  `json:"port"`
		} `json:"web_ui"`
	}
	err := json.Unmarshal(data, &cfg)
	if err != nil {
		return err
	}

	// convert DHCP IP addresses (strings) to iprange.Pool == []iprange.Range
	for _, r := range cfg.DhcpPool {
		dhcpR := ippool.NewRangeFromString(r.Start, r.End)
		if !dhcpR.IsValid() {
			return fmt.Errorf("invalid DHCP range %s-%s found in addon config file", r.Start, r.End)
		}

		// create also the IpNetworkInfo obj associated:
		ipNetInfo := IpNetworkInfo{
			Interface: r.Interface,
			Start:     dhcpR.Start,
			End:       dhcpR.End,
			Gateway:   net.ParseIP(r.Gateway),
		}

		m := net.ParseIP(r.Netmask)
		if m.To4() != nil {
			ipNetInfo.Netmask = net.IPMask(m.To4())
		} else {
			ipNetInfo.Netmask = net.IPMask(m.To16())
		}

		// check network definition
		if !ipNetInfo.HasValidIPs() {
			// RFC 1918 (IPv4 addresses) and RFC 4193 (IPv6 addresses).
			return fmt.Errorf("invalid DHCP network/range [%s] found in addon config file: the IP addresses should define a coherent network: a) they should be private IPs only according to RFC 1918 and RFC 4193; b) their start and end IPs must be within the same network", ipNetInfo.String())
		}
		if !ipNetInfo.HasValidGateway() {
			return fmt.Errorf("invalid DHCP network/range [%s] found in addon config file: the gateway must be an IP address within the network defined by the startIP/endIP/netmask parameters", ipNetInfo.String())
		}

		// all good: store the info
		b.dhcpPool.Ranges = append(b.dhcpPool.Ranges, dhcpR)
		b.dhcpRanges = append(b.dhcpRanges, ipNetInfo)
	}

	// ensure we have a valid port for web UI
	if cfg.WebUI.Port <= 0 || cfg.WebUI.Port > 32768 {
		return fmt.Errorf("invalid web UI port number: %d", cfg.WebUI.Port)
	}

	// convert IP address reservations to a map indexed by IP
	for _, r := range cfg.DhcpIpAddressReservations {
		ipAddr, err := netip.ParseAddr(r.IP)
		if err != nil {
			return fmt.Errorf("invalid IP address found inside 'ip_address_reservations': %s", r.IP)
		}
		macAddr, err := net.ParseMAC(r.Mac)
		if err != nil {
			return fmt.Errorf("invalid MAC address found inside 'ip_address_reservations': %s", r.Mac)
		}

		var linkTemplate *texttemplate.Template
		if r.Link != "" {
			linkTemplate, err = texttemplate.New("linkTemplate").Parse(r.Link)
			if err != nil {
				return fmt.Errorf("invalid golang template found inside 'link': %s", r.Link)
			}
		}

		// normalize the IP and MAC address format (e.g. to lowercase)
		r.IP = ipAddr.String()
		r.Mac = macAddr.String()

		ipReservation := IpAddressReservation{
			Name: r.Name,
			Mac:  macAddr,
			IP:   ipAddr,
			Link: linkTemplate,
		}

		b.ipAddressReservationsByIP[ipAddr] = ipReservation
		b.ipAddressReservationsByMAC[macAddr.String()] = ipReservation
	}

	// convert friendly names to a map of DhcpClientFriendlyName instances indexed by MAC address
	for _, client := range cfg.DhcpClientsFriendlyNames {
		macAddr, err := net.ParseMAC(client.Mac)
		if err != nil {
			return fmt.Errorf("invalid MAC address found inside 'dhcp_clients_friendly_names': %s", client.Mac)
		}

		var linkTemplate *texttemplate.Template
		if client.Link != "" {
			linkTemplate, err = texttemplate.New("linkTemplate").Parse(client.Link)
			if err != nil {
				return fmt.Errorf("invalid golang template found inside 'link': %s", client.Link)
			}
		}

		b.friendlyNames[macAddr.String()] = DhcpClientFriendlyName{
			MacAddress:   macAddr,
			FriendlyName: client.Name,
			Link:         linkTemplate,
		}
	}

	// copy basic settings
	b.logDHCP = cfg.DhcpServer.LogDHCP
	b.logWebUI = cfg.WebUI.Log
	b.webUIPort = cfg.WebUI.Port
	b.defaultLease = cfg.DhcpServer.DefaultLease
	b.addressReservationLease = cfg.DhcpServer.AddressReservationLease
	b.dnsEnable = cfg.DnsServer.Enable
	b.dnsDomain = cfg.DnsServer.DnsDomain
	b.dnsPort = cfg.DnsServer.Port

	return nil
}
