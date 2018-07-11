package main

import (
	"bytes"
	"container/list"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/kevinburke/nacl/sign"
	"math/big"
)

var peerTable [256]*list.List
var idealPeerIds [256]*big.Int

type PeerEntry struct {
	ID       sign.PublicKey
	Distance *big.Int
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

func addPeer(id []byte, trace bool) {
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

func initTable(id []byte) {
	idInt := new(big.Int)
	idInt.SetBytes(id)

	for i := 0; i < 256; i++ {
		peerTable[i] = list.New()
	}
}

func getFakePeers() [][]byte {
	var peers [][]byte
	for i := 0; i < 10000; i++ {
		id := make([]byte, 32)
		rand.Read(id)
		peers = append(peers, id)
		// fmt.Println(hex.EncodeToString(id))
	}

	return peers
}

func findClosest(idBytes, selfID []byte) *PeerEntry {
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

	entry := closestElement.Value.(*PeerEntry)
	if bytes.Compare(entry.ID, selfID) == 0 {
		fmt.Println()
		fmt.Println(hex.EncodeToString(entry.ID))
		fmt.Println(hex.EncodeToString(idBytes))
		fmt.Println(lowestIdealIdx)
	}

	return closestElement.Value.(*PeerEntry)
}

func main() {
	r := rand.Reader
	id, _, err := sign.Keypair(r)
	if err != nil {
		panic(err)
	}

	fmt.Println(hex.EncodeToString(id))

	calculateIdealTable(id)
	fakePeers := getFakePeers()

	fmt.Println(len(idealPeerIds))
	fmt.Println(len(fakePeers))
	fmt.Println(hex.EncodeToString(fakePeers[0]))
	fmt.Println(hex.EncodeToString(fakePeers[9999]))

	// var peerTable [256][]byte
	// fmt.Println(len(peerTable[0]))

	initTable(id)

	for i := 0; i < 10000; i++ {
		addPeer(fakePeers[i], false)
	}

	for i := 0; i < 256; i++ {
		fmt.Println(i, hex.EncodeToString(peerTable[i].Front().Value.(*PeerEntry).ID), peerTable[i].Len())
	}

	self := 0
	exact := 0
	other := 0
	for i := 0; i < 10000; i++ {
		closest := findClosest(fakePeers[i], id)
		if bytes.Compare(closest.ID, id) == 0 {
			self++
			initTable(id)
			for j := 0; j < 10000; j++ {
				trace := false
				if j == i {
					trace = true
				}
				addPeer(fakePeers[j], trace)
			}
		} else if bytes.Compare(closest.ID, fakePeers[i]) == 0 {
			exact++
		} else {
			other++
		}
	}

	fmt.Println()
	fmt.Println("self ", self)
	fmt.Println("exact", exact)
	fmt.Println("other", other)
}
