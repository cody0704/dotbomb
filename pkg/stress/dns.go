package stress

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/time/rate"
)

func (b *Bomb) DNS(ctx context.Context, limiter *rate.Limiter, requestIP string, requestPort int) {
	var timeout *time.Timer = time.NewTimer(b.LastTimeout)
	defer timeout.Stop()

	var domainCount = len(b.Domains)

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
