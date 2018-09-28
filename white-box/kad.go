package whitebox

import (
	"bytes"
	"container/list"
	"encoding/hex"
	// "fmt"
	"github.com/kevinburke/nacl/sign"
	"log"
	"math/big"
	"sync"
	"time"
)

type PeerEntry struct {
	Id       sign.PublicKey
	Distance *big.Int
	Peer     *Peer
	Seen     time.Time
}

type LockingPeerCacheMap struct {
	Map   map[string]PeerCache
	Mutex *sync.Mutex
}

func (lpcm LockingPeerCacheMap) Len() int {
	lpcm.Mutex.Lock()
	defer lpcm.Mutex.Unlock()
	return len(lpcm.Map)
}

func (lpcm LockingPeerCacheMap) Get(key string) (PeerCache, bool) {
	lpcm.Mutex.Lock()
	defer lpcm.Mutex.Unlock()
	pc, ok := lpcm.Map[key]
	return pc, ok
}

func (lpcm LockingPeerCacheMap) Set(key string, value PeerCache) {
	lpcm.Mutex.Lock()
	defer lpcm.Mutex.Unlock()
	lpcm.Map[key] = value
}

type PeerCache struct {
	Added        bool
	Announced    bool
	Disconnected bool
}

func (wb *WhiteBox) InitTable(idBytes []byte) {
	idInt := new(big.Int)
	idInt.SetBytes(idBytes)

	wb.PeerTable.Mutex.Lock()
	for i := 0; i < 256; i++ {
		peerDist := new(big.Int)
		peerDist.Xor(wb.IdealPeerIds[i], idInt)
		wb.PeerTable.Table[i] = list.New()
	}
	wb.PeerTable.Mutex.Unlock()
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
	wb.PeerTable.Mutex.Lock()
	for curr := wb.PeerTable.Table[idx].Front(); curr != nil; curr = curr.Next() {
		entry := curr.Value.(*PeerEntry)
		if bytes.Compare(entry.Id, bytesId) == 0 {
			removeList = append(removeList, curr)
		}
	}

	for _, element := range removeList {
		wb.PeerTable.Table[idx].Remove(element)
	}
	wb.PeerTable.Mutex.Unlock()

	if !wb.EmptyList && !wb.havePeers() {
		wb.chatStatus("all friends gone, bootstrap some new ones")
		wb.EmptyList = true
	}
}

func (wb *WhiteBox) removeStalePeers() {
	removed := false
	wb.PeerTable.Mutex.Lock()
	for i := 0; i < 256; i++ {
		removeList := make([]*list.Element, 0)
		for curr := wb.PeerTable.Table[i].Front(); curr != nil; curr = curr.Next() {
			entry := curr.Value.(*PeerEntry)
			if entry.Peer != nil && time.Now().Sub(entry.Seen) > 60*time.Second {
				removeList = append(removeList, curr)
			}
		}

		for _, element := range removeList {
			wb.setStatus("removed stale peer")
			wb.PeerTable.Table[i].Remove(element)
			removed = true
		}
	}
	wb.PeerTable.Mutex.Unlock()

	if removed && !wb.havePeers() && !wb.EmptyList {
		wb.chatStatus("all friends gone, bootstrap some new ones")
		wb.EmptyList = true
	}
}

func (wb *WhiteBox) havePeers() bool {
	wb.PeerTable.Mutex.Lock()
	defer wb.PeerTable.Mutex.Unlock()
	for i := 0; i < 256; i++ {
		if wb.PeerTable.Table[i].Len() > 1 {
			return true
		}

		element := wb.PeerTable.Table[i].Front()
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
	cache, _ := wb.PeerCache.Get(min.Id())
	wb.PeerCache.Set(min.Id(), cache)
}

func (wb *WhiteBox) addPeer(peer *Peer) {
	cache, seen := wb.PeerCache.Get(peer.Id())
	if seen && cache.Added {
		return
	}

	cache.Added = true
	wb.PeerCache.Set(peer.Id(), cache)

	idBytes := peer.SignPub
	insertId := new(big.Int)
	insertId.SetBytes(idBytes)

	idx := wb.closestIndex(insertId)

	wb.PeerTable.Mutex.Lock()
	peerList := wb.PeerTable.Table[idx]

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
		wb.PeerTable.Table[idx].PushFront(insertEntry)
	} else {
		wb.PeerTable.Table[idx].InsertAfter(insertEntry, curr)
	}
	wb.PeerTable.Mutex.Unlock()

	log.Println("peer added")

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

	wb.PeerTable.Mutex.Lock()
	defer wb.PeerTable.Mutex.Unlock()
	if wb.PeerTable.Table[idx].Len() < 20 {
		return true
	}

	last := wb.PeerTable.Table[idx].Back()
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
	wb.PeerTable.Mutex.Lock()
	peerList := wb.PeerTable.Table[closestIdx]
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

		// lol wut
		closest = append(
			closest[:i], append([]*PeerEntry{entry}, closest[i:]...)...)
		dists = append(
			dists[:i], append([]*big.Int{entryDist}, dists[i:]...)...)
		if len(closest) > n {
			closest = closest[:n]
		}
	}
	wb.PeerTable.Mutex.Unlock()

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

	wb.PeerTable.Mutex.Lock()
	for i := 0; i < 256; i++ {
		for curr := wb.PeerTable.Table[i].Front(); curr != nil; curr = curr.Next() {
			entry := curr.Value.(*PeerEntry)
			if bytes.Compare(entry.Id, bytesId) == 0 {
				entry.Seen = time.Now()
			}
		}
	}
	wb.PeerTable.Mutex.Unlock()
}
