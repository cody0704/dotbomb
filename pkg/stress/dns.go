package stress

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

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

	// Fake-source setup is the same for every worker (MACs, target, payloads), so
	// build the per-query frame templates once here; workers only patch the source
	// IP/port + IP checksum per packet (see fakesend.go). On a setup error we skip
	// launching any fake worker — the fakeWG watcher below then fires SignalDone.
	var fakeFrames [][]byte
	var fakeMaxLen int
	fakeReady := false
	if b.FakeIF != "" {
		srcMAC, errS := net.ParseMAC(b.FakeSourceMac)
		dstMAC, errD := net.ParseMAC(b.FakeTargetMac)
		switch {
		case errS != nil:
			log.Printf("fake: parse src MAC %q: %v", b.FakeSourceMac, errS)
		case errD != nil:
			log.Printf("fake: parse dst MAC %q: %v", b.FakeTargetMac, errD)
		case startingFakeIP(b.FakeIP, 0) == nil:
			log.Printf("fake: invalid -fip %q (need an IPv4 address)", b.FakeIP)
		default:
			var err error
			fakeFrames, fakeMaxLen, err = buildFakeFrames(prePacked, srcMAC, dstMAC, net.ParseIP(requestIP), requestPort)
			if err != nil {
				log.Printf("fake: build frames: %v", err)
			} else {
				fakeReady = true
			}
		}
	}

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

			go sendUDPBatched(ctx, conn, limiter, prePacked, b.TotalRequest, t1, expected, sendDriven)

			if b.IgnoreResponse {
				continue
			}

			go drainUDPReplies(conn, t1, expected)
		} else if fakeReady {
			fakeWG.Add(1)
			go func(workerID int) {
				defer fakeWG.Done()
				handle, err := pcap.OpenLive(b.FakeIF, 65535, true, pcap.BlockForever)
				if err != nil {
					log.Printf("worker %d: pcap open %s: %v", workerID, b.FakeIF, err)
					return
				}
				defer handle.Close()

				dstIP := net.ParseIP(requestIP)
				srcIP := startingFakeIP(b.FakeIP, workerID)
				scratch := make([]byte, fakeMaxLen)

				batchSize := limiter.Burst()
				for i := 0; i < b.TotalRequest; i += batchSize {
					count := batchSize
					if i+count > b.TotalRequest {
						count = b.TotalRequest - i
					}
					limiter.WaitN(ctx, count) // pace before sending

					for j := 0; j < count; j++ {
						curr := i + j

						// Rotate the spoofed source IP; reset every /16 wrap and
						// skip the target's own address.
						if curr%65535 == 0 {
							srcIP = startingFakeIP(b.FakeIP, workerID)
						}
						nextIPv4(srcIP)
						if srcIP.Equal(dstIP) {
							nextIPv4(srcIP)
						}

						frame := patchFakeFrame(scratch, fakeFrames[curr%domainCount], srcIP, uint16(curr%50000+3000))
						if err := handle.WritePacketData(frame); err != nil {
							log.Println("fake send:", err)
						}
					}

					Result.SendLastTime.Store(time.Since(t1).Nanoseconds())
					if Result.SendCount.Add(uint64(count)) >= expected {
						SignalDone()
					}
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
