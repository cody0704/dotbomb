package stress

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/time/rate"
)

// startEchoDNS spins up a local UDP responder that answers every query with a
// single A record, so the recv path has something to count. It returns the
// listen IP/port and a stop func.
func startEchoDNS(t *testing.T) (ip string, port int, stop func()) {
	t.Helper()
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			n, addr, err := pc.ReadFromUDP(buf)
			if err != nil {
				return
			}
			var q dns.Msg
			if q.Unpack(buf[:n]) != nil || len(q.Question) == 0 {
				continue
			}
			reply := new(dns.Msg)
			reply.SetReply(&q)
			reply.Answer = append(reply.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: q.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.IPv4(127, 0, 0, 1),
			})
			if packed, err := reply.Pack(); err == nil {
				pc.WriteToUDP(packed, addr)
			}
		}
	}()
	la := pc.LocalAddr().(*net.UDPAddr)
	return la.IP.String(), la.Port, func() { pc.Close(); close(done) }
}

// TestDNSCompletesOnSmallRun guards the regression where the recv path only
// flushed (and checked completion) every 500 replies: a run smaller than that
// would never signal done and would falsely report a timeout with zero recv
// counts. A correct run finishes well inside LastTimeout with full counts.
func TestDNSCompletesOnSmallRun(t *testing.T) {
	ip, port, stop := startEchoDNS(t)
	defer stop()

	const c, n = 4, 25
	b := Bomb{
		Concurrency:  c,
		TotalRequest: n,
		LastTimeout:  2 * time.Second, // generous: a working run finishes in ms
		Domains:      []string{"example.com."},
		DomainQType:  []string{"A"},
		Expected:     uint64(c * n),
	}

	limiter := rate.NewLimiter(rate.Limit(1_000_000), MaxBatch)
	t1 := time.Now()
	go b.DNS(context.Background(), limiter, ip, port)

	var status int
	select {
	case status = <-StatusChan:
	case <-time.After(5 * time.Second):
		t.Fatal("DNS run never produced a status (deadlock)")
	}

	elapsed := time.Since(t1)
	if status != 0 {
		t.Errorf("status = %d (want 0/Finish); run took %v", status, elapsed)
	}
	if elapsed >= b.LastTimeout {
		t.Errorf("run took %v (>= LastTimeout %v): completion was not detected promptly", elapsed, b.LastTimeout)
	}
	if got := Result.SendCount.Load(); got != c*n {
		t.Errorf("SendCount = %d, want %d", got, c*n)
	}
	if got := Result.Processed(); got != c*n {
		t.Errorf("Processed = %d, want %d", got, c*n)
	}
}
