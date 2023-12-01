package middleware

import (
	"github.com/porebric/resty/errors"
	"github.com/porebric/resty/requests"
)

type Middleware interface {
	Execute(requests.Request) (int32, string)
	SetNext(Middleware)
	GetKey() string
}

type RequestCheck struct {
	next Middleware
}

func (r *RequestCheck) Execute(_ requests.Request) (int32, string) {
	return errors.ErrorNoError, ""
}

func (r *RequestCheck) SetNext(next Middleware) {
	r.next = next
}

func (r *RequestCheck) GetKey() string {
	return ""
}
