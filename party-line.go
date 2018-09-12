package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"github.com/kevinburke/nacl"
	"github.com/kevinburke/nacl/box"
	"github.com/kevinburke/nacl/sign"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

/*
TODO:
	drop pack name in receiving file structure
	unthrottle fulfillments

	scrolling
	cursor on input
	better keys
	sent scrollback
	bs shortener
	name color
	mute user
	more statuses

	check channel capcity before adding
	partial packs (save / resume on exit)

	measure block packet size
	increase block size
*/

type Self struct {
	EncPub  nacl.Key
	EncPrv  nacl.Key `json:"-"`
	SignPub sign.PublicKey
	SignPrv sign.PrivateKey `json:"-"`
	Address string
}

type Peer struct {
	EncPub  nacl.Key
	SignPub sign.PublicKey
	Address string
	Conn    net.Conn `json:"-"`
}

func (peer *Peer) Id() string {
	signStr := hex.EncodeToString(peer.SignPub[:])
	encStr := hex.EncodeToString(peer.EncPub[:])
	return signStr + "." + encStr
}

func (peer *Peer) ShortId() string {
	signStr := hex.EncodeToString(peer.SignPub[:])
	return signStr
}

func (peer *Peer) Min() MinPeer {
	var min MinPeer
	min.EncPub = peer.EncPub
	min.SignPub = peer.SignPub
	return min
}

type MinPeer struct {
	EncPub  nacl.Key
	SignPub sign.PublicKey
}

func (min *MinPeer) Id() string {
	signStr := hex.EncodeToString(min.SignPub[:])
	encStr := hex.EncodeToString(min.EncPub[:])
	return signStr + "." + encStr
}

type Envelope struct {
	Type string
	From string
	To   string
	Data []byte
	Time time.Time
}

type MessageSuggestions struct {
	Peer           Peer
	RequestData    []byte
	SuggestedPeers []Peer
}

type MessageSuggestionRequest struct {
	Peer Peer
	To   string
}

type MessageChat struct {
	Min     MinPeer
	Message string
	Time    time.Time
}

type MessageTime struct {
	MessageType int
	Time        time.Time
}

type MessagePing struct {
	Min         MinPeer
	MessageType int
	Time        time.Time
}

var self Self
var peerSelf Peer
var bsId string

var seenChats map[string]bool
var chatChan chan string
var statusChan chan string

func idFront(id string) (string, error) {
	min, err := idToMin(id)
	if err != nil {
		setStatus(err.Error())
		return "", err
	}

	return hex.EncodeToString(min.SignPub[:]), nil
}

func idBack(id string) (string, error) {
	min, err := idToMin(id)
	if err != nil {
		setStatus(err.Error())
		return "", err
	}

	return hex.EncodeToString(min.EncPub[:]), nil
}

func idToMin(id string) (*MinPeer, error) {
	pubs := strings.Split(id, ".")
	if len(pubs) != 2 {
		return nil, errors.New("error invalid id (min)")
	}

	signHex := pubs[0]
	signBytes, err := hex.DecodeString(signHex)
	if err != nil {
		return nil, errors.New("error invalid id (min)")
	}

	encHex := pubs[1]
	encBytes, err := hex.DecodeString(encHex)
	if err != nil {
		return nil, errors.New("error invalid id (min)")
	}

	min := new(MinPeer)
	min.SignPub = sign.PublicKey(signBytes)

	var encFixed [nacl.KeySize]byte
	copy(encFixed[:], encBytes[:nacl.KeySize])
	min.EncPub = nacl.Key(&encFixed)

	return min, nil
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

	self.SignPub = signPub
	self.SignPrv = signPrv
	self.EncPub = encPub
	self.EncPrv = encPrv

	peerSelf.SignPub = self.SignPub
	peerSelf.EncPub = self.EncPub
	peerSelf.Address = self.Address
	log.Println(peerSelf.Id())
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

	reader := bufio.NewReaderSize(conn, 2*65536)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			setStatus("error reading")
		}

		log.Println("got", line)

		processMessage(line)
	}

}

var debugFlag *bool
var portFlag *uint
var ipFlag *string
var nonatFlag *bool
var shareFlag *string

func main() {
	debugFlag = flag.Bool("debug", false, "Debug.")
	portFlag = flag.Uint("port", 3499, "Port.")
	ipFlag = flag.String("ip", "", "Manually set external IP.")
	nonatFlag = flag.Bool("nonat", false, "Disable UPNP and PMP.")
	shareFlag = flag.String("share", "", "Base directory to share from.")
	flag.Parse()

	initFiles()

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

	// log to file
	logname := fmt.Sprintf("/tmp/partylog.%s", peerSelf.Id()[:6])
	f, err := os.OpenFile(logname, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(f)

	calculateIdealTableSelf(self.SignPub)
	initTable(self.SignPub)

	seenChats = make(map[string]bool)
	chatChan = make(chan string, 1)
	statusChan = make(chan string, 1)
	bsId = fmt.Sprintf("%s/%s/%s", extIP.String(), portStr, peerSelf.ShortId())
	log.Println(bsId)
	chatStatus(bsId)

	// var wg sync.WaitGroup
	// ctrlChan := make(chan bool, 1)

	// start network receiver
	go recv("", port)
	go sendPings()
	go fileRequester()
	go requestSender()
	go verifiedBlockWriter()

	userInterface()
}
