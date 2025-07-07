package main

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cody0704/dotbomb/pkg/stress"
)

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var bomb = stress.Bomb{
		Concurrency:  concurrency,
		TotalRequest: totalRequest,
		Server:       requestIP + ":" + requestPort,
		LastTimeout:  time.Second * time.Duration(timeout),
		Latency:      time.Microsecond * time.Duration(latency),
		Interval:     interval,
	}

	file, err := os.Open(domainFile)
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domain := scanner.Text()
		if domain == "" {
			continue
		}
		bomb.DomainArray = append(bomb.DomainArray, domain)
	}

	if len(bomb.DomainArray) == 0 {
		log.Fatal(domainFile, " does not have any domains")
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	file.Close()

	log.Println("DoTBomb start stress...")
	log.Printf("Timeout: %ds", timeout)
	log.Printf("Latency: %.6gs", bomb.Latency.Seconds())

	log.Println("total request:", concurrency*totalRequest)

	t1 := time.Now() // get current time

	switch mode {
	case "dns":
		log.Println("Mode:", mode)
		if err := bomb.VerifyDNS(); err != nil {
			log.Println("Cannot connect to DNS Server:", err)
			os.Exit(0)
			return
		}
		log.Println("DNS Server:", bomb.Server)

		go bomb.DNS()
	case "dot":
		log.Println("Mode:", mode)
		if !bomb.VerifyDoT() {
			log.Println("Cannot connect to DoT Server:", bomb.Server)
			os.Exit(0)
			return
		}
		log.Println("DoT Server:", bomb.Server)

		go bomb.DoT()
	case "doh":
		log.Println("Mode:", mode, "Method: POST")
		bomb.Server = "https://" + bomb.Server + "/dns-query{?dns}"
		bomb.Method = "POST"
		if !bomb.VerifyDoH() {
			log.Println("Cannot connect to DoH Server:", bomb.Server)
			os.Exit(0)
			return
		}
		log.Println("DoH Server:", bomb.Server)

		go bomb.DoH()
	case "dohp":
		log.Println("Mode:", mode, "Method: POST")
		bomb.Server = "https://" + bomb.Server + "/dns-query{?dns}"
		bomb.Method = "POST"
		if !bomb.VerifyDoH() {
			log.Println("Cannot connect to DoH Server:", bomb.Server)
			os.Exit(0)
			return
		}
		log.Println("DoH Server:", bomb.Server)

		go bomb.DoH()
	case "dohg":
		log.Println("Mode:", mode, "Method: GET")
		bomb.Server = "https://" + bomb.Server + "/dns-query{?dns}"
		bomb.Method = "GET"
		if !bomb.VerifyDoH() {
			log.Println("Cannot connect to DoH Server:", bomb.Server)
			os.Exit(0)
			return
		}
		log.Println("DoH Server:", bomb.Server)

		go bomb.DoH()
	}

	select {
	case <-sigChan:
		report(t1, stress.Result, 2)
	case status := <-stress.StatusChan:
		report(t1, stress.Result, status)
	}
}

func report(t1 time.Time, report stress.StressReport, status int) {
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
	fmt.Println("Send:\t\t", report.SendCount)
	fmt.Printf("  LastTime:\t %.6fs\n", report.SendLastTime.Seconds())
	fmt.Printf("  AvgTime:\t %.6fs\n", report.SendLastTime.Seconds()/float64(report.SendCount))
	fmt.Printf("  Send TPS:\t %.0f\n", float64(report.SendCount)/report.SendLastTime.Seconds())

	recvCount := report.RecvAnsCount + report.RecvNoAnsCount
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
	fmt.Println("    Answer:\t", report.RecvAnsCount)
	fmt.Println("    NoAnswer:\t", report.RecvNoAnsCount)
	fmt.Println("    Timeout:\t", report.TimeoutCount)
	fmt.Println("    Other:\t", report.OtherCount)
}
