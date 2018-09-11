package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kevinburke/nacl/sign"
	"log"
	"net"
	"time"
)

var noReroute map[time.Time]bool

func init() {
	noReroute = make(map[time.Time]bool)
}

func processMessage(strMsg string) {
	env := new(Envelope)
	err := json.Unmarshal([]byte(strMsg), env)
	if err != nil {
		log.Println(err)
		log.Println(strMsg)
		setStatus("invalid json message received")
		return
	}

	if !env.Time.IsZero() && env.To != peerSelf.Id() {
		_, seen := noReroute[env.Time]
		if !seen {
			return
		}

		noReroute[env.Time] = true

		route(env)
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
	case "party":
		processParty(env)
	case "invite":
		processInvite(env)
	default:
		chatStatus("unknown msg type: " + env.Type)
	}

	// chatStatus(fmt.Sprintf("got %s", env.Type))
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
		return nil, errors.New(
			fmt.Sprintf("questionable message integrity discarding (%s)", caller))
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

	var msgChat MessageChat
	err = json.Unmarshal(jsonData, &msgChat)
	if err != nil {
		log.Println(err)
		setStatus("error invalid json (chat)")
		return
	}

	if msgChat.Min.Id() != env.From {
		setStatus("error invalid peer (chat)")
		return
	}

	uniqueId := env.From + "." + msgChat.Time.String()
	_, seen := seenChats[uniqueId]
	if !seen {
		chat := Chat{
			Time:    time.Now(),
			Id:      env.From,
			Channel: "mainline",
			Message: msgChat.Message}

		addChat(chat)
		flood(env)
		seenChats[uniqueId] = true
	}

	cacheMin(msgChat.Min)
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

	if env.From != peer.Id() {
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

	if env.From != peer.Id() {
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

	if peer.Id() == peerSelf.Id() {
		return
	}

	cache, seen := peerCache[peer.Id()]
	if !seen || !cache.Added {
		peerConn, err := net.Dial("udp", peer.Address)
		if err != nil {
			log.Println(err)
			setStatus("could not connect to peer (bsverify)")
			return
		}

		peer.Conn = peerConn
		addPeer(peer)
	}

	if !seen || !cache.Announced {
		flood(env)
		cache = peerCache[peer.Id()]
		cache.Announced = true
		peerCache[peer.Id()] = cache
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

	if request.To != peerSelf.Id() {
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

	cache, seen := peerCache[peer.Id()]
	if !seen || !cache.Added {
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

	cache, seen := peerCache[peer.Id()]
	if !seen || !cache.Added {
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
		cache, seen := peerCache[newPeer.Id()]
		if !seen && !cache.Added && wouldAddPeer(&newPeer) {
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

	cache, seen := peerCache[env.From]
	if !seen || !cache.Disconnected {
		cache.Disconnected = true
		peerCache[env.From] = cache

		flood(env)
	}
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
