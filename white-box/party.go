package whitebox

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/kevinburke/nacl/box"
	"github.com/kevinburke/nacl/sign"
	"io"
	"io/ioutil"
	"log"
	mrand "math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

func init() {
	mrand.Seed(time.Now().UTC().UnixNano())
}

type VerifiedBlock struct {
	Block        *Block
	PackFileInfo *PackFileInfo
	Hash         string
}

type LockingMinList struct {
	Map   map[string]int
	Mutex *sync.Mutex
}

func (lml LockingMinList) Len() int {
	lml.Mutex.Lock()
	defer lml.Mutex.Unlock()
	return len(lml.Map)
}

func (lml LockingMinList) Set(key string, value int) {
	lml.Mutex.Lock()
	defer lml.Mutex.Unlock()
	lml.Map[key] = value
}

func (lml LockingMinList) Get(key string) (int, bool) {
	lml.Mutex.Lock()
	defer lml.Mutex.Unlock()
	value, ok := lml.Map[key]
	return value, ok
}

type PartyLine struct {
	MinList   LockingMinList
	Id        string
	SeenChats map[string]bool        `json:"-"`
	Packs     map[string]LockingPack `json:"-"`
	PacksLock *sync.Mutex            `json:"-"`
	WhiteBox  *WhiteBox              `json:"-"`
}

type PartyEnvelope struct {
	Type    string
	From    string
	PartyId string
	Data    []byte
}

type PartyAnnounce struct {
	PeerId  string
	PartyId string
}

type PartyChat struct {
	PeerId  string
	PartyId string
	Message string
	Time    time.Time
}

type PartyDisconnect struct {
	PeerId  string
	PartyId string
	Time    time.Time
}

type PartyAdvertisement struct {
	PeerId  string
	PartyId string
	Time    time.Time
	Hash    string
	Pack    Pack
}

type PartyRequest struct {
	PeerId   string
	PackHash string
	FileHash string
	Coverage []uint64
	Time     time.Time
	PartyId  string
}

type PartyFulfillment struct {
	PeerId   string
	PackHash string
	FileHash string
	PartyId  string
	Block    Block
}

type Since struct {
	Received time.Time
	Reported time.Time
}

func modFloor(i, m int) int {
	return ((i % m) + m) % m
}

func (party *PartyLine) getNeighbors() map[string]bool {
	sortedIds := make([]string, 0, party.MinList.Len())
	party.MinList.Mutex.Lock()
	for id, _ := range party.MinList.Map {
		sortedIds = append(sortedIds, id)
	}
	party.MinList.Mutex.Unlock()
	sort.Strings(sortedIds)

	var idx int = -1
	selfId := party.WhiteBox.PeerSelf.Id()
	for i, id := range sortedIds {
		if id == selfId {
			idx = i
			break
		}
	}

	neighbors := make(map[string]bool)
	if idx == -1 {
		party.WhiteBox.setStatus("error could not find self in min list")
		return neighbors
	}

	n1 := modFloor((idx - 1), len(sortedIds))
	n2 := modFloor((idx + 1), len(sortedIds))
	n3 := modFloor((idx + 2), len(sortedIds))

	neighbors[sortedIds[n1]] = true
	neighbors[sortedIds[n2]] = true
	neighbors[sortedIds[n3]] = true

	return neighbors
}

func (party *PartyLine) SendInvite(min *MinPeer) {
	env := Envelope{
		Type: "invite",
		From: party.WhiteBox.PeerSelf.Id(),
		To:   min.Id()}

	// keep message small so we don't limit party size
	// hopefully this doesn't fuck up delivery
	var sendParty *PartyLine = party
	if party.MinList.Len() > 20 {
		sendParty = new(PartyLine)
		sendParty.Id = party.Id
		idx := 0
		party.MinList.Mutex.Lock()
		for id, _ := range party.MinList.Map {
			sendParty.MinList.Set(id, 0)
			idx++
			if idx > 20 {
				break
			}
		}
		party.MinList.Mutex.Unlock()
	}

	jsonInvite, err := json.Marshal(sendParty)
	if err != nil {
		log.Println(err)
		return
	}

	closed := box.EasySeal(
		[]byte(jsonInvite), min.EncPub, party.WhiteBox.Self.EncPrv)
	env.Data = closed

	party.WhiteBox.route(&env)
	party.WhiteBox.setStatus("invite sent")
}

func (party *PartyLine) SendAnnounce() {
	env := Envelope{
		Type: "party",
		From: party.WhiteBox.PeerSelf.Id(),
		To:   ""}

	partyEnv := PartyEnvelope{
		Type:    "announce",
		From:    party.WhiteBox.PeerSelf.Id(),
		PartyId: party.Id}

	partyAnnounce := PartyAnnounce{
		PeerId:  party.WhiteBox.PeerSelf.Id(),
		PartyId: party.Id}

	jsonPartyAnnounce, err := json.Marshal(partyAnnounce)
	if err != nil {
		log.Println(err)
		return
	}

	signedPartyAnnounce := sign.Sign(
		[]byte(jsonPartyAnnounce), party.WhiteBox.Self.SignPrv)
	partyEnv.Data = signedPartyAnnounce

	jsonPartyEnv, err := json.Marshal(partyEnv)
	if err != nil {
		log.Println(err)
		return
	}

	party.MinList.Mutex.Lock()
	defer party.MinList.Mutex.Unlock()
	for idMin, _ := range party.MinList.Map {
		min, err := party.WhiteBox.IdToMin(idMin)
		if err != nil {
			party.WhiteBox.setStatus(err.Error())
			continue
		}

		closed := box.EasySeal(
			[]byte(jsonPartyEnv), min.EncPub, party.WhiteBox.Self.EncPrv)
		env.Data = closed
		env.To = idMin

		party.WhiteBox.route(&env)
	}
}

