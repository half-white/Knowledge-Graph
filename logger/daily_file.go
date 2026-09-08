package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// dailyFileWriter 实现 io.Writer：按日期轮转的日志文件。
// 当天写入 <dir>/<prefix>-YYYYMMDD<suffix>，超过 SSE_LOG_KEEP(默认7) 天的
// 历史文件会被自动清理。所有方法并发安全。
type dailyFileWriter struct {
	mu      sync.Mutex
	dir     string
	prefix  string
	suffix  string
	keepDay int
	cur     *os.File
	curDate string
}

// newDailyFileWriter 创建按天轮转的 writer。
func newDailyFileWriter(dir, prefix, suffix string) *dailyFileWriter {
	keep, err := strconv.Atoi(env("SSE_LOG_KEEP", "7"))
	if err != nil || keep < 1 {
		keep = 7
	}
	return &dailyFileWriter{
		dir:     dir,
		prefix:  prefix,
		suffix:  suffix,
		keepDay: keep,
	}
}

func (w *dailyFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	date := now.Format("20060102")
	if w.cur == nil || w.curDate != date {
		if err := w.rotate(now); err != nil {
			return 0, err
		}
	}
	return w.cur.Write(p)
}

// rotate 切换到当天的新文件，并顺手清理过期文件。
func (w *dailyFileWriter) rotate(now time.Time) error {
	if w.cur != nil {
		_ = w.cur.Close()
		w.cur = nil
	}
	name := fmt.Sprintf("%s-%s%s", w.prefix, now.Format("20060102"), w.suffix)
	f, err := os.OpenFile(filepath.Join(w.dir, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.cur = f
	w.curDate = now.Format("20060102")
	w.cleanup(now)
	return nil
}

// cleanup 删除超过保留期的日志文件。
func (w *dailyFileWriter) cleanup(now time.Time) {
	cutoff := now.AddDate(0, 0, -w.keepDay)
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, w.prefix+"-") || !strings.HasSuffix(name, w.suffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(w.dir, name))
		}
	}
}

// Close 关闭当前日志文件。供优雅退出时调用。
func (w *dailyFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cur != nil {
		err := w.cur.Close()
		w.cur = nil
		return err
	}
	return nil
}
