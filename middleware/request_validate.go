package middleware

import (
	"fmt"
	"github.com/porebric/resty/errors"
	"github.com/porebric/resty/requests"
)

const KeyRequestValidate = "request_validate"

type RequestValidate struct {
	next Middleware
}

func (r *RequestValidate) Execute(req requests.Request) (int32, string) {
	valid, field, msg := req.Validate()
	if valid {
		return r.next.Execute(req)
	}
	if field == "" {
		return errors.ErrorInvalidRequest, ""
	}

	if msg == "" {
		msg = "invalid"
	}
	return errors.ErrorInvalidRequest, fmt.Sprintf("field %s: %s", field, msg)
}

func (r *RequestValidate) SetNext(next Middleware) {
	r.next = next
}

func (r *RequestValidate) GetKey() string {
	return KeyRequestValidate
}