func (party *PartyLine) sendToNeighbors(
	msgType string, signedPartyData []byte) {
	env := Envelope{
		Type: "party",
		From: party.WhiteBox.PeerSelf.Id(),
		To:   ""}

	partyEnv := PartyEnvelope{
		Type:    msgType,
		From:    party.WhiteBox.PeerSelf.Id(),
		PartyId: party.Id}

	partyEnv.Data = signedPartyData

	jsonPartyEnv, err := json.Marshal(partyEnv)
	if err != nil {
		log.Println(err)
		return
	}

	neighbors := party.getNeighbors()
	for idMin, _ := range neighbors {
		min, err := party.WhiteBox.IdToMin(idMin)
		if err != nil {
			party.WhiteBox.setStatus(err.Error())
			continue
		}

		closed := box.EasySeal(
			[]byte(jsonPartyEnv), min.EncPub, party.WhiteBox.Self.EncPrv)
		env.Data = closed
		env.To = idMin

		party.WhiteBox.route(&env)
	}
}

func (party *PartyLine) SendChat(message string) {
	partyChat := PartyChat{
		PeerId:  party.WhiteBox.PeerSelf.Id(),
		PartyId: party.Id,
		Message: message,
		Time:    time.Now().UTC()}

	jsonPartyChat, err := json.Marshal(partyChat)
	if err != nil {
		log.Println(err)
		return
	}

	signedPartyChat := sign.Sign(
		[]byte(jsonPartyChat), party.WhiteBox.Self.SignPrv)

	party.sendToNeighbors("chat", signedPartyChat)
}

func (party *PartyLine) SendDisconnect() {
	partyDisconnect := PartyDisconnect{
		PeerId:  party.WhiteBox.PeerSelf.Id(),
		PartyId: party.Id,
		Time:    time.Now().UTC()}

	jsonPartyDisconnect, err := json.Marshal(partyDisconnect)
	if err != nil {
		log.Println(err)
		return
	}

	delete(party.WhiteBox.Parties.Map, party.Id)

	signedPartyDisconnect := sign.Sign(
		[]byte(jsonPartyDisconnect), party.WhiteBox.Self.SignPrv)
	party.sendToNeighbors("disconnect", signedPartyDisconnect)
}

func (party *PartyLine) SendAdvertisement(packSha256 string, pack *Pack) {
	partyAdvertisement := PartyAdvertisement{
		PeerId:  party.WhiteBox.PeerSelf.Id(),
		PartyId: party.Id,
		Time:    time.Now().UTC(),
		Hash:    packSha256,
		Pack:    *pack}

	jsonPartyAdvertisement, err := json.Marshal(partyAdvertisement)
	if err != nil {
		log.Println(err)
		return
	}

	signedPartyAdvertisement := sign.Sign(
		[]byte(jsonPartyAdvertisement), party.WhiteBox.Self.SignPrv)

	party.sendToNeighbors("ad", signedPartyAdvertisement)
}

func (party *PartyLine) ProcessAdvertisement(partyEnv *PartyEnvelope) {
	signedPartyAdvertisement := partyEnv.Data
	jsonPartyAdvertisement := signedPartyAdvertisement[sign.SignatureSize:]

	partyAdvertisement := new(PartyAdvertisement)
	err := json.Unmarshal(jsonPartyAdvertisement, partyAdvertisement)
	if err != nil {
		log.Println(err)
		party.WhiteBox.setStatus("error invalid json (party:ad)")
		return
	}

	if party.Id != partyAdvertisement.PartyId {
		party.WhiteBox.setStatus("error invalid party id for (party:ad)")
		return
	}

	min, err := party.WhiteBox.IdToMin(partyAdvertisement.PeerId)
	if err != nil {
		party.WhiteBox.setStatus("error bad id (party:ad)")
		return
	}

	verified := sign.Verify(signedPartyAdvertisement, min.SignPub)
	if !verified {
		party.WhiteBox.setStatus(
			"error questionable message integrity (party:ad)")
		return
	}

	newPack := new(Pack)
	*newPack = partyAdvertisement.Pack
	// DATA RACE: with files:227 (sha256Pack)
	// DATA RACE: with party:660 (StartPack)
	// DATA RACE: with client_test:243 (testGetPack)
	newPack.Peers = make(map[string]time.Time)
	newPack.FileLock = new(sync.Mutex)

	hash := partyAdvertisement.Hash
	if hash != sha256Pack(newPack) {
		party.WhiteBox.setStatus("error bad pack hash (party:ad)")
		return
	}

	party.PacksLock.Lock()
	lockingPack, ok := party.Packs[hash]
	party.PacksLock.Unlock()

	if !ok {
		newPack.State = AVAILABLE

		lockingPack.Mutex = new(sync.Mutex)
		lockingPack.Mutex.Lock()
		defer lockingPack.Mutex.Unlock()
		lockingPack.Pack = newPack

		party.PacksLock.Lock()
		party.Packs[hash] = lockingPack
		party.PacksLock.Unlock()
		// DATA RACE: with client_test:235 (testGetPack)
		// DATA RACE: with client_test:36 (checkPack)

		lockingPack.Pack.FileLock.Lock()
		for _, file := range lockingPack.Pack.Files {
			file.BlockMap = make(map[string]BlockInfo)
			file.BlockLookup = make(map[uint64]string)
			file.Coverage = make([]uint64, 0)
			file.Path = ""
		}
		lockingPack.Pack.FileLock.Unlock()
	}

	adTime := partyAdvertisement.Time
	peerTime, ok := lockingPack.Pack.Peers[min.Id()]
	if !ok || peerTime.Before(adTime) {
		if adTime.Sub(peerTime) > 30*time.Second {
			party.sendToNeighbors("ad", signedPartyAdvertisement)
		}
		lockingPack.Pack.Peers[min.Id()] = adTime
	}
}

