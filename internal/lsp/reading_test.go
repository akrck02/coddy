package lsp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHowLongAServerTookToReadIsKeptInItsDataFolder(test *testing.T) {
	data := test.TempDir()
	if took := LastReading(data); took != 0 {
		test.Errorf("a server never started took %v", took)
	}
	_ = os.WriteFile(filepath.Join(data, readingFile), []byte("72\n"), 0o644)
	if took := LastReading(data); took != 72*time.Second {
		test.Errorf("a server that took 72 s took %v", took)
	}
	_ = os.WriteFile(filepath.Join(data, readingFile), []byte("a while"), 0o644)
	if took := LastReading(data); took != 0 {
		test.Errorf("a file that says no time says %v", took)
	}
}
