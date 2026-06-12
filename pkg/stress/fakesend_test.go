package stress

import (
	"bytes"
	"net"
	"testing"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/miekg/dns"
)

// TestFakeFrameRoundTrip builds a frame the same way the fake path does, patches
// in a source IP/port, then parses the raw bytes back and checks every field —
// verifying the hand-written byte patching (and the IPv4 checksum) without needing
// a live interface.
func TestFakeFrameRoundTrip(t *testing.T) {
	q := new(dns.Msg)
	q.SetQuestion("example.com.", dns.TypeA)
	payload, _ := q.Pack()

	srcMAC, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	dstMAC, _ := net.ParseMAC("00:11:22:33:44:55")
	dstIP := net.ParseIP("192.0.2.1")
	const dstPort = 53

	frames, maxLen, err := buildFakeFrames([][]byte{payload}, srcMAC, dstMAC, dstIP, dstPort)
	if err != nil {
		t.Fatalf("buildFakeFrames: %v", err)
	}
	if maxLen != len(frames[0]) {
		t.Errorf("maxLen = %d, want %d", maxLen, len(frames[0]))
	}

	srcIP := net.IPv4(10, 1, 2, 3).To4()
	const srcPort = 4321
	frame := patchFakeFrame(make([]byte, maxLen), frames[0], srcIP, srcPort)

	pkt := gopacket.NewPacket(frame, layers.LayerTypeEthernet, gopacket.Default)

	eth, _ := pkt.Layer(layers.LayerTypeEthernet).(*layers.Ethernet)
	if eth == nil {
		t.Fatal("no Ethernet layer")
	}
	if eth.SrcMAC.String() != srcMAC.String() || eth.DstMAC.String() != dstMAC.String() {
		t.Errorf("MACs = %s -> %s, want %s -> %s", eth.SrcMAC, eth.DstMAC, srcMAC, dstMAC)
	}

	ip, _ := pkt.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
	if ip == nil {
		t.Fatal("no IPv4 layer")
	}
	if !ip.SrcIP.Equal(srcIP) {
		t.Errorf("SrcIP = %s, want %s", ip.SrcIP, srcIP)
	}
	if !ip.DstIP.Equal(dstIP) {
		t.Errorf("DstIP = %s, want %s", ip.DstIP, dstIP)
	}
	if ip.Protocol != layers.IPProtocolUDP {
		t.Errorf("Protocol = %v, want UDP", ip.Protocol)
	}
	if want := uint16(ipHdrLen + 8 + len(payload)); ip.Length != want {
		t.Errorf("IP total length = %d, want %d", ip.Length, want)
	}
	// A valid IPv4 checksum: summing the header (checksum field included) gives 0.
	if got := ipChecksum(frame[ipHdrStart : ipHdrStart+ipHdrLen]); got != 0 {
		t.Errorf("IPv4 checksum invalid: residual = %#04x, want 0", got)
	}

	udp, _ := pkt.Layer(layers.LayerTypeUDP).(*layers.UDP)
	if udp == nil {
		t.Fatal("no UDP layer")
	}
	if udp.SrcPort != srcPort || udp.DstPort != dstPort {
		t.Errorf("ports = %d -> %d, want %d -> %d", udp.SrcPort, udp.DstPort, srcPort, dstPort)
	}
	if want := uint16(8 + len(payload)); udp.Length != want {
		t.Errorf("UDP length = %d, want %d", udp.Length, want)
	}
	if udp.Checksum != 0 {
		t.Errorf("UDP checksum = %#04x, want 0 (disabled)", udp.Checksum)
	}
	if !bytes.Equal(udp.Payload, payload) {
		t.Errorf("payload mismatch: got %d bytes, want %d", len(udp.Payload), len(payload))
	}
}

func benchFakeInputs() (payload []byte, srcMAC, dstMAC net.HardwareAddr, dstIP net.IP) {
	q := new(dns.Msg)
	q.SetQuestion("example.com.", dns.TypeA)
	payload, _ = q.Pack()
	srcMAC, _ = net.ParseMAC("aa:bb:cc:dd:ee:ff")
	dstMAC, _ = net.ParseMAC("00:11:22:33:44:55")
	dstIP = net.ParseIP("192.0.2.1")
	return
}

// BenchmarkFakeSerializeOld is the old per-packet cost: a full gopacket
// SerializeLayers with IPv4+UDP checksums recomputed every packet.
func BenchmarkFakeSerializeOld(b *testing.B) {
	payload, srcMAC, dstMAC, dstIP := benchFakeInputs()
	eth := layers.Ethernet{SrcMAC: srcMAC, DstMAC: dstMAC, EthernetType: layers.EthernetTypeIPv4}
	ip := layers.IPv4{Version: 4, TTL: 64, Protocol: layers.IPProtocolUDP, DstIP: dstIP}
	udp := layers.UDP{DstPort: 53}
	opts := gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}
	buf := gopacket.NewSerializeBuffer()
	srcIP := net.IPv4(10, 1, 2, 3)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ip.SrcIP = srcIP
		udp.SrcPort = layers.UDPPort(3000 + i%50000)
		udp.SetNetworkLayerForChecksum(&ip)
		_ = gopacket.SerializeLayers(buf, opts, &eth, &ip, &udp, gopacket.Payload(payload))
		_ = buf.Bytes()
	}
}

// BenchmarkFakePatchNew is the new per-packet cost: copy a prebuilt template and
// patch source IP/port + IPv4 checksum (UDP checksum left 0).
func BenchmarkFakePatchNew(b *testing.B) {
	payload, srcMAC, dstMAC, dstIP := benchFakeInputs()
	frames, maxLen, _ := buildFakeFrames([][]byte{payload}, srcMAC, dstMAC, dstIP, 53)
	scratch := make([]byte, maxLen)
	srcIP := net.IPv4(10, 1, 2, 3).To4()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = patchFakeFrame(scratch, frames[0], srcIP, uint16(3000+i%50000))
	}
}