func (party *PartyLine) ProcessChat(partyEnv *PartyEnvelope) {
	signedPartyChat := partyEnv.Data
	jsonPartyChat := signedPartyChat[sign.SignatureSize:]

	partyChat := new(PartyChat)
	err := json.Unmarshal(jsonPartyChat, partyChat)
	if err != nil {
		log.Println(err)
		party.WhiteBox.setStatus("error invalid json (party:chat)")
		return
	}

	if partyChat.PartyId != party.Id {
		party.WhiteBox.setStatus("error invalid party (party:chat)")
		return
	}

	min, err := party.WhiteBox.IdToMin(partyChat.PeerId)
	if err != nil {
		party.WhiteBox.setStatus("error bad id (party:chat)")
		return
	}

	verified := sign.Verify(signedPartyChat, min.SignPub)
	if !verified {
		party.WhiteBox.setStatus(
			"error questionable message integrity (party:chat)")
		return
	}

	chatId := fmt.Sprintf("%s.%s", partyChat.PeerId, partyChat.Time.String())
	_, seen := party.SeenChats[chatId]
	if !seen {
		party.SeenChats[chatId] = true

		chat := Chat{
			Time:    time.Now().UTC(),
			Id:      partyChat.PeerId,
			Channel: party.Id,
			Message: partyChat.Message}

		party.WhiteBox.addChat(chat)

		party.sendToNeighbors("chat", signedPartyChat)
	}
}

func (party *PartyLine) ProcessDisconnect(partyEnv *PartyEnvelope) {
	signedPartyDisconnect := partyEnv.Data
	jsonPartyDisconnect := signedPartyDisconnect[sign.SignatureSize:]

	partyDisconnect := new(PartyDisconnect)
	err := json.Unmarshal(jsonPartyDisconnect, partyDisconnect)
	if err != nil {
		log.Println(err)
		party.WhiteBox.setStatus("error invalid json (party:disconnect)")
		return
	}

	if partyDisconnect.PartyId != party.Id {
		party.WhiteBox.setStatus("error invalid party (party:disconnect)")
		return
	}

	min, err := party.WhiteBox.IdToMin(partyDisconnect.PeerId)
	if err != nil {
		party.WhiteBox.setStatus("error bad id (party:disconnect)")
		return
	}

	verified := sign.Verify(signedPartyDisconnect, min.SignPub)
	if !verified {
		party.WhiteBox.setStatus(
			"error questionable message integrity (party:disconnect)")
		return
	}

	if time.Since(partyDisconnect.Time) > 200*time.Second {
		party.WhiteBox.setStatus(
			"error time exceeds max allowable (party:disconnect)")
		return
	}

	_, seen := party.MinList.Get(partyDisconnect.PeerId)

	if seen {
		party.MinList.Mutex.Lock()
		delete(party.MinList.Map, partyDisconnect.PeerId)
		party.MinList.Mutex.Unlock()
		party.sendToNeighbors("disconnect", signedPartyDisconnect)
	}
}

func (party *PartyLine) ProcessAnnounce(partyEnv *PartyEnvelope) {
	signedPartyAnnounce := partyEnv.Data
	jsonPartyAnnounce := signedPartyAnnounce[sign.SignatureSize:]

	partyAnnounce := new(PartyAnnounce)
	err := json.Unmarshal(jsonPartyAnnounce, partyAnnounce)
	if err != nil {
		log.Println(err)
		party.WhiteBox.setStatus("error invalid json (party:announce)")
		return
	}

	if partyAnnounce.PartyId != party.Id {
		party.WhiteBox.setStatus("error invalid party (party:announce)")
		return
	}

	min, err := party.WhiteBox.IdToMin(partyAnnounce.PeerId)
	if err != nil {
		party.WhiteBox.setStatus("error bad id (party:announce)")
		return
	}

	verified := sign.Verify(signedPartyAnnounce, min.SignPub)
	if !verified {
		party.WhiteBox.setStatus(
			"error questionable message integrity (party:announce)")
		return
	}

	_, seen := party.MinList.Get(partyAnnounce.PeerId)

	if !seen {
		party.MinList.Set(partyAnnounce.PeerId, 0)
		party.sendToNeighbors("announce", signedPartyAnnounce)
	}
}

