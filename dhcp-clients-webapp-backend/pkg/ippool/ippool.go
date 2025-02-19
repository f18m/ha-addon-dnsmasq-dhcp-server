package ippool

import (
	"bytes"
	"math/big"
	"net"
	"net/netip"
)

/* -------------------------------------------------------------------------- */
/*                                    Range                                   */
/* -------------------------------------------------------------------------- */

type Range struct {
	Start net.IP
	End   net.IP
}

func NewRange(start, end net.IP) Range {
	return Range{
		Start: start,
		End:   end,
	}
}

func NewRangeFromString(start, end string) Range {
	return Range{
		Start: net.ParseIP(start),
		End:   net.ParseIP(end),
	}
}

func (r Range) IsValid() bool {
	return r.Start != nil && r.End != nil
}

// Contains checks if the IP address is within the Range
func (r Range) Contains(ipOrig netip.Addr) bool {
	// Ensure that all IP addresses are in a consistent IPv4 or IPv6 form
	ip := net.IP(ipOrig.AsSlice()).To16()
	dhcpStartIP := r.Start.To16()
	dhcpEndIP := r.End.To16()

	if ip == nil || dhcpStartIP == nil || dhcpEndIP == nil {
		return false
	}

	// Check if the IP address is between dhcpStartIP and dhcpEndIP
	return bytes.Compare(ip, dhcpStartIP) >= 0 && bytes.Compare(ip, dhcpEndIP) <= 0
}

// Size returns the number of IP addresses in the range or -1 if they are too many to fit an int64
func (r Range) Size() int64 {
	size := big.NewInt(0)
	size.Add(size, big.NewInt(0).SetBytes(r.End))
	size.Sub(size, big.NewInt(0).SetBytes(r.Start))
	size.Add(size, big.NewInt(1))
	if size.IsInt64() {
		return size.Int64()
	}

	// too many IPs in range... this can happen with IPv6
	return -1
}

/* -------------------------------------------------------------------------- */
/*                                    Pool                                    */
/* -------------------------------------------------------------------------- */

type Pool struct {
	Ranges []Range
}

func NewPool(ranges []Range) Pool {
	return Pool{
		Ranges: ranges,
	}
}
func NewPoolFromString(start, end string) Pool {
	return Pool{
		Ranges: []Range{NewRangeFromString(start, end)},
	}
}

func (p Pool) Contains(ip netip.Addr) bool {
	for _, r := range p.Ranges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

func (p Pool) Size() int64 {
	size := int64(0)
	for _, r := range p.Ranges {
		s := r.Size()
		if s == -1 {
			return -1
		}
		size += s
	}
	return size
}
