package main

import (
	"dhcp-clients-webapp-backend/pkg/uibackend"
	"log"
	"os"
)

func main() {
	logger := log.New(os.Stderr, "UI Backend: ", log.LstdFlags)
	logger.Print("Web backend starting")

	ui := uibackend.NewUIBackend(logger)
	logger.Fatal(ui.ListenAndServe())
}
