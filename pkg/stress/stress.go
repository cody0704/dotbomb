package stress

import (
	"context"
	"encoding/binary"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"
	"golang.org/x/time/rate"
)

// headerLen is the fixed size of a DNS message header (RFC 1035 §4.1.1).
const headerLen = 12

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

func SignalDone() {
	doneOnce.Do(func() { close(DoneChan) })
}

// drainUDPReplies reads DNS replies on conn until the socket errors (closed or
// read failure), tallying answered vs. unanswered into the global Result.
//
// Reads are batched via ipv4.ReadBatch — on Linux that is one recvmmsg syscall per
// up-to-recvBatch packets; other platforms read one packet per call (same
// cross-platform shape as the send side, no build tags). For each reply it reads
// the ANCOUNT field (bytes 6-7) directly rather than a full Unpack, which would
// allocate the whole RR set on every packet just to be discarded.
//
// Local counters are flushed to the shared atomics every flushSize reads to keep
// contention off the hot path on large runs. A read deadline of flushInterval also
// forces a flush whenever the socket goes briefly idle, so a run that receives fewer
// than flushSize replies (the common case) still publishes its tallies and signals
// completion promptly instead of stalling until the idle watchdog times out. Shared
// verbatim by plain DNS and DNSSEC.
func drainUDPReplies(conn *net.UDPConn, t1 time.Time, expected uint64) {
	const flushSize = 500
	const flushInterval = 20 * time.Millisecond
	const recvBatch = 32

	pc := ipv4.NewPacketConn(conn)
	msgs := make([]ipv4.Message, recvBatch)
	for i := range msgs {
		msgs[i].Buffers = [][]byte{make([]byte, 4096)}
	}

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
		n, err := pc.ReadBatch(msgs, 0)
		if n > 0 {
			Result.RecvLastTime.Store(time.Since(t1).Nanoseconds())
			for i := range n {
				buf, sz := msgs[i].Buffers[0], msgs[i].N
				if sz >= headerLen && binary.BigEndian.Uint16(buf[6:8]) > 0 {
					localAns++
				} else {
					localNoAns++
				}
				localProcessed++
			}
			if localProcessed >= flushSize {
				flush()
			}
		}
		if err != nil {
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
	}
	flush()
}

// sendUDPBatched sends totalRequest pre-packed queries on conn, cycling through
// prePacked and rate-limited by limiter. The batch size is limiter.Burst() so each
// WaitN(count) request never exceeds the burst (which would make the limiter return
// an error and bypass the limit). Pacing happens before the send so completion in
// send-driven mode reflects the rate-limited timeline (see -timeout). Each batch is
// handed to a batchSender: on Linux that is one sendmmsg syscall, elsewhere a
// per-packet write fallback. When sendDriven is true it fires SignalDone once all
// sends are counted (the -ignore / fake case, where no reply path runs).
func sendUDPBatched(ctx context.Context, conn *net.UDPConn, limiter *rate.Limiter, prePacked [][]byte, totalRequest int, t1 time.Time, expected uint64, sendDriven bool) {
	batchSize := limiter.Burst()
	domainCount := len(prePacked)
	sender := newBatchSender(conn)
	batch := make([][]byte, 0, batchSize)

	for i := 0; i < totalRequest; i += batchSize {
		count := batchSize
		if i+count > totalRequest {
			count = totalRequest - i
		}
		limiter.WaitN(ctx, count) // pace before sending

		batch = batch[:0]
		for j := 0; j < count; j++ {
			batch = append(batch, prePacked[(i+j)%domainCount])
		}
		sender.send(batch)

		Result.SendLastTime.Store(time.Since(t1).Nanoseconds())
		if Result.SendCount.Add(uint64(count)) >= expected && sendDriven {
			SignalDone()
		}
	}
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
