package uibackend

import (
	"context"
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
	"strings"
	"sync"

	"github.com/b0ch3nski/go-dnsmasq-utils/dnsmasq"
	"github.com/gorilla/websocket"
	"github.com/netdata/go.d.plugin/pkg/iprange"
)

var defaultDnsmasqLeasesFile = "/data/dnsmasq.leases"
var defaultHomeAssistantConfigFile = "/data/options.json"

// These absolute paths must be in sync with the Dockerfile
var staticWebFilesDir = "/opt/web/static"
var templatesDir = "/opt/web/templates"

// other constants
var dnsmasqMarkerForMissingHostname = "*"
var websocketRelativeUrl = "/ws"

type UIBackend struct {
	// Static IP addresses, as read from the configuration
	ipAddressReservations map[netip.Addr]IpAddressReservation

	// DHCP client friendly names, as read from the configuration
	// The key of this map is the MAC address formatted as string (since net.HardwareAddr is not a valid map key type)
	friendlyNames map[string]DhcpClientFriendlyName

	// Log this backend activities?
	logWebUI bool

	// DHCP range
	// NOTE: if in the future we want to support more complex DHCP configurations where the DHCP pool
	//       consists of multiple disjoint IP address ranges, then we should consider using:
	//       the iprange.Pool object to represent that
	dhcpStartIP net.IP
	dhcpEndIP   net.IP

	// the actual HTTP server
	serverPort int
	server     http.Server
	upgrader   websocket.Upgrader

	// map of connected websockets
	clients     map[*websocket.Conn]bool
	clientsLock sync.Mutex

	// the most updated view on DHCP clients currently available
	dhcpClientData     []DhcpClientData
	dhcpClientDataLock sync.Mutex

	// channel used to broadcast tabular data from backend->frontend
	broadcastCh chan []DhcpClientData

	// channel used to link a goroutine watching for DHCP lease file changes and the lease file processor
	leasesCh chan []*dnsmasq.Lease
}

func NewUIBackend() UIBackend {
	return UIBackend{
		ipAddressReservations: make(map[netip.Addr]IpAddressReservation),
		friendlyNames:         make(map[string]DhcpClientFriendlyName),
		clients:               make(map[*websocket.Conn]bool),
		dhcpClientData:        nil,
		broadcastCh:           make(chan []DhcpClientData),
		leasesCh:              make(chan []*dnsmasq.Lease),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		server: http.Server{
			Addr:    "",
			Handler: nil,
		},
	}
}

func (b *UIBackend) logRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// this logging is quite verbose, enable only if explicitly asked so
		if b.logWebUI {
			log.Default().Printf("Method: %s, URL: %s, Host: %s, RemoteAddr: %s\n", r.Method, r.URL.String(), r.Host, r.RemoteAddr)

			// print headers
			fmt.Println("Headers:")
			for name, values := range r.Header {
				for _, value := range values {
					fmt.Printf("  %s: %s\n", name, value)
				}
			}
			fmt.Println("----")
		}

		// keep serving the request
		next.ServeHTTP(w, r)
	})
}

