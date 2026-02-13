package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/porebric/logger"
)

const (
	writeWait      = 10 * time.Second    // max time allowed for writing a message
	pongWait       = 60 * time.Second    // max time allowed for reading the next pong
	pingPeriod     = (pongWait * 9) / 10 // interval for sending ping frames
	maxMessageSize = 512                 // max size of a single message from client (login payload)
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

type client struct {
	hub    *Hub
	conn   *websocket.Conn
	ctx    context.Context
	sendCh chan []byte
	uuid   uuid.UUID
	key    string

	actions   []string
	actionsMu sync.RWMutex

	auth atomic.Bool

	closeOnce sync.Once
	isClosed  atomic.Bool
}

func newClient(ctx context.Context, hub *Hub, sendCh chan []byte, conn *websocket.Conn, key string) *client {
	uid := uuid.New()

	return &client{
		hub:       hub,
		conn:      conn,
		sendCh:    sendCh,
		ctx:       logger.ToContext(ctx, logger.FromContext(ctx).With("uuid", uid, "user", key)),
		uuid:      uid,
		key:       key,
		actions:   make([]string, 0),
		auth:      atomic.Bool{},
		closeOnce: sync.Once{},
	}
}

func (c *client) read() {
	defer func() {
		c.safeClose()
		c.hub.unregister <- c
	}()

	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		logger.Error(c.ctx, err, "set read deadline")
		return
	}

	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			logger.Error(c.ctx, err, "set read deadline on pong")
			return err
		}
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logger.Warn(c.ctx, "read message", "error", err)
			}
			break
		}

		msg := new(LoginMessage)

		if err = json.Unmarshal(bytes.TrimSpace(bytes.Replace(message, newline, space, -1)), msg); err != nil {
			logger.Warn(c.ctx, "parse message", "client", c.key, "body", string(message), "error", err)
			c.send(newError(InvalidMsgPrefix, "invalid body or action", c.key).Msg())
			continue
		}

		msg.key = c.key
		msg.uuid = c.uuid

		c.hub.loginMsgCh <- msg
	}
}

func (c *client) write() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.safeClose()
		logger.Info(c.ctx, "websocket connect closed", "user", c.key)
	}()

	for {
		select {
		case message, ok := <-c.sendCh:
			if !ok || message == nil {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				logger.Error(c.ctx, err, "next writer")
				return
			}

			if _, err = w.Write(message); err != nil {
				logger.Error(c.ctx, err, "write message")
				return
			}

			if err = w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Error(c.ctx, err, "ping message")
				return
			}
		}
	}
}

func (c *client) safeClose() {
	c.closeOnce.Do(func() {
		c.isClosed.Store(true)
		if c.conn != nil {
			_ = c.conn.Close()
		}
		close(c.sendCh)
	})
}

// send enqueues a message for the write goroutine. It does not unregister on
// timeout so that a temporarily slow write (e.g. during ping) does not drop
// the connection; the connection is still closed by read errors or pong timeout.
func (c *client) send(data []byte) {
	if c.isClosed.Load() {
		return
	}

	select {
	case c.sendCh <- data:
	case <-time.After(2 * time.Second):
		logger.Warn(c.ctx, "send buffer full, dropping message")
	}
}

func (c *client) waitAuth() {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if !c.auth.Load() {
				c.hub.unregister <- c
			}
			return
		case <-ticker.C:
			if c.auth.Load() {
				return
			}
		}
	}
}
