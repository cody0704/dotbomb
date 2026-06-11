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
	"github.com/miekg/dns"
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
	for lineNo := 1; scanner.Scan(); lineNo++ {
		// Fields tolerates extra/tab whitespace between the domain and qtype.
		parts := strings.Fields(scanner.Text())
		if len(parts) == 0 {
			continue
		}
		if len(parts) < 2 {
			log.Fatalf("%s:%d invalid format (need %q): %q", domainFile, lineNo, "<domain.> <QTYPE>", scanner.Text())
		}

		qtype := strings.ToUpper(parts[1])
		if _, ok := stress.QType[qtype]; !ok {
			log.Fatalf("%s:%d unsupported qtype %q (see README for the supported list)", domainFile, lineNo, parts[1])
		}

		bomb.Domains = append(bomb.Domains, dns.Fqdn(parts[0]))
		bomb.DomainQType = append(bomb.DomainQType, qtype)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	file.Close()

	if len(bomb.Domains) == 0 {
		log.Fatal(domainFile, " does not have any domains")
	}

	log.Println("DoTBomb start stress...")
	log.Printf("Timeout: %ds", timeout)

	log.Println("total request:", concurrency*totalRequest)

	t1 := time.Now() // get current time

	// TPS 限制. burst 必須 >= stress.MaxBatch, 否則 sender 的 WaitN(batchSize) 在
	// n > burst 時立刻回 error, rate limit 整個失效 (TPS 會衝到網卡上限). 同時 >=
	// concurrency 讓多個 worker 不會擠在 limiter 的 mutex 上. 平均速率仍由 rate.Limit 決定.
	var ctx = context.Background()
	limiter := rate.NewLimiter(rate.Limit(interval), max(stress.MaxBatch, concurrency))

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
		fmt.Println("Status:\t\t", "Cancel")
	}

	fmt.Println("======================================================")
	sendCount := report.SendCount.Load()
	sendLast := time.Duration(report.SendLastTime.Load())
	fmt.Println("Send:\t\t", sendCount)
	fmt.Printf("  LastTime:\t %.6fs\n", sendLast.Seconds())
	fmt.Printf("  AvgTime:\t %.6fs\n", safeDiv(sendLast.Seconds(), float64(sendCount)))
	fmt.Printf("  Send TPS:\t %.0f\n", safeDiv(float64(sendCount), sendLast.Seconds()))

	if fakeIF != "" || ignoreResponse {
		return
	}

	recvCount := report.RecvAnsCount.Load() + report.RecvNoAnsCount.Load()
	recvLast := time.Duration(report.RecvLastTime.Load())
	fmt.Println("Recv:\t\t", recvCount)
	fmt.Printf("  LastTime:\t %.6fs\n", recvLast.Seconds())
	fmt.Printf("  AvgTime:\t %.6fs\n", safeDiv(recvLast.Seconds(), float64(recvCount)))
	fmt.Printf("  Recv TPS:\t %.0f\n", safeDiv(float64(recvCount), recvLast.Seconds()))
	fmt.Println("  QType:")
	fmt.Println("    Answer:\t", report.RecvAnsCount.Load())
	fmt.Println("    NoAnswer:\t", report.RecvNoAnsCount.Load())
	fmt.Println("    Timeout:\t", report.TimeoutCount.Load())
	fmt.Println("    Other:\t", report.OtherCount.Load())
}

// safeDiv returns a/b, or 0 when the result would be NaN or ±Inf (b == 0, as
// happens on an empty or instantaneous run). Keeps the report free of "+Inf".
func safeDiv(a, b float64) float64 {
	if v := a / b; !math.IsNaN(v) && !math.IsInf(v, 0) {
		return v
	}
	return 0
}
