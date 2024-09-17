package uibackend

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"

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

	// ch used to broadcast tabular data from backend->frontend
	broadcastCh chan []DhcpClientData
}

func NewUIBackend() UIBackend {
	return UIBackend{
		clients:     make(map[*websocket.Conn]bool),
		broadcastCh: make(chan []DhcpClientData),
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
		log.Fatal(err)
	}
	defer ws.Close()

	b.clientsLock.Lock()
	b.clients[ws] = true
	b.clientsLock.Unlock()

	// listen till the end of the websocket
	for {
		var msg []DhcpClientData
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("error: %v", err)
			b.clientsLock.Lock()
			delete(b.clients, ws)
			b.clientsLock.Unlock()
			break
		}
	}
}

// Broadcast updater: any update posted on the broadcastCh is broadcasted to all clients
func (b *UIBackend) broadcastUpdatesToClients() {
	for {
		msg := <-b.broadcastCh
		b.clientsLock.Lock()
		for client := range b.clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(b.clients, client)
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

// FIXME
func (b *UIBackend) simulateData() {
	i := 0
	for {
		time.Sleep(5 * time.Second)
		newData := []DhcpClientData{
			//{Lease: dnsmasq.Lease{Hostname: "test"}, IP: "192.168.1.1", MAC: "AA:BB:CC:DD:EE:01"},
			//{IP: "192.168.1.2", MAC: "AA:BB:CC:DD:EE:02"},
			//{IP: "192.168.1." + strconv.Itoa(i), MAC: "AA:BB:CC:DD:EE:02"},
			// Aggiorna questi dati con qualsiasi informazione reale o simulata
		}
		b.broadcastCh <- newData
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

	// Sync the broadcastCh chan with all clients
	go b.broadcastUpdatesToClients()

	// FIXME
	go b.simulateData()

	// Watch for updates on DHCP leases file
	ctx := context.Background()
	leases := make(chan []*dnsmasq.Lease)
	go dnsmasq.WatchLeases(ctx, defaultDnsmasqLeasesFile, leases)

	// Start server
	log.Default().Printf("Server listening on %s\n", bindAddress)
	b.server.Handler = mux
	return b.server.ListenAndServe()
}
