package stress

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcap"
	"github.com/miekg/dns"
	"golang.org/x/time/rate"
)

func (b *Bomb) DNS(ctx context.Context, limiter *rate.Limiter, requestIP string, requestPort int) {
	var timeout *time.Timer = time.NewTimer(b.LastTimeout)
	defer timeout.Stop()

	var domainCount = len(b.Domains)

	t1 := time.Now() // get current time
	for range b.Concurrency {
		if b.FakeIF == "" {
			// 創建一個本地地址，使用端口 0，會自動分配一個可用端口
			laddr, err := net.ResolveUDPAddr("udp", ":0")
			if err != nil {
				fmt.Println("Error resolving local address:", err)
				continue
			}

			conn, err := net.DialUDP("udp", laddr, &net.UDPAddr{IP: net.ParseIP(requestIP), Port: requestPort})
			if err != nil {
				log.Println("cannot create udp socket:", err)
				continue
			}

			conn.SetWriteBuffer(32 * 1024 * 1024)
			conn.SetReadBuffer(256 * 1024 * 1024)

			go func() {
				// Build a query
				q := new(dns.Msg)

				for i := range b.TotalRequest {
					domain := b.Domains[i%domainCount]
					qtype := QType[b.DomainQType[i%len(b.DomainQType)]]

					q.SetQuestion(domain, qtype)

					dnsPacket, _ := q.Pack()

					conn.Write(dnsPacket)
					Result.SendLastTime = time.Since(t1)
					Result.SendCount.Add(1)

					limiter.Wait(ctx)
				}
			}()

			go func(conn *net.UDPConn) {
				for {
					var incoming [1024]byte
					var dnsReply dns.Msg
					n, err := conn.Read(incoming[:])
					if err != nil {
						log.Println("recv dns err", err)
						Result.StopSockCount.Add(1)
						// StressChannel <- fmt.Sprintf("%d close", count)
						break
					}
					Result.RecvLastTime = time.Since(t1)

					err = dnsReply.Unpack(incoming[:n])
					if err != nil {
						log.Println("recv dns msg err", err)
					}

					answers := dnsReply.Answer
					if len(answers) > 0 {
						switch len(strings.Split(answers[0].String(), "\t")) {
						case 5:
							Result.RecvAnsCount.Add(1)
						default:
							Result.RecvNoAnsCount.Add(1)
						}
					} else {
						Result.RecvNoAnsCount.Add(1)
					}

					timeout.Reset(b.LastTimeout)
				}
			}(conn)
		} else {
			// 打開網路介面
			handle, err := pcap.OpenLive(b.FakeIF, 65535, true, pcap.BlockForever)
			if err != nil {
				log.Fatalf("Error opening device %s: %v", b.FakeIF, err)
				return
			}
			defer handle.Close()

			srcMAC, err := net.ParseMAC(b.FakeSourceMac)
			if err != nil {
				log.Fatal(err)
			}
			dstMAC, err := net.ParseMAC(b.FakeTargetMac)
			if err != nil {
				log.Fatal(err)
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
				DstIP:    net.ParseIP(requestIP),
				Version:  4,
				TTL:      64,
				Protocol: layers.IPProtocolUDP,
			}

			// 建立 UDP 層
			udp := layers.UDP{
				DstPort: layers.UDPPort(requestPort),
			}

			ipv4 := net.ParseIP(b.FakeIP).To4()

			// Build a query
			q := new(dns.Msg)

			for i := range b.TotalRequest {
				domain := b.Domains[i%domainCount]
				qtype := QType[b.DomainQType[i%len(b.DomainQType)]]

				q.SetQuestion(domain, qtype)

				dnsPacket, _ := q.Pack()

				// 發送封包
				if i%65535 == 0 {
					ipv4 = net.ParseIP(b.FakeIP).To4()
				}

				nextIPv4(ipv4)

				ip.SrcIP = ipv4
				udp.SrcPort = layers.UDPPort(i%50000 + 3000)

				if ip.SrcIP.Equal(ip.DstIP) {
					nextIPv4(ipv4)
				}

				udp.SetNetworkLayerForChecksum(&ip)

				gopacket.SerializeLayers(buffer, options, &ethernet, &ip, &udp, gopacket.Payload(dnsPacket))
				outgoingPacket := buffer.Bytes()

				err = handle.WritePacketData(outgoingPacket)
				if err != nil {
					fmt.Println("Error sending packet:", err)
				}

				Result.SendLastTime = time.Since(t1)
				Result.SendCount.Add(1)

				limiter.Wait(ctx)
			}

			// 關閉封包
			StatusChan <- 0
		}
	}

	for {
		select {
		case <-timeout.C:
			StatusChan <- 1
			return
		default:
			if int(Result.RecvNoAnsCount.Load()+Result.RecvAnsCount.Load()) == b.Concurrency*b.TotalRequest {
				StatusChan <- 0
				return
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func nextIPv4(ip net.IP) {
	for {
		for i := len(ip) - 1; i >= 0; i-- {
			ip[i]++
			if ip[i] != 0 {
				break
			}
		}
		if ip[1] == 255 || ip[2] == 255 || ip[3] == 255 || ip[3] == 0 {
			continue
		}
		if ip[1] == 254 && ip[2] == 254 && ip[3] == 254 {
			break
		}

		break
	}
}
