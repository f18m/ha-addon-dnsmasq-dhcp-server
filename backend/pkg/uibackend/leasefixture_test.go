package uibackend

import (
	"os"
	"testing"

	"github.com/b0ch3nski/go-dnsmasq-utils/dnsmasq"
	"github.com/stretchr/testify/require"
)

// realLeaseFixture is a dnsmasq.leases file recorded from a real dnsmasq process serving real DHCP
// clients; see tests/integration/README.md and the 'test-integration-record-fixtures' Makefile target.
const realLeaseFixture = "../../../tests/integration/fixtures/dnsmasq.leases"

func TestRealDnsmasqLeaseFixtureIsParsable(t *testing.T) {
	f, err := os.Open(realLeaseFixture)
	if os.IsNotExist(err) {
		t.Skip("lease fixture not recorded yet: run 'make test-integration-record-fixtures'")
	}
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	leases, err := dnsmasq.ReadLeases(f)
	require.NoError(t, err)

	byMAC := map[string]string{}
	for _, l := range leases {
		byMAC[l.MacAddr.String()] = l.IPAddr.String()
	}
	require.Equal(t, "172.28.0.10", byMAC["aa:bb:cc:dd:ee:00"])
	require.Equal(t, "172.28.0.105", byMAC["aa:bb:cc:dd:ee:01"])
	require.Contains(t, byMAC, "aa:bb:cc:dd:ee:02")
	require.Contains(t, byMAC, "aa:bb:cc:dd:ee:03")
	require.NotContains(t, byMAC, "de:ad:be:ef:ca:fe")
}
