package ws

import (
	"fmt"
)

type KeyType string

func (t KeyType) String() string {
	return string(t)
}

const (
	InvalidMsgPrefix     = KeyType("invalid_msg")
	MaxConnectionsPrefix = KeyType("max connections")
	AuthPrefix           = KeyType("auth")
)

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
