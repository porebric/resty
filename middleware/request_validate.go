package middleware

import (
	"context"
	"fmt"

	"github.com/porebric/resty/errors"
	"github.com/porebric/resty/requests"
)

type RequestValidate struct {
	next Middleware
}

func (r *RequestValidate) Execute(ctx context.Context, req requests.Request) (context.Context, int32, string) {
	valid, field, msg := req.Validate()
	if valid {
		return r.next.Execute(ctx, req)
	}
	if field == "" {
		return ctx, errors.ErrorInvalidRequest, ""
	}

	if msg == "" {
		msg = "invalid"
	}
	return ctx, errors.ErrorInvalidRequest, fmt.Sprintf("field %s: %s", field, msg)
}

func (r *RequestValidate) SetNext(next Middleware) {
	r.next = next
}
