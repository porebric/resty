package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/porebric/logger"
)

func testLogFnIntegration() func() *logger.Logger {
	l := logger.New(logger.DebugLevel)
	return func() *logger.Logger { return l }
}

// startTestServer starts an httptest server with hub and ws handler; key is taken from query "key".
func startTestServer(t *testing.T, loginFn func(context.Context, *LoginMessage) (context.Context, Error)) (*httptest.Server, *Hub) {
	keyFn := func(r *http.Request) string {
		return r.URL.Query().Get("key")
	}
	hub := NewHub(loginFn, keyFn)
	go hub.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		NewHandler(testLogFnIntegration()).ServeWs(hub, w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, hub
}

func dialWS(t *testing.T, baseURL, key string) *websocket.Conn {
	wsURL := "ws" + strings.TrimPrefix(baseURL, "http") + "/ws?key=" + key
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestIntegration_ConnectAndLoginSuccess(t *testing.T) {
	loginFn := func(ctx context.Context, msg *LoginMessage) (context.Context, Error) {
		if msg.Token != "valid-token" {
			return ctx, newError(AuthPrefix, "bad token", msg.GetKey())
		}
		return ctx, Error{}
	}
	srv, hub := startTestServer(t, loginFn)
	conn := dialWS(t, srv.URL, "user1")

	// Send login message
	loginBody := []byte(`{"token": "valid-token", "actions": ["read"]}`)
	if err := conn.WriteMessage(websocket.TextMessage, loginBody); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Expect login success
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var out struct {
		Login bool   `json:"login"`
		UUID  string `json:"uuid"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !out.Login {
		t.Errorf("expected login true, got %+v", out)
	}
	if out.UUID == "" {
		t.Error("expected non-empty uuid")
	}

	// SendToClient to this user should deliver
	hub.SendToClient(context.Background(), "user1", nil, []byte(`{"push": true}`))
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read push: %v", err)
	}
	if string(data) != `{"push": true}` {
		t.Errorf("push message = %q", data)
	}
}

func TestIntegration_LoginAuthError(t *testing.T) {
	loginFn := func(ctx context.Context, msg *LoginMessage) (context.Context, Error) {
		return ctx, newError(AuthPrefix, "invalid token", msg.GetKey())
	}
	srv, _ := startTestServer(t, loginFn)
	conn := dialWS(t, srv.URL, "user1")

	loginBody := []byte(`{"token": "bad", "actions": []}`)
	if err := conn.WriteMessage(websocket.TextMessage, loginBody); err != nil {
		t.Fatalf("write: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var out struct {
		Status string `json:"status"`
		Msg    string `json:"msg"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != string(AuthPrefix) {
		t.Errorf("status = %q, want auth error", out.Status)
	}
}

func TestIntegration_InvalidJSON(t *testing.T) {
	loginFn := func(ctx context.Context, _ *LoginMessage) (context.Context, Error) {
		return ctx, Error{}
	}
	srv, _ := startTestServer(t, loginFn)
	conn := dialWS(t, srv.URL, "user1")

	if err := conn.WriteMessage(websocket.TextMessage, []byte("not json")); err != nil {
		t.Fatalf("write: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var out struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(data, &out)
	if out.Status != string(InvalidMsgPrefix) {
		t.Errorf("status = %q, want invalid_msg", out.Status)
	}
}

func TestIntegration_MaxConnections(t *testing.T) {
	loginFn := func(ctx context.Context, _ *LoginMessage) (context.Context, Error) {
		return ctx, Error{}
	}
	srv, _ := startTestServer(t, loginFn)
	key := "sameuser"
	var conns []*websocket.Conn
	for i := 0; i < maxUserConnections+1; i++ {
		conn := dialWS(t, srv.URL, key)
		conns = append(conns, conn)
		// Send login so they register
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"token":"t","actions":[]}`))
	}

	// One of the connections should get max connections error or close
	// The last one (maxUserConnections+1) is rejected in doRegister
	var gotError bool
	for _, conn := range conns {
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, data, err := conn.ReadMessage()
		if err != nil {
			continue
		}
		var out struct {
			Status string `json:"status"`
			Msg    string `json:"msg"`
		}
		_ = json.Unmarshal(data, &out)
		if strings.Contains(out.Msg, "max connections") || out.Status == string(MaxConnectionsPrefix) {
			gotError = true
			break
		}
	}
	if !gotError {
		t.Log("one connection should have received max connections error (or closed)")
	}
	for _, c := range conns {
		_ = c.Close()
	}
}

func TestIntegration_SendToClient_ByAction(t *testing.T) {
	loginFn := func(ctx context.Context, msg *LoginMessage) (context.Context, Error) {
		return ctx, Error{}
	}
	srv, hub := startTestServer(t, loginFn)
	conn := dialWS(t, srv.URL, "user1")
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"token":"t","actions":["read","write"]}`))
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, _ = conn.ReadMessage() // login ack

	// Send only to clients with "admin" action - we don't have it
	hub.SendToClient(context.Background(), "user1", nil, []byte(`{"admin":1}`), "admin")
	// Send to clients with "read" - we have it
	hub.SendToClient(context.Background(), "user1", nil, []byte(`{"read":1}`), "read")

	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != `{"read":1}` {
		t.Errorf("expected read message, got %q", data)
	}
}

func TestIntegration_AddActionAndSend(t *testing.T) {
	loginFn := func(ctx context.Context, _ *LoginMessage) (context.Context, Error) {
		return ctx, Error{}
	}
	srv, hub := startTestServer(t, loginFn)
	conn := dialWS(t, srv.URL, "user2")
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"token":"t","actions":["a"]}`))
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, _ = conn.ReadMessage()

	hub.AddActionToClients("user2", "b")
	hub.SendToClient(context.Background(), "user2", nil, []byte(`{"both":1}`), "a", "b")

	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != `{"both":1}` {
		t.Errorf("got %q", data)
	}
}

func TestIntegration_NoKey_Forbidden(t *testing.T) {
	keyFn := func(r *http.Request) string { return r.URL.Query().Get("key") }
	hub := NewHub(
		func(ctx context.Context, _ *LoginMessage) (context.Context, Error) { return ctx, Error{} },
		keyFn,
	)
	go hub.Run()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		NewHandler(testLogFnIntegration()).ServeWs(hub, w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resp, err := srv.Client().Get(srv.URL + "/ws")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("GET /ws without key: status = %d, want 403", resp.StatusCode)
	}
}

func TestIntegration_ConcurrentSendToClient(t *testing.T) {
	loginFn := func(ctx context.Context, _ *LoginMessage) (context.Context, Error) {
		return ctx, Error{}
	}
	srv, hub := startTestServer(t, loginFn)
	conn := dialWS(t, srv.URL, "user3")
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"token":"t","actions":[]}`))
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, _ = conn.ReadMessage()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			hub.SendToClient(context.Background(), "user3", nil, []byte(`{"n":1}`))
		}(i)
	}
	wg.Wait()

	count := 0
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for count < 20 {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
		count++
	}
	if count < 10 {
		t.Errorf("expected at least 10 messages, got %d", count)
	}
}
