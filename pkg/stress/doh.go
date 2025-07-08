package stress

import (
	"context"
	"crypto/tls"
	"log"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	rdns "github.com/folbricht/routedns"
	"github.com/miekg/dns"
	"golang.org/x/time/rate"
)

func (b Bomb) DoH() {
	var timeout *time.Timer = time.NewTimer(b.LastTimeout)
	defer timeout.Stop()

	// TPS 限制
	var ctx = context.Background()
	limiter := rate.NewLimiter(rate.Limit(b.Interval), 1)

	config := tls.Config{
		InsecureSkipVerify: true,
	}

	var domainCount = len(b.Domains)

	t1 := time.Now() // get current time

	for count := 1; count <= b.Concurrency; count++ {
		go func() {
			// Build a query
			q := new(dns.Msg)

			// Resolve the query
			dohClient, err := rdns.NewDoHClient("stress-doh-"+strconv.Itoa(count), b.Server, rdns.DoHClientOptions{
				Method:       b.Method,
				TLSConfig:    &config,
				QueryTimeout: b.LastTimeout,
			})
			if err != nil {
				log.Println(err)
				return
			}

			for i := 0; i < b.TotalRequest; i++ {
				domain := b.Domains[i%domainCount]
				qtype := QType[b.DomainQType[i%len(b.DomainQType)]]

				q.SetQuestion(domain, qtype)
				Result.SendLastTime = time.Since(t1)
				atomic.AddUint64(&Result.SendCount, 1)
				limiter.Wait(ctx)

				resp, err := dohClient.Resolve(q, rdns.ClientInfo{})
				if err != nil {
					if strings.Contains(err.Error(), "timed out") || strings.Contains(err.Error(), "timeout") {
						atomic.AddUint64(&Result.TimeoutCount, 1)
					} else {
						atomic.AddUint64(&Result.OtherCount, 1)
					}
					continue
				}

				Result.RecvLastTime = time.Since(t1)
				answers := resp.Answer
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

		}()
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
