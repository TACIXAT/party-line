package whitebox

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TACIXAT/party-line/party-lib"
	"github.com/kevinburke/nacl/sign"
	"log"
	"net"
	"time"
)

var noReroute map[time.Time]bool

func init() {
	noReroute = make(map[time.Time]bool)
}

func (wb *WhiteBox) processMessage(strMsg string) {
	env := new(Envelope)
	err := json.Unmarshal([]byte(strMsg), env)
	if err != nil {
		log.Println(err)
		log.Println(strMsg)
		wb.setStatus("invalid json message received")
		return
	}

	if !env.Time.IsZero() && env.To != wb.PeerSelf.Id() {
		_, seen := noReroute[env.Time]
		if !seen {
			return
		}

		noReroute[env.Time] = true

		wb.route(env)
		return
	}

	switch env.Type {
	case "announce":
		wb.processAnnounce(env)
	case "bootstrap":
		wb.processBootstrap(env)
	case "chat":
		wb.processChat(env)
	case "disconnect":
		wb.processDisconnect(env)
	case "request":
		wb.processSuggestionRequest(env)
	case "suggestions":
		wb.processSuggestions(env)
	case "ping":
		wb.processPing(env)
	case "pulse":
		wb.processPulse(env)
	case "verifybs":
		wb.processVerify(env)
	case "party":
		wb.processParty(env)
	case "invite":
		wb.processInvite(env)
	default:
		wb.chatStatus("unknown msg type: " + env.Type) // TODO: chat status
	}

	// chatStatus(fmt.Sprintf("got %s", env.Type))
}

