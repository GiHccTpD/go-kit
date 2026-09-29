package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/GiHccTpD/go-kit/known"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
)

const (
	traceIDHeader     = "Trace-Id"
	traceparentHeader = "traceparent"
	idempotencyHeader = "Idempotency-Key"
)

// Logger is the minimal logging interface accepted by the SDK.
type Logger interface {
	Infow(msg string, keysAndValues ...interface{})
}

// Config defines one downstream service's HTTP policy. Create a separate SDK per service.
type Config struct {
	BaseURL             string
	Timeout             time.Duration // Per attempt.
	TotalTimeout        time.Duration // Entire call, including retries and backoff.
	RetryCount          int
	RetryWaitTime       time.Duration
	RetryMaxWaitTime    time.Duration
	BreakerThreshold    int
	BreakerOpenDuration time.Duration
	Transport           http.RoundTripper // Optional; set once before the SDK is used.
	Logger              Logger            // Optional; nil disables SDK request logging.
}

// Request describes one HTTP call. IdempotencyKey opts POST/PATCH into retries only when
// the downstream service actually deduplicates requests by this key.
type Request struct {
	Method         string
	Path           string
	Headers        http.Header
	Query          url.Values
	Body           any
	Result         any
	Error          any
	IdempotencyKey string
}

// SDK owns the Resty client and a circuit breaker for one downstream service.
type SDK struct {
	client  *resty.Client
	breaker *breaker
	config  Config
}

// DefaultConfig returns a bounded policy suitable as a starting point for one service.
func DefaultConfig(baseURL string) Config {
	return Config{
		BaseURL:             baseURL,
		Timeout:             3 * time.Second,
		TotalTimeout:        10 * time.Second,
		RetryCount:          3,
		RetryWaitTime:       100 * time.Millisecond,
		RetryMaxWaitTime:    time.Second,
		BreakerThreshold:    5,
		BreakerOpenDuration: 30 * time.Second,
	}
}

// New creates an SDK with validated policy and a private Resty client.
func New(cfg Config) (*SDK, error) {
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || base == nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("invalid HTTP base URL")
	}
	if cfg.Timeout <= 0 || cfg.TotalTimeout <= 0 || cfg.RetryCount < 0 ||
		cfg.RetryWaitTime <= 0 || cfg.RetryMaxWaitTime < cfg.RetryWaitTime ||
		cfg.BreakerThreshold <= 0 || cfg.BreakerOpenDuration <= 0 {
		return nil, errors.New("invalid HTTP client policy")
	}

	client := resty.New().
		SetBaseURL(strings.TrimRight(cfg.BaseURL, "/")).
		SetTimeout(cfg.Timeout).
		SetRetryCount(cfg.RetryCount).
		SetRetryWaitTime(cfg.RetryWaitTime).
		SetRetryMaxWaitTime(cfg.RetryMaxWaitTime).
		SetLogger(silentRestyLogger{}).
		SetRedirectPolicy(resty.NoRedirectPolicy())
	if cfg.Transport != nil {
		client.SetTransport(cfg.Transport)
	}
	return &SDK{client: client, breaker: newBreaker(cfg.BreakerThreshold, cfg.BreakerOpenDuration), config: cfg}, nil
}

