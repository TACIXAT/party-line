package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/kevinburke/nacl"
	"github.com/kevinburke/nacl/box"
)

type MessageBootstrap struct {
	Name   string
	EncPub nacl.Key
}

func main() {
	r := rand.Reader
	pub, _, err := box.GenerateKey(r)
	if err != nil {
		panic(err)
	}

	var m MessageBootstrap
	m.Name = "tacixat"
	m.EncPub = pub

	out, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(out))

	var mb MessageBootstrap
	json.Unmarshal(out, &mb)

	fmt.Println(mb.Name)
}
