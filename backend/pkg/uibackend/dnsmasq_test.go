package uibackend

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestGetDnsStats_NoUpstreamServers(t *testing.T) {
	// Start a temporary dnsmasq instance
	dnsmasqCmd := exec.Command("dnsmasq", "--port=12345", "--cache-size=100", "--no-daemon", "--no-resolv") // Adjust arguments as needed
	if err := dnsmasqCmd.Start(); err != nil {
		t.Fatalf("Failed to start dnsmasq: %v", err)
	}
	defer func() {
		if err := dnsmasqCmd.Process.Kill(); err != nil {
			t.Errorf("Failed to kill dnsmasq: %v", err)
		}
	}()

	// Wait for dnsmasq to start listening
	for i := 0; i < 10; i++ {
		if conn, err := net.DialTimeout("tcp", "localhost:12345", 1*time.Second); err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	stats, err := getDnsStats("localhost", 12345)
	if err != nil {
		t.Fatalf("getDnsStats failed: %v", err)
	}

	// Assertions
	if stats.CacheSize != 100 {
		t.Errorf("Unexpected CacheSize: got %d, want %d", stats.CacheSize, 100)
	}

	// Check for upstream servers.  Since we started with --no-resolv, there shouldn't be any upstream servers initially.
	if len(stats.UpstreamServers) != 0 {
		t.Errorf("Unexpected Upstream Servers found: %v", stats.UpstreamServers)
	}
}

func TestGetDnsStats_WithUpstreamServers(t *testing.T) {
	// Test with resolv-file, simulating an upstream server
	resolvFileContent := "nameserver 8.8.4.4" // Example upstream server
	resolvFilePath := "/tmp/resolv.conf"      // Choose a temporary file
	err := writeTempFile(resolvFilePath, resolvFileContent)
	if err != nil {
		t.Fatalf("Failed to write temporary resolv file: %v", err)
	}

	// Restart dnsmasq with the resolv file
	dnsmasqCmd := exec.Command("dnsmasq", "--port=12346", "--cache-size=100", "--no-daemon", fmt.Sprintf("--resolv-file=%s", resolvFilePath)) //nolint:gosec
	if err := dnsmasqCmd.Start(); err != nil {
		t.Fatalf("Failed to restart dnsmasq with resolv-file: %v", err)
	}
	defer func() {
		if err := dnsmasqCmd.Process.Kill(); err != nil {
			t.Errorf("Failed to kill dnsmasq: %v", err)
		}
	}()
	for i := 0; i < 10; i++ {
		if conn, err := net.DialTimeout("tcp", "localhost:12346", 1*time.Second); err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	stats, err := getDnsStats("localhost", 12346)
	if err != nil {
		t.Fatalf("getDnsStats failed with resolv-file: %v", err)
	}
	if len(stats.UpstreamServers) != 1 {
		t.Errorf("Expected Upstream Servers but found none")
	}
	if stats.UpstreamServers[0].ServerURL != "8.8.4.4#53" {
		t.Errorf("Expected google upstream Servers but found something else")
	}
}

// Helper function to write to a temporary file
func writeTempFile(filePath, content string) error {
	file, err := os.Create(filePath) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	_, err = file.WriteString(content)
	return err
}
