package stress

import (
	"sync"
	"time"
)

type Bomb struct {
	Concurrency  int
	TotalRequest int
	Server       string
	Method       string
	DomainArray  []string
	Latency      time.Duration
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
var wg sync.WaitGroup
