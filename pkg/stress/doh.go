package stress

import (
	"context"
	"crypto/tls"
	"log"
	"strconv"
	"strings"
	"time"

	rdns "github.com/folbricht/routedns"
	"github.com/miekg/dns"
	"golang.org/x/time/rate"
)

func (b Bomb) DoH(ctx context.Context, limiter *rate.Limiter, server string) {
	var timeout *time.Timer = time.NewTimer(b.LastTimeout)
	defer timeout.Stop()

	config := tls.Config{
		InsecureSkipVerify: true,
	}

	var domainCount = len(b.Domains)

	t1 := time.Now() // get current time

	for workerID := range b.Concurrency {
		go func(workerID int) {
			// Build a query
			q := new(dns.Msg)

			// Resolve the query
			dohClient, err := rdns.NewDoHClient("stress-doh-"+strconv.Itoa(workerID), server, rdns.DoHClientOptions{
				Method:       b.Method,
				TLSConfig:    &config,
				QueryTimeout: b.LastTimeout,
			})
			if err != nil {
				log.Println(err)
				return
			}

			for i := range b.TotalRequest {
				domain := b.Domains[i%domainCount]
				qtype := QType[b.DomainQType[i%len(b.DomainQType)]]

				q.SetQuestion(domain, qtype)
				Result.SendLastTime = time.Since(t1)
				Result.SendCount.Add(1)
				limiter.Wait(ctx)

				resp, err := dohClient.Resolve(q, rdns.ClientInfo{})
				if err != nil {
					if strings.Contains(err.Error(), "timed out") || strings.Contains(err.Error(), "timeout") {
						Result.TimeoutCount.Add(1)
					} else {
						Result.OtherCount.Add(1)
					}
					continue
				}

				Result.RecvLastTime = time.Since(t1)
				answers := resp.Answer
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

		}(workerID)
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
