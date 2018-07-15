package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
)

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

// func wouldAddPeer(peer *Peer) bool {
// 	idBytes := peer.SignPub
// 	insertId := new(big.Int)
// 	insertId.SetBytes(idBytes)

// 	for i := 0; i < 256; i++ {
// 		insertDist := new(big.Int)
// 		insertDist.Xor(idealPeerIds[i], insertId)

// 		last := peerTable[i].Back()
// 		lastPeerEntry := last.Value.(*PeerEntry)
// 		if insertDist.Cmp(lastPeerEntry.Distance) < 0 {
// 			return true
// 		}
// 	}

// 	return false
// }

func main() {
	strIds := make([]string, 0)
	strIds = append(strIds, "4921a16193bde7ecf24f79497a795960468152ba259f6322e923332f186e3d8d")
	strIds = append(strIds, "e9e29710a2e84579d0a0acbe7df4c05a04bff13268027aee8dac653f54716cba")
	strIds = append(strIds, "c0fe0c06435f812e1dff643d8b1640b3648d7e4834a9206e08a47253307cdbc3")

	byteIds := make([][]byte, 0)
	intIds := make([]*big.Int, 0)
	for idx, idStr := range strIds {
		byteId, err := hex.DecodeString(idStr)
		if err != nil {
			panic(err)
		}

		byteIds = append(byteIds, byteId)

		intId := new(big.Int)
		intId.SetBytes(byteId)
		intIds = append(intIds, intId)

		fmt.Println(strIds[idx], byteIds[idx], intIds[idx])
	}

	idealTable := calculateIdealTable(byteIds[0])
	selfDist := new(big.Int)
	selfDist.Xor(idealTable[255], intIds[0])

	peerDist := new(big.Int)
	peerDist.Xor(idealTable[255], intIds[1])

	fmt.Printf("cmp result: %d\n", bytes.Compare(peerDist.Bytes(), selfDist.Bytes()))
	fmt.Printf("cmp result: %d\n", peerDist.Cmp(selfDist))
}
