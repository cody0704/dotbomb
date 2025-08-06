package stress

import (
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

type Bomb struct {
	Concurrency  int
	TotalRequest int
	Method       string
	Domains      []string
	DomainQType  []string
	LastTimeout  time.Duration
}

type StressReport struct {
	SendCount      atomic.Uint64
	RecvAnsCount   atomic.Uint64
	RecvNoAnsCount atomic.Uint64
	TimeoutCount   atomic.Uint64
	OtherCount     atomic.Uint64
	StopSockCount  atomic.Uint64

	RecvLastTime time.Duration
	SendLastTime time.Duration
}

var Result StressReport
var StatusChan = make(chan int, 1)

var QType map[string]uint16 = map[string]uint16{
	"A":      dns.TypeA,
	"AAAA":   dns.TypeAAAA,
	"CNAME":  dns.TypeCNAME,
	"MX":     dns.TypeMX,
	"NS":     dns.TypeNS,
	"TXT":    dns.TypeTXT,
	"SRV":    dns.TypeSRV,
	"PTR":    dns.TypePTR,
	"SOA":    dns.TypeSOA,
	"DNSKEY": dns.TypeDNSKEY,
	"DS":     dns.TypeDS,
	"CAA":    dns.TypeCAA,
	"NAPTR":  dns.TypeNAPTR,
	"TLSA":   dns.TypeTLSA,
	"SPF":    dns.TypeSPF,
	"ANY":    dns.TypeANY,
}
