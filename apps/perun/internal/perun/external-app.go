package perun

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/walkline/ToCloud9/apps/perun/internal/config"
	"github.com/walkline/ToCloud9/apps/perun/pkg/appctrl"
)

type ExternalApp struct {
	appName     string
	alias       []string
	isRunning   bool
	loggingFile *os.File
	appCtrl     *appctrl.AppController
}

func NewExternalAppFromConfig(appConf config.AppConfig, consoleWriter io.Writer) (*ExternalApp, error) {
	file, err := createLogFile(appConf.Name)
	if err != nil {
		return nil, fmt.Errorf("can't create logging file for app: %s, err: %w", appConf.Name, err)
	}

	startupMsg := "started"
	if appConf.PartOfAppStartedLogMsg != "" {
		startupMsg = appConf.PartOfAppStartedLogMsg
	}

	appControl := appctrl.NewAppController(appConf.Name, file, consoleWriter, startupMsg, appConf.Binary, appConf.Args...)
	appControl.SetEnv(appConf.Env)
	appControl.SetWorkingDir(appConf.WorkingDir)
	if appConf.StartupTimeoutSecs > 0 {
		appControl.SetStartupTimeoutDuration(time.Duration(appConf.StartupTimeoutSecs) * time.Second)
	}
	return &ExternalApp{
		appName:     appConf.Name,
		alias:       appConf.Alias,
		loggingFile: file,
		appCtrl:     appControl,
	}, nil
}

func (a *ExternalApp) Start() error {
	if a.isRunning {
		return nil
	}

	err := a.appCtrl.Start()
	if err != nil {
		return err
	}

	a.isRunning = true

	return nil
}

func (a *ExternalApp) Stop() error {
	if !a.isRunning {
		return nil
	}

	a.isRunning = false
	err := a.appCtrl.Stop()
	if err != nil {
		return err
	}

	return nil
}

func (a *ExternalApp) SetConsoleOutput(w io.Writer) {
	a.appCtrl.SetConsoleOutput(w)
}
