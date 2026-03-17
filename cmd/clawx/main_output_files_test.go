package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractOutputLocalFiles(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "out", "test-long.png")
	fileB := filepath.Join(dir, "out", "test-mini.png")
	if err := os.MkdirAll(filepath.Dir(fileA), 0o755); err != nil {
		t.Fatalf("mkdir fileA dir: %v", err)
	}
	if err := os.WriteFile(fileA, []byte("a"), 0o644); err != nil {
		t.Fatalf("write fileA: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("b"), 0o644); err != nil {
		t.Fatalf("write fileB: %v", err)
	}

	output := "[Agent Direct]\n已生成：[`test-long.png`](" + fileA + ")\n另一个产物: " + fileB
	paths := extractOutputLocalFiles(output, 3)
	if len(paths) != 2 {
		t.Fatalf("unexpected path count: %d paths=%v", len(paths), paths)
	}
	if paths[0] != fileA && paths[1] != fileA {
		t.Fatalf("expected fileA in extracted paths: %v", paths)
	}
	if paths[0] != fileB && paths[1] != fileB {
		t.Fatalf("expected fileB in extracted paths: %v", paths)
	}
}
