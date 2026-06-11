package stress

import (
	"log"
	"net"
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

// MaxBatch is the largest n any protocol passes to rate.Limiter.WaitN.
// main uses it as a burst floor — rate.Limiter.WaitN(n) returns immediately
// with an error when n > burst, which silently bypasses the TPS limit.
// Keep this >= every batchSize constant in dns.go / dnssec.go / dot.go / doh.go.
const MaxBatch = 100

var Result StressReport

// StatusChan capacity is 4 so -m all (DNS+DNSSEC+DoT+DoH, four protocol
// goroutines each writing once) won't block. Single-mode uses 1 of the 4 slots.
var StatusChan = make(chan int, 4)

// DoneChan is closed (via SignalDone) when the run is done. close + sync.Once
// lets multiple protocol goroutines all unblock together — required for
// -m all where four protocol goroutines all wait on the same signal.
var DoneChan = make(chan struct{})
var doneOnce sync.Once

var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 4096)
		return &b
	},
}

func SignalDone() {
	doneOnce.Do(func() { close(DoneChan) })
}

// drainUDPReplies reads DNS replies on conn until the socket errors (closed or
// read failure), tallying answered vs. unanswered into the global Result.
//
// Local counters are flushed to the shared atomics every flushSize reads to keep
// contention off the hot path on large runs. A read deadline of flushInterval
// also forces a flush whenever the socket goes briefly idle, so a run that
// receives fewer than flushSize replies (the common case) still publishes its
// tallies and signals completion promptly instead of stalling until the idle
// watchdog times out. Shared verbatim by plain DNS and DNSSEC.
func drainUDPReplies(conn *net.UDPConn, t1 time.Time, expected uint64) {
	const flushSize = 500
	const flushInterval = 20 * time.Millisecond
	var localAns, localNoAns, localProcessed uint64

	flush := func() {
		Result.RecvAnsCount.Add(localAns)
		Result.RecvNoAnsCount.Add(localNoAns)
		Result.MaybeSignalDone(expected)
		localAns, localNoAns, localProcessed = 0, 0, 0
		conn.SetReadDeadline(time.Now().Add(flushInterval))
	}

	conn.SetReadDeadline(time.Now().Add(flushInterval))
	for {
		bufPtr := bufPool.Get().(*[]byte)
		n, err := conn.Read(*bufPtr)
		if err != nil {
			bufPool.Put(bufPtr)
			// A deadline timeout just means the socket was idle: publish what we
			// have (which may complete the run) and keep waiting for more.
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				flush()
				continue
			}
			log.Println("recv dns err:", err)
			Result.StopSockCount.Add(1)
			break
		}
		Result.RecvLastTime.Store(time.Since(t1).Nanoseconds())

		var reply dns.Msg
		err = reply.Unpack((*bufPtr)[:n])
		bufPool.Put(bufPtr)
		if err != nil {
			log.Println("recv dns msg err:", err)
		}

		if len(reply.Answer) > 0 {
			localAns++
		} else {
			localNoAns++
		}
		localProcessed++

		if localProcessed >= flushSize {
			flush()
		}
	}
	flush()
}

// watchIdle blocks until the run completes (done is closed) or no new outcome
// has been recorded for the idle duration, returning true only on timeout. One
// watchdog per protocol replaces the previous per-goroutine timer.Reset calls,
// which were a data race when many receivers reset the same timer concurrently.
func watchIdle(done <-chan struct{}, idle time.Duration) bool {
	ticker := time.NewTicker(min(idle, 100*time.Millisecond))
	defer ticker.Stop()

	last := Result.Processed()
	lastChange := time.Now()
	for {
		select {
		case <-done:
			return false
		case now := <-ticker.C:
			if p := Result.Processed(); p != last {
				last, lastChange = p, now
			} else if now.Sub(lastChange) >= idle {
				return true
			}
		}
	}
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
