package tests

import (
	"errors"
	"github.com/TACIXAT/party-line/white-box"
	"strconv"
	"testing"
	"time"
)

func checkBS(wb0, wb1 *whitebox.WhiteBox, successChan chan bool) {
	cache0, seen0 := wb0.PeerCache[wb1.PeerSelf.Id()]
	cache1, seen1 := wb1.PeerCache[wb0.PeerSelf.Id()]
	for !cache0.Added || !seen0 || !cache1.Added || !seen1 {
		cache0, seen0 = wb0.PeerCache[wb1.PeerSelf.Id()]
		cache1, seen1 = wb1.PeerCache[wb0.PeerSelf.Id()]
		time.Sleep(10 * time.Millisecond)
	}
	successChan <- true
}

func testBootstrap(wb0, wb1 *whitebox.WhiteBox, port1Str string) error {
	wb0.SendBootstrap("127.0.0.1:"+port1Str, wb1.BsId)

	successChan := make(chan bool)
	go checkBS(wb0, wb1, successChan)
	select {
	case <-successChan:
		// nop
	case <-time.After(500 * time.Millisecond):
		return errors.New("Failed to bootstrap.")
	}

	return nil
}

func testChat(wb0, wb1 *whitebox.WhiteBox) error {
	wb1.SendChat("你好")
	select {
	case <-wb1.ChatChannel:
		// nop
	case <-time.After(500 * time.Millisecond):
		return errors.New("No chat received from wb1.")
	}

	select {
	case <-wb0.ChatChannel:
		// nop
	case <-time.After(500 * time.Millisecond):
		return errors.New("No chat received from wb0.")
	}

	return nil
}

func checkInvite(wb *whitebox.WhiteBox, successChan chan bool) {
	for len(wb.PendingInvites) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	successChan <- true
}

func checkAccept(party *whitebox.PartyLine, successChan chan bool) {
	for len(party.MinList) < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	successChan <- true
}

func testParty(wb0, wb1 *whitebox.WhiteBox) error {
	min1 := wb1.PeerSelf.Min()

	partyId := wb0.PartyStart("coolname")
	party0 := wb0.Parties[partyId]
	party0.SendInvite(&min1)

	successChan := make(chan bool)
	go checkInvite(wb1, successChan)
	select {
	case <-successChan:
		// nop
	case <-time.After(500 * time.Millisecond):
		return errors.New("No invite received (was it sent?).")
	}

	wb1.AcceptInvite(partyId)
	go checkAccept(party0, successChan)
	select {
	case <-successChan:
		// nop
	case <-time.After(500 * time.Millisecond):
		return errors.New("No acceptance received (was it sent?).")
	}

	return nil
}

func testBlah(wb0, wb1 *whitebox.WhiteBox) error {
	return nil
}

func TestClientInteractions(t *testing.T) {
	var port0 uint16 = 3499
	port0Str := strconv.FormatInt(int64(port0), 10)
	dir0 := "/tmp/partylog.test0"
	wb0 := whitebox.New(dir0, "127.0.0.1", port0Str)
	wb0.Run(port0)

	var port1 uint16 = 4919
	port1Str := strconv.FormatInt(int64(port1), 10)
	dir1 := "/tmp/partylog.test1"
	wb1 := whitebox.New(dir1, "127.0.0.1", port1Str)
	wb1.Run(port1)

	err := testBootstrap(wb0, wb1, port1Str)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	err = testChat(wb0, wb1)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	err = testParty(wb0, wb1)
	if err != nil {
		t.Errorf(err.Error())
		return
	}
}
