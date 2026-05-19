package stress

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/time/rate"
)

func (b *Bomb) DNSSEC(ctx context.Context, limiter *rate.Limiter, requestIP string, requestPort int) {
	var timeout *time.Timer = time.NewTimer(b.LastTimeout)
	defer timeout.Stop()

	var domainCount = len(b.Domains)
	expected := b.Expected

	// Pre-pack queries with DNSSEC options
	prePacked := make([][]byte, domainCount)
	for i := range domainCount {
		q := new(dns.Msg)
		// OPT RR for DNSSEC
		opt := &dns.OPT{
			Hdr: dns.RR_Header{
				Name:   ".",
				Rrtype: dns.TypeOPT,
				Class:  4096, // UDP payload size
			},
		}
		opt.SetDo()
		q.Extra = append(q.Extra, opt)

		qtype := QType[b.DomainQType[i%len(b.DomainQType)]]
		q.SetQuestion(b.Domains[i], qtype)
		packed, _ := q.Pack()
		prePacked[i] = packed
	}

	t1 := time.Now() // get current time
	for range b.Concurrency {
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
			for i := 0; i < b.TotalRequest; i += batchSize {
				count := batchSize
				if i+count > b.TotalRequest {
					count = b.TotalRequest - i
				}

				for j := 0; j < count; j++ {
					idx := (i + j) % domainCount
					dnsPacket := prePacked[idx]
					conn.Write(dnsPacket)
				}

				Result.SendLastTime.Store(time.Since(t1).Nanoseconds())
				Result.SendCount.Add(uint64(count))
				limiter.WaitN(ctx, count)
			}
		}()

		go func(conn *net.UDPConn) {
			const flushThreshold = 500
			var localAns, localNoAns, localProcessed uint64
			lastReset := time.Now()

			for {
				bufPtr := bufPool.Get().(*[]byte)
				incoming := *bufPtr

				n, err := conn.Read(incoming)
				if err != nil {
					bufPool.Put(bufPtr)
					log.Println("recv dns err", err)
					Result.StopSockCount.Add(1)
					break
				}
				Result.RecvLastTime.Store(time.Since(t1).Nanoseconds())

				var dnsReply dns.Msg
				err = dnsReply.Unpack(incoming[:n])
				bufPool.Put(bufPtr)

				if err != nil {
					log.Println("recv dns msg err", err)
				}

				if len(dnsReply.Answer) > 0 {
					localAns++
				} else {
					localNoAns++
				}
				localProcessed++

				if localProcessed >= flushThreshold {
					Result.RecvAnsCount.Add(localAns)
					Result.RecvNoAnsCount.Add(localNoAns)
					Result.MaybeSignalDone(expected)
					localAns, localNoAns, localProcessed = 0, 0, 0

					if time.Since(lastReset) > 100*time.Millisecond {
						timeout.Reset(b.LastTimeout)
						lastReset = time.Now()
					}
				}
			}
			Result.RecvAnsCount.Add(localAns)
			Result.RecvNoAnsCount.Add(localNoAns)
			Result.MaybeSignalDone(expected)
		}(conn)
	}

	select {
	case <-DoneChan:
		StatusChan <- 0
	case <-timeout.C:
		StatusChan <- 1
	}
}
