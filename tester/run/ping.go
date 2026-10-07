package run

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"time"

	"tester/netcfg"
	"tester/profile"
)

var ErrPingActive = errors.New("a ping test is running; try again when it has finished")

// pingCount echo requests are sent, each waiting up to pingTimeout.
const (
	pingCount   = 3
	pingTimeout = time.Second
)

// PingResult is one network's ping test, for the Setup page.
type PingResult struct {
	Plane     string    `json:"plane"`
	Interface string    `json:"interface"`
	Source    string    `json:"source"` // the address used, with its prefix length
	Target    string    `json:"target"`
	Added     bool      `json:"added"` // Source was added for the test and removed again
	Sent      int       `json:"sent"`
	Received  int       `json:"received"`
	RttMs     []float64 `json:"rttMs"`
	Error     string    `json:"error"` // why the test could not run, or a send error
}

// Ping tests one network the way a run would use it: it puts the address
// a run would add on the profile's interface (unless the host already has
// it), pings the core's address on that network, and removes the address
// again. It returns a *profile.ValidationError when that network's fields
// are wrong, ErrRunActive during a run and ErrPingActive during another
// test; a run cannot start while a test is going on.
func (c *Controller) Ping(p profile.Profile, plane string) (PingResult, error) {
	c.mu.Lock()
	switch {
	case c.current != nil && !c.current.state().Finished():
		c.mu.Unlock()
		return PingResult{}, ErrRunActive
	case c.pinging:
		c.mu.Unlock()
		return PingResult{}, ErrPingActive
	}
	c.pinging = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.pinging = false
		c.mu.Unlock()
	}()

	hostIPs, err := c.deps.Addrs.HostIPv4s()
	if err != nil {
		return PingResult{}, err
	}
	target, err := profile.PingTargetFor(p, plane, hostIPs)
	if err != nil {
		return PingResult{}, err
	}
	ifaces, err := c.deps.Addrs.Interfaces()
	if err != nil {
		return PingResult{}, err
	}
	if !slices.Contains(ifaces, target.Interface) {
		return PingResult{}, &profile.ValidationError{Errors: []profile.FieldError{{
			Field: "network." + plane + ".interface", Message: fmt.Sprintf("no interface named %q on this host", target.Interface)}}}
	}
	return c.pingFrom(plane, target, hostIPs), nil
}

func (c *Controller) pingFrom(plane string, t profile.PingTarget, hostIPs []netip.Addr) (res PingResult) {
	res = PingResult{Plane: plane, Interface: t.Interface, Source: t.Source.String(), Target: t.Peer.String(), Sent: pingCount, RttMs: []float64{}}
	if !slices.Contains(hostIPs, t.Source.Addr()) {
		if err := c.deps.Addrs.Add(t.Interface, t.Source); err != nil {
			res.Sent, res.Error = 0, fmt.Sprintf("could not add %s to %s: %v", t.Source, t.Interface, err)
			return res
		}
		res.Added = true
		defer func() {
			if err := c.deps.Addrs.Remove(t.Interface, t.Source); err != nil {
				res.Error = joinMessages(res.Error, fmt.Sprintf("could not remove %s from %s: %v", t.Source, t.Interface, err))
			}
		}()
	}
	rtts, err := c.deps.Pinger.Ping(t.Source.Addr(), t.Peer, pingCount, pingTimeout)
	for _, d := range rtts {
		res.RttMs = append(res.RttMs, float64(d)/float64(time.Millisecond))
	}
	res.Received = len(rtts)
	if err != nil {
		res.Error = err.Error()
	}
	return res
}

// Interfaces describes the host's links, for the Setup page's interface
// menus.
func (c *Controller) Interfaces() ([]netcfg.InterfaceInfo, error) {
	return c.deps.Addrs.InterfaceDetails()
}

func joinMessages(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}
