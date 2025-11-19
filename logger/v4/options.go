package v4

import "go.uber.org/zap/zapcore"

type Options struct {
	DisableCaller     bool     // 是否开启 caller，如果开启会在日志中显示调用日志所在的文件和行号
	DisableStacktrace bool     // 是否禁止在 panic 及以上级别打印堆栈信息
	Level             string   // 指定日志级别，可选值：debug, info, warn, error, dpanic, panic, fatal
	Format            string   // 指定日志显示格式，可选值：console, json
	OutputPaths       []string // 指定日志输出位置
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
