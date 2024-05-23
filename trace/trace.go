package trace

import (
	"encoding/json"
	"github.com/google/uuid"
	"strconv"
	"time"
)

type traceType string

const (
	H traceType = "http"
	S traceType = "sql"
	R traceType = "redis"
	C traceType = "client"
)

type TranInfo struct {
	TraceId string `json:"trace_id"`
	Span    string `json:"span"`
}

func (s *TranInfo) ToString() string {
	res, _ := json.Marshal(s)
	return string(res)
}

type LinkTrack struct {
	*TranInfo
	StartTime   int64       `json:"start_time"`
	Duration    int         `json:"duration"`
	Type        traceType   `json:"type"`
	Annotation  interface{} `json:"annotation"`
	ChildrenNum int         `json:"children_num"`
}

func (s *LinkTrack) SetDuration() {
	s.Duration = int(time.Now().UnixNano()/1e3 - s.StartTime)
	//logger.Debugf("Duration:%v", s.Duration)
}

func (s *LinkTrack) ToString() string {
	res, _ := json.Marshal(s)
	return string(res)
}

func (s *LinkTrack) SetType(chainType traceType) {
	s.Type = chainType
}

func (s *LinkTrack) SetAnnotation(annotation interface{}) {
	s.Annotation = annotation
}

func (s *LinkTrack) GenerateTranInfo() *TranInfo {
	res := new(TranInfo)
	res.TraceId = s.TraceId
	res.Span = s.Span + "." + strconv.Itoa(s.ChildrenNum)
	s.ChildrenNum++
	return res
}

func NewLickTrack(t traceType) *LinkTrack {
	res := &LinkTrack{}
	tranInfo := &TranInfo{}
	tranInfo.TraceId = uuid.New().String()
	tranInfo.Span = "Rn"
	res.TranInfo = tranInfo
	res.StartTime = time.Now().UnixNano() / 1e3
	res.Type = t
	return res
}

func NewLickTrackWithString(t traceType, parent string) (*LinkTrack, error) {
	res := &LinkTrack{}
	tranInfo := &TranInfo{}
	err := json.Unmarshal([]byte(parent), tranInfo)
	if err != nil {
		return res, err
	}
	res.Type = t
	res.TranInfo = tranInfo
	res.StartTime = time.Now().UnixNano() / 1e3
	return res, nil
}

func NewLickTrackWithTranInfo(t traceType, parent *TranInfo) *LinkTrack {
	res := &LinkTrack{}
	res.Type = t
	res.TranInfo = parent
	res.StartTime = time.Now().UnixNano() / 1e3
	return res
}
