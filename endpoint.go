package resty

import (
	"context"
	"strings"
	"sync"

	"github.com/porebric/resty/requests"
	"github.com/porebric/resty/responses"
)

type endpointsCollector struct {
	mu sync.Mutex

	endpoints map[string]endpoint
}

func (c *endpointsCollector) Get(method, path string) {
	c.mu.Lock()
	c.mu.Unlock()

}

type endpoint struct {
	middlewares map[string]bool
	action      func(ctx context.Context, req requests.Request) (responses.Response, int)
	request     func() requests.Request
}

func generateKey(method, path string) string {
	var b strings.Builder
	b.WriteString(method)
	b.WriteString("_")
	b.WriteString(path)

	return b.String()
}
