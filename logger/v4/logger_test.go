package v4

import (
	"context"
	"encoding/json"
	"github.com/GiHccTpD/go-kit/known"
	"github.com/google/uuid"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	var ctx = context.WithValue(context.Background(), known.XRequestIDKey, uuid.New().String())
	var level = "info"

	Init(&Options{
		DisableCaller:     false,
		DisableStacktrace: false,
		Level:             level,
		Format:            "json",
		OutputPaths:       []string{"stdout", "app.log"},
	})

	std.Debugw("debug")

	std.C(ctx).Infow("update log level", "level", level)

	std.C(ctx).Debugw("不输出")
	std.C(ctx).SetLevel("debug")
	std.C(ctx).Debugw("输出")
}

func TestNewLoggerDoesNotWriteDuringConstruction(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取当前目录失败: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("切换临时目录失败: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	logFile := filepath.Join(dir, "app.log")

	logger := NewLogger(&Options{
		Level:       "info",
		Format:      "json",
		OutputPaths: []string{"app.log"},
	})
	logger.Sync()

	content, err := os.ReadFile(logFile)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("读取日志文件失败: %v", err)
	}
	if len(content) != 0 {
		t.Fatalf("创建 logger 时不应主动写日志:\n%s", content)
	}
}

func TestDisableCallerRemovesCallerField(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取当前目录失败: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("切换临时目录失败: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	logFile := filepath.Join(dir, "app.log")
	logger := NewLogger(&Options{
		DisableCaller: true,
		Level:         "info",
		Format:        "json",
		OutputPaths:   []string{"app.log"},
	})

	logger.Infow("hello")
	logger.Sync()

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("解析日志失败: %v\n%s", err, content)
	}
	if _, ok := entry["file"]; ok {
		t.Fatalf("DisableCaller 为 true 时不应输出 file 字段: %s", content)
	}
}

func TestImportDoesNotReportCallerError(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("获取当前测试文件路径失败")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module loggerimport\n\ngo 1.20\n\nrequire github.com/GiHccTpD/go-kit v0.0.0\n\nreplace github.com/GiHccTpD/go-kit => "+repoRoot+"\n"), 0600); err != nil {
		t.Fatalf("写入 go.mod 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport (\n\t\"log\"\n\t_ \"github.com/GiHccTpD/go-kit/logger/v4\"\n)\n\nfunc main() {\n\tlog.Print(\"hello\")\n}\n"), 0600); err != nil {
		t.Fatalf("写入 main.go 失败: %v", err)
	}

	cmd := exec.Command("go", "run", "-mod=mod", ".")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("运行导入测试程序失败: %v\n%s", err, output)
	}
	if strings.Contains(string(output), "Logger.check error: failed to get caller") {
		t.Fatalf("导入 logger/v4 时不应输出 caller 错误:\n%s", output)
	}
}
