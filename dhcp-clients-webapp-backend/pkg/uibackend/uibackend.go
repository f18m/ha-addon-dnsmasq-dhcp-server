package uibackend

import (
	"context"
	"dhcp-clients-webapp-backend/pkg/trackerdb"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/b0ch3nski/go-dnsmasq-utils/dnsmasq"
	"github.com/gorilla/websocket"
	"github.com/netdata/go.d.plugin/pkg/iprange"
)

type UIBackend struct {
	// The configuration for this backend
	cfg AddonConfig

	// time this application was started
	startTimestamp time.Time
	startCounter   int

	// the actual HTTP server
	server   http.Server
	upgrader websocket.Upgrader

	// more HTTP server resources
	isTestingMode bool
	htmlTemplate  *template.Template // read from disk at startup
	jsContents    string             // read from disk at startup
	cssContents   string             // read from disk at startup

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

	// channel used to link a goroutine watching for DHCP lease file changes and the lease file processor
	leasesCh chan []*dnsmasq.Lease
}

// ReadFileAndParseInteger reads a file, parses the number, and returns it as an integer
func ReadFileAndParseInteger(filename string) (int, error) {
	// Read the file content
	content, err := os.ReadFile(filename)
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

func NewUIBackend() UIBackend {
	db, err := trackerdb.NewDhcpClientTrackerDB(defaultDhcpClientTrackerDB)
	if err != nil {
		log.Default().Fatalf("Failed to open DHCP clients tracking DB: %s", err.Error())
		return UIBackend{}
	}

	log.Default().Printf("Successfully opened DHCP clients tracking DB at %s", defaultDhcpClientTrackerDB)

	isTestingMode := os.Getenv("LOCAL_TESTING") != ""

	var startCounter int
	startCounter, err = ReadFileAndParseInteger(defaultStartCounter)
	if err != nil {
		log.Default().Fatalf("Failed to open start counter file: %s", err.Error())
		return UIBackend{}
	}

	log.Default().Printf("The current DHCP start counter is at %d", startCounter)

	return UIBackend{
		cfg: AddonConfig{
			ipAddressReservationsByIP:  make(map[netip.Addr]IpAddressReservation),
			ipAddressReservationsByMAC: make(map[string]IpAddressReservation),
			friendlyNames:              make(map[string]DhcpClientFriendlyName),
		},
		startTimestamp: time.Now(),
		startCounter:   startCounter,
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
			Addr:    "",
			Handler: nil,
		},
		isTestingMode: isTestingMode,
	}
}

func (b *UIBackend) logRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// this logging is quite verbose, enable only if explicitly asked so
		if b.cfg.logWebUI {
			// print headers
			var headerStr string
			for name, values := range r.Header {
				for _, value := range values {
					headerStr += fmt.Sprintf("  %s: %s\n", name, value)
				}
			}
			headerStr += "----"

			log.Default().Printf("Method: %s, URL: %s, Host: %s, RemoteAddr: %s\nHeaders:\n%s\n",
				r.Method, r.URL.String(), r.Host, r.RemoteAddr, headerStr)

		}

		// keep serving the request
		next.ServeHTTP(w, r)
	})
}

func (b *UIBackend) getWebSocketMessage() WebSocketMessage {

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
	var currentClientsMacs []net.HardwareAddr
	for _, c := range currentClients {
		currentClientsMacs = append(currentClientsMacs, c.Lease.MacAddr)
	}

	// now get from the tracker DB some historical data about "dead DHCP clients"
	deadClients, err := b.trackerDB.GetDeadDhcpClients(currentClientsMacs)
	if err != nil {
		log.Default().Printf("ERR: failed to get list of dead/past DHCP clients: %s", err.Error())
		// keep going with an empty list
		deadClients = []trackerdb.DhcpClient{}
	} else {
		if b.cfg.logWebUI {
			log.Default().Printf("Running query to the tracker DB: found %d past/dead DHCP clients", len(deadClients))
		}
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
				pastClients[i].FriendlyName = b.cfg.ipAddressReservationsByMAC[deadC.MacAddr.String()].Name
			}
		}

		// create note field
		if deadC.DhcpServerStartCounter < b.startCounter {
			// a past instance of dnsmasq provided a DHCP lease... but we have no news
			// of this DHCP client since last restart
			pastClients[i].Notes = "Last seen in a previous run of this addon"
		} else if deadC.DhcpServerStartCounter == b.startCounter {
			// typical case when the DHCP client is turned off or e.g. it's connected via WLAN
			// and is currently out of range
			pastClients[i].Notes = "Missed DHCP renewal or cannot reach the network"
		} else {
			log.Default().Printf("ERROR: the database contains a client with a DHCP server start counter %d while current start counter is %d",
				deadC.DhcpServerStartCounter, b.startCounter)
		}
	}

	// finally build the websocket message
	return WebSocketMessage{
		CurrentClients: currentClients,
		PastClients:    pastClients,
	}
}

