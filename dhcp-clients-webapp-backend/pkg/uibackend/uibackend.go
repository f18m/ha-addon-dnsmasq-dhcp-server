// This package implements an HTML/Websocket server that provides a tabular view of
// all DHCP clients served by the dnsmasq instance that lives into this HomeAssistant addon
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
	"os"
	"sync"
	"time"

	"github.com/b0ch3nski/go-dnsmasq-utils/dnsmasq"
	"github.com/gorilla/websocket"
)

var bindAddress = ":8080"
var defaultDnsmasqLeasesFile = "/data/dnsmasq.leases"
var defaultHomeAssistantConfigFile = "/data/options.json"
var dnsmasqMarkerForMissingHostname = "*"

// These absolute paths must be in sync with the Dockerfile
var staticWebFilesDir = "/opt/web/static"
var templatesDir = "/opt/web/templates"

// DhcpClientData holds all the information the backend has about a particular DHCP client
type DhcpClientData struct {
	// the lease as it is parsed from dnsmasq LEASE file:
	Lease dnsmasq.Lease

	// metadata associated with the DHCP client (obtained from configuration):
	HasStaticIP  bool
	FriendlyName string
}

func LeaseTimeToString(t time.Time) string {

	if t.IsZero() {
		return "Never expires"
	}

	now := time.Now()
	duration := t.Sub(now)
	if duration < 0 {
		return "Expired"
	}

	// compute hours, min, secs
	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%02dd, %02dh, %02dm, %02ds", days, hours, minutes, seconds)
	} else if hours > 0 {
		return fmt.Sprintf("%02dh, %02dm, %02ds", hours, minutes, seconds)
	} else {
		return fmt.Sprintf("%02dm, %02ds", minutes, seconds)
	}
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
		HasStaticIP  bool   `json:"has_static_ip"`
		FriendlyName string `json:"friendly_name"`
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
		HasStaticIP:  d.HasStaticIP,
		FriendlyName: d.FriendlyName,
	})
}

// DhcpClientFriendlyName
type DhcpClientFriendlyName struct {
	MacAddress   net.HardwareAddr
	FriendlyName string
}

type UIBackend struct {
	// DHCP client friendly names, as read from the configuration
	// The key of this map is the MAC address formatted as string (since net.HardwareAddr is not a valid map key type)
	friendlyNames map[string]DhcpClientFriendlyName

	// the actual HTTP server
	server   http.Server
	upgrader websocket.Upgrader

	// map of connected websockets
	clients     map[*websocket.Conn]bool
	clientsLock sync.Mutex

	// the most updated view on DHCP clients currently available
	dhcpClientData     []DhcpClientData
	dhcpClientDataLock sync.Mutex

	// ch used to broadcast tabular data from backend->frontend
	broadcastCh chan []DhcpClientData

	// ch used to link a goroutine watching for DHCP lease file changes and the lease file processor
	leasesCh chan []*dnsmasq.Lease
}

func NewUIBackend() UIBackend {
	return UIBackend{
		friendlyNames:  make(map[string]DhcpClientFriendlyName),
		clients:        make(map[*websocket.Conn]bool),
		dhcpClientData: nil,
		broadcastCh:    make(chan []DhcpClientData),
		leasesCh:       make(chan []*dnsmasq.Lease),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		server: http.Server{
			Addr:    bindAddress,
			Handler: nil,
		},
	}
}

func (b *UIBackend) logRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Default().Printf("Method: %s, URL: %s, Host: %s, RemoteAddr: %s\n", r.Method, r.URL.String(), r.Host, r.RemoteAddr)

		// print headers
		fmt.Println("Headers:")
		for name, values := range r.Header {
			for _, value := range values {
				fmt.Printf("  %s: %s\n", name, value)
			}
		}
		fmt.Println("----")

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
		msg := <-b.broadcastCh

		// loop over all clients
		b.clientsLock.Lock()
		for client := range b.clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Default().Printf("Error while writing JSON to WebSocket: %v", err)
				client.Close()
				delete(b.clients, client)
			} else {
				log.Default().Printf("Successfully pushed data to WebSocket: %v", msg)
			}
		}
		b.clientsLock.Unlock()
	}
}

