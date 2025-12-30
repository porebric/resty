package ws

import (
	"github.com/google/uuid"
)

type LoginMessage struct {
	key  string
	uuid uuid.UUID

	Token   string   `json:"token"`
	Actions []string `json:"actions"`
}

func (b *LoginMessage) Set(key string, uuid uuid.UUID) {
	b.key = key
	b.uuid = uuid
}

func (b *LoginMessage) GetKey() string {
	return b.key
}

func (b *LoginMessage) GetUuid() uuid.UUID {
	return b.uuid
}
