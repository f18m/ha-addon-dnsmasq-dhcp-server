package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Data struttura per contenere gli indirizzi IP e MAC
type Data struct {
	IP  string
	MAC string
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan []Data)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var mu sync.Mutex

var XFwdHost string
var XIngressPath string

// Middleware per loggare le richieste HTTP GET
func logRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Stampa il metodo e l'URL
			fmt.Printf("Metodo: %s, URL: %s\n", r.Method, r.URL.String())

			// Stampa tutti gli header
			fmt.Println("Headers:")
			for name, values := range r.Header {
				for _, value := range values {
					fmt.Printf("%s: %s\n", name, value)
				}

				if name == "X-Forwarded-Host" {
					XFwdHost = values[0]
				}
				if name == "X-Ingress-Path" {
					XIngressPath = values[0]
				}
			}
			fmt.Println("----")
		}
		// Prosegui con la richiesta
		next.ServeHTTP(w, r)
	})
}

// Handler per la connessione WebSocket
func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	mu.Lock()
	clients[ws] = true
	mu.Unlock()

	// In ascolto fino a chiusura della connessione
	for {
		var msg []Data
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("error: %v", err)
			mu.Lock()
			delete(clients, ws)
			mu.Unlock()
			break
		}
	}
}

// Broadcast degli aggiornamenti a tutti i client
func handleMessages() {
	for {
		msg := <-broadcast
		mu.Lock()
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
		mu.Unlock()
	}
}

// Pagina HTML servita
func renderPage(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("/opt/web/templates/index.templ.html"))

	data := struct {
		WebSocketHost string
	}{
		WebSocketHost: XFwdHost + XIngressPath,
	}

	tmpl.Execute(w, data)
}

// Simulazione di aggiornamenti degli indirizzi IP/MAC
func simulateData() {
	i := 0
	for {
		time.Sleep(5 * time.Second)
		newData := []Data{
			{IP: "192.168.1.1", MAC: "AA:BB:CC:DD:EE:01"},
			{IP: "192.168.1.2", MAC: "AA:BB:CC:DD:EE:02"},
			{IP: "192.168.1." + strconv.Itoa(i), MAC: "AA:BB:CC:DD:EE:02"},
			// Aggiorna questi dati con qualsiasi informazione reale o simulata
		}
		broadcast <- newData
		i += 1
	}
}

func main() {
	// File statici per la pagina web
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Wrappa il middleware di log per le richieste GET
	http.Handle("/", logRequestMiddleware(http.HandlerFunc(renderPage)))
	http.HandleFunc("/ws", handleConnections)

	// Gestione dei messaggi e simulazione dati
	go handleMessages()
	go simulateData()

	// Avvio del server
	fmt.Println("Server in ascolto su :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
