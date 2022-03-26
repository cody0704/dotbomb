package server

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cody0704/dotbomb/server/verify"

	rdns "github.com/folbricht/routedns"
	"github.com/miekg/dns"
)

func (b Bomb) DNS() {
	server := b.RequestIP + ":" + b.RequestPort
	if !verify.DNSServer(server) {
		log.Println("Cannot connect to DNS Server:", server)
		os.Exit(0)
		return
	}
	log.Println("DNS Server:", server)

	t1 := time.Now()

	wg.Add(b.Concurrency)
	var domainCount = len(b.DomainArray)
	finish := b.TotalRequest * b.Concurrency
	for count := 1; count <= b.Concurrency; count++ {
		go func(count, finish int) {
			// Build a query
			q := new(dns.Msg)

			// Resolve the query
			dnsClient, err := rdns.NewDNSClient("stress-dns-"+strconv.Itoa(count), server, "udp", rdns.DNSClientOptions{})
			if err != nil {
				log.Println(err)
				wg.Done()
				return
			}

			for i := 0; i < b.TotalRequest; i++ {
				domain := b.DomainArray[i%domainCount] + "."

				q.SetQuestion(domain, dns.TypeA)
				atomic.AddUint64(&Result.SendCount, 1)
				fmt.Printf("Progress:\t%d/%d\r", Result.SendCount, finish)
				resp, err := dnsClient.Resolve(q, rdns.ClientInfo{})
				if err != nil {
					if strings.Contains(err.Error(), "timed out") {
						atomic.AddUint64(&Result.TimeoutCount, 1)
					} else {
						atomic.AddUint64(&Result.OtherCount, 1)
					}
					continue
				}
				Result.LastTime = time.Since(t1)

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

			}
			wg.Done()
		}(count, finish)
	}
	wg.Wait()
	StatusChan <- 0
}
