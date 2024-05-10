package resty

import (
	"context"
	"encoding/json"
	"fmt"
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

func checkAction(ctx context.Context, r *http.Request, req requests.Request, w http.ResponseWriter) (context.Context, requests.Request) {
	checkRequest := &middleware.RequestCheck{}
	for i := 0; i < len(additionalMiddlewares); i++ {
		if i+1 == len(additionalMiddlewares) {
			additionalMiddlewares[i].SetNext(checkRequest)
			break
		}
		additionalMiddlewares[i].SetNext(additionalMiddlewares[i+1])
	}

	initRequest := middleware.NewRequestInit(r)
	initRequest.SetNext(additionalMiddlewares[0])

	ctx, code, msg := initRequest.Execute(ctx, req)

	if code != errors.ErrorNoError {
		resp, httpCode := errors.GetCustomError(msg, code)
		w.WriteHeader(httpCode)
		_ = json.NewEncoder(w).Encode(resp)
		return ctx, nil
	}

	return ctx, req
}
