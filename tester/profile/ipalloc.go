package profile

import (
	"fmt"
	"net/netip"
)

// AllocateIPs hands out count addresses from cidr, beginning at start and
// walking upward. It skips the network and broadcast addresses (for
// prefixes shorter than /31) and anything in exclude, which callers fill
// with the core-side IP and the IPs already configured on the host.
func AllocateIPs(cidr, start string, count int, exclude []netip.Addr) ([]netip.Addr, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil || !prefix.Addr().Is4() {
		return nil, fmt.Errorf("%q is not an IPv4 CIDR", cidr)
	}
	prefix = prefix.Masked()
	first, err := netip.ParseAddr(start)
	if err != nil || !first.Is4() {
		return nil, fmt.Errorf("%q is not an IPv4 address", start)
	}
	if !prefix.Contains(first) {
		return nil, fmt.Errorf("start IP %s is outside %s", first, prefix)
	}

	network, broadcast := prefix.Addr(), lastAddr(prefix)
	skipEdges := prefix.Bits() < 31
	excluded := make(map[netip.Addr]bool, len(exclude))
	for _, a := range exclude {
		excluded[a] = true
	}

	// Cap the up-front capacity at the CIDR size: a mistyped count must
	// end in the shortfall error below, not a huge allocation.
	out := make([]netip.Addr, 0, min(count, 1<<min(32-prefix.Bits(), 24)))
	for a := first; prefix.Contains(a) && len(out) < count; a = a.Next() {
		if skipEdges && (a == network || a == broadcast) {
			continue
		}
		if excluded[a] {
			continue
		}
		out = append(out, a)
		if a == broadcast {
			break
		}
	}
	if len(out) < count {
		return nil, fmt.Errorf("%s from %s has only %d usable IPs, need %d (short by %d)",
			prefix, first, len(out), count, count-len(out))
	}
	return out, nil
}

func lastAddr(p netip.Prefix) netip.Addr {
	b := p.Addr().As4()
	hostBits := 32 - p.Bits()
	v := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	v |= (1 << hostBits) - 1
	return netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
}
