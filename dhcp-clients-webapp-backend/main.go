package main

import (
	"dhcp-clients-webapp-backend/pkg/uibackend"
	"fmt"
	"log"
)

func main() {
	fmt.Println("Web backend starting")

	ui := uibackend.NewUIBackend()
	log.Fatal(ui.ListenAndServe())
}
