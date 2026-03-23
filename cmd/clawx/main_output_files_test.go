package main

import (
	"os"
	"path/filepath"
	"strings"
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

func TestShouldDeliverOutputFiles(t *testing.T) {
	if !shouldDeliverOutputFiles("把代码文件发我") {
		t.Fatalf("expected true for explicit file request")
	}
	if shouldDeliverOutputFiles("只要简要说明和文件路径，不要贴代码") {
		t.Fatalf("expected false for concise request")
	}
}

func TestSuppressVerboseCodeBlocks(t *testing.T) {
	in := "已完成变更：\n```python\nprint('hello')\n```\n请查看路径。"
	out := suppressVerboseCodeBlocks("只要结果摘要", in)
	if out == in {
		t.Fatalf("expected code blocks suppressed")
	}
	if !strings.Contains(out, "代码内容已省略") {
		t.Fatalf("expected suppression marker, got: %s", out)
	}
	keep := suppressVerboseCodeBlocks("请贴代码", in)
	if keep != in {
		t.Fatalf("expected keep original when user asks for code")
	}
}
