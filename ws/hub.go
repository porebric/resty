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
	defer func() {
		if rec := recover(); rec != any(nil) {
			logger.Fatal(logger.ToContext(context.Background(), logFn()), fmt.Sprintf("ws run critical error: %v", rec), "stacktrace", string(debug.Stack()))
		}
	}()

	for {
		select {
		case registerClient := <-h.register:
			h.wg.Add(1)
			activeClients.Inc()
			h.clients[registerClient.key] = append(h.clients[registerClient.key], registerClient)

			if len(h.clients[registerClient.key]) > maxUserConnections {
				h.clients[registerClient.key][0].send(
					newError(MaxConnectionsPrefix, fmt.Sprintf("max connections %d", maxUserConnections), h.clients[registerClient.key][0].key).Msg(),
				)

				h.deleteClient(registerClient.key, 0)

				break
			}

			logger.Debug(registerClient.ctx, "register user", "key", registerClient.key)
		case unregisterClient := <-h.unregister:
			if cc, ok := h.clients[unregisterClient.key]; ok {
				for i, c := range cc {
					if c.uniqueKey == unregisterClient.uniqueKey {
						logger.Debug(unregisterClient.ctx, "unregister user", "key", unregisterClient.key)
						h.deleteClient(unregisterClient.key, i)
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
						c.kind = b.Kind
						logger.Debug(c.ctx, "user auth", "user_id", c.key, "kind", b.Kind)
						h.SendToClient(c.ctx, c.key, &c.uniqueKey, c.kind, []byte(fmt.Sprintf(`{"login": true, "kind": "%s"}`, b.Kind)))
					} else {
						c.send(err.Msg())
					}
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

	deletedClients := make(map[string][]*client, len(h.clients))
	for k, clientList := range h.clients {
		deletedClients[k] = make([]*client, 0, len(clientList))
		for _, c := range clientList {
			deletedClients[k] = append(deletedClients[k], c)
		}
	}

	for _, clientList := range deletedClients {

		for _, c := range clientList {
			h.unregister <- c
		}
	}

	h.wg.Wait()

	return nil
}

func (h *Hub) SendToClient(ctx context.Context, key string, uuid *uuid.UUID, kind string, body []byte) {
	cc, ok := h.clients[key]
	if !ok || len(cc) == 0 {
		logger.Warn(ctx, "invalid user id for message", "user", key)
		return
	}

	for _, c := range cc {
		if uuid != nil && c.uniqueKey != *uuid {
			continue
		}

		if c.kind == kind {
			c.send(body)
		}
	}

	return
}

func (h *Hub) deleteClient(key string, i int) {
	close(h.clients[key][0].sendCh)
	h.clients[key] = slices.Delete(h.clients[key], i, i+1)
	activeClients.Dec()
	h.wg.Done()
}
