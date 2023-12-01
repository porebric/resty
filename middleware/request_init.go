package middleware

import (
	"github.com/porebric/resty/errors"
	"github.com/porebric/resty/requests"
	"net/http"
)

const KeyRequestInit = "request_init"

type RequestInit struct {
	next Middleware
	r    *http.Request
}

func NewRequestInit(r *http.Request) *RequestInit {
	return &RequestInit{r: r}
}

func (r *RequestInit) Execute(req requests.Request) (int32, string) {
	if err := req.Set(r.r); err != nil {
		return errors.ErrorInvalidRequest, ""
	}

	return r.next.Execute(req)
}

func (r *RequestInit) SetNext(next Middleware) {
	r.next = next
}

func (r *RequestInit) GetKey() string {
	return KeyRequestInit
}
