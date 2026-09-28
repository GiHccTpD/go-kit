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

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestLoggerRedactsSensitiveFieldsAcrossZapEntryPoints(t *testing.T) {
	dir := t.TempDir()
	chdirForLoggerTest(t, dir)
	logger := NewLogger(&Options{
		Level:       "debug",
		Format:      "json",
		OutputPaths: []string{"redaction.log"},
	})
	t.Cleanup(logger.Sync)

	type credentials struct {
		Password string `json:"oldPassword"`
		Token    string `json:"access_token"`
	}
	logger.Infow("instance",
		"password", "top-secret-password",
		"token", "visible-token",
		"payload", map[string]any{
			"credentials": credentials{Password: "nested-password", Token: "nested-visible-token"},
			"items":       []any{map[string]string{"db_password": "array-password"}},
		},
	)

	ctx := context.WithValue(context.Background(), known.XRequestIDKey, "request-1")
	logger.C(ctx).Infow("context", "confirmPassword", "context-password", "token", "context-visible-token")
	logger.DB().With(zap.String("login_pwd", "with-password")).Info("with field", zap.String("session_token", "with-visible-token"))
	logger.DB().Info("native", zap.String("pwd", "native-password"), zap.String("token", "native-visible-token"))
	logger.DB().Info("marshalers", zap.Object("object", testRedactionObject{}), zap.Array("array", testRedactionArray{}))

	logger.Sync()
	content, err := os.ReadFile(filepath.Join(dir, "redaction.log"))
	if err != nil {
		t.Fatalf("读取日志失败: %v", err)
	}
	output := string(content)
	for _, secret := range []string{"top-secret-password", "nested-password", "array-password", "context-password", "with-password", "native-password", "object-password", "array-marshaler-password", "reflected-array-password"} {
		if strings.Contains(output, secret) {
			t.Errorf("日志泄露了密码 %q:\n%s", secret, output)
		}
	}
	for _, visible := range []string{"visible-token", "nested-visible-token", "context-visible-token", "with-visible-token", "native-visible-token"} {
		if !strings.Contains(output, visible) {
			t.Errorf("日志应保留 token %q:\n%s", visible, output)
		}
	}
	if strings.Count(output, "[REDACTED]") < 9 {
		t.Errorf("密码字段应替换为 [REDACTED]:\n%s", output)
	}
}

func TestRedactKeysAreCopiedAndApplyToConsoleAndGlobalLogger(t *testing.T) {
	dir := t.TempDir()
	chdirForLoggerTest(t, dir)
	redactKeys := []string{"client_secret"}
	logger := NewLogger(&Options{Level: "info", Format: "console", OutputPaths: []string{"console.log"}, RedactKeys: redactKeys})
	redactKeys[0] = "changed"
	logger.Infow("custom", "clientSecret", "custom-secret", "token", "visible-token")
	logger.Sync()
	console, err := os.ReadFile(filepath.Join(dir, "console.log"))
	if err != nil {
		t.Fatalf("读取 console 日志失败: %v", err)
	}
	if strings.Contains(string(console), "custom-secret") || !strings.Contains(string(console), "[REDACTED]") || !strings.Contains(string(console), "visible-token") {
		t.Fatalf("console 日志的脱敏结果不正确: %s", console)
	}

	globalPath := filepath.Join(dir, "global.log")
	oldStd := std
	t.Cleanup(func() { std = oldStd })
	Init(&Options{Level: "info", Format: "json", OutputPaths: []string{"global.log"}})
	Infow("global", "password", "global-password", "token", "global-visible-token")
	GetZapLogger().Info("global native", zap.String("pwd", "global-native-password"))
	Sync()
	global, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("读取全局日志失败: %v", err)
	}
	if strings.Contains(string(global), "global-password") || strings.Contains(string(global), "global-native-password") || !strings.Contains(string(global), "global-visible-token") {
		t.Fatalf("全局 logger 的脱敏结果不正确: %s", global)
	}
}

func TestRedactionMatchesPasswordKeySegmentsAndPreservesUnstructuredText(t *testing.T) {
	dir := t.TempDir()
	chdirForLoggerTest(t, dir)
	logger := NewLogger(&Options{Level: "info", Format: "json", OutputPaths: []string{"segments.log"}})
	logger.Infow("free-text password=free-text-secret", "PASSWORD", "upper-secret", "db-password-hash", "compound-secret", "pwdHash", "prefix-secret")
	logger.Sync()
	content, err := os.ReadFile(filepath.Join(dir, "segments.log"))
	if err != nil {
		t.Fatalf("读取日志失败: %v", err)
	}
	output := string(content)
	for _, secret := range []string{"upper-secret", "compound-secret", "prefix-secret"} {
		if strings.Contains(output, secret) {
			t.Errorf("日志泄露了密码字段 %q: %s", secret, output)
		}
	}
	if !strings.Contains(output, "free-text-secret") {
		t.Errorf("自由文本内容应保持原样: %s", output)
	}
}

func chdirForLoggerTest(t *testing.T, dir string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取当前目录失败: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("切换到测试目录失败: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Errorf("恢复当前目录失败: %v", err)
		}
	})
}

type testRedactionObject struct{}

func (testRedactionObject) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddString("password", "object-password")
	return nil
}

type testRedactionArray struct{}

func (testRedactionArray) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	if err := encoder.AppendObject(testRedactionArrayObject{}); err != nil {
		return err
	}
	return encoder.AppendReflected(map[string]string{"pwd": "reflected-array-password"})
}

type testRedactionArrayObject struct{}

func (testRedactionArrayObject) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddString("confirmPassword", "array-marshaler-password")
	return nil
}

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
