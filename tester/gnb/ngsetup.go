// Package gnb holds what one simulated gNB needs on N2: its NGAP identity,
// the SCTP association to the AMF, and the NG Setup exchange.
package gnb

import (
	"fmt"
	"io"

	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
)

// RejectedError is returned when the AMF answers with NGSetupFailure.
type RejectedError struct {
	Cause string // e.g. "misc(4)", see DescribeCause
}

func (e *RejectedError) Error() string { return "ng setup rejected: " + e.Cause }

// ExchangeNGSetup sends request and waits for the AMF's answer. The read
// timeout comes from the connection itself (SO_RCVTIMEO on SCTP, see
// SCTPDialer), because free5gc/sctp does not support read deadlines.
func ExchangeNGSetup(conn io.ReadWriter, request []byte) error {
	if _, err := conn.Write(request); err != nil {
		return fmt.Errorf("send ng setup request: %w", err)
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("read ng setup response: %w", err)
	}
	msg, err := message.Parse(buf[:n])
	if err != nil {
		return fmt.Errorf("decode ng setup response: %w", err)
	}
	switch m := msg.(type) {
	case *message.NGSetupResponse:
		return nil
	case *message.NGSetupFailure:
		return &RejectedError{Cause: DescribeCause(m.Cause)}
	default:
		return fmt.Errorf("unexpected %T in reply to ng setup request", msg)
	}
}

// DescribeCause renders an NGAP cause as "group(value)", e.g. "misc(4)",
// which is what the UI groups failures by.
func DescribeCause(c *ie.Cause) string {
	if c == nil || c.Choice == nil {
		return "unknown"
	}
	switch v := c.Choice.(type) {
	case *ie.CauseRadioNetwork:
		return fmt.Sprintf("radioNetwork(%d)", v.Value)
	case *ie.CauseTransport:
		return fmt.Sprintf("transport(%d)", v.Value)
	case *ie.CauseNas:
		return fmt.Sprintf("nas(%d)", v.Value)
	case *ie.CauseProtocol:
		return fmt.Sprintf("protocol(%d)", v.Value)
	case *ie.CauseMisc:
		return fmt.Sprintf("misc(%d)", v.Value)
	default:
		return fmt.Sprintf("%T", v)
	}
}
