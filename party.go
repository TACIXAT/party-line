package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/kevinburke/nacl/box"
	"github.com/kevinburke/nacl/sign"
	"log"
	"sort"
	"time"
)

var parties map[string]*PartyLine

func init() {
	parties = make(map[string]*PartyLine)
}

type PartyLine struct {
	MinList   map[string]int
	SeenChats map[string]bool
	Id        string
}

type PartyEnvelope struct {
	Type    string
	From    string
	PartyId string
	Data    []byte
}

type PartyAnnounce struct {
	PeerId  string
	PartyId string
}

type PartyChat struct {
	PeerId  string
	PartyId string
	Message string
	Time    time.Time
}

type PartyDisconnect struct {
	PeerId  string
	PartyId string
	Time    time.Time
}

func modFloor(i, m int) int {
	return ((i % m) + m) % m
}

func (party *PartyLine) getNeighbors() map[string]bool {
	sortedIds := make([]string, 0, len(party.MinList))
	for id, _ := range party.MinList {
		sortedIds = append(sortedIds, id)
	}
	sort.Strings(sortedIds)

	var idx int = -1
	selfId := peerSelf.Id()
	for i, id := range sortedIds {
		if id == selfId {
			idx = i
			break
		}
	}

	neighbors := make(map[string]bool)
	if idx == -1 {
		setStatus("error could not find self in min list")
		return neighbors
	}

	n1 := modFloor((idx - 1), len(sortedIds))
	n2 := modFloor((idx + 1), len(sortedIds))
	n3 := modFloor((idx + 2), len(sortedIds))

	neighbors[sortedIds[n1]] = true
	neighbors[sortedIds[n2]] = true
	neighbors[sortedIds[n3]] = true

	return neighbors
}

func (party *PartyLine) SendInvite(min *MinPeer) {
	env := Envelope{
		Type: "invite",
		From: peerSelf.Id(),
		To:   min.Id()}

	// keep message small so we don't limit party size
	// hopefully this doesn't fuck up delivery
	var sendParty *PartyLine = party
	if len(party.MinList) > 20 {
		sendParty = new(PartyLine)
		sendParty.Id = party.Id
		idx := 0
		for id, _ := range party.MinList {
			sendParty.MinList[id] = 0
			idx++
			if idx > 20 {
				break
			}
		}
	}

	jsonInvite, err := json.Marshal(sendParty)
	if err != nil {
		log.Println(err)
		return
	}

	closed := box.EasySeal([]byte(jsonInvite), min.EncPub, self.EncPrv)
	env.Data = closed

	route(&env)
}

func (party *PartyLine) SendAnnounce() {
	env := Envelope{
		Type: "party",
		From: peerSelf.Id(),
		To:   ""}

	partyEnv := PartyEnvelope{
		Type:    "announce",
		From:    peerSelf.Id(),
		PartyId: party.Id}

	partyAnnounce := PartyAnnounce{
		PeerId:  peerSelf.Id(),
		PartyId: party.Id}

	jsonPartyAnnounce, err := json.Marshal(partyAnnounce)
	if err != nil {
		log.Println(err)
		return
	}

	signedPartyAnnounce := sign.Sign([]byte(jsonPartyAnnounce), self.SignPrv)
	partyEnv.Data = signedPartyAnnounce

	jsonPartyEnv, err := json.Marshal(partyEnv)
	if err != nil {
		log.Println(err)
		return
	}

	for idMin, _ := range party.MinList {
		min, err := idToMin(idMin)
		if err != nil {
			setStatus(err.Error())
			continue
		}

		closed := box.EasySeal([]byte(jsonPartyEnv), min.EncPub, self.EncPrv)
		env.Data = closed
		env.To = idMin

		route(&env)
	}
}

func (party *PartyLine) sendToNeighbors(msgType string, signedPartyData []byte) {
	env := Envelope{
		Type: "party",
		From: peerSelf.Id(),
		To:   ""}

	partyEnv := PartyEnvelope{
		Type:    msgType,
		From:    peerSelf.Id(),
		PartyId: party.Id}

	partyEnv.Data = signedPartyData

	jsonPartyEnv, err := json.Marshal(partyEnv)
	if err != nil {
		log.Println(err)
		return
	}

	neighbors := party.getNeighbors()
	for idMin, _ := range neighbors {
		min, err := idToMin(idMin)
		if err != nil {
			setStatus(err.Error())
			continue
		}

		closed := box.EasySeal([]byte(jsonPartyEnv), min.EncPub, self.EncPrv)
		env.Data = closed
		env.To = idMin

		route(&env)
	}
}

func (party *PartyLine) SendChat(message string) {
	partyChat := PartyChat{
		PeerId:  peerSelf.Id(),
		PartyId: party.Id,
		Message: message,
		Time:    time.Now()}

	jsonPartyChat, err := json.Marshal(partyChat)
	if err != nil {
		log.Println(err)
		return
	}

	signedPartyChat := sign.Sign([]byte(jsonPartyChat), self.SignPrv)

	party.sendToNeighbors("chat", signedPartyChat)
}

func (party *PartyLine) SendDisconnect() {
	partyDisconnect := PartyDisconnect{
		PeerId:  peerSelf.Id(),
		PartyId: party.Id,
		Time:    time.Now()}

	jsonPartyDisconnect, err := json.Marshal(partyDisconnect)
	if err != nil {
		log.Println(err)
		return
	}

	delete(parties, party.Id)

	signedPartyDisconnect := sign.Sign([]byte(jsonPartyDisconnect), self.SignPrv)
	party.sendToNeighbors("disconnect", signedPartyDisconnect)
}

