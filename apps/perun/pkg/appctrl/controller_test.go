package appctrl

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TODO: add Win support
func TestAppController_StartStop(t *testing.T) {
	tmpDir := t.TempDir()

	scriptPath := filepath.Join(tmpDir, "test_script.sh")
	scriptContent := []byte(`#!/bin/bash
echo 'Hello, startup World!'
sleep 1
`)
	err := os.WriteFile(scriptPath, scriptContent, 0755)
	if err != nil {
		t.Fatalf("Failed to write to temporary file: %v", err)
	}

	fileLogsWriter := io.Discard
	consoleLogWriter := io.Discard
	app := NewAppController("test", fileLogsWriter, consoleLogWriter, "startup", scriptPath)
	app.SetStartupTimeoutDuration(time.Second)
	startingTime := time.Now()
	assert.Nil(t, app.Start())

	// We're instantly sending startup message so Start() should return in less than 1 sec.
	assert.Less(t, time.Since(startingTime), time.Second)

	assert.Nil(t, app.Stop())

	// App Controller tries to gracefully stop script, but script ignores that.
	// So we should wait until script finishes.
	assert.Less(t, time.Since(startingTime), time.Second*2)
	assert.Greater(t, time.Since(startingTime), time.Second)
}

// TODO: add Win support
func TestAppController_Restart(t *testing.T) {
	tmpDir := t.TempDir()

	scriptPath := filepath.Join(tmpDir, "test_script.sh")
	scriptContent := []byte(`#!/bin/bash
echo 'Hello, startup World!'
sleep 1
`)
	err := os.WriteFile(scriptPath, scriptContent, 0755)
	if err != nil {
		t.Fatalf("Failed to write to temporary file: %v", err)
	}

	fileLogsWriter := io.Discard
	consoleLogWriter := io.Discard
	app := NewAppController("test", fileLogsWriter, consoleLogWriter, "startup", scriptPath)
	app.SetStartupTimeoutDuration(time.Second * 1)
	assert.Nil(t, app.Start())

	oldPid := app.cmd.Process.Pid
	// We need to wait for >1 sec until the script finishes execution and a new process starts.
	time.Sleep(time.Second + time.Millisecond*100)

	assert.NotEqual(t, oldPid, app.cmd.Process.Pid)
	_ = app.Stop()
}

func TestMultiErrChannel(t *testing.T) {
	source := make(chan error)
	multiChan := NewMultiChannel(source)

	ch, err := multiChan.GetChannel()
	if err != nil {
		t.Errorf("GetChannel() returned error: %v", err)
	}
	if ch == nil {
		t.Error("GetChannel() returned nil channel")
	}

	expectedErr := errors.New("test error")
	source <- expectedErr
	receivedErr := <-ch
	if receivedErr != expectedErr {
		t.Errorf("Received error %v, expected %v", receivedErr, expectedErr)
	}

	close(source)
	_, err = multiChan.GetChannel()
	if err == nil {
		t.Error("GetChannel() on closed source channel should return an error")
	}
}
