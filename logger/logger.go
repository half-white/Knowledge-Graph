// Package logger 提供基于标准库 log/slog 的结构化日志基础设施。
//
// 输出设计：
//   - 控制台：slog TextHandler，开发时便于阅读（可用 SSE_LOG_CONSOLE=false 关闭）；
//   - 落盘：slog JSONHandler，逐行 JSON 追加到 <项目根>/logs/sse-YYYYMMDD.jsonl，
//     按天自动轮转并清理过期文件，文件可被 Promtail 采集后送入 Loki/Grafana。
//
// 环境变量配置：
//
//	SSE_LOG_LEVEL   日志级别 debug|info|warn|error，默认 info
//	SSE_LOG_CONSOLE 是否输出控制台(true/false)，默认 true
//	SSE_LOG_FILE    是否写 JSON 文件(true/false)，默认 true
//	SSE_LOG_DIR     日志目录（默认项目根/logs），可通过该变量覆盖
//	SSE_LOG_KEEP    文件保留天数，默认 7
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// env 读取环境变量，未设置时返回默认值
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envBool 读取布尔型环境变量，未设置时返回默认值
func envBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	}
	return def
}

// levelFromEnv 解析日志级别配置
func levelFromEnv() slog.Level {
	switch strings.ToLower(env("SSE_LOG_LEVEL", "info")) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// findProjectRoot 从当前工作目录向上逐级查找含 go.mod 的目录（即项目根）。
// 找不到时返回空串，由调用方回退到默认目录。
func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// logDir 计算日志目录：优先使用 SSE_LOG_DIR，否则用 <项目根>/logs
func logDir() string {
	if dir := os.Getenv("SSE_LOG_DIR"); dir != "" {
		return dir
	}
	root := findProjectRoot()
	if root == "" {
		return "logs"
	}
	return filepath.Join(root, "logs")
}

// Init 初始化全局 slog logger 并设为默认。
// 调用方（main）应在程序最早期调用；之后代码直接使用 slog.Info/Warn/Error 等。
func Init() *slog.Logger {
	level := levelFromEnv()
	opts := &slog.HandlerOptions{Level: level}

	var handlers []slog.Handler

	if envBool("SSE_LOG_CONSOLE", true) {
		// 控制台用文本编码，便于开发时阅读
		handlers = append(handlers, slog.NewTextHandler(os.Stdout, opts))
	}
	if envBool("SSE_LOG_FILE", true) {
		dir := logDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "[logger] 创建日志目录失败: %v\n", err)
		}
		// 文件用 JSON 编码（每行一条），便于 Promtail/Loki 解析
		handlers = append(handlers, slog.NewJSONHandler(newDailyFileWriter(dir, "sse", ".jsonl"), opts))
	}

	var handler slog.Handler
	switch len(handlers) {
	case 0:
		handler = slog.NewTextHandler(io.Discard, opts)
	case 1:
		handler = handlers[0]
	default:
		handler = newMultiHandler(handlers...)
	}

	l := slog.New(handler)
	slog.SetDefault(l)
	return l
}
