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
	"strconv"
	"strings"
	"time"
)

/*
TODO:
	private channel
	advertise file
	advertise shared file
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

func (peer *Peer) ID() string {
	signStr := hex.EncodeToString(peer.SignPub[:])
	encStr := hex.EncodeToString(peer.EncPub[:])
	return signStr + "." + encStr
}

func (peer *Peer) ShortID() string {
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

func (min *MinPeer) ID() string {
	signStr := hex.EncodeToString(min.SignPub[:])
	encStr := hex.EncodeToString(min.EncPub[:])
	return signStr + "." + encStr
}

type Envelope struct {
	Type string
	From string
	To   string
	Data []byte
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
	Min  MinPeer
	Chat string
	Time time.Time
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
	log.Println(peerSelf.ID())
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

		processMessage(line)
	}

}

var debugFlag *bool
var portFlag *uint
var ipFlag *string
var nonatFlag *bool

func main() {
	debugFlag = flag.Bool("debug", false, "Debug.")
	portFlag = flag.Uint("port", 3499, "Port.")
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

	calculateIdealTableSelf(self.SignPub)
	initTable(self.SignPub)

	seenChats = make(map[string]bool)
	chatChan = make(chan string, 1)
	statusChan = make(chan string, 1)
	bsId = fmt.Sprintf("%s/%s/%s", extIP.String(), portStr, peerSelf.ShortID())
	log.Println(bsId)
	chatStatus(bsId)

	// var wg sync.WaitGroup
	// ctrlChan := make(chan bool, 1)

	// start network receiver
	go recv("", port)
	go sendPings()

	userInterface()
}
