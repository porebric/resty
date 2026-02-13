package resty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"reflect"
	"strings"

	"github.com/porebric/logger"
	"github.com/porebric/tracer"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/porebric/resty/errors"
	"github.com/porebric/resty/middleware"
	"github.com/porebric/resty/requests"
	"github.com/porebric/resty/responses"
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

func init() {
	prometheus.MustRegister(requestCounter)
}

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

		ctx, resp, httpCode = checkAction(ctx, req, w, mm...)
		if httpCode != 0 {
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

const defaultSuccessResponseJSON = `{"success":true,"message":""}`

// getQueryParamsFromRequest extracts query param names in snake_case from the request type's json tags.
// Used for GET endpoints so /api/swagger.html can show query param inputs.
func getQueryParamsFromRequest[R requests.Request](r R) map[string]string {
	v := any(r)
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr && val.IsNil() {
		val = reflect.New(val.Type().Elem())
	}
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}
	out := make(map[string]string)
	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "" {
			continue
		}
		snake := toSnakeCase(name)
		out[snake] = ""
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func toSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

// marshalEmptyRequest produces JSON for the request type's zero value (empty struct with json tags).
// If R is a pointer type and nil, a new instance is created so the full structure is emitted.
func marshalEmptyRequest[R requests.Request](r R) string {
	v := any(r)
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr && val.IsNil() {
		val = reflect.New(val.Type().Elem())
		v = val.Interface()
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	if string(data) == "null" {
		return "{}"
	}
	return string(data)
}

func Endpoint[R requests.Request](r Router, req func(ctx context.Context, r *http.Request) (context.Context, R, error), action func(context.Context, R) (responses.Response, int), mm ...func() middleware.Middleware) {
	var exampleReq R
	path, _ := exampleReq.Path()
	methods := exampleReq.Methods()
	r.MuxRouter().HandleFunc(path, serveHTTP(action, r.LogFn, req, mm...)).Methods(methods...)
	summary, description := "", ""
	if doc, ok := any(exampleReq).(requests.OpenAPIDoc); ok {
		summary, description = doc.OpenAPISummary(), doc.OpenAPIDescription()
	}
	bodyExample := marshalEmptyRequest(exampleReq)
	if b, ok := any(exampleReq).(requests.OpenAPIBody); ok {
		bodyExample = b.OpenAPIBody()
	}
	respMap := map[int]string{200: defaultSuccessResponseJSON}
	if ro, ok := any(exampleReq).(requests.OpenAPIResponses); ok {
		for code, jsonStr := range ro.OpenAPIResponses() {
			respMap[code] = jsonStr
		}
	}
	var queryParams map[string]string
	for _, m := range methods {
		if m == "GET" {
			queryParams = getQueryParamsFromRequest(exampleReq)
			break
		}
	}
	r.RegisterAppDoc(path, methods, summary, description, bodyExample, respMap, queryParams)
}
