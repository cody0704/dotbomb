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
	var domainCount = len(b.Domains)
	expected := b.Expected
	sendDriven := b.IgnoreResponse || b.FakeIF != ""

	// Pre-pack queries to avoid Pack() overhead in the hot loop
	prePacked := make([][]byte, domainCount)
	for i := range domainCount {
		q := new(dns.Msg)
		qtype := QType[b.DomainQType[i%len(b.DomainQType)]]
		q.SetQuestion(b.Domains[i], qtype)
		packed, _ := q.Pack()
		prePacked[i] = packed
	}

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
				const batchSize = 100
				var localSend uint64
				for i := 0; i < b.TotalRequest; i += batchSize {
					count := batchSize
					if i+count > b.TotalRequest {
						count = b.TotalRequest - i
					}

					for j := 0; j < count; j++ {
						idx := (i + j) % domainCount
						dnsPacket := prePacked[idx]

						// Update Message ID to be unique per packet
						// (Optional: for stress testing, sometimes it's okay to reuse ID,
						// but better to have it unique for correct RTT/match tracking)
						// dnsPacket[0], dnsPacket[1] = byte(id>>8), byte(id&0xff)

						conn.Write(dnsPacket)
						localSend++
					}

					Result.SendLastTime.Store(time.Since(t1).Nanoseconds())
					if Result.SendCount.Add(uint64(count)) >= expected && sendDriven {
						SignalDone()
					}

					limiter.WaitN(ctx, count)
				}
			}()

			if b.IgnoreResponse {
				continue
			}

			go drainUDPReplies(conn, t1, expected)
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

				const batchSize = 100
				for i := 0; i < b.TotalRequest; i += batchSize {
					count := batchSize
					if i+count > b.TotalRequest {
						count = b.TotalRequest - i
					}

					for j := 0; j < count; j++ {
						curr := i + j
						idx := curr % domainCount
						dnsPacket := prePacked[idx]

						// 發送封包
						if curr%65535 == 0 {
							ipv4 = startingFakeIP(b.FakeIP, workerID)
						}

						nextIPv4(ipv4)

						ip.SrcIP = ipv4
						udp.SrcPort = layers.UDPPort(curr%50000 + 3000)

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
					}

					Result.SendLastTime.Store(time.Since(t1).Nanoseconds())
					if Result.SendCount.Add(uint64(count)) >= expected {
						SignalDone()
					}

					limiter.WaitN(ctx, count)
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

	if watchIdle(DoneChan, b.LastTimeout) {
		StatusChan <- 1
	} else {
		StatusChan <- 0
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
