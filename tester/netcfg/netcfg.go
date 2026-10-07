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
	// FindAddr reports which interface has ip, and with which prefix
	// length; ok is false when no interface has it.
	FindAddr(ip netip.Addr) (iface string, prefix netip.Prefix, ok bool, err error)
	// Interfaces lists the names of every link on the host.
	Interfaces() ([]string, error)
	// InterfaceDetails describes every link on the host, for the Setup
	// page's interface menus.
	InterfaceDetails() ([]InterfaceInfo, error)
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
	// MTU is iface's MTU.
	MTU(iface string) (int, error)
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

func (Netlink) FindAddr(ip netip.Addr) (string, netip.Prefix, bool, error) {
	list, err := netlink.AddrList(nil, netlink.FAMILY_V4)
	if err != nil {
		return "", netip.Prefix{}, false, fmt.Errorf("list host addresses: %w", err)
	}
	for _, a := range list {
		got, ok := netip.AddrFromSlice(a.IP.To4())
		if !ok || got != ip || a.IPNet == nil {
			continue
		}
		ones, _ := a.IPNet.Mask.Size()
		name := ""
		if link, err := netlink.LinkByIndex(a.LinkIndex); err == nil {
			name = link.Attrs().Name
		}
		return name, netip.PrefixFrom(ip, ones), true, nil
	}
	return "", netip.Prefix{}, false, nil
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

// InterfaceInfo is one link on the host.
type InterfaceInfo struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`  // "device" (a NIC), "bridge", "veth", "macvlan", "vlan", "bond", …
	State     string   `json:"state"` // operational state: "up", "down", "unknown", …
	Mtu       int      `json:"mtu"`
	Addresses []string `json:"addresses"` // IPv4, with prefix length
}

func (Netlink) InterfaceDetails() ([]InterfaceInfo, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("list host interfaces: %w", err)
	}
	out := make([]InterfaceInfo, 0, len(links))
	for _, l := range links {
		a := l.Attrs()
		info := InterfaceInfo{Name: a.Name, Kind: l.Type(), State: a.OperState.String(), Mtu: a.MTU, Addresses: []string{}}
		if addrs, err := netlink.AddrList(l, netlink.FAMILY_V4); err == nil {
			for _, ad := range addrs {
				if ad.IPNet != nil {
					info.Addresses = append(info.Addresses, ad.IPNet.String())
				}
			}
		}
		out = append(out, info)
	}
	return out, nil
}

func (Netlink) MTU(iface string) (int, error) {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return 0, fmt.Errorf("interface %q: %w", iface, err)
	}
	return link.Attrs().MTU, nil
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
