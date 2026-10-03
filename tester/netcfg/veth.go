package netcfg

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"syscall"
	"unsafe"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// VethTuning is what TuneVethGRO changed. Undo puts it back.
type VethTuning struct {
	Links []string // what was changed, for the Run page
	Undo  func() error
}

// TuneVethGRO lets the kernel merge what a container on a veth sends to
// this host into GRO batches, so the tester's sockets (which set UDP_GRO)
// read many packets per message. A veth only does that when its host side
// has GRO on and its container side has TSO off: a forwarded packet (the
// UPF's output) goes through the host side's GRO only if the sending side
// cannot do TSO. TSO only concerns TCP, so the container's GTP-U path is
// unchanged.
//
// iface is a bridge (its veth ports are looked at) or a veth. Only ports
// whose container-side peer has one of ips (the UPF's N3 and N6 IPs) are
// changed. Anything that is not a veth is left alone: a physical NIC does
// GRO by itself.
func (Netlink) TuneVethGRO(iface string, ips []netip.Addr) (VethTuning, error) {
	t := VethTuning{Undo: func() error { return nil }}
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return t, fmt.Errorf("interface %q: %w", iface, err)
	}
	var ports []*netlink.Veth
	switch l := link.(type) {
	case *netlink.Veth:
		ports = []*netlink.Veth{l}
	case *netlink.Bridge:
		all, err := netlink.LinkList()
		if err != nil {
			return t, fmt.Errorf("list interfaces: %w", err)
		}
		for _, p := range all {
			if v, ok := p.(*netlink.Veth); ok && p.Attrs().MasterIndex == l.Attrs().Index {
				ports = append(ports, v)
			}
		}
	default:
		return t, nil
	}
	if len(ports) == 0 {
		return t, nil
	}
	namespaces, err := netnsByID()
	if err != nil {
		return t, err
	}
	defer func() {
		for _, h := range namespaces {
			_ = h.Close()
		}
	}()

	var undo []func() error
	unseen := 0
	t.Undo = func() error {
		var errs []error
		for i := len(undo) - 1; i >= 0; i-- {
			errs = append(errs, undo[i]())
		}
		return errors.Join(errs...)
	}
	for _, port := range ports {
		ns, ok := namespaces[port.Attrs().NetNsID]
		if !ok {
			unseen++ // its peer is in a namespace no process here holds
			continue
		}
		peerName, match, err := peerWithIP(ns, port, ips)
		if err != nil || !match {
			continue
		}
		hostOn, err := setFeature(-1, port.Attrs().Name, ethtoolGGRO, ethtoolSGRO, true)
		if err != nil {
			return t, fmt.Errorf("turn GRO on for %s: %w", port.Attrs().Name, err)
		}
		if hostOn {
			name := port.Attrs().Name
			undo = append(undo, func() error { _, err := setFeature(-1, name, ethtoolGGRO, ethtoolSGRO, false); return err })
		}
		fd, err := socketIn(ns)
		if err != nil {
			return t, err
		}
		peerOff, err := setFeature(fd, peerName, ethtoolGTSO, ethtoolSTSO, false)
		if err != nil {
			_ = unix.Close(fd)
			return t, fmt.Errorf("turn TSO off for %s's peer: %w", port.Attrs().Name, err)
		}
		if peerOff {
			undo = append(undo, func() error {
				defer func() { _ = unix.Close(fd) }()
				_, err := setFeature(fd, peerName, ethtoolGTSO, ethtoolSTSO, true)
				return err
			})
		} else {
			_ = unix.Close(fd)
		}
		t.Links = append(t.Links, fmt.Sprintf("%s (GRO on) and %s in its container (TSO off)", port.Attrs().Name, peerName))
	}
	if len(t.Links) == 0 && unseen > 0 {
		return t, fmt.Errorf("%s has %d veth ports whose containers this process cannot see; run fru-tester on the host, or in Docker with pid: host and SYS_ADMIN", iface, unseen)
	}
	return t, nil
}

