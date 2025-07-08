package stress

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/time/rate"
)

func (b *Bomb) DNS(requestIP string, requestPort int) {
	var timeout *time.Timer = time.NewTimer(b.LastTimeout)
	defer timeout.Stop()

	// TPS 限制
	var ctx = context.Background()
	limiter := rate.NewLimiter(rate.Limit(b.Interval), 1)

	var domainCount = len(b.Domains)

	t1 := time.Now() // get current time
	for count := 1; count <= b.Concurrency; count++ {
		// 創建一個本地地址，使用端口 0
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
				atomic.AddUint64(&Result.SendCount, 1)

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
					atomic.AddUint64(&Result.StopSockCount, 1)
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
						atomic.AddUint64(&Result.RecvAnsCount, 1)
					default:
						atomic.AddUint64(&Result.RecvNoAnsCount, 1)
					}
				} else {
					atomic.AddUint64(&Result.RecvNoAnsCount, 1)
				}

				timeout.Reset(b.LastTimeout)
			}
		}(conn)
	}

	for {
		select {
		case <-timeout.C:
			StatusChan <- 1
			return
		default:
			if int(Result.RecvNoAnsCount+Result.RecvAnsCount) == b.Concurrency*b.TotalRequest {
				StatusChan <- 0
				return
			}
		}

		time.Sleep(1 * time.Second)
	}
}
