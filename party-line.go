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
	test dropped pack name in receiving file structure
	unthrottle fulfillments

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

	use releases
	bs shortener
	perm nodes

	CONV TODO:
		status receiver go routine
		chat receiver go routine
*/

var chatChan chan string
var statusChan chan string

var debugFlag *bool
var portFlag *uint
var ipFlag *string
var nonatFlag *bool
var shareFlag *string

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

	// var wg sync.WaitGroup
	// ctrlChan := make(chan bool, 1)

	// // start network receiver
	go wb.Recv("", port)
	go wb.SendPings()
	go wb.FileRequester()
	go wb.RequestSender()
	go wb.VerifiedBlockWriter()
	go wb.Advertise()

	userInterface(wb)
}