func (wb *WhiteBox) processParty(env *Envelope) {
	min, err := wb.IdToMin(env.From)
	if err != nil {
		wb.setStatus(err.Error())
		return
	}

	jsonData, err := box.EasyOpen(env.Data, min.EncPub, wb.Self.EncPrv)
	if err != nil {
		wb.setStatus("error invalid crypto (party)")
		return
	}

	partyEnv := new(PartyEnvelope)
	err = json.Unmarshal(jsonData, partyEnv)
	if err != nil {
		log.Println(err)
		wb.setStatus("error invalid json (party)")
		return
	}

	wb.Parties.Mutex.Lock()
	party, exists := wb.Parties.Map[partyEnv.PartyId]
	wb.Parties.Mutex.Unlock()

	if !exists {
		wb.setStatus("error invalid party (party)")
		return
	}

	switch partyEnv.Type {
	case "ad":
		party.ProcessAdvertisement(partyEnv)
	case "announce":
		party.ProcessAnnounce(partyEnv)
	case "chat":
		party.ProcessChat(partyEnv)
	case "disconnect":
		party.ProcessDisconnect(partyEnv)
	case "request":
		party.ProcessRequest(partyEnv)
	case "fulfillment":
		party.ProcessFulfillment(partyEnv)
	default:
		wb.setStatus(
			fmt.Sprintf("unknown message type %s (party)", partyEnv.Type))
	}

	// chatStatus(fmt.Sprintf("got %s", partyEnv.Type))

	if env.From == partyEnv.From && partyEnv.Type != "disconnect" {
		party.MinList.Set(partyEnv.From, 0)
	}
}

func (wb *WhiteBox) processInvite(env *Envelope) {
	min, err := wb.IdToMin(env.From)
	if err != nil {
		wb.setStatus(err.Error())
		return
	}

	jsonData, err := box.EasyOpen(
		env.Data, min.EncPub, wb.Self.EncPrv)
	if err != nil {
		wb.setStatus("error invalid crypto (invite)")
		return
	}

	party := new(PartyLine)
	err = json.Unmarshal(jsonData, party)
	if err != nil {
		log.Println(err)
		party.WhiteBox.setStatus("error invalid json (invite)")
		return
	}

	match, err := regexp.MatchString("^[a-zA-Z0-9]{32}$", party.Id)
	if err != nil || !match {
		wb.setStatus("error invalid party id (invite)")
		return
	}

	party.WhiteBox = wb
	party.SeenChats = make(map[string]bool)
	party.Packs = make(map[string]LockingPack)
	party.PacksLock = new(sync.Mutex)

	wb.PendingInvites.Mutex.Lock()
	_, inPending := wb.PendingInvites.Map[party.Id]
	wb.PendingInvites.Mutex.Unlock()

	wb.Parties.Mutex.Lock()
	_, inParties := wb.Parties.Map[party.Id]
	wb.Parties.Mutex.Unlock()

	if inPending || inParties {
		wb.setStatus("reinvite ignored for " + party.Id)
		return
	}

	wb.PendingInvites.Mutex.Lock()
	wb.PendingInvites.Map[party.Id] = party
	wb.PendingInvites.Mutex.Unlock()

	party.WhiteBox.setStatus(fmt.Sprintf("invite received for %s", party.Id))
}

func (wb *WhiteBox) AcceptInvite(partyId string) {
	wb.PendingInvites.Mutex.Lock()
	party := wb.PendingInvites.Map[partyId]
	wb.PendingInvites.Mutex.Unlock()

	wb.Parties.Mutex.Lock()
	_, joined := wb.Parties.Map[partyId]
	wb.Parties.Mutex.Unlock()

	if joined {
		party.WhiteBox.setStatus("error already joined party with id")
		log.Println("party id in both pending and parties")
		return
	}

	wb.PendingInvites.Mutex.Lock()
	delete(wb.PendingInvites.Map, partyId)
	wb.PendingInvites.Mutex.Unlock()

	party.SendAnnounce()

	wb.Parties.Mutex.Lock()
	wb.Parties.Map[party.Id] = party
	wb.Parties.Mutex.Unlock()

	party.WhiteBox.setStatus(fmt.Sprintf("accepted invite %s", party.Id))
}

func (party *PartyLine) AdvertisePacks() {
	party.PacksLock.Lock()
	defer party.PacksLock.Unlock()
	for packSha256, lockingPack := range party.Packs {
		lockingPack.Mutex.Lock()
		if lockingPack.Pack.State == COMPLETE {
			party.SendAdvertisement(packSha256, lockingPack.Pack)
		}
		lockingPack.Mutex.Unlock()
	}
}

func (party *PartyLine) ClearPacks() {
	party.PacksLock.Lock()
	defer party.PacksLock.Unlock()
	party.Packs = make(map[string]LockingPack)
}

