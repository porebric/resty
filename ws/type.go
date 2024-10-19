package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/porebric/logger"
)

type KeyType string

func (t KeyType) String() string {
	return string(t)
}

const (
	UndefinedPrefix      = KeyType("undefined")
	InvalidMsgPrefix     = KeyType("invalid_msg")
	MaxConnectionsPrefix = KeyType("max connections")
	AuthPrefix           = KeyType("auth")
)

type Broadcast interface {
	Set(string, uuid.UUID)
	GetKey() string
	GetUuid() uuid.UUID
}

type broadcastContainer struct {
	Action string `json:"action"`
}

type broadcastContainerWithBody struct {
	Action    string    `json:"action"`
	Broadcast Broadcast `json:"body"`
}

func getBroadcast(ctx context.Context, b []byte, key string, uuid uuid.UUID, broadcasts map[string]Broadcast) Broadcast {
	var container broadcastContainer

	if err := json.Unmarshal(bytes.TrimSpace(bytes.Replace(b, newline, space, -1)), &container); err != nil {
		logger.Warn(ctx, "parse message", "client", key, "body", string(b), "error", err)
		return nil
	}

	broadcast, ok := broadcasts[container.Action]
	if !ok {
		logger.Warn(ctx, "action not found", "client", key, "body", string(b), "action", container.Action)
		return nil
	}

	containerWithBody := broadcastContainerWithBody{
		Action:    container.Action,
		Broadcast: broadcast,
	}

	if err := json.Unmarshal(bytes.TrimSpace(bytes.Replace(b, newline, space, -1)), &containerWithBody); err != nil {
		logger.Warn(ctx, "parse message", "client", key, "body", string(b), "error", err, "action", containerWithBody.Action)
		return nil
	}

	containerWithBody.Broadcast.Set(key, uuid)

	return containerWithBody.Broadcast
}

type Error struct {
	Code KeyType `json:"code"`
	M    string  `json:"msg"`

	Key string `json:"key"`
}

func newError(code KeyType, msg, key string) Error {
	return Error{code, msg, key}
}

func (e Error) Msg() []byte {
	return []byte(fmt.Sprintf(`{"status": "%s", "msg": "%s"}`, e.Code, e.M))
}
