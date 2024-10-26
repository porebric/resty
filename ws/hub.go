package ws

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"slices"
	"sync"

	"github.com/google/uuid"
	"github.com/porebric/logger"
	"github.com/porebric/resty/ws/login"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const maxUserConnections = 10

var activeClients = promauto.NewGauge(
	prometheus.GaugeOpts{
		Name: "active_clients_total",
		Help: "The total number of active clients",
	},
)

type Hub struct {
	clients    map[string][]*client
	broadcast  chan Broadcast
	register   chan *client
	unregister chan *client
	handleFn   func(ctx context.Context, broadcast Broadcast) Error
	loginFn    func(ctx context.Context, broadcast *login.Broadcast) (context.Context, Error)
	broadcasts map[string]Broadcast
	keyFn      func(r *http.Request) string
	wg         sync.WaitGroup
	isClosed   bool
}

func NewHub(
	handleFn func(context.Context, Broadcast) Error,
	loginFn func(ctx context.Context, broadcast *login.Broadcast) (context.Context, Error),
	broadcasts map[string]Broadcast,
	keyFn func(r *http.Request) string,
) *Hub {
	broadcasts[login.Action] = new(login.Broadcast)

	hub := &Hub{
		broadcast:  make(chan Broadcast),
		register:   make(chan *client),
		unregister: make(chan *client),
		clients:    make(map[string][]*client),
		handleFn:   handleFn,
		loginFn:    loginFn,
		broadcasts: broadcasts,
		keyFn:      keyFn,
	}

	return hub
}

func (h *Hub) Run(logFn func() *logger.Logger) {
	for {
		select {
		case registerClient := <-h.register:
			h.wg.Add(1)
			activeClients.Inc()
			h.clients[registerClient.key] = append(h.clients[registerClient.key], registerClient)

			if len(h.clients[registerClient.key]) > maxUserConnections {
				h.clients[registerClient.key][0].send(newError(MaxConnectionsPrefix, fmt.Sprintf("max connections %d", maxUserConnections), h.clients[registerClient.key][0].key).Msg())
				logger.Debug(h.clients[registerClient.key][0].ctx, "unregister user", "key", h.clients[registerClient.key][0].key)
				h.deleteClient(logFn, h.clients[registerClient.key][0].key, 0)
				break
			}

			logger.Debug(registerClient.ctx, "register user", "key", registerClient.key)

		case unregisterClient := <-h.unregister:
			if cc, ok := h.clients[unregisterClient.key]; ok {
				for i, c := range cc {
					if c.uniqueKey == unregisterClient.uniqueKey {
						logger.Debug(unregisterClient.ctx, "unregister user", "key", unregisterClient.key)
						h.deleteClient(logFn, unregisterClient.key, i)
						break
					}
				}

				if len(cc) == 0 {
					delete(h.clients, unregisterClient.key)
				}
			}

		case broadcast := <-h.broadcast:
			if broadcast == nil {
				continue
			}

			clients, ok := h.clients[broadcast.GetKey()]
			if !ok {
				continue
			}

			for _, c := range clients {
				if c.uniqueKey != broadcast.GetUuid() {
					continue
				}

				logger.Debug(c.ctx, "get message", "user_id", c.key, "body", broadcast)

				if b, isLogin := broadcast.(*login.Broadcast); isLogin {
					var err Error
					if c.ctx, err = h.loginFn(c.ctx, b); err.Code == "" {
						if _, actionOk := c.hub.broadcasts[b.Action]; !actionOk {
							c.send(newError(InvalidMsgPrefix, "invalid action", c.key).Msg())
							break
						}

						c.action = b.Action
						h.SendToClient(c.ctx, c.key, &c.uniqueKey, c.action, []byte(fmt.Sprintf(`{"login": true, "action": "%s"}`, b.Action)))
					} else {
						c.send(err.Msg())
					}
					break
				}

				if c.action == "" {
					c.send(newError(AuthPrefix, "not auth", c.key).Msg())
					break
				}

				if err := h.handleFn(c.ctx, broadcast); err.Code != "" {
					c.send(err.Msg())
				}
				break
			}
		}
	}
}

func (h *Hub) Close(_ context.Context) error {
	h.isClosed = true

	for _, clientList := range h.clients {
		for _, c := range clientList {
			h.unregister <- c
		}
	}

	h.wg.Wait()
	return nil
}

func (h *Hub) SendToClient(ctx context.Context, key string, uuid *uuid.UUID, action string, body []byte) {
	cc, ok := h.clients[key]
	if !ok || len(cc) == 0 {
		logger.Warn(ctx, "invalid user id for message", "user", key)
		return
	}

	for _, c := range cc {
		if uuid != nil && c.uniqueKey != *uuid {
			continue
		}

		if c.action == action {
			c.send(body)
		}
	}
}

func (h *Hub) deleteClient(logFn func() *logger.Logger, key string, i int) {
	defer func() {
		if r := recover(); r != nil {
			logFn().Error(fmt.Errorf("%v", r), "delete client", "key", key, "stacktrace", string(debug.Stack()))
		}
	}()
	close(h.clients[key][i].sendCh)
	h.clients[key] = slices.Delete(h.clients[key], i, i+1)
	activeClients.Dec()
	h.wg.Done()
}
