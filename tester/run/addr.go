package run

import (
	"errors"
	"fmt"
	"net/netip"

	"tester/netcfg"
)

// addrChange is what placeAddr did to the host, so it can be undone.
type addrChange struct {
	iface    string
	added    netip.Prefix // what to take off again; invalid if the host already had it as wanted
	replaced netip.Prefix // the host's own address with another prefix, removed for now; invalid if none
	// release drops this tester's claim on added; nil if added is not
	// claimed (an address no other tester can use, such as a gNB's)
	release func()
}

// addrKey names an address on an interface for netcfg's claims.
func addrKey(iface string, p netip.Prefix) string { return "addr/" + iface + "/" + p.String() }

// replacedNote is how a claim tells the next claimant which host address
// the first one replaced, so whoever lets go last puts it back.
func replacedNote(p netip.Prefix) string {
	if !p.IsValid() {
		return "-"
	}
	return p.String()
}

// placeAddr makes sure want (an address with its prefix length) is on
// iface. Other testers on this host may place the same address (a shared
// N6 sink), so the address is claimed while in use:
//   - the host lacks the IP: it is added;
//   - another tester added the IP as wanted: it is shared, and only the
//     last tester to undo takes it off;
//   - the host has the IP with the same prefix length on its own: it is
//     used as it is, and undo leaves it there;
//   - iface has the IP with another prefix length and no tester uses it:
//     that one is removed and want added, and undo puts the original back;
//   - another interface has the IP with another prefix length, or another
//     tester uses it with another one: an error, since taking it away
//     could cut that interface or tester off.
func placeAddr(m netcfg.AddrManager, iface string, want netip.Prefix) (addrChange, error) {
	unlock, err := m.Lock()
	if err != nil {
		return addrChange{iface: iface}, err
	}
	defer unlock()
	ch := addrChange{iface: iface}
	onIface, have, found, err := m.FindAddr(want.Addr())
	if err != nil {
		return ch, err
	}
	if found && onIface == iface {
		claims, err := m.Claims(addrKey(iface, have))
		if err != nil {
			return ch, err
		}
		switch {
		case len(claims) > 0 && have.Bits() == want.Bits():
			// another tester's: share it, and put back what it replaced
			// if this one lets go last
			replaced, _ := netip.ParsePrefix(claims[0])
			ch.release, err = m.Claim(addrKey(iface, want), claims[0])
			if err != nil {
				return addrChange{iface: iface}, err
			}
			ch.added, ch.replaced = want, replaced
			return ch, nil
		case len(claims) > 0:
			return ch, fmt.Errorf("%s is in use by another tester on %s as %s; use the same prefix length or another address", want.Addr(), iface, have)
		}
	}
	switch {
	case !found:
	case have.Bits() == want.Bits():
		return ch, nil
	case onIface != iface:
		return ch, fmt.Errorf("%s is already on %s as %s; remove it there or use another address", want.Addr(), onIface, have)
	default:
		if err := m.Remove(iface, have); err != nil {
			return ch, fmt.Errorf("replace %s on %s with %s: %w", have, iface, want, err)
		}
		ch.replaced = have
	}
	putBack := func(err error) error {
		if ch.replaced.IsValid() {
			err = errors.Join(err, m.Add(iface, ch.replaced)) // put the host's address back
		}
		return err
	}
	if err := m.Add(iface, want); err != nil {
		return addrChange{iface: iface}, putBack(err)
	}
	release, err := m.Claim(addrKey(iface, want), replacedNote(ch.replaced))
	if err != nil {
		return addrChange{iface: iface}, putBack(errors.Join(err, m.Remove(iface, want)))
	}
	ch.added, ch.release = want, release
	return ch, nil
}

// undo takes off what placeAddr added and puts back what it replaced,
// unless another tester still uses the address.
func (ch addrChange) undo(m netcfg.AddrManager) error {
	if !ch.added.IsValid() {
		return nil
	}
	if ch.release != nil {
		unlock, err := m.Lock()
		if err != nil {
			ch.release()
			return err
		}
		defer unlock()
		ch.release()
		others, err := m.Claims(addrKey(ch.iface, ch.added))
		if err != nil {
			return fmt.Errorf("leave %s on %s: %w", ch.added, ch.iface, err)
		}
		if len(others) > 0 {
			return nil // another tester still uses it; the last one takes it off
		}
	}
	var errs []error
	errs = append(errs, m.Remove(ch.iface, ch.added))
	if ch.replaced.IsValid() {
		if err := m.Add(ch.iface, ch.replaced); err != nil {
			errs = append(errs, fmt.Errorf("put %s back on %s: %w", ch.replaced, ch.iface, err))
		}
	}
	return errors.Join(errs...)
}

// routeChange is a route a run uses and claimed.
type routeChange struct {
	iface   string
	dst     netip.Prefix
	gw      netip.Addr
	release func()
}

func routeKey(iface string, dst netip.Prefix, gw netip.Addr) string {
	return "route/" + iface + "/" + dst.String() + "/" + gw.String()
}

// placeRoute makes sure dst is routed via gw on iface. A route another
// tester added is shared and taken off by the last tester to undo; a
// route the host had on its own is used and left there (ok false).
func placeRoute(m netcfg.AddrManager, iface string, dst netip.Prefix, gw netip.Addr) (rc routeChange, ok bool, err error) {
	unlock, err := m.Lock()
	if err != nil {
		return rc, false, err
	}
	defer unlock()
	key := routeKey(iface, dst, gw)
	claims, err := m.Claims(key)
	if err != nil {
		return rc, false, err
	}
	added, err := m.EnsureRoute(iface, dst, gw)
	if err != nil {
		return rc, false, err
	}
	if !added && len(claims) == 0 {
		return rc, false, nil // the host's own route
	}
	release, err := m.Claim(key, "-")
	if err != nil {
		if added {
			err = errors.Join(err, m.RemoveRoute(iface, dst, gw))
		}
		return rc, false, err
	}
	return routeChange{iface: iface, dst: dst, gw: gw, release: release}, true, nil
}

// undo removes the route unless another tester still uses it.
func (rc routeChange) undo(m netcfg.AddrManager) error {
	unlock, err := m.Lock()
	if err != nil {
		rc.release()
		return err
	}
	defer unlock()
	rc.release()
	others, err := m.Claims(routeKey(rc.iface, rc.dst, rc.gw))
	if err != nil {
		return fmt.Errorf("leave route %s via %s: %w", rc.dst, rc.gw, err)
	}
	if len(others) > 0 {
		return nil
	}
	return m.RemoveRoute(rc.iface, rc.dst, rc.gw)
}

// ensurePromote turns on promote_secondaries on each interface once, so
// removing this run's addresses never takes another tester's with it.
func ensurePromote(m netcfg.AddrManager, log func(string, ...any), ifaces ...string) error {
	seen := map[string]bool{}
	for _, iface := range ifaces {
		if iface == "" || seen[iface] {
			continue
		}
		seen[iface] = true
		changed, err := m.EnsurePromoteSecondaries(iface)
		if err != nil {
			return fmt.Errorf("%w (needed so that removing this run's addresses keeps every other address in their subnets)", err)
		}
		if changed {
			log("turned on net.ipv4.conf.%s.promote_secondaries, so removing a run's addresses keeps every other address in their subnets", iface)
		}
	}
	return nil
}
