// Package dataplane generates and verifies user-plane traffic for
// established UEs without TUN devices: uplink is built as GTP-U and sent
// to the UPF's N3 from the gNB's N3 IP; downlink is sent as plain UDP to
// the UE's IP (routed to the UPF's N6) and comes back as GTP-U on the
// gNB's N3 IP. Every packet carries a small header so the receiving side
// can attribute it to a UE even through NAT and measure one-way latency.
package dataplane

import (
	"encoding/binary"
	"errors"
	"net/netip"
)

const (
	// HeaderLen is the tester header at the start of every UDP payload.
	HeaderLen = 24
	// MinPacketSize is the smallest inner IP packet: IPv4 + UDP + header.
	MinPacketSize = ipv4HeaderLen + udpHeaderLen + HeaderLen
	// GtpPort is the GTP-U port on both N3 ends.
	GtpPort = 2152

	ipv4HeaderLen = 20
	udpHeaderLen  = 8
	gtpHeaderLen  = 8
	ulHeadLen     = gtpHeaderLen + ipv4HeaderLen + udpHeaderLen // before the tester header in an uplink G-PDU
	dlFlag        = 1 << 31                                     // high bit of the UE field marks downlink
)

var magic = [4]byte{'F', 'R', 'U', 'T'}

// header is the payload header (design §8):
// magic(4) runID(4) ue|dir(4) seq(4) txNanos(8), big endian.
type header struct {
	runID   uint32
	ue      uint32 // 0-based UE index
	dl      bool
	seq     uint32
	txNanos int64
}

func putHeader(b []byte, h header) {
	copy(b[0:4], magic[:])
	binary.BigEndian.PutUint32(b[4:8], h.runID)
	v := h.ue
	if h.dl {
		v |= dlFlag
	}
	binary.BigEndian.PutUint32(b[8:12], v)
	binary.BigEndian.PutUint32(b[12:16], h.seq)
	binary.BigEndian.PutUint64(b[16:24], uint64(h.txNanos))
}

func parseHeader(b []byte) (header, bool) {
	if len(b) < HeaderLen || [4]byte(b[0:4]) != magic {
		return header{}, false
	}
	v := binary.BigEndian.Uint32(b[8:12])
	return header{
		runID:   binary.BigEndian.Uint32(b[4:8]),
		ue:      v &^ dlFlag,
		dl:      v&dlFlag != 0,
		seq:     binary.BigEndian.Uint32(b[12:16]),
		txNanos: int64(binary.BigEndian.Uint64(b[16:24])),
	}, true
}

// ulTemplate builds one UE's uplink G-PDU: an 8-byte GTP-U header (no
// sequence number or extension headers, per design Q15) carrying an IPv4/
// UDP packet from the UE to the N6 sink. packetSize is the inner IP
// packet's total length. Only the tester header changes per packet, and
// UDP checksum 0 is valid for IPv4, so the template is reused as is.
func ulTemplate(teid uint32, ue, sink netip.Addr, port uint16, packetSize int) []byte {
	b := make([]byte, gtpHeaderLen+packetSize)
	b[0] = 0x30 // version 1, protocol type GTP, no optional fields
	b[1] = 0xff // G-PDU
	binary.BigEndian.PutUint16(b[2:4], uint16(packetSize))
	binary.BigEndian.PutUint32(b[4:8], teid)
	putIPv4UDP(b[gtpHeaderLen:], ue, sink, port, port)
	return b
}

// putIPv4UDP writes IPv4 and UDP headers for a packet of len(b) bytes.
func putIPv4UDP(b []byte, src, dst netip.Addr, sport, dport uint16) {
	ip := b[:ipv4HeaderLen]
	ip[0] = 0x45 // v4, 20-byte header
	binary.BigEndian.PutUint16(ip[2:4], uint16(len(b)))
	ip[6] = 0x40 // don't fragment
	ip[8] = 64   // TTL
	ip[9] = 17   // UDP
	s, d := src.As4(), dst.As4()
	copy(ip[12:16], s[:])
	copy(ip[16:20], d[:])
	binary.BigEndian.PutUint16(ip[10:12], ipChecksum(ip))
	udp := b[ipv4HeaderLen:]
	binary.BigEndian.PutUint16(udp[0:2], sport)
	binary.BigEndian.PutUint16(udp[2:4], dport)
	binary.BigEndian.PutUint16(udp[4:6], uint16(len(udp)))
}

func ipChecksum(h []byte) uint16 {
	var sum uint32
	for i := 0; i < len(h); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(h[i:]))
	}
	for sum > 0xffff {
		sum = sum>>16 + sum&0xffff
	}
	return ^uint16(sum)
}

var errNotGpdu = errors.New("not a G-PDU")

// parseGpdu returns the TEID and inner packet of a GTP-U G-PDU, skipping
// the optional fields and any extension headers (gtp5g adds a PDU Session
// Container to downlink packets).
func parseGpdu(b []byte) (uint32, []byte, error) {
	if len(b) < gtpHeaderLen || b[0]>>5 != 1 || b[1] != 0xff {
		return 0, nil, errNotGpdu
	}
	teid := binary.BigEndian.Uint32(b[4:8])
	off := gtpHeaderLen
	if b[0]&0x07 != 0 { // E, S or PN set: 4 more bytes
		if len(b) < off+4 {
			return 0, nil, errNotGpdu
		}
		next := b[off+3]
		off += 4
		for b[0]&0x04 != 0 && next != 0 { // extension headers
			if len(b) < off+1 {
				return 0, nil, errNotGpdu
			}
			l := int(b[off]) * 4
			if l == 0 || len(b) < off+l {
				return 0, nil, errNotGpdu
			}
			next = b[off+l-1]
			off += l
		}
	}
	return teid, b[off:], nil
}

// udpPayload returns the UDP payload of an IPv4/UDP packet.
func udpPayload(ip []byte) ([]byte, bool) {
	if len(ip) < ipv4HeaderLen || ip[0]>>4 != 4 || ip[9] != 17 {
		return nil, false
	}
	ihl := int(ip[0]&0x0f) * 4
	if len(ip) < ihl+udpHeaderLen {
		return nil, false
	}
	return ip[ihl+udpHeaderLen:], true
}
