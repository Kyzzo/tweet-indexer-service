package connection

import "errors"

var (
	ErrHttpRequest = errors.New("error on registry tier http request")
	ErrDecodeResp  = errors.New("error decoding registry tier response")
)
