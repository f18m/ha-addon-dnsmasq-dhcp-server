package uibackend

import (
	"bytes"
	"cmp"
	"context"
	"dnsmasq-dhcp-backend/pkg/logger"
	"dnsmasq-dhcp-backend/pkg/trackerdb"
	"encoding/json"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	texttemplate "text/template"
	"time"

	human_duration "github.com/davidbanham/human_duration/v3"

	"github.com/b0ch3nski/go-dnsmasq-utils/dnsmasq"
	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"
)

type UIBackend struct {
	logger *logger.CustomLogger

	// The configuration for this backend
	options AddonOptions
	config  AddonConfig

	// time this application was started
	startTimestamp time.Time
	startEpoch     int

	// the actual HTTP server
	server   http.Server
	upgrader websocket.Upgrader

	// more HTTP server resources
	isTestingMode bool
	htmlTemplate  *htmltemplate.Template // read from disk at startup

	// map of connected websockets
	clients     map[*websocket.Conn]bool
	clientsLock sync.Mutex

	// the most updated view on DHCP clients currently available
	dhcpClientData     []DhcpClientData
	dhcpClientDataLock sync.Mutex

	// DB tracking all DHCP clients, used to provide the "past DHCP clients" feature
	trackerDB trackerdb.DhcpClientTrackerDB

	// channel used to broadcast tabular data from backend->frontend
	broadcastCh chan struct{}

	// channel used to link a goroutine watching for DHCP lease file changes and the DHCP lease file processor
	leasesCh chan []*dnsmasq.Lease
}

// ReadFileAndParseInteger reads a file, parses the number, and returns it as an integer
func ReadFileAndParseInteger(filename string) (int, error) {
	// Read the file content
	content, err := os.ReadFile(filename) //nolint:gosec
	if err != nil {
		return 0, err
	}

	// Trim any leading/trailing spaces or newlines
	str := strings.TrimSpace(string(content))

	// Convert the string to an integer
	num, err := strconv.Atoi(str)
	if err != nil {
		return 0, err
	}

	return num, nil
}

func NewUIBackend(logger *logger.CustomLogger) UIBackend {
	db, err := trackerdb.NewDhcpClientTrackerDB(defaultDhcpClientTrackerDB)
	if err != nil {
		logger.Fatalf("Failed to open DHCP clients tracking DB: %s", err.Error())
		return UIBackend{}
	}

	logger.Infof("Successfully opened DHCP clients tracking DB at %s", defaultDhcpClientTrackerDB)

	isTestingMode := os.Getenv("LOCAL_TESTING") != ""

	var startEpoch int
	startEpoch, err = ReadFileAndParseInteger(defaultStartEpoch)
	if err != nil {
		logger.Fatalf("Failed to open start Epoch file: %s", err.Error())
		return UIBackend{}
	}

	logger.Infof("The current DHCP start Epoch is at %d", startEpoch)

	return UIBackend{
		logger: logger,
		options: AddonOptions{
			ipAddressReservationsByIP:  make(map[netip.Addr]IpAddressReservation),
			ipAddressReservationsByMAC: make(map[string]IpAddressReservation),
			friendlyNames:              make(map[string]DhcpClientFriendlyName),
		},
		startTimestamp: time.Now(),
		startEpoch:     startEpoch,
		clients:        make(map[*websocket.Conn]bool),
		dhcpClientData: nil,
		trackerDB:      *db,
		broadcastCh:    make(chan struct{}),
		leasesCh:       make(chan []*dnsmasq.Lease),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		server: http.Server{
			Addr:              "",
			Handler:           nil,
			ReadHeaderTimeout: 3 * time.Second,
		},
		isTestingMode: isTestingMode,
	}
}

func (b *UIBackend) logRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// this logging is quite verbose, enable only if explicitly asked so
		if b.options.logWebUI {
			// print headers
			var headerStr string
			for name, values := range r.Header {
				for _, value := range values {
					headerStr += fmt.Sprintf("  %s: %s\n", name, value)
				}
			}
			headerStr += "----"

			b.logger.Infof("Method: %s, URL: %s, Host: %s, RemoteAddr: %s\nHeaders:\n%s\n",
				r.Method, r.URL.String(), r.Host, r.RemoteAddr, headerStr)

		}

		// keep serving the request
		next.ServeHTTP(w, r)
	})
}

