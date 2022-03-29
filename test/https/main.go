package main

import (
	"log"
	"net/http"
	"time"
)

func handler(w http.ResponseWriter, req *http.Request) {
	log.Println("HiT")
	time.Sleep(time.Second * 10)
}

func main() {
	http.HandleFunc("/dns-query", handler)
	err := http.ListenAndServeTLS(":443", "./test.crt", "./test.key", nil)
	log.Fatal(err)
}
