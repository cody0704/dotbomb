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
			// Build a query
			q := new(dns.Msg)
			// 新增 OPT RR 啟用 DNSSEC
			opt := &dns.OPT{
				Hdr: dns.RR_Header{
					Name:   ".",
					Rrtype: dns.TypeOPT,
					Class:  4096, // UDP payload size
				},
			}
			// 啟用 DO bit
			opt.SetDo()
			q.Extra = append(q.Extra, opt)

			for i := range b.TotalRequest {
				domain := b.Domains[i%domainCount]
				qtype := QType[b.DomainQType[i%len(b.DomainQType)]]

				q.SetQuestion(domain, qtype)

				dnsPacket, _ := q.Pack()

				conn.Write(dnsPacket)
				Result.SendLastTime.Store(time.Since(t1).Nanoseconds())
				Result.SendCount.Add(1)

				limiter.Wait(ctx)
			}
		}()

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
	}

	select {
	case <-DoneChan:
		StatusChan <- 0
	case <-timeout.C:
		StatusChan <- 1
	}
}
