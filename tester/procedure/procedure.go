// Package procedure drives one UE through registration and PDU session
// establishment over its gNB's association, and reports how it ended in
// the terms the stage cards count (metrics.Outcome + cause).
package procedure

import (
	"errors"
	"fmt"
	"net/netip"
	"time"

	"tester/gnb"
	"tester/metrics"
	"tester/ue"
)

// Outcome is how one attempt ended.
type Outcome struct {
	Result metrics.Outcome
	Cause  string        // empty when Accepted
	UeIP   netip.Addr    // PDU only
	Pdu    *gnb.PduSetup // PDU only; nil if the accept came without a resource setup
}

var errTimeout = errors.New("timeout")

// Register runs one initial registration attempt. On success the returned
// link stays attached for the PDU session; on failure it is detached.
// Timing: from sending Initial UE Message to sending Registration Complete.
func Register(assoc *gnb.Association, u *ue.UE, timeout time.Duration) (*gnb.UeLink, Outcome) {
	link, err := assoc.Attach()
	if err != nil {
		return nil, Outcome{Result: metrics.Failed, Cause: err.Error()}
	}
	out := register(assoc, link, u, time.Now().Add(timeout))
	if out.Result != metrics.Accepted {
		assoc.Detach(link)
		return nil, out
	}
	return link, out
}

func register(assoc *gnb.Association, link *gnb.UeLink, u *ue.UE, deadline time.Time) Outcome {
	req, err := u.RegistrationRequest()
	if err != nil {
		return failed(fmt.Errorf("encode registration request: %w", err))
	}
	if err := assoc.SendInitialUE(link, req); err != nil {
		return failed(err)
	}
	for {
		d, err := next(link, deadline)
		if err != nil {
			return failed(err)
		}
		switch d.Kind {
		case gnb.DownlinkLost:
			return failed(fmt.Errorf("association lost: %w", d.Err))
		case gnb.DownlinkReleased:
			return Outcome{Result: metrics.Rejected, Cause: "ue context released"}
		}
		r, err := u.Handle(d.Nas)
		if err != nil {
			return failed(err)
		}
		if r.Reply != nil {
			if err := assoc.SendUplinkNas(link, r.Reply); err != nil {
				return failed(err)
			}
		}
		switch r.Event {
		case ue.EventRegistered:
			return Outcome{Result: metrics.Accepted}
		case ue.EventRegistrationRejected:
			return Outcome{Result: metrics.Rejected, Cause: r.Cause}
		}
	}
}

// EstablishPdu runs one PDU session attempt for a registered UE.
// Timing: from sending the request to receiving the accept, by which point
// the gNB has already answered the PDU Session Resource Setup.
func EstablishPdu(assoc *gnb.Association, link *gnb.UeLink, u *ue.UE, timeout time.Duration) Outcome {
	deadline := time.Now().Add(timeout)
	req, err := u.PduSessionRequest()
	if err != nil {
		return failed(err)
	}
	if err := assoc.SendUplinkNas(link, req); err != nil {
		return failed(err)
	}
	var setup *gnb.PduSetup
	for {
		d, err := next(link, deadline)
		if err != nil {
			return failed(err)
		}
		switch d.Kind {
		case gnb.DownlinkLost:
			return failed(fmt.Errorf("association lost: %w", d.Err))
		case gnb.DownlinkReleased:
			return Outcome{Result: metrics.Rejected, Cause: "ue context released"}
		}
		if d.Pdu != nil {
			setup = d.Pdu
		}
		if d.Nas == nil {
			continue
		}
		r, err := u.Handle(d.Nas)
		if err != nil {
			return failed(err)
		}
		switch r.Event {
		case ue.EventPduEstablished:
			return Outcome{Result: metrics.Accepted, UeIP: r.UeIP, Pdu: setup}
		case ue.EventPduRejected:
			return Outcome{Result: metrics.Rejected, Cause: r.Cause}
		}
	}
}

func next(link *gnb.UeLink, deadline time.Time) (gnb.Downlink, error) {
	t := time.NewTimer(time.Until(deadline))
	defer t.Stop()
	select {
	case d := <-link.Downlinks:
		return d, nil
	case <-t.C:
		return gnb.Downlink{}, errTimeout
	}
}

func failed(err error) Outcome {
	if errors.Is(err, errTimeout) {
		return Outcome{Result: metrics.TimedOut, Cause: "timeout"}
	}
	return Outcome{Result: metrics.Failed, Cause: err.Error()}
}
