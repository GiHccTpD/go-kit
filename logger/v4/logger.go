package v4

import (
	"context"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/GiHccTpD/go-kit/known"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

/* -------------------- Interface -------------------- */

type Logger interface {
	Debugw(msg string, keysAndValues ...interface{})
	Infow(msg string, keysAndValues ...interface{})
	Warnw(msg string, keysAndValues ...interface{})
	Errorw(msg string, keysAndValues ...interface{})
	Panicw(msg string, keysAndValues ...interface{})
	Fatalw(msg string, keysAndValues ...interface{})
	C(ctx context.Context) *zapLogger
	SetLevel(level string)
	GetLevel() string
	DB() *zap.Logger
	Sync()
}

/* -------------------- Global -------------------- */

var (
	mu  sync.Mutex
	std = NewLogger(NewOptions())
)

///* -------------------- Options -------------------- */
//
//type Options struct {
//	Level       string   // info/debug/warn/error
//	OutputPaths []string // stdout, xxx.log
//}
//
//func NewOptions() *Options {
//	return &Options{
//		Level:       "info",
//		OutputPaths: []string{"stdout"},
//	}
//}

/* -------------------- zapLogger -------------------- */

type zapLogger struct {
	z     *zap.Logger
	level zap.AtomicLevel
}

/* -------------------- Init -------------------- */

func Init(opts *Options) {
	mu.Lock()
	defer mu.Unlock()

	std = NewLogger(opts)
}

func GetZapLogger() *zap.Logger { return std.z }

/* -------------------- NewLogger -------------------- */

func NewLogger(opts *Options) *zapLogger {
	if opts == nil {
		opts = NewOptions()
	}

	// AtomicLevel 支持动态调整日志等级
	level := zap.NewAtomicLevel()

	if err := level.UnmarshalText([]byte(opts.Level)); err != nil {
		level.SetLevel(zap.InfoLevel)
	}

	encoder := getEncoder()
	writer := getWriteSyncer(opts.OutputPaths)

	core := zapcore.NewCore(encoder, writer, level)

	z := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zap.ErrorLevel),
	)

	zap.RedirectStdLog(z)

	z.Info("log v4 初始化成功🎉")

	return &zapLogger{z: z, level: level}
}

/* -------------------- Encoder -------------------- */

func getEncoder() zapcore.Encoder {
	cfg := zap.NewProductionEncoderConfig()
	cfg.MessageKey = "message"
	cfg.LevelKey = "level"
	cfg.TimeKey = "timestamp"
	cfg.CallerKey = "file"
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	cfg.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}
	return zapcore.NewJSONEncoder(cfg)
}

/* -------------------- WriteSyncer -------------------- */

func getWriteSyncer(paths []string) zapcore.WriteSyncer {
	stdout := lo.Contains(paths, "stdout")

	filename, ok := lo.Find(paths, func(x string) bool { return strings.HasSuffix(x, ".log") })

	var fileWS zapcore.WriteSyncer
	if ok {
		local, _ := os.Getwd()
		out := path.Join(local, filename)
		fileWS = zapcore.AddSync(&lumberjack.Logger{
			Filename:   out,
			MaxSize:    500,
			MaxBackups: 30,
			MaxAge:     30,
			Compress:   true,
		})
	}

	switch {
	case ok && stdout:
		return zapcore.NewMultiWriteSyncer(fileWS, zapcore.AddSync(os.Stdout))
	case ok:
		return fileWS
	default:
		return zapcore.AddSync(os.Stdout)
	}
}

/* -------------------- Core Logging -------------------- */

func (l *zapLogger) Debugw(msg string, keysAndValues ...interface{}) {
	l.z.Sugar().Debugw(msg, keysAndValues...)
}
func (l *zapLogger) Infow(msg string, keysAndValues ...interface{}) {
	l.z.Sugar().Infow(msg, keysAndValues...)
}
func (l *zapLogger) Warnw(msg string, keysAndValues ...interface{}) {
	l.z.Sugar().Warnw(msg, keysAndValues...)
}
func (l *zapLogger) Errorw(msg string, keysAndValues ...interface{}) {
	l.z.Sugar().Errorw(msg, keysAndValues...)
}
func (l *zapLogger) Panicw(msg string, keysAndValues ...interface{}) {
	l.z.Sugar().Panicw(msg, keysAndValues...)
}
func (l *zapLogger) Fatalw(msg string, keysAndValues ...interface{}) {
	l.z.Sugar().Fatalw(msg, keysAndValues...)
}

/* -------------------- Context Fields 注入 -------------------- */

func C(ctx context.Context) *zapLogger {
	return std.C(ctx)
}

func (l *zapLogger) C(ctx context.Context) *zapLogger {
	lc := l.clone()

	if ctx == nil {
		return lc
	}

	if requestID := ctx.Value(known.XRequestIDKey); requestID != nil {
		lc.z = lc.z.With(zap.Any(known.XRequestIDKey, requestID))
	}

	if userID := ctx.Value(known.XUsernameKey); userID != nil {
		lc.z = lc.z.With(zap.Any(known.XUsernameKey, userID))
	}

	return lc
}

// ========== Global Logging API (no context) ==========

func Debugw(msg string, keysAndValues ...interface{}) {
	std.Debugw(msg, keysAndValues...)
}

func Infow(msg string, keysAndValues ...interface{}) {
	std.Infow(msg, keysAndValues...)
}

func Warnw(msg string, keysAndValues ...interface{}) {
	std.Warnw(msg, keysAndValues...)
}

func Errorw(msg string, keysAndValues ...interface{}) {
	std.Errorw(msg, keysAndValues...)
}

func Panicw(msg string, keysAndValues ...interface{}) {
	std.Panicw(msg, keysAndValues...)
}

func Fatalw(msg string, keysAndValues ...interface{}) {
	std.Fatalw(msg, keysAndValues...)
}

/* -------------------- Clone -------------------- */

func (l *zapLogger) clone() *zapLogger {
	n := *l
	return &n
}

/* -------------------- AtomicLevel API -------------------- */

func (l *zapLogger) SetLevel(level string) {
	var zl zapcore.Level
	if err := zl.UnmarshalText([]byte(level)); err != nil {
		return
	}
	l.level.SetLevel(zl)
}

func (l *zapLogger) GetLevel() string {
	return l.level.Level().String()
}

/* -------------------- Others -------------------- */

func (l *zapLogger) DB() *zap.Logger { return l.z }

func Sync() { std.Sync() }
func (l *zapLogger) Sync() {
	_ = l.z.Sync()
}
