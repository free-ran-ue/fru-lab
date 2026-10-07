package xdp

import (
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/vishvananda/netlink"
)

// NextHop is where Ethernet frames for an address go: out of Ifindex,
// from Src to Dst.
type NextHop struct {
	Ifindex int
	Src     net.HardwareAddr
	Dst     net.HardwareAddr
	Via     netip.Addr // the gateway, or the address itself when it is on-link
}

// usable is every neighbour state whose MAC can be used.
const usable = netlink.NUD_REACHABLE | netlink.NUD_STALE | netlink.NUD_DELAY | netlink.NUD_PROBE | netlink.NUD_PERMANENT | netlink.NUD_NOARP

// Resolve finds the next hop for dst the way the kernel would route it,
// and its MAC from the neighbour table. When the table has no entry yet
// it sends dst one UDP packet through the kernel, which makes the kernel
// ARP for it, and waits up to timeout.
func Resolve(dst netip.Addr, timeout time.Duration) (NextHop, error) {
	routes, err := netlink.RouteGet(net.IP(dst.AsSlice()))
	if err != nil || len(routes) == 0 {
		return NextHop{}, fmt.Errorf("no route to %s: %v", dst, err)
	}
	r := routes[0]
	hop := NextHop{Ifindex: r.LinkIndex, Via: dst}
	if gw, ok := netip.AddrFromSlice(r.Gw.To4()); ok && r.Gw != nil {
		hop.Via = gw
	}
	l, err := netlink.LinkByIndex(r.LinkIndex)
	if err != nil {
		return NextHop{}, err
	}
	hop.Src = l.Attrs().HardwareAddr
	deadline := time.Now().Add(timeout)
	poked := false
	for {
		neighs, err := netlink.NeighList(r.LinkIndex, netlink.FAMILY_V4)
		if err != nil {
			return NextHop{}, fmt.Errorf("neighbour table: %w", err)
		}
		for _, n := range neighs {
			if n.IP.Equal(net.IP(hop.Via.AsSlice())) && len(n.HardwareAddr) == 6 && n.State&usable != 0 {
				hop.Dst = n.HardwareAddr
				return hop, nil
			}
		}
		if time.Now().After(deadline) {
			return NextHop{}, fmt.Errorf("no MAC for %s on %s: it does not answer ARP", hop.Via, l.Attrs().Name)
		}
		if !poked {
			poke(dst)
			poked = true
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// poke sends dst a single UDP packet through the kernel so it resolves
// the next hop's MAC.
func poke(dst netip.Addr) {
	c, err := net.DialUDP("udp4", nil, net.UDPAddrFromAddrPort(netip.AddrPortFrom(dst, 9)))
	if err != nil {
		return
	}
	_, _ = c.Write([]byte("fru-tester"))
	_ = c.Close()
}
