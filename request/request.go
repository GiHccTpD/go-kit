package request

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/GiHccTpD/go-kit/logger/v3"
	"github.com/GiHccTpD/go-kit/sugar"
	"github.com/GiHccTpD/go-kit/trace"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	hdrUserAgentKey       = http.CanonicalHeaderKey("User-Agent")
	hdrAcceptKey          = http.CanonicalHeaderKey("Accept")
	hdrContentTypeKey     = http.CanonicalHeaderKey("Content-Type")
	hdrContentLengthKey   = http.CanonicalHeaderKey("Content-Length")
	hdrContentEncodingKey = http.CanonicalHeaderKey("Content-Encoding")
	hdrLocationKey        = http.CanonicalHeaderKey("Location")

	plainTextType   = "text/plain; charset=utf-8"
	jsonContentType = "application/json"
	formContentType = "application/x-www-form-urlencoded"

	jsonCheck = regexp.MustCompile(`(?i:(application|text)/(json|.*\+json|json\-.*)(;|$))`)
	bufPool   = &sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}
)

type Request struct {
	URL              string
	Method           string
	Header           http.Header
	QueryParam       url.Values
	Body             interface{}
	FormData         url.Values
	responseWriter   io.Writer
	pathParams       map[string]string
	rawRequest       *http.Request
	bodyBuf          *bytes.Buffer
	setContentLength bool
	ctx              context.Context
	timeOut          time.Duration
	traceInfo        *TraceInfo
	linkTrance       *trace.LinkTrack
	isNotKeepAlive   bool
}

func New() *Request {
	return &Request{
		URL:              "",
		Method:           "",
		QueryParam:       url.Values{},
		pathParams:       map[string]string{},
		Header:           http.Header{},
		Body:             nil,
		FormData:         url.Values{},
		bodyBuf:          nil,
		setContentLength: false,
		rawRequest:       nil,
		ctx:              nil,
		timeOut:          0,
		traceInfo:        nil,
		isNotKeepAlive:   false,
	}
}

func (s *Request) SetHeader(header, value string) *Request {
	s.Header.Set(header, value)
	return s
}

func (s *Request) SetHeaders(headers map[string]string) *Request {
	for h, v := range headers {
		s.Header.Set(h, v)
	}
	return s
}

func (s *Request) SetQueryParam(param, value string) *Request {
	s.QueryParam.Set(param, value)
	return s
}

func (s *Request) SetQueryParams(params map[string]string) *Request {
	for p, v := range params {
		s.SetQueryParam(p, v)
	}
	return s
}

func (s *Request) SetPathParam(param, value string) *Request {
	s.pathParams[param] = value
	return s
}

func (s *Request) SetPathParams(params map[string]string) *Request {
	for p, v := range params {
		s.SetPathParam(p, v)
	}
	return s
}

func (s *Request) SetContentLength(contentLength string) *Request {
	s.setContentLength = true
	s.SetHeader(hdrContentLengthKey, contentLength)
	return s
}

func (s *Request) SetFormData(data map[string]string) *Request {
	for k, v := range data {
		s.FormData.Set(k, v)
	}
	return s
}

func (s *Request) SetFormDataFromValues(data url.Values) *Request {
	for k, v := range data {
		for _, kv := range v {
			s.FormData.Add(k, kv)
		}
	}
	return s
}

func (s *Request) SetBody(body interface{}) *Request {
	s.Body = body
	return s
}

func (s *Request) SetTimeout(timeout time.Duration) *Request {
	s.timeOut = timeout
	return s
}

func (s *Request) IsNotKeepAlive(isNotKeepAlive bool) *Request {
	s.isNotKeepAlive = isNotKeepAlive
	return s
}

func (s *Request) SetTrace(parent *trace.TranInfo) *Request {
	s.traceInfo = &TraceInfo{}
	s.linkTrance = trace.NewLickTrackWithTranInfo(trace.C, parent)
	return s
}

func (s *Request) SetResponseWriter(writer io.Writer) *Request {
	s.responseWriter = writer
	return s
}

func (s *Request) isTrace() bool {
	return s.traceInfo != nil
}

