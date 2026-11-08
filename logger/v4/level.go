package v4

import (
	"go.uber.org/zap"
)

// SetLevel 动态设置全局 Logger 的日志等级。
// 可选值：debug, info, warn, error, dpanic, panic, fatal；非法值会被忽略。
func SetLevel(level string) {
	std.SetLevel(level)
}

// GetLevel 返回全局 Logger 当前的日志等级。
func GetLevel() string {
	return std.GetLevel()
}

// AtomicLevel 返回全局 Logger 的 zap.AtomicLevel，
// 便于与其他基于 zap 的组件共享同一份动态等级。
func AtomicLevel() zap.AtomicLevel {
	return std.level
}
