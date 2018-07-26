package main

import (
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
	case "disconnect":
		processDisconnect(env)
	case "request":
		processSuggestionRequest(env)
	case "suggestions":
		processSuggestions(env)
	case "ping":
		processPing(env)
	case "pulse":
		processPulse(env)
	case "verifybs":
		processVerify(env)
	default:
		chatStatus("unknown msg type: " + env.Type)
	}
}

func verifyEnvelope(env *Envelope, caller string) ([]byte, error) {
	min, err := idToMin(env.From)
	if err != nil {
		log.Println(err)
		return nil, errors.New(fmt.Sprintf("error bad id (%s:from)", caller))
	}

	data := env.Data
	verified := sign.Verify(data, min.SignPub)
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
		flood(env)
		seenChats[uniqueID] = true
	}

	cacheMin(chat.Min)
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

	if env.From != peer.ID() {
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

	if env.From != peer.ID() {
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

	_, seen := peerCache[peer.ID()]
	if !seen {
		peerConn, err := net.Dial("udp", peer.Address)
		if err != nil {
			log.Println(err)
			setStatus("could not connect to peer (bsverify)")
			return
		}

		peer.Conn = peerConn
		addPeer(peer)

		flood(env)
	}
}

func processSuggestionRequest(env *Envelope) {
	jsonData, err := verifyEnvelope(env, "request")
	if err != nil {
		setStatus(err.Error())
		return
	}

	var request MessageSuggestionRequest
	err = json.Unmarshal(jsonData, &request)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (request)")
		return
	}

	if request.To != peerSelf.ID() {
		setStatus("error invalid to (request)")
		return
	}

	peer := new(Peer)
	*peer = request.Peer

	peerConn, err := net.Dial("udp", peer.Address)
	if err != nil {
		log.Println(err)
		setStatus("could not connect to peer (request)")
		return
	}

	peer.Conn = peerConn

	sendSuggestions(peer, env.Data)

	_, seen := peerCache[peer.ID()]
	if !seen {
		addPeer(peer)
	}
}

func processSuggestions(env *Envelope) {
	jsonData, err := verifyEnvelope(env, "suggestions")
	if err != nil {
		setStatus(err.Error())
		return
	}

	var suggestions MessageSuggestions
	err = json.Unmarshal(jsonData, &suggestions)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (suggestions)")
		return
	}

	requestData := suggestions.RequestData
	verified := sign.Verify(requestData, self.SignPub)
	if !verified {
		setStatus("error originating req not signed self (suggestions)")
		return
	}

	peer := new(Peer)
	*peer = suggestions.Peer

	_, seen := peerCache[peer.ID()]
	if !seen {
		peerConn, err := net.Dial("udp", peer.Address)
		if err != nil {
			log.Println(err)
			setStatus("could not connect to peer (suggestions)")
			return
		}

		peer.Conn = peerConn
		addPeer(peer)
	}

	for _, newPeer := range suggestions.SuggestedPeers {
		_, seen := peerCache[newPeer.ID()]
		if !seen && wouldAddPeer(&newPeer) {
			peerConn, err := net.Dial("udp", newPeer.Address)
			if err != nil {
				log.Println(err)
				setStatus("could not connect to new peer (suggestions)")
				continue
			}

			newPeer.Conn = peerConn
			sendSuggestionRequest(&newPeer)
		}
	}
}

func processDisconnect(env *Envelope) {
	jsonData, err := verifyEnvelope(env, "disconnect")
	if err != nil {
		setStatus(err.Error())
		return
	}

	var messageTime MessageTime
	err = json.Unmarshal(jsonData, &messageTime)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (disconnect)")
		return
	}

	if messageTime.MessageType != -1 {
		setStatus("error invalid message type (disconnect)")
		return
	}

	idShort, err := idFront(env.From)
	if err != nil {
		setStatus("error bad id (disconnect)")
		return
	}

	removePeer(idShort)
	flood(env)
}

func processPing(env *Envelope) {
	jsonData, err := verifyEnvelope(env, "ping")
	if err != nil {
		setStatus(err.Error())
		return
	}

	var messagePing MessagePing
	err = json.Unmarshal(jsonData, &messagePing)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (ping)")
		return
	}

	if messagePing.MessageType != 0 {
		setStatus("error invalid message type (ping)")
		return
	}

	min := messagePing.Min
	sendPulse(min)
}

func processPulse(env *Envelope) {
	jsonData, err := verifyEnvelope(env, "pulse")
	if err != nil {
		setStatus(err.Error())
		return
	}

	var messageTime MessageTime
	err = json.Unmarshal(jsonData, &messageTime)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (pulse)")
		return
	}

	if messageTime.MessageType != 1 {
		setStatus("error invalid message type (pulse)")
		return
	}

	idShort, err := idFront(env.From)
	if err != nil {
		setStatus("error bad id (disconnect)")
		return
	}

	refreshPeer(idShort)
}
