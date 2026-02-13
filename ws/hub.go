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

// Run processes register, unregister, and login messages. Login handling runs
// in a goroutine so a slow loginFn does not block the hub.
func (h *Hub) Run() {
	for {
		select {
		case registerClient := <-h.register:
			h.doRegister(registerClient)
		case unregisterClient := <-h.unregister:
			h.doUnRegister(unregisterClient)
		case loginMsg := <-h.loginMsgCh:
			go h.handleBroadcast(loginMsg)
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

func (h *Hub) doRegister(c *client) {
	h.mu.Lock()
	if len(h.clients[c.key]) >= maxUserConnections {
		h.mu.Unlock()
		c.send(newError(MaxConnectionsPrefix, fmt.Sprintf("max connections %d", maxUserConnections), c.key).Msg())
		c.safeClose()
		h.unregister <- c
		return
	}
	h.clients[c.key] = append(h.clients[c.key], c)
	activeClients.Inc()
	h.mu.Unlock()

	logger.Debug(c.ctx, "register user", "uuid", c.uuid, "user", c.key)
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
		clientRef.actionsMu.Lock()
		clientRef.actions = loginMsg.Actions
		clientRef.actionsMu.Unlock()

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

// SendToClient sends body to clients for key (and optional uuid). Does not hold
// the hub lock during send so message transmission is not blocked by register/unregister.
func (h *Hub) SendToClient(ctx context.Context, key string, uuid *uuid.UUID, body []byte, availableActions ...string) {
	if key == "" {
		return
	}

	h.mu.RLock()
	cc, ok := h.clients[key]
	if !ok || len(cc) == 0 {
		h.mu.RUnlock()
		logger.Warn(ctx, "invalid user id for message", "user", key)
		return
	}

	// Build list of clients to send to while holding hub RLock and each client's actionsMu.
	toSend := make([]*client, 0, len(cc))
	for _, c := range cc {
		if uuid != nil && c.uuid != *uuid {
			continue
		}
		if len(availableActions) == 0 {
			toSend = append(toSend, c)
			continue
		}
		c.actionsMu.RLock()
		match := true
		for _, a := range availableActions {
			if !slices.Contains(c.actions, a) {
				match = false
				break
			}
		}
		c.actionsMu.RUnlock()
		if match {
			toSend = append(toSend, c)
		}
	}
	h.mu.RUnlock()

	uid := ""
	if uuid != nil {
		uid = uuid.String()
	}
	logger.Debug(ctx, "get response for client", "uuid", uid, "user", key)

	for _, c := range toSend {
		c.send(body)
	}
}

func (h *Hub) AddActionToClients(key, action string) {
	h.mu.RLock()
	cc := make([]*client, len(h.clients[key]))
	copy(cc, h.clients[key])
	h.mu.RUnlock()

	for _, c := range cc {
		c.actionsMu.Lock()
		c.actions = append(c.actions, action)
		c.actionsMu.Unlock()
	}
}

func (h *Hub) AddActionToClient(key, action string, id uuid.UUID) {
	h.mu.RLock()
	cc := make([]*client, len(h.clients[key]))
	copy(cc, h.clients[key])
	h.mu.RUnlock()

	for _, c := range cc {
		if c.uuid != id {
			continue
		}
		c.actionsMu.Lock()
		c.actions = append(c.actions, action)
		c.actionsMu.Unlock()
		break
	}
}
