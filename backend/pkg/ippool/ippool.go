package ippool

import (
	"net/netip"
)

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
