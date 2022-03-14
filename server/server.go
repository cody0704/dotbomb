package server

import (
	"crypto/tls"
	"log"
	"math/rand"
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
	SendCount     uint64
	RecvCount     uint64
	RecvErrCount  uint64
	StopSockCount uint64
	LastTime      time.Duration
}

var Result StressReport
var StatusChan = make(chan int, 1)
var wg sync.WaitGroup

func (b DoTBomb) Start() {
	server := b.RequestIP + ":" + b.RequestPort
	if !verify.VerifyDoTServer(server) {
		log.Println("Cannot connect to DNS Over TLS Server:", b.RequestIP+":"+b.RequestPort)
		os.Exit(0)
		return
	}

	var timeout *time.Timer
	var lastTimer <-chan time.Time
	if b.LastTimeout > 0 {
		timeout = time.NewTimer(b.LastTimeout)
		defer timeout.Stop()
		lastTimer = timeout.C
	}

	t1 := time.Now()
	go func() {
		for range lastTimer {
			StatusChan <- 1
			Result.LastTime = time.Since(t1)
		}
	}()

	config := tls.Config{
		InsecureSkipVerify: true,
	}

	wg.Add(b.Concurrency)
	var domainCount = len(b.DomainArray)
	for count := 1; count <= b.Concurrency; count++ {
		go func(count int) {
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
				randNo := rand.Intn(domainCount)
				q.SetQuestion(b.DomainArray[randNo]+".", dns.TypeA)
				atomic.AddUint64(&Result.SendCount, 1)
				resp, err := dotClient.Resolve(q, rdns.ClientInfo{})
				if err != nil {
					Result.LastTime = time.Since(t1)
					atomic.AddUint64(&Result.StopSockCount, 1)
					wg.Done()
					break
				}
				Result.LastTime = time.Since(t1)
				timeout.Reset(b.LastTimeout)

				switch len(strings.Split(resp.Answer[0].String(), "\t")) {
				case 5:
					atomic.AddUint64(&Result.RecvCount, 1)
				default:
					atomic.AddUint64(&Result.RecvErrCount, 1)
				}
			}
			wg.Done()
		}(count)
	}
	wg.Wait()
	StatusChan <- 0
}
