package v3

import (
	"context"
	"github.com/GiHccTpD/go-kit/known"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger 定义了 miniblog 项目的日志接口. 该接口只包含了支持的日志记录方法.
type Logger interface {
	Debugw(msg string, keysAndValues ...interface{})
	Infow(msg string, keysAndValues ...interface{})
	Warnw(msg string, keysAndValues ...interface{})
	Errorw(msg string, keysAndValues ...interface{})
	Panicw(msg string, keysAndValues ...interface{})
	Fatalw(msg string, keysAndValues ...interface{})
	Sync()
}

// zapLogger 是 Logger 接口的具体实现. 它底层封装了 zap.Logger.
type zapLogger struct {
	z *zap.Logger
}

// 确保 zapLogger 实现了 Logger 接口. 以下变量赋值，可以使错误在编译期被发现.
var _ Logger = &zapLogger{}

var (
	mu sync.Mutex

	// std 定义了默认的全局 Logger.
	std = NewLogger(NewOptions())
)

// Init 使用指定的选项初始化 Logger.
func Init(opts *Options) {
	mu.Lock()
	defer mu.Unlock()

	std = NewLogger(opts)
}

// NewLogger 根据传入的 opts 创建 Logger.
func NewLogger(opts *Options) *zapLogger {
	if opts == nil {
		opts = NewOptions()
	}

	// 将文本格式的日志级别，例如 info 转换为 zapcore.Level 类型以供后面使用
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(opts.Level)); err != nil {
		// 如果指定了非法的日志级别，则默认使用 info 级别
		zapLevel = zapcore.InfoLevel
	}

	var outputPath = opts.OutputPaths
	writeSyncer := getLogWriter(outputPath)
	encoder := getEncoder()
	core := zapcore.NewCore(encoder, writeSyncer, zapLevel)

	// 开启开发模式，堆栈跟踪
	caller := zap.AddCaller()
	// 开启文件及行号
	development := zap.Development()

	z := zap.New(core, zap.AddStacktrace(zapcore.PanicLevel), zap.AddCallerSkip(1), caller, development)
	logger := &zapLogger{z: z}

	// 把标准库的 log.Logger 的 info 级别的输出重定向到 zap.Logger
	zap.RedirectStdLog(z)

	z.Info("log v3初始化成功🎉")
	return logger
}

func getEncoder() zapcore.Encoder {
	// 创建一个默认的 encoder 配置
	encoderConfig := zap.NewProductionEncoderConfig()
	// 自定义 MessageKey 为 message，message 语义更明确
	encoderConfig.MessageKey = "message"
	// 用于指定在日志条目中记录日志级别时使用的键名
	encoderConfig.LevelKey = "level"
	// 自定义 TimeKey 为 timestamp，timestamp 语义更明确
	encoderConfig.TimeKey = "timestamp"
	// 指定时间序列化函数，将时间序列化为 `2006-01-02 15:04:05.000` 格式，更易读
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}
	// 用于指定在日志条目中记录调用者信息（即日志记录所在的文件和行号）时使用的键名
	encoderConfig.CallerKey = "file"
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	// 指定 time.Duration 序列化函数，将 time.Duration 序列化为经过的毫秒数的浮点数
	// 毫秒数比默认的秒数更精确
	encoderConfig.EncodeDuration = func(d time.Duration, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendFloat64(float64(d) / float64(time.Millisecond))
	}
	return zapcore.NewJSONEncoder(encoderConfig) //获取编码器,NewJSONEncoder()输出json格式，NewConsoleEncoder()输出普通文本格式
}

