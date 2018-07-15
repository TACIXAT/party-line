package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kevinburke/nacl/sign"
	"log"
	"net"
)

func processMessage(strMsg string) {
	env := new(Envelope)
	err := json.Unmarshal([]byte(strMsg), env)
	if err != nil {
		log.Println(err)
		setStatus("invalid json message received")
		return
	}

	switch env.Type {
	case "announce":
		processAnnounce(env)
	case "bootstrap":
		processBootstrap(env)
	case "chat":
		processChat(env)
	case "verifybs":
		processVerify(env)
	default:
		chatStatus("unknown msg type: " + env.Type)
	}
}

func verifyEnvelope(env *Envelope, caller string) ([]byte, error) {
	fromPub, err := hex.DecodeString(env.From)
	if err != nil {
		log.Println(err)
		return nil, errors.New(fmt.Sprintf("error decoding hex (%s:from)", caller))
	}

	data, err := hex.DecodeString(env.Data)
	if err != nil {
		log.Println(err)
		return nil, errors.New(fmt.Sprintf("error decoding hex (%s:data)", caller))
	}

	verified := sign.Verify(data, fromPub)
	if !verified {
		return nil, errors.New(fmt.Sprintf("questionable message integrity discarding (%s)", caller))
	}

	jsonData := data[sign.SignatureSize:]
	return jsonData, nil
}

func processChat(env *Envelope) {
	jsonData, err := verifyEnvelope(env, "chat")
	if err != nil {
		setStatus(err.Error())
		return
	}

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

func processBootstrap(env *Envelope) {
	jsonData, err := verifyEnvelope(env, "bs")
	if err != nil {
		setStatus(err.Error())
		return
	}

	peer := new(Peer)
	err = json.Unmarshal(jsonData, peer)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (bs)")
		return
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

	sendVerify(peer)
	addPeer(peer)
}

func processVerify(env *Envelope) {
	jsonData, err := verifyEnvelope(env, "bsverify")
	if err != nil {
		setStatus(err.Error())
		return
	}

	peer := new(Peer)
	err = json.Unmarshal(jsonData, peer)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (bsverify)")
		return
	}

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
	setStatus("verified")
	sendAnnounce(peer)
	sendSuggestionRequest(peer)
	chatStatus("happy chatting!")
}

func processAnnounce(env *Envelope) {
	jsonData, err := verifyEnvelope(env, "announce")
	if err != nil {
		setStatus(err.Error())
		return
	}

	peer := new(Peer)
	err = json.Unmarshal(jsonData, peer)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (announce)")
		return
	}

	_, seen := seenPeers[peer.ID]
	if !seen {
		peerConn, err := net.Dial("udp", peer.Address)
		if err != nil {
			log.Println(err)
			setStatus("could not connect to peer (bsverify)")
			return
		}

		peer.Conn = peerConn
		addPeer(peer)
		seenPeers[peer.ID] = true

		flood(env)
	}
}

// func processSuggestionRequest(env *Envelope) {
// 	jsonData, err := verifyEnvelope(env, "suggestionreq")
// 	if err != nil {
// 		setStatus(err.Error())
// 		return
// 	}

// 	sendSuggestions(peer)
// }
