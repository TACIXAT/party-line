package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/kevinburke/nacl/sign"
	"log"
	"math/big"
	"net"
	"time"
)

func route(env *Envelope) {
	jsonEnv, err := json.Marshal(env)
	if err != nil {
		log.Println(err)
		return
	}

	idShortString, err := idFront(env.To)
	if err != nil {
		log.Println(err)
		return
	}

	idShortBytes, err := hex.DecodeString(idShortString)
	if err != nil {
		log.Println(err)
		return
	}

	shortId, err := idFront(env.To)
	if err != nil {
		log.Println(err)
		return
	}

	bytesId, err := hex.DecodeString(shortId)
	if err != nil {
		log.Println(err)
		return
	}

	idInt := new(big.Int)
	idInt.SetBytes(bytesId)

	selfDist := new(big.Int)
	selfDist.SetBytes(peerSelf.SignPub)
	selfDist.Xor(selfDist, idInt)

	closest := findClosestN(idShortBytes, 3)
	for _, peerEntry := range closest {
		peer := peerEntry.Peer
		if peer != nil {
			peerDist := new(big.Int)
			peerDist.SetBytes(peer.SignPub)
			peerDist.Xor(peerDist, idInt)

			if peerDist.Cmp(selfDist) < 0 {
				peer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
			}
		}
	}
}

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

			_, sent := sentPeers[currPeer.Id()]
			if !sent {
				if currPeer.Conn != nil {
					currPeer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
				} else {
					chatStatus(fmt.Sprintf("currPeer conn nil %s", currPeer.Id()))
				}
				sentPeers[currPeer.Id()] = true
			}
		}
	}

	setStatus("flooded")
}

func sendSuggestionRequest(peer *Peer) {
	env := Envelope{
		Type: "request",
		From: peerSelf.Id(),
		To:   peer.Id()}

	request := MessageSuggestionRequest{
		Peer: peerSelf,
		To:   peer.Id()}

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
		From: peerSelf.Id(),
		To:   peer.Id()}

	// calculate ideal for id
	peerIdealTable := calculateIdealTable(peer.SignPub)

	// find closest for each and make a unique list
	peerSetHelper := make(map[string]bool)
	peerSet := make([]Peer, 0)
	peerSet = append(peerSet)
	for _, idInt := range peerIdealTable {
		closestPeerEntry := findClosest(idInt.Bytes())
		if closestPeerEntry == nil {
			continue
		}

		if bytes.Compare(peer.SignPub, closestPeerEntry.Id) == 0 {
			continue
		}

		closestPeer := closestPeerEntry.Peer
		_, contains := peerSetHelper[closestPeer.Id()]

		if !contains {
			peerSetHelper[closestPeer.Id()] = true
			peerSet = append(peerSet, *closestPeer)
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

func sendBootstrap(addr, peerId string) {
	env := Envelope{
		Type: "bootstrap",
		From: peerSelf.Id(),
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

func sendVerify(peer *Peer) {
	env := Envelope{
		Type: "verifybs",
		From: peerSelf.Id(),
		To:   peer.Id()}

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
		From: peerSelf.Id(),
		To:   ""}

	msgChat := MessageChat{
		Message: msg,
		Time:    time.Now().UTC(),
		Min:     peerSelf.Min()}

	jsonChat, err := json.Marshal(msgChat)
	if err != nil {
		log.Println(err)
		return
	}

	sendPeers := make(map[string]*Peer)
	for _, list := range peerTable {
		curr := list.Front()
		if curr == nil {
			continue
		}

		currEntry := curr.Value.(*PeerEntry)
		currPeer := currEntry.Peer

		sendPeers[currPeer.Id()] = currPeer
	}

	if len(sendPeers) == 0 {
		chatStatus("you have no friends, bootstrap to a peer to get started")
		return
	}

	env.Data = sign.Sign([]byte(jsonChat), self.SignPrv)
	jsonEnv, err := json.Marshal(env)
	if err != nil {
		log.Println(err)
		setStatus("error marshalling env to json")
		return
	}

	for _, peer := range sendPeers {
		// closed := box.EasySeal([]byte(jsonChat), peer.EncPub, self.EncPrv)
		peer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
	}
	setStatus("chat sent")
}

func sendAnnounce(peer *Peer) {
	env := Envelope{
		Type: "announce",
		From: peerSelf.Id(),
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
		From: peerSelf.Id(),
		To:   ""}

	disconnect := MessageTime{
		MessageType: -1,
		Time:        time.Now().UTC()}

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
			From: peerSelf.Id(),
			To:   ""}

		ping := MessagePing{
			Min:         peerSelf.Min(),
			MessageType: 0,
			Time:        time.Now().UTC()}

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
					_, seen := peerSeen[entry.Peer.Id()]
					if !seen {
						entry.Peer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
						peerSeen[entry.Peer.Id()] = true
					}
				}
			}
		}
	}
}

func sendPulse(min MinPeer) {
	env := Envelope{
		Type: "pulse",
		From: peerSelf.Id(),
		To:   min.Id()}

	pulse := MessageTime{
		MessageType: 1,
		Time:        time.Now().UTC()}

	jsonPulse, err := json.Marshal(pulse)
	if err != nil {
		log.Println(err)
		return
	}

	env.Data = sign.Sign([]byte(jsonPulse), self.SignPrv)

	route(&env)
}
