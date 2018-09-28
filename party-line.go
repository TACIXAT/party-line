package main

import (
	"flag"
	"fmt"
	"github.com/TACIXAT/party-line/white-box"
	"log"
	"net"
	"os"
	"strconv"
)

/*
TODO:
	write client to client tests
	lock for sendpings, removestalepeers
	more deadlocks?

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

	use releases
	bs shortener
	perm nodes
*/

var chatChan chan string
var statusChan chan string

var debugFlag *bool
var portFlag *uint
var ipFlag *string
var nonatFlag *bool
var shareFlag *string

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

func main() {
	debugFlag = flag.Bool("debug", false, "Debug.")
	portFlag = flag.Uint("port", 3499, "Port.")
	ipFlag = flag.String("ip", "", "Manually set external IP.")
	nonatFlag = flag.Bool("nonat", false, "Disable UPNP and PMP.")
	shareFlag = flag.String("share", "", "Base directory to share from.")
	flag.Parse()

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

	wb := whitebox.New(dir, extIP.String(), portStr)

	// log to file
	// TODO: change name irl
	logname := fmt.Sprintf("/tmp/partylog.%s", wb.PeerSelf.Id()[:6])
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

	userInterface(wb)
}
