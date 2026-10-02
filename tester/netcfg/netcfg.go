// Package netcfg adds and removes the per-gNB IPs on host interfaces.
package netcfg

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"

	"github.com/vishvananda/netlink"
)

// AddrManager is what a run needs from the host network stack. The run
// package depends on this interface so tests can use a fake.
type AddrManager interface {
	// HostIPv4s lists every IPv4 address already on any interface.
	HostIPv4s() ([]netip.Addr, error)
	// Interfaces lists the names of every link on the host.
	Interfaces() ([]string, error)
	Add(iface string, addr netip.Prefix) error
	// Remove must treat an already-missing address as success, because the
	// kernel drops secondary addresses when their primary is removed.
	Remove(iface string, addr netip.Prefix) error
	// EnsureRoute makes sure dst is routed via gw on iface. It reports
	// whether it added the route; an identical existing route is left
	// alone (added=false), a different existing route is an error.
	EnsureRoute(iface string, dst netip.Prefix, gw netip.Addr) (added bool, err error)
	// RemoveRoute deletes a route EnsureRoute added; a missing route is
	// not an error.
	RemoveRoute(iface string, dst netip.Prefix, gw netip.Addr) error
}

// Netlink is the real AddrManager; it needs CAP_NET_ADMIN.
type Netlink struct{}

func (Netlink) HostIPv4s() ([]netip.Addr, error) {
	list, err := netlink.AddrList(nil, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("list host addresses: %w", err)
	}
	out := make([]netip.Addr, 0, len(list))
	for _, a := range list {
		if ip, ok := netip.AddrFromSlice(a.IP.To4()); ok {
			out = append(out, ip)
		}
	}
	return out, nil
}

func (Netlink) Interfaces() ([]string, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("list host interfaces: %w", err)
	}
	names := make([]string, 0, len(links))
	for _, l := range links {
		names = append(names, l.Attrs().Name)
	}
	return names, nil
}

func (Netlink) Add(iface string, addr netip.Prefix) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("interface %q: %w", iface, err)
	}
	if err := netlink.AddrAdd(link, toNetlink(addr)); err != nil {
		return fmt.Errorf("add %s to %s: %w", addr, iface, err)
	}
	return nil
}

func (Netlink) Remove(iface string, addr netip.Prefix) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("interface %q: %w", iface, err)
	}
	if err := netlink.AddrDel(link, toNetlink(addr)); err != nil {
		if errors.Is(err, syscall.EADDRNOTAVAIL) {
			return nil
		}
		return fmt.Errorf("remove %s from %s: %w", addr, iface, err)
	}
	return nil
}

func (Netlink) EnsureRoute(iface string, dst netip.Prefix, gw netip.Addr) (bool, error) {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return false, fmt.Errorf("interface %q: %w", iface, err)
	}
	existing, err := netlink.RouteListFiltered(netlink.FAMILY_V4,
		&netlink.Route{Dst: ipNet(dst)}, netlink.RT_FILTER_DST)
	if err != nil {
		return false, fmt.Errorf("list routes to %s: %w", dst, err)
	}
	if len(existing) > 0 {
		r := existing[0]
		if r.Gw.Equal(net.IP(gw.AsSlice())) && r.LinkIndex == link.Attrs().Index {
			return false, nil
		}
		return false, fmt.Errorf("%s is already routed via %s; remove that route or change the UE pool", dst, r.Gw)
	}
	if err := netlink.RouteAdd(&netlink.Route{LinkIndex: link.Attrs().Index, Dst: ipNet(dst), Gw: net.IP(gw.AsSlice())}); err != nil {
		return false, fmt.Errorf("route %s via %s dev %s: %w", dst, gw, iface, err)
	}
	return true, nil
}

func (Netlink) RemoveRoute(iface string, dst netip.Prefix, gw netip.Addr) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("interface %q: %w", iface, err)
	}
	err = netlink.RouteDel(&netlink.Route{LinkIndex: link.Attrs().Index, Dst: ipNet(dst), Gw: net.IP(gw.AsSlice())})
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("remove route %s via %s: %w", dst, gw, err)
	}
	return nil
}

func ipNet(p netip.Prefix) *net.IPNet {
	p = p.Masked()
	ip := p.Addr().As4()
	return &net.IPNet{IP: net.IP(ip[:]), Mask: net.CIDRMask(p.Bits(), 32)}
}

func toNetlink(p netip.Prefix) *netlink.Addr {
	ip := p.Addr().As4()
	return &netlink.Addr{IPNet: &net.IPNet{
		IP:   net.IP(ip[:]),
		Mask: net.CIDRMask(p.Bits(), 32),
	}}
}
