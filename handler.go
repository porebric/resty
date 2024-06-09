package resty

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/porebric/logger"
	"github.com/porebric/resty/errors"
	"github.com/porebric/resty/middleware"
	"github.com/porebric/resty/requests"
	"github.com/porebric/resty/responses"
	"github.com/porebric/tracer"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func serveHTTP[R requests.Request](
	e endpoint[R],
	log *logger.Logger,
	initRequest func(ctx context.Context, r *http.Request) (context.Context, R, error),
	mm ...func() middleware.Middleware,
) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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

		if e.method != r.Method {
			logger.Warn(ctx, "unknown method", "method", r.Method, "path", r.URL.Path)
			w.WriteHeader(405)
			_ = json.NewEncoder(w).Encode(&responses.ErrorResponse{Message: "unknown method"})
			return
		}

		ctx, req, err := initRequest(ctx, r)
		if err != nil {
			resp, httpCode := errors.GetCustomError("", errors.ErrorInvalidRequest)
			w.WriteHeader(httpCode)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		ctx, ok := checkAction(ctx, req, w, mm...)
		if !ok {
			return
		}

		logger.Info(ctx, "request", "content", req, "method", r.Method, "path", r.URL.Path)
		resp, httpCode := e.action(ctx, req)
		w.WriteHeader(httpCode)

		if err := resp.PrepareResponse(w); err != nil {
			w.WriteHeader(http.StatusExpectationFailed)
			_, _ = w.Write([]byte{})
		}

		return
	}
}

func Endpoint[R requests.Request](
	log *logger.Logger,
	path, method string,
	initRequest func(ctx context.Context, r *http.Request) (context.Context, R, error),
	action func(context.Context, R) (responses.Response, int),
	mm ...func() middleware.Middleware,
) {
	e := endpoint[R]{
		action: action,
		method: method,
	}

	http.HandleFunc(path, serveHTTP(e, log, initRequest, mm...))
}
