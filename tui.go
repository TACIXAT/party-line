package main

import (
	"fmt"
	"github.com/gizak/termui"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Chat struct {
	Time    time.Time
	Id      string
	Channel string
	Message string
}

var chatLog []Chat
var messageBox *termui.Par
var IDS int // id display size
var chatMutex *sync.Mutex

func init() {
	IDS = 6
	chatMutex = new(sync.Mutex)
}

func displayId(id string) string {
		idLen := len(id)
		if IDS <= idLen {
			return id[:IDS]
		}
		
		return id + strings.Repeat(" ", IDS - idLen)	
}		

func formatChatsFit() string {
	height := messageBox.Height - 2
	width := messageBox.Width - 2

	lines := make([]string, 0)
	for i := 0; i < len(chatLog); i++ {
		chat := chatLog[i]
		msg := chat.Time.Format("15:04:05 ") + displayId(chat.Id) + " " + chat.Message
		if i != len(chatLog)-1 && msg[len(msg)-1] != '\n' {
			msg += "\n"
		}

		length := len(msg) / width

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
		if i != 0 {
			chatStr += "\n"
		}

		msg := chat.Time.Format("15:04:05 ") + displayId(chat.Id) + " " + chat.Message
		chatStr += msg
	}

	return chatStr
}

func handleBootstrap(toks []string) {
	if len(toks) == 1 {
		chatStatus(bsId)
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

	sendBootstrap(addr, id)
}

func chatStatus(status string) {
	chat := Chat{
		Time:    time.Now(),
		Id:      "SYSTEM",
		Channel: "",
		Message: status}

	addChat(chat)
}

func setStatus(status string) {
	statusChan <- status
}

func addChat(chat Chat) {
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

func handleChat(buf string) {
	sendChat(buf)
	setStatus("sent")
}

func statusSetter(statusBox *termui.Par) {
	for {
		status := <-statusChan
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
func handleStart(toks []string) {
	if len(toks) < 2 {
		setStatus("error insufficient args to create command")
		return
	}

	id := partyStart(toks[1])
	setStatus(fmt.Sprintf("party started %s", id))
}

// invite channel
func handleInvite(toks []string) {
	if len(toks) < 3 {
		setStatus("error insufficient args to invite command")
		return
	}

	partyPrefix := toks[1]
	userPrefix := toks[2]

	// iterate parties
	var party *PartyLine
	for id, p := range parties {
		if strings.HasPrefix(id, partyPrefix) {
			if party != nil {
				setStatus(fmt.Sprintf(
					"error multiple parties found for %s", partyPrefix))
				return
			}
			party = p
		}
	}

	if party == nil {
		setStatus(fmt.Sprintf("error party not found for %s", partyPrefix))
		return
	}

	// iterate peers
	var min *MinPeer
	for id, _ := range peerCache {
		front, err := idFront(id)
		if err != nil {
			setStatus("error decoding peer id")
			log.Println(err)
			continue
		}

		if strings.HasPrefix(front, userPrefix) {
			if min != nil {
				setStatus(fmt.Sprintf(
					"error multiple peers found for %s", userPrefix))
				return
			}

			min, err = idToMin(id)
			if err != nil {
				setStatus("error decoding peer")
				log.Println(err)
				continue
			}
		}
	}

	if min == nil {
		setStatus(fmt.Sprintf("error peer not found for %s", userPrefix))
		return
	}

	party.SendInvite(min)
}

// show message visibility
func handleShow(toks []string) {
	// all
	// mainline
	// channel name
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
func handleSend(toks []string) {
	if len(toks) < 3 {
		setStatus("error insufficient args to leave command")
		return
	}

	partyPrefix := toks[1]
	message := strings.Join(toks[2:], " ")

	// iterate parties
	partyId := ""
	for id, _ := range parties {
		if strings.HasPrefix(id, partyPrefix) {
			if partyId != "" {
				setStatus(fmt.Sprintf(
					"error multiple parties found for %s", partyPrefix))
				return
			}
			partyId = id
		}
	}

	if partyId == "" {
		setStatus(fmt.Sprintf("error party not found for %s", partyPrefix))
		return
	}

	parties[partyId].SendChat(message)
	return
}

// clear messages
func handleClear(toks []string) {
	chatLog = nil
	redrawChats()
}

func handleList() {
	if len(parties) == 0 {
		setStatus("error no parties to list")
		return
	}

	for id, _ := range parties {
		chatStatus(fmt.Sprintf("%s", id))
	}
}

func handleLeave(toks []string) {
	if len(toks) < 2 {
		setStatus("error insufficient args to leave command")
		return
	}

	partyPrefix := toks[1]

	// iterate parties
	partyId := ""
	for id, _ := range parties {
		if strings.HasPrefix(id, partyPrefix) {
			if partyId != "" {
				setStatus(fmt.Sprintf(
					"error multiple parties found for %s", partyPrefix))
				return
			}
			partyId = id
		}
	}

	if partyId == "" {
		setStatus(fmt.Sprintf("error party not found for %s", partyPrefix))
		return
	}

	parties[partyId].SendDisconnect()
}

func handleHelp() {
	chatStatus("this is probably wildly out of date...")
	chatStatus("/bs [bootstrap info]")
	chatStatus("    list id (no arg) or bootstrap to a peer id")
	chatStatus("/start <party_name>")
	chatStatus("    start a party (name limit 8 characters)")
	chatStatus("/invite <party_id> <user_id>")
	chatStatus("    invite a user to a party (partial ids ok)")
	chatStatus("/list")
	chatStatus("    list party ids")
	chatStatus("/send <party_id> msg")
	chatStatus("    send message to party (partial id ok)")
	chatStatus("/leave <party_id>")
	chatStatus("    leaves the party (partial id ok)")
	chatStatus("/show <all|mainline|party_id>")
	chatStatus("    change what messages are displayed (partial id ok)")
	chatStatus("/clear")
	chatStatus("    clear chat log")
	chatStatus("/ids <size>")
	chatStatus("    change id display size (hex)")
	chatStatus("/help")
	chatStatus("    display this")
	chatStatus("/quit")
	chatStatus("    we'll miss you, but it was fun while you were here")
	return
}

func handleUserInput(buf string) {
	if len(buf) == 0 {
		return
	}

	toks := strings.Split(buf, " ")
	switch toks[0] {
	case "/quit":
		sendDisconnect()
		termui.StopLoop()
	case "/bs":
		handleBootstrap(toks)
	case "/start":
		handleStart(toks)
	case "/invite":
		handleInvite(toks)
	case "/send":
		handleSend(toks)
	case "/leave":
		handleLeave(toks)
	case "/show":
		handleShow(toks)
	case "/clear":
		handleClear(toks)
	case "/ids":
		handleIds(toks)
	case "/help":
		handleHelp()
	case "/list":
		handleList()
	default:
		handleChat(buf)
	}
}

func userInterface() {
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
		handleUserInput(buf)
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
