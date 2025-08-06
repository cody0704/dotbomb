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

	"github.com/cody0704/dotbomb/pkg/stress"
	"golang.org/x/time/rate"
)

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var bomb = stress.Bomb{
		Concurrency:  concurrency,
		TotalRequest: totalRequest,
		LastTimeout:  time.Second * time.Duration(timeout),
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

	// TPS 限制
	var ctx = context.Background()
	limiter := rate.NewLimiter(rate.Limit(interval), 1)

	switch mode {
	case "all":
		log.Println("Mode:", mode)

		go bomb.DNSSEC(ctx, limiter, requestIP, 53)
		go bomb.DNS(ctx, limiter, requestIP, 53)
		go bomb.DoT(ctx, limiter, requestIP, 853)
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
		server := fmt.Sprintf("https://%s:%d/dns-query{?dns}", requestIP, requestPort)
		log.Println("DoH Server: ", server)
		bomb.Method = "POST"

		go bomb.DoH(ctx, limiter, server)
	case "dohg":
		log.Println("Mode:", mode, "Method: GET")
		server := fmt.Sprintf("https://%s:%d/dns-query", requestIP, requestPort)
		log.Println("DoH Server: ", server)
		bomb.Method = "GET"

		go bomb.DoH(ctx, limiter, server)
	}

	select {
	case <-sigChan:
		report(t1, &stress.Result, 2)
	case status := <-stress.StatusChan:
		report(t1, &stress.Result, status)
	}
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
	fmt.Println("Send:\t\t", report.SendCount.Load())
	fmt.Printf("  LastTime:\t %.6fs\n", report.SendLastTime.Seconds())
	fmt.Printf("  AvgTime:\t %.6fs\n", report.SendLastTime.Seconds()/float64(report.SendCount.Load()))
	fmt.Printf("  Send TPS:\t %.0f\n", float64(report.SendCount.Load())/report.SendLastTime.Seconds())

	recvCount := report.RecvAnsCount.Load() + report.RecvNoAnsCount.Load()
	fmt.Println("Recv:\t\t", recvCount)
	fmt.Printf("  LastTime:\t %.6fs\n", report.RecvLastTime.Seconds())
	recvAvgTime := report.RecvLastTime.Seconds() / float64(recvCount)
	if math.IsNaN(recvAvgTime) || math.IsInf(recvAvgTime, 0) {
		fmt.Println("  AvgTime:\t 0.000000s")
	} else {
		fmt.Printf("  AvgTime:\t %.6fs\n", recvAvgTime)
	}
	fmt.Printf("  Recv TPS:\t %.0f\n", float64(recvCount)/report.RecvLastTime.Seconds())
	fmt.Println("  QType:")
	fmt.Println("    Answer:\t", report.RecvAnsCount.Load())
	fmt.Println("    NoAnswer:\t", report.RecvNoAnsCount.Load())
	fmt.Println("    Timeout:\t", report.TimeoutCount.Load())
	fmt.Println("    Other:\t", report.OtherCount.Load())
}
