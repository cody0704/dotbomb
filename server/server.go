package server

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cody0704/dotbomb/server/verify"

	rdns "github.com/folbricht/routedns"
	"github.com/miekg/dns"
)

type DoTBomb struct {
	Concurrency  int
	TotalRequest int
	RequestIP    string
	RequestPort  string
	DomainArray  []string
	LastTimeout  time.Duration
}

type StressReport struct {
	SendCount      uint64
	RecvAnsCount   uint64
	RecvNoAnsCount uint64
	TimeoutCount   uint64
	OtherCount     uint64
	StopSockCount  uint64
	LastTime       time.Duration
}

var Result StressReport
var StatusChan = make(chan int, 1)
var wg sync.WaitGroup

func (b DoTBomb) Start() {
	server := b.RequestIP + ":" + b.RequestPort
	if !verify.VerifyDoTServer(server) {
		log.Println("Cannot connect to DNS Over TLS Server:", server)
		os.Exit(0)
		return
	}
	log.Println("DNS Over TLS Server:", server)

	t1 := time.Now()

	config := tls.Config{
		InsecureSkipVerify: true,
	}

	wg.Add(b.Concurrency)
	var domainCount = len(b.DomainArray)
	finish := b.TotalRequest * b.Concurrency
	for count := 1; count <= b.Concurrency; count++ {
		go func(count, finish int) {
			// Build a query
			q := new(dns.Msg)

			// Resolve the query
			dotClient, err := rdns.NewDoTClient("stress-dns-"+strconv.Itoa(count), server, rdns.DoTClientOptions{TLSConfig: &config})
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
				resp, err := dotClient.Resolve(q, rdns.ClientInfo{})
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
