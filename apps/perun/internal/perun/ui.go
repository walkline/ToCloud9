package perun

import (
	"fmt"
	"time"

	tcell "github.com/gdamore/tcell/v2"
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

	logsTextView.SetMaxLines(1000)
	logsTextView.SetWordWrap(false)
	logsTextView.ScrollToEnd()

	following := true

	logsTextView.SetBorder(true)
	logsTextView.SetTitle(" Logs - All [yellow]FOLLOWING[-] ")
	logsTextView.SetBorderColor(tcell.ColorGrey)

	appList := tview.NewList().
		ShowSecondaryText(false)
	appList.SetBorder(true)
	appList.SetTitle(" Filter Logs ")
	appList.SetBorderColor(tcell.ColorGrey)
	appList.SetHighlightFullLine(true)
	appList.SetSelectedBackgroundColor(tcell.ColorDarkCyan)
	appList.SetMainTextStyle(tcell.Style{}.Foreground(tcell.ColorWhite))

	helpBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	helpBar.SetText("[yellow]Ctrl+C[-] Quit  [yellow]Ctrl+K[-] Clear  [yellow]Ctrl+E[-] Follow  [yellow]Ctrl+F[-] Filter  [yellow]Tab[-] Switch Panel")

	updateLogsTitle := func(filter string) {
		source := "All"
		if filter != "" {
			source = filter
		}
		if following {
			logsTextView.SetTitle(fmt.Sprintf(" Logs - %s [yellow]FOLLOWING[-] ", source))
		} else {
			logsTextView.SetTitle(fmt.Sprintf(" Logs - %s [red]PAUSED[-] ", source))
		}
	}

	redrawPending := false
	var redrawTimer *time.Timer
	const redrawInterval = 300 * time.Millisecond

	scheduleRedraw := func() {
		if redrawPending {
			return
		}
		redrawPending = true
		if redrawTimer == nil {
			redrawTimer = time.AfterFunc(redrawInterval, func() {
				app.QueueUpdateDraw(func() {
					redrawPending = false
					updateLogsTitle(p.focusedApp)
				})
			})
		} else {
			redrawTimer.Reset(redrawInterval)
		}
	}

	logsTextView.SetChangedFunc(func() {
		if following {
			scheduleRedraw()
		}
	})

	pause := func() {
		if following {
			following = false
			row, col := logsTextView.GetScrollOffset()
			logsTextView.ScrollTo(row, col)
			p.muteAllApps()
			updateLogsTitle(p.focusedApp)
		}
	}

	resume := func() {
		following = true
		p.resumeApps()
		updateLogsTitle(p.focusedApp)
	}

	logsTextView.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseScrollUp || action == tview.MouseScrollDown {
			pause()
		}
		return action, event
	})

	inputField := tview.NewInputField().
		SetLabel("❯ ").
		SetLabelStyle(tcell.Style{}.Normal().Foreground(tcell.ColorYellowGreen).Bold(true)).
		SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	inputField.SetBackgroundColor(tcell.ColorDarkGrey)

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

	contentFlex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(appList, 24, 0, false).
		AddItem(logsTextView, 0, 1, false)

	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(contentFlex, 0, 1, false).
		AddItem(inputField, 1, 0, true).
		AddItem(helpBar, 1, 0, false)

	app.SetRoot(root, true).EnableMouse(true)

	sidebarVisible := true
	focusables := []tview.Primitive{inputField, appList, logsTextView}
	focusIndex := 0

	toggleSidebar := func() {
		if sidebarVisible {
			contentFlex.RemoveItem(appList)
			sidebarVisible = false
			focusables = []tview.Primitive{inputField, logsTextView}
			if focusIndex > len(focusables)-1 {
				focusIndex = 0
			}
			app.SetFocus(focusables[focusIndex])
		} else {
			contentFlex.Clear()
			contentFlex.AddItem(appList, 24, 0, false)
			contentFlex.AddItem(logsTextView, 0, 1, false)
			sidebarVisible = true
			focusables = []tview.Primitive{inputField, appList, logsTextView}
		}
	}

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			go p.Stop()
			return nil
		}

		if event.Key() == tcell.KeyCtrlK {
			logsTextView.Clear()
			return nil
		}

		if event.Key() == tcell.KeyCtrlE {
			resume()
			return nil
		}

		if event.Key() == tcell.KeyCtrlF {
			toggleSidebar()
			return nil
		}

		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab {
			if event.Key() == tcell.KeyTab {
				focusIndex = (focusIndex + 1) % len(focusables)
			} else {
				focusIndex = (focusIndex - 1 + len(focusables)) % len(focusables)
			}
			app.SetFocus(focusables[focusIndex])
			return nil
		}

		if app.GetFocus() == logsTextView {
			if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown ||
				event.Key() == tcell.KeyPgUp || event.Key() == tcell.KeyPgDn ||
				event.Key() == tcell.KeyHome || event.Key() == tcell.KeyEnd {
				pause()
			}
		}

		return event
	})

	p.tviewApp = app
	p.logsTextView = logsTextView
	p.appList = appList
	p.updateLogsTitle = updateLogsTitle
	p.setFollowing = func(f bool) {
		if f {
			resume()
		} else {
			pause()
		}
	}
}

func (p *Perun) setupSidebar() {
	allItem := "All"
	p.appList.AddItem(allItem, "", 0, func() {
		p.selectApp("")
	})

	for _, name := range p.runningOrder {
		appName := name
		p.appList.AddItem(appName, "", 0, func() {
			p.selectApp(appName)
		})
	}
}

func (p *Perun) refreshSidebarStatus() {
	for i, name := range p.runningOrder {
		app := p.apps[name]
		if app == nil {
			continue
		}
		listIdx := i + 1
		if app.isRunning {
			p.appList.SetItemText(listIdx, fmt.Sprintf("[green]●[-] %s", name), "")
		} else {
			p.appList.SetItemText(listIdx, fmt.Sprintf("[red]●[-] %s", name), "")
		}
	}
}

func (p *Perun) startSidebarRefresh() {
	ticker := time.NewTicker(time.Second)
	go func() {
		for range ticker.C {
			p.tviewApp.QueueUpdateDraw(func() {
				p.refreshSidebarStatus()
			})
		}
	}()
}
