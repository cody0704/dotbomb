package stress

import (
	"sync/atomic"
	"testing"

	"github.com/miekg/dns"
)

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
