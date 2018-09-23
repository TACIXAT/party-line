package whitebox

import (
	"bufio"
	"container/list"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/kevinburke/nacl"
	"github.com/kevinburke/nacl/box"
	"github.com/kevinburke/nacl/sign"
	"log"
	"math/big"
	"net"
	"strings"
	"time"
)

const (
	TONE_LOW = iota
	TONE_HIGH
)

type WhiteBox struct {
	BsId              string
	ChatChannel       chan Chat
	StatusChannel     chan Status
	Self              Self
	PeerSelf          Peer
	PeerTable         [256]*list.List
	IdealPeerIds      [256]*big.Int
	EmptyList         bool
	SeenChats         map[string]bool
	Parties           map[string]*PartyLine
	PendingInvites    map[string]*PartyLine
	PeerCache         map[string]PeerCache
	SharedDir         string
	FreshRequests     map[string]*Since
	RequestChan       chan *PartyRequest
	VerifiedBlockChan chan *VerifiedBlock
	NoReroute         map[time.Time]bool
}

func (wb *WhiteBox) Run(port uint16) {
	go wb.Recv("", port)
	go wb.SendPings()
	go wb.FileRequester()
	go wb.RequestSender()
	go wb.VerifiedBlockWriter()
	go wb.Advertise()
}

func New(dir, addr, port string) *WhiteBox {
	wb := new(WhiteBox)
	wb.ChatChannel = make(chan Chat, 100)
	wb.StatusChannel = make(chan Status, 100)
	wb.SeenChats = make(map[string]bool)

	wb.InitFiles(dir)
	wb.GetKeys(addr + ":" + port)
	wb.CalculateIdealTableSelf(wb.Self.SignPub)
	wb.InitTable(wb.Self.SignPub)

	wb.BsId = fmt.Sprintf("%s/%s/%s", addr, port, wb.PeerSelf.ShortId())
	wb.EmptyList = true
	wb.Parties = make(map[string]*PartyLine)
	wb.PendingInvites = make(map[string]*PartyLine)
	wb.PeerCache = make(map[string]PeerCache)

	wb.FreshRequests = make(map[string]*Since)
	wb.RequestChan = make(chan *PartyRequest, 100)
	wb.VerifiedBlockChan = make(chan *VerifiedBlock, 100)
	wb.NoReroute = make(map[time.Time]bool)

	log.Println(wb.BsId)
	wb.chatStatus(wb.BsId)

	return wb
}

func (wb *WhiteBox) addChat(chat Chat) {
	wb.ChatChannel <- chat
}

type Status struct {
	Priority int
	Message  string
}

type Chat struct {
	Time    time.Time
	Id      string
	Channel string
	Message string
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

func (wb *WhiteBox) IdFront(id string) (string, error) {
	min, err := wb.IdToMin(id)
	if err != nil {
		wb.setStatus(err.Error())
		return "", err
	}

	return hex.EncodeToString(min.SignPub[:]), nil
}

func (wb *WhiteBox) IdBack(id string) (string, error) {
	min, err := wb.IdToMin(id)
	if err != nil {
		wb.setStatus(err.Error())
		return "", err
	}

	return hex.EncodeToString(min.EncPub[:]), nil
}

// TODO: lib candidate
func (wb *WhiteBox) IdToMin(id string) (*MinPeer, error) {
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

func (wb *WhiteBox) GetKeys(address string) {
	r := rand.Reader
	signPub, signPrv, err := sign.Keypair(r)
	if err != nil {
		log.Fatal(err)
	}

	encPub, encPrv, err := box.GenerateKey(r)
	if err != nil {
		log.Fatal(err)
	}

	wb.Self.SignPub = signPub
	wb.Self.SignPrv = signPrv
	wb.Self.EncPub = encPub
	wb.Self.EncPrv = encPrv
	wb.Self.Address = address

	wb.PeerSelf.SignPub = wb.Self.SignPub
	wb.PeerSelf.EncPub = wb.Self.EncPub
	wb.PeerSelf.Address = wb.Self.Address
	log.Println(wb.PeerSelf.Id())
}

func (wb *WhiteBox) Recv(address string, port uint16) {
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
			wb.setStatus("error reading")
		}

		log.Println("got", line)

		wb.processMessage(line)
	}

}

func (wb *WhiteBox) setStatus(message string) {
	status := Status{
		Priority: TONE_LOW,
		Message:  message,
	}
	wb.StatusChannel <- status
}

func (wb *WhiteBox) chatStatus(message string) {
	status := Status{
		Priority: TONE_HIGH,
		Message:  message,
	}
	wb.StatusChannel <- status
}