func (s *Request) doTrace(resp *Response) {

	if !s.isTrace() {
		return
	}

	s.linkTrance.SetDuration()
	traceInfo := s.traceInfo
	traceInfo.Url = s.URL
	reqHeader := make(map[string]string)
	for key, _ := range s.rawRequest.Header {
		reqHeader[key] = s.rawRequest.Header.Get(key)
	}
	traceInfo.ReqHeader = reqHeader

	resHeader := make(map[string]string)
	for key, _ := range resp.rawResponse.Header {
		resHeader[key] = resp.rawResponse.Header.Get(key)
	}
	traceInfo.ResHeader = resHeader

	traceInfo.StatusCode = resp.GetStatusCode()
	traceInfo.Method = s.rawRequest.Method

	if resp.GetErr() == nil {
		if resp.rawResponse.ContentLength != -1 && resp.rawResponse.ContentLength <= 256*1024 {
			resBodyByte, _ := ioutil.ReadAll(resp.Body)
			traceInfo.ResBody = string(resBodyByte)
		} else {
			traceInfo.ResBody = fmt.Sprintf("response body too large length = %d", resp.rawResponse.ContentLength)
		}
	} else {
		traceInfo.ResBody = resp.GetErr().Error()
	}
	s.linkTrance.SetAnnotation(traceInfo)
}

func (s *Request) Get(url string) *Response {
	return s.execute(http.MethodGet, url)
}
func (s *Request) Head(url string) *Response {
	return s.execute(http.MethodHead, url)
}
func (s *Request) Post(url string) *Response {
	return s.execute(http.MethodPost, url)
}
func (s *Request) Put(url string) *Response {
	return s.execute(http.MethodPut, url)
}
func (s *Request) Delete(url string) *Response {
	return s.execute(http.MethodDelete, url)
}
func (s *Request) Options(url string) *Response {
	return s.execute(http.MethodOptions, url)
}
func (s *Request) Patch(url string) *Response {
	return s.execute(http.MethodPatch, url)
}

func (s *Request) execute(method, url string) *Response {
	resp := new(Response)
	resp.Request = s
	s.Method = method

	defer releaseBuffer(s.bodyBuf)
	s.parseRequestURL(url)
	resp.err = s.parseRequestBody()
	if resp.err != nil {
		resp.rawResponse.StatusCode = 500
		return resp
	}
	resp.err = s.createHTTPRequest()
	if resp.err != nil {
		resp.rawResponse.StatusCode = 500
		return resp
	}
	s.parseRequestHeader()
	if s.timeOut > 0 {
		ctx, cancel, err := getContextByTimeOut(s.timeOut)
		if err != nil {
			resp.err = err
			return resp
		}
		defer cancel()
		s.rawRequest = s.rawRequest.WithContext(ctx)
	}
	log.Debugw("execute", zap.String("method", method),
		zap.String("url", s.URL),
		zap.Any("header", s.Header),
		zap.Any("body", sugar.IfExpress(s.bodyBuf == nil, func() interface{} {
			return ""
		}, func() interface{} {
			return string(s.bodyBuf.Bytes())
		})), zap.String("timeout", s.timeOut.String()))
	resp.rawResponse, resp.err = httpClient.Do(s.rawRequest)
	if resp.err == nil {
		defer func() {
			closeQuiet(resp.rawResponse.Body)
		}()
		if s.responseWriter != nil {
			_, err := io.Copy(s.responseWriter, resp.rawResponse.Body)
			if err != nil {
				resp.err = err
				return resp
			}
			resp.Body = bytes.NewBuffer(make([]byte, 0))
		} else {
			//if resp.rawResponse.ContentLength >= 0 {
			bodyData, err := ioutil.ReadAll(resp.rawResponse.Body)
			if err != nil {
				resp.err = err
				return resp
			}
			bodyBuf := bytes.NewBuffer(bodyData)
			resp.Body = bodyBuf
			//}
		}
	}
	s.doTrace(resp)
	return resp
}

func getContextByTimeOut(timeOut time.Duration) (context.Context, context.CancelFunc, error) {
	log.Debugw("接口超时时间", zap.Any("timeOut", timeOut))
	ctx, cancel := context.WithTimeout(context.Background(), timeOut)
	return ctx, cancel, nil
}

func (s *Request) createHTTPRequest() (err error) {

	if s.bodyBuf == nil {
		if reader, ok := s.Body.(io.Reader); ok {
			s.rawRequest, err = http.NewRequest(s.Method, s.URL, reader)
		} else if s.setContentLength {
			s.rawRequest, err = http.NewRequest(s.Method, s.URL, http.NoBody)
		} else {
			s.rawRequest, err = http.NewRequest(s.Method, s.URL, nil)
		}
	} else {
		s.rawRequest, err = http.NewRequest(s.Method, s.URL, s.bodyBuf)
	}
	s.rawRequest.Close = s.isNotKeepAlive

	return
}

func (s *Request) parseRequestBody() error {

	if len(s.FormData) > 0 {
		_ = s.handleFormData()
		goto CL
	}

	if s.Body != nil {
		s.handleContentType()

		if err := s.handleRequestBody(); err != nil {
			return err
		}
	}

CL:
	if s.setContentLength && s.bodyBuf != nil {
		s.Header.Set(hdrContentLengthKey, fmt.Sprintf("%d", s.bodyBuf.Len()))
	}
	return nil

}

