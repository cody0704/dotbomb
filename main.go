package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cody0704/dotbomb/server"
)

var (
	versionID    string = "%VERSION%"
	version      bool
	timeout      int
	concurrency  int
	totalRequest int
	requestIP    string
	requestPort  string
	domainFile   string
)

func init() {
	flag.BoolVar(&version, "v", false, "number of concurrency")
	flag.IntVar(&timeout, "t", 3, "RecvTimeout")
	flag.IntVar(&concurrency, "c", 1, "number of concurrency")
	flag.IntVar(&totalRequest, "n", 1, "number of request")
	flag.StringVar(&requestIP, "r", "", "request ip address")
	flag.StringVar(&requestPort, "p", "853", "request port")
	flag.StringVar(&domainFile, "f", "", "domain list file")

	flag.Parse()

	if version {
		fmt.Println(versionID)
		os.Exit(0)
	}

	if concurrency == 0 || totalRequest == 0 || requestIP == "" {
		fmt.Println("Example: ./dotbomb -c 10 -n 100 -r 8.8.8.8 -f domains.txt")
		fmt.Println("Example: ./dotbomb -c 10 -n 100 -r 8.8.8.8 -p 853 -f domains.txt")
		fmt.Println("-c [Concurrency] <Number>")
		fmt.Println("-n [request] <Number>")
		fmt.Println("-r <DNS Over TLS Server IP>")
		fmt.Println("-p <Default Port 853>")

		flag.Usage()
		os.Exit(0)
	}
}

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var dotbomb = server.DoTBomb{
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
		dotbomb.DomainArray = append(dotbomb.DomainArray, domain)
	}

	if len(dotbomb.DomainArray) == 0 {
		log.Fatal(domainFile, " does not have any domains")
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	file.Close()

	log.Println("DoTBomb start stress...")

	go dotbomb.Start()

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
