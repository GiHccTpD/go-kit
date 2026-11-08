package v4

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestGlobalSetLevelAndGetLevel(t *testing.T) {
	dir := t.TempDir()
	chdirForLoggerTest(t, dir)
	oldStd := std
	t.Cleanup(func() { std = oldStd })

	Init(&Options{Level: "info", Format: "json", OutputPaths: []string{"level.log"}})

	if got := GetLevel(); got != "info" {
		t.Fatalf("初始等级为 %q，期望 info", got)
	}

	SetLevel("debug")
	if got := GetLevel(); got != "debug" {
		t.Fatalf("设置后等级为 %q，期望 debug", got)
	}
	if got := AtomicLevel().Level(); got != zapcore.DebugLevel {
		t.Fatalf("AtomicLevel 为 %v，期望 debug", got)
	}

	SetLevel("not-a-level")
	if got := GetLevel(); got != "debug" {
		t.Fatalf("非法等级不应生效，当前为 %q", got)
	}
}

func TestLoggerLevelIsIndependentOfGlobalLevel(t *testing.T) {
	dir := t.TempDir()
	chdirForLoggerTest(t, dir)
	oldStd := std
	t.Cleanup(func() { std = oldStd })

	Init(&Options{Level: "info", Format: "json", OutputPaths: []string{"global.log"}})
	logger := NewLogger(&Options{Level: "info", Format: "json", OutputPaths: []string{"local.log"}})

	logger.SetLevel("debug")

	if got := logger.GetLevel(); got != "debug" {
		t.Fatalf("实例等级为 %q，期望 debug", got)
	}
	if got := GetLevel(); got != "info" {
		t.Fatalf("全局等级不应被实例修改，当前为 %q", got)
	}
}
