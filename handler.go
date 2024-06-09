package resty

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/porebric/logger"
	"github.com/porebric/resty/middleware"
	"github.com/porebric/resty/requests"
	"github.com/porebric/resty/responses"
	"github.com/porebric/tracer"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var additionalMiddlewares []middleware.Middleware

func InitMiddlewares(mm ...middleware.Middleware) {
	additionalMiddlewares = make([]middleware.Middleware, len(mm)+1, len(mm)+1)
	additionalMiddlewares[0] = &middleware.RequestValidate{}
	for i := len(mm) - 1; i > -1; i-- {
		additionalMiddlewares[i+1] = mm[i]
	}
}

func serveHTTP[R requests.Request](e endpoint[R], log *logger.Logger) func(w http.ResponseWriter, r *http.Request) {
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

		var req R

		ctx = checkAction(ctx, r, req, w)

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

func Endpoint[R requests.Request](log *logger.Logger, path, method string, action func(context.Context, R) (responses.Response, int), mm ...string) {
	e := endpoint[R]{
		action:      action,
		middlewares: make(map[string]bool),
		method:      method,
	}
	for _, m := range mm {
		e.middlewares[m] = true
	}
	e.middlewares[middleware.KeyRequestValidate] = true
	e.middlewares[middleware.KeyRequestInit] = true

	http.HandleFunc(path, serveHTTP(e, log))
}
