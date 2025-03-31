package trackerdb

import (
	"net"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

// MustParseMAC acts like ParseMAC but panics if in case of an error
func MustParseMAC(s string) net.HardwareAddr {
	mac, err := net.ParseMAC(s)
	if err != nil {
		panic(err)
	}
	return mac
}

// CompareDhcpClients compares two DhcpClient instances and returns true if they are equal, false otherwise
func CompareDhcpClients(client1, client2 DhcpClient) bool {
	// Compare all fields
	return client1.MacAddr.String() == client2.MacAddr.String() &&
		client1.Hostname == client2.Hostname &&
		client1.DhcpServerStartEpoch == client2.DhcpServerStartEpoch

	// FIXME: understand why copying time.Time instances seems to alter them:
	/*&&
	client1.LastSeen.String() == client2.LastSeen.String()*/
}

// CompareDhcpClientSlices compares two slices of DhcpClient instances and returns true if they are equal, false otherwise
func CompareDhcpClientSlices(slice1, slice2 []DhcpClient) bool {
	// First, check if the slices are of the same length
	if len(slice1) != len(slice2) {
		return false
	}

	// Compare each DhcpClient in both slices
	for i := range slice1 {
		if !CompareDhcpClients(slice1[i], slice2[i]) {
			return false
		}
	}

	// If all elements are equal, return true
	return true
}

// TestAddDhcpClient tests the AddDhcpClient method.
func TestAddDhcpClient(t *testing.T) {
	client := DhcpClient{
		MacAddr:  MustParseMAC("AA:BB:CC:DD:EE:FF"),
		Hostname: "test-host",
		// HasStaticIP:  true,
		// FriendlyName: "Test Client",
		LastSeen: time.Now(),
	}

	db := NewTestDBWithData([]DhcpClient{client})

	// Check that the client was successfully added
	retrievedClient, err := db.GetDhcpClient(client.MacAddr)
	assert.NoError(t, err, "Failed to retrieve added client")

	// Compare the inserted and retrieved client using assertions
	assert.Equal(t, client.MacAddr, retrievedClient.MacAddr, "MacAddr mismatch")
	assert.Equal(t, client.Hostname, retrievedClient.Hostname, "Hostname mismatch")

	// Allow for slight differences in time, but the retrieved and original times should be very close
	assert.WithinDuration(t, client.LastSeen, retrievedClient.LastSeen, time.Second, "LastSeen timestamp mismatch")
}

// TestGetDhcpClient tests the GetDhcpClient method.
func TestGetDhcpClient(t *testing.T) {
	client := DhcpClient{
		MacAddr:  MustParseMAC("AA:BB:CC:DD:EE:FF"),
		Hostname: "test-host",
		// HasStaticIP:  true,
		// FriendlyName: "Test Client",
		LastSeen: time.Now(),
	}

	db := NewTestDBWithData([]DhcpClient{client})

	// Test retrieving an existing client
	retrievedClient, err := db.GetDhcpClient(client.MacAddr)
	assert.NoError(t, err, "Failed to retrieve client")

	// Check the values of the retrieved client using assertions
	assert.Equal(t, client.MacAddr, retrievedClient.MacAddr, "MacAddr mismatch")
	assert.Equal(t, client.Hostname, retrievedClient.Hostname, "Hostname mismatch")
	assert.WithinDuration(t, client.LastSeen, retrievedClient.LastSeen, time.Second, "LastSeen timestamp mismatch")

	// Test retrieving a non-existent client
	_, err = db.GetDhcpClient(MustParseMAC("FF:EE:DD:CC:BB:AA"))
	assert.Error(t, err, "Expected error when retrieving non-existent client, but got nil")
}

// TestGetDeadDhcpClients tests the GetDeadDhcpClients method.
func TestGetDeadDhcpClients(t *testing.T) {
	timeNow := time.Now()

	// Create some test DHCP clients in the database
	clientsInDB := []DhcpClient{
		{
			MacAddr:  MustParseMAC("AA:BB:CC:DD:EE:FF"),
			Hostname: "test-host-1",
			// HasStaticIP:  true,
			// FriendlyName: "Test Client 1",
			LastSeen: timeNow,
		},
		{
			MacAddr:  MustParseMAC("11:22:33:44:55:66"),
			Hostname: "test-host-2",
			// HasStaticIP:  false,
			// FriendlyName: "Test Client 2",
			LastSeen: timeNow,
		},
		{
			MacAddr:  MustParseMAC("77:88:99:AA:BB:CC"),
			Hostname: "test-host-3",
			// HasStaticIP:  true,
			// FriendlyName: "Test Client 3",
			LastSeen: timeNow,
		},
	}

	db := NewTestDBWithData(clientsInDB)

	// Case 1: One of the alive clients is in the database
	clientData := []net.HardwareAddr{
		MustParseMAC("AA:BB:CC:DD:EE:FF"),
	}

	// Expected output: Clients not in the provided slice (two remaining clients)
	expectedMissingClients := []DhcpClient{
		clientsInDB[1],
		clientsInDB[2],
	}

	// Run the test
	missingClients, err := db.GetDeadDhcpClients(clientData)
	assert.NoError(t, err, "Unexpected error while getting clients not in data")
	assert.Equal(t, CompareDhcpClientSlices(expectedMissingClients, missingClients), true, "Mismatch in missing clients")

	// Case 2: All alive clients are in the database
	clientData = []net.HardwareAddr{
		MustParseMAC("AA:BB:CC:DD:EE:FF"),
		MustParseMAC("11:22:33:44:55:66"),
		MustParseMAC("77:88:99:AA:BB:CC"),
	}

	// Expected output: No missing clients
	expectedMissingClients = []DhcpClient{}

	// Run the test
	missingClients, err = db.GetDeadDhcpClients(clientData)
	assert.NoError(t, err, "Unexpected error while getting clients not in data")
	assert.Equal(t, CompareDhcpClientSlices(expectedMissingClients, missingClients), true, "Expected no missing clients but found some")

	// Case 3: None of the alive clients are in the database
	clientData = []net.HardwareAddr{
		MustParseMAC("99:88:77:66:55:44"),
		MustParseMAC("FF:EE:DD:CC:BB:AA"),
	}

	// Expected output: All clients from the database
	expectedMissingClients = clientsInDB

	// Run the test
	missingClients, err = db.GetDeadDhcpClients(clientData)
	assert.NoError(t, err, "Unexpected error while getting clients not in data")
	assert.Equal(t, CompareDhcpClientSlices(expectedMissingClients, missingClients), true, "Mismatch in missing clients when none are in the provided data")
}
