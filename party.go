package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/kevinburke/nacl/box"
	"github.com/kevinburke/nacl/sign"
	"log"
)

var parties map[string]*PartyLine

func init() {
	parties = make(map[string]*PartyLine)
}

type PartyLine struct {
	MinList map[string]int
	ID      string
}

type PartyEnvelope struct {
	Type    string
	PartyID string
	Data    []byte
}

type PartyAnnounce struct {
	PeerID  string
	PartyID string
}

func (party *PartyLine) SendInvite(min MinPeer) {
	env := Envelope{
		Type: "invite",
		From: peerSelf.ID(),
		To:   min.ID()}

	jsonInvite, err := json.Marshal(party)
	if err != nil {
		log.Println(err)
		return
	}

	closed := box.EasySeal([]byte(jsonInvite), min.EncPub, self.EncPrv)
	env.Data = closed

	jsonEnv, err := json.Marshal(env)
	if err != nil {
		log.Println(err)
		return
	}

	route(jsonEnv)
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

	if partyAnnounce.PartyID != party.ID {
		setStatus("error invalid party (party:announce)")
		return
	}

	min, err := idToMin(partyAnnounce.PeerID)
	if err != nil {
		setStatus("error bad id (part:announce)")
		return
	}

	verified := sign.Verify(signedPartyAnnounce, min.SignPub)
	if !verified {
		setStatus("error questionable message integrity (part:announce)")
		return
	}

	_, seen := party.MinList[partyAnnounce.PeerID]

	if !seen {
		party.MinList[partyAnnounce.PeerID] = 0
		party.ForwardAnnounce(signedPartyAnnounce)
	}
}

func (party *PartyLine) ForwardAnnounce(signedPartyAnnounce []byte) {

}

func (party *PartyLine) SendAnnounce() {
	env := Envelope{
		Type: "party",
		From: peerSelf.ID(),
		To:   ""}

	partyEnv := PartyEnvelope{
		Type:    "announce",
		PartyID: party.ID}

	partyAnnounce := PartyAnnounce{
		PeerID:  peerSelf.ID(),
		PartyID: party.ID}

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

		jsonEnv, err := json.Marshal(env)
		if err != nil {
			log.Println(err)
			continue
		}

		route(jsonEnv)
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

	party, exists := parties[partyEnv.PartyID]
	if !exists {
		setStatus("error invalid party (party)")
		return
	}

	switch partyEnv.Type {
	case "announce":
		party.ProcessAnnounce(partyEnv)
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

	// party.SendAnnounce()
	parties[party.ID] = party
}

func minimum(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func newParty(name string) {
	idBytes := make([]byte, 16)
	rand.Read(idBytes)

	party := new(PartyLine)

	// this shouldn't be guessable, so we will enforce 12 bytes random
	name = name[:minimum(len(name), 8)]
	idHex := hex.EncodeToString(idBytes)
	party.ID = name + idHex[:len(idHex)-len(name)]

	party.MinList[peerSelf.ID()] = 0

	parties[party.ID] = party
}
