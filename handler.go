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
	"github.com/rs/cors"
)

var additionalMiddlewares []middleware.Middleware

type handler struct {
	*cors.Cors
	log *logger.Logger

	endpoints map[endpointKey]*endpoint
}

func NewHandler(log *logger.Logger, mm ...middleware.Middleware) *handler {
	additionalMiddlewares = make([]middleware.Middleware, len(mm)+1, len(mm)+1)
	additionalMiddlewares[0] = &middleware.RequestValidate{}
	for i := len(mm) - 1; i > -1; i-- {
		additionalMiddlewares[i+1] = mm[i]
	}

	return &handler{
		log:       log,
		endpoints: make(map[endpointKey]*endpoint),
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer getDeferCatchPanic(h.log, w)

	if r.Method == http.MethodGet && r.URL.Path == "/metrics" {
		promhttp.Handler().ServeHTTP(w, r)
		return
	}

	ctx, span := tracer.StartSpan(context.Background(), r.URL.Path)
	span.Tag("method", r.Method)
	defer span.End()

	ctx = logger.ToContext(ctx, h.log.With("token", span.TraceId()))

	w.Header().Set("Content-Type", "application/json")

	e, ok := h.endpoints[endpointKey{r.URL.Path, r.Method}]
	if !ok || e == nil {
		logger.Warn(ctx, "unknown method", "method", r.Method, "path", r.URL.Path)
		w.WriteHeader(405)
		_ = json.NewEncoder(w).Encode(&responses.ErrorResponse{Message: "unknown method"})
		return
	}

	req := checkAction(r, e.request, w)
	if req == nil {
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

func (h *handler) Endpoint(path, method string, request requests.Request, action func(ctx context.Context, req requests.Request) (responses.Response, int), mm ...string) {
	e := &endpoint{
		action:      action,
		request:     request,
		middlewares: make(map[string]bool),
	}
	for _, m := range mm {
		e.middlewares[m] = true
	}
	e.middlewares[middleware.KeyRequestValidate] = true
	e.middlewares[middleware.KeyRequestInit] = true

	h.endpoints[endpointKey{path, method}] = e
}
