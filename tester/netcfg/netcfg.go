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

func toNetlink(p netip.Prefix) *netlink.Addr {
	ip := p.Addr().As4()
	return &netlink.Addr{IPNet: &net.IPNet{
		IP:   net.IP(ip[:]),
		Mask: net.CIDRMask(p.Bits(), 32),
	}}
}