// WebSocket connection handler
func (b *UIBackend) handleWebSocketConn(w http.ResponseWriter, r *http.Request) {
	ws, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Default().Fatal(err)
	}
	defer ws.Close()

	msg := b.getWebSocketMessage()
	log.Default().Printf("Received new websocket client: pushing %d/%d current/past DHCP clients to it",
		len(msg.CurrentClients), len(msg.PastClients))

	// register new client
	b.clientsLock.Lock()
	b.clients[ws] = true
	if err := ws.WriteJSON(msg); err != nil { // push the current status on the websocket
		log.Default().Printf("WARN: failed to push initial data to the new websocket: %s", err.Error())
		// keep going, we will delete the client connection shortly in the loop below if the error
		// keeps popping up
	}
	b.clientsLock.Unlock()

	// listen till the end of the websocket
	for {
		var msg WebSocketMessage
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Default().Printf("Error while reading JSON from WebSocket: %v", err)
			b.clientsLock.Lock()
			delete(b.clients, ws)
			b.clientsLock.Unlock()
			break
		}
		log.Default().Printf("Received data from the websocket: %v", msg)
	}
}

// Broadcast updater: any update posted on the broadcastCh is broadcasted to all clients
func (b *UIBackend) broadcastUpdatesToClients() {

	ticker := time.NewTicker(10 * time.Second)

	msg := b.getWebSocketMessage()
	for {
		select {
		case <-b.broadcastCh:
			// if we get a message from this channel, it means the global list of
			// current DHCP clients has changed

		case <-ticker.C:
			// let's refresh the websocket with whatever data we already have;
			// this is done for 2 reasons:
			// 1. trigger a refresh on the webpage (the JS client-side will recompute
			//    countdowns, etc)
			// 2. keep the websocket TCP connection alive (otherwise it might be
			//    considered "stale" and get reset)
		}

		if len(b.clients) > 0 {
			// regen message
			msg = b.getWebSocketMessage()

			// loop over all clients
			numSuccess := 0
			b.clientsLock.Lock()
			for client := range b.clients {
				err := client.WriteJSON(msg)
				if err != nil {
					log.Default().Printf("Error while writing JSON to WebSocket: %v", err)
					client.Close()
					delete(b.clients, client)
				} else {
					numSuccess++
					if b.cfg.logWebUI {
						_, err := json.Marshal(msg)
						if err != nil {
							log.Default().Printf("Failed to marshal to JSON: %s.\nMessage:%v\n", err.Error(), msg)
						} /* else {
							log.Default().Printf("Successfully pushed data to WebSocket: %s", string(jsonData))
						} */
					}
				}
			}
			b.clientsLock.Unlock()

			if b.cfg.logWebUI {
				log.Default().Printf("Successfully pushed %d/%d current/past DHCP clients to %d websockets",
					len(msg.CurrentClients), len(msg.PastClients), numSuccess)
			}

		}
	}
}