func (party *PartyLine) StartPack(packHash string) {
	party.PacksLock.Lock()
	lockingPack := party.Packs[packHash]
	party.PacksLock.Unlock()

	lockingPack.Mutex.Lock()
	defer lockingPack.Mutex.Unlock()
	pack := lockingPack.Pack

	// party ids are alphanum
	partyDir := filepath.Join(party.WhiteBox.SharedDir, party.Id)
	partyDirAbs, err := filepath.Abs(partyDir)
	if err != nil {
		log.Println(err)
		party.WhiteBox.setStatus(
			"error could not get absolute path for party dir")
		return
	}

	if strings.Contains(pack.Name, "..") {
		// DATA RACE: with unknown:N (unknown)
		party.WhiteBox.setStatus("error pack name potential directory traversal")
		return
	}

	err = os.MkdirAll(partyDirAbs, 0700)
	if err != nil {
		log.Println(err)
		party.WhiteBox.setStatus("error could not create destination dir")
		return
	}

	pendingPack := pack.ToPendingPack()
	jsonPendingPack, err := json.Marshal(pendingPack)
	if err != nil {
		log.Println(err)
		party.WhiteBox.setStatus("error marshalling pending pack to json")
		return
	}

	pendingFileName := filepath.Join(partyDirAbs, pack.Name+".pending")
	err = ioutil.WriteFile(pendingFileName, []byte(jsonPendingPack), 0644)
	if err != nil {
		log.Println(err)
		party.WhiteBox.setStatus("error writing pending pack to file")
		return
	}

	pack.SetPaths(partyDirAbs)

	pack.FileLock.Lock()
	// write zeros to files
	for _, file := range pack.Files {
		party.WhiteBox.writeZeroFile(file.Path, file.Size)
		file.Coverage = emptyCoverage(file.Size)
	}
	pack.FileLock.Unlock()

	pack.State = ACTIVE
}

func (party *PartyLine) ProcessRequest(partyEnv *PartyEnvelope) {
	signedPartyRequest := partyEnv.Data
	jsonPartyRequest := signedPartyRequest[sign.SignatureSize:]

	partyRequest := new(PartyRequest)
	err := json.Unmarshal(jsonPartyRequest, partyRequest)
	if err != nil {
		log.Println(err)
		party.WhiteBox.setStatus("error invalid json (party:request)")
		return
	}

	min, err := party.WhiteBox.IdToMin(partyRequest.PeerId)
	if err != nil {
		party.WhiteBox.setStatus("error bad id (party:request)")
		return
	}

	verified := sign.Verify(signedPartyRequest, min.SignPub)
	if !verified {
		party.WhiteBox.setStatus(
			"error questionable message integrity (party:request)")
		return
	}

	if partyRequest.PartyId != party.Id {
		party.WhiteBox.setStatus("error invalid party id (party:request)")
		return
	}

	// check seen hash + time
	uniqueId := min.Id() + party.Id
	uniqueId += partyRequest.PackHash + partyRequest.FileHash
	idBytes := []byte(uniqueId)
	id := sha256Bytes(idBytes)
	since, ok := party.WhiteBox.FreshRequests[id]
	if ok && (partyRequest.Time.Before(since.Reported) ||
		time.Now().UTC().Sub(since.Received) < 5*time.Second) {
		// request is stale ||
		// we've seen this peer in the last 5 seconds
		return
	}

	// forward
	party.sendToNeighbors("request", signedPartyRequest)

	// enqueue
	now := time.Now().UTC()
	since = new(Since)
	since.Reported = partyRequest.Time
	since.Received = now
	party.WhiteBox.FreshRequests[id] = since

	party.PacksLock.Lock()
	lockingPack, ok := party.Packs[partyRequest.PackHash]
	party.PacksLock.Unlock()

	lockingPack.Mutex.Lock()
	defer lockingPack.Mutex.Unlock()
	pack := lockingPack.Pack
	if !ok || pack.State == AVAILABLE {
		// we don't have the pack
		return
	}

	if pack.GetFileInfo(partyRequest.FileHash) == nil {
		// we don't have the file
		return
	}

	// reuse the time field as expiry, set for 6 seconds
	partyRequest.Time = now.Add(6 * time.Second)

	party.WhiteBox.RequestChan <- partyRequest
}

func (party *PartyLine) SendRequest(packHash string, file *PackFileInfo) {
	partyRequest := PartyRequest{
		PeerId:   party.WhiteBox.PeerSelf.Id(),
		PackHash: packHash,
		FileHash: file.Hash,
		Coverage: file.Coverage,
		Time:     time.Now().UTC(),
		PartyId:  party.Id}

	log.Printf("(dbg) sent coverage %v\n", partyRequest.Coverage)

	jsonPartyRequest, err := json.Marshal(partyRequest)
	if err != nil {
		log.Println(err)
		return
	}

	signedPartyRequest := sign.Sign(
		[]byte(jsonPartyRequest), party.WhiteBox.Self.SignPrv)

	party.sendToNeighbors("request", signedPartyRequest)
}

func (party *PartyLine) SendRequests(packHash string, pack *Pack) {
	complete := true
	pack.FileLock.Lock()
	for _, file := range pack.Files {
		if !isFullCoverage(file.Size, file.Coverage) {
			party.SendRequest(packHash, file)
			log.Println("(dbg) sent file request")
			complete = false
		}
	}
	pack.FileLock.Unlock()

	if complete {
		pack.State = COMPLETE
		log.Println("(dbg) pack complete")
	}
}

