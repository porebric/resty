package login

import (
	"github.com/google/uuid"
)

const Action = "login"

type Broadcast struct {
	uuid uuid.UUID
	key  string

	Token  string `json:"token"`
	Action string `json:"action"`
}

func (b *Broadcast) Set(key string, uuid uuid.UUID) {
	b.key = key
	b.uuid = uuid
}

func (b *Broadcast) GetKey() string {
	return b.key
}

func (b *Broadcast) GetUuid() uuid.UUID {
	return b.uuid
}
