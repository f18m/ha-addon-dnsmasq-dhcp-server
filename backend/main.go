// Package main is the entry point for the DHCP clients web application backend.
package main

import (
	"dnsmasq-dhcp-backend/pkg/logger"
	"dnsmasq-dhcp-backend/pkg/uibackend"
)

func main() {
	logger := logger.NewCustomLogger("webui-backend")

	logger.Info("Web backend starting")

	ui := uibackend.NewUIBackend(logger)
	_ = ui.ListenAndServe()
}
