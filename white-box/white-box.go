package whitebox

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/douggard/party-line/party-lib"
	"github.com/kevinburke/nacl"
	"github.com/kevinburke/nacl/box"
	"github.com/kevinburke/nacl/sign"
	"log"
	"net"
	"strings"
	"time"
)

type WhiteBox struct {
	Name string
	ChatChannel chan partylib.Chat
}

func New(name string) *WhiteBox {
	wb := new(WhiteBox)
	wb.Name = name
	wb.ChatChannel = make(chan partylib.Chat, 100)
	return wb
}

func (wb *WhiteBox) addChat(chat partylib.Chat) {
	wb.ChatChannel <- chat
}

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

func (wb *WhiteBox) recv(address string, port uint16) {
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

		wb.processMessage(line)
	}

}

func setStatus(status string) {
	fmt.Println(status)
}

func chatStatus(status string) {
	fmt.Println(status)
}

