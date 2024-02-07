package appctrl

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

type AppController struct {
	binaryPath             string
	args                   []string
	envs                   map[string]string
	workingDir             string
	name                   string
	autoRestart            bool
	partOfStartupMsg       string
	startupTimeoutDuration time.Duration

	triggerStopChan chan struct{}

	consoleWriter io.Writer

	appLogger *AppLogger

	cmd          *exec.Cmd
	runMultiChan *MultiErrChannel
}

func NewAppController(name string, fileLogsWriter, consoleLogWriter io.Writer, startupMsg, binaryPath string, args ...string) *AppController {
	return &AppController{
		binaryPath:       binaryPath,
		name:             name,
		args:             args,
		autoRestart:      true,
		partOfStartupMsg: startupMsg,
		appLogger:        NewAppLogger(name, fileLogsWriter, consoleLogWriter),
		consoleWriter:    consoleLogWriter,
	}
}

func (a *AppController) SetEnv(env map[string]string) {
	a.envs = env
}

func (a *AppController) SetWorkingDir(dir string) {
	a.workingDir = dir
}

func (a *AppController) SetStartupTimeoutDuration(d time.Duration) {
	a.startupTimeoutDuration = d
}

func (a *AppController) SetConsoleOutput(w io.Writer) {
	a.appLogger.SetConsoleWriter(w)
}

func (a *AppController) Start() error {
	if a.cmd != nil {
		a.cmd.Stderr = nil
		a.cmd.Stdout = nil
	}

	a.appLogger.ResetForNewRun()

	a.cmd = exec.Command(a.binaryPath)
	if a.workingDir != "" {
		a.cmd.Dir = a.workingDir
	}

	if len(a.envs) > 0 {
		mergedEnvs := mergeTwoEnvs(a.envs, envSliceStringToEnvMap(os.Environ()))
		a.cmd.Env = envMapToEnvSliceString(mergedEnvs)
	}

	a.cmd.Stdout = a.appLogger
	a.cmd.Stderr = a.appLogger

	if err := a.cmd.Start(); err != nil {
		return err
	}

	runChan := make(chan error, 1)
	a.runMultiChan = NewMultiChannel(runChan)

	go func() {
		runChan <- a.cmd.Wait()
		close(runChan)
	}()

	runErrChan, err := a.runMultiChan.GetChannel()
	if err != nil {
		return err
	}

	startupTimeout := time.NewTimer(a.startupTimeoutDuration)
	for {
		select {
		case err = <-runErrChan:
			return err
		case <-startupTimeout.C:
			_ = a.cmd.Process.Kill()
			return fmt.Errorf("startup timeouted")
		case msg := <-a.appLogger.startupLogMsgsChan:
			if strings.Contains(strings.ToLower(string(msg)), a.partOfStartupMsg) {
				a.appLogger.StopSendingStartupLogs()
				a.triggerStopChan = make(chan struct{}, 1)
				a.startRestarter(a.triggerStopChan)
				return nil
			}
		}
	}
}

func (a *AppController) Stop() error {
	if a.cmd.Process == nil {
		return nil
	}

	if a.triggerStopChan != nil {
		a.triggerStopChan <- struct{}{}
		close(a.triggerStopChan)
		a.triggerStopChan = nil
	}

	runCh, err := a.runMultiChan.GetChannel()
	if err != nil {
		return err
	}

	err = a.cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		_ = a.cmd.Process.Kill()
		return fmt.Errorf("failed to gracefully stop, err: %w", err)
	}

	const shutdownTimeoutDur = time.Second * 10
	shutdownTimeout := time.NewTimer(shutdownTimeoutDur)
	select {
	case err = <-runCh:
		if errors.Is(err, errors.New(syscall.SIGTERM.String())) {
			return nil
		}
		return err
	case <-shutdownTimeout.C:
		_ = a.cmd.Process.Kill()
		return fmt.Errorf("shutdown timeouted")
	}
}

func (a *AppController) startRestarter(cancel <-chan struct{}) {
	go func() {
		ch, err := a.runMultiChan.GetChannel()
		if err != nil {
			log.Printf("can't start restarting process, err: %s\n", err)
			return
		}
		select {
		case <-ch:
			log.Printf("💥 App '%s' stopped, restarting...\n", a.name)
			for {
				err = a.Start()
				if err == nil {
					return
				}

				select {
				case <-cancel:
					return
				default:
				}

				log.Printf("💥 App restart '%s' failed (err: %s), retrying...\n", a.name, err)
			}
		case <-cancel:
		}
	}()
}

type MultiErrChannel struct {
	source   <-chan error
	channels []chan error

	newChanReq  chan struct{}
	newChanResp chan chan error

	closeLock sync.Mutex
	closed    bool
}

func NewMultiChannel(source <-chan error, channels ...chan error) *MultiErrChannel {
	mc := &MultiErrChannel{
		source:      source,
		channels:    channels,
		newChanReq:  make(chan struct{}, 1),
		newChanResp: make(chan chan error, 1),
	}
	go mc.eventLoop()
	return mc
}

func (mc *MultiErrChannel) eventLoop() {
	for {
		select {
		case e, open := <-mc.source:
			if !open {
				for i := range mc.channels {
					close(mc.channels[i])
				}
				close(mc.newChanResp)
				mc.closed = true
				return
			}

			for i := range mc.channels {
				mc.channels[i] <- e
			}
		case <-mc.newChanReq:
			newChan := make(chan error, 1)
			mc.channels = append(mc.channels, newChan)
			mc.newChanResp <- newChan
		}
	}
}

func (mc *MultiErrChannel) GetChannel() (<-chan error, error) {
	var channelClosedErr = errors.New("source channel already closed")
	select {
	case _, open := <-mc.newChanResp:
		if !open {
			close(mc.newChanReq)
			return nil, channelClosedErr
		}
	default:
	}

	mc.newChanReq <- struct{}{}

	newChan, open := <-mc.newChanResp
	if !open {
		close(mc.newChanReq)
		return nil, channelClosedErr
	}
	return newChan, nil
}