func (b *UIBackend) generateWebSocketMessage() WebSocketMessage {
	// get a copy of latest status -- lock it during the copy, to avoid race conditions
	// with the dnsmasq.leases watcher goroutine:
	b.dhcpClientDataLock.Lock()
	currentClients := make([]DhcpClientData, len(b.dhcpClientData))
	copy(currentClients, b.dhcpClientData)
	b.dhcpClientDataLock.Unlock()

	// sort the slice by IP (the user can sort again later based on some other criteria):
	slices.SortFunc(currentClients, func(a, b DhcpClientData) int {
		return a.Lease.IPAddr.Compare(b.Lease.IPAddr)
	})

	// convert currentClients to a simple slice of MAC addresses
	currentClientsMacs := make([]net.HardwareAddr, 0, len(currentClients))
	for _, c := range currentClients {
		currentClientsMacs = append(currentClientsMacs, c.Lease.MacAddr)
	}

	// if the dnsmasq-provided hostname is an asterisk *, that indicates that the DHCP client
	// did not advertise his own hostname to the DHCP server.
	// Make this more clear:
	for _, c := range currentClients {
		if c.Lease.Hostname == dnsmasqMarkerForMissingHostname {
			c.Lease.Hostname = unknownHostnameHtmlString
		}
	}

	// now get from the tracker DB some historical data about "dead DHCP clients"
	deadClients, err := b.trackerDB.GetDeadDhcpClients(currentClientsMacs)
	if err != nil {
		b.logger.Warnf("failed to get list of dead/past DHCP clients: %s", err.Error())
		// keep going with an empty list
		deadClients = []trackerdb.DhcpClient{}
	} else if b.options.logWebUI {
		b.logger.Infof("Running query to the tracker DB: found %d past/dead DHCP clients", len(deadClients))
	}

	// enrich FriendlyName, HasStaticIP fields of dead clients, creating the list of "past clients"
	pastClients := make([]PastDhcpClientData, len(deadClients))
	for i, deadC := range deadClients {
		pastClients[i].PastInfo = deadC

		// fill additional metadata
		pastClients[i].HasStaticIP = b.hasIpAddressReservationByMAC(deadC.MacAddr)
		pastClients[i].FriendlyName = b.getFriendlyNameFor(deadC.MacAddr, deadC.Hostname)
		if pastClients[i].FriendlyName == deadC.Hostname {
			// look also in the IP address reservations "friendly names"
			if pastClients[i].HasStaticIP {
				pastClients[i].FriendlyName = b.options.ipAddressReservationsByMAC[deadC.MacAddr.String()].Name
			}
		}

		if pastClients[i].FriendlyName == "" {
			pastClients[i].FriendlyName = "N/A"
		}
		if pastClients[i].PastInfo.Hostname == "" {
			pastClients[i].PastInfo.Hostname = unknownHostnameHtmlString
		}

		// create note field
		if deadC.DhcpServerStartEpoch < b.startEpoch { //nolint:gocritic
			// a past instance of dnsmasq provided a DHCP lease... but we have no news
			// of this DHCP client since last restart
			pastClients[i].Notes = "Last seen in a previous run of this addon"
		} else if deadC.DhcpServerStartEpoch == b.startEpoch {
			// typical case when the DHCP client is turned off or e.g. it's connected via WLAN
			// and is currently out of range
			pastClients[i].Notes = "Seen after last DHCP server restart but it missed DHCP renewal or it cannot reach the network anymore"
		} else {
			pastClients[i].Notes = "Tagged with wrong DHCP server start epoch"
			b.logger.Warnf("the database contains a client with a DHCP server start Epoch %d while current start Epoch is %d",
				deadC.DhcpServerStartEpoch, b.startEpoch)
		}
	}

	// sort the slice by LastSeen (the user can sort again later based on some other criteria):
	slices.SortFunc(pastClients, func(a, b PastDhcpClientData) int {
		return cmp.Compare(a.PastInfo.LastSeen.Unix(), b.PastInfo.LastSeen.Unix())
	})

	// this code is meant to be executed on the same machine/container where dnsmasq is running, so
	// that's why we pass "localhost" as DNS server host:
	dnsStats, err := getDnsStats("localhost", b.options.dnsPort)
	if err != nil {
		b.logger.Warnf("failed to get updated DNS stats: %s", err.Error())
		// keep going
	}

	// finally build the websocket message
	return WebSocketMessage{
		CurrentClients: currentClients,
		PastClients:    pastClients,
		DnsStats:       dnsStats,
	}
}

