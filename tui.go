package main

import (
	"fmt"
	"github.com/TACIXAT/party-line/white-box"
	"github.com/gizak/termui"
	"github.com/mattn/go-runewidth"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

var chatLog []whitebox.Chat
var messageBox *termui.Par
var IDS int // id display size
var chatMutex *sync.Mutex
var show string

func init() {
	IDS = 6
	chatMutex = new(sync.Mutex)
	show = ""
}

func displayId(id string) string {
	idLen := len(id)
	if IDS <= idLen {
		return id[:IDS]
	}

	return strings.Repeat(" ", IDS-idLen) + id
}

func displayChannel(channel string) string {
	channelLen := len(channel)
	if 8 <= channelLen {
		return channel[:8]
	}

	return strings.Repeat(" ", 8-channelLen) + channel
}

func formatChatsFit() string {
	height := messageBox.Height - 2
	width := messageBox.Width - 2

	lines := make([]string, 0)
	for i := 0; i < len(chatLog); i++ {
		chat := chatLog[i]

		if show != "" && chat.Channel != "" && show != chat.Channel {
			continue
		}

		msg := chat.Time.Format("15:04:05 ")
		msg += "(" + displayChannel(chat.Channel) + ") "
		msg += displayId(chat.Id) + " "
		msg += chat.Message

		if i != len(chatLog)-1 && msg[len(msg)-1] != '\n' {
			msg += "\n"
		}

		length := runewidth.StringWidth(msg) / width

		for length > 0 {
			line := msg[:width]
			msg = msg[width:]
			lines = append(lines, line)
			length--
		}

		if len(msg) > 0 {
			lines = append(lines, msg)
		}
	}

	start := len(lines) - height
	if start < 0 {
		start = 0
	}

	return strings.Join(lines[start:], "")
}

func formatChats() string {
	if messageBox != nil {
		return formatChatsFit()
	}

	chatStr := ""
	for i := 0; i < len(chatLog); i++ {
		chat := chatLog[i]
		if show != "" && show != chat.Channel {
			continue
		}

		if i != 0 {
			chatStr += "\n"
		}

		msg := chat.Time.Format("15:04:05 ")
		msg += "(" + displayChannel(chat.Channel) + ") "
		msg += displayId(chat.Id) + " "
		msg += chat.Message
		chatStr += msg
	}

	return chatStr
}

func handleBootstrap(wb *whitebox.WhiteBox, toks []string) {
	if len(toks) == 1 {
		chatStatus(wb.BsId)
		return
	}

	if len(toks) != 2 {
		setStatus("error processing bootstrap command")
		return
	}

	bs := toks[1]
	bsToks := strings.Split(bs, "/")
	if len(bsToks) != 3 {
		setStatus("error processing bootstrap command")
		return
	}

	ip := bsToks[0]
	port := bsToks[1]
	id := bsToks[2]
	addr := ip + ":" + port

	wb.SendBootstrap(addr, id)
}

func chatStatus(status string) {
	log.Println(status)
	chat := whitebox.Chat{
		Time:    time.Now(),
		Id:      "SYSTEM",
		Channel: "",
		Message: status}

	addChat(chat)
}

func setStatus(status string) {
	statusChan <- status
}

func addChat(chat whitebox.Chat) {
	chatMutex.Lock()
	chatLog = append(chatLog, chat)
	chats := formatChats()
	chatChan <- chats
	chatMutex.Unlock()
}

func redrawChats() {
	chats := formatChats()
	chatChan <- chats
}

func handleChat(wb *whitebox.WhiteBox, buf string) {
	if show == "" || show == "mainline" {
		wb.SendChat(buf)
		setStatus("sent")
		return
	}

	wb.Parties.Mutex.Lock()
	defer wb.Parties.Mutex.Unlock()
	for partyId, party := range wb.Parties.Map {
		if show == partyId {
			party.SendChat(buf)
			setStatus("sent")
			return
		}
	}
}

func statusSetter(statusBox *termui.Par) {
	for {
		status := <-statusChan
		log.Println(status)
		statusBox.Text = status
		termui.Clear()
		termui.Render(termui.Body)
	}
}

func chatDrawer(messageBox *termui.Par) {
	for {
		chatsFormatted := <-chatChan
		messageBox.Text = chatsFormatted
		termui.Clear()
		termui.Render(termui.Body)
	}
}

// create channel
func handleStart(wb *whitebox.WhiteBox, toks []string) {
	if len(toks) < 2 {
		setStatus("error insufficient args to create command")
		return
	}

	id := wb.PartyStart(toks[1])
	if id != "" {
		setStatus(fmt.Sprintf("party started %s", id))
	}
}

// invite channel
func handleInvite(wb *whitebox.WhiteBox, toks []string) {
	if len(toks) < 3 {
		setStatus("error insufficient args to invite command")
		return
	}

	partyPrefix := toks[1]
	userPrefix := toks[2]

	// iterate parties
	var party *whitebox.PartyLine
	wb.Parties.Mutex.Lock()
	for id, p := range wb.Parties.Map {
		if strings.HasPrefix(id, partyPrefix) {
			if party != nil {
				setStatus(fmt.Sprintf(
					"error multiple parties found for %s", partyPrefix))
				wb.Parties.Mutex.Unlock()
				return
			}
			party = p
		}
	}
	wb.Parties.Mutex.Unlock()

	if party == nil {
		setStatus(fmt.Sprintf("error party not found for %s", partyPrefix))
		return
	}

	// iterate peers
	var min *whitebox.MinPeer
	wb.PeerCache.Mutex.Lock()
	for id, _ := range wb.PeerCache.Map {
		front, err := wb.IdFront(id)
		if err != nil {
			setStatus("error decoding peer id")
			log.Println(err)
			continue
		}

		if strings.HasPrefix(front, userPrefix) {
			if min != nil {
				setStatus(fmt.Sprintf(
					"error multiple peers found for %s", userPrefix))
				wb.PeerCache.Mutex.Unlock()
				return
			}

			min, err = wb.IdToMin(id)
			if err != nil {
				setStatus("error decoding peer")
				log.Println(err)
				continue
			}
		}
	}
	wb.PeerCache.Mutex.Unlock()

	if min == nil {
		setStatus(fmt.Sprintf("error peer not found for %s", userPrefix))
		return
	}

	party.SendInvite(min)
}

// show message visibility
func handleShow(wb *whitebox.WhiteBox, toks []string) {
	if len(toks) < 2 {
		show = ""
		redrawChats()
		return
	}

	if strings.HasPrefix("all", toks[1]) {
		show = ""
		redrawChats()
		return
	}

	if strings.HasPrefix("mainline", toks[1]) {
		show = "mainline"
		redrawChats()
		return
	}

	partyPrefix := toks[1]

	var partyId string
	wb.Parties.Mutex.Lock()
	for id, _ := range wb.Parties.Map {
		if strings.HasPrefix(id, partyPrefix) {
			if partyId != "" {
				setStatus(fmt.Sprintf(
					"error multiple parties found for %s", partyPrefix))
				wb.Parties.Mutex.Unlock()
				return
			}
			partyId = id
		}
	}
	wb.Parties.Mutex.Unlock()

	if partyId == "" {
		setStatus(fmt.Sprintf("error party not found for %s", partyPrefix))
		return
	}

	show = partyId
	redrawChats()
}

// set id display size
func handleIds(toks []string) {
	if len(toks) < 2 {
		setStatus("error insufficient args to ids command")
		return
	}

	// hex for lol
	size, err := strconv.ParseInt(toks[1], 16, 64)
	if err != nil {
		setStatus("error insufficient args to ids command")
		log.Println(err)
		return
	}

	IDS = int(size)
	setStatus(fmt.Sprintf("set id display size to %d", IDS))
	redrawChats()
}

// send message on channel
func handleSend(wb *whitebox.WhiteBox, toks []string) {
	if len(toks) < 3 {
		setStatus("error insufficient args to leave command")
		return
	}

	partyPrefix := toks[1]
	message := strings.Join(toks[2:], " ")

	// iterate parties
	partyId := ""
	wb.Parties.Mutex.Lock()
	for id, _ := range wb.Parties.Map {
		if strings.HasPrefix(id, partyPrefix) {
			if partyId != "" {
				setStatus(fmt.Sprintf(
					"error multiple parties found for %s", partyPrefix))
				wb.Parties.Mutex.Unlock()
				return
			}
			partyId = id
		}
	}
	wb.Parties.Mutex.Unlock()

	if partyId == "" {
		setStatus(fmt.Sprintf("error party not found for %s", partyPrefix))
		return
	}

	wb.Parties.Mutex.Lock()
	wb.Parties.Map[partyId].SendChat(message)
	wb.Parties.Mutex.Unlock()
	return
}

// clear messages
func handleClear(toks []string) {
	chatLog = nil
	redrawChats()
}

func handleList(wb *whitebox.WhiteBox, toks []string) {
	show := "both"
	if len(toks) > 2 {
		if strings.HasPrefix("invites", toks[1]) {
			show = "invites"
		} else if strings.HasPrefix("parties", toks[1]) {
			show = "parties"
		}
	}

	if show == "both" || show == "parties" {
		chatStatus("      ==== PARTY LIST ====      ")
		if wb.Parties.Len() > 0 {
			wb.Parties.Mutex.Lock()
			for id, _ := range wb.Parties.Map {
				chatStatus(fmt.Sprintf("%s", id))
			}
			wb.Parties.Mutex.Unlock()
		} else {
			chatStatus("           no parties           ")
		}
	}

	if show == "both" || show == "parties" {
		chatStatus("  ==== ACCEPTANCE PENDING ====  ")
		if wb.PendingInvites.Len() > 0 {
			wb.PendingInvites.Mutex.Lock()
			for id, _ := range wb.PendingInvites.Map {
				chatStatus(fmt.Sprintf("%s", id))
			}
			wb.PendingInvites.Mutex.Unlock()
		} else {
			chatStatus("           no invites           ")
		}
	}
}

func handleAccept(wb *whitebox.WhiteBox, toks []string) {
	if len(toks) < 2 {
		setStatus("error insufficient args to join command")
		return
	}

	partyPrefix := toks[1]

	// iterate pending
	partyId := ""
	wb.PendingInvites.Mutex.Lock()
	for id, _ := range wb.PendingInvites.Map {
		if strings.HasPrefix(id, partyPrefix) {
			if partyId != "" {
				setStatus(fmt.Sprintf(
					"error multiple invites found for %s", partyPrefix))
				return
			}
			partyId = id
		}
	}
	wb.PendingInvites.Mutex.Unlock()

	if partyId == "" {
		setStatus(fmt.Sprintf("error invite not found for %s", partyPrefix))
		return
	}

	wb.AcceptInvite(partyId)
}

func handleLeave(wb *whitebox.WhiteBox, toks []string) {
	if len(toks) < 2 {
		setStatus("error insufficient args to leave command")
		return
	}

	partyPrefix := toks[1]

	// iterate parties
	partyId := ""
	wb.Parties.Mutex.Lock()
	for id, _ := range wb.Parties.Map {
		if strings.HasPrefix(id, partyPrefix) {
			if partyId != "" {
				setStatus(fmt.Sprintf(
					"error multiple parties found for %s", partyPrefix))
				wb.Parties.Mutex.Unlock()
				return
			}
			partyId = id
		}
	}
	wb.Parties.Mutex.Unlock()

	if partyId == "" {
		setStatus(fmt.Sprintf("error party not found for %s", partyPrefix))
		return
	}

	wb.Parties.Mutex.Lock()
	wb.Parties.Map[partyId].SendDisconnect()
	wb.Parties.Mutex.Unlock()
	setStatus("left the party " + partyId)
}

func handlePacks(wb *whitebox.WhiteBox, toks []string) {
	wb.Parties.Mutex.Lock()
	for partyId, party := range wb.Parties.Map {
		chatStatus("== " + partyId + " ==")
		party.PacksLock.Lock()
		for packHash, lockingPack := range party.Packs {
			lockingPack.Mutex.Lock()
			pack := lockingPack.Pack
			line := "\"" + pack.Name + "\""
			// TODO: change to count in last TIME
			line += " (" + strconv.FormatInt(int64(len(pack.Peers)), 10) + ")"

			if pack.State == whitebox.COMPLETE {
				line += "*"
			}

			chatStatus("PACK: " + packHash)
			chatStatus(line)

			pack.FileLock.Lock()
			// TODO: is file lock still necessary now that we have pack lock?
			for _, packFileInfo := range pack.Files {
				chatStatus("  FILE: " + packFileInfo.Hash)
				chatStatus("  \"" + packFileInfo.Name + "\"")
			}
			pack.FileLock.Unlock()
			lockingPack.Mutex.Unlock()
		}
		party.PacksLock.Unlock()
	}
	wb.Parties.Mutex.Unlock()
}

func handleGet(wb *whitebox.WhiteBox, toks []string) {
	// find party
	if len(toks) < 3 {
		setStatus("error insufficient args to get command")
		return
	}

	partyPrefix := toks[1]

	// iterate parties
	partyId := ""
	wb.Parties.Mutex.Lock()
	for id, _ := range wb.Parties.Map {
		if strings.HasPrefix(id, partyPrefix) {
			if partyId != "" {
				setStatus(fmt.Sprintf(
					"error multiple parties found for %s", partyPrefix))
				wb.Parties.Mutex.Unlock()
				return
			}
			partyId = id
		}
	}
	wb.Parties.Mutex.Unlock()

	if partyId == "" {
		setStatus(fmt.Sprintf("error party not found for %s", partyPrefix))
		return
	}

	hashPrefix := toks[2]

	// find pack
	packHash := ""
	wb.Parties.Mutex.Lock()
	party := wb.Parties.Map[partyId]
	wb.Parties.Mutex.Unlock()

	party.PacksLock.Lock()
	for hash, _ := range party.Packs {
		if strings.HasPrefix(hash, hashPrefix) {
			if packHash != "" {
				setStatus(fmt.Sprintf(
					"error multiple packs found for %s", hashPrefix))
				party.PacksLock.Unlock()
				return
			}
			packHash = hash
		}
	}
	party.PacksLock.Unlock()

	if packHash == "" {
		setStatus(fmt.Sprintf("error pack not found for %s", hashPrefix))
		return
	}

	party.StartPack(packHash)
}

func handleHelp() {
	chatStatus("this is probably wildly out of date...")
	chatStatus("/bs [bootstrap info]")
	chatStatus("    show bs info (no arg) or bootstrap to a peer")
	chatStatus("/start <party_name>")
	chatStatus("    start a party (name limit 8 characters)")
	chatStatus("/invite <party_id> <user_id>")
	chatStatus("    invite a user to a party (partial ids ok)")
	chatStatus("/accept <party_id>")
	chatStatus("    accept an invite (partial ids ok)")
	chatStatus("/list [parties|invites]")
	chatStatus("    list parties, invites, or both")
	chatStatus("/send <party_id> msg")
	chatStatus("    send message to party (partial id ok)")
	chatStatus("/leave <party_id>")
	chatStatus("    leaves the party (partial id ok)")
	chatStatus("/show [all|mainline|party_id]")
	chatStatus("    change what messages are displayed (partial id ok)")
	chatStatus("/clear")
	chatStatus("    clear chat log")
	chatStatus("/ids <size>")
	chatStatus("    change id display size (hex)")
	chatStatus("/packs")
	chatStatus("    list available packs")
	chatStatus("/get <party_id> <pack_id>")
	chatStatus("    get a pack from a party (partial ids ok)")
	chatStatus("/rescan")
	chatStatus("    recan share dir for new packs")
	chatStatus("/help")
	chatStatus("    display this")
	chatStatus("/quit")
	chatStatus("    we'll miss you, but it was fun while you were here")
	return
}

func handleUserInput(wb *whitebox.WhiteBox, buf string) {
	if len(buf) == 0 {
		return
	}

	toks := strings.Split(buf, " ")
	switch toks[0] {
	case "/bs":
		handleBootstrap(wb, toks)
	case "/start":
		handleStart(wb, toks)
	case "/invite":
		handleInvite(wb, toks)
	case "/accept":
		handleAccept(wb, toks)
	case "/list":
		handleList(wb, toks)
	case "/send":
		handleSend(wb, toks)
	case "/leave":
		handleLeave(wb, toks)
	case "/show":
		handleShow(wb, toks)
	case "/clear":
		handleClear(toks)
	case "/ids":
		handleIds(toks)
	case "/packs":
		handlePacks(wb, toks)
	case "/get":
		handleGet(wb, toks)
	case "/rescan":
		wb.RescanPacks()
	case "/help":
		handleHelp()
	case "/quit":
		wb.DisconnectParties()
		wb.SendDisconnect()
		termui.StopLoop()
	default:
		handleChat(wb, buf)
	}
}

func userInterface(wb *whitebox.WhiteBox) {
	err := termui.Init()
	if err != nil {
		panic(err)
	}
	defer termui.Close()

	messageBox = termui.NewPar("")
	messageBox.Height = termui.TermHeight() - 4
	messageBox.BorderLabel = "Party-Line"
	messageBox.BorderLabelFg = termui.ColorYellow
	messageBox.BorderFg = termui.ColorMagenta

	inputBox := termui.NewPar("")
	inputBox.Height = 3

	statusBox := termui.NewPar("good")
	statusBox.Height = 1
	statusBox.Bg = termui.ColorBlue
	statusBox.TextBgColor = termui.ColorBlue
	statusBox.TextFgColor = termui.ColorWhite
	statusBox.Border = false

	termui.Body.AddRows(
		termui.NewRow(
			termui.NewCol(12, 0, messageBox)),
		termui.NewRow(
			termui.NewCol(12, 0, inputBox)),
		termui.NewRow(
			termui.NewCol(12, 0, statusBox)))

	termui.Body.Align()

	termui.Render(termui.Body)

	go chatDrawer(messageBox)
	go statusSetter(statusBox)

	buf := ""
	termui.Handle("/sys/kbd/<enter>", func(evt termui.Event) {
		handleUserInput(wb, buf)
		buf = ""
		inputBox.Text = buf
		termui.Clear()
		termui.Render(termui.Body)
	})

	termui.Handle("/sys/kbd/<backspace>", func(evt termui.Event) {
		if len(buf) > 0 {
			buf = buf[:len(buf)-1]
		}
		start := len(buf) - inputBox.Width + 2
		if start < 0 {
			start = 0
		}
		inputBox.Text = buf[start:]
		termui.Clear()
		termui.Render(termui.Body)
	})

	termui.Handle("/sys/kbd/C-8", func(evt termui.Event) {
		if len(buf) > 0 {
			buf = buf[:len(buf)-1]
		}
		start := len(buf) - inputBox.Width + 2
		if start < 0 {
			start = 0
		}
		inputBox.Text = buf[start:]
		termui.Clear()
		termui.Render(termui.Body)
	})

	termui.Handle("/sys/kbd/<space>", func(evt termui.Event) {
		buf += " "
		start := len(buf) - inputBox.Width + 2
		if start < 0 {
			start = 0
		}
		inputBox.Text = buf[start:]
		termui.Clear()
		termui.Render(termui.Body)
	})

	termui.Handle("/sys/kbd/", func(evt termui.Event) {
		buf += evt.Data.(termui.EvtKbd).KeyStr
		start := len(buf) - inputBox.Width + 2
		if start < 0 {
			start = 0
		}
		inputBox.Text = buf[start:]
		termui.Clear()
		termui.Render(termui.Body)
	})

	termui.Handle("/sys/wnd/resize", func(e termui.Event) {
		termui.Body.Width = termui.TermWidth()
		messageBox.Height = termui.TermHeight() - 4
		termui.Body.Align()
		termui.Clear()
		termui.Render(termui.Body)
		redrawChats()
	})

	termui.Loop()
}
