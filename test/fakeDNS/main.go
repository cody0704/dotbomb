package main

import (
	"log"
	"net"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcap"
	"github.com/miekg/dns"
)

func main() {
	// 打開網路介面
	handle, err := pcap.OpenLive("en0", 65535, true, pcap.BlockForever)
	if err != nil {
		log.Fatal("Error opening device %v: %v", "en0", err)
		return
	}
	defer handle.Close()

	srcMAC, err := net.ParseMAC("42:05:77:c4:ed:71")
	if err != nil {
		log.Fatal("srcMAC parse err", err)
	}
	dstMAC, err := net.ParseMAC("f2:e2:08:2d:67:ef")
	if err != nil {
		log.Fatal("dstMAC parse err", err)
	}
	// 建立 Ethernet II frame
	ethernet := layers.Ethernet{
		SrcMAC:       srcMAC,
		DstMAC:       dstMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}

	// 建立封包
	buffer := gopacket.NewSerializeBuffer()
	options := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	// 建立 IP 層
	ip := layers.IPv4{
		DstIP:    net.ParseIP("192.168.0.205"),
		Version:  4,
		TTL:      64,
		Protocol: layers.IPProtocolUDP,
	}

	// 建立 UDP 層
	udp := layers.UDP{
		DstPort: layers.UDPPort(53),
	}

	// Build a query
	q := new(dns.Msg)

	q.SetQuestion("google.com.", dns.TypeA)

	dnsPacket, _ := q.Pack()

	ip.SrcIP = net.ParseIP("192.168.0.1")
	udp.SrcPort = layers.UDPPort(1234)

	udp.SetNetworkLayerForChecksum(&ip)

	// 發送封包
	err = gopacket.SerializeLayers(buffer, options, &ethernet, &ip, &udp, gopacket.Payload(dnsPacket))
	if err != nil {
		log.Fatal("gopacket.SerializeLayers failed:", err)
		return
	}
	outgoingPacket := buffer.Bytes()

	err = handle.WritePacketData(outgoingPacket)
	if err != nil {
		log.Fatal("Error sending packet:", err)
		return
	}

	log.Println("Packet sent successfully")
}
