package stress

import (
	"encoding/binary"
	"net"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

// Fixed byte offsets within a fake frame: Ethernet(14) + IPv4 header(20, no
// options) + UDP(8). The fake path emits exactly this shape, so the per-packet
// fields live at constant positions and can be patched without re-serializing.
const (
	ipHdrStart    = 14 // start of the IPv4 header
	ipChecksumOff = 24 // IPv4 header checksum (2 bytes)
	ipSrcOff      = 26 // IPv4 source address (4 bytes)
	udpSrcPortOff = 34 // UDP source port (2 bytes)
	ipHdrLen      = 20
)

// buildFakeFrames pre-builds one complete Ethernet/IPv4/UDP frame per query. Every
// field is final except the source IP, source port and IPv4 checksum, which are
// left zero and patched per packet by patchFakeFrame. The UDP checksum is left 0 —
// legal for IPv4 ("not computed") — so the payload isn't re-checksummed on every
// send, which (together with avoiding a full re-serialize) was the bulk of the old
// per-packet cost. gopacket runs here once per query, never in the hot loop.
// Returns the frames and the longest frame length (for sizing a scratch buffer).
func buildFakeFrames(prePacked [][]byte, srcMAC, dstMAC net.HardwareAddr, dstIP net.IP, dstPort int) (frames [][]byte, maxLen int, err error) {
	// ComputeChecksums is intentionally off: IPv4 and UDP checksum fields stay 0.
	opts := gopacket.SerializeOptions{FixLengths: true}
	frames = make([][]byte, len(prePacked))
	for i, payload := range prePacked {
		eth := layers.Ethernet{SrcMAC: srcMAC, DstMAC: dstMAC, EthernetType: layers.EthernetTypeIPv4}
		// SrcIP is a 4-byte zero placeholder (patched per packet); gopacket rejects
		// a nil address during serialization.
		ip := layers.IPv4{Version: 4, TTL: 64, Protocol: layers.IPProtocolUDP, SrcIP: net.IP{0, 0, 0, 0}, DstIP: dstIP}
		udp := layers.UDP{DstPort: layers.UDPPort(dstPort)}

		buf := gopacket.NewSerializeBuffer()
		if err = gopacket.SerializeLayers(buf, opts, &eth, &ip, &udp, gopacket.Payload(payload)); err != nil {
			return nil, 0, err
		}
		frames[i] = append([]byte(nil), buf.Bytes()...) // own a private copy
		if len(frames[i]) > maxLen {
			maxLen = len(frames[i])
		}
	}
	return frames, maxLen, nil
}

// patchFakeFrame copies template into dst (which must have cap >= len(template))
// and fills in the per-packet source IP, source port and IPv4 header checksum,
// returning the ready-to-send frame.
func patchFakeFrame(dst, template []byte, srcIP net.IP, srcPort uint16) []byte {
	dst = dst[:len(template)]
	copy(dst, template)

	if v4 := srcIP.To4(); v4 != nil {
		copy(dst[ipSrcOff:ipSrcOff+4], v4)
	}
	binary.BigEndian.PutUint16(dst[udpSrcPortOff:], srcPort)

	dst[ipChecksumOff], dst[ipChecksumOff+1] = 0, 0
	binary.BigEndian.PutUint16(dst[ipChecksumOff:], ipChecksum(dst[ipHdrStart:ipHdrStart+ipHdrLen]))
	return dst
}

// ipChecksum computes the standard 16-bit one's-complement header checksum. The
// checksum field in hdr must be zero. Summing a header that already carries a
// valid checksum yields 0, which the test relies on.
func ipChecksum(hdr []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(hdr); i += 2 {
		sum += uint32(hdr[i])<<8 | uint32(hdr[i+1])
	}
	if len(hdr)%2 == 1 {
		sum += uint32(hdr[len(hdr)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}
