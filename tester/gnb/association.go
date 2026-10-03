package gnb

import (
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
)

// DownlinkKind says what a Downlink carries.
type DownlinkKind int

const (
	DownlinkNas      DownlinkKind = iota // Nas is set; Pdu too if it came in a PDU Session Resource Setup
	DownlinkReleased                     // AMF released the UE context (e.g. after a reject)
	DownlinkLost                         // the association went down
)

// PduSetup is what the gNB learned and allocated in a PDU Session
// Resource Setup; the data plane (phase 3) needs all of it.
type PduSetup struct {
	UlTeid  []byte     // UPF side
	UpfIP   netip.Addr // UPF N3 address from the transfer
	DlTeid  uint32     // allocated by the tester
	GnbN3IP netip.Addr // the gNB address the downlink tunnel ends at
	Qfi     int64
}

// Downlink is one event for a UE, delivered in arrival order.
type Downlink struct {
	Kind DownlinkKind
	Nas  []byte
	Pdu  *PduSetup
	Err  error // for DownlinkLost
}

// UeLink is one UE's slot on an association. Downlinks are buffered; if a
// UE stops reading (its procedure timed out) further events are dropped.
type UeLink struct {
	RanUeID   int64
	amfUeID   atomic.Int64 // -1 until the AMF's first downlink
	Downlinks chan Downlink
}

// TeidAllocator hands out DL TEIDs unique across every gNB of a run.
type TeidAllocator struct{ next atomic.Uint32 }

func (t *TeidAllocator) Allocate() uint32 { return t.next.Add(1) }

// Association is one gNB's N2 after NG Setup: it multiplexes every UE of
// the gNB over the single SCTP association, standing in for the Uu link
// free-ran-ue needs between a UE process and a gNB process.
type Association struct {
	conn  Conn
	id    Identity
	n3IPs []netip.Addr // downlink tunnel addresses, handed out in turn
	nextN3 atomic.Uint32
	teids *TeidAllocator

	writeMu sync.Mutex
	nextID  atomic.Int64

	mu     sync.Mutex
	ues    map[int64]*UeLink
	lost   error
	lostCh chan struct{} // closed once when the association fails
}

// NewAssociation hands out n3IPs as downlink tunnel addresses, one PDU
// session after another in turn (most gNBs have just one).
func NewAssociation(conn Conn, id Identity, n3IPs []netip.Addr, teids *TeidAllocator) *Association {
	return &Association{conn: conn, id: id, n3IPs: n3IPs, teids: teids, ues: map[int64]*UeLink{}, lostCh: make(chan struct{})}
}

// Lost is closed when the association fails. Unlike the DownlinkLost
// event, which a UE with a full buffer would miss, it cannot be dropped,
// so a waiting procedure always learns about the loss at once.
func (a *Association) Lost() <-chan struct{} { return a.lostCh }

// Err is the failure that closed Lost, nil before that.
func (a *Association) Err() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lost
}

// Attach gives a UE a fresh RAN UE NGAP ID (a retry attaches again).
func (a *Association) Attach() (*UeLink, error) {
	l := &UeLink{RanUeID: a.nextID.Add(1), Downlinks: make(chan Downlink, 16)}
	l.amfUeID.Store(-1)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lost != nil {
		return nil, a.lost
	}
	a.ues[l.RanUeID] = l
	return l, nil
}

func (a *Association) Detach(l *UeLink) {
	a.mu.Lock()
	delete(a.ues, l.RanUeID)
	a.mu.Unlock()
}

func (a *Association) write(b []byte) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	if _, err := a.conn.Write(b); err != nil {
		return fmt.Errorf("association lost: %w", err)
	}
	return nil
}

// SendInitialUE carries the UE's first NAS message (Registration Request).
func (a *Association) SendInitialUE(l *UeLink, nas []byte) error {
	b, err := a.id.initialUEMessage(l.RanUeID, nas)
	if err != nil {
		return fmt.Errorf("encode initial ue message: %w", err)
	}
	return a.write(b)
}

// SendUplinkNas carries a later NAS message; it needs the AMF UE NGAP ID,
// so it fails if the AMF has not sent anything for this UE yet.
func (a *Association) SendUplinkNas(l *UeLink, nas []byte) error {
	amfID := l.amfUeID.Load()
	if amfID < 0 {
		return errors.New("uplink nas before the amf assigned an amf ue ngap id")
	}
	b, err := a.id.uplinkNASTransport(amfID, l.RanUeID, nas)
	if err != nil {
		return fmt.Errorf("encode uplink nas transport: %w", err)
	}
	return a.write(b)
}

// Run reads NGAP until the association fails and returns that error.
// Every UE still attached then gets DownlinkLost. EAGAIN (SO_RCVTIMEO on
// an idle association) and EINTR are not failures.
func (a *Association) Run() error {
	buf := make([]byte, 65536)
	for {
		n, err := a.conn.Read(buf)
		if err != nil {
			if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EINTR) {
				continue
			}
			a.fail(err)
			return err
		}
		a.dispatch(buf[:n])
	}
}

