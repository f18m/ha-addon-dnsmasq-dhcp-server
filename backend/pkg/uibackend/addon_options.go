package uibackend

import (
	"dnsmasq-dhcp-backend/pkg/ippool"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strings"
	texttemplate "text/template"
	"time"
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

	forgetPastClientsAfter time.Duration

	// Log this backend activities?
	logDHCP  bool
	logWebUI bool

	// web UI
	webUIPort            int
	webUIRefreshInterval time.Duration

	// Lease times
	defaultLease            string
	addressReservationLease string

	// DNS
	dnsEnable bool
	dnsDomain string
	dnsPort   int
}

// ParseDuration parses a duration string.
// examples: "10d", "-1.5w" or "3Y4M5d".
// Add time units are "d"="D", "w"="W", "M", "y"="Y".
// Taken from https://gist.github.com/xhit/79c9e137e1cfe332076cdda9f5e24699
func parseDuration(s string) (time.Duration, error) {
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}

	re := regexp.MustCompile(`(\d*\.\d+|\d+)[^\d]*`)
	unitMap := map[string]int{
		"d": 24,
		"D": 24,
		"w": 7 * 24,
		"W": 7 * 24,
		"M": 30 * 24,
		"y": 365 * 24,
		"Y": 365 * 24,
	}

	strs := re.FindAllString(s, -1)
	if len(strs) == 0 {
		return 0, fmt.Errorf("invalid duration string: %s", s)
	}

	var sumDur time.Duration
	for _, str := range strs {
		_hours := 1
		for unit, hours := range unitMap {
			if strings.Contains(str, unit) {
				str = strings.ReplaceAll(str, unit, "h")
				_hours = hours
				break
			}
		}

		dur, err := time.ParseDuration(str)
		if err != nil {
			return 0, err
		}

		sumDur += time.Duration(int(dur) * _hours)
	}

	if neg {
		sumDur = -sumDur
	}
	return sumDur, nil
}

// UnmarshalJSON reads the configuration of this Home Assistant addon and converts it
// into maps and slices that get stored into the UIBackend instance
func (o *AddonOptions) UnmarshalJSON(data []byte) error {
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
			ForgetPastClientsAfter  string `json:"forget_past_clients_after"`
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
			Log                bool `json:"log_activity"`
			Port               int  `json:"port"`
			RefreshIntervalSec int  `json:"refresh_interval_sec"`
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
		o.dhcpPool.Ranges = append(o.dhcpPool.Ranges, dhcpR)
		o.dhcpRanges = append(o.dhcpRanges, ipNetInfo)
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

		o.ipAddressReservationsByIP[ipAddr] = ipReservation
		o.ipAddressReservationsByMAC[macAddr.String()] = ipReservation
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

		o.friendlyNames[macAddr.String()] = DhcpClientFriendlyName{
			MacAddress:   macAddr,
			FriendlyName: client.Name,
			Link:         linkTemplate,
		}
	}

	// parse time duration
	o.forgetPastClientsAfter, err = parseDuration(cfg.DhcpServer.ForgetPastClientsAfter)
	if err != nil {
		return fmt.Errorf("invalid time duration found inside 'forget_past_clients_after': %s", cfg.DhcpServer.ForgetPastClientsAfter)
	}

	o.webUIRefreshInterval = time.Duration(cfg.WebUI.RefreshIntervalSec) * time.Second

	// copy basic settings
	o.logDHCP = cfg.DhcpServer.LogDHCP
	o.logWebUI = cfg.WebUI.Log
	o.webUIPort = cfg.WebUI.Port
	o.defaultLease = cfg.DhcpServer.DefaultLease
	o.addressReservationLease = cfg.DhcpServer.AddressReservationLease
	o.dnsEnable = cfg.DnsServer.Enable
	o.dnsDomain = cfg.DnsServer.DnsDomain
	o.dnsPort = cfg.DnsServer.Port

	return nil
}