func (party *PartyLine) SendFulfillment(request *PartyRequest, block *Block) {
	env := Envelope{
		Type: "party",
		From: party.WhiteBox.PeerSelf.Id(),
		To:   request.PeerId}

	partyEnv := PartyEnvelope{
		Type:    "fulfillment",
		From:    party.WhiteBox.PeerSelf.Id(),
		PartyId: party.Id}

	partyFulfillment := PartyFulfillment{
		PeerId:   party.WhiteBox.PeerSelf.Id(),
		PackHash: request.PackHash,
		FileHash: request.FileHash,
		PartyId:  party.Id,
		Block:    *block}

	jsonPartyFulfillment, err := json.Marshal(partyFulfillment)
	if err != nil {
		log.Println(err)
		return
	}

	signedPartyFulfillment :=
		sign.Sign([]byte(jsonPartyFulfillment), party.WhiteBox.Self.SignPrv)

	partyEnv.Data = signedPartyFulfillment

	jsonPartyEnv, err := json.Marshal(partyEnv)
	if err != nil {
		log.Println(err)
		return
	}

	min, err := party.WhiteBox.IdToMin(request.PeerId)
	if err != nil {
		party.WhiteBox.setStatus(err.Error())
		return
	}

	closed := box.EasySeal(
		[]byte(jsonPartyEnv), min.EncPub, party.WhiteBox.Self.EncPrv)
	env.Data = closed

	jsonEnv, err := json.Marshal(env)
	if err == nil {
		log.Printf("Size of fulfillment: %d", len(jsonEnv)+1)
	}

	log.Println(string(jsonEnv))

	party.WhiteBox.route(&env)
}

func (party *PartyLine) ProcessFulfillment(partyEnv *PartyEnvelope) {
	log.Println("(dbg) got fulfillment")
	signedPartyFulfillment := partyEnv.Data
	jsonPartyFulfillment := signedPartyFulfillment[sign.SignatureSize:]

	partyFulfillment := new(PartyFulfillment)
	err := json.Unmarshal(jsonPartyFulfillment, partyFulfillment)
	if err != nil {
		log.Println(err)
		party.WhiteBox.setStatus("error invalid json (party:fulfillment)")
		return
	}

	min, err := party.WhiteBox.IdToMin(partyFulfillment.PeerId)
	if err != nil {
		party.WhiteBox.setStatus("error bad id (party:fulfillment)")
		return
	}

	verified := sign.Verify(signedPartyFulfillment, min.SignPub)
	if !verified {
		party.WhiteBox.setStatus(
			"error questionable message integrity (party:fulfillment)")
		return
	}

	if partyFulfillment.PartyId != party.Id {
		// wrong party ??!?
		return
	}

	party.PacksLock.Lock()
	lockingPack, ok := party.Packs[partyFulfillment.PackHash]
	party.PacksLock.Unlock()

	lockingPack.Mutex.Lock()
	pack := lockingPack.Pack
	if !ok || pack.State != ACTIVE {
		// we aren't downloading the pack
		lockingPack.Mutex.Unlock()
		return
	}

	packFileInfo := pack.GetFileInfo(partyFulfillment.FileHash)
	lockingPack.Mutex.Unlock()
	if packFileInfo == nil {
		// we don't have the file
		return
	}

	block := partyFulfillment.Block

	dataHash := sha256Bytes(block.Data)
	if dataHash != block.DataHash {
		// invalid data hash
		return
	}

	blockHash := sha256Block(&block)

	// verify block hash
	if block.Index == 0 {
		if blockHash != packFileInfo.FirstBlockHash {
			return
		}
	} else {
		checkBlockHash := ""

		prevIndex := block.Index - 1
		prevBlockHash, ok := packFileInfo.BlockLookup[prevIndex]
		if ok {
			prevParentBlock, ok := packFileInfo.BlockMap[prevBlockHash]
			if ok {
				checkBlockHash = prevParentBlock.NextBlockHash
			}
		}

		treeIndex := treeParent(block.Index)
		treeBlockHash, ok := packFileInfo.BlockLookup[treeIndex]
		if ok {
			treeParentBlock, ok := packFileInfo.BlockMap[treeBlockHash]
			if ok {
				childBlockHash := ""
				if block.Index%2 == 1 {
					childBlockHash = treeParentBlock.LeftBlockHash
				} else {
					childBlockHash = treeParentBlock.RightBlockHash
				}

				if checkBlockHash != "" && checkBlockHash != childBlockHash {
					// disagreement between prev and tree parents
					return
				}

				checkBlockHash = childBlockHash
			}
		}

		if checkBlockHash == "" {
			// cannot verify
			return
		}

		if checkBlockHash != blockHash {
			// invalid block hash
			return
		}
	} // verified

	// create verified block
	verifiedBlock := new(VerifiedBlock)
	verifiedBlock.Block = &block
	verifiedBlock.PackFileInfo = packFileInfo
	if packFileInfo.BlockLookup == nil {
		log.Println("(dbg) block lookup nil")
	}
	verifiedBlock.Hash = blockHash

	party.WhiteBox.VerifiedBlockChan <- verifiedBlock
}