// WebSocket connection handler
func (b *UIBackend) handleWebSocketConn(w http.ResponseWriter, r *http.Request) {
	ws, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Default().Fatal(err)
	}
	defer ws.Close()

	// get a copy of latest status so we can release the lock ASAP
	b.dhcpClientDataLock.Lock()
	updatedStatus := make([]DhcpClientData, len(b.dhcpClientData))
	copy(updatedStatus, b.dhcpClientData)
	b.dhcpClientDataLock.Unlock()

	log.Default().Printf("Received new websocket client: pushing %d DHCP clients to it", len(updatedStatus))

	// register new client
	b.clientsLock.Lock()
	b.clients[ws] = true
	if err := ws.WriteJSON(updatedStatus); err != nil { // push the current status on the websocket
		log.Default().Printf("WARN: failed to push initial data to the new websocket: %s", err.Error())
		// keep going, we will delete the client connection shortly in the loop below if the error
		// keeps popping up
	}
	b.clientsLock.Unlock()

	// listen till the end of the websocket
	for {
		var msg []DhcpClientData
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
	for {
		dhcpClientsSlice := <-b.broadcastCh

		// sort the slice by IP (the user can sort again later based on some other criteria):
		slices.SortFunc(dhcpClientsSlice, func(a, b DhcpClientData) int {
			return a.Lease.IPAddr.Compare(b.Lease.IPAddr)
		})

		// loop over all clients
		b.clientsLock.Lock()
		for client := range b.clients {
			err := client.WriteJSON(dhcpClientsSlice)
			if err != nil {
				log.Default().Printf("Error while writing JSON to WebSocket: %v", err)
				client.Close()
				delete(b.clients, client)
			} else {
				if b.logWebUI {
					jsonData, err := json.Marshal(dhcpClientsSlice)
					if err != nil {
						log.Default().Printf("Successfully pushed data to WebSocket: %s", string(jsonData))
					}
				}
			}
		}
		b.clientsLock.Unlock()
	}
}

// Render HTML page
func (b *UIBackend) renderPage(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles(templatesDir + "/index.templ.html"))

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
		log.Default().Printf("WARN: missing headers in HTTP GET")
		//http.Error(w, "The request does not have the 'X-Ingress-Path' header", http.StatusBadRequest)
		//return
		XIngressPath = []string{""}
	}

	// compute pool size:
	dhcpPoolSize := 0
	if b.dhcpStartIP != nil && b.dhcpEndIP != nil {
		dhcpPoolSize = int(iprange.New(b.dhcpStartIP, b.dhcpEndIP).Size().Int64())
	}

	data := struct {
		WebSocketURI string
		DhcpStartIP  string
		DhcpEndIP    string
		DhcpPoolSize int
	}{
		// We use relative URL for the websocket in the form "/79957c2e_dnsmasq-dhcp/ingress/ws"
		// In this way we don't need to know whether the browser is passing through some TLS
		// reverse proxy or uses HomeAssistant built-in TLS or is connecting in plaintext (HTTP).
		// Based on the scheme used by the browser, the websocket will use the associated scheme
		// ('wss' for 'https' and 'ws' for 'http)
		WebSocketURI: XIngressPath[0] + websocketRelativeUrl,
		DhcpStartIP:  b.dhcpStartIP.String(),
		DhcpEndIP:    b.dhcpEndIP.String(),
		DhcpPoolSize: dhcpPoolSize,
	}

	err := tmpl.Execute(w, data)
	if err != nil {
		log.Default().Printf("WARN: error while rendering template: %s\n", err.Error())
		// keep going
	} else {
		log.Default().Printf("Successfully rendered web page template\n")
	}
}

// Read from the leasesCh and push to broadcastCh
func (b *UIBackend) processLeaseUpdates() {
	i := 0
	for {
		updatedLeases := <-b.leasesCh
		log.Default().Printf("INotify detected a change to the DHCP client lease file... updated list contains %d clients\n", len(updatedLeases))
		b.processLeaseUpdatesFromArray(updatedLeases)

		// once the new list of DHCP client data entries is ready, push it in broadcast
		b.broadcastCh <- b.dhcpClientData
		i += 1
	}
}

