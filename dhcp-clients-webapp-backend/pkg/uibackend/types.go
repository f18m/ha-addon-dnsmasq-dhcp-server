package uibackend

import (
	"dhcp-clients-webapp-backend/pkg/ippool"
	"dhcp-clients-webapp-backend/pkg/trackerdb"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"net"
	"net/netip"
	texttemplate "text/template"

	"github.com/b0ch3nski/go-dnsmasq-utils/dnsmasq"
)

// DhcpClientFriendlyName is the 1:1 binding between a MAC address and a human-friendly name
type DhcpClientFriendlyName struct {
	MacAddress   net.HardwareAddr
	FriendlyName string
	Link         *texttemplate.Template // maybe nil
}

// IpAddressReservation represents a static IP configuration loaded from the addon configuration file
type IpAddressReservation struct {
	Name string
	Mac  net.HardwareAddr
	IP   netip.Addr
	Link *texttemplate.Template // maybe nil
}

// AddonConfig contains the configuration provided by the user to the Home Assistant addon
// in the HomeAssistant YAML editor
type AddonConfig struct {
	// Static IP addresses, as read from the configuration
	ipAddressReservationsByIP  map[netip.Addr]IpAddressReservation
	ipAddressReservationsByMAC map[string]IpAddressReservation

	// DHCP client friendly names, as read from the configuration
	// The key of this map is the MAC address formatted as string (since net.HardwareAddr is not a valid map key type)
	friendlyNames map[string]DhcpClientFriendlyName

	// Multiple IP ranges all together form the DHCP pool
	dhcpPool ippool.Pool

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
func (b *AddonConfig) UnmarshalJSON(data []byte) error {

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
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"dhcp_pool"`

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

		b.dhcpPool.Ranges = append(b.dhcpPool.Ranges, dhcpR)
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
	// non-informative, so we allow users to override that from configuration.
	// If such an override is available in config, this field gets populated.
	FriendlyName string

	// In the configuration file it's possible to specify a golang template that is rendered to
	// produce a string which is intended to be an URL/URI to show for each DHCP client in the web UI.
	// If such link template is available in config, this field gets populated.
	EvaluatedLink string
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
		EvaluatedLink    string `json:"evaluated_link"`
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
		EvaluatedLink:    d.EvaluatedLink,
	})
}

// PastDhcpClientData identifies a DHCP client that was connected in the past, but not anymore
type PastDhcpClientData struct {
	PastInfo     trackerdb.DhcpClient `json:"past_info"`
	HasStaticIP  bool                 `json:"has_static_ip"`
	FriendlyName string               `json:"friendly_name"`
	Notes        string               `json:"notes"`
}

type DnsUpstreamStats struct {
	// ServerURL typical content (as reported by dnsmasq) looks like "8.8.8.8#53", i.e. IP#PORT
	ServerURL string `json:"server_url"`

	QueriesSent   int `json:"queries_sent"`
	QueriesFailed int `json:"queries_failed"`
}

// DnsServerStats contains all the available dnsmasq DNS server metrics
type DnsServerStats struct {
	// The domain names are cachesize.bind, insertions.bind, evictions.bind, misses.bind, hits.bind, auth.bind and servers.bind unless disabled at compile-time. An example command to query this, using the dig utility would be
	// dig +short chaos txt cachesize.bind
	CacheSize       int                `json:"cache_size"`
	CacheInsertions int                `json:"cache_insertions"`
	CacheEvictions  int                `json:"cache_evictions"`
	CacheMisses     int                `json:"cache_misses"`
	CacheHits       int                `json:"cache_hits"`
	UpstreamServers []DnsUpstreamStats `json:"upstream_servers_stats"`
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

	// DnsStats provides a live feed about DNS server basic metrics.
	DnsStats DnsServerStats `json:"dns_stats"`
}

// HtmlTemplateIpRange is used inside HtmlTemplate
type HtmlTemplateIpRange struct {
	Start string
	End   string
}

// HtmlTemplate is the struct used to render the "index.templ.html" file
type HtmlTemplate struct {
	// websockets
	WebSocketURI string

	// DHCP config info that are handy to have in the UI page
	DhcpRanges              []HtmlTemplateIpRange
	DhcpPoolSize            int64
	DefaultLease            string
	AddressReservationLease string
	DHCPServerStartTime     int64

	// DNS config info
	DnsEnabled string
	DnsDomain  string

	// embedded contents
	CssFileContent        htmltemplate.CSS
	JavascriptFileContent htmltemplate.JS
}
