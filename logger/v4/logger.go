package v4

import (
	"context"
	"os"
	"path"
	"strings"
	"sync"

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
	With(fields ...zap.Field) *zapLogger
	SetLevel(level string)
	GetLevel() string
	DB() *zap.Logger
	Sync()
}

// 确保 zapLogger 实现了 Logger 接口，编译期即可发现遗漏。
var _ Logger = (*zapLogger)(nil)

/* -------------------- Global -------------------- */

var (
	mu  sync.Mutex
	std = NewLogger(NewOptions())
)

/* -------------------- zapLogger -------------------- */

type zapLogger struct {
	// z 供本包的封装方法使用，已补偿封装层带来的调用栈偏移。
	z *zap.Logger
	// native 供 DB()/GetZapLogger() 直接调用 zap 时使用，保持原始调用栈。
	native *zap.Logger
	level  zap.AtomicLevel
}

/* -------------------- Init -------------------- */

func Init(opts *Options) {
	mu.Lock()
	defer mu.Unlock()

	std = NewLogger(opts)
}

func GetZapLogger() *zap.Logger { return std.DB() }

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

	encoder := newEncoder(opts.Format)
	writer := getWriteSyncer(opts.OutputPaths)

	// 默认仅在 error 及以上级别输出 caller；CallerOnAllLevels 可放开该限制。
	core := newRedactingCore(zapcore.NewCore(encoder, writer, level), opts.RedactKeys, !opts.CallerOnAllLevels)

	zapOptions := make([]zap.Option, 0, 3)
	// 封装方法会多一层栈帧，即使不输出 caller 也要补偿，否则 error 堆栈会包含本包内部帧。
	zapOptions = append(zapOptions, zap.AddCallerSkip(1))
	if !opts.DisableCaller {
		zapOptions = append(zapOptions, zap.AddCaller())
	}
	if !opts.DisableStacktrace {
		zapOptions = append(zapOptions, zap.AddStacktrace(zap.ErrorLevel))
	}

	z := zap.New(core, zapOptions...)
	// 直接使用 zap 时不需要补偿封装层栈帧，因此单独保留一个无偏移的实例。
	native := z.WithOptions(zap.AddCallerSkip(-1))

	zap.RedirectStdLog(native)

	return &zapLogger{z: z, native: native, level: level}
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

/* -------------------- Fields -------------------- */

func With(fields ...zap.Field) *zapLogger {
	return std.With(fields...)
}

func (l *zapLogger) With(fields ...zap.Field) *zapLogger {
	return l.with(fields...)
}

// with 同时派生封装与原生两个 logger，保证附加字段在 DB() 返回的实例上同样生效。
func (l *zapLogger) with(fields ...zap.Field) *zapLogger {
	lc := l.clone()

	if len(fields) == 0 {
		return lc
	}

	lc.z = lc.z.With(fields...)
	lc.native = lc.native.With(fields...)

	return lc
}

/* -------------------- Context Fields 注入 -------------------- */

func C(ctx context.Context) *zapLogger {
	return std.C(ctx)
}

func (l *zapLogger) C(ctx context.Context) *zapLogger {
	if ctx == nil {
		return l.clone()
	}

	fields := make([]zap.Field, 0, 2)

	if requestID := ctx.Value(known.XRequestIDKey); requestID != nil {
		fields = append(fields, zap.Any(known.XRequestIDKey, requestID))
	}

	if username := ctx.Value(known.XUsernameKey); username != nil {
		fields = append(fields, zap.Any(known.XUsernameKey, username))
	}

	return l.with(fields...)
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

func (l *zapLogger) DB() *zap.Logger { return l.native }

func Sync() { std.Sync() }
func (l *zapLogger) Sync() {
	_ = l.z.Sync()
}
