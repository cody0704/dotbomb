package stress

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	rdns "github.com/folbricht/routedns"
	"github.com/miekg/dns"
	"golang.org/x/time/rate"
)

func (b *Bomb) DoT(ctx context.Context, limiter *rate.Limiter, requestIP string, requestPort int) {
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
			dotClient, err := rdns.NewDoTClient("stress-dot-"+strconv.Itoa(workerID), fmt.Sprintf("%s:%d", requestIP, requestPort), rdns.DoTClientOptions{
				TLSConfig:    &config,
				QueryTimeout: b.LastTimeout,
			})
			if err != nil {
				log.Println(err)
				return
			}

			// 每個 worker 內開 inflight 條 inner goroutine 共用 dotClient.
			// routedns DoT 的 pipeline 會把多筆 Resolve 在同一條 TLS 連線上 multiplex.
			var wg sync.WaitGroup
			for slot := range inflight {
				wg.Add(1)
				go func(slot int) {
					defer wg.Done()
					const batchSize = 10
					const flushThreshold = 100
					var localAns, localNoAns, localTimeout, localOther, localProcessed uint64

					q := new(dns.Msg)
					for i := slot; i < b.TotalRequest; i += inflight {
						// Send batching logic (simple here since we have many goroutines)
						if i%(inflight*batchSize) == slot {
							waitCount := batchSize
							if i+waitCount*inflight > b.TotalRequest {
								waitCount = (b.TotalRequest - i + inflight - 1) / inflight
							}
							limiter.WaitN(ctx, waitCount)
						}

						domain := b.Domains[i%domainCount]
						qtype := QType[b.DomainQType[i%len(b.DomainQType)]]

						q.SetQuestion(domain, qtype)
						Result.SendLastTime.Store(time.Since(t1).Nanoseconds())
						Result.SendCount.Add(1)

						resp, err := dotClient.Resolve(q, rdns.ClientInfo{})
						if err != nil {
							if strings.Contains(err.Error(), "timed out") {
								localTimeout++
							} else {
								localOther++
							}
							localProcessed++
						} else {
							Result.RecvLastTime.Store(time.Since(t1).Nanoseconds())
							if len(resp.Answer) > 0 {
								localAns++
							} else {
								localNoAns++
							}
							localProcessed++
						}

						if localProcessed >= flushThreshold {
							Result.RecvAnsCount.Add(localAns)
							Result.RecvNoAnsCount.Add(localNoAns)
							Result.TimeoutCount.Add(localTimeout)
							Result.OtherCount.Add(localOther)
							Result.MaybeSignalDone(expected)
							localAns, localNoAns, localTimeout, localOther, localProcessed = 0, 0, 0, 0, 0
						}
					}
					// Final flush
					Result.RecvAnsCount.Add(localAns)
					Result.RecvNoAnsCount.Add(localNoAns)
					Result.TimeoutCount.Add(localTimeout)
					Result.OtherCount.Add(localOther)
					Result.MaybeSignalDone(expected)
				}(slot)
			}
			wg.Wait()
		}(workerID)
	}

	if watchIdle(DoneChan, b.LastTimeout) {
		StatusChan <- 1
	} else {
		StatusChan <- 0
	}
}
