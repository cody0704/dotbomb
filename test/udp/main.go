package main

import (
	"log"
	"net"
	"time"

	"github.com/pion/udp"
)

func main() {

	l, err := udp.Listen("udp", &net.UDPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: 53,
	})
	if err != nil {
		return
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}

		go handle(conn)
	}
}

func handle(conn net.Conn) {
	log.Println("HiT")
	time.Sleep(time.Second * 10)
}
