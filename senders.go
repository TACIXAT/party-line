package main

import (
	"bytes"
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
			currPeer := currEntry.Peer

			if currPeer == nil {
				continue
			}

			_, sent := sentPeers[currPeer.ID()]
			if !sent {
				if currPeer.Conn != nil {
					currPeer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
				} else {
					chatStatus(fmt.Sprintf("currPeer conn nil %s", currPeer.ID()))
				}
				sentPeers[currPeer.ID()] = true
			}
		}
	}

	setStatus("flooded")
}

func sendSuggestionRequest(peer *Peer) {
	env := Envelope{
		Type: "request",
		From: self.ID,
		To:   peer.ID()}

	request := MessageSuggestionRequest{
		Peer: peerSelf,
		To:   peer.ID()}

	jsonReq, err := json.Marshal(request)
	if err != nil {
		log.Println(err)
		return
	}

	env.Data = sign.Sign([]byte(jsonReq), self.SignPrv)

	jsonEnv, err := json.Marshal(env)
	if err != nil {
		log.Println(err)
		return
	}

	peer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
	setStatus("suggestion request sent")
}

func sendSuggestions(peer *Peer, requestData []byte) {
	env := Envelope{
		Type: "suggestions",
		From: self.ID,
		To:   peer.ID()}

	// calculate ideal for id
	peerIdealTable := calculateIdealTable(peer.SignPub)

	// find closest for each and make a unique list
	peerSetHelper := make(map[string]bool)
	peerSet := make([]Peer, 0)
	peerSet = append(peerSet)
	for _, idInt := range peerIdealTable {
		closestPeerEntry := findClosest(idInt.Bytes())
		isRequestingPeer := bytes.Compare(peer.SignPub, closestPeerEntry.ID) == 0
		if closestPeerEntry.Peer != nil && !isRequestingPeer {
			closestPeer := closestPeerEntry.Peer
			_, contains := peerSetHelper[closestPeer.ID()]

			if !contains {
				peerSetHelper[closestPeer.ID()] = true
				peerSet = append(peerSet, *closestPeer)
			}
		}
	}

	// truncate
	// each encoded peer is about 300 bytes
	// this tops things off around 38kb
	// well below max udp packet size (65kb)
	if len(peerSet) > 128 {
		peerSet = peerSet[:128]
	}

	suggestions := MessageSuggestions{
		Peer:           peerSelf,
		RequestData:    requestData,
		SuggestedPeers: peerSet}

	jsonSuggestions, err := json.Marshal(suggestions)
	if err != nil {
		log.Println(err)
		return
	}

	env.Data = sign.Sign([]byte(jsonSuggestions), self.SignPrv)

	jsonEnv, err := json.Marshal(env)
	if err != nil {
		log.Println(err)
		return
	}

	peer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
}

func sendVerify(peer *Peer) {
	env := Envelope{
		Type: "verifybs",
		From: self.ID,
		To:   peer.ID()}

	jsonBs, err := json.Marshal(peerSelf)
	if err != nil {
		log.Println(err)
		return
	}

	env.Data = sign.Sign([]byte(jsonBs), self.SignPrv)

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
		To:   ""}

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
		currPeer := currEntry.Peer

		if currPeer == nil {
			continue
		}

		sendPeers[currPeer.ID()] = currPeer
	}

	if len(sendPeers) == 0 {
		chatStatus("you have no friends, bootstrap to a peer to get started")
		return
	}

	env.Data = sign.Sign([]byte(jsonChat), self.SignPrv)
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
		To:   peerId}

	jsonBs, err := json.Marshal(peerSelf)
	if err != nil {
		log.Println(err)
		return
	}

	env.Data = sign.Sign([]byte(jsonBs), self.SignPrv)

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
		To:   ""}

	jsonAnnounce, err := json.Marshal(peerSelf)
	if err != nil {
		log.Println(err)
		return
	}

	env.Data = sign.Sign([]byte(jsonAnnounce), self.SignPrv)

	jsonEnv, err := json.Marshal(env)
	if err != nil {
		log.Println(err)
		return
	}

	peer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
	setStatus("announce sent")
}

func sendDisconnect() {
	env := Envelope{
		Type: "disconnect",
		From: self.ID,
		To:   ""}

	disconnect := MessageTime{
		MessageType: -1,
		Time:        time.Now()}

	jsonDisconnect, err := json.Marshal(disconnect)
	if err != nil {
		log.Println(err)
		return
	}

	env.Data = sign.Sign([]byte(jsonDisconnect), self.SignPrv)
	flood(&env)
	setStatus("disconnect sent")
}

func sendPings() {
	for {
		time.Sleep(time.Second * 30)
		removeStalePeers()
		env := Envelope{
			Type: "ping",
			From: self.ID,
			To:   ""}

		ping := MessagePing{
			Min:         peerSelf.Min(),
			MessageType: 0,
			Time:        time.Now()}

		jsonPing, err := json.Marshal(ping)
		if err != nil {
			log.Println(err)
			return
		}

		env.Data = sign.Sign([]byte(jsonPing), self.SignPrv)

		jsonEnv, err := json.Marshal(env)
		if err != nil {
			log.Println(err)
			return
		}

		peerSeen := make(map[string]bool)
		for i := 0; i < 256; i++ {
			bucketList := peerTable[i]
			for curr := bucketList.Front(); curr != nil; curr = curr.Next() {
				entry := curr.Value.(*PeerEntry)
				if entry.Peer != nil {
					_, seen := peerSeen[entry.Peer.ID()]
					if !seen {
						entry.Peer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
						peerSeen[entry.Peer.ID()] = true
					}
				}
			}
		}
	}
}

func sendPulse(min MinPeer) {
	env := Envelope{
		Type: "pulse",
		From: self.ID,
		To:   ""}

	pulse := MessageTime{
		MessageType: 1,
		Time:        time.Now()}

	jsonPulse, err := json.Marshal(pulse)
	if err != nil {
		log.Println(err)
		return
	}

	env.Data = sign.Sign([]byte(jsonPulse), self.SignPrv)

	jsonEnv, err := json.Marshal(env)
	if err != nil {
		log.Println(err)
		return
	}

	route([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
}
