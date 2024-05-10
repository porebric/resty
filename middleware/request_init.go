package middleware

import (
	"context"
	"net/http"

	"github.com/porebric/resty/errors"
	"github.com/porebric/resty/requests"
)

const KeyRequestInit = "request_init"

type RequestInit struct {
	next Middleware
	r    *http.Request
}

func NewRequestInit(r *http.Request) *RequestInit {
	return &RequestInit{r: r}
}

func (r *RequestInit) Execute(ctx context.Context, req requests.Request) (context.Context, int32, string) {
	if err := req.Set(r.r); err != nil {
		return ctx, errors.ErrorInvalidRequest, ""
	}

	return r.next.Execute(ctx, req)
}

func (r *RequestInit) SetNext(next Middleware) {
	r.next = next
}

func (r *RequestInit) GetKey() string {
	return KeyRequestInit
}
