package ws

import (
	"context"
	"fmt"
	"net/http"
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
	handleFn   func(ctx context.Context, broadcast Broadcast) (map[string]string, Error)
	loginFn    func(ctx context.Context, broadcast *login.Broadcast) (context.Context, Error)
	broadcasts map[string]func() Broadcast
	keyFn      func(r *http.Request) string
	mu         sync.Mutex
}

func NewHub(
	handleFn func(context.Context, Broadcast) (map[string]string, Error),
	loginFn func(ctx context.Context, broadcast *login.Broadcast) (context.Context, Error),
	broadcasts map[string]func() Broadcast,
	keyFn func(r *http.Request) string,
) *Hub {
	broadcasts[login.Action] = func() Broadcast {
		return new(login.Broadcast)
	}

	hub := &Hub{
		broadcast:  make(chan Broadcast, 1024),
		register:   make(chan *client, 1024),
		unregister: make(chan *client, 1024),
		clients:    make(map[string][]*client),
		handleFn:   handleFn,
		loginFn:    loginFn,
		broadcasts: broadcasts,
		keyFn:      keyFn,
		mu:         sync.Mutex{},
	}

	return hub
}

func (h *Hub) Run() {
	for {
		select {
		case registerClient := <-h.register:
			h.doRegister(registerClient)
		case unregisterClient := <-h.unregister:
			h.doUnRegister(unregisterClient)
		case broadcast := <-h.broadcast:
			h.handleBroadcast(broadcast)
		}
	}
}

func (h *Hub) doUnRegister(client *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	cc, ok := h.clients[client.key]
	if !ok {
		return
	}

	logger.Debug(client.ctx, "unregister user", "uuid", client.uuid, "user", client.key)

	for i, c := range cc {
		if c.uuid == client.uuid {
			client.safeClose()
			h.clients[client.key] = slices.Delete(h.clients[client.key], i, i+1)
			activeClients.Dec()
			break
		}
	}

	if len(h.clients[client.key]) == 0 {
		delete(h.clients, client.key)
		logger.Debug(client.ctx, "delete user", "user", client.key)
	}
}

func (h *Hub) doRegister(client *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client.key] = append(h.clients[client.key], client)
	activeClients.Inc()

	logger.Debug(client.ctx, "register user", "uuid", client.uuid, "user", client.key)

	if len(h.clients[client.key]) >= maxUserConnections+1 {
		client.send(newError(MaxConnectionsPrefix, fmt.Sprintf("max connections %d", maxUserConnections), client.key).Msg())
		client.send(nil)
	}
}

func (h *Hub) handleBroadcast(broadcast Broadcast) {
	h.mu.Lock()

	if broadcast == nil {
		h.mu.Unlock()
		return
	}

	clients, ok := h.clients[broadcast.GetKey()]
	if !ok {
		h.mu.Unlock()
		return
	}

	var currentClient *client

	for _, c := range clients {
		if c.uuid == broadcast.GetUuid() {
			currentClient = c
			break
		}
	}

	h.mu.Unlock()

	if currentClient == nil {
		return
	}

	logger.Debug(currentClient.ctx, "get message", "uuid", currentClient.uuid, "user", currentClient.key, "body", broadcast)

	if b, isLogin := broadcast.(*login.Broadcast); isLogin {
		var err Error
		if currentClient.ctx, err = h.loginFn(currentClient.ctx, b); err.Code == "" {
			if _, actionOk := currentClient.hub.broadcasts[b.Action]; !actionOk {
				currentClient.send(newError(InvalidMsgPrefix, "invalid action", currentClient.key).Msg())
				return
			}

			currentClient.action = b.Action
			h.SendToClient(
				currentClient.ctx,
				currentClient.key,
				&currentClient.uuid,
				currentClient.action,
				[]byte(fmt.Sprintf(`{"login": true, "action": "%s", "uuid": "%s"}`, b.Action, currentClient.uuid)),
			)
		} else {
			currentClient.send(err.Msg())
		}
		return
	}

	if currentClient.action == "" {
		currentClient.send(newError(AuthPrefix, "not auth", currentClient.key).Msg())
		return
	}

	additional, err := h.handleFn(currentClient.ctx, broadcast)
	if err.Code != "" {
		currentClient.send(err.Msg())
	}

	h.mu.Lock()
	currentClient.additional = additional
	h.mu.Unlock()
}

func (h *Hub) SendToClient(ctx context.Context, key string, uuid *uuid.UUID, action string, body []byte, additional ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	uid := ""
	if uuid != nil {
		uid = uuid.String()
	}

	logger.Debug(ctx, "get response for client", "uuid", uid, "user", key)

	cc, ok := h.clients[key]

	if !ok || len(cc) == 0 {
		logger.Warn(ctx, "invalid user id for message", "uuid", uid, "user", key)
		return
	}

	for _, c := range cc {
		if uuid != nil && c.uuid != *uuid {
			continue
		}

		if c.action == action || action == "" {
			if len(additional) == 0 {
				c.send(body)
				continue
			}

			send := false
			for i := 0; i < len(additional); i += 2 {
				if i+1 >= len(additional) {
					break
				}
				additionalKey, additionalValue := additional[i], additional[i+1]
				if val, exists := c.additional[additionalKey]; exists && val == additionalValue {
					send = true
					break
				}
			}

			if send {
				c.send(body)
			} else {
				logger.Debug(ctx, "client map does not match additional parameters", "uuid", uid, "user", key, "additional", additional)
			}
		}
	}
}
