package stress

import (
	"encoding/binary"
	"net"
	"sync/atomic"
	"testing"

	"github.com/miekg/dns"
)

// buildReply returns a packed DNS response carrying one A answer — the shape the
// recv path tallies.
func buildReply() []byte {
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	m.Response = true
	m.Answer = append(m.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
		A:   net.IPv4(1, 2, 3, 4),
	})
	b, _ := m.Pack()
	return b
}

// BenchmarkRecvUnpack is the old recv-side cost: a full Unpack per reply just to
// check whether it had an answer.
func BenchmarkRecvUnpack(b *testing.B) {
	reply := buildReply()
	var ans int
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var m dns.Msg
		_ = m.Unpack(reply)
		if len(m.Answer) > 0 {
			ans++
		}
	}
	_ = ans
}

// BenchmarkRecvAncount is the new recv-side cost: read the ANCOUNT field straight
// from the header (no parse, no allocation).
func BenchmarkRecvAncount(b *testing.B) {
	reply := buildReply()
	var ans int
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if len(reply) >= headerLen && binary.BigEndian.Uint16(reply[6:8]) > 0 {
			ans++
		}
	}
	_ = ans
}

// BenchmarkOldWay simulates the original loop: packing every time and atomic update every time
func BenchmarkOldWay(b *testing.B) {
	q := new(dns.Msg)
	q.SetQuestion("example.com.", dns.TypeA)
	var count atomic.Uint64

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulation of Pack()
		_, _ = q.Pack()
		// Simulation of atomic update per request
		count.Add(1)
	}
}

// BenchmarkNewWay simulates the optimized loop: pre-packed and batched atomic updates
func BenchmarkNewWay(b *testing.B) {
	q := new(dns.Msg)
	q.SetQuestion("example.com.", dns.TypeA)
	prePacked, _ := q.Pack()
	var count atomic.Uint64

	const batchSize = 100

	b.ResetTimer()
	for i := 0; i < b.N; i += batchSize {
		iterations := batchSize
		if i+batchSize > b.N {
			iterations = b.N - i
		}

		for j := 0; j < iterations; j++ {
			// Just use the pre-packed slice
			_ = prePacked
		}
		// Batched atomic update
		count.Add(uint64(iterations))
	}
}

// BenchmarkBufferAlloc measures the cost of allocating a buffer vs sync.Pool
func BenchmarkBufferAlloc(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := make([]byte, 4096)
		_ = buf
	}
}

func BenchmarkBufferPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bufPtr := bufPool.Get().(*[]byte)
		_ = *bufPtr
		bufPool.Put(bufPtr)
	}
}
