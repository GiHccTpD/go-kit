package request

type TraceInfo struct {
	Url        string            `json:"url"`
	StatusCode int               `json:"status_code"`
	ResHeader  map[string]string `json:"res_header"`
	ResBody    string            `json:"res_body"`
	ReqHeader  map[string]string `json:"req_header"`
	ReqBody    string            `json:"req_body"`
	Method     string            `json:"method"`
}
