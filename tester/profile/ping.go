package profile

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

// Networks the Setup page's ping test can check.
const (
	PlaneN2 = "n2"
	PlaneN3 = "n3"
	PlaneN6 = "n6"
)

var ErrUnknownPlane = errors.New(`plane must be "n2", "n3" or "n6"`)

// PingTarget is what a ping test of one network uses: the address a run
// would put on Interface for that network, and the core address to ping.
type PingTarget struct {
	Interface string
	Source    netip.Prefix // with its prefix length, as it goes on Interface
	Peer      netip.Addr
}

// PingTargetFor checks only plane's own fields, so a half-filled profile
// can still test one network, and returns its PingTarget. N2 and N3 use
// the first gNB IP of their CIDR (skipping the core's IPs and hostIPs, as
// Expand does) and ping the AMF and the UPF's N3; N6 uses the sink and
// pings the UPF's N6.
func PingTargetFor(p Profile, plane string, hostIPs []netip.Addr) (PingTarget, error) {
	verr := &ValidationError{}
	nw := p.Network
	switch plane {
	case PlaneN2, PlaneN3:
		base, iface, cidr, start, peerField, peer, portField, port := "network.n2", nw.N2.Interface, nw.N2.Cidr, nw.N2.StartIP, "amfIp", nw.N2.AmfIP, "amfPort", nw.N2.AmfPort
		if plane == PlaneN3 {
			base, iface, cidr, start, peerField, peer, portField, port = "network.n3", nw.N3.Interface, nw.N3.Cidr, nw.N3.StartIP, "upfIp", nw.N3.UpfIP, "upfPort", nw.N3.UpfPort
		}
		validateEndpoint(verr, base, iface, cidr, start, peerField, peer, portField, port)
		if len(verr.Errors) > 0 {
			return PingTarget{}, verr
		}
		ips, err := AllocateIPs(cidr, start, 1, append(coreIPs(nw), hostIPs...))
		if err != nil {
			verr.add(base+".cidr", err.Error())
			return PingTarget{}, verr
		}
		return PingTarget{Interface: iface, Source: netip.PrefixFrom(ips[0], netip.MustParsePrefix(cidr).Bits()),
			Peer: netip.MustParseAddr(peer)}, nil
	case PlaneN6:
		if strings.TrimSpace(nw.N6.Interface) == "" {
			verr.add("network.n6.interface", "must not be empty")
		}
		validateSink(verr, nw.N6.SinkIP)
		peer, err := netip.ParseAddr(nw.N6.UpfIP)
		if err != nil || !peer.Is4() {
			verr.add("network.n6.upfIp", fmt.Sprintf("%q is not an IPv4 address", nw.N6.UpfIP))
		}
		if len(verr.Errors) > 0 {
			return PingTarget{}, verr
		}
		return PingTarget{Interface: nw.N6.Interface, Source: nw.N6.Sink(), Peer: peer}, nil
	default:
		return PingTarget{}, ErrUnknownPlane
	}
}

// coreIPs are the core's and the sink's addresses that parse; a gNB never
// gets one of them.
func coreIPs(nw Network) []netip.Addr {
	var out []netip.Addr
	for _, s := range []string{nw.N2.AmfIP, nw.N3.UpfIP, nw.N6.UpfIP} {
		if a, err := netip.ParseAddr(s); err == nil {
			out = append(out, a)
		}
	}
	if p, err := netip.ParsePrefix(nw.N6.SinkIP); err == nil {
		out = append(out, p.Addr())
	}
	return out
}
