package ws

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestLoginMessage_GetKey_GetUuid(t *testing.T) {
	key := "user123"
	uid := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	msg := &LoginMessage{}
	msg.Set(key, uid)
	if msg.GetKey() != key {
		t.Errorf("GetKey() = %q, want %q", msg.GetKey(), key)
	}
	if msg.GetUuid() != uid {
		t.Errorf("GetUuid() = %v, want %v", msg.GetUuid(), uid)
	}
}

func TestLoginMessage_JSON(t *testing.T) {
	body := []byte(`{"token": "abc", "actions": ["read", "write"]}`)
	var msg LoginMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Token != "abc" {
		t.Errorf("Token = %q, want %q", msg.Token, "abc")
	}
	if len(msg.Actions) != 2 || msg.Actions[0] != "read" || msg.Actions[1] != "write" {
		t.Errorf("Actions = %v", msg.Actions)
	}
}
