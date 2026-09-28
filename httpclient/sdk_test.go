package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GiHccTpD/go-kit/known"
)

// newTestSDK builds a client with short waits for HTTP behavior tests.
func newTestSDK(t *testing.T, transport http.RoundTripper) *SDK {
	t.Helper()
	cfg := DefaultConfig("http://downstream.test")
	cfg.RetryCount = 2
	cfg.RetryWaitTime = time.Millisecond
	cfg.RetryMaxWaitTime = 2 * time.Millisecond
	cfg.BreakerThreshold = 2
	cfg.BreakerOpenDuration = 20 * time.Millisecond
	cfg.Transport = transport
	sdk, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return sdk
}

// testResponse returns a minimal HTTP response for an in-memory transport.
func testResponse(req *http.Request, status int) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: req}
}

func TestRetryWhitelistAndHeaders(t *testing.T) {
	var attempts atomic.Int32
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get(known.XRequestIDKey) != "request-1" || r.Header.Get(traceIDHeader) != "trace-1" ||
			r.Header.Get(traceparentHeader) != "00-trace-parent" || r.Header.Get(known.XUsernameKey) != "user-1" {
			t.Errorf("missing propagated headers: %v", r.Header)
		}
		if attempts.Add(1) < 3 {
			return testResponse(r, http.StatusServiceUnavailable), nil
		}
		return testResponse(r, http.StatusOK), nil
	})

	ctx := context.WithValue(context.Background(), known.XRequestIDKey, "request-1")
	ctx = context.WithValue(ctx, traceIDHeader, "trace-1")
	ctx = context.WithValue(ctx, traceparentHeader, "00-trace-parent")
	ctx = context.WithValue(ctx, known.XUsernameKey, "user-1")
	resp, err := newTestSDK(t, transport).Do(ctx, Request{Method: http.MethodGet, Path: "/resource"})
	if err != nil || resp.StatusCode() != http.StatusOK || attempts.Load() != 3 {
		t.Fatalf("response=%v err=%v attempts=%d", resp, err, attempts.Load())
	}
}

func TestBadRequestNeverRetries(t *testing.T) {
	var attempts atomic.Int32
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return testResponse(r, http.StatusBadRequest), nil
	})
	resp, err := newTestSDK(t, transport).Do(context.Background(), Request{Method: http.MethodGet, Path: "/bad"})
	if err != nil || resp.StatusCode() != http.StatusBadRequest || attempts.Load() != 1 {
		t.Fatalf("response=%v err=%v attempts=%d", resp, err, attempts.Load())
	}
}

func TestRedirectDoesNotRetry(t *testing.T) {
	var attempts atomic.Int32
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts.Add(1)
		resp := testResponse(r, http.StatusFound)
		resp.Header.Set("Location", "/elsewhere")
		return resp, nil
	})
	_, _ = newTestSDK(t, transport).Do(context.Background(), Request{Method: http.MethodGet, Path: "/start"})
	if attempts.Load() != 1 {
		t.Fatalf("redirect was retried: attempts=%d", attempts.Load())
	}
}

func TestPostRetryRequiresIdempotencyKey(t *testing.T) {
	var attempts atomic.Int32
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts.Add(1)
		if r.URL.Path == "/keyed" && r.Header.Get(idempotencyHeader) != "operation-1" {
			t.Errorf("missing idempotency key")
		}
		return testResponse(r, http.StatusBadGateway), nil
	})
	sdk := newTestSDK(t, transport)
	resp, err := sdk.Do(context.Background(), Request{Method: http.MethodPost, Path: "/plain"})
	if err != nil || resp.StatusCode() != http.StatusBadGateway || attempts.Load() != 1 {
		t.Fatalf("plain post: response=%v err=%v attempts=%d", resp, err, attempts.Load())
	}
	resp, err = sdk.Do(context.Background(), Request{Method: http.MethodPost, Path: "/keyed", IdempotencyKey: "operation-1"})
	if err != nil || resp.StatusCode() != http.StatusBadGateway || attempts.Load() != 4 {
		t.Fatalf("keyed post: response=%v err=%v attempts=%d", resp, err, attempts.Load())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type recordingLogger struct {
	mu     sync.Mutex
	called int
	fields []interface{}
}

func (l *recordingLogger) Infow(_ string, fields ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.called++
	l.fields = append([]interface{}(nil), fields...)
}

func TestConfiguredLoggerReceivesCall(t *testing.T) {
	logger := &recordingLogger{}
	cfg := DefaultConfig("http://downstream.test")
	cfg.Logger = logger
	cfg.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return testResponse(req, http.StatusOK), nil
	})
	sdk, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sdk.Do(context.Background(), Request{Method: http.MethodGet, Path: "/resource"})
	if err != nil {
		t.Fatal(err)
	}
	logger.mu.Lock()
	defer logger.mu.Unlock()
	if logger.called != 1 {
		t.Fatalf("logger called %d times", logger.called)
	}
	if len(logger.fields)%2 != 0 {
		t.Fatalf("unexpected log fields: %v", logger.fields)
	}
	var hasRequestID bool
	for i := 0; i < len(logger.fields); i += 2 {
		if logger.fields[i] == "requestId" && logger.fields[i+1] != "" {
			hasRequestID = true
		}
	}
	if !hasRequestID {
		t.Fatalf("request ID missing from log fields: %v", logger.fields)
	}
}