// Do executes a request with a total deadline, retry policy and circuit breaker.
// HTTP error statuses are returned as responses, as in Resty; only transport or SDK
// failures return an error. A non-idempotent request is retried only with an explicit
// IdempotencyKey and a downstream deduplication guarantee.
func (s *SDK) Do(ctx context.Context, call Request) (*resty.Response, error) {
	if s == nil {
		return nil, errors.New("HTTP SDK is nil")
	}
	if ctx == nil {
		return nil, errors.New("HTTP context is nil")
	}
	method := strings.ToUpper(call.Method)
	if method == "" || !isSupportedMethod(method) {
		return nil, fmt.Errorf("unsupported HTTP method %q", call.Method)
	}
	path, err := url.Parse(call.Path)
	if err != nil || path == nil || path.IsAbs() || path.Host != "" || path.User != nil || path.Fragment != "" || !strings.HasPrefix(call.Path, "/") || strings.HasPrefix(call.Path, "//") {
		return nil, errors.New("invalid relative HTTP path")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	probe, err := s.breaker.allow(time.Now())
	if err != nil {
		return nil, err
	}
	callerCtx := ctx
	ctx, cancel := context.WithTimeout(ctx, s.config.TotalTimeout)
	defer cancel()

	headers := call.Headers.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	if call.IdempotencyKey != "" {
		headers.Set(idempotencyHeader, call.IdempotencyKey)
	}
	requestID := headers.Get(known.XRequestIDKey)
	if requestID == "" {
		requestID = contextString(ctx, known.XRequestIDKey)
	}
	if requestID == "" {
		requestID = uuid.NewString()
	}
	headers.Set(known.XRequestIDKey, requestID)
	traceID := headers.Get(traceIDHeader)
	if traceID == "" {
		traceID = contextString(ctx, traceIDHeader)
	}
	if traceID == "" {
		traceID = requestID
	}
	headers.Set(traceIDHeader, traceID)
	if headers.Get(traceparentHeader) == "" {
		if v := contextString(ctx, traceparentHeader); v != "" {
			headers.Set(traceparentHeader, v)
		}
	}
	if headers.Get(known.XUsernameKey) == "" {
		if v := contextString(ctx, known.XUsernameKey); v != "" {
			headers.Set(known.XUsernameKey, v)
		}
	}

	req := s.client.R().SetContext(ctx).SetHeaderMultiValues(headers)
	if call.Query != nil {
		req.SetQueryParamsFromValues(call.Query)
	}
	if call.Body != nil {
		req.SetBody(call.Body)
	}
	if call.Result != nil {
		req.SetResult(call.Result)
	}
	if call.Error != nil {
		req.SetError(call.Error)
	}
	retryAllowed := isIdempotentMethod(method) || call.IdempotencyKey != ""
	req.AddRetryCondition(func(resp *resty.Response, err error) bool {
		if !retryAllowed || ctx.Err() != nil {
			return false
		}
		return isTransient(resp, err)
	})

	started := time.Now()
	resp, err := req.Execute(method, call.Path)
	outcome := outcomeSuccess
	if callerCtx.Err() != nil {
		outcome = outcomeNeutral
	} else if isTransient(resp, err) {
		outcome = outcomeFailure
	}
	s.breaker.record(probe, outcome)
	status := 0
	if resp != nil {
		status = resp.StatusCode()
	}
	if s.config.Logger != nil {
		s.config.Logger.Infow("🌐 HTTP request done", "method", method, "url", req.URL,
			"query", requestLogQuery(req), "status", status,
			"costMs", time.Since(started).Milliseconds(), "attempts", req.Attempt,
			"requestId", requestID, "traceId", traceID,
			"reqBody", requestLogBody(req))
	}
	if err != nil {
		return resp, fmt.Errorf("execute %s %s: %w", method, path.Path, err)
	}
	return resp, nil
}

// contextString reads a string context value without assuming all callers use the same type.
func contextString(ctx context.Context, key string) string {
	v, _ := ctx.Value(key).(string)
	return v
}

// isSupportedMethod limits the SDK to standard HTTP methods.
func isSupportedMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace,
		http.MethodPut, http.MethodDelete, http.MethodPost, http.MethodPatch:
		return true
	default:
		return false
	}
}

// isIdempotentMethod identifies methods safe to replay under HTTP semantics.
func isIdempotentMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace,
		http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

// isTransient is the retry and breaker failure whitelist.
func isTransient(resp *resty.Response, err error) bool {
	if resp != nil && resp.StatusCode() != 0 {
		switch resp.StatusCode() {
		case http.StatusTooManyRequests, http.StatusBadGateway,
			http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		default:
			return false
		}
	}
	return err != nil
}

// silentRestyLogger leaves final error logging to the caller's service boundary.
type silentRestyLogger struct{}

func (silentRestyLogger) Errorf(string, ...interface{}) {}
func (silentRestyLogger) Warnf(string, ...interface{})  {}
func (silentRestyLogger) Debugf(string, ...interface{}) {}
