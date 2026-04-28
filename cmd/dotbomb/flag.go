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
	requestIP    string
	requestPort  int
	domainFile   string

	// Stress
	interval       int
	inflight       int
	ignoreResponse bool

	// Fake
	fakeIF        string
	fakeIP        string
	fakeSourceMac string
	fakeTargetMac string
)

func init() {
	flag.BoolVar(&version, "v", false, "number of concurrency")
	flag.StringVar(&mode, "m", "dot", "dot / doh / dns / dnssec / all")
	flag.IntVar(&timeout, "t", 1, "request Timeout")
	flag.IntVar(&concurrency, "c", 1, "number of concurrency")
	flag.IntVar(&totalRequest, "n", 1, "number of request")
	flag.StringVar(&requestIP, "r", "", "dns ip address")
	flag.IntVar(&requestPort, "p", 0, "dns port")
	flag.StringVar(&domainFile, "f", "", "domain list file")
	flag.IntVar(&interval, "tps", 30, "Packet send tps")
	flag.IntVar(&inflight, "inflight", 1, "in-flight queries per DoT/DoH worker (1 = current sequential behavior)")
	flag.BoolVar(&ignoreResponse, "ignore", false, "ignore DNS query response (dns only); for tapped/mirrored traffic where no reply will arrive")

	// Make
	flag.StringVar(&fakeIF, "finet", "", "fake interface")
	flag.StringVar(&fakeIP, "fip", "", "fake ip address")
	flag.StringVar(&fakeSourceMac, "fsmac", "", "fake source mac address")
	flag.StringVar(&fakeTargetMac, "fdmac", "", "fake target mac address")

	flag.Parse()

	if version {
		fmt.Println(versionID)
		os.Exit(0)
	}

	// check fake
	if fakeIF != "" {
		if fakeIP == "" || fakeSourceMac == "" || fakeTargetMac == "" {
			fmt.Println("Fake need -finet -fip -fsmac -fdmac")
			os.Exit(0)
		}
	} else if concurrency == 0 || totalRequest == 0 || requestIP == "" || mode == "" {
		fmt.Println("Example: dotbomb -m dot -c 10 -n 100 -r 8.8.8.8 -p 853 -f domains.txt")
		fmt.Println("-v [Version]")
		fmt.Println("-tps [TPS] <Number> Default: 30")
		fmt.Println("-m [Mode] Default: dot, Option: dot / doh (POST) / dohg (GET) / dns / dnssec / all (fan out across DNS+DNSSEC+DoT+DoH)")
		fmt.Println("-c [Concurrency] <Number>")
		fmt.Println("-t [Timeout] <Second>")
		fmt.Println("-n [request] <Number>")
		fmt.Println("-r <Server IP>")
		fmt.Println("-p <Port: DoT 853 / DoH 443 / DNS 53>")
		fmt.Println("-f <DomainList File Path>")

		os.Exit(0)
	}

	switch mode {
	case "dns", "dot", "doh", "dohg", "dohp", "dnssec", "all":
	default:
		fmt.Println("-m [Mode] Default: dot, Option: dot / doh / dohg / dns / dnssec / all")
		os.Exit(0)
	}

	if ignoreResponse && mode != "dns" {
		fmt.Println("-ignore is only supported with -m dns")
		os.Exit(0)
	}

	if requestPort == 0 {
		switch mode {
		case "dns", "dnssec":
			requestPort = 53
		case "dot":
			requestPort = 853
		case "doh", "dohg":
			requestPort = 443
		}
	}
	// -m all 跨四協定, 每個用各自的標準 port (53/53/853/443); 忽略 -p.
}