// WebSocket connection handler
func (b *UIBackend) handleWebSocketConn(w http.ResponseWriter, r *http.Request) {
	ws, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		b.logger.Fatalf("Failed to upgrade websocket connection: %s", err)
	}
	defer func() {
		_ = ws.Close()
	}()

	msg := b.generateWebSocketMessage() //nolint:contextcheck
	b.logger.Infof("Received new websocket client: pushing %d/%d current/past DHCP clients to it",
		len(msg.CurrentClients), len(msg.PastClients))

	// register new client
	b.clientsLock.Lock()
	b.clients[ws] = true
	if err := ws.WriteJSON(msg); err != nil { // push the current status on the websocket
		b.logger.Warnf("failed to push initial data to the new websocket: %s", err.Error())
		// keep going, we will delete the client connection shortly in the loop below if the error
		// keeps popping up
	}
	b.clientsLock.Unlock()

	// listen till the end of the websocket
	for {
		var msg WebSocketMessage
		err := ws.ReadJSON(&msg)
		if err != nil {
			b.logger.Warnf("failed to read JSON from WebSocket: %v", err)
			b.clientsLock.Lock()
			delete(b.clients, ws)
			b.clientsLock.Unlock()
			break
		}
		b.logger.Infof("Received data from the websocket: %v", msg)
	}
}

// Broadcast updater: any update posted on the broadcastCh is broadcasted to all clients
func (b *UIBackend) broadcastUpdatesToClients() {
	var tickerCh <-chan time.Time
	if b.options.webUIRefreshInterval > 0 {
		ticker := time.NewTicker(b.options.webUIRefreshInterval)
		tickerCh = ticker.C
	} else {
		// refresh is disabled, create a channel that will never get a message
		tickerCh = make(<-chan time.Time)
	}

	for {
		select {
		case <-b.broadcastCh:
			// if we get a message from this channel, it means the global list of
			// current DHCP clients has changed

		case <-tickerCh:
			// let's refresh the websocket with whatever data we already have;
			// this is done for 2 reasons:
			// 1. trigger a refresh on the webpage (the JS client-side will recompute
			//    countdowns, etc)
			// 2. keep the websocket TCP connection alive (otherwise it might be
			//    considered "stale" and get reset)
		}

		if len(b.clients) > 0 {
			// regen message
			msg := b.generateWebSocketMessage()

			// loop over all clients
			numSuccess := 0
			b.clientsLock.Lock()
			for client := range b.clients {
				err := client.WriteJSON(msg)
				if err != nil {
					b.logger.Warnf("failed writing JSON to WebSocket: %v", err)
					_ = client.Close()
					delete(b.clients, client)
				} else {
					numSuccess++
					if b.options.logWebUI {
						_, err := json.Marshal(msg)
						if err != nil {
							b.logger.Infof("Failed to marshal to JSON: %s.\nMessage:%v\n", err.Error(), msg)
						} /* else {
							b.logger.Infof("Successfully pushed data to WebSocket: %s", string(jsonData))
						} */
					}
				}
			}
			b.clientsLock.Unlock()

			if b.options.logWebUI {
				b.logger.Infof("Successfully pushed %d/%d current/past DHCP clients to %d websockets",
					len(msg.CurrentClients), len(msg.PastClients), numSuccess)
			}

		}
	}
}

// Reload the templates. Typically this happens only once at startup, but when testing
// env var is set, it happens on every page load.
func (b *UIBackend) reloadTemplates() {
	htmlF := templatesDir + "/index.templ.html"
	b.htmlTemplate = htmltemplate.Must(htmltemplate.ParseFiles(htmlF))
	b.logger.Infof("Parsed template file %s", htmlF)
}

