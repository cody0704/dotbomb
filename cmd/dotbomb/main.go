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
		Timeout:      time.Second * time.Duration(timeout),
		Latency:      time.Microsecond * time.Duration(latency),
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

	switch mode {
	case "dns":
		log.Println("Mode:", mode)
		if !bomb.VerifyDNS() {
			log.Println("Cannot connect to DNS Server:", bomb.Server)
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
		report(stress.Result, 1)
	case status := <-stress.StatusChan:
		report(stress.Result, status)
	}
}

func report(report stress.StressReport, status int) {
	switch status {
	case 0:
		fmt.Println("\n\nStatus:\t\t", "Finish")
	case 1:
		fmt.Println("\nStatus:\t\t", "Cancle")
	}
	fmt.Printf("Time:\t\t %.6fs\n", report.LastTime.Seconds())
	totalResponse := report.RecvAnsCount + report.RecvNoAnsCount
	avgLantency := report.LastTime.Seconds() / float64(totalResponse)
	if math.IsInf(avgLantency, 0) || math.IsNaN(avgLantency) {
		fmt.Println("Avg Latency:\t 0.000000s")
	} else {
		fmt.Printf("Avg Latency:\t %.6fs\n", report.LastTime.Seconds()/float64(totalResponse))
	}
	fmt.Println("==========================================")
	fmt.Println("Send:\t\t", report.SendCount)
	fmt.Println("Recv:\t\t", report.RecvAnsCount+report.RecvNoAnsCount+report.TimeoutCount+report.OtherCount)
	fmt.Println("  Answer:\t", report.RecvAnsCount)
	fmt.Println("  NoAnswer:\t", report.RecvNoAnsCount)
	fmt.Println("  Timeout:\t", report.TimeoutCount)
	fmt.Println("  Other:\t", report.OtherCount)
}
