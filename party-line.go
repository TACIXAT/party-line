package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"github.com/kevinburke/nacl"
	"github.com/kevinburke/nacl/box"
	"github.com/kevinburke/nacl/sign"
	// "io"
	"log"
	"net"
	"strconv"
	// "sync"
	"fmt"
	"time"
)

/*
TODO:
	peer table
	bootstrap
	chat
	unmarshal
*/

type Self struct {
	ID      string
	Handle  string
	EncPub  nacl.Key
	EncPrv  nacl.Key
	SignPub sign.PublicKey
	SignPrv sign.PrivateKey
	Address string
}

type Peer struct {
	ID      string
	Handle  string
	EncPub  nacl.Key
	SignPub sign.PublicKey
	Address string
	Writer  *bufio.Writer
}

type Envelope struct {
	Type string
	From string
	To   string
	Data string
}

type MessageBootstrap struct {
	ID      string
	Handle  string
	EncPub  nacl.Key
	SignPub sign.PublicKey
	Address string
}

type MessageChat struct {
	Chat string
	Time time.Time
}

var self Self
var peerTable map[string]Peer

var chatChan chan string
var statusChan chan string

func sendChat(msg string) {
	env := Envelope{
		Type: "chat",
		From: self.ID,
		To:   "",
		Data: ""}

	chat := MessageChat{
		Chat: msg,
		Time: time.Now()}

	jsonChat, err := json.Marshal(chat)
	if err != nil {
		log.Println(err)
		return
	}

	for _, peer := range peerTable {
		// closed := box.EasySeal([]byte(jsonChat), peer.EncPub, self.EncPrv)
		signed := sign.Sign([]byte(jsonChat), self.SignPrv)
		env.Data = hex.EncodeToString(signed)
		jsonEnv, err := json.Marshal(env)
		if err != nil {
			log.Println(err)
			continue
		}
		peer.Writer.WriteString(string(jsonEnv) + "\n")
	}
}

func sendBootstrap(addr, peerId string) {
	env := Envelope{
		Type: "chat",
		From: self.ID,
		To:   peerId,
		Data: ""}

	bs := MessageBootstrap{
		ID:      self.ID,
		Handle:  self.Handle,
		EncPub:  self.EncPub,
		Address: self.Address,
		SignPub: self.SignPub}

	jsonBs, err := json.Marshal(bs)
	if err != nil {
		log.Println(err)
		return
	}

	signed := sign.Sign([]byte(jsonBs), self.SignPrv)
	env.Data = hex.EncodeToString(signed)

	jsonEnv, err := json.Marshal(env)
	if err != nil {
		log.Println(err)
		return
	}

	chatStatus(string(jsonEnv))

	conn, err := net.Dial("udp", addr)
	if err != nil {
		log.Println(err)
		return
	}

	writer := bufio.NewWriter(conn)
	writer.WriteString(string(jsonEnv) + "\n")
}

func getKeys() {
	r := rand.Reader
	signPub, signPrv, err := sign.Keypair(r)
	if err != nil {
		log.Fatal(err)
	}

	encPub, encPrv, err := box.GenerateKey(r)
	if err != nil {
		log.Fatal(err)
	}

	self.ID = hex.EncodeToString(signPub[:])
	self.Handle = *handleFlag
	self.EncPub = encPub
	self.EncPrv = encPrv
	self.SignPub = signPub
	self.SignPrv = signPrv
	log.Println(self.ID)
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

		chatStatus(line)
	}

}

var debugFlag *bool
var portFlag *uint
var handleFlag *string

func main() {
	debugFlag = flag.Bool("debug", false, "Debug.")
	portFlag = flag.Uint("port", 3499, "Port.")
	handleFlag = flag.String("handle", "anon", "Handle.")
	flag.Parse()

	// get port
	var port uint16 = uint16(*portFlag)

	// get external ip and open ports
	extIP := natStuff(port)
	defer natCleanup()

	// build self info (addr, keys, id)
	portStr := strconv.FormatUint(uint64(port), 10)
	self.Address = extIP.String() + ":" + portStr
	getKeys()

	chatChan = make(chan string, 1)
	statusChan = make(chan string, 1)
	bsId := fmt.Sprintf("%s/%s/%s", extIP.String(), portStr, self.ID)
	log.Println(bsId)
	chatStatus(bsId)

	// var wg sync.WaitGroup
	// ctrlChan := make(chan bool, 1)

	// start network receiver
	go recv("0.0.0.0", port)

	userInterface()
}
