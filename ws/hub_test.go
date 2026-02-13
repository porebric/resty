package ws

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestNewHub(t *testing.T) {
	loginFn := func(ctx context.Context, _ *LoginMessage) (context.Context, Error) {
		return ctx, Error{}
	}
	keyFn := func(r *http.Request) string { return "" }
	hub := NewHub(loginFn, keyFn)
	if hub == nil {
		t.Fatal("NewHub returned nil")
	}
}

func TestHub_SendToClient_emptyKey(t *testing.T) {
	loginFn := func(ctx context.Context, _ *LoginMessage) (context.Context, Error) {
		return ctx, Error{}
	}
	keyFn := func(r *http.Request) string { return "" }
	hub := NewHub(loginFn, keyFn)
	hub.SendToClient(context.Background(), "", nil, []byte("test"))
	hub.SendToClient(context.Background(), "", nil, []byte("test"), "action1")
}

func TestHub_SendToClient_unknownKey(t *testing.T) {
	loginFn := func(ctx context.Context, _ *LoginMessage) (context.Context, Error) {
		return ctx, Error{}
	}
	keyFn := func(r *http.Request) string { return "" }
	hub := NewHub(loginFn, keyFn)
	hub.SendToClient(context.Background(), "nonexistent", nil, []byte("test"))
	hub.SendToClient(context.Background(), "nonexistent", nil, []byte("test"), "a")
}

func TestHub_AddActionToClients_unknownKey(t *testing.T) {
	loginFn := func(ctx context.Context, _ *LoginMessage) (context.Context, Error) {
		return ctx, Error{}
	}
	keyFn := func(r *http.Request) string { return "" }
	hub := NewHub(loginFn, keyFn)
	hub.AddActionToClients("nonexistent", "action1")
}

func TestHub_AddActionToClient_unknownKey(t *testing.T) {
	loginFn := func(ctx context.Context, _ *LoginMessage) (context.Context, Error) {
		return ctx, Error{}
	}
	keyFn := func(r *http.Request) string { return "" }
	hub := NewHub(loginFn, keyFn)
	hub.AddActionToClient("nonexistent", "action1", uuid.Nil)
}

func TestHub_handleBroadcast_nilMessage(t *testing.T) {
	loginFn := func(ctx context.Context, _ *LoginMessage) (context.Context, Error) {
		return ctx, Error{}
	}
	keyFn := func(r *http.Request) string { return "" }
	hub := NewHub(loginFn, keyFn)
	hub.handleBroadcast(nil)
}

func TestHub_handleBroadcast_unknownKey(t *testing.T) {
	loginFn := func(ctx context.Context, _ *LoginMessage) (context.Context, Error) {
		return ctx, Error{}
	}
	keyFn := func(r *http.Request) string { return "" }
	hub := NewHub(loginFn, keyFn)
	msg := &LoginMessage{}
	msg.Set("unknown", uuid.Nil)
	hub.handleBroadcast(msg)
}
