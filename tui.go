package main

import (
	"github.com/gizak/termui"
	"strings"
	"time"
	"log"
)

type Chat struct {
	Time    time.Time
	ID      string
	Message string
}

var chatLog []Chat

func formatChats() string {
	chatStr := ""
	for i := 0; i < len(chatLog); i++ {
		chat := chatLog[i]
		chatStr += chat.Time.Format("15:04:05 " + chat.ID[:6] + " " + chat.Message + "\n")
	}
	return chatStr
}

func handleBootstrap(toks []string) {
	if len(toks) < 2 {
		// TODO: show error
		log.Println("invalid bs")
		return
	}

	bs := toks[1]
	bsToks := strings.Split(bs, "/")
	if len(bsToks) != 2 {
		log.Println("invalid bs")
		return
	}

	addr := toks[0]
	id := toks[1]

	sendBootstrap(addr, id)
}

func handleChat(chatChan chan string, buf string) {
	chatMsg := Chat{
		Time:    time.Now(),
		ID:      self.ID,
		Message: buf}

	chatLog = append(chatLog, chatMsg)
	chats := formatChats()
	chatChan <- chats
}

func chatDrawer(chatChan chan string, messageBox *termui.Par) {
	for {
		chatsFormatted := <-chatChan
		messageBox.Text = chatsFormatted
		termui.Clear()
		termui.Render(termui.Body)
	}
}

func handleUserInput(chatChan chan string, buf string) {
	if len(buf) == 0 {
		return
	}

	toks := strings.Split(buf, " ")
	switch toks[0] {
	case "/quit":
		termui.StopLoop()
	case "/bs":
		handleBootstrap(toks)
	default:
		handleChat(chatChan, buf)
	}
}

func userInterface() {
	err := termui.Init()
	if err != nil {
		panic(err)
	}
	defer termui.Close()

	messageBox := termui.NewPar("")
	messageBox.Height = termui.TermHeight() - 3
	messageBox.Y = 4
	messageBox.BorderLabel = "Party-Line"
	messageBox.BorderFg = termui.ColorYellow

	inputBox := termui.NewPar("")
	inputBox.Height = 3
	inputBox.Y = 9

	termui.Body.AddRows(
		termui.NewRow(
			termui.NewCol(12, 0, messageBox)),
		termui.NewRow(
			termui.NewCol(12, 0, inputBox)))

	termui.Body.Align()

	termui.Render(termui.Body)

	chatChan := make(chan string, 1)
	go chatDrawer(chatChan, messageBox)

	buf := ""
	termui.Handle("/sys/kbd/<enter>", func(evt termui.Event) {
		handleUserInput(chatChan, buf)
		buf = ""
		inputBox.Text = buf
		termui.Clear()
		termui.Render(termui.Body)
	})

	termui.Handle("/sys/kbd/<backspace>", func(evt termui.Event) {
		if len(buf) > 0 {
			buf = buf[:len(buf)-1]
		}
		inputBox.Text = buf
		termui.Clear()
		termui.Render(termui.Body)
	})

	termui.Handle("/sys/kbd/C-8", func(evt termui.Event) {
		if len(buf) > 0 {
			buf = buf[:len(buf)-1]
		}
		inputBox.Text = buf
		termui.Clear()
		termui.Render(termui.Body)
	})

	termui.Handle("/sys/kbd/<space>", func(evt termui.Event) {
		buf += " "
		inputBox.Text = buf
		termui.Clear()
		termui.Render(termui.Body)
	})

	termui.Handle("/sys/kbd/", func(evt termui.Event) {
		buf += evt.Data.(termui.EvtKbd).KeyStr
		inputBox.Text = buf
		termui.Clear()
		termui.Render(termui.Body)
	})

	termui.Handle("/sys/wnd/resize", func(e termui.Event) {
		termui.Body.Width = termui.TermWidth()
		messageBox.Height = termui.TermHeight() - 3
		termui.Body.Align()
		termui.Clear()
		termui.Render(termui.Body)
	})

	termui.Loop()
}