// Reload the templates. Typically this happens only once at startup, but when testing
// env var is set, it happens on every page load.
func (b *UIBackend) reloadTemplates() {
	cssF := templatesDir + "/style.css"
	jsF := templatesDir + "/dnsmasq-dhcp.js"
	htmlF := templatesDir + "/index.templ.html"

	cssContents, err := os.ReadFile(cssF)
	if err != nil {
		log.Default().Fatalf("Failed to open CSS file: %s", err.Error())
		return
	}
	log.Default().Printf("Read CSS file %s: %d bytes", cssF, len(cssContents))
	b.cssContents = string(cssContents)

	jsContents, err := os.ReadFile(jsF)
	if err != nil {
		log.Default().Fatalf("Failed to open Javascript file: %s", err.Error())
		return
	}
	log.Default().Printf("Read Javascript file %s: %d bytes", jsF, len(jsContents))
	b.jsContents = string(jsContents)

	b.htmlTemplate = template.Must(template.ParseFiles(htmlF))
	log.Default().Printf("Parsed template file %s", htmlF)
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
			log.Default().Printf("WARN: testing mode detected... ignoring the absence of the INGRESS header")
			XIngressPath = []string{""}
		} else {
			log.Default().Printf("WARN: missing headers in HTTP GET")
			http.Error(w, "The request does not have the 'X-Ingress-Path' header", http.StatusBadRequest)
			return
		}
	}

	// compute pool size:
	dhcpPoolSize := 0
	if b.cfg.dhcpStartIP != nil && b.cfg.dhcpEndIP != nil {
		dhcpPoolSize = int(iprange.New(b.cfg.dhcpStartIP, b.cfg.dhcpEndIP).Size().Int64())
	}

	templateData := struct {
		// websockets
		WebSocketURI string

		// config info that are handy to have in the UI page
		DhcpStartIP             string
		DhcpEndIP               string
		DhcpPoolSize            int
		DefaultLease            string
		AddressReservationLease string
		DHCPServerStartTime     int64

		// embedded contents
		CssFileContent        template.CSS
		JavascriptFileContent template.JS
	}{
		// We use relative URL for the websocket in the form "/79957c2e_dnsmasq-dhcp/ingress/ws"
		// In this way we don't need to know whether the browser is passing through some TLS
		// reverse proxy or uses HomeAssistant built-in TLS or is connecting in plaintext (HTTP).
		// Based on the scheme used by the browser, the websocket will use the associated scheme
		// ('wss' for 'https' and 'ws' for 'http)
		WebSocketURI:            XIngressPath[0] + websocketRelativeUrl,
		DhcpStartIP:             b.cfg.dhcpStartIP.String(),
		DhcpEndIP:               b.cfg.dhcpEndIP.String(),
		DhcpPoolSize:            dhcpPoolSize,
		DefaultLease:            b.cfg.defaultLease,
		AddressReservationLease: b.cfg.addressReservationLease,
		// we approximate the DHCP server start time with this app's start time;
		// the reason is that inside the HA addon, dnsmasq is started at about the same
		// time of this app
		DHCPServerStartTime: b.startTimestamp.Unix(),

		CssFileContent:        template.CSS(b.cssContents),
		JavascriptFileContent: template.JS(b.jsContents),
	}

	err := b.htmlTemplate.Execute(w, templateData)
	if err != nil {
		log.Default().Printf("WARN: error while rendering template: %s\n", err.Error())
		// keep going
	} else {
		if b.cfg.logWebUI {
			log.Default().Printf("Successfully rendered web page template, responding with 200 OK\n")
		}
	}
}

// Read from the leasesCh and push to broadcastCh
func (b *UIBackend) processLeaseUpdates() {
	i := 0
	for {
		updatedLeases := <-b.leasesCh
		log.Default().Printf("INotify detected a change (#%d) to the DHCP client lease file... list size before=%d, after=%d clients\n",
			i, len(b.dhcpClientData), len(updatedLeases))
		b.processLeaseUpdatesFromArray(updatedLeases)

		// once the new list of DHCP client data entries is ready, notify the broadcast channel
		b.broadcastCh <- struct{}{}
		i += 1
	}
}

func (b *UIBackend) getFriendlyNameFor(mac net.HardwareAddr, hostname string) string {
	// do we have a friendly-name registered for this MAC address?
	metadata, ok := b.cfg.friendlyNames[mac.String()]
	if ok {
		// yes: enrich with some metadata this DHCP client entry
		return metadata.FriendlyName
	} else {
		if hostname != dnsmasqMarkerForMissingHostname {
			// no: user didn't provide any friendly name but the dnsmasq DHCP server
			// has received (over DHCP protocol) an hostname... better than nothing:
			// use that to create a "friendly name"
			return hostname
		}
	}
	return ""
}

