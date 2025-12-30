package ws

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"

	"github.com/google/uuid"
	"github.com/porebric/logger"
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
	loginMsgCh chan *LoginMessage
	register   chan *client
	unregister chan *client
	loginFn    func(context.Context, *LoginMessage) (context.Context, Error)
	keyFn      func(r *http.Request) string
	mu         sync.RWMutex
}

func NewHub(loginFn func(ctx context.Context, broadcast *LoginMessage) (context.Context, Error), keyFn func(r *http.Request) string) *Hub {
	hub := &Hub{
		loginMsgCh: make(chan *LoginMessage, 1024),
		register:   make(chan *client, 1024),
		unregister: make(chan *client, 1024),
		clients:    make(map[string][]*client),
		loginFn:    loginFn,
		keyFn:      keyFn,
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
		case broadcast := <-h.loginMsgCh:
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

func (h *Hub) handleBroadcast(loginMsg *LoginMessage) {
	if loginMsg == nil {
		return
	}

	h.mu.RLock()
	clients, ok := h.clients[loginMsg.GetKey()]
	if !ok {
		h.mu.RUnlock()
		return
	}

	// Находим клиента под RLock
	var currentClient *client
	for _, c := range clients {
		if c.uuid == loginMsg.GetUuid() {
			currentClient = c
			break
		}
	}

	if currentClient == nil {
		h.mu.RUnlock()
		return
	}

	clientRef := currentClient
	h.mu.RUnlock()

	logger.Debug(clientRef.ctx, "get message", "uuid", clientRef.uuid, "user", clientRef.key, "body", loginMsg)

	var err Error
	ctx, err := h.loginFn(clientRef.ctx, loginMsg)

	if err.Code == "" {
		clientRef.auth.Store(true)

		clientRef.hub.mu.Lock()
		clientRef.actions = loginMsg.Actions
		clientRef.hub.mu.Unlock()

		h.SendToClient(
			ctx,
			clientRef.key,
			&clientRef.uuid,
			[]byte(fmt.Sprintf(`{"login": true, "uuid": "%s"}`, clientRef.uuid)),
		)

		return
	}

	if !clientRef.auth.Load() {
		clientRef.send(newError(AuthPrefix, "not auth", clientRef.key).Msg())
	}
}

func (h *Hub) SendToClient(ctx context.Context, key string, uuid *uuid.UUID, body []byte, availableActions ...string) {
	if key == "" {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	cc, ok := h.clients[key]
	if !ok || len(cc) == 0 {
		logger.Warn(ctx, "invalid user id for message", "user", key)
		return
	}

	uid := ""
	if uuid != nil {
		uid = uuid.String()
	}

	logger.Debug(ctx, "get response for client", "uuid", uid, "user", key)

	for _, c := range cc {
		if uuid != nil && c.uuid != *uuid {
			continue
		}

		if len(availableActions) == 0 {
			c.send(body)
			continue
		}

		send := true
		for _, availableAction := range availableActions {
			if !slices.Contains(c.actions, availableAction) {
				send = false
				break
			}
		}

		if !send {
			logger.Debug(ctx, "client map does not match actions parameters",
				"uuid", c.uuid.String(), "user", key,
				"actions", availableActions)
			continue
		}

		c.send(body)
	}
}

func (h *Hub) AddActionToClients(key, action string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, c := range h.clients[key] {
		c.actions = append(c.actions, action)
	}
}

func (h *Hub) AddActionToClient(key, action string, id uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, c := range h.clients[key] {
		if c.uuid == id {
			c.actions = append(c.actions, action)
		}
	}
}
