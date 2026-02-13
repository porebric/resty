package resty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/pprof"

	"github.com/gorilla/mux"
	"github.com/porebric/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/porebric/resty/responses"
	"github.com/porebric/resty/ws"
)

// RouteDoc holds metadata for one route (path + methods) for API docs and swagger HTML.
type RouteDoc struct {
	Path         string            `json:"path"`
	Methods      []string          `json:"methods"`
	Summary      string            `json:"summary,omitempty"`
	Description  string            `json:"description,omitempty"`
	BodyExample  string            `json:"bodyExample,omitempty"`
	Responses    map[int]string    `json:"responses,omitempty"`
	QueryParams  map[string]string `json:"queryParams,omitempty"` // GET only: param name (snake_case) -> example value
}

type Router interface {
	MuxRouter() *mux.Router
	LogFn() *logger.Logger
	GetWsHub() *ws.Hub

	SetCors(allowedOrigins, allowedMethods, allowedHeaders []string)
	CorsAllowedOrigins() []string
	CorsAllowedMethods() []string
	CorsAllowedHeaders() []string

	// RegisterDoc records a system endpoint for OpenAPI spec only (not shown on /api/swagger.html).
	RegisterDoc(path string, methods []string, summary, description string, bodyExample string, responses map[int]string)
	// RegisterAppDoc records an endpoint from Endpoint(); used for OpenAPI spec and /api/swagger.html (try-it UI).
	RegisterAppDoc(path string, methods []string, summary, description string, bodyExample string, responses map[int]string, queryParams map[string]string)
	// GetDocRoutes returns all registered route docs for spec generation.
	GetDocRoutes() []RouteDoc
	// GetAppDocRoutes returns only endpoints registered via Endpoint() (for /api/swagger.html).
	GetAppDocRoutes() []RouteDoc
}

type router struct {
	router *mux.Router
	logFn  func() *logger.Logger
	wsHub  *ws.Hub

	corsAllowedOrigins []string
	corsAllowedMethods []string
	corsAllowedHeaders []string
	docRoutes          []RouteDoc
	appDocRoutes       []RouteDoc // only from Endpoint(); shown on /api/swagger.html
}

func NewRouter(logFn func() *logger.Logger, wsHub *ws.Hub) Router {
	r := mux.NewRouter()

	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)

	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Warn(logger.ToContext(context.Background(), logFn()), "not found", "method", r.Method, "path", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(&responses.ErrorResponse{Message: "not found"})
	})

	r.Handle("/metrics", promhttp.Handler())

	rt := &router{
		router:       r,
		logFn:        logFn,
		wsHub:        wsHub,
		docRoutes:    make([]RouteDoc, 0),
		appDocRoutes: make([]RouteDoc, 0),
	}
	rt.registerSwagger()
	return rt
}

func (r *router) RegisterDoc(path string, methods []string, summary, description string, bodyExample string, responses map[int]string) {
	r.docRoutes = append(r.docRoutes, RouteDoc{
		Path:        path,
		Methods:     methods,
		Summary:     summary,
		Description: description,
		BodyExample: bodyExample,
		Responses:   responses,
	})
}

func (r *router) RegisterAppDoc(path string, methods []string, summary, description string, bodyExample string, responses map[int]string, queryParams map[string]string) {
	doc := RouteDoc{
		Path:        path,
		Methods:     methods,
		Summary:     summary,
		Description: description,
		BodyExample: bodyExample,
		Responses:   responses,
		QueryParams: queryParams,
	}
	r.docRoutes = append(r.docRoutes, doc)
	r.appDocRoutes = append(r.appDocRoutes, doc)
}

func (r *router) GetDocRoutes() []RouteDoc {
	return r.docRoutes
}

func (r *router) GetAppDocRoutes() []RouteDoc {
	return r.appDocRoutes
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

func (r *router) registerSwagger() {
	r.router.HandleFunc("/api/swagger.html", r.serveSwaggerHTML).Methods("GET")
}
