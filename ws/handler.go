package ws

import (
	"context"
	"net/http"

	"github.com/porebric/logger"
)

type Handler struct {
	logFn func() *logger.Logger
}

func NewHandler(logFn func() *logger.Logger) *Handler {
	return &Handler{logFn: logFn}
}

func (h *Handler) ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		_ = conn.Close()
		return
	}

	ctx := logger.ToContext(context.Background(), h.logFn())

	key := hub.keyFn(r)
	if key == "" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	c := newClient(ctx, hub, make(chan []byte, 512), conn, key)

	c.hub.register <- c

	go c.write()
	go c.read()

	go c.waitAuth()

	logger.Info(ctx, "new client", "ip", r.RemoteAddr, "client", key)
}