func (party *PartyLine) ProcessChat(partyEnv *PartyEnvelope) {
	signedPartyChat := partyEnv.Data
	jsonPartyChat := signedPartyChat[sign.SignatureSize:]

	partyChat := new(PartyChat)
	err := json.Unmarshal(jsonPartyChat, partyChat)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (party:chat)")
		return
	}

	if partyChat.PartyId != party.Id {
		setStatus("error invalid party (party:chat)")
		return
	}

	min, err := idToMin(partyChat.PeerId)
	if err != nil {
		setStatus("error bad id (party:chat)")
		return
	}

	verified := sign.Verify(signedPartyChat, min.SignPub)
	if !verified {
		setStatus("error questionable message integrity (party:chat)")
		return
	}

	chatId := fmt.Sprintf("%s.%s", partyChat.PeerId, partyChat.Time.String())
	_, seen := party.SeenChats[chatId]
	if !seen {
		party.SeenChats[chatId] = true

		chat := Chat{
			Time:    time.Now(),
			Id:      partyChat.PeerId,
			Channel: party.Id,
			Message: partyChat.Message}

		addChat(chat)

		party.sendToNeighbors("chat", signedPartyChat)
	}
}

func (party *PartyLine) ProcessDisconnect(partyEnv *PartyEnvelope) {
	signedPartyDisconnect := partyEnv.Data
	jsonPartyDisconnect := signedPartyDisconnect[sign.SignatureSize:]

	partyDisconnect := new(PartyDisconnect)
	err := json.Unmarshal(jsonPartyDisconnect, partyDisconnect)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (party:disconnect)")
		return
	}

	if partyDisconnect.PartyId != party.Id {
		setStatus("error invalid party (party:disconnect)")
		return
	}

	min, err := idToMin(partyDisconnect.PeerId)
	if err != nil {
		setStatus("error bad id (party:disconnect)")
		return
	}

	verified := sign.Verify(signedPartyDisconnect, min.SignPub)
	if !verified {
		setStatus("error questionable message integrity (party:disconnect)")
		return
	}

	if time.Since(partyDisconnect.Time) > 200*time.Second {
		setStatus("error time exceeds max allowable (party:disconnect)")
		return
	}

	_, seen := party.MinList[partyDisconnect.PeerId]

	if seen {
		delete(party.MinList, partyDisconnect.PeerId)
		party.sendToNeighbors("disconnect", signedPartyDisconnect)
	}
}

func (party *PartyLine) ProcessAnnounce(partyEnv *PartyEnvelope) {
	signedPartyAnnounce := partyEnv.Data
	jsonPartyAnnounce := signedPartyAnnounce[sign.SignatureSize:]

	partyAnnounce := new(PartyAnnounce)
	err := json.Unmarshal(jsonPartyAnnounce, partyAnnounce)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (party:announce)")
		return
	}

	if partyAnnounce.PartyId != party.Id {
		setStatus("error invalid party (party:announce)")
		return
	}

	min, err := idToMin(partyAnnounce.PeerId)
	if err != nil {
		setStatus("error bad id (party:announce)")
		return
	}

	verified := sign.Verify(signedPartyAnnounce, min.SignPub)
	if !verified {
		setStatus("error questionable message integrity (party:announce)")
		return
	}

	_, seen := party.MinList[partyAnnounce.PeerId]

	if !seen {
		party.MinList[partyAnnounce.PeerId] = 0
		party.sendToNeighbors("announce", signedPartyAnnounce)
	}
}

func processParty(env *Envelope) {
	min, err := idToMin(env.From)
	if err != nil {
		setStatus(err.Error())
		return
	}

	jsonData, err := box.EasyOpen(env.Data, min.EncPub, self.EncPrv)
	if err != nil {
		setStatus("error invalid crypto (party)")
		return
	}

	partyEnv := new(PartyEnvelope)
	err = json.Unmarshal(jsonData, partyEnv)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (party)")
		return
	}

	party, exists := parties[partyEnv.PartyId]
	if !exists {
		setStatus("error invalid party (party)")
		return
	}

	switch partyEnv.Type {
	case "announce":
		party.ProcessAnnounce(partyEnv)
	case "chat":
		party.ProcessChat(partyEnv)
	case "disconnect":
		party.ProcessDisconnect(partyEnv)
	default:
		setStatus(fmt.Sprintf("unknown message type %s (party)", partyEnv.Type))
	}

	// chatStatus(fmt.Sprintf("got %s", partyEnv.Type))

	if env.From == partyEnv.From && partyEnv.Type != "disconnect" {
		party.MinList[partyEnv.From] = 0
	}
}

func processInvite(env *Envelope) {
	min, err := idToMin(env.From)
	if err != nil {
		setStatus(err.Error())
		return
	}

	jsonData, err := box.EasyOpen(env.Data, min.EncPub, self.EncPrv)
	if err != nil {
		setStatus("error invalid crypto (invite)")
		return
	}

	party := new(PartyLine)
	err = json.Unmarshal(jsonData, party)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (invite)")
		return
	}

	party.SendAnnounce()
	parties[party.Id] = party
}

func minimum(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func partyStart(name string) string {
	idBytes := make([]byte, 16)
	rand.Read(idBytes)

	party := new(PartyLine)

	// this shouldn't be guessable, so we will enforce 12 bytes random
	name = name[:minimum(len(name), 8)]
	idHex := hex.EncodeToString(idBytes)
	party.Id = name + idHex[:len(idHex)-len(name)]
	party.MinList = make(map[string]int)
	party.SeenChats = make(map[string]bool)

	party.MinList[peerSelf.Id()] = 0

	parties[party.Id] = party

	return party.Id
}