package requests

import (
	"context"
	"net/http"
)

type Request interface {
	Validate() (bool, string, string)
	Set(ctx context.Context, r *http.Request) (context.Context, error)
	Middlewares() map[string]bool
}
