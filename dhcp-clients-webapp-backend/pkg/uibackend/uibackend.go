package uibackend

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"

	"github.com/b0ch3nski/go-dnsmasq-utils/dnsmasq"
	"github.com/gorilla/websocket"
)

var bindAddress = ":8080"
var defaultDnsmasqLeasesFile = "/var/lib/misc/dnsmasq.leases"

// These absolute paths must be in sync with the Dockerfile
var staticWebFilesDir = "/opt/web/static"
var templatesDir = "/opt/web/templates"

// DhcpClientData holds all the information the backend has about a particular DHCP client
type DhcpClientData struct {
	Lease       dnsmasq.Lease
	HasStaticIP bool
	PrettyName  string
}

type UIBackend struct {
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
func (b *UIBackend) handleWebSocketConns(w http.ResponseWriter, r *http.Request) {
	ws, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Default().Fatal(err)
	}
	defer ws.Close()

	// get a copy of latest status
	var updatedStatus []DhcpClientData
	b.dhcpClientDataLock.Lock()
	copy(updatedStatus, b.dhcpClientData)
	b.dhcpClientDataLock.Unlock()

	log.Default().Printf("Received new websocket client: pushing %d DHCP clients to it", len(updatedStatus))

	// register new client
	b.clientsLock.Lock()
	b.clients[ws] = true
	ws.WriteJSON(updatedStatus) // push the current status on the websocket
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

		log.Default().Printf("Processing DHCP client lease updates... received %d clients\n", len(updatedLeases))

		b.dhcpClientData = make([]DhcpClientData, 0, len(updatedLeases) /* capacity */)
		for _, lease := range updatedLeases {
			b.dhcpClientData = append(b.dhcpClientData, DhcpClientData{Lease: *lease})
		}

		// once the new list of DHCP client data entries is ready, push it in broadcast
		b.broadcastCh <- b.dhcpClientData
		i += 1
	}
}

func (b *UIBackend) ListenAndServe() error {

	mux := http.NewServeMux()

	// Serve static files, if any
	fs := http.FileServer(http.Dir(staticWebFilesDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Log requests (for debug only) + serve HTML pages
	mux.Handle("/", b.logRequestMiddleware(http.HandlerFunc(b.renderPage)))

	// Serve Websocket requests
	mux.HandleFunc("/ws", b.handleWebSocketConns)

	// Read from the leasesCh and push to broadcastCh
	go b.processLeaseUpdates()

	// Read from the broadcastCh chan and push to all Websocket clients
	go b.broadcastUpdatesToClients()

	// Watch for updates on DHCP leases file
	ctx := context.Background()
	go dnsmasq.WatchLeases(ctx, defaultDnsmasqLeasesFile, b.leasesCh)

	// Start server
	log.Default().Printf("Server listening on %s\n", bindAddress)
	b.server.Handler = mux
	return b.server.ListenAndServe()
}
