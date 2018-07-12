package main

import (
	"bufio"
	// "bytes"
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
	send table
	announce
	chat
	pulse
	disconnect
	private message

	private channel
	advertise file
	advertise shared file
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
	Conn    net.Conn
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

var chatChan chan string
var statusChan chan string

func processBootstrap(env *Envelope) {
	fromPub, err := hex.DecodeString(env.From)
	if err != nil {
		log.Println(err)
		setStatus("error decoding hex (bs:from)")
		return
	}

	data, err := hex.DecodeString(env.Data)
	if err != nil {
		log.Println(err)
		setStatus("error decoding hex (bs:data)")
		return
	}

	verified := sign.Verify(data, fromPub)
	if !verified {
		setStatus("questionable message integrity discarding (bs)")
		return
	}

	jsonData := data[sign.SignatureSize:]
	chatStatus(string(jsonData))
	chatStatus(fmt.Sprintf("%d", len(jsonData)))

	var bs MessageBootstrap
	err = json.Unmarshal(jsonData, &bs)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (bs)")
		return
	}

	peer := new(Peer)
	peer.ID = bs.ID
	peer.Handle = bs.Handle
	peer.EncPub = bs.EncPub
	peer.SignPub = bs.SignPub
	peer.Address = bs.Address

	if env.From != peer.ID {
		setStatus("id does not match from (bs)")
		return
	}

	peerConn, err := net.Dial("udp", peer.Address)
	if err != nil {
		log.Println(err)
		setStatus("could not connect to peer (bs)")
		return
	}

	peer.Conn = peerConn
	sendVerify(peer)
	sendTable(peer)
	addPeer(peer)
}

func sendTable(peer *Peer) {

}

func sendVerify(peer *Peer) {
	env := Envelope{
		Type: "verifybs",
		From: self.ID,
		To:   peer.ID,
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

	peer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
	setStatus("verify sent")
}

func processVerify(env *Envelope) {
	fromPub, err := hex.DecodeString(env.From)
	if err != nil {
		log.Println(err)
		setStatus("error decoding hex (bsverify:from)")
		return
	}

	data, err := hex.DecodeString(env.Data)
	if err != nil {
		log.Println(err)
		setStatus("error decoding hex (bsverify:data)")
		return
	}

	verified := sign.Verify(data, fromPub)
	if !verified {
		setStatus("questionable message integrity discarding (bsverify)")
		return
	}

	jsonData := data[sign.SignatureSize:]

	var bs MessageBootstrap
	err = json.Unmarshal(jsonData, &bs)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (bsverify)")
		return
	}

	peer := new(Peer)
	peer.ID = bs.ID
	peer.Handle = bs.Handle
	peer.EncPub = bs.EncPub
	peer.SignPub = bs.SignPub
	peer.Address = bs.Address

	if env.From != peer.ID {
		setStatus("id does not match from (bsverify)")
		return
	}

	peerConn, err := net.Dial("udp", peer.Address)
	if err != nil {
		log.Println(err)
		setStatus("could not connect to peer (bsverify)")
		return
	}

	peer.Conn = peerConn
	addPeer(peer)
}

func processMessage(strMsg string) {
	env := new(Envelope)
	err := json.Unmarshal([]byte(strMsg), env)
	if err != nil {
		log.Println(err)
		setStatus("invalid json message received")
		return
	}

	switch env.Type {
	case "bootstrap":
		processBootstrap(env)
	case "verifybs":
		processVerify(env)
	default:
		setStatus("unknown msg type: " + env.Type)
	}
}

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

	for _, list := range peerTable {
		curr := list.Front()
		currEntry := curr.Value.(*PeerEntry)
		currPeer := currEntry.Entry

		if currPeer == nil {
			continue
		}

		// closed := box.EasySeal([]byte(jsonChat), peer.EncPub, self.EncPrv)
		signed := sign.Sign([]byte(jsonChat), self.SignPrv)
		env.Data = hex.EncodeToString(signed)
		jsonEnv, err := json.Marshal(env)
		if err != nil {
			log.Println(err)
			continue
		}

		currPeer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
	}
	setStatus("chat sent")
}

func sendBootstrap(addr, peerId string) {
	env := Envelope{
		Type: "bootstrap",
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

	conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
	setStatus("bs sent")
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
			setStatus("error reading")
		}

		chatStatus("got: " + line)
		processMessage(line)
	}

}

var debugFlag *bool
var portFlag *uint
var handleFlag *string
var ipFlag *string
var nonatFlag *bool

func main() {
	debugFlag = flag.Bool("debug", false, "Debug.")
	portFlag = flag.Uint("port", 3499, "Port.")
	handleFlag = flag.String("handle", "anon", "Handle.")
	ipFlag = flag.String("ip", "", "Manually set external IP.")
	nonatFlag = flag.Bool("nonat", false, "Disable UPNP and PMP.")
	flag.Parse()

	// get port
	var port uint16 = uint16(*portFlag)

	// get external ip and open ports
	var extIP net.IP
	if *nonatFlag {
		if *ipFlag == "" {
			log.Fatal("Must provide an IP address with nonat flag.")
		}

		extIP = net.ParseIP(*ipFlag)
	} else {
		extIP = natStuff(port)
		defer natCleanup()
	}

	// build self info (addr, keys, id)
	portStr := strconv.FormatUint(uint64(port), 10)
	self.Address = extIP.String() + ":" + portStr
	getKeys()

	calculateIdealTable(self.SignPub)
	initTable(self.SignPub)

	chatChan = make(chan string, 1)
	statusChan = make(chan string, 1)
	bsId := fmt.Sprintf("%s/%s/%s", extIP.String(), portStr, self.ID)
	log.Println(bsId)
	chatStatus(bsId)

	// var wg sync.WaitGroup
	// ctrlChan := make(chan bool, 1)

	// start network receiver
	go recv("", port)

	userInterface()
}
