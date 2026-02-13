package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/porebric/logger"
)

func testLogFn() func() *logger.Logger {
	l := logger.New(logger.DebugLevel)
	return func() *logger.Logger { return l }
}

func TestHandler_ServeWs_ForbiddenWhenKeyEmpty(t *testing.T) {
	keyFn := func(*http.Request) string { return "" }
	hub := NewHub(
		func(ctx context.Context, _ *LoginMessage) (context.Context, Error) { return ctx, Error{} },
		keyFn,
	)
	h := NewHandler(testLogFn())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	h.ServeWs(hub, w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("ServeWs() code = %d, want %d", w.Code, http.StatusForbidden)
	}
	if w.Body.Len() == 0 {
		t.Error("expected body with forbidden message")
	}
}

func TestHandler_ServeWs_ForbiddenWhenKeyEmpty_WithCheckOrigin(t *testing.T) {
	keyFn := func(*http.Request) string { return "" }
	hub := NewHub(
		func(ctx context.Context, _ *LoginMessage) (context.Context, Error) { return ctx, Error{} },
		keyFn,
	)
	rejectOrigin := func(r *http.Request) bool { return false }
	h := NewHandler(testLogFn(), WithCheckOrigin(rejectOrigin))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	h.ServeWs(hub, w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("ServeWs() code = %d, want %d", w.Code, http.StatusForbidden)
	}
}
