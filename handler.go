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

		var req R

		path, showPath := req.Path()
		logPath := path
		if showPath {
			logPath = r.URL.Path
		}

		defer func() {
			requestCounter.WithLabelValues(fmt.Sprintf("%s:%s", r.Method, path), fmt.Sprintf("%d", httpCode)).Inc()
		}()

		ctx, span := tracer.StartSpan(r.Context(), fmt.Sprintf("%s:%s", r.Method, path))
		defer span.End()

		ctx = logger.ToContext(ctx, logFn().With("token", span.TraceId()))

		w.Header().Set("Content-Type", "application/json")

		ok := false
		for _, method := range req.Methods() {
			if method == r.Method {
				ok = true
			}
		}

		if !ok {
			httpCode = http.StatusMethodNotAllowed
			resp = &responses.ErrorResponse{Message: "unknown method"}

			w.WriteHeader(httpCode)
			_ = json.NewEncoder(w).Encode(resp)

			logger.Info(ctx, "http request", "content", req.String(), "method", r.Method, "path", logPath, "response", resp.String())
			return
		}

		if ctx, req, err = initRequest(ctx, r); err != nil {
			resp, httpCode = errors.GetCustomError("", errors.ErrorInvalidRequest)
			w.WriteHeader(httpCode)
			_ = json.NewEncoder(w).Encode(resp)
			logger.Info(ctx, "http request", "content", req.String(), "method", r.Method, "path", logPath, "response", resp.String())
			return
		}

		if ctx, resp, httpCode = checkAction(ctx, req, w, mm...); resp != nil {
			w.WriteHeader(httpCode)
			_ = json.NewEncoder(w).Encode(resp)
			logger.Info(ctx, "http request", "content", req.String(), "method", r.Method, "path", logPath, "response", resp.String())
			return
		}

		resp, httpCode = action(ctx, req)

		w.WriteHeader(httpCode)

		if err = resp.PrepareResponse(w); err != nil {
			w.WriteHeader(http.StatusExpectationFailed)
			_, _ = w.Write([]byte{})
		}

		logger.Info(ctx, "http request", "content", req.String(), "method", r.Method, "path", logPath, "response", resp.String())
		return
	}
}

func Endpoint[R requests.Request](r Router, req func(ctx context.Context, r *http.Request) (context.Context, R, error), action func(context.Context, R) (responses.Response, int), mm ...func() middleware.Middleware) {
	var exampleReq R
	path, _ := exampleReq.Path()
	r.MuxRouter().HandleFunc(path, serveHTTP(action, r.LogFn, req, mm...)).Methods(exampleReq.Methods()...)
}
