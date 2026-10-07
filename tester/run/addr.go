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
	added    netip.Prefix // what was added; invalid if the host already had it as wanted
	replaced netip.Prefix // the host's own address with another prefix, removed for now; invalid if none
}

// placeAddr makes sure want (an address with its prefix length) is on
// iface:
//   - the host lacks the IP: it is added;
//   - the host has the IP with the same prefix length: it is used as it
//     is, and undo leaves it there;
//   - iface has the IP with another prefix length: that one is removed and
//     want added, and undo puts the original back;
//   - another interface has the IP with another prefix length: an error,
//     since taking it away could cut that interface off.
func placeAddr(m netcfg.AddrManager, iface string, want netip.Prefix) (addrChange, error) {
	ch := addrChange{iface: iface}
	onIface, have, found, err := m.FindAddr(want.Addr())
	switch {
	case err != nil:
		return ch, err
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
	if err := m.Add(iface, want); err != nil {
		if ch.replaced.IsValid() {
			err = errors.Join(err, m.Add(iface, ch.replaced)) // put the host's address back
		}
		return addrChange{iface: iface}, err
	}
	ch.added = want
	return ch, nil
}

// undo removes what placeAddr added and puts back what it replaced.
func (ch addrChange) undo(m netcfg.AddrManager) error {
	var errs []error
	if ch.added.IsValid() {
		errs = append(errs, m.Remove(ch.iface, ch.added))
	}
	if ch.replaced.IsValid() {
		if err := m.Add(ch.iface, ch.replaced); err != nil {
			errs = append(errs, fmt.Errorf("put %s back on %s: %w", ch.replaced, ch.iface, err))
		}
	}
	return errors.Join(errs...)
}
