package ws

import (
	"context"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/porebric/logger"
)

// Handler handles WebSocket upgrade and connection lifecycle.
type Handler struct {
	logFn       func() *logger.Logger
	checkOrigin func(*http.Request) bool
}

// HandlerOption configures the WebSocket handler.
type HandlerOption func(*Handler)

// WithCheckOrigin sets the origin check for upgrades. If not set, all origins are allowed.
func WithCheckOrigin(fn func(*http.Request) bool) HandlerOption {
	return func(h *Handler) { h.checkOrigin = fn }
}

// NewHandler creates a new WebSocket handler.
func NewHandler(logFn func() *logger.Logger, opts ...HandlerOption) *Handler {
	h := &Handler{logFn: logFn}
	for _, o := range opts {
		o(h)
	}
	return h
}

func (h *Handler) upgrader() *websocket.Upgrader {
	checkOrigin := func(*http.Request) bool {
		return true
	}

	if h.checkOrigin != nil {
		checkOrigin = h.checkOrigin
	}

	return &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     checkOrigin,
	}
}

// ServeWs upgrades the request to WebSocket and runs the client loop (login-only).
// Key is validated before upgrade so 403 can be returned when appropriate.
func (h *Handler) ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	ctx := logger.ToContext(context.Background(), h.logFn())

	key := hub.keyFn(r)
	if key == "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	conn, err := h.upgrader().Upgrade(w, r, nil)
	if err != nil {
		logger.Warn(ctx, "websocket upgrade failed", "error", err, "client", key)
		return
	}

	c := newClient(ctx, hub, make(chan []byte, 256), conn, key)
	hub.register <- c

	go c.write()
	go c.read()
	go c.waitAuth()

	logger.Info(ctx, "new websocket client", "ip", r.RemoteAddr, "client", key)
}
