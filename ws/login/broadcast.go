package login

import (
	"github.com/google/uuid"
)

const Action = "login"

type Broadcast struct {
	key string

	Token  string    `json:"token"`
	Action string    `json:"action"`
	Uuid   uuid.UUID `json:"uuid"`
}

func (b *Broadcast) Set(key string, uuid uuid.UUID) {
	b.key = key
	b.Uuid = uuid
}

func (b *Broadcast) GetKey() string {
	return b.key
}

func (b *Broadcast) GetUuid() uuid.UUID {
	return b.Uuid
}
