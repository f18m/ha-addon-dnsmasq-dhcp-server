package trackerdb

import (
	"encoding/json"
	"net"
	"time"

	// import sqlite3 driver, so that database/sql package will know how to deal with "sqlite3" type
	_ "github.com/mattn/go-sqlite3"
)

// DhcpClient represents the structure for a DHCP client.
// The DHCP client might be currently connected to the server or not; in other words this
// may represent a DHCP client that has been connected in the past, but currently is not.
type DhcpClient struct {
	MacAddr                net.HardwareAddr
	Hostname               string
	HasStaticIP            bool
	FriendlyName           string
	LastSeen               time.Time
	DhcpServerStartCounter int
}

// MarshalJSON customizes the JSON serialization for DhcpClientData
func (d DhcpClient) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		MacAddr                string `json:"mac_addr"`
		Hostname               string `json:"hostname"`
		HasStaticIP            bool   `json:"has_static_ip"`
		FriendlyName           string `json:"friendly_name"`
		LastSeen               int64  `json:"last_seen"`
		DhcpServerStartCounter int    `json:"dhcp_server_start_counter"`
	}{
		MacAddr:                d.MacAddr.String(),
		Hostname:               d.Hostname,
		HasStaticIP:            d.HasStaticIP,
		FriendlyName:           d.FriendlyName,
		LastSeen:               d.LastSeen.Unix(),
		DhcpServerStartCounter: d.DhcpServerStartCounter,
	})
}