// Render HTML page
func (b *UIBackend) renderPage(w http.ResponseWriter, r *http.Request) {
	if b.isTestingMode {
		b.reloadTemplates()
	}

	//
	// REVERSE PROXY LOGIC
	//
	// In order for the browser JS engine to estabilish the WebSocket connection successfully,
	// we need to direct the browser to the Hassio Ingress endpoint. There, the request
	// will be routed to the add-on nginx instance used as REVERSE PROXY which, finally,
	// will route the request to this webserver.
	// To get the Hassio ingress endpoint we can simply read some HTTP headers that the ingress
	// is adding to any request that goes through:
	//
	XIngressPath, ok2 := r.Header["X-Ingress-Path"]
	if !ok2 || len(XIngressPath) == 0 {
		if b.isTestingMode {
			// local testing mode... the docker container is not running under HA Supervisor,
			// so there is no ingress at all...
			b.logger.Warnf("testing mode detected... ignoring the absence of the INGRESS header")
			XIngressPath = []string{""}
		} else {
			b.logger.Warnf("missing headers in HTTP GET")
			http.Error(w, "The request does not have the 'X-Ingress-Path' header", http.StatusBadRequest)
			return
		}
	}

	// DNS
	dnsEnableString := "disabled"
	if b.options.dnsEnable {
		dnsEnableString = "enabled"
	}

	templateData := HtmlTemplate{
		// We use relative URL for the websocket in the form "/79957c2e_dnsmasq-dhcp/ingress/ws"
		// In this way we don't need to know whether the browser is passing through some TLS
		// reverse proxy or uses HomeAssistant built-in TLS or is connecting in plaintext (HTTP).
		// Based on the scheme used by the browser, the websocket will use the associated scheme
		// ('wss' for 'https' and 'ws' for 'http)
		WebSocketURI:            XIngressPath[0] + websocketRelativeUrl,
		DhcpRanges:              IpPoolToHtmlTemplateRanges(b.options.dhcpRanges),
		DhcpPoolSize:            b.options.dhcpPool.Size(),
		DefaultLease:            b.options.defaultLease,
		AddressReservationLease: b.options.addressReservationLease,
		// we approximate the DHCP server start time with this app's start time;
		// the reason is that inside the HA addon, dnsmasq is started at about the same
		// time of this app
		DHCPServerStartTime:        b.startTimestamp.Unix(),
		DHCPForgetPastClientsAfter: human_duration.ShortString(b.options.forgetPastClientsAfter, human_duration.Minute),

		// DNS config info
		DnsEnabled: dnsEnableString,
		DnsDomain:  b.options.dnsDomain,

		// misc
		AddonVersion: b.config.Version,
	}

	err := b.htmlTemplate.Execute(w, templateData)
	if err != nil {
		b.logger.Warnf("error while rendering template: %s\n", err.Error())
		// keep going
	} else if b.options.logWebUI {
		b.logger.Infof("Successfully rendered web page template, responding with 200 OK\n")
	}
}

// Read from the leasesCh and push to broadcastCh
func (b *UIBackend) processLeaseUpdates() {
	i := 0
	for {
		updatedLeases := <-b.leasesCh
		b.logger.Infof("INotify detected a change (#%d) to the DHCP client lease file... list size before=%d, after=%d clients\n",
			i, len(b.dhcpClientData), len(updatedLeases))
		b.processLeaseUpdatesFromArray(updatedLeases)

		// once the new list of DHCP client data entries is ready, notify the broadcast channel
		b.broadcastCh <- struct{}{}
		i += 1
	}
}

func (b *UIBackend) getFriendlyNameFor(mac net.HardwareAddr, hostname string) string {
	// do we have a friendly-name registered for this MAC address?
	metadata, ok := b.options.friendlyNames[mac.String()]
	if ok {
		// yes: enrich with some metadata this DHCP client entry
		return metadata.FriendlyName
	} else if hostname != dnsmasqMarkerForMissingHostname {
		// no: user didn't provide any friendly name but the dnsmasq DHCP server
		// has received (over DHCP protocol) an hostname... better than nothing:
		// use that to create a "friendly name"
		return hostname
	}
	return ""
}

func (b *UIBackend) hasIpAddressReservationByIP(ip netip.Addr, macExpected net.HardwareAddr) bool {
	_, hasReservation := b.options.ipAddressReservationsByIP[ip]
	if hasReservation {
		// the IP address provided is a reserved one...
		// check if the MAC address is the one for which that IP was intended...
		if strings.EqualFold(macExpected.String(), b.options.ipAddressReservationsByIP[ip].Mac.String()) {
			return true
		} else {
			b.logger.Warnf("the IP %s was leased to MAC address %s, but in configuration it was reserved for MAC %s\n",
				ip.String(), macExpected.String(), b.options.ipAddressReservationsByIP[ip].Mac.String())
		}
	}
	return false
}

func (b *UIBackend) hasIpAddressReservationByMAC(mac net.HardwareAddr) bool {
	_, hasReservation := b.options.ipAddressReservationsByMAC[mac.String()]
	return hasReservation
}

// isValidURI checks if the given string is a valid URI.
func isValidURI(uri string) bool {
	parsedURL, err := url.ParseRequestURI(uri)
	return err == nil && parsedURL.Scheme != "" && parsedURL.Host != ""
}