func (party *PartyLine) chooseBlock(request *PartyRequest) *Block {
	// get blocks that request can verify
	nextBlocks := make([]uint64, len(request.Coverage))
	for majorIdx, ea := range request.Coverage {
		var minorIdx uint64
		for minorIdx = 0; minorIdx < 64; minorIdx++ {
			if (ea>>minorIdx)&1 == 1 {
				baseIdx := uint64(majorIdx) * 64

				next := baseIdx + minorIdx + 1
				nextMajorIdx := next / 64
				nextMinorIdx := next % 64
				if (request.Coverage[nextMajorIdx]>>nextMinorIdx)&1 == 0 {
					nextBlocks[nextMajorIdx] |= (1 << nextMinorIdx)
				}

				left := leftChild(baseIdx + minorIdx)
				leftMajorIdx := left / 64
				leftMinorIdx := left % 64
				if (request.Coverage[leftMajorIdx]>>leftMinorIdx)&1 == 0 {
					nextBlocks[leftMajorIdx] |= (1 << leftMinorIdx)
				}

				right := rightChild(baseIdx + minorIdx)
				rightMajorIdx := right / 64
				rightMinorIdx := right % 64
				if (request.Coverage[rightMajorIdx]>>rightMinorIdx)&1 == 0 {
					nextBlocks[rightMajorIdx] |= (1 << rightMinorIdx)
				}
			}
		}
	}

	blockIndices := make([]uint64, 0)
	party.PacksLock.Lock()
	lockingPack := party.Packs[request.PackHash]
	party.PacksLock.Unlock()

	lockingPack.Mutex.Lock()
	pack := lockingPack.Pack

	packFileInfo := pack.GetFileInfo(request.FileHash)
	lockingPack.Mutex.Unlock()

	selfCoverage := packFileInfo.Coverage

	// xor with self coverage to find candidates to send
	for majorIdx, _ := range nextBlocks {
		nextBlocks[majorIdx] &= selfCoverage[majorIdx]
		var minorIdx uint64
		for minorIdx = 0; minorIdx < 64; minorIdx++ {
			if (nextBlocks[majorIdx]>>minorIdx)&1 == 1 {
				blockIndices = append(blockIndices, uint64(majorIdx)*64+minorIdx)
			}
		}
	}

	if isEmptyCoverage(selfCoverage) {
		return nil
	}

	var blockIdx uint64 = 0
	// choose random candidate
	if len(blockIndices) > 0 {
		mrandIdx := mrand.Intn(len(blockIndices))
		blockIdx = blockIndices[mrandIdx]
	}

	// get block
	blockHash, ok := packFileInfo.BlockLookup[blockIdx]
	if !ok {
		log.Println("error block hash not found")
		return nil
	}

	blockInfo, ok := packFileInfo.BlockMap[blockHash]
	if !ok {
		log.Println("error block info not found")
		return nil
	}

	// read data from disk
	file, err := os.Open(packFileInfo.Path)
	if err != nil {
		log.Println(err)
		return nil
	}

	off, err := file.Seek(int64(blockIdx)*BUFFER_SIZE, os.SEEK_SET)
	if err != nil || off != int64(blockIdx)*BUFFER_SIZE {
		if err != nil {
			log.Println(err)
		}
		return nil
	}

	buffer := make([]byte, BUFFER_SIZE)
	bytesRead, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		log.Println(err)
		return nil
	}

	block := blockInfo.ToBlock(buffer[:bytesRead])
	if block.DataHash != sha256Bytes(block.Data) {
		log.Println("error data hash does not equal hash of data")
		return nil
	}

	return block
}

func (wb *WhiteBox) FileRequester() {
	for {
		wb.Parties.Mutex.Lock()
		for _, party := range wb.Parties.Map {
			party.PacksLock.Lock()
			for packHash, lockingPack := range party.Packs {
				lockingPack.Mutex.Lock()
				pack := lockingPack.Pack
				if pack.State == ACTIVE {
					party.SendRequests(packHash, pack)
					log.Println("(dbg) sent requests")
				}
				lockingPack.Mutex.Unlock()
			}
			party.PacksLock.Unlock()
		}
		wb.Parties.Mutex.Unlock()
		time.Sleep(5 * time.Second)
	}
}

func (wb *WhiteBox) RequestSender() {
	for {
		request := <-wb.RequestChan
		if request.PeerId == wb.PeerSelf.Id() {
			// it me
			log.Println("(dbg) request self")
			continue
		}

		if time.Now().UTC().After(request.Time) {
			// skip if expiry
			log.Println("(dbg) request expired")
			continue
		}

		// TODO: add lock to party, block delete
		// TODO: check party not nil (have)
		wb.Parties.Mutex.Lock()
		party, ok := wb.Parties.Map[request.PartyId]
		wb.Parties.Mutex.Unlock()
		if !ok || party == nil {
			log.Println("(dbg) party over")
			return
		}

		log.Printf("(dbg) got coverage %v\n", request.Coverage)

		// choose block
		block := party.chooseBlock(request)
		if block == nil {
			continue
		}

		// send block
		party.SendFulfillment(request, block)
		log.Println("(dbg) sent fulfillment")

		// mark coverage
		majorIdx := block.Index / 64
		minorIdx := block.Index % 64
		request.Coverage[majorIdx] |= (1 << minorIdx)

		// requeue
		wb.RequestChan <- request

		// we sleep a little to let other stuff get the party lock
		// 10kb * 500/s = 5MB/s
		time.Sleep(2 * time.Millisecond)
	}
}

