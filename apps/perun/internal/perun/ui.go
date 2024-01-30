package perun

import (
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (p *Perun) setupUI() {
	theme := tview.Theme{
		PrimitiveBackgroundColor:    tcell.ColorDefault,
		ContrastBackgroundColor:     tcell.ColorDefault,
		MoreContrastBackgroundColor: tcell.ColorDefault,
		BorderColor:                 tcell.ColorDefault,
		TitleColor:                  tcell.ColorDefault,
		GraphicsColor:               tcell.ColorDefault,
		PrimaryTextColor:            tcell.ColorDefault,
		SecondaryTextColor:          tcell.ColorDefault,
		TertiaryTextColor:           tcell.ColorDefault,
		InverseTextColor:            tcell.ColorDefault,
		ContrastSecondaryTextColor:  tcell.ColorDefault,
	}
	tview.Styles = theme

	app := tview.NewApplication()

	logsTextView := tview.NewTextView().
		SetScrollable(true).
		SetText("").
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true)

	logsTextView.SetMaxLines(100)
	logsTextView.SetWordWrap(true)

	drawBarrier := NewRedrawBarrier(time.Millisecond*150, func() {
		app.Draw()
		logsTextView.ScrollToEnd()
	})
	logsTextView.SetChangedFunc(func() {
		drawBarrier.TryDraw()
	})

	// InputField at the bottom
	inputField := tview.NewInputField().
		SetLabel("❯ ").
		SetLabelStyle(tcell.Style{}.Normal().Foreground(tcell.ColorYellowGreen).Bold(true)).
		SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	inputField.SetBackgroundColor(tcell.ColorDarkGrey)

	// Handle user input
	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			inputText := inputField.GetText()
			if inputText == "" {
				return
			}

			inputField.SetText("")
			p.commandsInputChan <- inputText
		}
	})

	logsViewFlex := tview.NewFlex()
	logsViewFlex.AddItem(logsTextView, 0, 1, false)
	logsViewFlex.SetBorder(true)
	logsViewFlex.SetBorderColor(tcell.ColorWhite)
	logsViewFlex.SetBorderPadding(1, 1, 0, 0)
	logsViewFlex.SetDirection(tview.FlexRow)

	inputFlex := tview.NewFlex()
	inputFlex.AddItem(inputField, 1, 1, true)
	inputFlex.SetBackgroundColor(tcell.ColorWhite)
	inputFlex.SetBorderColor(tcell.ColorWhite)
	inputFlex.SetDirection(tview.FlexRow)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(logsTextView, 0, 8, false).
		AddItem(inputFlex, 1, 0, true)

	app.SetRoot(flex, true).EnableMouse(true)
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			go p.Stop()
			return nil
		}

		if event.Key() == tcell.KeyCtrlK {
			logsTextView.Clear()
			return nil
		}

		return event
	})

	p.tviewApp = app
	p.logsTextView = logsTextView
}

type RedrawBarrier struct {
	lastRedrawTime time.Time
	updateQueued   atomic.Bool
	barrierTTL     time.Duration
	drawFunc       func()
}

func NewRedrawBarrier(barrierTTL time.Duration, drawFunc func()) RedrawBarrier {
	return RedrawBarrier{
		drawFunc:   drawFunc,
		barrierTTL: barrierTTL,
	}
}

func (b *RedrawBarrier) TryDraw() bool {
	past := time.Since(b.lastRedrawTime)
	if past < b.barrierTTL {
		queued := b.updateQueued.Load()
		if queued {
			return false
		}

		b.lastRedrawTime = time.Now().Add(b.barrierTTL - past)
		b.updateQueued.Store(true)
		go func() {
			time.Sleep(b.barrierTTL - past)
			b.drawFunc()
			b.updateQueued.Store(false)
		}()

		return false
	}

	queued := b.updateQueued.Load()
	if queued {
		return false
	}

	b.drawFunc()
	b.lastRedrawTime = time.Now()
	return true
}
