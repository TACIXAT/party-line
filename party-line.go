package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"log"
	mrand "math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Message struct {
	// Kind string
	// From string
	// Signature string
	Data string
}

func recv(conn *net.UDPConn, wg *sync.WaitGroup) {
	defer wg.Done()

	decoder := json.NewDecoder(conn)
	for {
		var msg Message
		err := decoder.Decode(&msg)
		if err != nil {
			log.Println("error decoding:", err)
		}

		if msg.Data == "exit" {
			break
		}

		log.Println(msg.Data)
	}
}

func send(conn net.Conn, wg *sync.WaitGroup) {
	log.Println("sending...")
	defer wg.Done()

	encoder := json.NewEncoder(conn)
	var msg Message
	for i := 0; i < 10; i++ {
		msg.Data = "hello"
		encoder.Encode(msg)
		time.Sleep(1 * time.Second)
	}
	msg.Data = "exit"
	encoder.Encode(msg)
}

func main() {
	debugFlag := flag.Bool("debug", false, "Debug.")
	portFlag := flag.Uint("port", 3499, "Port.")
	bootstrapFlag := flag.String("bs", "", "Boostrap.")
	flag.Parse()

	var port uint16 = uint16(*portFlag)

	extIP := natStuff(port)
	defer natCleanup()

	var r io.Reader
	if *debugFlag {
		r = mrand.New(mrand.NewSource(int64(port)))
	} else {
		r = rand.Reader
	}

	key, err := rsa.GenerateKey(r, 4096)
	if err != nil {
		log.Fatal(err)
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err = enc.Encode(key.PublicKey)
	if err != nil {
		log.Fatal(err)
	}

	pubBytes := buf.Bytes()
	hasher := sha256.New()
	hasher.Write(pubBytes)

	id := hex.EncodeToString(hasher.Sum(nil))
	log.Printf(id)
	portStr := strconv.FormatUint(uint64(port), 10)
	log.Printf("%s/%s/%s\n", extIP, portStr, id)

	addr := net.UDPAddr{
		Port: int(port),
		IP:   net.ParseIP("0.0.0.0"),
	}

	conn, err := net.ListenUDP("udp", &addr)
	log.Println("listening...")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	if *bootstrapFlag != "" {
		bs := strings.Split(*bootstrapFlag, "/")
		if len(bs) != 3 {
			log.Fatal("invalid bootstrap string")
		}

		peerIP := bs[0]
		peerPort := bs[1]
		// peerID := bs[2]

		peerConn, err := net.Dial("udp", peerIP+":"+peerPort)
		if err != nil {
			log.Fatal("error connecting to peer", err)
		}

		go send(peerConn, &wg)
	} else {
		go recv(conn, &wg)
	}
	wg.Wait()
}
