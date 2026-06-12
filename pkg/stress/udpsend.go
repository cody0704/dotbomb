package stress

import (
	"net"

	"golang.org/x/net/ipv4"
)

// batchSender sends a group of connected-UDP packets, batching where the platform
// allows it — no build tags required, because ipv4.WriteBatch is cross-platform.
//
// The catch (and why the loop below exists): on Linux WriteBatch issues one
// sendmmsg syscall for the whole group, but on every other platform it documents
// that it "will write only a single message" — it sends ms[0] and returns 1. So we
// loop, advancing by the returned count, which sends the whole batch everywhere:
// one syscall per batch on Linux, one packet per iteration elsewhere.
//
// The socket is connected (net.DialUDP), so each message leaves Addr nil. msgs and
// its one-element Buffers are reused across calls to stay allocation-free. Any
// WriteBatch error falls back to plain conn.Write for the remainder, so a platform
// where the batch path misbehaves still degrades to the ordinary send.
type batchSender struct {
	conn *net.UDPConn
	pc   *ipv4.PacketConn
	msgs []ipv4.Message
}

func newBatchSender(conn *net.UDPConn) *batchSender {
	return &batchSender{conn: conn, pc: ipv4.NewPacketConn(conn)}
}

func (s *batchSender) send(packets [][]byte) {
	if cap(s.msgs) < len(packets) {
		s.msgs = make([]ipv4.Message, len(packets))
		for i := range s.msgs {
			s.msgs[i].Buffers = make([][]byte, 1)
		}
	}
	s.msgs = s.msgs[:len(packets)]
	for i, p := range packets {
		s.msgs[i].Buffers[0] = p
		s.msgs[i].Addr = nil
	}

	for off := 0; off < len(s.msgs); {
		n, err := s.pc.WriteBatch(s.msgs[off:], 0)
		if n > 0 {
			off += n
		}
		if err != nil {
			// Best-effort fallback for the remainder (a stress tool tolerates the
			// occasional drop, same as the plain Write path always did).
			for _, m := range s.msgs[off:] {
				s.conn.Write(m.Buffers[0])
			}
			return
		}
		if n <= 0 {
			return
		}
	}
}
