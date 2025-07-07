package stress

import (
	"context"
	"log"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	rdns "github.com/folbricht/routedns"
	"github.com/miekg/dns"
	"golang.org/x/time/rate"
)

func (b *Bomb) DNS() {
	var timeout *time.Timer = time.NewTimer(b.LastTimeout)
	defer timeout.Stop()

	// TPS 限制
	var ctx = context.Background()
	limiter := rate.NewLimiter(rate.Limit(b.Interval), 1)

	var domainCount = len(b.DomainArray)
	finish := b.TotalRequest * b.Concurrency

	t1 := time.Now() // get current time
	for count := 1; count <= b.Concurrency; count++ {
		go func(count, finish int) {
			// Build a query
			q := new(dns.Msg)

			// Resolve the query
			dnsClient, err := rdns.NewDNSClient("stress-dns-"+strconv.Itoa(count), b.Server, "udp", rdns.DNSClientOptions{
				QueryTimeout: b.LastTimeout,
			})
			if err != nil {
				pc, file, line, ok := runtime.Caller(1)
				if ok {
					fn := runtime.FuncForPC(pc)
					log.Printf("[ERROR] %s:%d [%s] %v", file, line, fn.Name(), err)
				}
				return
			}

			for i := range b.TotalRequest {
				domain := b.DomainArray[i%domainCount]

				q.SetQuestion(domain, dns.TypeA)
				Result.SendLastTime = time.Since(t1)
				atomic.AddUint64(&Result.SendCount, 1)

				resp, err := dnsClient.Resolve(q, rdns.ClientInfo{})
				if err != nil {
					if strings.Contains(err.Error(), "timed out") {
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

				limiter.Wait(ctx)
			}
		}(count, finish)
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

func (b Bomb) VerifyDNS() error {
	// Resolve the query
	r, err := rdns.NewDNSClient("test-dns", b.Server, "udp", rdns.DNSClientOptions{
		QueryTimeout: b.LastTimeout,
	})
	if err != nil {
		return err
	}

	// Build a query
	q := new(dns.Msg)
	q.SetQuestion(b.DomainArray[0], dns.TypeA)

	if _, err = r.Resolve(q, rdns.ClientInfo{}); err != nil {
		return err
	}

	return nil
}
