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

	"go.uber.org/zap"
)

func TestErrorOnlyCallerRespectsLevelAndDisableCaller(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		for _, disable := range []bool{false, true} {
			t.Run(format, func(t *testing.T) {
				file := filepath.Join(t.TempDir(), "caller.log")
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				relative, err := filepath.Rel(cwd, file)
				if err != nil {
					t.Fatal(err)
				}
				logger := NewLogger(&Options{
					Level: "debug", Format: format, OutputPaths: []string{relative},
					DisableCaller: disable, DisableStacktrace: true,
				})
				logger.C(nil).Debugw("debug entry")
				logger.C(nil).Infow("info entry")
				logger.C(nil).Warnw("warn entry")
				logger.C(nil).Errorw("error entry")
				logger.Sync()
				body, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				lines := bytes.Split(bytes.TrimSpace(body), []byte("\n"))
				if len(lines) != 4 {
					t.Fatalf("got %d entries, want 4", len(lines))
				}
				for i, line := range lines {
					if hasCaller := bytes.Contains(line, []byte("caller_level_test.go:")); hasCaller != (i == 3 && !disable) {
						t.Fatalf("unexpected caller: %s", line)
					}
				}
			})
		}
	}
}

func TestNewOptionsDefaultsToErrorOnlyCaller(t *testing.T) {
	opts := NewOptions()
	if opts.DisableCaller {
		t.Fatal("默认应启用 caller")
	}
	if opts.CallerOnAllLevels {
		t.Fatal("默认应仅在 error 及以上级别输出 caller")
	}
	if opts.DisableStacktrace {
		t.Fatal("默认应在 error 及以上级别输出堆栈")
	}
}

func TestCallerOnAllLevelsOutputsCallerForNonError(t *testing.T) {
	dir := t.TempDir()
	chdirForLoggerTest(t, dir)
	logger := NewLogger(&Options{
		Level: "debug", Format: "json", OutputPaths: []string{"all-levels.log"},
		CallerOnAllLevels: true, DisableStacktrace: true,
	})
	logger.Debugw("debug entry")
	logger.Infow("info entry")
	logger.Warnw("warn entry")
	logger.Sync()

	body, err := os.ReadFile(filepath.Join(dir, "all-levels.log"))
	if err != nil {
		t.Fatalf("读取日志失败: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(body), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("got %d entries, want 3:\n%s", len(lines), body)
	}
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("解析日志失败: %v\n%s", err, line)
		}
		if _, ok := entry["file"]; !ok {
			t.Fatalf("%v 级别应输出 caller: %s", entry["level"], line)
		}
	}
}

func TestDefaultOptionsEmitCallerAndStackOnlyForErrors(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			chdirForLoggerTest(t, dir)
			logger := NewLogger(&Options{Level: "debug", Format: format, OutputPaths: []string{"levels.log"}})
			logger.Debugw("debug entry")
			logger.Infow("info entry")
			logger.Warnw("warn entry")
			logger.Errorw("error entry")
			logger.Sync()

			body, err := os.ReadFile(filepath.Join(dir, "levels.log"))
			if err != nil {
				t.Fatalf("读取日志失败: %v", err)
			}
			if format == "json" {
				assertJSONCallerAndStack(t, body)
				return
			}
			assertConsoleCallerAndStack(t, body)
		})
	}
}

func assertJSONCallerAndStack(t *testing.T, body []byte) {
	t.Helper()

	lines := bytes.Split(bytes.TrimSpace(body), []byte("\n"))
	if len(lines) != 4 {
		t.Fatalf("got %d entries, want 4:\n%s", len(lines), body)
	}
	for i, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("解析日志失败: %v\n%s", err, line)
		}
		_, hasCaller := entry["file"]
		_, hasStack := entry["stacktrace"]
		if i < 3 {
			if hasCaller || hasStack {
				t.Fatalf("%v 级别不应输出 caller/stack: %s", entry["level"], line)
			}
			continue
		}
		if !hasCaller {
			t.Fatalf("error 级别应输出 caller: %s", line)
		}
		if !hasStack {
			t.Fatalf("error 级别应输出 stacktrace: %s", line)
		}
	}
}

