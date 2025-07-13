package resty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/pprof"

	"github.com/gorilla/mux"
	"github.com/porebric/logger"
	"github.com/porebric/resty/responses"
	"github.com/porebric/resty/ws"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Router interface {
	MuxRouter() *mux.Router
	LogFn() *logger.Logger
	GetWsHub() *ws.Hub

	SetCors(allowedOrigins, allowedMethods, allowedHeaders []string)
	CorsAllowedOrigins() []string
	CorsAllowedMethods() []string
	CorsAllowedHeaders() []string
}

type router struct {
	router *mux.Router
	logFn  func() *logger.Logger
	wsHub  *ws.Hub

	corsAllowedOrigins []string
	corsAllowedMethods []string
	corsAllowedHeaders []string
}

func NewRouter(logFn func() *logger.Logger, wsHub *ws.Hub) Router {
	r := mux.NewRouter()

	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)

	prometheus.MustRegister(requestCounter)

	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Warn(logger.ToContext(context.Background(), logFn()), "not found", "method", r.Method, "path", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(&responses.ErrorResponse{Message: "not found"})
		return
	})

	r.Handle("/metrics", promhttp.Handler())

	return &router{
		router: r,
		logFn:  logFn,
		wsHub:  wsHub,
	}
}

func (r *router) CorsAllowedOrigins() []string {
	return r.corsAllowedOrigins
}

func (r *router) CorsAllowedMethods() []string {
	return r.corsAllowedMethods
}

func (r *router) CorsAllowedHeaders() []string {
	return r.corsAllowedHeaders
}

func (r *router) MuxRouter() *mux.Router {
	return r.router
}

func (r *router) LogFn() *logger.Logger {
	return r.logFn()
}

func (r *router) SetCors(allowedOrigins, allowedMethods, allowedHeaders []string) {
	r.corsAllowedOrigins = allowedOrigins
	r.corsAllowedMethods = allowedMethods
	r.corsAllowedHeaders = allowedHeaders
}

func (r *router) GetWsHub() *ws.Hub {
	return r.wsHub
}
