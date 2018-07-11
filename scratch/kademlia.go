package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/kevinburke/nacl/sign"
	"math/big"
)

func calculateIdealTable(idBytes []byte) [256]*big.Int {
	id := new(big.Int)
	id.SetBytes(idBytes)

	mask := new(big.Int)
	mask.SetUint64(1)

	var idealPeerIds [256]*big.Int
	for i := 0; i < len(idBytes)*8; i++ {
		idealPeerId := new(big.Int)
		idealPeerId.Xor(id, mask)
		idealPeerIds[i] = idealPeerId
		mask.Lsh(mask, 1)
	}

	return idealPeerIds
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

func findClosest(idealPeerIds [256]*big.Int, idBytes []byte) {
	id := new(big.Int)
	id.SetBytes(idBytes)

	lowestDist := new(big.Int)
	lowestIdx := 0
	for i := 0; i < 256; i++ {
		dist := new(big.Int)
		dist.Sub(idealPeerIds[i], id)
		// fmt.Println("ideal", idealPeerIds[i])
		// fmt.Println("id", id)
		dist.Abs(dist)
		// fmt.Println("dist", dist)
		// fmt.Println()

		if i == 0 {
			lowestDist = dist
			lowestIdx = i
		}
		// fmt.Println(dist.Cmp(lowestDist))
		if dist.Cmp(lowestDist) < 0 {
			lowestDist = dist
			lowestIdx = i
		}
	}

	if lowestIdx < 241 {
		fmt.Println("id   ", id)
		fmt.Println("ideal", idealPeerIds[lowestIdx])
		fmt.Println("dist ", lowestDist)
		fmt.Println(lowestIdx)
	}
}

func addPeer(peerTable *[256][]byte, idealPeerIds [256]*big.Int, peer []byte) {
	insertId := new(big.Int)
	insertId.SetBytes(peer)

	for i := 0; i < 256; i++ {
		insertDist := new(big.Int)
		insertDist.Sub(idealPeerIds[i], insertId)
		insertDist.Abs(insertDist)

		if len(peerTable[i]) == 0 {
			// fmt.Println("new peer at", i)
			peerTable[i] = peer
			continue
		}

		currId := new(big.Int)
		currId.SetBytes(peerTable[i])
		currDist := new(big.Int)
		currDist.Sub(idealPeerIds[i], currId)
		currDist.Abs(currDist)

		if insertDist.Cmp(currDist) < 0 {
			peerTable[i] = peer
			// fmt.Println("added peer at", i)
		}
	}
}

func main() {
	r := rand.Reader
	id, _, err := sign.Keypair(r)
	if err != nil {
		panic(err)
	}

	fmt.Println(hex.EncodeToString(id))

	idealPeerIds := calculateIdealTable(id)
	fakePeers := getFakePeers()

	fmt.Println(len(idealPeerIds))
	fmt.Println(len(fakePeers))
	fmt.Println(hex.EncodeToString(fakePeers[0]))
	fmt.Println(hex.EncodeToString(fakePeers[9999]))

	var peerTable [256][]byte
	fmt.Println(len(peerTable[0]))

	for i := 0; i < 10000; i++ {
		addPeer(&peerTable, idealPeerIds, fakePeers[i])
		// fmt.Println()
	}

	for i := 0; i < 256; i++ {
		fmt.Println(hex.EncodeToString(peerTable[i]))
	}
}
