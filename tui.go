package main

import (
	"github.com/gizak/termui"
	"strings"
)

func handleUserInput(buf string) {
	if len(buf) == 0 {
		return
	}

	toks := strings.Split(buf, " ")
	switch toks[0] {
	case "/quit":
		termui.StopLoop()
	}
}

func userInterface() {
	err := termui.Init()
	if err != nil {
		panic(err)
	}
	defer termui.Close()

	messageBox := termui.NewPar("13:37:00   tacixat | sup jerks")
	messageBox.Height = termui.TermHeight() - 3
	messageBox.Y = 4
	messageBox.BorderLabel = "Party-Line"
	messageBox.BorderFg = termui.ColorYellow

	inputBox := termui.NewPar("/bs ...")
	inputBox.Height = 3
	inputBox.Y = 9

	termui.Body.AddRows(
		termui.NewRow(
			termui.NewCol(12, 0, messageBox)),
		termui.NewRow(
			termui.NewCol(12, 0, inputBox)))

	termui.Body.Align()

	termui.Render(termui.Body)

	buf := ""
	termui.Handle("/sys/kbd/<enter>", func(evt termui.Event) {
		handleUserInput(buf)
		buf = ""
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
