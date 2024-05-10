package middleware

import (
	"context"

	"github.com/porebric/resty/errors"
	"github.com/porebric/resty/requests"
)

type Middleware interface {
	Execute(context.Context, requests.Request) (context.Context, int32, string)
	SetNext(Middleware)
	GetKey() string
}

type RequestCheck struct {
	next Middleware
}

func (r *RequestCheck) Execute(ctx context.Context, _ requests.Request) (context.Context, int32, string) {
	return ctx, errors.ErrorNoError, ""
}

func (r *RequestCheck) SetNext(next Middleware) {
	r.next = next
}

func (r *RequestCheck) GetKey() string {
	return ""
}
