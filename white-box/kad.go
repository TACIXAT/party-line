package whitebox

import (
	"bytes"
	"container/list"
	"encoding/hex"
	// "fmt"
	"github.com/kevinburke/nacl/sign"
	"log"
	"math/big"
	"time"
)

type PeerEntry struct {
	Id       sign.PublicKey
	Distance *big.Int
	Peer     *Peer
	Seen     time.Time
}

type PeerCache struct {
	Added        bool
	Announced    bool
	Disconnected bool
}

func (wb *WhiteBox) InitTable(idBytes []byte) {
	idInt := new(big.Int)
	idInt.SetBytes(idBytes)

	for i := 0; i < 256; i++ {
		peerDist := new(big.Int)
		peerDist.Xor(wb.IdealPeerIds[i], idInt)
		wb.PeerTable[i] = list.New()
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

func (wb *WhiteBox) CalculateIdealTableSelf(idBytes []byte) {
	wb.IdealPeerIds = calculateIdealTable(idBytes)
}

func (wb *WhiteBox) removePeer(peerId string) {
	// TODO: use closest index instead of exhaustive search
	bytesId, err := hex.DecodeString(peerId)
	if err != nil {
		log.Println(err)
		return
	}

	intId := new(big.Int)
	intId.SetBytes(bytesId)
	idx := wb.closestIndex(intId)

	removeList := make([]*list.Element, 0)
	for curr := wb.PeerTable[idx].Front(); curr != nil; curr = curr.Next() {
		entry := curr.Value.(*PeerEntry)
		if bytes.Compare(entry.Id, bytesId) == 0 {
			removeList = append(removeList, curr)
		}
	}

	for _, element := range removeList {
		wb.PeerTable[idx].Remove(element)
	}

	if !wb.EmptyList && !wb.havePeers() {
		wb.chatStatus("all friends gone, bootstrap some new ones")
		wb.EmptyList = true
	}
}

func (wb *WhiteBox) removeStalePeers() {
	removed := false
	for i := 0; i < 256; i++ {
		removeList := make([]*list.Element, 0)
		for curr := wb.PeerTable[i].Front(); curr != nil; curr = curr.Next() {
			entry := curr.Value.(*PeerEntry)
			if entry.Peer != nil && time.Now().Sub(entry.Seen) > 60*time.Second {
				removeList = append(removeList, curr)
			}
		}

		for _, element := range removeList {
			wb.setStatus("removed stale peer")
			wb.PeerTable[i].Remove(element)
			removed = true
		}
	}

	if removed && !wb.havePeers() && !wb.EmptyList {
		wb.chatStatus("all friends gone, bootstrap some new ones")
		wb.EmptyList = true
	}
}

func (wb *WhiteBox) havePeers() bool {
	for i := 0; i < 256; i++ {
		if wb.PeerTable[i].Len() > 1 {
			return true
		}

		element := wb.PeerTable[i].Front()
		if element != nil {
			entry := element.Value.(*PeerEntry)
			if entry.Peer != nil {
				return true
			}
		}
	}

	return false
}

func (wb *WhiteBox) cacheMin(min MinPeer) {
	cache := wb.PeerCache[min.Id()]
	wb.PeerCache[min.Id()] = cache
}

func (wb *WhiteBox) addPeer(peer *Peer) {
	cache, seen := wb.PeerCache[peer.Id()]
	if seen && cache.Added {
		return
	}

	cache.Added = true
	wb.PeerCache[peer.Id()] = cache

	idBytes := peer.SignPub
	insertId := new(big.Int)
	insertId.SetBytes(idBytes)

	idx := wb.closestIndex(insertId)
	peerList := wb.PeerTable[idx]

	insertDist := new(big.Int)
	insertDist.Xor(wb.IdealPeerIds[idx], insertId)

	insertEntry := new(PeerEntry)
	insertEntry.Id = idBytes
	insertEntry.Distance = insertDist
	insertEntry.Peer = peer
	insertEntry.Seen = time.Now()

	curr := peerList.Back()
	for curr != nil && insertDist.Cmp(curr.Value.(*PeerEntry).Distance) < 0 {
		curr = curr.Prev()
	}

	if curr == nil {
		wb.PeerTable[idx].PushFront(insertEntry)
	} else {
		wb.PeerTable[idx].InsertAfter(insertEntry, curr)
	}

	if wb.EmptyList {
		wb.chatStatus("peer added, happy chatting!")
		wb.EmptyList = false
	}
}

func (wb *WhiteBox) wouldAddPeer(peer *Peer) bool {
	idBytes := peer.SignPub
	insertId := new(big.Int)
	insertId.SetBytes(idBytes)

	idx := wb.closestIndex(insertId)

	insertDist := new(big.Int)
	insertDist.Xor(wb.IdealPeerIds[idx], insertId)

	if wb.PeerTable[idx].Len() < 20 {
		return true
	}

	last := wb.PeerTable[idx].Back()
	lastPeerEntry := last.Value.(*PeerEntry)
	if insertDist.Cmp(lastPeerEntry.Distance) < 0 {
		return true
	}

	return false
}

func (wb *WhiteBox) closestIndex(idInt *big.Int) int {
	// find lowest of ideal table
	lowestIdealDist := new(big.Int)
	lowestIdealIdx := 0
	for i := 0; i < 256; i++ {
		dist := new(big.Int)
		dist.Xor(wb.IdealPeerIds[i], idInt)

		if i == 0 {
			lowestIdealDist = dist
			lowestIdealIdx = i
		}

		if dist.Cmp(lowestIdealDist) < 0 {
			lowestIdealDist = dist
			lowestIdealIdx = i
		}
	}

	return lowestIdealIdx
}

func (wb *WhiteBox) findClosestN(idBytes []byte, n int) []*PeerEntry {
	idInt := new(big.Int)
	idInt.SetBytes(idBytes)

	closestIdx := wb.closestIndex(idInt)

	closest := make([]*PeerEntry, 0)
	dists := make([]*big.Int, 0)

	// find lowest entry in bucket
	peerList := wb.PeerTable[closestIdx]
	for curr := peerList.Front(); curr != nil; curr = curr.Next() {
		entry := curr.Value.(*PeerEntry)
		entryDist := new(big.Int)
		entryDist.SetBytes(entry.Id)
		entryDist.Xor(entryDist, idInt)

		i := 0
		for i = 0; i < len(closest); i++ {
			if entryDist.Cmp(dists[i]) < 0 {
				break
			}
		}

		closest = append(closest[:i], append([]*PeerEntry{entry}, closest[i:]...)...)
		dists = append(dists[:i], append([]*big.Int{entryDist}, dists[i:]...)...)
		if len(closest) > n {
			closest = closest[:n]
		}
	}

	return closest
}

func (wb *WhiteBox) findClosest(idBytes []byte) *PeerEntry {
	closest := wb.findClosestN(idBytes, 1)

	if len(closest) == 0 {
		return nil
	}

	return closest[0]
}

func (wb *WhiteBox) refreshPeer(peerId string) {
	bytesId, err := hex.DecodeString(peerId)
	if err != nil {
		log.Println(err)
		return
	}

	for i := 0; i < 256; i++ {
		for curr := wb.PeerTable[i].Front(); curr != nil; curr = curr.Next() {
			entry := curr.Value.(*PeerEntry)
			if bytes.Compare(entry.Id, bytesId) == 0 {
				entry.Seen = time.Now()
			}
		}
	}
}
