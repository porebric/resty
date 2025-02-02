package resty

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/porebric/resty/responses"
	"net/http"
	"runtime/debug"

	"github.com/porebric/logger"
	"github.com/porebric/resty/errors"
	"github.com/porebric/resty/middleware"
	"github.com/porebric/resty/requests"
)

func getDeferCatchPanic(log *logger.Logger, w http.ResponseWriter) {
	if rec := recover(); rec != any(nil) {
		logger.Error(
			logger.ToContext(context.Background(), log),
			fmt.Errorf("error: %v", rec), "critical error", "stacktrace", string(debug.Stack()),
		)
		resp, httpCode := errors.GetCustomError("", errors.ErrorCritical)
		w.WriteHeader(httpCode)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}
}

func checkAction[R requests.Request](ctx context.Context, req R, w http.ResponseWriter, mm ...func() middleware.Middleware) (context.Context, *responses.ErrorResponse, int) {

	middlewares := make([]middleware.Middleware, 1, len(mm)+1)
	middlewares[0] = new(middleware.RequestValidate)

	if len(mm) == 0 {
		return execute(ctx, middlewares, req, w)
	}

	for _, m := range mm {
		newMiddleware := m()
		middlewares[len(middlewares)-1].SetNext(newMiddleware)

		middlewares = append(middlewares, newMiddleware)
	}

	return execute(ctx, middlewares, req, w)
}

func execute(ctx context.Context, mm []middleware.Middleware, req requests.Request, w http.ResponseWriter) (context.Context, *responses.ErrorResponse, int) {
	checkRequest := &middleware.RequestCheck{}
	mm[len(mm)-1].SetNext(checkRequest)
	ctx, code, msg := mm[0].Execute(ctx, req)

	if code != errors.ErrorNoError {
		resp, httpCode := errors.GetCustomError(msg, code)
		if httpCode == 0 {
			logger.Warn(ctx, "invalid middleware http code", "code", code)
			httpCode = http.StatusBadRequest
		}
		return ctx, resp, httpCode
	}

	return ctx, nil, 0
}
