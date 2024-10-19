package ws

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/porebric/logger"
)

const (
	// writeWait Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// pongWait Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// pingPeriod Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// maxMessageSize Maximum message size allowed from peer.
	maxMessageSize = 512
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
	hub       *Hub
	conn      *websocket.Conn
	ctx       context.Context
	sendCh    chan []byte
	uniqueKey uuid.UUID
	userId    int
	kind      string
	key       string
}

func newClient(ctx context.Context, hub *Hub, sendCh chan []byte, coon *websocket.Conn, key string) *client {
	return &client{
		hub:       hub,
		conn:      coon,
		sendCh:    sendCh,
		ctx:       ctx,
		uniqueKey: uuid.New(),
		key:       key,
	}
}

func (c *client) read() {
	defer func() {
		if c.conn.Close() != nil {
			return
		}
		c.hub.unregister <- c
	}()

	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		logger.Error(c.ctx, err, "new read deadline", "user", c.key)
		return
	}

	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			logger.Error(c.ctx, err, "new read deadline", "user", c.key)
			return err
		}
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				logger.Error(c.ctx, err, "read message", "user", c.key)
				break
			}
			break
		}

		if b := getBroadcast(c.ctx, message, c.key, c.uniqueKey, c.hub.broadcasts); b != nil {
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
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.sendCh:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				logger.Warn(c.ctx, "next writer", "msg", string(message))
				return
			}

			if _, err = w.Write(message); err != nil {
				logger.Warn(c.ctx, "write", "msg", string(message))
				return
			}

			n := len(c.sendCh)

			for i := 0; i < n; i++ {
				if _, err = w.Write(newline); err != nil {
					logger.Warn(c.ctx, "write new line", "msg", string(message))
					return
				}
				if _, err = w.Write(<-c.sendCh); err != nil {
					logger.Warn(c.ctx, "write sendCh", "msg", string(message))
					return
				}
			}

			if err = w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *client) send(body []byte) {
	c.sendCh <- body
}

func (c *client) waitAuth() {
	time.Sleep(60 * time.Second)
	if c.kind == "" {
		c.hub.unregister <- c
	}
}
