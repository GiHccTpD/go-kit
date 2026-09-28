# 统一 HTTP SDK

`New` 为一个下游服务创建独立的 Resty 客户端和熔断器。客户端初始化后可在多个 goroutine 中复用；不要在运行中修改 `Config.Transport` 或底层传输配置。

```go
cfg := httpclient.DefaultConfig("https://account.internal")
cfg.TotalTimeout = 8 * time.Second
cfg.Logger = myLogger // 可选；传入实现 Infow 的现有日志实例
account, err := httpclient.New(cfg)
if err != nil {
    return err
}

var user User
resp, err := account.Do(ctx, httpclient.Request{
    Method: http.MethodGet,
    Path:   "/v1/users/42",
    Result: &user,
})
if err != nil {
    return err
}
if resp.IsError() {
    return fmt.Errorf("account returned HTTP %d", resp.StatusCode())
}
```

`Timeout` 限制单次尝试，`TotalTimeout` 限制完整调用（含退避）。Resty 执行带 jitter 的指数退避，默认最多重试 3 次；仅对网络错误、HTTP 429/502/503/504 重试。HTTP 400 等其他状态直接返回给调用方。

GET、HEAD、OPTIONS、TRACE、PUT、DELETE 可按 HTTP 幂等语义重试。POST/PATCH 默认不重试；只有下游**确实按 `Idempotency-Key` 去重**时，调用方才应填写 `Request.IdempotencyKey`。相同业务操作的重试必须使用相同的 key，不能每次生成新 key。普通 `Headers` 中的同名 header 不会启用重试。

```go
resp, err := account.Do(ctx, httpclient.Request{
    Method:         http.MethodPost,
    Path:           "/v1/messages",
    Body:           message,
    IdempotencyKey: message.ID,
})
```

每个 SDK 实例按**完整调用**统计连续临时故障；默认连续 5 次后断路 30 秒。断路期间返回 `ErrCircuitOpen`，到期只放行一个探测调用，成功后恢复。调用方可用 `errors.Is(err, httpclient.ErrCircuitOpen)` 判断。由于状态属于实例，应按目标服务分别创建实例，长期复用。

请求会透传 context 中的字符串 `X-Request-ID`、`Trace-Id`、`traceparent`、`X-Username`；显式 `Headers` 优先。缺少 request ID 时自动生成并在该调用的所有尝试中保持不变；缺少 trace ID 时使用 request ID。SDK 只透传 trace 上下文，不创建 span。启用日志后，记录 method、path、status、attempts、耗时、request ID、trace ID、完整 query 和 body；JSON 字节体解析为结构化值，流式 body 不会因日志被读取。注入的 logger 应对敏感字段进行脱敏。

`Config.Logger` 接受实现 `Infow(string, ...interface{})` 的现有日志实例；例如调用方自己的 logger 或 `zap.Logger.Sugar()`。未设置时不打印 SDK 请求日志。SDK 只打印一次调用摘要，错误仍返回给调用方，由服务边界决定错误日志级别。

现有 `Init`/`Client` 是兼容入口。`Init()` 不启用请求日志；旧调用若需日志，可使用 `InitWithLogger(myLogger)`，同样记录 request ID、trace ID、query 和 body。使用 `Client.R()` 的旧调用不会经过本 SDK 的熔断与幂等策略。新调用应使用 `New`/`Do`；旧调用可按服务逐步迁移。
