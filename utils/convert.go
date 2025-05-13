package utils

import (
	standardHttp "net/http"

	http "github.com/bogdanfinn/fhttp"
)

func ConvertFhttpRequestToHttpRequest(req *http.Request) *standardHttp.Request {
	standardReq, _ := standardHttp.NewRequest(req.Method, req.URL.String(), req.Body)
	standardReq.Header = make(standardHttp.Header)
	for key, values := range req.Header {
		for _, value := range values {
			standardReq.Header.Add(key, value)
		}
	}
	return standardReq
}
