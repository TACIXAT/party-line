package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/kevinburke/nacl/sign"
	"log"
	"net"
)

func forwardChat(env *Envelope) {
	jsonEnv, err := json.Marshal(env)

	sendPeers := make(map[string]*Peer)
	for _, list := range peerTable {
		curr := list.Front()
		currEntry := curr.Value.(*PeerEntry)
		currPeer := currEntry.Entry

		if currPeer == nil {
			continue
		}

		if err != nil {
			log.Println(err)
			continue
		}

		sendPeers[currPeer.ID] = currPeer
	}

	for _, peer := range sendPeers {
		peer.Conn.Write([]byte(fmt.Sprintf("%s\n", string(jsonEnv))))
	}

	setStatus("chat fowarded")
}

func processChat(env *Envelope) {
	fromPub, err := hex.DecodeString(env.From)
	if err != nil {
		log.Println(err)
		setStatus("error decoding hex (chat:from)")
		return
	}

	data, err := hex.DecodeString(env.Data)
	if err != nil {
		log.Println(err)
		setStatus("error decoding hex (chat:data)")
		return
	}

	verified := sign.Verify(data, fromPub)
	if !verified {
		setStatus("questionable message integrity discarding (chat)")
		return
	}

	jsonData := data[sign.SignatureSize:]

	var chat MessageChat
	err = json.Unmarshal(jsonData, &chat)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (chat)")
		return
	}

	uniqueID := env.From + "." + chat.Time.String()
	_, seen := seenChats[uniqueID]
	if !seen {
		displayChat(env.From, chat)
		forwardChat(env)
		seenChats[uniqueID] = true
	}
}

func processMessage(strMsg string) {
	env := new(Envelope)
	err := json.Unmarshal([]byte(strMsg), env)
	if err != nil {
		log.Println(err)
		setStatus("invalid json message received")
		return
	}

	switch env.Type {
	case "bootstrap":
		processBootstrap(env)
	case "verifybs":
		processVerify(env)
	case "chat":
		processChat(env)
	default:
		setStatus("unknown msg type: " + env.Type)
	}
}

func processBootstrap(env *Envelope) {
	fromPub, err := hex.DecodeString(env.From)
	if err != nil {
		log.Println(err)
		setStatus("error decoding hex (bs:from)")
		return
	}

	data, err := hex.DecodeString(env.Data)
	if err != nil {
		log.Println(err)
		setStatus("error decoding hex (bs:data)")
		return
	}

	verified := sign.Verify(data, fromPub)
	if !verified {
		setStatus("questionable message integrity discarding (bs)")
		return
	}

	jsonData := data[sign.SignatureSize:]

	var bs MessageBootstrap
	err = json.Unmarshal(jsonData, &bs)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (bs)")
		return
	}

	peer := new(Peer)
	peer.ID = bs.ID
	peer.Handle = bs.Handle
	peer.EncPub = bs.EncPub
	peer.SignPub = bs.SignPub
	peer.Address = bs.Address

	jsonPeer, err := json.Marshal(peer)
	if err != nil {
		chatStatus(fmt.Sprintf("size of encoded peer: %d", len(jsonPeer)))
	}

	if env.From != peer.ID {
		setStatus("id does not match from (bs)")
		return
	}

	peerConn, err := net.Dial("udp", peer.Address)
	if err != nil {
		log.Println(err)
		setStatus("could not connect to peer (bs)")
		return
	}

	peer.Conn = peerConn

	jsonPeer, err = json.Marshal(peer)
	if err != nil {
		chatStatus(fmt.Sprintf("size of encoded peer w/ conn: %d", len(jsonPeer)))
	}

	sendVerify(peer)
	addPeer(peer)
}

func processVerify(env *Envelope) {
	fromPub, err := hex.DecodeString(env.From)
	if err != nil {
		log.Println(err)
		setStatus("error decoding hex (bsverify:from)")
		return
	}

	data, err := hex.DecodeString(env.Data)
	if err != nil {
		log.Println(err)
		setStatus("error decoding hex (bsverify:data)")
		return
	}

	verified := sign.Verify(data, fromPub)
	if !verified {
		setStatus("questionable message integrity discarding (bsverify)")
		return
	}

	jsonData := data[sign.SignatureSize:]

	var bs MessageBootstrap
	err = json.Unmarshal(jsonData, &bs)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (bsverify)")
		return
	}

	peer := new(Peer)
	peer.ID = bs.ID
	peer.Handle = bs.Handle
	peer.EncPub = bs.EncPub
	peer.SignPub = bs.SignPub
	peer.Address = bs.Address

	if env.From != peer.ID {
		setStatus("id does not match from (bsverify)")
		return
	}

	peerConn, err := net.Dial("udp", peer.Address)
	if err != nil {
		log.Println(err)
		setStatus("could not connect to peer (bsverify)")
		return
	}

	peer.Conn = peerConn
	addPeer(peer)
	// add peers
	setStatus("bs success")
	chatStatus("happy chatting!")
}
