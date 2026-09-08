package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInitWritesFileAndConsole(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SSE_LOG_DIR", dir)
	t.Setenv("SSE_LOG_KEEP", "1")

	Init()
	slog.Info("hello", "key", "value")
	slog.Error("boom", "err", "x")

	// 控制台与文件都输出后，检查 JSON 文件已生成
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no log file created")
	}
	name := entries[0].Name()
	if filepath.Ext(name) != ".jsonl" {
		t.Fatalf("unexpected file name: %s", name)
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("log file empty")
	}
}

func TestRotateCreatesNewFilePerDay(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SSE_LOG_DIR", dir)

	w := newDailyFileWriter(dir, "sse", ".jsonl")
	_ = w.rotate(time.Now())

	// 模拟第二天：强制切换文件
	_ = w.rotate(time.Now().AddDate(0, 0, 1))

	files, _ := filepath.Glob(filepath.Join(dir, "sse-*.jsonl"))
	if len(files) < 2 {
		t.Fatalf("expected >=2 files after rotation, got %d", len(files))
	}
	_ = w.Close()
}

func TestConsoleDisabled(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SSE_LOG_DIR", dir)
	t.Setenv("SSE_LOG_CONSOLE", "false")

	Init() // 不应 panic
	slog.Info("only-file")
}
