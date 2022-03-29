package main

import (
	"crypto/tls"
	"log"
	"net"
	"time"
)

func main() {

	cert, err := tls.LoadX509KeyPair("./test.crt", "./test.key")
	if err != nil {
		return
	}

	l, err := tls.Listen("tcp", ":853", &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true},
	)
	if err != nil {
		log.Println(err)
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
	tlsconn, ok := conn.(*tls.Conn)
	if ok {
		err := tlsconn.Handshake()
		if err != nil {
			tlsconn.Close()
			return
		}
	}
	log.Println("HiT")
	time.Sleep(time.Second * 10)
}
