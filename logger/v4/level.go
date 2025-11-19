package v4

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// atomicLevel 是全局可动态调整的等级（注意用 zap.NewAtomicLevelAt）
var atomicLevel = zap.NewAtomicLevelAt(zapcore.InfoLevel)

// SetLevel 动态设置等级
func SetLevel(level zapcore.Level) {
	atomicLevel.SetLevel(level)
}

// GetLevel 返回当前等级
func GetLevel() zapcore.Level {
	return atomicLevel.Level()
}

// AtomicLevel 返回该 AtomicLevel（用于创建 core）
func AtomicLevel() zap.AtomicLevel {
	return atomicLevel
}
