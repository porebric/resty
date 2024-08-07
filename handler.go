package resty

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"github.com/porebric/logger"
	"github.com/porebric/resty/errors"
	"github.com/porebric/resty/middleware"
	"github.com/porebric/resty/requests"
	"github.com/porebric/resty/responses"
	"github.com/porebric/tracer"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var first uint32

func serveHTTP[R requests.Request](
	action func(context.Context, R) (responses.Response, int),
	log *logger.Logger,
	initRequest func(ctx context.Context, r *http.Request) (context.Context, R, error),
	mm ...func() middleware.Middleware,
) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		defer getDeferCatchPanic(log, w)

		if r.Method == http.MethodGet && r.URL.Path == "/metrics" {
			promhttp.Handler().ServeHTTP(w, r)
			return
		}

		ctx, span := tracer.StartSpan(context.Background(), r.URL.Path)
		span.Tag("method", r.Method)
		defer span.End()

		ctx = logger.ToContext(ctx, log.With("token", span.TraceId()))

		w.Header().Set("Content-Type", "application/json")

		var req R

		ok := false
		for _, method := range req.Methods() {
			if method == r.Method {
				ok = true
			}
		}

		if !ok {
			logger.Warn(ctx, "unknown method", "method", r.Method, "path", r.URL.Path)
			w.WriteHeader(405)
			_ = json.NewEncoder(w).Encode(&responses.ErrorResponse{Message: "unknown method"})
			return
		}

		ctx, req, err = initRequest(ctx, r)
		if err != nil {
			resp, httpCode := errors.GetCustomError("", errors.ErrorInvalidRequest)
			w.WriteHeader(httpCode)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		ctx, ok = checkAction(ctx, req, w, mm...)
		if !ok {
			return
		}

		logger.Info(ctx, "request", "content", req, "method", r.Method, "path", r.URL.Path)
		resp, httpCode := action(ctx, req)
		w.WriteHeader(httpCode)

		if err = resp.PrepareResponse(w); err != nil {
			w.WriteHeader(http.StatusExpectationFailed)
			_, _ = w.Write([]byte{})
		}

		return
	}
}

func Endpoint[R requests.Request](l *logger.Logger, req func(ctx context.Context, r *http.Request) (context.Context, R, error), action func(context.Context, R) (responses.Response, int), mm ...func() middleware.Middleware) {
	if atomic.CompareAndSwapUint32(&first, 0, 1) {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				logger.Warn(logger.ToContext(context.Background(), l), "not found", "method", r.Method, "path", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(404)
				_ = json.NewEncoder(w).Encode(&responses.ErrorResponse{Message: "not found"})
				return
			}
		})
	}

	var r R
	http.HandleFunc(r.Path(), serveHTTP(action, l, req, mm...))
}
