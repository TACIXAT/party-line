package main

import "fmt"
import "unicode/utf8"
import "golang.org/x/text/width"
import "github.com/mattn/go-runewidth"

func getm() map[int]string {
	m := make(map[int]string)
	return m
}

func main() {
	var m map[int]string
	m = getm()
	m[1] = "a"
	fmt.Printf("%v\n", m)
	msg := "你好！d"
	nsg := "aabbccd"
	fmt.Println(msg)
	fmt.Println(nsg)
	fmt.Println(len(msg))
	r, byteCount := utf8.DecodeRuneInString(msg)
	fmt.Println(byteCount)

	p := width.LookupRune(r)
	fmt.Printf("%c %v\n", r, p.Kind())

	q := width.LookupRune(rune(nsg[0]))
	fmt.Printf("%c %v\n", nsg[0], q.Kind())

	fmt.Println(runewidth.StringWidth(msg))
	fmt.Println(runewidth.StringWidth(nsg))
}
