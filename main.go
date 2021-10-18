package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cody0704/dotbomb/server"
)

var (
	concurrency  int
	totalRequest int
	requestIP    string
	domainFile   string
)

func init() {
	flag.IntVar(&concurrency, "c", 1, "concurrency number")
	flag.IntVar(&totalRequest, "n", 1, "request number")
	flag.StringVar(&requestIP, "r", "", "request ip address")
	flag.StringVar(&domainFile, "f", "", "domain list file")

	flag.Parse()

	if concurrency == 0 || totalRequest == 0 || requestIP == "" {
		fmt.Printf("Example: dotbomb -c 1 -n 1 -r 8.8.8.8:853 \n")

		flag.Usage()
		os.Exit(0)
	}
}

func main() {
	var dotbomb = server.DoTBomb{}
	dotbomb.Concurrency = concurrency
	dotbomb.TotalRequest = totalRequest
	dotbomb.RequestIP = requestIP

	if domainFile != "" {
		domainList, err := ioutil.ReadFile(domainFile)
		if err != nil {
			fmt.Println("Domain list file is not supported.")
			return
		}

		dotbomb.DomainArray = strings.Split(string(domainList), "\r\n")
	}

	fmt.Println("DoTBomb start stress...")
	t1 := time.Now() // get current time
	stressChannel, err := dotbomb.Start()
	if err != nil {
		log.Fatal(err)
	}

	var online = make(map[string]string)
	var onlineInfo = make(map[string]int)
	var stopNumber int
	var errorNumber int
	var successNumber int
	var avgDelayTime time.Duration

	for {
		select {
		case msg := <-stressChannel:
			status := strings.Split(msg, " ")
			if status[1] == "true" {
				online[status[0]] = status[1]
				if onlineInfo[status[0]] != totalRequest {
					onlineInfo[status[0]] = onlineInfo[status[0]] + 1
					if onlineInfo[status[0]] == totalRequest {
						stopNumber++
					}
				}
				successNumber++
			}

			if status[1] == "false" {
				delete(online, status[0])
				errorNumber++
			}

			if status[1] == "delay" {
				d, _ := strconv.Atoi(status[0])
				avgDelayTime = time.Duration((d) / successNumber)
			}

			if stopNumber == concurrency {
				elapsed := time.Since(t1)
				fmt.Printf("Time:\t%.2fs\n", elapsed.Seconds())
				fmt.Println("Concurrency:\t", len(online))
				fmt.Println("├─Total Query:\t", successNumber+errorNumber)
				fmt.Println("├─Success:\t", successNumber)
				fmt.Println("├─Fail:\t\t", errorNumber)
				fmt.Printf("└─Success Rate:\t %.2f", float32(successNumber)/float32(successNumber+errorNumber)*100)
				fmt.Println("%")
				fmt.Println("Avg Delay:\t", avgDelayTime)
				os.Exit(0)
			}
		}
	}
}
