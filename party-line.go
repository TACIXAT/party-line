package main

import (
	"github.com/douggard/party-line/white-box"
	"flag"
	"net"
	"log"
)

/*
TODO:
	test dropped pack name in receiving file structure
	unthrottle fulfillments

	scrolling
	cursor on input
	better keys
	sent scrollback
	bs shortener
	name color
	mute user
	more statuses

	check channel capcity before adding
	partial packs (save / resume on exit)

	measure block packet size
	increase block size
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

	wb := whitebox.New("default")
	dir := ""
	if shareFlag != nil {
		dir = *shareFlag
	}
	wb.InitFiles(dir)

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

	log.Println(extIP)

	// // build self info (addr, keys, id)
	// portStr := strconv.FormatUint(uint64(port), 10)
	// self.Address = extIP.String() + ":" + portStr
	// getKeys()

	// // log to file
	// logname := fmt.Sprintf("/tmp/partylog.%s", peerSelf.Id()[:6])
	// f, err := os.OpenFile(logname, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	// if err != nil {
	// 	log.Fatalf("error opening file: %v", err)
	// }
	// defer f.Close()

	// log.SetFlags(log.LstdFlags | log.Lshortfile)
	// log.SetOutput(f)

	// calculateIdealTableSelf(self.SignPub)
	// initTable(self.SignPub)

	// seenChats = make(map[string]bool)
	// chatChan = make(chan string, 1)
	// statusChan = make(chan string, 1)
	// bsId = fmt.Sprintf("%s/%s/%s", extIP.String(), portStr, peerSelf.ShortId())
	// log.Println(bsId)
	// chatStatus(bsId)

	// // var wg sync.WaitGroup
	// // ctrlChan := make(chan bool, 1)

	// // start network receiver
	// go recv("", port)
	// go sendPings()
	// go fileRequester()
	// go requestSender()
	// go verifiedBlockWriter()

	// userInterface()
}
