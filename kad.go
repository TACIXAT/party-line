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
}

func initTable(id []byte) {
	idInt := new(big.Int)
	idInt.SetBytes(id)

	for i := 0; i < 256; i++ {
		peerTable[i] = list.New()
	}
}

func calculateIdealTable(idBytes []byte) {
	id := new(big.Int)
	id.SetBytes(idBytes)

	mask := new(big.Int)
	mask.SetUint64(1)

	for i := 0; i < len(idBytes)*8; i++ {
		idealPeerId := new(big.Int)
		idealPeerId.Xor(id, mask)
		idealPeerIds[i] = idealPeerId
		mask.Lsh(mask, 1)
	}
}

func addPeer(id []byte) {
	insertId := new(big.Int)
	insertId.SetBytes(id)
	for i := 0; i < 256; i++ {
		insertDist := new(big.Int)
		insertDist.Sub(idealPeerIds[i], insertId)
		insertDist.Abs(insertDist)

		last := peerTable[i].Back()

		insertEntry := new(PeerEntry)
		insertEntry.ID = id
		insertEntry.Distance = insertDist

		if last == nil {
			peerTable[i].PushBack(insertEntry)
		} else {
			lastPeerEntry := last.Value.(*PeerEntry)

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
	id := new(big.Int)
	id.SetBytes(idBytes)

	// find lowest of ideal table
	lowestIdealDist := new(big.Int)
	lowestIdealIdx := 0
	for i := 0; i < 256; i++ {
		dist := new(big.Int)
		dist.Sub(idealPeerIds[i], id)
		dist.Abs(dist)

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
		entryDist.Xor(entryDist, id)
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