func (b *UIBackend) hasIpAddressReservationByIP(ip netip.Addr, macExpected net.HardwareAddr) bool {
	_, hasReservation := b.cfg.ipAddressReservationsByIP[ip]
	if hasReservation {
		// the IP address provided is a reserved one...
		// check if the MAC address is the one for which that IP was intended...
		if strings.EqualFold(macExpected.String(), b.cfg.ipAddressReservationsByIP[ip].Mac) {
			return true
		} else {
			log.Default().Printf("WARN: the IP %s was leased to MAC address %s, but in configuration it was reserved for MAC %s\n",
				ip.String(), macExpected.String(), b.cfg.ipAddressReservationsByIP[ip].Mac)
		}
	}
	return false
}

func (b *UIBackend) hasIpAddressReservationByMAC(mac net.HardwareAddr) bool {
	_, hasReservation := b.cfg.ipAddressReservationsByMAC[mac.String()]
	return hasReservation
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
		d.IsInsideDHCPPool = IpInRange(lease.IPAddr, b.cfg.dhcpStartIP, b.cfg.dhcpEndIP)

		// processing complete:
		b.dhcpClientData = append(b.dhcpClientData, d)
	}

	// sort the slice by IP
	slices.SortFunc(b.dhcpClientData, func(a, b DhcpClientData) int {
		return a.Lease.IPAddr.Compare(b.Lease.IPAddr)
	})

	b.dhcpClientDataLock.Unlock()

	log.Default().Printf("Updated DHCP clients internal status with %d entries\n", len(b.dhcpClientData))
}

// Reads the current DNS masq lease file, before any INotify hook gets installed, to get a baseline
func (b *UIBackend) readCurrentLeaseFile() error {
	log.Default().Printf("Reading DHCP client lease file '%s'\n", defaultDnsmasqLeasesFile)

	// Read current DHCP leases
	leaseFile, errOpen := os.OpenFile(defaultDnsmasqLeasesFile, os.O_RDONLY|os.O_CREATE, 0644)
	if errOpen != nil {
		return errOpen
	}
	defer leaseFile.Close()
	leases, errRead := dnsmasq.ReadLeases(leaseFile)
	if errRead != nil {
		return errRead
	}

	b.processLeaseUpdatesFromArray(leases)
	return nil
}

// readAddonConfig reads the configuration of this Home Assistant addon and converts it
// into maps and slices that get stored into the UIBackend instance
func (b *UIBackend) readAddonConfig() error {
	log.Default().Printf("Reading addon config file '%s'\n", defaultHomeAssistantConfigFile)

	optionFile, errOpen := os.OpenFile(defaultHomeAssistantConfigFile, os.O_RDONLY|os.O_CREATE, 0644)
	if errOpen != nil {
		return errOpen
	}
	defer optionFile.Close()

	// read whole file
	data, err := io.ReadAll(optionFile)
	if err != nil {
		return err
	}

	// JSON parse
	err = json.Unmarshal(data, &b.cfg)
	if err != nil {
		return err
	}

	log.Default().Printf("Acquired %d IP address reservations\n", len(b.cfg.ipAddressReservationsByIP))
	log.Default().Printf("Acquired %d friendly names\n", len(b.cfg.friendlyNames))
	log.Default().Printf("Web server on port %d; Web UI logging enabled=%t; DHCP requests logging enabled=%t\n",
		b.cfg.webUIPort, b.cfg.logWebUI, b.cfg.logDHCP)

	return nil
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
	if err := b.readAddonConfig(); err != nil {
		log.Default().Fatalf("FATAL: error while reading HomeAssistant addon options: %s\n", err.Error())
	}

	// Initialize current DHCP client data table
	if err := b.readCurrentLeaseFile(); err != nil {
		log.Default().Fatalf("FATAL: error while reading DHCP leases file: %s\n", err.Error())
	}

	// Watch for updates on DHCP leases file and push to leasesCh
	ctx := context.Background()
	go func() {
		err := dnsmasq.WatchLeases(ctx, defaultDnsmasqLeasesFile, b.leasesCh)
		if err != nil {
			log.Default().Fatalf("FATAL: error while watching DHCP leases file: %s\n", err.Error())
		}
	}()

	// Read from the leasesCh and push to broadcastCh
	go b.processLeaseUpdates()

	// Read from the broadcastCh chan and push to all Websocket clients
	go b.broadcastUpdatesToClients()

	// Start server
	log.Default().Printf("Starting server to listen on port %d\n", b.cfg.webUIPort)
	b.server.Addr = fmt.Sprintf(":%d", b.cfg.webUIPort)
	b.server.Handler = mux
	return b.server.ListenAndServe()
}
