package perun

import (
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/walkline/ToCloud9/apps/perun/internal/config"
	"github.com/walkline/ToCloud9/apps/perun/pkg/appctrl"
)

type Perun struct {
	apps  map[string]*ExternalApp
	alias map[string]string

	runningOrder []string

	commands     map[string]Command
	commandAlias map[string]string

	commandsInputChan chan string

	tviewApp        *tview.Application
	logsTextView    *tview.TextView
	appList         *tview.List
	focusedApp      string
	updateLogsTitle func(filter string)
	setFollowing    func(bool)
}

func NewWithApps(apps []config.AppConfig) (*Perun, error) {
	runningOrder := make([]string, 0, len(apps))
	alias := map[string]string{}
	for _, app := range apps {
		runningOrder = append(runningOrder, app.Name)
		alias[app.Name] = app.Name
		for _, a := range app.Alias {
			alias[a] = app.Name
		}
	}

	p := &Perun{
		alias:             alias,
		runningOrder:      runningOrder,
		commandsInputChan: make(chan string, 1),
	}

	p.setupUI()
	p.setupCommands()

	appsMap := map[string]*ExternalApp{}
	for _, app := range apps {
		externalApp, err := NewExternalAppFromConfig(app, tview.ANSIWriter(p.logsTextView))
		if err != nil {
			return nil, err
		}
		appsMap[app.Name] = externalApp
	}

	p.apps = appsMap

	log.SetOutput(appctrl.NewAppLogger("perun", io.Discard, tview.ANSIWriter(p.logsTextView)))

	return p, nil
}

func (p *Perun) muteAllApps() {
	for _, app := range p.apps {
		app.SetConsoleOutput(io.Discard)
	}
}

func (p *Perun) resumeApps() {
	writer := tview.ANSIWriter(p.logsTextView)
	p.logsTextView.Clear()

	if p.focusedApp == "" {
		for _, appName := range p.runningOrder {
			app := p.apps[appName]
			if app == nil {
				continue
			}
			for _, line := range app.RecentLines() {
				_, _ = writer.Write(line)
			}
			app.SetConsoleOutput(writer)
		}
	} else {
		if a := p.AppByName(p.focusedApp); a != nil {
			for _, line := range a.RecentLines() {
				_, _ = writer.Write(line)
			}
			a.SetConsoleOutput(writer)
		}
	}

	p.logsTextView.ScrollToEnd()
}

func (p *Perun) selectApp(name string) {
	p.focusedApp = name

	allApps := p.GetAllApps()
	for _, app := range allApps {
		app.SetConsoleOutput(io.Discard)
	}

	p.logsTextView.Clear()

	writer := tview.ANSIWriter(p.logsTextView)
	if name == "" {
		for _, appName := range p.runningOrder {
			app := p.apps[appName]
			if app == nil {
				continue
			}
			for _, line := range app.RecentLines() {
				_, _ = writer.Write(line)
			}
			app.SetConsoleOutput(writer)
		}
	} else {
		if a := p.AppByName(name); a != nil {
			for _, line := range a.RecentLines() {
				_, _ = writer.Write(line)
			}
			a.SetConsoleOutput(writer)
		}
	}

	p.logsTextView.ScrollToEnd()
	p.setFollowing(true)
}

func (p *Perun) Run() error {
	p.setupSidebar()
	p.refreshSidebarStatus()
	p.startSidebarRefresh()

	go func() {
		for _, s := range p.runningOrder {
			log.Printf("Starting %s app... \n", s)
			err := p.apps[s].Start()
			if err != nil {
				log.Println("Failed to start with err:", err)
				time.Sleep(time.Second * 3)
				p.Stop()
			}
		}
		log.Println("All apps are running 👍")
	}()

	return p.tviewApp.Run()
}

func (p *Perun) Stop() error {
	log.Println("Stopping apps...")

	for i := len(p.runningOrder) - 1; i >= 0; i-- {
		log.Printf("Stopping %s...\n", p.runningOrder[i])
		err := p.apps[p.runningOrder[i]].Stop()
		if err != nil {
			log.Printf("Failed to stop app '%s', err: %s\n", p.runningOrder[i], err)
		}
	}

	p.tviewApp.Stop()

	close(p.commandsInputChan)
	return nil
}

func (p *Perun) AppByName(name string) *ExternalApp {
	realAppName := p.alias[name]
	if realAppName == "" {
		return nil
	}

	return p.apps[realAppName]
}

func (p *Perun) GetAllApps() []*ExternalApp {
	apps := make([]*ExternalApp, 0, len(p.apps))
	for _, app := range p.apps {
		apps = append(apps, app)
	}

	return apps
}

func createLogFile(appName string) (*os.File, error) {
	file, err := os.OpenFile(normalizeFileName(appName)+".log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func normalizeFileName(fileName string) string {
	// Remove special characters using regular expression
	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		// If regex compilation fails, return original string
		return fileName
	}
	normalized := reg.ReplaceAllString(fileName, "")

	// Convert to lowercase
	normalized = strings.ToLower(normalized)

	// Replace spaces with underscores
	normalized = strings.ReplaceAll(normalized, " ", "_")

	return normalized
}
