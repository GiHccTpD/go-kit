package v4

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestCallerResolvesInShallowStackPaths 覆盖 goroutine / time.AfterFunc 等浅栈入口，
// 防止 addCallerSkip 偏移过大导致 caller 退到 runtime 帧或触发
// "Logger.check error: failed to get caller"。
func TestCallerResolvesInShallowStackPaths(t *testing.T) {
	dir := t.TempDir()
	chdirForLoggerTest(t, dir)

	logger := NewLogger(&Options{
		Level: "debug", Format: "json", OutputPaths: []string{"shallow.log"},
		DisableStacktrace: true,
	})
	t.Cleanup(logger.Sync)

	expected := make(map[string]int)

	_, _, line, _ := runtime.Caller(0)
	logger.Errorw("method")
	expected["method"] = line + 1

	_, _, line, _ = runtime.Caller(0)
	logger.With(zap.String("k", "v")).Errorw("with")
	expected["with"] = line + 1

	_, _, line, _ = runtime.Caller(0)
	logger.DB().Error("native")
	expected["native"] = line + 1

	lineCh := make(chan int, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, line, _ := runtime.Caller(0)
		logger.Errorw("goroutine")
		lineCh <- line + 1
	}()
	expected["goroutine"] = <-lineCh
	<-done

	timerDone := make(chan struct{})
	time.AfterFunc(time.Millisecond, func() {
		defer close(timerDone)
		_, _, line, _ := runtime.Caller(0)
		logger.Errorw("timer")
		lineCh <- line + 1
	})
	expected["timer"] = <-lineCh
	<-timerDone

	logger.Sync()

	body, err := os.ReadFile(filepath.Join(dir, "shallow.log"))
	if err != nil {
		t.Fatalf("读取日志失败: %v", err)
	}

	entries := make(map[string]string)
	for _, line := range bytes.Split(bytes.TrimSpace(body), []byte("\n")) {
		var entry struct {
			File    string `json:"file"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("解析日志失败: %v\n%s", err, line)
		}
		entries[entry.Message] = entry.File
	}

	for msg, wantLine := range expected {
		file, ok := entries[msg]
		if !ok {
			t.Fatalf("未找到日志条目 %q", msg)
		}
		want := fmt.Sprintf("caller_shallow_test.go:%d", wantLine)
		if !strings.HasSuffix(file, want) {
			t.Fatalf("%q 的 caller 为 %q，期望以 %q 结尾", msg, file, want)
		}
		if strings.Contains(file, "runtime/") {
			t.Fatalf("%q 的 caller 退到了 runtime 帧: %q", msg, file)
		}
	}
}
