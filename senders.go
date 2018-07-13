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

func sendVerify(peer *Peer) {
	env := Envelope{
		Type: "verifybs",
		From: self.ID,
		To:   peer.ID,
		Data: ""}

	bs := MessageBootstrap{
		ID:      self.ID,
		Handle:  self.Handle,
		EncPub:  self.EncPub,
		Address: self.Address,
		SignPub: self.SignPub}

	jsonBs, err := json.Marshal(bs)
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

func sendTable(peer *Peer) {
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
		setStatus("no unique peers, table not sent")
		return
	}

	uniquePeers := make([]Peer, 0)
	for _, peer := range sendPeers {
		uniquePeers = append(uniquePeers, *peer)
	}

	for i := 0; i*64 < len(uniquePeers); i++ {
		jsonTable, err := json.Marshal(uniquePeers[i*64 : (i+1)*64])
		if err != nil {
			setStatus("error encoding peer list")
			continue
		}
		chatStatus(fmt.Sprintf("size of 64 peers in JSON: %d\n", len(jsonTable)))
	}
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

	bs := MessageBootstrap{
		ID:      self.ID,
		Handle:  self.Handle,
		EncPub:  self.EncPub,
		Address: self.Address,
		SignPub: self.SignPub}

	jsonBs, err := json.Marshal(bs)
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