func TestTransportErrorsRespectIdempotency(t *testing.T) {
	var attempts atomic.Int32
	cfg := DefaultConfig("http://downstream.test")
	cfg.RetryWaitTime = time.Millisecond
	cfg.RetryMaxWaitTime = 2 * time.Millisecond
	cfg.RetryCount = 2
	cfg.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if attempts.Add(1) < 3 {
			return nil, errors.New("connection reset")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok")), Request: req}, nil
	})
	sdk, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sdk.Do(context.Background(), Request{Method: http.MethodPost, Path: "/write"})
	if err == nil || attempts.Load() != 1 {
		t.Fatalf("unsafe post: err=%v attempts=%d", err, attempts.Load())
	}
	resp, err := sdk.Do(context.Background(), Request{Method: http.MethodGet, Path: "/read"})
	if err != nil || resp.StatusCode() != http.StatusOK || attempts.Load() != 3 {
		t.Fatalf("safe get: response=%v err=%v attempts=%d", resp, err, attempts.Load())
	}
}

func TestCircuitBreakerCountsCallsAndProbes(t *testing.T) {
	var attempts atomic.Int32
	var status atomic.Int32
	status.Store(http.StatusServiceUnavailable)
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return testResponse(r, int(status.Load())), nil
	})
	sdk := newTestSDK(t, transport)
	for i := 0; i < 2; i++ {
		resp, err := sdk.Do(context.Background(), Request{Method: http.MethodGet, Path: "/health"})
		if err != nil || resp.StatusCode() != http.StatusServiceUnavailable {
			t.Fatalf("failure %d: response=%v err=%v", i, resp, err)
		}
	}
	if attempts.Load() != 6 {
		t.Fatalf("breaker counted attempts instead of calls: %d", attempts.Load())
	}
	_, err := sdk.Do(context.Background(), Request{Method: http.MethodGet, Path: "/health"})
	if !errors.Is(err, ErrCircuitOpen) || attempts.Load() != 6 {
		t.Fatalf("open circuit: err=%v attempts=%d", err, attempts.Load())
	}
	status.Store(http.StatusOK)
	time.Sleep(25 * time.Millisecond)
	resp, err := sdk.Do(context.Background(), Request{Method: http.MethodGet, Path: "/health"})
	if err != nil || resp.StatusCode() != http.StatusOK {
		t.Fatalf("half-open probe: response=%v err=%v", resp, err)
	}
	resp, err = sdk.Do(context.Background(), Request{Method: http.MethodGet, Path: "/health"})
	if err != nil || resp.StatusCode() != http.StatusOK {
		t.Fatalf("closed circuit: response=%v err=%v", resp, err)
	}
}

func TestTotalTimeoutStopsRetries(t *testing.T) {
	var attempts atomic.Int32
	cfg := DefaultConfig("http://slow.test")
	cfg.Timeout = 100 * time.Millisecond
	cfg.TotalTimeout = 10 * time.Millisecond
	cfg.BreakerThreshold = 1
	cfg.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts.Add(1)
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	sdk, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sdk.Do(context.Background(), Request{Method: http.MethodGet, Path: "/slow"})
	if !errors.Is(err, context.DeadlineExceeded) || attempts.Load() != 1 {
		t.Fatalf("err=%v attempts=%d", err, attempts.Load())
	}
	_, err = sdk.Do(context.Background(), Request{Method: http.MethodGet, Path: "/slow"})
	if !errors.Is(err, ErrCircuitOpen) || attempts.Load() != 1 {
		t.Fatalf("timed-out downstream should open circuit: err=%v attempts=%d", err, attempts.Load())
	}
}
