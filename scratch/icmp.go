package main

import (
    "fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
    "log"
    "net"
)

func recv() {
    protocol := "icmp"
    netaddr, err := net.ResolveIPAddr("ip4", "127.0.0.1")
    if err != nil {
		log.Fatal(err)
	}

    conn, err := net.ListenIP("ip4:"+protocol, netaddr)
    if err != nil {
		log.Fatal(err)
	}

    buf := make([]byte, 1024)
    numRead, _, err := conn.ReadFrom(buf)
    if err != nil {
		log.Fatal(err)
	}

    fmt.Printf("% X\n", buf[:numRead])
}

func send() {
	// srcIP := net.ParseIP("127.0.0.1").To4()
	dstIP := net.ParseIP("3.3.3.3").To4()

	srcIP, err := net.ResolveIPAddr("ip4", "localhost")
    if err != nil {
        log.Fatal(err.Error())
    }

    _, err = net.DialIP("ip4:icmp", srcIP, &net.IPAddr{IP: dstIP})
	if err != nil {
        log.Fatal(err)
    }

	//IP Layer
	ip := layers.IPv4{
		SrcIP:    srcIP.IP,
		DstIP:    dstIP,
		Version:  4,
		TTL:      64,
		Protocol: layers.IPProtocolICMPv4,
	}

	icmp := layers.ICMPv4{
		TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0)}
		// TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeTimeExceeded, 0)}

	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	buf := gopacket.NewSerializeBuffer()

	if err = gopacket.SerializeLayers(buf, opts, &ip, &icmp,
		gopacket.Payload([]byte{1, 2, 3, 4})); err != nil {
		log.Fatal("serialize err:", err)
	}

	log.Printf("%v", buf.Bytes())
}

func main() {
	send()
}