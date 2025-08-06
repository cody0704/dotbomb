package stress

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	rdns "github.com/folbricht/routedns"
	"github.com/miekg/dns"
	"golang.org/x/time/rate"
)

func (b Bomb) DoT(ctx context.Context, limiter *rate.Limiter, requestIP string, requestPort int) {
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
			dotClient, err := rdns.NewDoTClient("stress-dot-"+strconv.Itoa(workerID), fmt.Sprintf("%s:%d", requestIP, requestPort), rdns.DoTClientOptions{
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

				resp, err := dotClient.Resolve(q, rdns.ClientInfo{})
				if err != nil {
					if strings.Contains(err.Error(), "timed out") {
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
