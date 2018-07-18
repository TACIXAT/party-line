package main

import (
	"bytes"
	"container/list"
	"encoding/hex"
	"github.com/kevinburke/nacl/sign"
	"log"
	"math/big"
	"time"
)

var peerTable [256]*list.List
var idealPeerIds [256]*big.Int

type PeerEntry struct {
	ID       sign.PublicKey
	Distance *big.Int
	Peer     *Peer
	Seen     time.Time
}

func initTable(idBytes []byte) {
	idInt := new(big.Int)
	idInt.SetBytes(idBytes)

	for i := 0; i < 256; i++ {
		peerDist := new(big.Int)
		peerDist.Xor(idealPeerIds[i], idInt)

		peerEntry := new(PeerEntry)
		peerEntry.ID = idBytes
		peerEntry.Distance = peerDist
		peerEntry.Peer = nil
		peerEntry.Seen = time.Now()

		peerTable[i] = list.New()
		peerTable[i].PushFront(peerEntry)
	}
}

func calculateIdealTable(idBytes []byte) [256]*big.Int {
	var idealIds [256]*big.Int

	idInt := new(big.Int)
	idInt.SetBytes(idBytes)

	mask := new(big.Int)
	mask.SetUint64(1)

	for i := 0; i < len(idBytes)*8; i++ {
		idealPeerId := new(big.Int)
		idealPeerId.Xor(idInt, mask)
		idealIds[i] = idealPeerId
		mask.Lsh(mask, 1)
	}

	return idealIds
}

func calculateIdealTableSelf(idBytes []byte) {
	idealPeerIds = calculateIdealTable(idBytes)
}

func removePeer(peerId string) {
	bytesId, err := hex.DecodeString(peerId)
	if err != nil {
		log.Println(err)
		return
	}

	for i := 0; i < 256; i++ {
		removeList := make([]*list.Element, 0)
		for curr := peerTable[i].Front(); curr != nil; curr = curr.Next() {
			entry := curr.Value.(*PeerEntry)
			if bytes.Compare(entry.ID, bytesId) == 0 {
				removeList = append(removeList, curr)
			}
		}

		for _, element := range removeList {
			peerTable[i].Remove(element)
		}
	}
}

func removeStalePeers() {
	for i := 0; i < 256; i++ {
		removeList := make([]*list.Element, 0)
		for curr := peerTable[i].Front(); curr != nil; curr = curr.Next() {
			entry := curr.Value.(*PeerEntry)
			if entry.Peer != nil && time.Now().Sub(entry.Seen) > 60*time.Second {
				removeList = append(removeList, curr)
			}
		}

		for _, element := range removeList {
			setStatus("removed stale peer")
			peerTable[i].Remove(element)
		}
	}
}

func addPeer(peer *Peer) {
	seenPeers[peer.ID] = true

	idBytes := peer.SignPub
	insertId := new(big.Int)
	insertId.SetBytes(idBytes)

	for i := 0; i < 256; i++ {
		insertDist := new(big.Int)
		insertDist.Xor(idealPeerIds[i], insertId)

		last := peerTable[i].Back()
		lastPeerEntry := last.Value.(*PeerEntry)
		if insertDist.Cmp(lastPeerEntry.Distance) < 0 {
			insertEntry := new(PeerEntry)
			insertEntry.ID = idBytes
			insertEntry.Distance = insertDist
			insertEntry.Peer = peer
			insertEntry.Seen = time.Now()

			curr := last
			currPeerEntry := lastPeerEntry
			for curr != nil && insertDist.Cmp(currPeerEntry.Distance) < 0 {
				curr = curr.Prev()
				if curr != nil {
					currPeerEntry = curr.Value.(*PeerEntry)
				}
			}

			if curr == nil {
				peerTable[i].PushFront(insertEntry)
			} else {
				peerTable[i].InsertAfter(insertEntry, curr)
			}

			if peerTable[i].Len() > 20 {
				peerTable[i].Remove(last)
			}
		}
	}
}

func wouldAddPeer(peer *Peer) bool {
	idBytes := peer.SignPub
	insertId := new(big.Int)
	insertId.SetBytes(idBytes)

	for i := 0; i < 256; i++ {
		insertDist := new(big.Int)
		insertDist.Xor(idealPeerIds[i], insertId)

		last := peerTable[i].Back()
		lastPeerEntry := last.Value.(*PeerEntry)
		if insertDist.Cmp(lastPeerEntry.Distance) < 0 {
			return true
		}
	}

	return false
}

func findClosest(idBytes []byte) *PeerEntry {
	idInt := new(big.Int)
	idInt.SetBytes(idBytes)

	// find lowest of ideal table
	lowestIdealDist := new(big.Int)
	lowestIdealIdx := 0
	for i := 0; i < 256; i++ {
		dist := new(big.Int)
		dist.Xor(idealPeerIds[i], idInt)

		if i == 0 {
			lowestIdealDist = dist
			lowestIdealIdx = i
		}

		if dist.Cmp(lowestIdealDist) < 0 {
			lowestIdealDist = dist
			lowestIdealIdx = i
		}
	}

	// find lowest entry in bucket
	closestDist := new(big.Int)
	closestElement := peerTable[lowestIdealIdx].Front()
	for curr := closestElement; curr != nil; curr = curr.Next() {
		entry := curr.Value.(*PeerEntry)
		entryDist := new(big.Int)
		entryDist.SetBytes(entry.ID)
		entryDist.Xor(entryDist, idInt)
		if entryDist.Cmp(closestDist) < 0 {
			closestDist = entryDist
			closestElement = curr
		}
	}

	if closestElement == nil {
		return nil
	}

	return closestElement.Value.(*PeerEntry)
}

func refreshPeer(peerId string) {
	bytesId, err := hex.DecodeString(peerId)
	if err != nil {
		log.Println(err)
		return
	}

	for i := 0; i < 256; i++ {
		for curr := peerTable[i].Front(); curr != nil; curr = curr.Next() {
			entry := curr.Value.(*PeerEntry)
			if bytes.Compare(entry.ID, bytesId) == 0 {
				entry.Seen = time.Now()
			}
		}
	}
}
