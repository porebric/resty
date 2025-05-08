package ws

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/porebric/logger"
)

const (
	writeWait      = 10 * time.Second    // Время, разрешенное на отправку сообщения
	pongWait       = 60 * time.Second    // Время, разрешенное для чтения следующего pong-сообщения
	pingPeriod     = (pongWait * 9) / 10 // Период отправки ping-сообщений
	maxMessageSize = 512                 // Максимальный размер сообщения, разрешенный от клиента
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type client struct {
	hub    *Hub
	conn   *websocket.Conn
	ctx    context.Context
	sendCh chan []byte
	uuid   uuid.UUID
	userId int
	action string
	key    string

	closeOnce sync.Once
	isClosed  atomic.Bool
}

func newClient(ctx context.Context, hub *Hub, sendCh chan []byte, conn *websocket.Conn, key string) *client {
	uid := uuid.New()

	return &client{
		hub:    hub,
		conn:   conn,
		sendCh: sendCh,
		ctx:    logger.ToContext(ctx, logger.FromContext(ctx).With("uuid", uid, "user", key)),
		uuid:   uid,
		key:    key,
	}
}

func (c *client) read() {
	defer func() {
		if err := c.conn.Close(); err != nil {
			logger.Error(c.ctx, err, "failed to close connection")
		}
		c.hub.unregister <- c
	}()

	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		logger.Error(c.ctx, err, "new read deadline")
		return
	}

	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			logger.Error(c.ctx, err, "new read deadline")
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

		if b := getBroadcast(c.ctx, message, c.key, c.uuid, c.hub.broadcasts); b != nil {
			c.hub.broadcast <- b
		} else {
			c.send(newError(InvalidMsgPrefix, "invalid body or action", c.key).Msg())
		}
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

func (c *client) send(data []byte) {
	if c.isClosed.Load() {
		return
	}

	select {
	case c.sendCh <- data:
	case <-time.After(500 * time.Millisecond):
		logger.Warn(c.ctx, "send timeout")
		c.hub.unregister <- c
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
			if c.action == "" {
				c.hub.unregister <- c
			}
			return
		case <-ticker.C:
			if c.action != "" {
				return
			}
		}
	}
}