func getLogWriter(outputPaths []string) zapcore.WriteSyncer {
	var writeSyncer zapcore.WriteSyncer
	var fileWriteSyncer zapcore.WriteSyncer

	stdout := lo.Contains(outputPaths, "stdout")
	filename, filenameOk := lo.Find(outputPaths, func(x string) bool {
		return strings.HasSuffix(x, ".log")
	})

	local, _ := os.Getwd()
	outfilename := path.Join(local, filename)

	if filenameOk {
		lumberJackLogger := newLumberJackLogger(outfilename)
		fileWriteSyncer = zapcore.AddSync(lumberJackLogger)
	}

	if len(outputPaths) >= 2 && filenameOk && stdout {
		writeSyncer = zapcore.NewMultiWriteSyncer(fileWriteSyncer, zapcore.AddSync(os.Stdout))
	} else if filenameOk && !stdout {
		writeSyncer = fileWriteSyncer
	} else {
		writeSyncer = zapcore.AddSync(os.Stdout)
	}
	return writeSyncer
}

func newLumberJackLogger(filename string) *lumberjack.Logger {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filename, // 日志输出文件
		MaxSize:    500,      // 日志最大保存
		MaxBackups: 30,       // 就日志保留5个备份
		MaxAge:     30,       // 最多保留30个日志 和MaxBackups参数配置1个就可以
		Compress:   true,     // 自导打 gzip包 默认false
	}
	return lumberJackLogger
}

// Sync 调用底层 zap.Logger 的 Sync 方法，将缓存中的日志刷新到磁盘文件中. 主程序需要在退出前调用 Sync.
func Sync() { std.Sync() }

func (l *zapLogger) Sync() {
	_ = l.z.Sync()
}

// Debugw 输出 debug 级别的日志.
func Debugw(msg string, keysAndValues ...interface{}) {
	std.z.Sugar().Debugw(msg, keysAndValues...)
}

func (l *zapLogger) Debugw(msg string, keysAndValues ...interface{}) {
	l.z.Sugar().Debugw(msg, keysAndValues...)
}

// Infow 输出 info 级别的日志.
func Infow(msg string, keysAndValues ...interface{}) {
	std.z.Sugar().Infow(msg, keysAndValues...)
}

func (l *zapLogger) Infow(msg string, keysAndValues ...interface{}) {
	l.z.Sugar().Infow(msg, keysAndValues...)
}

// Warnw 输出 warning 级别的日志.
func Warnw(msg string, keysAndValues ...interface{}) {
	std.z.Sugar().Warnw(msg, keysAndValues...)
}

func (l *zapLogger) Warnw(msg string, keysAndValues ...interface{}) {
	l.z.Sugar().Warnw(msg, keysAndValues...)
}

// Errorw 输出 error 级别的日志.
func Errorw(msg string, keysAndValues ...interface{}) {
	std.z.Sugar().Errorw(msg, keysAndValues...)
}

func (l *zapLogger) Errorw(msg string, keysAndValues ...interface{}) {
	l.z.Sugar().Errorw(msg, keysAndValues...)
}

// Panicw 输出 panic 级别的日志.
func Panicw(msg string, keysAndValues ...interface{}) {
	std.z.Sugar().Panicw(msg, keysAndValues...)
}

func (l *zapLogger) Panicw(msg string, keysAndValues ...interface{}) {
	l.z.Sugar().Panicw(msg, keysAndValues...)
}

// Fatalw 输出 fatal 级别的日志.
func Fatalw(msg string, keysAndValues ...interface{}) {
	std.z.Sugar().Fatalw(msg, keysAndValues...)
}

func (l *zapLogger) Fatalw(msg string, keysAndValues ...interface{}) {
	l.z.Sugar().Fatalw(msg, keysAndValues...)
}

// C 解析传入的 context，尝试提取关注的键值，并添加到 zap.Logger 结构化日志中.
func C(ctx context.Context) *zapLogger {
	return std.C(ctx)
}

func (l *zapLogger) C(ctx context.Context) *zapLogger {
	lc := l.clone()

	if requestID := ctx.Value(known.XRequestIDKey); requestID != nil {
		lc.z = lc.z.With(zap.Any(known.XRequestIDKey, requestID))
	}

	if userID := ctx.Value(known.XUsernameKey); userID != nil {
		lc.z = lc.z.With(zap.Any(known.XUsernameKey, userID))
	}

	return lc
}

// clone 深度拷贝 zapLogger.
func (l *zapLogger) clone() *zapLogger {
	lc := *l
	return &lc
}
