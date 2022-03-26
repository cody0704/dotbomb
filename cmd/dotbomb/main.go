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

	"github.com/cody0704/dotbomb/server"
)

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var bomb = server.Bomb{
		Concurrency:  concurrency,
		TotalRequest: totalRequest,
		RequestIP:    requestIP,
		RequestPort:  requestPort,
		LastTimeout:  time.Second * time.Duration(timeout),
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
	log.Println("Mode:", mode)

	switch mode {
	case "dns":
		go bomb.DNS()
	case "dot":
		go bomb.DoT()
	case "doh":
		go bomb.DoH()
	}

	select {
	case <-sigChan:
		report(server.Result, 1)
	case status := <-server.StatusChan:
		report(server.Result, status)
	}
}

func report(report server.StressReport, status int) {
	switch status {
	case 0:
		fmt.Println("\n\nStatus:\t\t", "Finish")
	case 1:
		fmt.Println("\nStatus:\t\t", "Cancle")
	}
	fmt.Printf("Finish Time:\t %.6fs\n", report.LastTime.Seconds())
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
