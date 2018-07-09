package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"github.com/kevinburke/nacl"
	"github.com/kevinburke/nacl/box"
	"io"
	"log"
	"net"
	"strconv"
	// "sync"
	// "time"
)

/*
TODO:
	peer table
	bootstrap
	chat
	unmarshal
*/

type PeerSelf struct {
	ID      string
	Pub     nacl.Key
	Prv     nacl.Key
	Address string
}

type Peer struct {
	ID      string
	Pub     nacl.Key
	Address string
}

type Envelope struct {
	Type string
	From string
	To   string
	Data string
}

type MessageBootstrap struct {
	Address string
}

type MessageChat struct {
	Chat string
}

var peerSelf PeerSelf

func handleConn(conn net.Conn) {
	var env Envelope
	decoder := json.NewDecoder(conn)
	err := decoder.Decode(&env)

	if err != nil {
		log.Println("error decoding:", err)
	}

	log.Println(env.Data)
}

func getKeys() {
	var r io.Reader
	r = rand.Reader

	pub, prv, err := box.GenerateKey(r)
	if err != nil {
		log.Fatal(err)
	}

	peerSelf.ID = hex.EncodeToString(pub[:])
	peerSelf.Pub = pub
	peerSelf.Prv = prv
	log.Println(peerSelf.ID)
}

func recv(address string, port uint16) {
	addr := net.UDPAddr{
		Port: int(port),
		IP:   net.ParseIP(address),
	}
	// set up listener
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Fatal(err)
	}

	defer conn.Close()
	log.Println("listening...")

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Println("error reading")
		}

		log.Println(line)
	}

}

var debugFlag *bool
var portFlag *uint

func main() {
	debugFlag = flag.Bool("debug", false, "Debug.")
	portFlag = flag.Uint("port", 3499, "Port.")
	flag.Parse()

	// get port
	var port uint16 = uint16(*portFlag)

	// get external ip and open ports
	extIP := natStuff(port)
	defer natCleanup()

	// build self info (addr, keys, id)
	portStr := strconv.FormatUint(uint64(port), 10)
	peerSelf.Address = extIP.String() + ":" + portStr
	getKeys()
	log.Printf("%s/%s\n", peerSelf.Address, peerSelf.ID)

	// var wg sync.WaitGroup
	// ctrlChan := make(chan bool, 1)

	// start network receiver
	go recv("0.0.0.0", port)

	userInterface()
}
