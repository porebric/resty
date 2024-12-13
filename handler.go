package resty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof"

	"github.com/porebric/logger"
	"github.com/porebric/resty/errors"
	"github.com/porebric/resty/middleware"
	"github.com/porebric/resty/requests"
	"github.com/porebric/resty/responses"
	"github.com/porebric/tracer"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	requestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "The number of HTTP requests, tracked by path and response code.",
		},
		[]string{"path", "code"},
	)
)

func serveHTTP[R requests.Request](
	action func(context.Context, R) (responses.Response, int),
	logFn func() *logger.Logger,
	initRequest func(ctx context.Context, r *http.Request) (context.Context, R, error),
	mm ...func() middleware.Middleware,
) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer getDeferCatchPanic(logFn(), w)

		var (
			err      error
			httpCode int
			resp     responses.Response
		)

		defer func() {
			requestCounter.WithLabelValues(r.URL.Path, fmt.Sprintf("%d", httpCode)).Inc()
		}()

		ctx, span := tracer.StartSpan(r.Context(), r.URL.Path)
		span.Tag("method", r.Method)
		defer span.End()

		ctx = logger.ToContext(ctx, logFn().With("token", span.TraceId()))
		var req R

		defer func() {
			if httpCode >= http.StatusBadRequest {
				logger.Warn(ctx, "request", "content", req, "method", r.Method, "path", r.URL.Path, "response", resp)
			} else {
				logger.Info(ctx, "request", "content", req, "method", r.Method, "path", r.URL.Path, "response", resp)
			}
		}()

		w.Header().Set("Content-Type", "application/json")

		ok := false
		for _, method := range req.Methods() {
			if method == r.Method {
				ok = true
			}
		}

		if !ok {
			logger.Warn(ctx, "unknown method", "method", r.Method, "path", r.URL.Path)
			httpCode = http.StatusMethodNotAllowed

			w.WriteHeader(httpCode)
			_ = json.NewEncoder(w).Encode(&responses.ErrorResponse{Message: "unknown method"})

			return
		}

		if ctx, req, err = initRequest(ctx, r); err != nil {
			resp, httpCode = errors.GetCustomError("", errors.ErrorInvalidRequest)

			w.WriteHeader(httpCode)
			_ = json.NewEncoder(w).Encode(resp)

			return
		}

		ctx, ok = checkAction(ctx, req, w, mm...)
		if !ok {
			return
		}

		resp, httpCode = action(ctx, req)

		w.WriteHeader(httpCode)

		if err = resp.PrepareResponse(w); err != nil {
			w.WriteHeader(http.StatusExpectationFailed)
			_, _ = w.Write([]byte{})
		}

		return
	}
}

func Endpoint[R requests.Request](r Router, req func(ctx context.Context, r *http.Request) (context.Context, R, error), action func(context.Context, R) (responses.Response, int), mm ...func() middleware.Middleware) {
	var exampleReq R
	r.MuxRouter().HandleFunc(exampleReq.Path(), serveHTTP(action, r.LogFn, req, mm...)).Methods(exampleReq.Methods()...)
}
