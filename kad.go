package main

import (
	"container/list"
	// "encoding/hex"
	// "fmt"
	"github.com/kevinburke/nacl/sign"
	"math/big"
)

var peerTable [256]*list.List
var idealPeerIds [256]*big.Int

type PeerEntry struct {
	ID       sign.PublicKey
	Distance *big.Int
	Entry    *Peer
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
		peerEntry.Entry = nil

		peerTable[i] = list.New()
		peerTable[i].PushFront(peerEntry)
	}
}

func calculateIdealTable(idBytes []byte) {
	idInt := new(big.Int)
	idInt.SetBytes(idBytes)

	mask := new(big.Int)
	mask.SetUint64(1)

	for i := 0; i < len(idBytes)*8; i++ {
		idealPeerId := new(big.Int)
		idealPeerId.Xor(idInt, mask)
		idealPeerIds[i] = idealPeerId
		mask.Lsh(mask, 1)
	}
}

func addPeer(peer *Peer) {
	idBytes := peer.SignPub
	insertId := new(big.Int)
	insertId.SetBytes(idBytes)

	for i := 0; i < 256; i++ {
		insertDist := new(big.Int)
		insertDist.Sub(idealPeerIds[i], insertId)
		insertDist.Abs(insertDist)

		last := peerTable[i].Back()
		lastPeerEntry := last.Value.(*PeerEntry)
		if insertDist.Cmp(lastPeerEntry.Distance) < 0 {
			insertEntry := new(PeerEntry)
			insertEntry.ID = idBytes
			insertEntry.Distance = insertDist
			insertEntry.Entry = peer

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
