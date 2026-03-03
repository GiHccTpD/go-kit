package gin_request_log_middleware

import (
	"bytes"
	"io/ioutil"
	"time"

	"github.com/GiHccTpD/go-kit/known"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapLogger 使用 Zap 进行日志记录
func ZapLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 解析 requestID（提前解析）
		requestID, ok := c.Get(known.XRequestIDKey)
		if !ok || requestID == nil {
			requestID = "unknown"
		}

		// ====== 读取 Body（提前读取，供请求/响应都使用） ======
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = ioutil.ReadAll(c.Request.Body)
			c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// ================== 请求进入时打印 ==================
		entryFields := []zapcore.Field{
			zap.String("event", "request_in"),
			zap.String(known.XRequestIDKey, requestID.(string)),
			zap.String("client_ip", c.ClientIP()),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Any("query_params", c.Request.URL.Query()),
			zap.String("body_params", string(bodyBytes)),
		}
		logger.Info("📥HTTP request received", entryFields...)

		// 继续处理请求
		c.Next()

		// ================ 响应完成时打印 =====================
		latency := time.Since(start).Milliseconds()
		statusCode := c.Writer.Status()

		exitFields := []zapcore.Field{
			zap.String("event", "request_out"),
			zap.String(known.XRequestIDKey, requestID.(string)),
			zap.Int("status", statusCode),
			zap.Int64("latency_ms", latency),
			zap.String("client_ip", c.ClientIP()),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Any("params", c.Params),
			zap.Any("query_params", c.Request.URL.Query()),
			zap.String("body_params", string(bodyBytes)),
		}

		switch {
		case statusCode >= 200 && statusCode < 300:
			logger.Info("📤HTTP request completed", exitFields...)
		case statusCode >= 300 && statusCode < 400:
			logger.Warn("📤HTTP request completed", exitFields...)
		case statusCode >= 400:
			logger.Error("📤HTTP request completed", exitFields...)
		default:
			logger.Debug("📤HTTP request completed", exitFields...)
		}
	}
}
