package httpclient

import (
	"encoding/json"
	"io"
	"net/url"
	"unicode/utf8"

	"github.com/go-resty/resty/v2"
)

// requestLogQuery returns the final query sent on the wire when available.
func requestLogQuery(req *resty.Request) url.Values {
	if req.RawRequest != nil {
		return req.RawRequest.URL.Query()
	}
	query := make(url.Values)
	if parsed, err := url.Parse(req.URL); err == nil {
		query = parsed.Query()
	}
	for key, values := range req.QueryParam {
		query[key] = append(query[key], values...)
	}
	return query
}

// requestLogBody keeps JSON bodies readable without consuming stream bodies.
func requestLogBody(req *resty.Request) any {
	if req.Body == nil {
		if len(req.FormData) > 0 {
			return req.FormData
		}
		return nil
	}
	switch body := req.Body.(type) {
	case []byte:
		return decodeLogBytes(body)
	case json.RawMessage:
		return decodeLogBytes(body)
	case string:
		return decodeLogBytes([]byte(body))
	case io.Reader:
		return "<stream body>"
	default:
		return body
	}
}

// decodeLogBytes parses JSON for structured logging and keeps other text readable.
func decodeLogBytes(body []byte) any {
	var value any
	if json.Unmarshal(body, &value) == nil {
		return value
	}
	if utf8.Valid(body) {
		return string(body)
	}
	return "<binary body>"
}