// Process a slice of dnsmasq.Lease and store that into the UIBackend object
func (b *UIBackend) processLeaseUpdatesFromArray(updatedLeases []*dnsmasq.Lease) {

	b.dhcpClientDataLock.Lock()
	b.dhcpClientData = make([]DhcpClientData, 0, len(updatedLeases) /* capacity */)
	for _, lease := range updatedLeases {

		d := DhcpClientData{Lease: *lease}

		// friendly-name metadata

		// do we have a friendly-name registered for this MAC address?
		metadata, ok := b.friendlyNames[lease.MacAddr.String()]
		if ok {
			// yes: enrich with some metadata this DHCP client entry
			d.FriendlyName = metadata.FriendlyName
		} else {
			if lease.Hostname != dnsmasqMarkerForMissingHostname {
				// no: user didn't provide any friendly name but the dnsmasq DHCP server
				// has received (over DHCP protocol) an hostname... better than nothing:
				// use that to create a "friendly name"
				d.FriendlyName = lease.Hostname
			}
		}

		// has-static-ip metadata
		_, hasReservation := b.ipAddressReservations[lease.IPAddr]
		if hasReservation {
			// the IP address provided in this lease is a reserved one...
			// check if the MAC address is the one for which that IP was intended...
			if strings.EqualFold(lease.MacAddr.String(), b.ipAddressReservations[lease.IPAddr].Mac) {
				d.HasStaticIP = true
			} else {
				log.Default().Printf("WARN: the IP %s was leased to MAC address %s, but in configuration it was reserved for MAC %s\n",
					lease.IPAddr.String(), lease.MacAddr.String(), b.ipAddressReservations[lease.IPAddr].Mac)
			}
		}

		// is-inside-dhcp-pool metadata
		d.IsInsideDHCPPool = IpInRange(lease.IPAddr, b.dhcpStartIP, b.dhcpEndIP)

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
	var cfg AddonConfig
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return err
	}

	// convert DHCP IP strings to net.IP
	b.dhcpStartIP = net.ParseIP(cfg.DhcpRange.StartIP)
	b.dhcpEndIP = net.ParseIP(cfg.DhcpRange.EndIP)
	if b.dhcpStartIP == nil || b.dhcpEndIP == nil {
		log.Default().Fatalf("Invalid DHCP range found in addon config file")
		return os.ErrInvalid // I'm lazy, recycle a similar error type
	}

	// ensure we have a valid port for web UI
	if cfg.WebUIPort <= 0 || cfg.WebUIPort > 32768 {
		log.Default().Fatalf("Invalid Web UI port in addon config file")
		return os.ErrInvalid // I'm lazy, recycle a similar error type
	}

	// copy basic settings
	b.serverPort = cfg.WebUIPort

	// convert IP address reservations to a map indexed by IP
	for _, r := range cfg.IpAddressReservations {
		ipAddr, err := netip.ParseAddr(r.IP)
		if err != nil {
			log.Default().Fatalf("Invalid IP address found inside 'ip_address_reservations': %s", r.IP)
			continue
		}
		macAddr, err := net.ParseMAC(r.Mac)
		if err != nil {
			log.Default().Fatalf("Invalid MAC address found inside 'ip_address_reservations': %s", r.Mac)
			continue
		}

		// normalize the IP and MAC address format (e.g. to lowercase)
		r.IP = ipAddr.String()
		r.Mac = macAddr.String()

		b.ipAddressReservations[ipAddr] = r
	}
	log.Default().Printf("Acquired %d IP address reservations\n", len(b.ipAddressReservations))

	// convert friendly names to a map of DhcpClientFriendlyName instances indexed by MAC address
	for _, client := range cfg.DhcpClientsFriendlyNames {
		macAddr, err := net.ParseMAC(client.Mac)
		if err != nil {
			log.Default().Fatalf("Invalid MAC address found inside 'dhcp_clients_friendly_names': %s", client.Mac)
			continue
		}

		b.friendlyNames[macAddr.String()] = DhcpClientFriendlyName{
			MacAddress:   macAddr,
			FriendlyName: client.Name,
		}
	}

	log.Default().Printf("Acquired %d friendly names\n", len(b.friendlyNames))

	b.logWebUI = cfg.LogWebUI
	log.Default().Printf("Web UI logging enabled=%t\n", b.logWebUI)

	return nil
}

// ListenAndServe is starting the whole UI backend:
// a web server, a WebSocket server, INotify-based watch on dnsmasq lease files, etc
func (b *UIBackend) ListenAndServe() error {

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
	log.Default().Printf("Starting server to listen on port %d\n", b.serverPort)
	b.server.Addr = fmt.Sprintf(":%d", b.serverPort)
	b.server.Handler = mux
	return b.server.ListenAndServe()
}