// TODO: lib candidate
func (wb *WhiteBox) verifyEnvelope(env *Envelope, caller string) ([]byte, error) {
	min, err := wb.IdToMin(env.From)
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

func (wb *WhiteBox) processChat(env *Envelope) {
	jsonData, err := wb.verifyEnvelope(env, "chat")
	if err != nil {
		wb.setStatus(err.Error())
		return
	}

	var msgChat MessageChat
	err = json.Unmarshal(jsonData, &msgChat)
	if err != nil {
		log.Println(err)
		wb.setStatus("error invalid json (chat)")
		return
	}

	if msgChat.Min.Id() != env.From {
		wb.setStatus("error invalid peer (chat)")
		return
	}

	uniqueId := env.From + "." + msgChat.Time.String()
	_, seen := wb.SeenChats[uniqueId]
	if !seen {
		chat := partylib.Chat{
			Time:    time.Now(),
			Id:      env.From,
			Channel: "mainline",
			Message: msgChat.Message}

		wb.addChat(chat)
		wb.flood(env)
		wb.SeenChats[uniqueId] = true
	}

	wb.cacheMin(msgChat.Min)
}

func (wb *WhiteBox) processBootstrap(env *Envelope) {
	jsonData, err := wb.verifyEnvelope(env, "bs")
	if err != nil {
		wb.setStatus(err.Error())
		return
	}

	peer := new(Peer)
	err = json.Unmarshal(jsonData, peer)
	if err != nil {
		log.Println(err)
		wb.setStatus("error invalid json (bs)")
		return
	}

	if env.From != peer.Id() {
		wb.setStatus("id does not match from (bs)")
		return
	}

	peerConn, err := net.Dial("udp", peer.Address)
	if err != nil {
		log.Println(err)
		wb.setStatus("could not connect to peer (bs)")
		return
	}

	peer.Conn = peerConn

	wb.sendVerify(peer)
	wb.addPeer(peer)
}

func (wb *WhiteBox) processVerify(env *Envelope) {
	jsonData, err := wb.verifyEnvelope(env, "bsverify")
	if err != nil {
		wb.setStatus(err.Error())
		return
	}

	peer := new(Peer)
	err = json.Unmarshal(jsonData, peer)
	if err != nil {
		log.Println(err)
		wb.setStatus("error invalid json (bsverify)")
		return
	}

	if env.From != peer.Id() {
		wb.setStatus("id does not match from (bsverify)")
		return
	}

	peerConn, err := net.Dial("udp", peer.Address)
	if err != nil {
		log.Println(err)
		wb.setStatus("could not connect to peer (bsverify)")
		return
	}

	peer.Conn = peerConn
	wb.addPeer(peer)
	wb.setStatus("verified")
	wb.sendAnnounce(peer)
	wb.sendSuggestionRequest(peer)
}

func (wb *WhiteBox) processAnnounce(env *Envelope) {
	jsonData, err := wb.verifyEnvelope(env, "announce")
	if err != nil {
		wb.setStatus(err.Error())
		return
	}

	peer := new(Peer)
	err = json.Unmarshal(jsonData, peer)
	if err != nil {
		log.Println(err)
		wb.setStatus("error invalid json (announce)")
		return
	}

	if peer.Id() == wb.PeerSelf.Id() {
		return
	}

	cache, seen := wb.PeerCache[peer.Id()]
	if !seen || !cache.Added {
		peerConn, err := net.Dial("udp", peer.Address)
		if err != nil {
			log.Println(err)
			wb.setStatus("could not connect to peer (bsverify)")
			return
		}

		peer.Conn = peerConn
		wb.addPeer(peer)
	}

	if !seen || !cache.Announced {
		wb.flood(env)
		cache = wb.PeerCache[peer.Id()]
		cache.Announced = true
		wb.PeerCache[peer.Id()] = cache
	}
}

func (wb *WhiteBox) processSuggestionRequest(env *Envelope) {
	jsonData, err := wb.verifyEnvelope(env, "request")
	if err != nil {
		wb.setStatus(err.Error())
		return
	}

	var request MessageSuggestionRequest
	err = json.Unmarshal(jsonData, &request)
	if err != nil {
		log.Println(err)
		wb.setStatus("error invalid json (request)")
		return
	}

	if request.To != wb.PeerSelf.Id() {
		wb.setStatus("error invalid to (request)")
		return
	}

	peer := new(Peer)
	*peer = request.Peer

	peerConn, err := net.Dial("udp", peer.Address)
	if err != nil {
		log.Println(err)
		wb.setStatus("could not connect to peer (request)")
		return
	}

	peer.Conn = peerConn

	wb.sendSuggestions(peer, env.Data)

	cache, seen := wb.PeerCache[peer.Id()]
	if !seen || !cache.Added {
		wb.addPeer(peer)
	}
}

func (wb *WhiteBox) processSuggestions(env *Envelope) {
	jsonData, err := wb.verifyEnvelope(env, "suggestions")
	if err != nil {
		wb.setStatus(err.Error())
		return
	}

	var suggestions MessageSuggestions
	err = json.Unmarshal(jsonData, &suggestions)
	if err != nil {
		log.Println(err)
		wb.setStatus("error invalid json (suggestions)")
		return
	}

	requestData := suggestions.RequestData
	verified := sign.Verify(requestData, wb.Self.SignPub)
	if !verified {
		wb.setStatus("error originating req not signed self (suggestions)")
		return
	}

	peer := new(Peer)
	*peer = suggestions.Peer

	cache, seen := wb.PeerCache[peer.Id()]
	if !seen || !cache.Added {
		peerConn, err := net.Dial("udp", peer.Address)
		if err != nil {
			log.Println(err)
			wb.setStatus("could not connect to peer (suggestions)")
			return
		}

		peer.Conn = peerConn
		wb.addPeer(peer)
	}

	for _, newPeer := range suggestions.SuggestedPeers {
		cache, seen := wb.PeerCache[newPeer.Id()]
		if !seen && !cache.Added && wouldAddPeer(&newPeer) {
			peerConn, err := net.Dial("udp", newPeer.Address)
			if err != nil {
				log.Println(err)
				wb.setStatus("could not connect to new peer (suggestions)")
				continue
			}

			newPeer.Conn = peerConn
			wb.sendSuggestionRequest(&newPeer)
		}
	}
}

func (wb *WhiteBox) processDisconnect(env *Envelope) {
	jsonData, err := wb.verifyEnvelope(env, "disconnect")
	if err != nil {
		wb.setStatus(err.Error())
		return
	}

	var messageTime MessageTime
	err = json.Unmarshal(jsonData, &messageTime)
	if err != nil {
		log.Println(err)
		wb.setStatus("error invalid json (disconnect)")
		return
	}

	if messageTime.MessageType != -1 {
		wb.setStatus("error invalid message type (disconnect)")
		return
	}

	idShort, err := wb.IdFront(env.From)
	if err != nil {
		wb.setStatus("error bad id (disconnect)")
		return
	}

	wb.removePeer(idShort)

	cache, seen := wb.PeerCache[env.From]
	if !seen || !cache.Disconnected {
		cache.Disconnected = true
		wb.PeerCache[env.From] = cache

		wb.flood(env)
	}
}

func (wb *WhiteBox) processPing(env *Envelope) {
	jsonData, err := wb.verifyEnvelope(env, "ping")
	if err != nil {
		wb.setStatus(err.Error())
		return
	}

	var messagePing MessagePing
	err = json.Unmarshal(jsonData, &messagePing)
	if err != nil {
		log.Println(err)
		wb.setStatus("error invalid json (ping)")
		return
	}

	if messagePing.MessageType != 0 {
		wb.setStatus("error invalid message type (ping)")
		return
	}

	min := messagePing.Min
	wb.sendPulse(min)
}

func (wb *WhiteBox) processPulse(env *Envelope) {
	jsonData, err := wb.verifyEnvelope(env, "pulse")
	if err != nil {
		wb.setStatus(err.Error())
		return
	}

	var messageTime MessageTime
	err = json.Unmarshal(jsonData, &messageTime)
	if err != nil {
		log.Println(err)
		wb.setStatus("error invalid json (pulse)")
		return
	}

	if messageTime.MessageType != 1 {
		wb.setStatus("error invalid message type (pulse)")
		return
	}

	idShort, err := wb.IdFront(env.From)
	if err != nil {
		wb.setStatus("error bad id (disconnect)")
		return
	}

	refreshPeer(idShort)
}
