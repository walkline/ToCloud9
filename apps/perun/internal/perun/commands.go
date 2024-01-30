package perun

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/rivo/tview"
)

func (p *Perun) setupCommands() {
	p.commands = map[string]Command{
		"restart": &RestartCommand{},
		"clear":   &ClearCommand{},
		"stop":    &StopCommand{},
		"focus":   &FocusCommand{},
	}

	p.commandAlias = map[string]string{
		"restart": "restart",
		"start":   "restart",
		"re":      "restart",
		"r":       "restart",
		"clear":   "clear",
		"cl":      "clear",
		"stop":    "stop",
		"focus":   "focus",
		"fo":      "focus",
		"f":       "focus",
	}

	go func() {
		for s := range p.commandsInputChan {
			p.handleCommand(s)
		}
	}()
}

func (p *Perun) handleCommand(input string) {
	strs := strings.SplitN(input, " ", 2)
	commandName := p.commandAlias[strs[0]]
	if commandName == "" {
		log.Printf("Unknown command '%s'", strs[0])
		return
	}

	args := ""
	if len(strs) > 1 {
		args = strs[1]
	}

	err := p.commands[commandName].Handle(args, p)
	if err != nil {
		log.Printf("Can't handle command '%s', err: %s", strs[0], err)
		return
	}
}

type Command interface {
	Handle(args string, perun *Perun) error
}

type RestartCommand struct {
}

func (r *RestartCommand) Handle(args string, perun *Perun) error {
	apps, err := findAppsWithArgs(perun, args)
	if err != nil {
		return err
	}

	if len(apps) == 0 {
		return errors.New("need at least one app name")
	}

	log.Printf("⏳ Restarting %d apps...\n", len(apps))

	for _, app := range apps {
		log.Printf("🤞 Restarting '%s' app...\n", app.appName)
		if err = app.Stop(); err != nil {
			log.Printf("Failed to stop 'app', err: %s", err)
		}
		if err = app.Start(); err != nil {
			log.Printf("❌Couldn't start '%s' app, err: %s\n", app.appName, err)
		}
	}

	log.Println("✅ Restart finished.")

	return nil
}

type ClearCommand struct {
}

func (r *ClearCommand) Handle(args string, perun *Perun) error {
	perun.logsTextView.Clear()
	return nil
}

type StopCommand struct {
}

func (r *StopCommand) Handle(args string, perun *Perun) error {
	apps, err := findAppsWithArgs(perun, args)
	if err != nil {
		return err
	}

	if len(apps) == 0 {
		return errors.New("need at least one app name")
	}

	log.Printf("⏳ Stopping %d apps...\n", len(apps))

	for _, app := range apps {
		log.Printf("🛑 Stopping '%s' app...\n", app.appName)
		if err := app.Stop(); err != nil {
			log.Printf("❌Couldn't stop '%s' app, err: %s\n", app.appName, err)
		}
	}

	log.Println("✅ Stop operation finished.")

	return nil
}

type FocusCommand struct {
}

func (r *FocusCommand) Handle(args string, perun *Perun) error {
	var apps []*ExternalApp
	var err error
	if args == "all" {
		apps = perun.GetAllApps()
	} else {
		apps, err = findAppsWithArgs(perun, args)
		if err != nil {
			return err
		}
	}

	if len(apps) == 0 {
		return errors.New("need at least one app name")
	}

	allApps := perun.GetAllApps()
	for _, app := range allApps {
		app.SetConsoleOutput(io.Discard)
	}

	fullAppNames := make([]string, 0, len(apps))
	for _, app := range apps {
		fullAppNames = append(fullAppNames, app.appName)
	}

	perun.logsTextView.Clear()

	log.Printf("🎯 Showing output for '%s' apps.\n", strings.Join(fullAppNames, ", "))

	for _, app := range apps {
		app.SetConsoleOutput(tview.ANSIWriter(perun.logsTextView))
	}

	return nil
}

func findAppsWithArgs(p *Perun, args string) ([]*ExternalApp, error) {
	appNames := strings.Split(args, " ")
	apps := make([]*ExternalApp, 0, len(appNames))
	for _, name := range appNames {
		app := p.AppByName(name)
		if app == nil {
			return nil, fmt.Errorf("unknown app with name '%s'", name)
		}

		apps = append(apps, app)
	}

	return apps, nil
}