func (a *Association) fail(err error) {
	a.mu.Lock()
	if a.lost != nil {
		a.mu.Unlock()
		return
	}
	a.lost = fmt.Errorf("association lost: %w", err)
	close(a.lostCh)
	ues := a.ues
	a.ues = map[int64]*UeLink{}
	a.mu.Unlock()
	for _, l := range ues {
		deliver(l, Downlink{Kind: DownlinkLost, Err: err})
	}
}

func (a *Association) link(ranUeID, amfUeID int64) *UeLink {
	a.mu.Lock()
	l := a.ues[ranUeID]
	a.mu.Unlock()
	if l != nil && amfUeID >= 0 {
		l.amfUeID.CompareAndSwap(-1, amfUeID)
	}
	return l
}

func deliver(l *UeLink, d Downlink) {
	select {
	case l.Downlinks <- d:
	default: // the UE's procedure gave up and nobody is reading
	}
}

func (a *Association) dispatch(raw []byte) {
	msg, err := message.Parse(raw)
	if err != nil {
		return // not ours to fix; the AMF will time the UE out
	}
	switch m := msg.(type) {
	case *message.DownlinkNASTransport:
		if m.RANUENGAPID == nil || m.AMFUENGAPID == nil || m.NASPDU == nil {
			return
		}
		if l := a.link(m.RANUENGAPID.Value, m.AMFUENGAPID.Value); l != nil {
			deliver(l, Downlink{Kind: DownlinkNas, Nas: append([]byte(nil), m.NASPDU.Value...)})
		}
	case *message.InitialContextSetupRequest:
		if m.RANUENGAPID == nil || m.AMFUENGAPID == nil {
			return
		}
		l := a.link(m.RANUENGAPID.Value, m.AMFUENGAPID.Value)
		if l == nil {
			return
		}
		if rsp, err := initialContextSetupResponse(m.AMFUENGAPID.Value, m.RANUENGAPID.Value); err == nil {
			_ = a.write(rsp)
		}
		if m.NASPDU != nil {
			deliver(l, Downlink{Kind: DownlinkNas, Nas: append([]byte(nil), m.NASPDU.Value...)})
		}
	case *message.PDUSessionResourceSetupRequest:
		a.onPduSessionResourceSetup(m)
	case *message.UEContextReleaseCommand:
		if m.UENGAPIDs == nil {
			return
		}
		pair, ok := m.UENGAPIDs.Choice.(*ie.UENGAPIDPair)
		if !ok || pair.AMFUENGAPID == nil || pair.RANUENGAPID == nil {
			return
		}
		if rsp, err := a.id.ueContextReleaseComplete(pair.AMFUENGAPID.Value, pair.RANUENGAPID.Value); err == nil {
			_ = a.write(rsp)
		}
		if l := a.link(pair.RANUENGAPID.Value, -1); l != nil {
			deliver(l, Downlink{Kind: DownlinkReleased})
		}
	}
}

func (a *Association) onPduSessionResourceSetup(m *message.PDUSessionResourceSetupRequest) {
	if m.RANUENGAPID == nil || m.AMFUENGAPID == nil || m.PDUSessionResourceSetupListSUReq == nil {
		return
	}
	l := a.link(m.RANUENGAPID.Value, m.AMFUENGAPID.Value)
	if l == nil {
		return
	}
	for _, item := range m.PDUSessionResourceSetupListSUReq.List {
		if item.PDUSessionID == nil {
			continue
		}
		setup := &PduSetup{Qfi: 1, DlTeid: a.teids.Allocate(), GnbN3IP: a.n3IPs[(a.nextN3.Add(1)-1)%uint32(len(a.n3IPs))]}
		if item.PDUSessionResourceSetupRequestTransfer != nil {
			var transfer ie.PDUSessionResourceSetupRequestTransfer
			if err := ie.UnmarshalBinary(*item.PDUSessionResourceSetupRequestTransfer, &transfer); err == nil && transfer.ProtocolIEs != nil {
				for _, f := range transfer.ProtocolIEs.List {
					if f.ULNGUUPTNLInformation != nil {
						if t, ok := f.ULNGUUPTNLInformation.Choice.(*ie.GTPTunnel); ok {
							if t.GTPTEID != nil {
								setup.UlTeid = append([]byte(nil), t.GTPTEID.Value...)
							}
							if t.TransportLayerAddress != nil && len(t.TransportLayerAddress.Value.Bytes) >= 4 {
								setup.UpfIP, _ = netip.AddrFromSlice(t.TransportLayerAddress.Value.Bytes[:4])
							}
						}
					}
					if f.QosFlowSetupRequestList != nil && len(f.QosFlowSetupRequestList.List) > 0 &&
						f.QosFlowSetupRequestList.List[0].QosFlowIdentifier != nil {
						setup.Qfi = f.QosFlowSetupRequestList.List[0].QosFlowIdentifier.Value
					}
				}
			}
		}
		if rsp, err := pduSessionResourceSetupResponse(m.AMFUENGAPID.Value, m.RANUENGAPID.Value,
			item.PDUSessionID.Value, setup.DlTeid, setup.GnbN3IP, setup.Qfi); err == nil {
			_ = a.write(rsp)
		}
		var nas []byte
		if item.PDUSessionNASPDU != nil {
			nas = append([]byte(nil), item.PDUSessionNASPDU.Value...)
		}
		deliver(l, Downlink{Kind: DownlinkNas, Nas: nas, Pdu: setup})
	}
}
