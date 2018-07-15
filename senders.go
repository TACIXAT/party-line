package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/kevinburke/nacl/sign"
	"log"
	"net"
	"time"
)

func flood(env *Envelope) {
	jsonEnv, err := json.Marshal(env)
	if err != nil {
		log.Println(err)
		return
	}

	sentPeers := make(map[string]bool)
	for _, list := range peerTable {
		for curr := list.Front(); curr != nil; curr = curr.Next() {
			currEntry := curr.Value.(*PeerEntry)
			currPeer := currEntry.Entry

			if currPeer == nil {
				continue
			}

			_, sent := sentPeers[currPeer.ID]
			if !sent {
				currPeer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
				sentPeers[currPeer.ID] = true
			}
		}
	}

	setStatus("flooded")
}

func forwardChat(env *Envelope) {
	jsonEnv, err := json.Marshal(env)
	if err != nil {
		log.Println(err)
		return
	}

	sendPeers := make(map[string]*Peer)
	for _, list := range peerTable {
		curr := list.Front()
		currEntry := curr.Value.(*PeerEntry)
		currPeer := currEntry.Entry

		if currPeer == nil {
			continue
		}

		sendPeers[currPeer.ID] = currPeer
	}

	for _, peer := range sendPeers {
		peer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
	}

	setStatus("chat fowarded")
}

func sendSuggestionRequest(peer *Peer) {
	// env := Envelope{
	// 	Type: "suggestionRequest",
	// 	From: self.ID,
	// 	To:   peer.ID,
	// 	Data: ""}

	// jsonBs, err := json.Marshal(peerSelf)
	// if err != nil {
	// 	log.Println(err)
	// 	return
	// }
}

func sendSuggestions(peer *Peer) {
	// calculate ideal for id
	peerIdealTable := calculateIdealTable(peer.SignPub)

	// find closest for each and make a unique list
	peerSetHelper := make(map[string]bool)
	peerSet := make([]Peer, 0)
	peerSet = append(peerSet)
	for _, idInt := range peerIdealTable {
		closestPeerEntry := findClosest(idInt.Bytes())
		if closestPeerEntry.Entry != nil {
			closestPeer := closestPeerEntry.Entry
			_, contains := peerSetHelper[closestPeer.ID]
			if !contains {
				peerSetHelper[closestPeer.ID] = true
				peerSet = append(peerSet, *closestPeer)
			}
		}
	}

	// truncate
	// each encoded peer is about 303 bytes
	// this tops things off around 38kb
	// well below max udp packet size
	if len(peerSet) > 128 {
		peerSet = peerSet[:128]
	}

	chatStatus(fmt.Sprintf("sending %d peers", len(peerSet)))
}

func sendVerify(peer *Peer) {
	env := Envelope{
		Type: "verifybs",
		From: self.ID,
		To:   peer.ID,
		Data: ""}

	jsonBs, err := json.Marshal(peerSelf)
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

	sendPeers := make(map[string]*Peer)
	for _, list := range peerTable {
		curr := list.Front()
		currEntry := curr.Value.(*PeerEntry)
		currPeer := currEntry.Entry

		if currPeer == nil {
			continue
		}

		sendPeers[currPeer.ID] = currPeer
	}

	if len(sendPeers) == 0 {
		chatStatus("you have no friends, bootstrap to a peer to get started")
		return
	}

	signed := sign.Sign([]byte(jsonChat), self.SignPrv)
	env.Data = hex.EncodeToString(signed)
	jsonEnv, err := json.Marshal(env)

	for _, peer := range sendPeers {
		// closed := box.EasySeal([]byte(jsonChat), peer.EncPub, self.EncPrv)
		peer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
	}
	setStatus("chat sent")
}

func sendBootstrap(addr, peerId string) {
	env := Envelope{
		Type: "bootstrap",
		From: self.ID,
		To:   peerId,
		Data: ""}

	jsonBs, err := json.Marshal(peerSelf)
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

	conn, err := net.Dial("udp", addr)
	if err != nil {
		log.Println(err)
		return
	}

	conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
	setStatus("bs sent")
}

func sendAnnounce(peer *Peer) {
	env := Envelope{
		Type: "announce",
		From: self.ID,
		To:   "",
		Data: ""}

	jsonAnnounce, err := json.Marshal(peerSelf)
	if err != nil {
		log.Println(err)
		return
	}

	signed := sign.Sign([]byte(jsonAnnounce), self.SignPrv)
	env.Data = hex.EncodeToString(signed)

	jsonEnv, err := json.Marshal(env)
	if err != nil {
		log.Println(err)
		return
	}

	peer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
	setStatus("announce sent")
}
