package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"math/rand"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

/*

TODO: https://github.com/b0ch3nski/go-dnsmasq-utils
*/

func generateRandomIP() string {
	return fmt.Sprintf("192.168.0.%d", rand.Intn(255))
}

func generateRandomMAC() string {
	return fmt.Sprintf("00:0a:95:9d:68:%02x", rand.Intn(255))
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	for {
		ip := generateRandomIP()
		mac := generateRandomMAC()

		data := map[string]string{
			"ip":  ip,
			"mac": mac,
		}

		err = ws.WriteJSON(data)
		if err != nil {
			log.Println("Error writing JSON:", err)
			break
		}

		time.Sleep(5 * time.Second)
	}
}

func main() {
	http.HandleFunc("/ws", handleConnections)

	log.Println("Server WebSocket avviato su :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