// Render HTML page
func (b *UIBackend) renderPage(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles(templatesDir + "/index.templ.html"))

	XFwdHost, ok1 := r.Header["X-Forwarded-Host"]
	XIngressPath, ok2 := r.Header["X-Ingress-Path"]
	var WebSocketHost string
	if !ok1 || !ok2 || len(XFwdHost) == 0 || len(XIngressPath) == 0 {
		log.Default().Printf("WARN: missing headers in HTTP GET")
		http.Error(w, "The request does not have the 'X-Forwarded-Host' and 'X-Ingress-Path' headers", http.StatusBadRequest)
		return
		//log.Default().Printf("The request does not have the 'X-Forwarded-Host' and 'X-Ingress-Path' headers")
		//WebSocketHost = r.Host
	} else {
		WebSocketHost = XFwdHost[0] + XIngressPath[0]
	}

	data := struct {
		WebSocketHost string
	}{
		WebSocketHost: WebSocketHost,
	}

	err := tmpl.Execute(w, data)
	if err != nil {
		log.Default().Printf("WARN: error while rendering template: %s\n", err.Error())
		// keep going
	} else {
		log.Default().Printf("Successfully rendered web page template using WebSocketHost=%s\n", data.WebSocketHost)
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
func (b *UIBackend) processLeaseUpdatesFromArray(updatedLeases []*dnsmasq.Lease) error {
	b.dhcpClientDataLock.Lock()
	b.dhcpClientData = make([]DhcpClientData, 0, len(updatedLeases) /* capacity */)
	for _, lease := range updatedLeases {

		d := DhcpClientData{Lease: *lease}

		metadata, ok := b.friendlyNames[lease.MacAddr.String()]
		if ok {
			// enrich with some metadata this DHCP client entry
			d.FriendlyName = metadata.FriendlyName
		} else {
			if lease.Hostname != dnsmasqMarkerForMissingHostname {
				// dnsmasq DHCP server has received an hostname so use that to create a "friendly name"
				d.FriendlyName = "Hostname: " + lease.Hostname
			}
		}

		b.dhcpClientData = append(b.dhcpClientData, d)
	}
	b.dhcpClientDataLock.Unlock()

	log.Default().Printf("Updated DHCP clients internal status with %d entries\n", len(b.dhcpClientData))

	return nil
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

	return b.processLeaseUpdatesFromArray(leases)
}

// dummy struct used to unmarshal HomeAssistant option file correctly
type DhcpClients struct {
	Clients []struct {
		Name string `json:"name"`
		Mac  string `json:"mac"`
	} `json:"dhcp_clients_friendly_names"`
}

func (b *UIBackend) readAddonFriendlyNames() error {
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
	var dhcpClients DhcpClients
	err = json.Unmarshal(data, &dhcpClients)
	if err != nil {
		return err
	}

	// convert to a slice of DhcpClientFriendlyName
	for _, client := range dhcpClients.Clients {
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

	return nil
}

func (b *UIBackend) ListenAndServe() error {

	mux := http.NewServeMux()

	// Serve static files, if any
	fs := http.FileServer(http.Dir(staticWebFilesDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Log requests (for debug only) + serve HTML pages
	mux.Handle("/", b.logRequestMiddleware(http.HandlerFunc(b.renderPage)))

	// Serve Websocket requests
	mux.HandleFunc("/ws", b.handleWebSocketConn)

	// Read friendly names from the HomeAssistant addon config
	if err := b.readAddonFriendlyNames(); err != nil {
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
	log.Default().Printf("Starting server to listen on %s\n", bindAddress)
	b.server.Handler = mux
	return b.server.ListenAndServe()
}
