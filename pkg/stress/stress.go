package stress

import (
	"sync"
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
	// Expected is the total number of recv-side outcomes the run waits for
	// before SignalDone fires. Single-mode = Concurrency*TotalRequest;
	// -m all = 4 * Concurrency * TotalRequest (DNS+DNSSEC+DoT+DoH share the
	// singleton Result counters). Set by main before launching any protocol.
	Expected uint64
	// Inflight is the number of concurrent in-flight queries per worker for
	// DoT/DoH. Default 1 (sequential). Higher values pipeline on the same
	// routedns client connection — DoT pipelines through its request channel,
	// DoH uses http.Client which auto-multiplexes via HTTP/2.
	Inflight int
	// IgnoreResponse skips the recv goroutine and finishes on send completion.
	// Use case: traffic is being tapped/mirrored to the target, so no reply
	// will ever come back on this socket.
	IgnoreResponse bool
	FakeIF         string // interface for fake
	FakeIP         string // for dns - class b
	FakeSourceMac  string // for dns - class b
	FakeTargetMac  string // for dns - class b
}

type StressReport struct {
	SendCount      atomic.Uint64
	RecvAnsCount   atomic.Uint64
	RecvNoAnsCount atomic.Uint64
	TimeoutCount   atomic.Uint64
	OtherCount     atomic.Uint64
	StopSockCount  atomic.Uint64

	// Last-operation timestamps relative to run start, stored as nanoseconds.
	// Atomic because many sender/receiver goroutines write concurrently;
	// 64-bit non-atomic writes are not guaranteed torn-free by the Go memory model.
	RecvLastTime atomic.Int64
	SendLastTime atomic.Int64
}

// Processed returns the total number of queries the recv path has finalized
// (answered, no-answer, timed out, or otherwise errored).
func (r *StressReport) Processed() uint64 {
	return r.RecvAnsCount.Load() + r.RecvNoAnsCount.Load() + r.TimeoutCount.Load() + r.OtherCount.Load()
}

// MaybeSignalDone fires DoneChan if the processed count has reached expected.
// Safe to call from any goroutine; signals are coalesced.
func (r *StressReport) MaybeSignalDone(expected uint64) {
	if r.Processed() >= expected {
		SignalDone()
	}
}

var Result StressReport

// StatusChan capacity is 4 so -m all (DNS+DNSSEC+DoT+DoH, four protocol
// goroutines each writing once) won't block. Single-mode uses 1 of the 4 slots.
var StatusChan = make(chan int, 4)

// DoneChan is closed (via SignalDone) when the run is done. close + sync.Once
// lets multiple protocol goroutines all unblock together — required for
// -m all where four protocol goroutines all wait on the same signal.
var DoneChan = make(chan struct{})
var doneOnce sync.Once

func SignalDone() {
	doneOnce.Do(func() { close(DoneChan) })
}

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

