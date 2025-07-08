package stress

import (
	"time"

	"github.com/miekg/dns"
)

type Bomb struct {
	Concurrency  int
	TotalRequest int
	Server       string
	Method       string
	Domains      []string
	DomainQType  []string
	LastTimeout  time.Duration

	Interval int
}

type StressReport struct {
	SendCount      uint64
	RecvAnsCount   uint64
	RecvNoAnsCount uint64
	TimeoutCount   uint64
	OtherCount     uint64
	StopSockCount  uint64

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
