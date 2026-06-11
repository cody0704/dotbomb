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

		go drainUDPReplies(conn, t1, expected)
	}

	if watchIdle(DoneChan, b.LastTimeout) {
		StatusChan <- 1
	} else {
		StatusChan <- 0
	}
}
