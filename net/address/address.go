package address

import (
	"fmt"
	"net"

	"github.com/weaveworks/weave/common"
)

// Using 32-bit integer to represent IPv4 address
type Address uint32
type Offset uint32

type Range struct {
	Start, End Address // [Start, End); Start <= End
}

func NewRange(start Address, size Offset) Range {
	return Range{Start: start, End: Add(start, size)}
}
func (r Range) Size() Offset               { return Subtract(r.End, r.Start) }
func (r Range) String() string             { return fmt.Sprintf("%s-%s", r.Start, r.End-1) }
func (r Range) Overlaps(or Range) bool     { return !(r.Start >= or.End || r.End <= or.Start) }
func (r Range) Contains(addr Address) bool { return addr >= r.Start && addr < r.End }
func (r Range) Equals(or Range) bool       { return r.Start == or.Start && r.End == or.End }

// IsCIDR checks whether the range is CIDR.
func (r Range) IsCIDR() bool {
	start, end := r.Start, r.End-1
	for mask := ^Address(0); mask != 0; mask <<= 1 {
		if (mask&start == start) && (^mask|start == end) {
			return true
		}
	}
	return false
}

func (r Range) AsCIDRString() string {
	prefixLen := 32
	for size := r.Size(); size > 1; size = size / 2 {
		if size%2 != 0 { // Size not a power of two; cannot be expressed as a CIDR.
			return r.String()
		}
		prefixLen--
	}
	return CIDR{Addr: r.Start, PrefixLen: prefixLen}.String()
}

func MakeCIDR(subnet CIDR, addr Address) CIDR {
	return CIDR{Addr: addr, PrefixLen: subnet.PrefixLen}
}

// CIDRs return list of subranges in the CIDR notation which covers completely
// the [r.Start, r.end) range
func (r Range) CIDRs() []CIDR {
	start, end := r.Start, r.End-1
	cidrList := make([]CIDR, 0)

	const (
		fullMask     = ^Address(0)
		maxPrefixLen = 32
	)

	for end >= start {
		mask := fullMask
		prefixLen := maxPrefixLen
		for mask > 0 {
			tmpMask := mask << 1
			if (start&tmpMask) != start || (start|^tmpMask) > end {
				break
			}
			mask = tmpMask
			prefixLen--
		}
		cidrList = append(cidrList, CIDR{start, prefixLen})
		start |= ^mask
		if (start + 1) < start { // check for overflow
			break
		}
		start++
	}

	return cidrList
}

type CIDR struct {
	Addr      Address
	PrefixLen int
}

func ParseIP(s string) (Address, error) {
	if ip := net.ParseIP(s); ip != nil {
		return FromIP4(ip), nil
	}
	return 0, &net.ParseError{Type: "IP Address", Text: s}
}

func ParseCIDR(s string) (Address, CIDR, error) {
	if ip, ipnet, err := net.ParseCIDR(s); err != nil {
		return 0, CIDR{}, err
	} else if ipnet.IP.To4() == nil {
		return 0, CIDR{}, &net.ParseError{Type: "Non-IPv4 address not supported", Text: s}
	} else {
		prefixLen, _ := ipnet.Mask.Size()
		return FromIP4(ip), CIDR{Addr: FromIP4(ipnet.IP), PrefixLen: prefixLen}, nil
	}
}

func (cidr CIDR) Size() Offset { return 1 << uint(32-cidr.PrefixLen) }

func (cidr CIDR) Range() Range {
	return NewRange(cidr.Addr, cidr.Size())
}
func (cidr CIDR) HostRange() Range {
	// Respect RFC1122 exclusions of first and last addresses
	return NewRange(cidr.Addr+1, cidr.Size()-2)
}

func (cidr CIDR) String() string {
	return fmt.Sprintf("%s/%d", cidr.Addr.String(), cidr.PrefixLen)
}

// FromIP4 converts an ipv4 address to our integer address type
func FromIP4(ip4 net.IP) (r Address) {
	for _, b := range ip4.To4() {
		r <<= 8
		r |= Address(b)
	}
	return
}

// IP4 converts our integer address type to an ipv4 address
func (addr Address) IP4() (r net.IP) {
	r = make([]byte, net.IPv4len)
	for i := 3; i >= 0; i-- {
		r[i] = byte(addr)
		addr >>= 8
	}
	return
}

func (addr Address) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", addr.String())), nil
}

func (addr Address) String() string {
	return addr.IP4().String()
}

func Add(addr Address, i Offset) Address {
	return addr + Address(i)
}

func Subtract(a, b Address) Offset {
	common.Assert(a >= b)
	return Offset(a - b)
}

func Min(a, b Offset) Offset {
	if a > b {
		return b
	}
	return a
}

func (addr Address) Reverse() Address {
	return ((addr >> 24) & 0xff) | // move byte 3 to byte 0
		((addr << 8) & 0xff0000) | // move byte 1 to byte 2
		((addr >> 8) & 0xff00) | // move byte 2 to byte 1
		((addr << 24) & 0xff000000) // byte 0 to byte 3
}