func (s *Request) handleRequestBody() error {
	var bodyBytes []byte
	var err error
	contentType := s.Header.Get(hdrContentTypeKey)
	kind := reflect.Indirect(reflect.ValueOf(s.Body)).Type().Kind()
	s.bodyBuf = nil

	if _, ok := s.Body.(io.Reader); ok {
		s.traceInfo.ReqBody = "body is reader type"
		return nil
	} else if b, ok := s.Body.([]byte); ok {
		bodyBytes = b
	} else if str, ok := s.Body.(string); ok {
		bodyBytes = []byte(str)
	} else if isJSONType(contentType) &&
		(kind == reflect.Struct || kind == reflect.Map || kind == reflect.Slice) {

		bodyBytes, err = json.Marshal(s.Body)
		if err != nil {
			return err
		}
	}

	if bodyBytes == nil && s.bodyBuf == nil {
		err = errors.New("unsupported 'Body' type/value")
	}

	// if any errors during body bytes handling, return it
	if err != nil {
		return err
	}

	// []byte into Buffer
	if bodyBytes != nil && s.bodyBuf == nil {
		s.bodyBuf = acquireBuffer()
		_, _ = s.bodyBuf.Write(bodyBytes)
		if s.isTrace() {
			s.traceInfo.ReqBody = string(bodyBytes)
		}
	}

	return err
}

func (s *Request) handleFormData() error {
	formData := url.Values{}
	for k, v := range s.FormData {
		for _, iv := range v {
			formData.Add(k, iv)
		}
	}
	s.bodyBuf = bytes.NewBuffer([]byte(formData.Encode()))
	s.Header.Set(hdrContentTypeKey, formContentType)
	if s.isTrace() {
		s.traceInfo.ReqBody = formData.Encode()
	}
	return nil
}

func (s *Request) parseRequestHeader() {
	s.rawRequest.Header = s.Header
}

func (s *Request) parseRequestURL(priUrl string) string {

	if len(s.pathParams) > 0 {
		for p, v := range s.pathParams {
			priUrl = strings.Replace(priUrl, "{"+p+"}", url.PathEscape(v), -1)
		}
	}

	query := make(url.Values)
	for k, v := range s.QueryParam {
		for _, iv := range v {
			query.Add(k, iv)
		}
	}

	if len(query) > 0 {
		if strings.Contains(priUrl, "?") {
			priUrl = priUrl + query.Encode()
		} else {
			priUrl = priUrl + "?" + query.Encode()
		}
	}
	s.URL = priUrl
	return priUrl

}

func (s *Request) handleContentType() {
	contentType := s.Header.Get(hdrContentTypeKey)
	if IsEmpty(contentType) {
		contentType = DetectContentType(s.Body)
		s.Header.Set(hdrContentTypeKey, contentType)
	}
}

func DetectContentType(body interface{}) string {
	contentType := plainTextType
	kind := reflect.Indirect(reflect.ValueOf(body)).Type().Kind()
	switch kind {
	case reflect.Struct, reflect.Map:
		contentType = jsonContentType
	case reflect.String:
		contentType = plainTextType
	default:
		if b, ok := body.([]byte); ok {
			contentType = http.DetectContentType(b)
		} else if kind == reflect.Slice {
			contentType = jsonContentType
		}
	}

	return contentType
}

func acquireBuffer() *bytes.Buffer {
	return bufPool.Get().(*bytes.Buffer)
}

func releaseBuffer(buf *bytes.Buffer) {
	if buf != nil {
		buf.Reset()
		bufPool.Put(buf)
	}
}

func getBodyCopy(r *Request) (*bytes.Buffer, error) {
	// If r.bodyBuf present, return the copy
	if r.bodyBuf != nil {
		return bytes.NewBuffer(r.bodyBuf.Bytes()), nil
	}

	// Maybe body is `io.Reader`.
	// Note: Resty user have to watchout for large body size of `io.Reader`
	if r.rawRequest.Body != nil {
		b, err := ioutil.ReadAll(r.rawRequest.Body)
		if err != nil {
			return nil, err
		}

		// Restore the Body
		closeQuiet(r.rawRequest.Body)
		r.rawRequest.Body = ioutil.NopCloser(bytes.NewBuffer(b))

		// Return the Body bytes
		return bytes.NewBuffer(b), nil
	}
	return nil, nil
}

func closeQuiet(v interface{}) {
	if c, ok := v.(io.Closer); ok {
		silently(c.Close())
	}
}

func silently(_ ...interface{}) {}

func isJSONType(ct string) bool {
	return jsonCheck.MatchString(ct)
}

func IsEmpty(s ...string) bool {
	for _, str := range s {
		if !(len(str) > 0) {
			return true
		}
	}
	return false
}