func haveBlock(verifiedBlock *VerifiedBlock) bool {
	block := verifiedBlock.Block
	majorIdx := block.Index / 64
	minorIdx := block.Index % 64
	packFileInfo := verifiedBlock.PackFileInfo
	return ((packFileInfo.Coverage[majorIdx] >> minorIdx) & 1) == 1
}

func setBlockWritten(verifiedBlock *VerifiedBlock) {
	block := verifiedBlock.Block
	packFileInfo := verifiedBlock.PackFileInfo
	majorIdx := block.Index / 64
	minorIdx := block.Index % 64
	packFileInfo.Coverage[majorIdx] |= (1 << minorIdx)

	log.Printf("(dbg) set coverage %v\n", packFileInfo.Coverage)

	blockInfo := block.ToBlockInfo()
	packFileInfo.BlockMap[verifiedBlock.Hash] = *blockInfo
	if packFileInfo.BlockLookup == nil {
		log.Println("(dbg) block lookup nil")
	}
	packFileInfo.BlockLookup[block.Index] = verifiedBlock.Hash
}

func (wb *WhiteBox) VerifiedBlockWriter() {
	for {
		verifiedBlock := <-wb.VerifiedBlockChan

		if haveBlock(verifiedBlock) {
			log.Println("(dbg) have block skipping")
			continue
		}

		mode := os.O_RDWR | os.O_CREATE
		f, err := os.OpenFile(verifiedBlock.PackFileInfo.Path, mode, 0755)
		if err != nil {
			log.Println(err)
			wb.setStatus("error opening file for block")
			continue
		}

		// seek block
		offset := BUFFER_SIZE * verifiedBlock.Block.Index
		pos, err := f.Seek(int64(offset), os.SEEK_SET)
		if err != nil || pos != int64(offset) {
			if err != nil {
				log.Println(err)
			}
			wb.setStatus("error seeking in file for block")
			continue
		}

		// write block
		count, err := f.Write(verifiedBlock.Block.Data)
		if err != nil || count != len(verifiedBlock.Block.Data) {
			if err != nil {
				log.Println(err)
			}
			wb.setStatus("error writing to file for block")
			continue
		}

		err = f.Close()
		if err != nil {
			log.Println(err)
			wb.setStatus("error closing file for block")
			continue
		}

		setBlockWritten(verifiedBlock)
		log.Printf("(dbg) wrote block %d\n", verifiedBlock.Block.Index)
	}
}

func (wb *WhiteBox) writeZeroFile(name string, size int64) {
	wb.setStatus("writing empty file for " + name)

	fileDir := filepath.Dir(name)
	err := os.MkdirAll(fileDir, 0700)
	if err != nil {
		log.Println(err)
		wb.setStatus("error when prepping dirs")
		return
	}

	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Println(err)
		wb.setStatus("error when prepping file")
		return
	}

	defer f.Close()

	single := []byte{0}
	mb100 := 102400000
	buf100 := bytes.Repeat(single, mb100)

	remaining := size
	for remaining > int64(mb100) {
		n, err := f.Write(buf100)
		if err != nil || n != mb100 {
			if err != nil {
				log.Println(err)
			}
			wb.setStatus("error writing empty file")
			return
		}
		remaining -= int64(mb100)
	}

	if remaining > 0 {
		bufRemaining := bytes.Repeat(single, int(remaining))
		n, err := f.Write(bufRemaining)
		if err != nil || int64(n) != remaining {
			if err != nil {
				log.Println(err)
			}
			wb.setStatus("error writing empty file")
			return
		}
	}

	f.Sync()
	wb.setStatus("empty file written for " + name)
}

func (wb *WhiteBox) advertiseAll() {
	wb.Parties.Mutex.Lock()
	for _, party := range wb.Parties.Map {
		party.AdvertisePacks()
	}
	wb.Parties.Mutex.Unlock()
}

func (wb *WhiteBox) DisconnectParties() {
	wb.Parties.Mutex.Lock()
	for _, party := range wb.Parties.Map {
		party.SendDisconnect()
	}
	wb.Parties.Mutex.Unlock()
}

func minimum(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (wb *WhiteBox) PartyStart(name string) string {
	idBytes := make([]byte, 16)
	rand.Read(idBytes)

	party := new(PartyLine)

	// this shouldn't be guessable, so we will enforce 12 bytes random
	// that's like 8 times as many bits
	name = name[:minimum(len(name), 8)]
	idHex := hex.EncodeToString(idBytes)

	party.Id = name + idHex[:len(idHex)-len(name)]
	match, err := regexp.MatchString("^[a-zA-Z0-9]{32}$", party.Id)
	if err != nil || !match {
		wb.setStatus("error invalid party id (start)")
		return ""
	}

	party.MinList.Map = make(map[string]int)
	party.MinList.Mutex = new(sync.Mutex)
	party.SeenChats = make(map[string]bool)
	party.Packs = make(map[string]LockingPack)
	party.PacksLock = new(sync.Mutex)
	party.WhiteBox = wb

	party.MinList.Set(party.WhiteBox.PeerSelf.Id(), 0)

	wb.Parties.Mutex.Lock()
	wb.Parties.Map[party.Id] = party
	wb.Parties.Mutex.Unlock()

	return party.Id
}

func (wb *WhiteBox) Advertise() {
	for {
		wb.advertiseAll()
		time.Sleep(60 * time.Second)
	}
}
