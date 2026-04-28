package stress

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
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
	expected := b.Expected
	sendDriven := b.IgnoreResponse || b.FakeIF != ""

	// fakeWG fires SignalDone when all fake workers have exited, so a setup
	// failure (bad interface, bad MAC) doesn't leave main hanging on DoneChan.
	var fakeWG sync.WaitGroup

	t1 := time.Now() // get current time
	for workerID := range b.Concurrency {
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
					Result.SendLastTime.Store(time.Since(t1).Nanoseconds())
					if Result.SendCount.Add(1) >= expected && sendDriven {
						SignalDone()
					}

					limiter.Wait(ctx)
				}
			}()

			if b.IgnoreResponse {
				continue
			}

			go func(conn *net.UDPConn) {
				for {
					var incoming [4096]byte
					var dnsReply dns.Msg
					n, err := conn.Read(incoming[:])
					if err != nil {
						log.Println("recv dns err", err)
						Result.StopSockCount.Add(1)
						// StressChannel <- fmt.Sprintf("%d close", count)
						break
					}
					Result.RecvLastTime.Store(time.Since(t1).Nanoseconds())

					err = dnsReply.Unpack(incoming[:n])
					if err != nil {
						log.Println("recv dns msg err", err)
					}

					if len(dnsReply.Answer) > 0 {
						Result.RecvAnsCount.Add(1)
					} else {
						Result.RecvNoAnsCount.Add(1)
					}

					Result.MaybeSignalDone(expected)
					timeout.Reset(b.LastTimeout)
				}
			}(conn)
		} else {
			fakeWG.Add(1)
			go func(workerID int) {
				defer fakeWG.Done()
				// 打開網路介面
				handle, err := pcap.OpenLive(b.FakeIF, 65535, true, pcap.BlockForever)
				if err != nil {
					log.Printf("worker %d: pcap open %s: %v", workerID, b.FakeIF, err)
					return
				}
				defer handle.Close()

				srcMAC, err := net.ParseMAC(b.FakeSourceMac)
				if err != nil {
					log.Printf("worker %d: parse src MAC: %v", workerID, err)
					return
				}
				dstMAC, err := net.ParseMAC(b.FakeTargetMac)
				if err != nil {
					log.Printf("worker %d: parse dst MAC: %v", workerID, err)
					return
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

				ipv4 := startingFakeIP(b.FakeIP, workerID)

				// Build a query
				q := new(dns.Msg)

				for i := range b.TotalRequest {
					domain := b.Domains[i%domainCount]
					qtype := QType[b.DomainQType[i%len(b.DomainQType)]]

					q.SetQuestion(domain, qtype)

					dnsPacket, _ := q.Pack()

					// 發送封包
					if i%65535 == 0 {
						ipv4 = startingFakeIP(b.FakeIP, workerID)
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

					Result.SendLastTime.Store(time.Since(t1).Nanoseconds())
					if Result.SendCount.Add(1) >= expected {
						SignalDone()
					}

					limiter.Wait(ctx)
				}
			}(workerID)
		}
	}

	// 若所有 fake worker 都提早返回（pcap/MAC 設定錯誤），這個 goroutine
	// 會送出 DoneChan，避免 main 卡在 <-DoneChan. 只在 fake 模式啟動 — 非 fake
	// 模式 fakeWG 計數為 0，Wait 會立刻返回造成過早 SignalDone.
	if b.FakeIF != "" {
		go func() {
			fakeWG.Wait()
			SignalDone()
		}()
	}

	if sendDriven {
		<-DoneChan
		StatusChan <- 0
		return
	}

	select {
	case <-DoneChan:
		StatusChan <- 0
	case <-timeout.C:
		StatusChan <- 1
	}
}

// startingFakeIP returns FakeIP with byte 2 advanced by workerID so each fake
// worker rotates through a disjoint /16-ish slice of the source-IP space.
// Wraps within 1..253 to avoid 0/255 (network/broadcast) and the 254 plateau
// where nextIPv4 stops advancing.
func startingFakeIP(fakeIP string, workerID int) net.IP {
	ip := net.ParseIP(fakeIP).To4()
	if ip == nil {
		return ip
	}
	ip[2] = byte(1 + (int(ip[2])-1+workerID)%253)
	return ip
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
