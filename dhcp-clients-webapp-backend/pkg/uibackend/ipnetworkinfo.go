package uibackend

import (
	"fmt"
	"net"
)

// IpNetworkInfo contains all details about a network attached to the addon, as specified in the config file
type IpNetworkInfo struct {
	Interface string
	Start     net.IP
	End       net.IP
	Gateway   net.IP
	Netmask   net.IPMask
}

func (nw IpNetworkInfo) HasValidIPs() bool {
	if nw.Start.IsPrivate() &&
		nw.End.IsPrivate() &&
		nw.Gateway.IsPrivate() {

		// net.IPMask.Size() returns 0,0 in case of invalid netmasks
		if ones, zeros := nw.Netmask.Size(); ones != 0 && zeros != 0 {
			// all private IPs and mask is valid: ok

			// check start/end IPs are in the same network

			theNetwork := net.IPNet{IP: nw.Start, Mask: nw.Netmask}
			return theNetwork.Contains(nw.End)
		}
	}
	return false
}

func (nw IpNetworkInfo) HasValidGateway() bool {
	// ensure IPs are of the same family (IPv4 or IPv6)
	theNetwork := net.IPNet{IP: nw.Start, Mask: nw.Netmask}

	// check that the gateway IP is inside the network
	return theNetwork.Contains(nw.Gateway)
}

func (nw IpNetworkInfo) String() string {
	return fmt.Sprintf("Interface: %s, Start: %s, End: %s, Gateway: %s, Netmask: %s",
		nw.Interface, nw.Start, nw.End, nw.Gateway, nw.Netmask)
}
