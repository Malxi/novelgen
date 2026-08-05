package logger

import (
	"bytes"
	"os"
	"testing"
)

func TestEnableFileLoggingWritesUTF8BOM(t *testing.T) {
	log := New(InfoLevel)
	log.SetProjectDir(t.TempDir())
	if err := log.EnableFileLogging(); err != nil {
		t.Fatalf("EnableFileLogging() error = %v", err)
	}
	defer log.Close()

	path := log.LogFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatalf("log file does not start with UTF-8 BOM: % x", data[:min(3, len(data))])
	}
}
