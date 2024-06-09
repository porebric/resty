package resty

import (
	"context"

	"github.com/porebric/resty/requests"
	"github.com/porebric/resty/responses"
)

var paths map[string]bool

type endpoint[R requests.Request] struct {
	middlewares map[string]bool
	action      func(ctx context.Context, req R) (responses.Response, int)

	method string
}
