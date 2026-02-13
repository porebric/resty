package ws

import (
	"encoding/json"
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
	Key  string  `json:"key"`
}

func newError(code KeyType, msg, key string) Error {
	return Error{Code: code, M: msg, Key: key}
}

// errWire is the JSON shape sent to the client.
type errWire struct {
	Status KeyType `json:"status"`
	Msg    string  `json:"msg"`
}

func (e Error) Msg() []byte {
	b, _ := json.Marshal(errWire{Status: e.Code, Msg: e.M})
	return b
}
