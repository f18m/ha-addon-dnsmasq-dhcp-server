package trackerdb

import (
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

// TestAddDhcpClient tests the AddDhcpClient method.
func TestAddDhcpClient(t *testing.T) {
	db := NewTestDB()

	// Define a test client
	client := DhcpClient{
		MacAddr:      "AA:BB:CC:DD:EE:FF",
		Hostname:     "test-host",
		HasStaticIP:  true,
		FriendlyName: "Test Client",
		LastSeen:     time.Now(),
	}

	// Test adding the client
	err := db.TrackNewDhcpClient(client)
	assert.NoError(t, err, "Failed to add client")

	// Check that the client was successfully added
	retrievedClient, err := db.GetDhcpClient(client.MacAddr)
	assert.NoError(t, err, "Failed to retrieve added client")

	// Compare the inserted and retrieved client using assertions
	assert.Equal(t, client.MacAddr, retrievedClient.MacAddr, "MacAddr mismatch")
	assert.Equal(t, client.Hostname, retrievedClient.Hostname, "Hostname mismatch")
	assert.Equal(t, client.HasStaticIP, retrievedClient.HasStaticIP, "HasStaticIP mismatch")
	assert.Equal(t, client.FriendlyName, retrievedClient.FriendlyName, "FriendlyName mismatch")

	// Allow for slight differences in time, but the retrieved and original times should be very close
	assert.WithinDuration(t, client.LastSeen, retrievedClient.LastSeen, time.Second, "LastSeen timestamp mismatch")
}

// TestGetDhcpClient tests the GetDhcpClient method.
func TestGetDhcpClient(t *testing.T) {
	db := NewTestDB()

	// Define a test client
	client := DhcpClient{
		MacAddr:      "AA:BB:CC:DD:EE:FF",
		Hostname:     "test-host",
		HasStaticIP:  true,
		FriendlyName: "Test Client",
		LastSeen:     time.Now(),
	}

	// Add the client to the database
	err := db.TrackNewDhcpClient(client)
	assert.NoError(t, err, "Failed to add client")

	// Test retrieving an existing client
	retrievedClient, err := db.GetDhcpClient(client.MacAddr)
	assert.NoError(t, err, "Failed to retrieve client")

	// Check the values of the retrieved client using assertions
	assert.Equal(t, client.MacAddr, retrievedClient.MacAddr, "MacAddr mismatch")
	assert.Equal(t, client.Hostname, retrievedClient.Hostname, "Hostname mismatch")
	assert.Equal(t, client.HasStaticIP, retrievedClient.HasStaticIP, "HasStaticIP mismatch")
	assert.Equal(t, client.FriendlyName, retrievedClient.FriendlyName, "FriendlyName mismatch")
	assert.WithinDuration(t, client.LastSeen, retrievedClient.LastSeen, time.Second, "LastSeen timestamp mismatch")

	// Test retrieving a non-existent client
	_, err = db.GetDhcpClient("FF:EE:DD:CC:BB:AA")
	assert.Error(t, err, "Expected error when retrieving non-existent client, but got nil")
}
