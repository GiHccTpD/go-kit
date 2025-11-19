package httpclient

import (
	"context"
	"time"

	log "github.com/GiHccTpD/go-kit/logger/v4"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"

	"github.com/GiHccTpD/go-kit/known"
)

var Client *resty.Client

func Init() {
	Client = resty.New().
		SetTimeout(3*time.Second).
		SetRetryCount(2).
		SetRetryWaitTime(200*time.Millisecond).
		SetHeader("Content-Type", "application/json")

	// ----------- 全局 BeforeRequest Hook ------------
	Client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		ctx := req.Context()

		// 注入 request-id / user-id
		if v := ctx.Value(known.XRequestIDKey); v != nil {
			req.SetHeader(known.XRequestIDKey, v.(string))
		}
		if v := ctx.Value(known.XUsernameKey); v != nil {
			req.SetHeader(known.XUsernameKey, v.(string))
		}

		// 记录开始时间
		req.SetContext(context.WithValue(ctx, "startTime", time.Now()))
		return nil
	})

	// ----------- 全局 AfterResponse Hook ------------
	Client.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		ctx := resp.Request.Context()
		start := ctx.Value("startTime").(time.Time)
		cost := time.Since(start)

		log.C(ctx).Infow("🌐 HTTP Request Done",
			zap.String("method", resp.Request.Method),
			zap.String("url", resp.Request.URL),
			zap.Any("query", resp.Request.QueryParam),
			zap.Int("status", resp.StatusCode()),
			zap.Int64("cost_ms", cost.Milliseconds()), // 转毫秒
			// zap.ByteString("body", resp.Body()), // 可开启
		)
		return nil
	})
}
