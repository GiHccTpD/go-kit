package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
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

func (l *recordingLogger) fieldsByName(t *testing.T) map[string]interface{} {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.fields)%2 != 0 {
		t.Fatalf("unexpected log fields: %v", l.fields)
	}
	fields := make(map[string]interface{}, len(l.fields)/2)
	for i := 0; i < len(l.fields); i += 2 {
		key, ok := l.fields[i].(string)
		if !ok {
			t.Fatalf("unexpected log key: %v", l.fields[i])
		}
		fields[key] = l.fields[i+1]
	}
	return fields
}

func TestConfiguredLoggerReceivesCall(t *testing.T) {
	logger := &recordingLogger{}
	cfg := DefaultConfig("http://downstream.test")
	cfg.Logger = logger
	cfg.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil || string(body) != `{"message":"hello"}` {
			t.Errorf("request body=%q err=%v", body, err)
		}
		return testResponse(req, http.StatusOK), nil
	})
	sdk, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), known.XRequestIDKey, "request-1")
	ctx = context.WithValue(ctx, traceIDHeader, "trace-1")
	_, err = sdk.Do(ctx, Request{Method: http.MethodPost, Path: "/resource?inline=yes",
		Query: url.Values{"extra": {"two"}}, Body: json.RawMessage(`{"message":"hello"}`)})
	if err != nil {
		t.Fatal(err)
	}
	logger.mu.Lock()
	called := logger.called
	logger.mu.Unlock()
	if called != 1 {
		t.Fatalf("logger called %d times", called)
	}
	fields := logger.fieldsByName(t)
	query, ok := fields["query"].(url.Values)
	if !ok || query.Get("inline") != "yes" || query.Get("extra") != "two" {
		t.Fatalf("query=%v", fields["query"])
	}
	body, ok := fields["body"].(map[string]interface{})
	if !ok || body["message"] != "hello" {
		t.Fatalf("body=%v", fields["body"])
	}
	if fields["requestId"] != "request-1" || fields["traceId"] != "trace-1" {
		t.Fatalf("request/trace ID fields=%v", fields)
	}
}

func TestLegacyInitWithLogger(t *testing.T) {
	previous := Client
	t.Cleanup(func() { Client = previous })
	logger := &recordingLogger{}
	InitWithLogger(logger)
	Client.SetTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get(known.XRequestIDKey) != "legacy-request" {
			t.Errorf("request ID was not propagated")
		}
		if req.Header.Get(traceIDHeader) != "legacy-trace" {
			t.Errorf("trace ID was not propagated")
		}
		body, err := io.ReadAll(req.Body)
		if err != nil || string(body) != `{"message":"hello"}` {
			t.Errorf("request body=%q err=%v", body, err)
		}
		return testResponse(req, http.StatusOK), nil
	}))
	ctx := context.WithValue(context.Background(), known.XRequestIDKey, "legacy-request")
	ctx = context.WithValue(ctx, traceIDHeader, "legacy-trace")
	resp, err := Client.R().SetContext(ctx).SetQueryParam("extra", "two").
		SetBody([]byte(`{"message":"hello"}`)).Post("http://downstream.test/resource?inline=yes")
	if err != nil || resp.StatusCode() != http.StatusOK {
		t.Fatalf("response=%v err=%v", resp, err)
	}
	logger.mu.Lock()
	called := logger.called
	logger.mu.Unlock()
	if called != 1 {
		t.Fatalf("legacy logger called %d times", called)
	}
	fields := logger.fieldsByName(t)
	query, ok := fields["query"].(url.Values)
	if !ok || query.Get("inline") != "yes" || query.Get("extra") != "two" {
		t.Fatalf("legacy query=%v", fields["query"])
	}
	body, ok := fields["body"].(map[string]interface{})
	if !ok || body["message"] != "hello" {
		t.Fatalf("legacy body=%v", fields["body"])
	}
	if fields["requestId"] != "legacy-request" || fields["traceId"] != "legacy-trace" {
		t.Fatalf("legacy request/trace ID fields=%v", fields)
	}
}

func TestLegacyInitGeneratesTraceID(t *testing.T) {
	previous := Client
	t.Cleanup(func() { Client = previous })
	logger := &recordingLogger{}
	InitWithLogger(logger)
	var sentRequestID string
	Client.SetTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		sentRequestID = req.Header.Get(known.XRequestIDKey)
		if sentRequestID == "" || req.Header.Get(traceIDHeader) != sentRequestID {
			t.Errorf("generated IDs were not sent: %v", req.Header)
		}
		return testResponse(req, http.StatusOK), nil
	}))
	if _, err := Client.R().Get("http://downstream.test/resource"); err != nil {
		t.Fatal(err)
	}
	fields := logger.fieldsByName(t)
	if fields["requestId"] != sentRequestID || fields["traceId"] != sentRequestID {
		t.Fatalf("generated IDs were not logged: %v", fields)
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
