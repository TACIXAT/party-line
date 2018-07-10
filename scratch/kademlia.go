package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/kevinburke/nacl/sign"
	"math/big"
)

func calculateIdealTable(idBytes []byte) {
	id := new(big.Int)
	id.SetBytes(idBytes)
	fmt.Println(id)
	fmt.Println(hex.EncodeToString(idBytes))
	mask := new(big.Int)
	mask.SetUint64(1)
	for i := 0; i < 32; i++ {
		tmp := new(big.Int)
		fmt.Println(mask)
		tmp.Xor(id, mask)
		fmt.Println(tmp)
		mask.Lsh(mask, 1)
	}
}

func main() {
	r := rand.Reader
	id, _, err := sign.Keypair(r)
	if err != nil {
		panic(err)
	}
	calculateIdealTable(id)
}
