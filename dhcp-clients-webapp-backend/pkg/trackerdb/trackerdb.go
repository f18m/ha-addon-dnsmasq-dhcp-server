package trackerdb

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"time"

	// import sqlite3 driver, so that database/sql package will know how to deal with "sqlite3" type
	_ "github.com/mattn/go-sqlite3"
)

// DhcpClient represents the structure for a DHCP client.
// The DHCP client might be currently connected to the server or not; in other words this
// may represent a DHCP client that has been connected in the past, but currently is not.
type DhcpClient struct {
	MacAddr      net.HardwareAddr `json:"mac_addr"`
	Hostname     string           `json:"hostname"`
	HasStaticIP  bool             `json:"has_static_ip"`
	FriendlyName string           `json:"friendly_name"`
	LastSeen     time.Time        `json:"last_seen"`
}

// DhcpClientTrackerDB manages the database operations for DHCP clients.
type DhcpClientTrackerDB struct {
	DB *sql.DB
}

// NewDhcpClientTrackerDB initializes the database.
func NewDhcpClientTrackerDB(dbPath string) (*DhcpClientTrackerDB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create table if not exists
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS dhcp_clients (
		mac_addr TEXT PRIMARY KEY,
		hostname TEXT,
		has_static_ip INTEGER,
		friendly_name TEXT,
		last_seen TEXT
	);
	`
	_, err = db.Exec(createTableQuery)
	if err != nil {
		return nil, err
	}

	return &DhcpClientTrackerDB{DB: db}, nil
}

// NewTestDB returns a mock DB for testing
func NewTestDB() DhcpClientTrackerDB {
	// Create an in-memory SQLite database for testing
	db, err := NewDhcpClientTrackerDB(":memory:")
	if err != nil {
		log.Fatal("Failed to initialize test database")
	}
	return *db
}

// NewTestDBWithData returns a mock DB for testing
func NewTestDBWithData(clientsInDB []DhcpClient) DhcpClientTrackerDB {
	db := NewTestDB()

	// Insert test data into the database
	for _, client := range clientsInDB {
		err := db.TrackNewDhcpClient(client)
		if err != nil {
			log.Fatal("Failed to initialize test database")
		}
	}
	return db
}

// TrackNewDhcpClient inserts a new DHCP client into the database.
func (d *DhcpClientTrackerDB) TrackNewDhcpClient(client DhcpClient) error {
	insertQuery := `
	INSERT INTO dhcp_clients (mac_addr, hostname, has_static_ip, friendly_name, last_seen)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(mac_addr) DO UPDATE SET 
		hostname=excluded.hostname, 
		has_static_ip=excluded.has_static_ip,
		friendly_name=excluded.friendly_name,
		last_seen=excluded.last_seen;
	`

	_, err := d.DB.Exec(insertQuery, client.MacAddr.String(), client.Hostname, client.HasStaticIP, client.FriendlyName, client.LastSeen.Format(time.RFC3339))
	if err != nil {
		return err
	}

	return nil
}

// GetDhcpClient retrieves a DHCP client by its MAC address.
func (d *DhcpClientTrackerDB) GetDhcpClient(macAddr net.HardwareAddr) (*DhcpClient, error) {
	query := `SELECT mac_addr, hostname, has_static_ip, friendly_name, last_seen FROM dhcp_clients WHERE mac_addr = ?`
	row := d.DB.QueryRow(query, macAddr.String())

	var client DhcpClient
	var lastSeen string
	var mac string

	err := row.Scan(&mac, &client.Hostname, &client.HasStaticIP, &client.FriendlyName, &lastSeen)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("client with mac_addr %s not found", macAddr)
		}
		return nil, err
	}

	client.MacAddr, err = net.ParseMAC(mac)
	if err != nil {
		return nil, err
	}

	client.LastSeen, err = time.Parse(time.RFC3339, lastSeen)
	if err != nil {
		return nil, err
	}

	return &client, nil
}

/*

// MarshalDhcpClient marshals the DHCP client struct to a JSON string.
func (d *DhcpClientTrackerDB) MarshalDhcpClient(macAddr string) (string, error) {
	client, err := d.GetDhcpClient(macAddr)
	if err != nil {
		return "", err
	}

	clientJSON, err := json.Marshal(client)
	if err != nil {
		return "", err
	}

	return string(clientJSON), nil
}

// UnmarshalDhcpClient unmarshals a JSON string into a DHCP client and stores it in the database.
func (d *DhcpClientTrackerDB) UnmarshalDhcpClient(data string) error {
	var client DhcpClient
	err := json.Unmarshal([]byte(data), &client)
	if err != nil {
		return err
	}

	return d.AddDhcpClient(client)
}
*/

// GetDeadDhcpClients finds DHCP clients in the database that are NOT appearing in the given list of MAC addresses
// which identifies the currently-alive DHCP clients.
func (d *DhcpClientTrackerDB) GetDeadDhcpClients(aliveClients []net.HardwareAddr) ([]DhcpClient, error) {
	// Step 1: Get all DHCP clients from the database
	rows, err := d.DB.Query("SELECT mac_addr, hostname, has_static_ip, friendly_name, last_seen FROM dhcp_clients")
	if err != nil {
		return nil, fmt.Errorf("failed to query dhcp_clients: %v", err)
	}
	defer rows.Close()

	// Create a map to store the MAC addresses from aliveClients slice for quick lookup
	macAddrMap := make(map[string]struct{})
	for _, aliveMAC := range aliveClients {
		macAddrMap[aliveMAC.String()] = struct{}{}
	}

	// Step 2: Collect all clients from the database
	var deadClients []DhcpClient
	for rows.Next() {
		var client DhcpClient
		var lastSeenStr string
		var mac string

		// Scan the row data into the DhcpClient struct
		err := rows.Scan(&mac, &client.Hostname, &client.HasStaticIP, &client.FriendlyName, &lastSeenStr)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		client.MacAddr, err = net.ParseMAC(mac)
		if err != nil {
			return nil, err
		}

		// Convert lastSeenStr to time.Time format (assuming it's stored as text)
		client.LastSeen, err = parseTime(lastSeenStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse LastSeen: %v", err)
		}

		// Step 3: Check if the client MAC address exists in the provided DhcpClientData slice
		if _, exists := macAddrMap[client.MacAddr.String()]; !exists {
			// If the MAC address is not in the slice, add the client to the result
			deadClients = append(deadClients, client)
		}
	}

	// Return the slice of DhcpClient instances that are not present in the provided DhcpClientData slice
	return deadClients, nil
}

// Helper function to parse a time string (assuming stored as ISO 8601 or RFC3339 format)
func parseTime(timeStr string) (time.Time, error) {
	return time.Parse(time.RFC3339, timeStr)
}
