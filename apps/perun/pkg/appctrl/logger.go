package appctrl

import (
	"fmt"
	"io"
	"sync"
)

const maxTagFormatLen = 20

type AppLogger struct {
	prefix           []byte
	fileWriter       io.Writer
	consoleWriter    io.Writer
	consoleWriterLoc sync.RWMutex

	ringBuf    [][]byte
	ringBufMax int
	ringBufLoc sync.Mutex

	startupContextLoc    sync.RWMutex
	startupChannelClosed bool
	startupLogMsgsChan   chan []byte // TODO: close on exit
	startupMsgReceived   bool
}

func NewAppLogger(tag string, fileWriter, consoleWriter io.Writer) *AppLogger {
	nameTag := fmt.Sprintf("<%s>", tag)
	return &AppLogger{
		prefix:             []byte(fmt.Sprintf("\x1b[33m%-20s\x1b[0m ", nameTag)),
		fileWriter:         fileWriter,
		consoleWriter:      consoleWriter,
		consoleWriterLoc:   sync.RWMutex{},
		ringBuf:            make([][]byte, 0, 1000),
		ringBufMax:         1000,
		startupLogMsgsChan: make(chan []byte, 100),
	}
}

func (l *AppLogger) StartupLogMsgChan() <-chan []byte {
	return l.startupLogMsgsChan
}

func (l *AppLogger) ResetForNewRun() {
	l.startupContextLoc.Lock()
	defer l.startupContextLoc.Unlock()
	if !l.startupChannelClosed {
		close(l.startupLogMsgsChan)
	}

	l.startupMsgReceived = false
	l.startupLogMsgsChan = make(chan []byte, 100)
	l.startupChannelClosed = false
}

func (l *AppLogger) StopSendingStartupLogs() {
	l.startupContextLoc.Lock()
	defer l.startupContextLoc.Unlock()

	l.startupMsgReceived = true
}

func (l *AppLogger) SetConsoleWriter(w io.Writer) {
	l.consoleWriterLoc.Lock()
	defer l.consoleWriterLoc.Unlock()

	l.consoleWriter = w
}

func (l *AppLogger) ConsoleWriter() io.Writer {
	l.consoleWriterLoc.RLock()
	defer l.consoleWriterLoc.RUnlock()

	return l.consoleWriter
}

func (l *AppLogger) RecentLines() [][]byte {
	l.ringBufLoc.Lock()
	defer l.ringBufLoc.Unlock()

	cp := make([][]byte, len(l.ringBuf))
	copy(cp, l.ringBuf)
	return cp
}

func (l *AppLogger) appendToRingBuf(line []byte) {
	cp := make([]byte, len(line))
	copy(cp, line)
	if len(l.ringBuf) >= l.ringBufMax {
		l.ringBuf = l.ringBuf[1:]
	}
	l.ringBuf = append(l.ringBuf, cp)
}

func (l *AppLogger) Write(p []byte) (n int, err error) {
	l.consoleWriterLoc.RLock()
	consoleWriter := l.consoleWriter
	l.consoleWriterLoc.RUnlock()

	newLineStart := 0
	for i := range p {
		if p[i] == '\n' {
			prefixed := append(l.prefix, p[newLineStart:i+1]...)
			l.ringBufLoc.Lock()
			l.appendToRingBuf(prefixed)
			l.ringBufLoc.Unlock()
			if consoleWriter != nil {
				n, err = consoleWriter.Write(prefixed)
				if err != nil {
					fmt.Println("Error writing:", err)
				}
			}
			newLineStart = i + 1
		}
	}
	if newLineStart < len(p) {
		prefixed := append(l.prefix, p[newLineStart:]...)
		l.ringBufLoc.Lock()
		l.appendToRingBuf(prefixed)
		l.ringBufLoc.Unlock()
		if consoleWriter != nil {
			n, err = consoleWriter.Write(prefixed)
			if err != nil {
				fmt.Println("Error writing:", err)
			}
		}
	}

	// Added two flags to minimise locking.
	if !l.startupChannelClosed {
		l.startupContextLoc.RLock()
		isAlreadyStarted := l.startupMsgReceived
		l.startupContextLoc.RUnlock()

		if isAlreadyStarted {
			close(l.startupLogMsgsChan)
			l.startupChannelClosed = true
		} else {
			l.startupLogMsgsChan <- p
		}
	}

	return l.fileWriter.Write(p)
}
