package v4

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GiHccTpD/go-kit/known"
	"go.uber.org/zap"
)

// TestGlobalAPIReportsCaller 覆盖包级函数（Errorw/Infow 等）。
// 包级函数比实例方法多一层调用，必须与实例方法一样指向真实调用点，
// 否则调用方会误以为是 caller 配置问题而自行叠加 AddCallerSkip，
// 进而在 goroutine 等浅栈入口触发 "Logger.check error: failed to get caller"。
func TestGlobalAPIReportsCaller(t *testing.T) {
	dir := t.TempDir()
	chdirForLoggerTest(t, dir)

	oldStd := std
	t.Cleanup(func() { std = oldStd })

	Init(&Options{
		Level: "debug", Format: "json", OutputPaths: []string{"global.log"},
		DisableStacktrace: true, CallerOnAllLevels: true,
	})

	ctx := context.WithValue(context.Background(), known.XRequestIDKey, "req-1")
	expected := make(map[string]int)

	_, _, line, _ := runtime.Caller(0)
	Errorw("pkg")
	expected["pkg"] = line + 1

	_, _, line, _ = runtime.Caller(0)
	Infow("pkg-info")
	expected["pkg-info"] = line + 1

	_, _, line, _ = runtime.Caller(0)
	With(zap.String("k", "v")).Errorw("pkg-with")
	expected["pkg-with"] = line + 1

	_, _, line, _ = runtime.Caller(0)
	C(ctx).Errorw("pkg-ctx")
	expected["pkg-ctx"] = line + 1

	lineCh := make(chan int, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, line, _ := runtime.Caller(0)
		Errorw("pkg-goroutine")
		lineCh <- line + 1
	}()
	expected["pkg-goroutine"] = <-lineCh
	<-done

	_, _, line, _ = runtime.Caller(0)
	GetZapLogger().Error("native")
	expected["native"] = line + 1

	Sync()

	entries := readCallerEntries(t, filepath.Join(dir, "global.log"))
	for msg, wantLine := range expected {
		file, ok := entries[msg]
		if !ok {
			t.Fatalf("未找到日志条目 %q", msg)
		}
		want := fmt.Sprintf("caller_global_test.go:%d", wantLine)
		if !strings.HasSuffix(file, want) {
			t.Fatalf("%q 的 caller 为 %q，期望以 %q 结尾", msg, file, want)
		}
		if strings.Contains(file, "runtime/") {
			t.Fatalf("%q 的 caller 退到了 runtime 帧: %q", msg, file)
		}
	}
}

// TestGlobalAPIDisableCallerOmitsCaller 确认关闭 caller 后不再解析调用栈，
// 日志条目中不出现 file 字段（也不会触发 caller 解析失败）。
func TestGlobalAPIDisableCallerOmitsCaller(t *testing.T) {
	dir := t.TempDir()
	chdirForLoggerTest(t, dir)

	oldStd := std
	t.Cleanup(func() { std = oldStd })

	Init(&Options{
		Level: "debug", Format: "json", OutputPaths: []string{"no-caller.log"},
		DisableStacktrace: true, DisableCaller: true,
	})

	Errorw("no-caller")
	Sync()

	body, err := os.ReadFile(filepath.Join(dir, "no-caller.log"))
	if err != nil {
		t.Fatalf("读取日志失败: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(body), &entry); err != nil {
		t.Fatalf("解析日志失败: %v\n%s", err, body)
	}
	if _, ok := entry["file"]; ok {
		t.Fatalf("DisableCaller 下不应输出 caller: %s", body)
	}
}

func readCallerEntries(t *testing.T, path string) map[string]string {
	t.Helper()

	body, err := os.ReadFile(path)
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

	return entries
}
