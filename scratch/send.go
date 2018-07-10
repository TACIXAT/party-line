package main

import (
	"net"
)

func main() {
	conn, err := net.Dial("udp", "138.197.69.124:3499")
	if err != nil {
		panic(err)
	}

	conn.Write([]byte("HET\n"))
}