func (b *UIBackend) evaluateLink(hostname string, ip netip.Addr, mac net.HardwareAddr) string {
	var theTemplate *texttemplate.Template
	var friendlyName string

	r, hasFriendlyName := b.options.friendlyNames[mac.String()]
	if hasFriendlyName {
		theTemplate = r.Link
		friendlyName = r.FriendlyName
	} else {
		r, hasReservation := b.options.ipAddressReservationsByIP[ip]
		if hasReservation {
			theTemplate = r.Link
		}
	}

	if theTemplate == nil {
		return ""
	}

	// Create a buffer to capture the output
	var buf bytes.Buffer

	// Execute the template with the provided data
	err := theTemplate.Execute(&buf, map[string]string{
		"mac":           mac.String(),
		"ip":            ip.String(),
		"hostname":      hostname,
		"friendly_name": friendlyName,
		"dns_domain":    b.options.dnsDomain,
		// 'fqdn' is something that should be resolvable by the dnsmasq DNS server:
		"fqdn": fmt.Sprintf("%s.%s", hostname, b.options.dnsDomain),
	})
	if err != nil {
		b.logger.Warnf("failed to render the link template [%v]", theTemplate)
		return ""
	}

	lnk := buf.String()
	if !isValidURI(lnk) {
		b.logger.Warnf("rendering [%v] produced an invalid URI [%s]", theTemplate, lnk)
		return ""
	}
	return lnk
}

// Process a slice of dnsmasq.Lease and store that into the UIBackend object
func (b *UIBackend) processLeaseUpdatesFromArray(updatedLeases []*dnsmasq.Lease) {
	b.dhcpClientDataLock.Lock()
	b.dhcpClientData = make([]DhcpClientData, 0, len(updatedLeases) /* capacity */)
	for _, lease := range updatedLeases {

		d := DhcpClientData{Lease: *lease}

		// fill metadata
		d.FriendlyName = b.getFriendlyNameFor(lease.MacAddr, lease.Hostname)
		d.HasStaticIP = b.hasIpAddressReservationByIP(lease.IPAddr, lease.MacAddr)
		d.IsInsideDHCPPool = b.options.dhcpPool.Contains(lease.IPAddr)
		d.EvaluatedLink = b.evaluateLink(lease.Hostname, lease.IPAddr, lease.MacAddr)

		// processing complete:
		b.dhcpClientData = append(b.dhcpClientData, d)
	}

	// sort the slice by IP
	slices.SortFunc(b.dhcpClientData, func(a, b DhcpClientData) int {
		return a.Lease.IPAddr.Compare(b.Lease.IPAddr)
	})

	b.dhcpClientDataLock.Unlock()

	b.logger.Infof("Updated DHCP clients internal status with %d entries\n", len(b.dhcpClientData))
}

// Reads the current DNS masq lease file, before any INotify hook gets installed, to get a baseline
func (b *UIBackend) readCurrentLeaseFile() error {
	b.logger.Infof("Reading DHCP client lease file '%s'\n", defaultDnsmasqLeasesFile)

	// Read current DHCP leases
	leaseFile, errOpen := os.OpenFile(defaultDnsmasqLeasesFile, os.O_RDONLY|os.O_CREATE, 0o600)
	if errOpen != nil {
		return errOpen
	}
	defer func() {
		_ = leaseFile.Close()
	}()
	leases, errRead := dnsmasq.ReadLeases(leaseFile)
	if errRead != nil {
		return errRead
	}

	b.processLeaseUpdatesFromArray(leases)
	return nil
}

// readAddonOptions reads the OPTIONS of this Home Assistant addon and converts it
// into maps and slices that get stored into the UIBackend instance
func (b *UIBackend) readAddonOptions() error {
	b.logger.Infof("Reading addon options file '%s'\n", defaultHomeAssistantOptionsFile)

	optionFile, errOpen := os.Open(defaultHomeAssistantOptionsFile)
	if errOpen != nil {
		return errOpen
	}
	defer func() {
		_ = optionFile.Close()
	}()

	// read whole file
	data, err := io.ReadAll(optionFile)
	if err != nil {
		return err
	}

	// JSON parse
	err = json.Unmarshal(data, &b.options)
	if err != nil {
		return err
	}

	b.logger.Infof("Acquired %d DHCP network/ranges\n", len(b.options.dhcpRanges))
	b.logger.Infof("Acquired %d IP address reservations\n", len(b.options.ipAddressReservationsByIP))
	b.logger.Infof("Acquired %d friendly name definitions\n", len(b.options.friendlyNames))
	b.logger.Infof("DHCP requests logging enabled=%t; cleanup threshold for past DHCP clients set to %s\n",
		b.options.logDHCP, human_duration.ShortString(b.options.forgetPastClientsAfter, human_duration.Minute))
	b.logger.Infof("Web server on port %d; Web UI logging enabled=%t; Web UI refresh interval=%s\n",
		b.options.webUIPort, b.options.logWebUI, b.options.webUIRefreshInterval.String())

	return nil
}

