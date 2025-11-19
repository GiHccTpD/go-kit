package v4

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"time"
)

func newEncoder(format string) zapcore.Encoder {
	encCfg := zap.NewProductionEncoderConfig()

	encCfg.TimeKey = "timestamp"
	encCfg.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}
	encCfg.LevelKey = "level"
	encCfg.MessageKey = "message"
	encCfg.CallerKey = "file"
	encCfg.EncodeLevel = zapcore.CapitalLevelEncoder

	if format == "console" {
		return zapcore.NewConsoleEncoder(encCfg)
	}
	return zapcore.NewJSONEncoder(encCfg)
}
