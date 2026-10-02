package dataplane

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHeaderRoundTrip(t *testing.T) {
	b := make([]byte, HeaderLen)
	in := header{runID: 7, ue: 41, dl: true, seq: 99, txNanos: 1234567890123}
	putHeader(b, in)
	out, ok := parseHeader(b)
	require.True(t, ok)
	require.Equal(t, in, out)
	b[0] = 'X'
	_, ok = parseHeader(b)
	require.False(t, ok, "foreign traffic is ignored")
}

func TestUlTemplateIsAValidGpdu(t *testing.T) {
	pkt := ulTemplate(0x10000001, netip.MustParseAddr("10.60.0.7"), netip.MustParseAddr("10.0.1.1"), 9200, 100)
	require.Len(t, pkt, 8+100)
	teid, inner, err := parseGpdu(pkt)
	require.NoError(t, err)
	require.Equal(t, uint32(0x10000001), teid)
	require.Len(t, inner, 100)
	require.Equal(t, uint16(0), ipChecksum(inner[:20]), "valid IPv4 header checksum sums to zero")
	require.Equal(t, []byte{10, 60, 0, 7}, inner[12:16])
	payload, ok := udpPayload(inner)
	require.True(t, ok)
	require.Len(t, payload, 100-28)
}

func TestParseGpduSkipsExtensionHeaders(t *testing.T) {
	inner := make([]byte, 40)
	putIPv4UDP(inner, netip.MustParseAddr("10.0.1.1"), netip.MustParseAddr("10.60.0.7"), 9200, 9200)
	// flags E set, seq/npdu zero, next ext = 0x85 (PDU session container,
	// 1 unit = 4 bytes, ending with next type 0)
	pkt := append([]byte{0x34, 0xff, 0, 0, 0, 0, 0, 9, 0, 0, 0, 0x85, 1, 0x00, 0x09, 0x00}, inner...)
	teid, got, err := parseGpdu(pkt)
	require.NoError(t, err)
	require.Equal(t, uint32(9), teid)
	require.Equal(t, inner, got)

	_, _, err = parseGpdu([]byte{0x30, 0x01, 0, 0, 0, 0, 0, 1}) // echo request, not a G-PDU
	require.ErrorIs(t, err, errNotGpdu)
	_, _, err = parseGpdu([]byte{0x34, 0xff, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0x85, 0}) // ext length 0
	require.ErrorIs(t, err, errNotGpdu)
}
