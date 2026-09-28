package httpclient

import (
	"net/url"
	"time"

	"github.com/GiHccTpD/go-kit/known"
	"github.com/go-resty/resty/v2"
)

var Client *resty.Client

// Init creates the legacy Resty client without request logging.
func Init() {
	InitWithLogger(nil)
}

// InitWithLogger creates the legacy Resty client and optionally logs responses.
func InitWithLogger(logger Logger) {
	Client = resty.New().
		SetTimeout(3*time.Second).
		SetRetryCount(2).
		SetRetryWaitTime(200*time.Millisecond).
		SetHeader("Content-Type", "application/json")

	Client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		ctx := req.Context()
		if v := contextString(ctx, known.XRequestIDKey); v != "" {
			req.SetHeader(known.XRequestIDKey, v)
		}
		if v := contextString(ctx, known.XUsernameKey); v != "" {
			req.SetHeader(known.XUsernameKey, v)
		}
		return nil
	})

	if logger != nil {
		Client.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
			path := resp.Request.URL
			if parsed, err := url.Parse(path); err == nil {
				path = parsed.Path
			}
			logger.Infow("🌐 HTTP request done", "method", resp.Request.Method,
				"path", path, "status", resp.StatusCode(), "durationMs", resp.Time().Milliseconds())
			return nil
		})
	}
}
