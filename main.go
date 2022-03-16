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
	flag.IntVar(&timeout, "t", 3, "Last Recv Packet Timeout")
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
	t1 := time.Now() // get current time

	go dotbomb.Start()

	select {
	case <-sigChan:
		report(t1, server.Result, 2)
	case status := <-server.StatusChan:
		report(t1, server.Result, status)
	}
}

func report(t1 time.Time, report server.StressReport, status int) {
	elapsed := time.Since(t1)
	fmt.Printf("\nRun Time:\t %.6fs\n", elapsed.Seconds())
	fmt.Println("Concurrency:\t", concurrency)
	switch status {
	case 0:
		fmt.Println("|-Status:\t", "Finish")
	case 1:
		fmt.Println("|-Status:\t", "Timeout")
	case 2:
		fmt.Println("|-Status:\t", "Cancle")
	}
	fmt.Println("|-Count:\t", concurrency*totalRequest)
	fmt.Println("|-Success")
	fmt.Println("| |-Send:\t", report.SendCount)
	totalResponse := report.RecvAnsCount + report.RecvNoAnsCount
	fmt.Println("| |-Recv:\t", totalResponse)
	fmt.Println("|   |-Answer:\t", report.RecvAnsCount)
	fmt.Println("|   |-NoAnswer:\t", report.RecvNoAnsCount)
	fmt.Printf("|   |-LastTime:\t %.6fs\n", report.LastTime.Seconds())
	avgTime := report.LastTime.Seconds() / float64(totalResponse)
	if math.IsInf(avgTime, 0) || math.IsNaN(avgTime) {
		fmt.Println("|   `-AvgTime:\t 0.000000s")
	} else {
		fmt.Printf("|   `-AvgTime:\t %.6fs\n", report.LastTime.Seconds()/float64(totalResponse))
	}
	// fmt.Printf("  └─Success Rate:\t %.2f %%", float32(successNumber)/float32(successNumber+errorNumber)*100)
	fmt.Println("`-Error:\t")
	fmt.Println("  `-CloseSock:\t", report.StopSockCount)
}
