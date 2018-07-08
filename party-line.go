package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	mrand "math/rand"
	"flag"
	"encoding/gob"
	"encoding/hex"
	"io"
	"log"
)

func main() {
	debug := flag.Bool("debug", false, "Debug flag.")
	flag.Parse()

	var port uint16 = 3499

	natStuff(port)

	var r io.Reader
	if *debug {
		r = mrand.New(mrand.NewSource(int64(port)))
	} else {
		r = rand.Reader
	}

	key, err := rsa.GenerateKey(r, 4096)
	if err != nil {
		log.Fatal(err)
	}

	var buf bytes.Buffer
    enc := gob.NewEncoder(&buf)
	err = enc.Encode(key.PublicKey)
	if err != nil {
		log.Fatal(err)
	}

	pubBytes := buf.Bytes()
	hasher := sha256.New()
	hasher.Write(pubBytes)

	log.Printf(hex.EncodeToString(hasher.Sum(nil)))
	defer natCleanup()
}