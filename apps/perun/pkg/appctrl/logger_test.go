package appctrl

import (
	"bytes"
	"fmt"
	"testing"
)

type MockWriter struct {
	WrittenBytes []byte
}

func (w *MockWriter) Write(p []byte) (n int, err error) {
	w.WrittenBytes = append(w.WrittenBytes, p...)
	return len(p), nil
}

func TestAppLogger_Write(t *testing.T) {
	mockFileWriter := &MockWriter{}
	mockConsoleWriter := &MockWriter{}
	logger := NewAppLogger("TAG", mockFileWriter, mockConsoleWriter)

	msg := []byte("Test message\n")
	n, err := logger.Write(msg)
	if err != nil {
		t.Errorf("Write() returned error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write() returned incorrect byte count: expected %d, got %d", len(msg), n)
	}

	if !bytes.Equal(mockFileWriter.WrittenBytes, msg) {
		t.Errorf("Message not written to file writer correctly")
	}

	expectedConsoleOutput := append([]byte(fmt.Sprintf("\x1b[33m%-20s\x1b[0m ", "<TAG>")), msg...)
	if !bytes.Equal(mockConsoleWriter.WrittenBytes, expectedConsoleOutput) {
		t.Errorf("Message not written to console writer correctly. Expected:\n%s\nGot:\n%s", expectedConsoleOutput, mockConsoleWriter.WrittenBytes)
	}

	rcvdStartupMsg := <-logger.StartupLogMsgChan()
	if !bytes.Equal(rcvdStartupMsg, msg) {
		t.Errorf("Message not written to console writer correctly. Expected:\n%s\nGot:\n%s", expectedConsoleOutput, rcvdStartupMsg)
	}
}
