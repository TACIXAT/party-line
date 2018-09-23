package beigebox

import (
	"errors"
	"github.com/TACIXAT/party-line/white-box"
	"log"
	"os"
	"path/filepath"
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
	case <-time.After(1000 * time.Millisecond):
		return errors.New("Failed to bootstrap (timeout).")
	}

	return nil
}

func testChat(wb0, wb1 *whitebox.WhiteBox) error {
	wb1.SendChat("你好")
	select {
	case <-wb1.ChatChannel:
		// nop
	case <-time.After(1000 * time.Millisecond):
		return errors.New("No chat received by wb1 (timeout).")
	}

	select {
	case <-wb0.ChatChannel:
		// nop
	case <-time.After(1000 * time.Millisecond):
		return errors.New("No chat received by wb0 (timeout).")
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

func testPartyInvite(wb0, wb1 *whitebox.WhiteBox) (string, error) {
	min1 := wb1.PeerSelf.Min()

	partyId := wb0.PartyStart("coolname")
	party0 := wb0.Parties[partyId]
	party0.SendInvite(&min1)

	successChan := make(chan bool)
	go checkInvite(wb1, successChan)
	select {
	case <-successChan:
		// nop
	case <-time.After(1000 * time.Millisecond):
		return "", errors.New("No invite received (timeout).")
	}

	wb1.AcceptInvite(partyId)
	go checkAccept(party0, successChan)
	select {
	case <-successChan:
		// nop
	case <-time.After(1000 * time.Millisecond):
		return "", errors.New("No acceptance received (timeout).")
	}

	return partyId, nil
}

func testPartyChat(wb0, wb1 *whitebox.WhiteBox, partyId string) error {
	party1 := wb1.Parties[partyId]
	party1.SendChat("encrypted lol :D fuck nasa spies")

	select {
	case chat := <-wb1.ChatChannel:
		if chat.Channel != partyId {
			return errors.New("Bad channel for chat received by wb1.")
		}
	case <-time.After(1000 * time.Millisecond):
		return errors.New("No chat received by wb1 (timeout).")
	}

	select {
	case chat := <-wb0.ChatChannel:
		if chat.Channel != partyId {
			return errors.New("Bad channel for chat received by wb0.")
		}
	case <-time.After(1000 * time.Millisecond):
		return errors.New("No chat received by wb0 (timeout).")
	}

	return nil
}

func checkPack(wb *whitebox.WhiteBox, partyId string, successChan chan bool) {
	for len(wb.Parties[partyId].Packs) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	successChan <- true
}

func testScanPack(wb0, wb1 *whitebox.WhiteBox, partyId string) error {
	partyDir := filepath.Join(wb0.SharedDir, partyId)
	err := os.MkdirAll(partyDir, 0700)
	if err != nil {
		log.Println("(TEST)", err)
		return errors.New("Error creating party dir.")
	}

	testPackPath := filepath.Join(partyDir, "test.pack")
	packContents := `
		{
			"name": "test.pack",
			"files": [
				"test.file"
			]
		}`

	testPackFile, err := os.OpenFile(testPackPath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Println("(TEST)", err)
		return errors.New("Error creating pack file.")
	}

	n, err := testPackFile.Write([]byte(packContents))
	if err != nil || n != len(packContents) {
		return errors.New("Error writing pack file.")
	}

	err = testPackFile.Close()
	if err != nil {
		return errors.New("Error closing pack file.")
	}

	testFilePath := filepath.Join(partyDir, "test.file")
	fileContents :=
		`X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`

	testFileFile, err := os.OpenFile(testFilePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return errors.New("Error creating file file.")
	}

	n, err = testFileFile.Write([]byte(fileContents))
	if err != nil || n != len(fileContents) {
		return errors.New("Error writing file file.")
	}

	err = testFileFile.Close()
	if err != nil {
		return errors.New("Error closing file file.")
	}

	wb0.RescanPacks()

	successChan := make(chan bool)
	go checkPack(wb0, partyId, successChan)
	select {
	case <-successChan:
		// nop
	case <-time.After(1000 * time.Millisecond):
		return errors.New("No pack created (timeout).")
	}

	go checkPack(wb1, partyId, successChan)
	select {
	case <-successChan:
		// nop
	case <-time.After(1000 * time.Millisecond):
		return errors.New("No pack received (timeout).")
	}

	return nil
}

func checkDownload(
	wb *whitebox.WhiteBox, partyId, packHash string, successChan chan bool) {
	for wb.Parties[partyId].Packs[packHash].State != whitebox.COMPLETE {
		time.Sleep(10 * time.Millisecond)
	}
	successChan <- true
}

func testGetPack(wb *whitebox.WhiteBox, partyId string) error {
	party := wb.Parties[partyId]

	var packHash string
	var pack *whitebox.Pack
	for packHash, pack = range party.Packs {
		break
	}

	if pack == nil {
		return errors.New("Pack not found.")
	}

	if pack.State != whitebox.AVAILABLE {
		return errors.New("Pack not available.")
	}

	party.StartPack(packHash)

	successChan := make(chan bool)
	go checkDownload(wb, partyId, packHash, successChan)
	select {
	case <-successChan:
		// nop
	case <-time.After(30000 * time.Millisecond):
		return errors.New("No pack downloaded (timeout).")
	}

	return nil
}

func testDisconnect(wb0, wb1 *whitebox.WhiteBox) error {
	// causes nilptr while download is going
	// wb0.DisconnectParties()
	// validate dc

	// wb0.SendDisconnect()
	// validate dc

	return nil
}

func testTemplate(wb0, wb1 *whitebox.WhiteBox) error {
	return nil
}

func TestClientInteractions(t *testing.T) {
	var partyId string // predec so we can use goto

	var port0 uint16 = 3499
	port0Str := strconv.FormatInt(int64(port0), 10)
	dir0 := filepath.Join(os.TempDir(), "partytest.dir0")
	wb0 := whitebox.New(dir0, "127.0.0.1", port0Str)
	wb0.Run(port0)

	var port1 uint16 = 4919
	port1Str := strconv.FormatInt(int64(port1), 10)
	dir1 := filepath.Join(os.TempDir(), "partytest.dir1")
	wb1 := whitebox.New(dir1, "127.0.0.1", port1Str)
	wb1.Run(port1)

	err := testBootstrap(wb0, wb1, port1Str)
	if err != nil {
		t.Errorf(err.Error())
		goto cleanup
	}

	err = testChat(wb0, wb1)
	if err != nil {
		t.Errorf(err.Error())
		goto cleanup
	}

	partyId, err = testPartyInvite(wb0, wb1)
	if err != nil {
		t.Errorf(err.Error())
		goto cleanup
	}

	err = testPartyChat(wb0, wb1, partyId)
	if err != nil {
		t.Errorf(err.Error())
		goto cleanup
	}

	err = testScanPack(wb0, wb1, partyId)
	if err != nil {
		t.Errorf(err.Error())
		goto cleanup
	}

	err = testGetPack(wb1, partyId)
	if err != nil {
		t.Errorf(err.Error())
		goto cleanup
	}

	err = testDisconnect(wb0, wb1)
	if err != nil {
		t.Errorf(err.Error())
		goto cleanup
	}

cleanup:
	os.RemoveAll(dir0)
	os.RemoveAll(dir1)
	return
}
