package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/kevinburke/nacl/box"
	"log"
)

type PartyLine struct {
	MinList []MinPeer
	ID      string
}

func (party *PartyLine) SendInvite(min MinPeer) {
	env := Envelope{
		Type: "invite",
		From: self.ID,
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

// func (party *PartyLine) SendAnnounce() {
// 	env := Envelope{
// 		Type: "party",
// 		From: self.ID,
// 		To:   ""}

// 	for(_, min := range party.MinList) {

// 	}

// }

// func ProcessParty(env *Envelope) {

// }

func ProcessInvite(env *Envelope) {
	min, hit := peerCache[env.From]
	if !hit {
		setStatus("error no hit in min cache (invite)")
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
	parties = append(parties, party)
}

var parties []*PartyLine

func init() {
	parties = make([]*PartyLine, 0)
}

func newParty(name string) {
	idBytes := make([]byte, 16)
	rand.Read(idBytes)

	party := new(PartyLine)
	party.ID = hex.EncodeToString(idBytes)
	party.MinList = append(party.MinList, peerSelf.Min())

	parties = append(parties, party)
}