// peerWithIP returns the name of port's container-side peer, and whether
// it has one of ips.
func peerWithIP(ns netns.NsHandle, port *netlink.Veth, ips []netip.Addr) (string, bool, error) {
	idx, err := netlink.VethPeerIndex(port)
	if err != nil {
		return "", false, err
	}
	h, err := netlink.NewHandleAt(ns)
	if err != nil {
		return "", false, err
	}
	defer h.Close()
	peer, err := h.LinkByIndex(idx)
	if err != nil {
		return "", false, err
	}
	addrs, err := h.AddrList(peer, netlink.FAMILY_V4)
	if err != nil {
		return "", false, err
	}
	for _, a := range addrs {
		if ip, ok := netip.AddrFromSlice(a.IP.To4()); ok && slices.Contains(ips, ip) {
			return peer.Attrs().Name, true, nil
		}
	}
	return peer.Attrs().Name, false, nil
}

// netnsByID opens every network namespace a process here or a named
// namespace (ip netns, Docker's) holds, keyed by its ID as this
// namespace sees it, which is what a veth reports for its peer.
func netnsByID() (map[int]netns.NsHandle, error) {
	paths, _ := filepath.Glob("/proc/[0-9]*/ns/net")
	for _, dir := range []string{"/run/netns", "/var/run/docker/netns", "/run/docker/netns"} {
		more, _ := filepath.Glob(filepath.Join(dir, "*"))
		paths = append(paths, more...)
	}
	seen := map[uint64]bool{}
	out := map[int]netns.NsHandle{}
	for _, p := range paths {
		var st syscall.Stat_t
		if err := syscall.Stat(p, &st); err != nil || seen[st.Ino] {
			continue
		}
		seen[st.Ino] = true
		h, err := netns.GetFromPath(p)
		if err != nil {
			continue
		}
		id, err := netlink.GetNetNsIdByFd(int(h))
		if err != nil || id < 0 {
			_ = h.Close()
			continue
		}
		if _, dup := out[id]; dup {
			_ = h.Close()
			continue
		}
		out[id] = h
	}
	if len(out) == 0 {
		if _, err := os.Stat("/proc/1/ns/net"); err != nil {
			return nil, fmt.Errorf("no network namespaces visible: %w", err)
		}
	}
	return out, nil
}

// socketIn opens an AF_INET socket inside ns, for ethtool calls on its
// interfaces.
func socketIn(ns netns.NsHandle) (int, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	orig, err := netns.Get()
	if err != nil {
		return -1, err
	}
	defer func() { _ = orig.Close() }()
	if err := netns.Set(ns); err != nil {
		return -1, err
	}
	fd, serr := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err := netns.Set(orig); err != nil {
		panic(fmt.Sprintf("cannot return to the original network namespace: %v", err)) // this thread is unusable
	}
	return fd, serr
}

// Legacy ethtool commands for one feature flag (include/uapi/linux/ethtool.h).
const (
	ethtoolGTSO = 0x1e
	ethtoolSTSO = 0x1f
	ethtoolGGRO = 0x2b
	ethtoolSGRO = 0x2c
)

type ethtoolValue struct {
	cmd, data uint32
}

type ifreqEthtool struct {
	name [unix.IFNAMSIZ]byte
	data unsafe.Pointer
	_    [16]byte
}

func ethtoolValueCall(fd int, ifname string, cmd, data uint32) (uint32, error) {
	ev := ethtoolValue{cmd: cmd, data: data}
	var ifr ifreqEthtool
	copy(ifr.name[:], ifname)
	ifr.data = unsafe.Pointer(&ev)
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.SIOCETHTOOL, uintptr(unsafe.Pointer(&ifr))); errno != 0 {
		return 0, errno
	}
	return ev.data, nil
}

// setFeature sets a feature of ifname to on through fd (a socket in its
// namespace; -1 opens one here) and reports whether it changed anything.
func setFeature(fd int, ifname string, get, set uint32, on bool) (bool, error) {
	if fd < 0 {
		s, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
		if err != nil {
			return false, err
		}
		defer func() { _ = unix.Close(s) }()
		fd = s
	}
	cur, err := ethtoolValueCall(fd, ifname, get, 0)
	if err != nil {
		return false, err
	}
	want := uint32(0)
	if on {
		want = 1
	}
	if (cur != 0) == on {
		return false, nil
	}
	_, err = ethtoolValueCall(fd, ifname, set, want)
	return err == nil, err
}
