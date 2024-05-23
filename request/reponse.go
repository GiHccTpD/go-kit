package request

import (
	"bytes"
	"net/http"
)

type Response struct {
	err         error
	Request     *Request
	rawResponse *http.Response
	Body        *bytes.Buffer
}

func (s *Response) GetBody() []byte {
	//resBody, _ := ioutil.ReadAll(s.rawResponse.Body)
	return s.Body.Bytes()
}

func (s *Response) GetErr() error {
	return s.err
}

func (s *Response) GetStatusCode() int {
	return s.rawResponse.StatusCode
}

func (s *Response) GetStatus() int {
	return s.rawResponse.StatusCode
}

func (s *Response) GetHeader() http.Header {
	return s.rawResponse.Header
}
