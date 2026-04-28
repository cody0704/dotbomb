package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/acom-networks/dnsbomb/pkg/stress"
	"golang.org/x/time/rate"
)

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var bomb = stress.Bomb{
		Concurrency:    concurrency,
		TotalRequest:   totalRequest,
		LastTimeout:    time.Second * time.Duration(timeout),
		IgnoreResponse: ignoreResponse,
		Inflight:       inflight,
		// Fake
		FakeIF:        fakeIF,
		FakeIP:        fakeIP,
		FakeSourceMac: fakeSourceMac,
		FakeTargetMac: fakeTargetMac,
	}

	file, err := os.Open(domainFile)
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Split(line, " ")
		if len(parts) < 2 {
			log.Fatal("Invalid domain format in", domainFile, ":", line)
		}

		bomb.Domains = append(bomb.Domains, parts[0])
		bomb.DomainQType = append(bomb.DomainQType, parts[1])
	}

	if len(bomb.Domains) == 0 {
		log.Fatal(domainFile, " does not have any domains")
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	file.Close()

	log.Println("DoTBomb start stress...")
	log.Printf("Timeout: %ds", timeout)

	log.Println("total request:", concurrency*totalRequest)

	t1 := time.Now() // get current time

	// TPS 限制. burst = concurrency 讓 worker 不會在 limiter mutex 上排隊;
	// 平均速率仍由 rate.Limit 決定.
	var ctx = context.Background()
	limiter := rate.NewLimiter(rate.Limit(interval), max(1, concurrency))

	// Expected 是 recv 端要累積的「成果數」(Ans/NoAns/Timeout/Other 加總),
	// 同時也是 SignalDone 的觸發門檻. 單 mode = C*T; -m all 因為四個 protocol
	// 都把計數寫進同一個 singleton Result, 所以 Expected = 4*C*T.
	statusN := 1
	if mode == "all" {
		statusN = 4
	}
	bomb.Expected = uint64(statusN * concurrency * totalRequest)

	switch mode {
	case "all":
		log.Println("Mode:", mode, "(DNS + DNSSEC + DoT + DoH)")
		log.Printf("DNS: %s:53, DNSSEC: %s:53, DoT: %s:853", requestIP, requestIP, requestIP)
		dohServer := fmt.Sprintf("https://%s:443/dns-query", requestIP)
		log.Println("DoH:", dohServer, "(POST)")
		bomb.Method = "POST"

		go bomb.DNS(ctx, limiter, requestIP, 53)
		go bomb.DNSSEC(ctx, limiter, requestIP, 53)
		go bomb.DoT(ctx, limiter, requestIP, 853)
		go bomb.DoH(ctx, limiter, dohServer)
	case "dnssec":
		log.Println("Mode:", mode)
		log.Printf("DNS Server: %s:%d", requestIP, requestPort)

		go bomb.DNSSEC(ctx, limiter, requestIP, requestPort)
	case "dns":
		log.Println("Mode:", mode)
		log.Printf("DNS Server: %s:%d", requestIP, requestPort)

		go bomb.DNS(ctx, limiter, requestIP, requestPort)
	case "dot":
		log.Println("Mode:", mode)
		log.Printf("DNS Server: %s:%d", requestIP, requestPort)

		go bomb.DoT(ctx, limiter, requestIP, requestPort)
	case "doh":
		log.Println("Mode:", mode, "Method: POST")
		server := fmt.Sprintf("https://%s:%d/dns-query", requestIP, requestPort)
		log.Println("DoH Server: ", server)
		bomb.Method = "POST"

		go bomb.DoH(ctx, limiter, server)
	case "dohg":
		log.Println("Mode:", mode, "Method: GET")
		// routedns DoH GET 要求 URI template — {?dns} 會被展開成 ?dns=<base64>.
		server := fmt.Sprintf("https://%s:%d/dns-query{?dns}", requestIP, requestPort)
		log.Println("DoH Server: ", server)
		bomb.Method = "GET"

		go bomb.DoH(ctx, limiter, server)
	}

	// 收 N 個 status; 取最差 (timeout > finish) 當整體狀態.
	combined := 0
	for range statusN {
		select {
		case <-sigChan:
			report(t1, &stress.Result, 2)
			return
		case s := <-stress.StatusChan:
			if s > combined {
				combined = s
			}
		}
	}
	report(t1, &stress.Result, combined)
}

func report(t1 time.Time, report *stress.StressReport, status int) {
	elapsed := time.Since(t1)
	fmt.Printf("\nRun Time:\t %.6fs\n", elapsed.Seconds())
	fmt.Println("Concurrency:\t", concurrency)
	switch status {
	case 0:
		fmt.Println("Status:\t\t", "Finish")
	case 1:
		fmt.Println("Status:\t\t", "Timeout")
	case 2:
		fmt.Println("Status:\t\t", "Cancle")
	}

	fmt.Println("======================================================")
	sendCount := report.SendCount.Load()
	sendLast := time.Duration(report.SendLastTime.Load())
	fmt.Println("Send:\t\t", sendCount)
	fmt.Printf("  LastTime:\t %.6fs\n", sendLast.Seconds())
	sendAvgTime := sendLast.Seconds() / float64(sendCount)
	if math.IsNaN(sendAvgTime) || math.IsInf(sendAvgTime, 0) {
		fmt.Println("  AvgTime:\t 0.000000s")
	} else {
		fmt.Printf("  AvgTime:\t %.6fs\n", sendAvgTime)
	}
	fmt.Printf("  Send TPS:\t %.0f\n", float64(sendCount)/sendLast.Seconds())

	if fakeIF != "" || ignoreResponse {
		return
	}

	recvCount := report.RecvAnsCount.Load() + report.RecvNoAnsCount.Load()
	recvLast := time.Duration(report.RecvLastTime.Load())
	fmt.Println("Recv:\t\t", recvCount)
	fmt.Printf("  LastTime:\t %.6fs\n", recvLast.Seconds())
	recvAvgTime := recvLast.Seconds() / float64(recvCount)
	if math.IsNaN(recvAvgTime) || math.IsInf(recvAvgTime, 0) {
		fmt.Println("  AvgTime:\t 0.000000s")
	} else {
		fmt.Printf("  AvgTime:\t %.6fs\n", recvAvgTime)
	}
	fmt.Printf("  Recv TPS:\t %.0f\n", float64(recvCount)/recvLast.Seconds())
	fmt.Println("  QType:")
	fmt.Println("    Answer:\t", report.RecvAnsCount.Load())
	fmt.Println("    NoAnswer:\t", report.RecvNoAnsCount.Load())
	fmt.Println("    Timeout:\t", report.TimeoutCount.Load())
	fmt.Println("    Other:\t", report.OtherCount.Load())
}
