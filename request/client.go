package request

import (
	"crypto/tls"
	"net/http"
	"time"
)

var httpClient *http.Client

func init() {
	InitHttpClientWithTimeOutAndCert(time.Minute*5, nil)
}

func InitHttpClientWithTimeOutAndCert(timeOutDuration time.Duration, certs []tls.Certificate) {
	var tr *http.Transport
	if certs != nil {
		tr = &http.Transport{
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true, Certificates: certs},
			ResponseHeaderTimeout: timeOutDuration,
		}
	} else {
		tr = &http.Transport{
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			ResponseHeaderTimeout: timeOutDuration,
		}
	}

	//cookieJar, _ := cookiejar.New(nil)
	httpClient = &http.Client{
		Transport: tr,
		//Timeout:   time.Minute*5,
		//CheckRedirect: func(req *http.Request, via []*http.Request) error {
		//    // return nil
		//    //},
		//    //
	}
}