// readAddonConfig reads the CONFIG of this Home Assistant addon
func (b *UIBackend) readAddonConfig() error {
	b.logger.Infof("Reading addon config file '%s'\n", defaultHomeAssistantConfigFile)

	cfgFile, errOpen := os.Open(defaultHomeAssistantConfigFile)
	if errOpen != nil {
		return errOpen
	}
	defer func() {
		_ = cfgFile.Close()
	}()

	d := yaml.NewDecoder(cfgFile)
	for {
		addonCfg := new(AddonConfig)
		err := d.Decode(&addonCfg)
		// break the loop in case of EOF
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		// check it was parsed
		if addonCfg == nil {
			continue
		}

		// check if the version is set
		if addonCfg.Version != "" {
			b.config = *addonCfg
			break
		}
	}

	b.logger.Infof("Acquired addon version: %s\n", b.config.Version)
	return nil
}

// forgetPastDhcpClients typically runs in a separate goroutine and removes past DHCP clients
// above some configurable threshold
func (b *UIBackend) forgetPastDhcpClients() {
	for {
		purgedClients, err := b.trackerDB.PurgeOldDeadClients(b.options.forgetPastClientsAfter)

		if err != nil {
			b.logger.Warnf("failed to purge past clients from tracker DB: %s", err.Error())
		} else if len(purgedClients) > 0 {
			desc := ""
			for _, c := range purgedClients {
				desc += fmt.Sprintf("%s, ", c.String())
			}
			b.logger.Infof("Purged %d past DHCP clients from tracker DB, last seen more than %s time ago: %s",
				len(purgedClients), b.options.forgetPastClientsAfter, desc)
		} /* else {
			b.logger.Info("No past DHCP client to purge from tracker DB")
		} */

		time.Sleep(pastClientsCheckInterval) // wait some time before next check
	}
}

// ListenAndServe is starting the whole UI backend:
// a web server, a WebSocket server, INotify-based watch on dnsmasq lease files, etc
func (b *UIBackend) ListenAndServe() error {
	b.reloadTemplates()

	mux := http.NewServeMux()

	// Serve static files, if any
	fs := http.FileServer(http.Dir(staticWebFilesDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Log requests (for debug only) + serve HTML pages
	mux.Handle("/", b.logRequestMiddleware(http.HandlerFunc(b.renderPage)))

	// Serve Websocket requests
	mux.HandleFunc(websocketRelativeUrl, b.handleWebSocketConn)

	// Read friendly names from the HomeAssistant addon config
	if err := b.readAddonOptions(); err != nil {
		b.logger.Fatalf("error while reading HomeAssistant addon options: %s\n", err.Error())
		return err
	}
	if err := b.readAddonConfig(); err != nil {
		b.logger.Fatalf("error while reading HomeAssistant addon config: %s\n", err.Error())
		return err
	}

	// Initialize current DHCP client data table
	if err := b.readCurrentLeaseFile(); err != nil {
		b.logger.Fatalf("error while reading DHCP leases file: %s\n", err.Error())
		return err
	}

	// Watch for updates on DHCP leases file and push to leasesCh
	ctx := context.Background()
	go func() {
		err := dnsmasq.WatchLeases(ctx, defaultDnsmasqLeasesFile, b.leasesCh)
		if err != nil {
			b.logger.Fatalf("error while watching DHCP leases file: %s\n", err.Error())
		}
	}()

	// Read from the leasesCh and push to broadcastCh
	go b.processLeaseUpdates()

	// Read from the broadcastCh chan and push to all Websocket clients
	go b.broadcastUpdatesToClients()

	// Check old tracker DB entries and delete them
	if b.options.forgetPastClientsAfter > 0 {
		go b.forgetPastDhcpClients()
	}

	// Start server
	b.logger.Infof("Starting server to listen on port %d\n", b.options.webUIPort)
	b.server.Addr = fmt.Sprintf(":%d", b.options.webUIPort)
	b.server.Handler = mux
	return b.server.ListenAndServe()
}
