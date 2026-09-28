package v4

import "go.uber.org/zap/zapcore"

type Options struct {
	DisableCaller     bool     // 是否禁用 caller，如果不禁用会在日志中显示调用日志所在的文件和行号
	DisableStacktrace bool     // 是否禁止在 panic 及以上级别打印堆栈信息
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
