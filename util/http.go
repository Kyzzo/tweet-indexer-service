package util

import (
	"encoding/json"
	"net/http"
	"time"
)

var myClient = &http.Client{Timeout: 10 * time.Second}

type HttpResponse[T any] struct {
	Msg      string `json:"msg"`
	HttpCode int    `json:"http_code"`
	Success  bool   `json:"success"`
	Data     T      `json:"data"`
}

func HttpOK[T any](msg string, data T) HttpResponse[T] {
	return HttpResponse[T]{
		Msg:      msg,
		HttpCode: 200,
		Success:  true,
		Data:     data,
	}
}

func HttpCreated[T any](msg string, data T) HttpResponse[T] {
	return HttpResponse[T]{
		Msg:      msg,
		HttpCode: 201,
		Success:  true,
		Data:     data,
	}
}

func HttpNotFound[T any](msg string, data T) HttpResponse[T] {
	return HttpResponse[T]{
		Msg:      msg,
		HttpCode: 404,
		Success:  false,
		Data:     data,
	}
}

func HttpInternalServerError[T any](data T) HttpResponse[T] {
	return HttpResponse[T]{
		Msg:      "Internal server error :(",
		HttpCode: 500,
		Success:  false,
		Data:     data,
	}
}

func HttpBadRequestError[T any](data T) HttpResponse[T] {
	return HttpResponse[T]{
		Msg:      "Bad Request :(",
		HttpCode: 400,
		Success:  false,
		Data:     data,
	}
}

func HttpConflict[T any](data T) HttpResponse[T] {
	return HttpResponse[T]{
		Msg:      "Conflict",
		HttpCode: 409,
		Success:  false,
		Data:     data,
	}
}

func HttpUnauthorized[T any](data T) HttpResponse[T] {
	return HttpResponse[T]{
		Msg:      "Unauthorized :x",
		HttpCode: 401,
		Success:  false,
		Data:     data,
	}
}

func GetJSON(url string, target interface{}) error {
	r, err := myClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

func GetJSONWithAgent(url string, target interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header = http.Header{
		"User-Agent": {req.UserAgent()},
	}

	r, err := myClient.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}
