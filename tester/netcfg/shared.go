package netcfg

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// Several fru-testers can run on one host and share its interfaces. What
// one of them adds there that another may also use (the N6 sink, the UE
// pool route) is claimed while in use, and only the last tester to let go
// takes it off. A claim is an abstract unix socket: abstract names live in
// the network namespace, so every tester on the host network sees them,
// containers included, and a tester that dies drops its claims with it.

const (
	claimPrefix = "fru-tester/claim/"
	lockName    = "fru-tester/lock"
	// lockWait bounds the wait for another tester's address change.
	lockWait = 30 * time.Second
)

// Lock serialises address changes between the testers on this host.
func (Netlink) Lock() (unlock func(), err error) {
	deadline := time.Now().Add(lockWait)
	for {
		c, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: "@" + lockName, Net: "unixgram"})
		if err == nil {
			return func() { _ = c.Close() }, nil
		}
		if !errors.Is(err, syscall.EADDRINUSE) {
			return nil, fmt.Errorf("lock the host's addresses: %w", err)
		}
		if time.Now().After(deadline) {
			return nil, errors.New("lock the host's addresses: another tester has held the lock for 30 s")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Claim marks key as in use by this tester until release; note is what
// the other claimants see (Claims).
func (Netlink) Claim(key, note string) (release func(), err error) {
	if strings.ContainsAny(key+note, "# \n") {
		return nil, fmt.Errorf("claim %q: no spaces or #", key)
	}
	var nonce [6]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	name := "@" + claimPrefix + key + "#" + hex.EncodeToString(nonce[:]) + "#" + note
	if len(name) > 107 {
		return nil, fmt.Errorf("claim %q: name too long", key)
	}
	c, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: name, Net: "unixgram"})
	if err != nil {
		return nil, fmt.Errorf("claim %s: %w", key, err)
	}
	return func() { _ = c.Close() }, nil
}

// Claims lists the notes of every claim on key held on this host.
func (Netlink) Claims(key string) ([]string, error) {
	f, err := os.Open("/proc/net/unix")
	if err != nil {
		return nil, fmt.Errorf("list claims: %w", err)
	}
	defer func() { _ = f.Close() }()
	return parseClaims(f, key)
}

func parseClaims(f interface{ Read([]byte) (int, error) }, key string) ([]string, error) {
	want := "@" + claimPrefix + key + "#"
	var notes []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 8 || !strings.HasPrefix(fields[7], want) {
			continue
		}
		rest := strings.SplitN(strings.TrimPrefix(fields[7], want), "#", 2)
		if len(rest) == 2 {
			notes = append(notes, rest[1])
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("list claims: %w", err)
	}
	return notes, nil
}

// ipv4DevconfPromoteSecondaries is IPV4_DEVCONF_PROMOTE_SECONDARIES from
// linux/ip.h.
const ipv4DevconfPromoteSecondaries = 20

// EnsurePromoteSecondaries turns on net.ipv4.conf.<iface>.promote_secondaries
// unless it (or "all") is already on. Off, the kernel deletes every
// address of a subnet when its first (primary) address is removed, so one
// tester stopping would take every other tester's addresses in that subnet
// with it; on, another address becomes the primary instead. It is set over
// netlink, which works where /proc/sys is read-only (a container), and is
// left on.
func (Netlink) EnsurePromoteSecondaries(iface string) (changed bool, err error) {
	if promoteOn(iface) {
		return false, nil
	}
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return false, fmt.Errorf("interface %q: %w", iface, err)
	}
	req := nl.NewNetlinkRequest(unix.RTM_SETLINK, unix.NLM_F_ACK)
	msg := nl.NewIfInfomsg(unix.AF_UNSPEC)
	msg.Index = int32(link.Attrs().Index)
	req.AddData(msg)
	spec := nl.NewRtAttr(unix.IFLA_AF_SPEC, nil)
	inet := spec.AddRtAttr(unix.AF_INET, nil)
	conf := inet.AddRtAttr(unix.IFLA_INET_CONF, nil)
	conf.AddRtAttr(ipv4DevconfPromoteSecondaries, nl.Uint32Attr(1))
	req.AddData(spec)
	if _, err := req.Execute(unix.NETLINK_ROUTE, 0); err != nil {
		return false, fmt.Errorf("turn on net.ipv4.conf.%s.promote_secondaries: %w", iface, err)
	}
	if !promoteOn(iface) {
		return false, fmt.Errorf("net.ipv4.conf.%s.promote_secondaries is still off", iface)
	}
	return true, nil
}

// promoteOn reads the setting; the kernel uses it when either the
// interface's or "all" is on.
func promoteOn(iface string) bool {
	on := func(name string) bool {
		b, err := os.ReadFile("/proc/sys/net/ipv4/conf/" + name + "/promote_secondaries")
		return err == nil && strings.TrimSpace(string(b)) == "1"
	}
	return on("all") || on(iface)
}