func assertConsoleCallerAndStack(t *testing.T, body []byte) {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) < 5 {
		t.Fatalf("error 级别应输出堆栈:\n%s", body)
	}
	for i := 0; i < 3; i++ {
		if strings.Contains(lines[i], "caller_level_test.go:") {
			t.Fatalf("非 error 级别不应输出 caller: %s", lines[i])
		}
	}
	if !strings.Contains(lines[3], "ERROR") || !strings.Contains(lines[3], "caller_level_test.go:") {
		t.Fatalf("error 级别应输出 caller: %s", lines[3])
	}
	stack := strings.Join(lines[4:], "\n")
	if !strings.Contains(stack, "logger/v4") {
		t.Fatalf("error 级别应输出堆栈:\n%s", body)
	}
}

func TestDisableCallerKeepsErrorStacktraceFreeOfInternalFrames(t *testing.T) {
	dir := t.TempDir()
	chdirForLoggerTest(t, dir)
	logger := NewLogger(&Options{
		Level: "debug", Format: "json", OutputPaths: []string{"disable-caller.log"}, DisableCaller: true,
	})
	logger.Errorw("wrapper error")
	logger.DB().Error("native error")
	logger.Sync()

	body, err := os.ReadFile(filepath.Join(dir, "disable-caller.log"))
	if err != nil {
		t.Fatalf("读取日志失败: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(body), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("got %d entries, want 2:\n%s", len(lines), body)
	}
	for _, line := range lines {
		var entry struct {
			Stacktrace string `json:"stacktrace"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("解析日志失败: %v\n%s", err, line)
		}
		firstFrame := strings.SplitN(entry.Stacktrace, "\n", 2)[0]
		if !strings.Contains(firstFrame, "TestDisableCallerKeepsErrorStacktraceFreeOfInternalFrames") {
			t.Fatalf("堆栈起始帧应指向调用方，实际为 %q", firstFrame)
		}
	}
}

func TestNativeZapCallsReportAccurateCaller(t *testing.T) {
	dir := t.TempDir()
	chdirForLoggerTest(t, dir)
	logger := NewLogger(&Options{
		Level: "debug", Format: "json", OutputPaths: []string{"native.log"}, DisableStacktrace: true,
	})
	t.Cleanup(logger.Sync)

	_, _, dbLine, _ := runtime.Caller(0)
	logger.DB().Error("native error")
	_, _, withLine, _ := runtime.Caller(0)
	logger.DB().With(zap.String("k", "v")).Error("native with error")
	logger.Sync()

	body, err := os.ReadFile(filepath.Join(dir, "native.log"))
	if err != nil {
		t.Fatalf("读取日志失败: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(body), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("got %d entries, want 2:\n%s", len(lines), body)
	}

	expected := []int{dbLine + 1, withLine + 1}
	for i, line := range lines {
		var entry struct {
			File    string `json:"file"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("解析日志失败: %v\n%s", err, line)
		}
		want := fmt.Sprintf("caller_level_test.go:%d", expected[i])
		if !strings.HasSuffix(entry.File, want) {
			t.Fatalf("%q 的 caller 为 %q，期望以 %q 结尾", entry.Message, entry.File, want)
		}
	}
}

func TestGetZapLoggerReportsAccurateCaller(t *testing.T) {
	dir := t.TempDir()
	chdirForLoggerTest(t, dir)
	oldStd := std
	t.Cleanup(func() { std = oldStd })

	Init(&Options{
		Level: "debug", Format: "json", OutputPaths: []string{"global-native.log"}, DisableStacktrace: true,
	})
	_, _, line, _ := runtime.Caller(0)
	GetZapLogger().Error("global native error")
	Sync()

	body, err := os.ReadFile(filepath.Join(dir, "global-native.log"))
	if err != nil {
		t.Fatalf("读取日志失败: %v", err)
	}
	var entry struct {
		File string `json:"file"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(body), &entry); err != nil {
		t.Fatalf("解析日志失败: %v\n%s", err, body)
	}
	want := fmt.Sprintf("caller_level_test.go:%d", line+1)
	if !strings.HasSuffix(entry.File, want) {
		t.Fatalf("GetZapLogger 的 caller 为 %q，期望以 %q 结尾", entry.File, want)
	}
}
