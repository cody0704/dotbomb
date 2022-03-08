package server

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/cody0704/dotbomb/server/resolver"
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
}

var stressChannel = make(chan string)

func (b DoTBomb) Start() (chan string, error) {
	if !verify.VerifyDoTServer("8.8.8.8:853") {
		return nil, errors.New("DoT Server can't connect")
	}

	var totalDelayTime time.Duration
	var domainCount = len(b.DomainArray)
	for count := 1; count <= b.Concurrency; count++ {
		go func(count int) {
			// Build a query
			q := new(dns.Msg)
			dotClient := resolver.DotClient(b.RequestIP + ":" + b.RequestPort)
			for i := 0; i < b.TotalRequest; i++ {
				t1 := time.Now() // get current time
				q.SetQuestion(b.DomainArray[rand.Intn(domainCount)]+".", dns.TypeA)
				_, err := dotClient.Resolve(q, rdns.ClientInfo{})
				if err != nil {
					stressChannel <- fmt.Sprintf("%d false", count)
				}

				elapsed := time.Since(t1)
				stressChannel <- fmt.Sprintf("%d true", count)
				totalDelayTime += elapsed
				stressChannel <- fmt.Sprintf("%d delay", totalDelayTime)
			}
		}(count)
	}

	return stressChannel, nil
}
