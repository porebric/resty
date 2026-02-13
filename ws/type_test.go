package ws

import (
	"encoding/json"
	"testing"
)

func TestKeyType_String(t *testing.T) {
	tests := []struct {
		name string
		k    KeyType
		want string
	}{
		{"invalid_msg", InvalidMsgPrefix, "invalid_msg"},
		{"auth", AuthPrefix, "auth"},
		{"max_connections", MaxConnectionsPrefix, "max connections"},
		{"empty", KeyType(""), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.k.String(); got != tt.want {
				t.Errorf("KeyType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_newError(t *testing.T) {
	e := newError(InvalidMsgPrefix, "bad request", "user1")
	if e.Code != InvalidMsgPrefix {
		t.Errorf("Code = %v, want InvalidMsgPrefix", e.Code)
	}
	if e.M != "bad request" {
		t.Errorf("M = %q, want %q", e.M, "bad request")
	}
	if e.Key != "user1" {
		t.Errorf("Key = %q, want %q", e.Key, "user1")
	}
}

func TestError_Msg(t *testing.T) {
	e := newError(AuthPrefix, "not authorized", "key1")
	b := e.Msg()
	if len(b) == 0 {
		t.Fatal("Msg() returned empty bytes")
	}
	var out struct {
		Status KeyType `json:"status"`
		Msg    string  `json:"msg"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Msg() returned invalid JSON: %v", err)
	}
	if out.Status != AuthPrefix {
		t.Errorf("status = %q, want %q", out.Status, AuthPrefix)
	}
	if out.Msg != "not authorized" {
		t.Errorf("msg = %q, want %q", out.Msg, "not authorized")
	}
}

func TestError_Msg_escapesSpecialCharacters(t *testing.T) {
	e := newError(InvalidMsgPrefix, `say "hello"`, "k")
	b := e.Msg()
	var out struct {
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Msg() returned invalid JSON: %v", err)
	}
	if out.Msg != `say "hello"` {
		t.Errorf("msg = %q, want %q", out.Msg, `say "hello"`)
	}
}
