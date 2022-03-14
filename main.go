package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
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

	if domainFile != "" {
		domainList, err := ioutil.ReadFile(domainFile)
		if err != nil {
			fmt.Println("Domain list file is not supported.")
			return
		}

		dotbomb.DomainArray = strings.Split(string(domainList), "\r\n")
	}

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
	fmt.Println("| |-Recv:\t", report.RecvCount)
	fmt.Printf("|   |-LastTime:\t %.6fs\n", report.LastTime.Seconds())
	fmt.Printf("|   `-AvgTIme:\t %.6fs\n", report.LastTime.Seconds()/float64(report.RecvCount))
	// fmt.Printf("  └─Success Rate:\t %.2f %%", float32(successNumber)/float32(successNumber+errorNumber)*100)
	fmt.Println("`-Error:\t")
	fmt.Println("  |-Recv:\t", report.RecvErrCount)
	fmt.Println("  `-CloseSock:\t", report.StopSockCount)
}
