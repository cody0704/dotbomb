package stress

import (
	"context"
	"crypto/tls"
	"log"
	"strconv"
	"strings"
	"sync"
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
	expected := b.Expected
	inflight := max(1, b.Inflight)

	t1 := time.Now() // get current time

	for workerID := range b.Concurrency {
		go func(workerID int) {
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

			// 每個 worker 內開 inflight 條 inner goroutine 共用 dohClient (http.Client).
			// 對 HTTP/2 server 會自動 multiplex 在同一條 TLS 連線上.
			var wg sync.WaitGroup
			for slot := range inflight {
				wg.Add(1)
				go func(slot int) {
					defer wg.Done()
					q := new(dns.Msg)
					for i := slot; i < b.TotalRequest; i += inflight {
						domain := b.Domains[i%domainCount]
						qtype := QType[b.DomainQType[i%len(b.DomainQType)]]

						q.SetQuestion(domain, qtype)
						Result.SendLastTime.Store(time.Since(t1).Nanoseconds())
						Result.SendCount.Add(1)
						limiter.Wait(ctx)

						resp, err := dohClient.Resolve(q, rdns.ClientInfo{})
						if err != nil {
							if strings.Contains(err.Error(), "timed out") || strings.Contains(err.Error(), "timeout") {
								Result.TimeoutCount.Add(1)
							} else {
								Result.OtherCount.Add(1)
							}
							Result.MaybeSignalDone(expected)
							continue
						}

						Result.RecvLastTime.Store(time.Since(t1).Nanoseconds())
						if len(resp.Answer) > 0 {
							Result.RecvAnsCount.Add(1)
						} else {
							Result.RecvNoAnsCount.Add(1)
						}

						Result.MaybeSignalDone(expected)
						timeout.Reset(b.LastTimeout)
					}
				}(slot)
			}
			wg.Wait()
		}(workerID)
	}

	select {
	case <-DoneChan:
		StatusChan <- 0
	case <-timeout.C:
		StatusChan <- 1
	}
}
