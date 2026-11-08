package v4

import "go.uber.org/zap/zapcore"

type Options struct {
	DisableCaller     bool // 是否完全禁用 caller；为 true 时任何级别都不输出文件和行号
	DisableStacktrace bool // 是否禁止在 error 及以上级别打印堆栈信息
	// Deprecated: 仅在 error 及以上级别输出 caller 已成为默认行为，保留该字段仅为兼容旧版本；
	ErrorOnlyCaller   bool     // 如需在所有级别输出 caller，请使用 CallerOnAllLevels。
	CallerOnAllLevels bool     // CallerOnAllLevels 是否在所有级别输出 caller；默认只在 error 及以上级别输出。
	Level             string   // 指定日志级别，可选值：debug, info, warn, error, dpanic, panic, fatal
	Format            string   // 指定日志显示格式，可选值：console, json
	OutputPaths       []string // 指定日志输出位置
	RedactKeys        []string // 追加需要隐藏值的结构化字段名（不区分大小写）；密码类默认字段始终隐藏
}

func NewOptions() *Options {
	return &Options{
		DisableCaller:     false,
		DisableStacktrace: false,
		Level:             zapcore.InfoLevel.String(),
		Format:            "console",
		OutputPaths:       []string{"stdout"},
	}
}
