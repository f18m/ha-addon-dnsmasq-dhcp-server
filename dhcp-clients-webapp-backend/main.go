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
	tmpl := template.Must(template.ParseFiles("templates/index.templ.html"))
	tmpl.Execute(w, nil)
}

// Simulazione di aggiornamenti degli indirizzi IP/MAC
func simulateData() {
	i := 0
	for {
		time.Sleep(5 * time.Second)
		newData := []Data{
			{IP: "192.168.1." + strconv.Itoa(i), MAC: "AA:BB:CC:DD:EE:01"},
			// Aggiorna questi dati con qualsiasi informazione reale o simulata
		}
		broadcast <- newData
		i += 1
	}
}

func main() {
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", renderPage)
	http.HandleFunc("/ws", handleConnections)

	go handleMessages()
	go simulateData()

	fmt.Println("Server in ascolto su :8099")
	log.Fatal(http.ListenAndServe(":8099", nil))
}
