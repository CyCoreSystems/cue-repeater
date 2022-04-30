package main

import (
	"flag"
	"fmt"
	"log"
	"net"
)

var targetPort int

func init() {
	flag.IntVar(&targetPort, "p", 8080, "target port")
}

func main() {
	flag.Parse()

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", targetPort))
	if err != nil {
		log.Fatal("failed to resolve target port:", targetPort, ":", err.Error())
	}

	c, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Fatal("failed to dial target port:", targetPort, ":", err.Error())
	}

	if _, err := c.Write([]byte("test")); err != nil {
		log.Println("failed to send message:", err)
	}

	buf := make([]byte, 2048)

	for i := 0; i < 2048; i++ {
		buf[i] = 'x'
}

	// Too large for buffer
	if _, err := c.Write(buf); err != nil {
		log.Println("failed to write too-large buffer")
}

	c.Close()

}

