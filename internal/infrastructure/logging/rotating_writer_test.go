package logging

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotatingWriterRotatesBySize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")
	writer, err := NewRotatingWriter(path, 16, 2)
	if err != nil {
		t.Fatalf("new rotating writer: %v", err)
	}
	defer writer.Close()

	if _, err := writer.Write([]byte("1234567890\n")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := writer.Write([]byte("abcdefghij\n")); err != nil {
		t.Fatalf("second write: %v", err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected rotated backup file: %v", err)
	}
}
