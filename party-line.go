package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/TACIXAT/party-line/white-box"
	"github.com/mitchellh/go-homedir"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

/*
TODO:
	bs perm node, wait for correct signal (don't just sleep 1 second)
	perm node connecting to self (why key not invalid?)
	use releases
	figure out smooth update process

	scrolling
	cursor on input
	better keys
	sent scrollback
	name color
	mute user
	more statuses

	check channel capcity before adding
	partial packs (save / resume on exit)

	measure block packet size
	increase block size

	make decent public interface (locking and shit)
	bs shortener
*/

var chatChan chan string
var statusChan chan string

var debugFlag *bool
var portFlag *uint
var ipFlag *string
var nonatFlag *bool
var shareFlag *string
var permFlag *bool

var permParties []string

func statusReceiver(wb *whitebox.WhiteBox) {
	for {
		status := <-wb.StatusChannel
		switch status.Priority {
		case whitebox.TONE_HIGH:
			chatStatus(status.Message)
		case whitebox.TONE_LOW:
			setStatus(status.Message)
		default:
			log.Println("unknown priority in message: ", status.Priority)
		}
	}
}

func chatReceiver(wb *whitebox.WhiteBox) {
	for {
		chat := <-wb.ChatChannel
		addChat(chat)
	}
}

// TODO: make this not a race condition
// TODO: flag to not do this
// TODO: select peer that isn't self
func bsInASecond(wb *whitebox.WhiteBox) {
	front, err := wb.IdFront(wb.PeerSelf.Id())
	if err != nil || permParties[1] == front {
		return
	}

	select {
	case <-time.After(1 * time.Second):
		wb.SendBootstrap(permParties[0], permParties[1])
	}
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func savePerm(self whitebox.Self) {
	home, err := homedir.Dir()
	if err != nil {
		log.Fatal("could not get home dir")
	}

	basePath := filepath.Join(home, "party-line")
	err = os.MkdirAll(basePath, 0700)
	if err != nil {
		log.Fatal(err)
	}

	// marshal json
	jsonSelf, err := json.Marshal(self)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("self size:", len(jsonSelf))
	path := filepath.Join(basePath, "perm.self")

	// write file
	selfFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}

	n, err := selfFile.Write([]byte(jsonSelf))
	if err != nil || n != len(jsonSelf) {
		log.Fatal(err)
	}

	err = selfFile.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func fetchPerm(self whitebox.Self) whitebox.Self {
	home, err := homedir.Dir()
	if err != nil {
		log.Fatal("could not get home dir")
	}
	path := filepath.Join(home, "party-line", "perm.self")

	exists, err := pathExists(path)
	if err != nil {
		log.Fatal(err)
	}

	if !exists {
		return self
	}

	// read file
	selfFile, err := os.Open(path)
	defer selfFile.Close()
	if err != nil {
		log.Fatal(err)
	}

	contents, err := ioutil.ReadAll(selfFile)
	if err != nil {
		log.Fatal(err)
	}

	// unmarshal json
	err = json.Unmarshal(contents, &self)
	if err != nil {
		log.Fatal(err)
	}

	return self
}

func main() {
	debugFlag = flag.Bool("debug", false, "Debug.")
	portFlag = flag.Uint("port", 3499, "Port.")
	ipFlag = flag.String("ip", "", "Manually set external IP.")
	nonatFlag = flag.Bool("nonat", false, "Disable UPNP and PMP.")
	shareFlag = flag.String("share", "", "Base directory to share from.")
	permFlag = flag.Bool("perm", false, "Use a permanent ID (keys).")
	flag.Parse()

	permParties = make([]string, 0)
	permParties = append(permParties, "138.197.201.244:3499")
	permParties = append(
		permParties,
		"3ce244e4426fd2cb1c41c5954c879ce0a3c19bf1452fb66be84de03825bc6f30")

	dir := ""
	if shareFlag != nil {
		dir = *shareFlag
	}

	// get port
	var port uint16 = uint16(*portFlag)

	// get external ip and open ports
	var extIP net.IP
	if *nonatFlag {
		if *ipFlag == "" {
			log.Fatal("Must provide an IP address with nonat flag.")
		}

		extIP = net.ParseIP(*ipFlag)
	} else {
		extIP = natStuff(port)
		defer natCleanup()
	}

	// build self info (addr, keys, id)
	portStr := strconv.FormatUint(uint64(port), 10)

	var self whitebox.Self
	if *permFlag {
		self = fetchPerm(self)
	}

	wb := whitebox.New(dir, extIP.String(), portStr, self)

	if *permFlag {
		savePerm(wb.Self)
	}

	// log to file
	// TODO: change name irl
	logname := fmt.Sprintf("partylog.%s", wb.PeerSelf.Id()[:6])
	logname = filepath.Join(os.TempDir(), logname)
	f, err := os.OpenFile(logname, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(f)

	chatChan = make(chan string, 1)
	statusChan = make(chan string, 1)

	go statusReceiver(wb)
	go chatReceiver(wb)

	// start network receiver
	wb.Run(port)

	bsInASecond(wb)
	userInterface(wb)
}
