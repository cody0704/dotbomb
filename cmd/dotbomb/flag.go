package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	versionID    string = "%VERSION%"
	version      bool
	mode         string
	timeout      int
	concurrency  int
	totalRequest int
	latency      int
	requestIP    string
	requestPort  string
	domainFile   string
)

func init() {
	flag.BoolVar(&version, "v", false, "number of concurrency")
	flag.StringVar(&mode, "m", "dot", "dot / doh Method: Post / dohp: DoH Method - Post / doh: DoH Method - Get / dns")
	flag.IntVar(&latency, "l", 0, "request latency")
	flag.IntVar(&timeout, "t", 1, "request Timeout")
	flag.IntVar(&concurrency, "c", 1, "number of concurrency")
	flag.IntVar(&totalRequest, "n", 1, "number of request")
	flag.StringVar(&requestIP, "r", "", "request ip address")
	flag.StringVar(&requestPort, "p", "", "request port")
	flag.StringVar(&domainFile, "f", "", "domain list file")

	flag.Parse()

	if version {
		fmt.Println(versionID)
		os.Exit(0)
	}

	if concurrency == 0 || totalRequest == 0 || requestIP == "" || mode == "" {
		fmt.Println("Example: dotbomb -m dot -c 10 -n 100 -r 8.8.8.8 -p 853 -f domains.txt")
		fmt.Println("-v [Version]")
		fmt.Println("-m [Mode] Default: dot, Option: dot / doh (doh - POST, dohg - GET, dohp - POST) / dns")
		fmt.Println("-l [Request Latency] <Number> Microsecond Unit 0.000001")
		fmt.Println("-c [Concurrency] <Number>")
		fmt.Println("-t [Timeout] <Second>")
		fmt.Println("-n [request] <Number>")
		fmt.Println("-r <Server IP>")
		fmt.Println("-p <Port: DoT 853 / DoH 443 / DNS 53>")
		fmt.Println("-f <DomainList File Path>")

		os.Exit(0)
	}

	switch mode {
	case "dns", "dot", "doh", "dohg", "dohp":
	default:
		fmt.Println("-m [Mode] Default: dot, Option: dot / doh / dns")
		os.Exit(0)
	}

	if requestPort == "" {
		switch mode {
		case "dns":
			requestPort = "53"
		case "dot":
			requestPort = "853"
		case "doh":
			requestPort = "443"
		case "dohg":
			requestPort = "443"
		case "dohp":
			requestPort = "443"
		}
	}
}
