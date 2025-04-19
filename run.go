package resty

import (
	"context"
	"fmt"
	"net/http"

	"github.com/porebric/logger"
	"github.com/porebric/resty/closer"
	"github.com/porebric/resty/ws"
	"github.com/rs/cors"
)

func RunServer(ctx context.Context, router Router, closerFns ...func(ctx context.Context) error) {
	c := &closer.Closer{}

	if router.GetWsHub() != nil {
		go router.GetWsHub().Run()

		router.MuxRouter().HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			ws.NewHandler(router.LogFn).ServeWs(router.GetWsHub(), w, r)
		})
	}

	for _, closerFn := range closerFns {
		c.Add(closerFn)
	}

	opt := newOptions(ctx)

	var routerHandler http.Handler

	if len(router.CorsAllowedOrigins()) != 0 || len(router.CorsAllowedMethods()) != 0 || len(router.CorsAllowedHeaders()) != 0 {
		routerHandler = cors.New(cors.Options{
			AllowedOrigins:   router.CorsAllowedOrigins(),
			AllowedMethods:   router.CorsAllowedMethods(),
			AllowedHeaders:   router.CorsAllowedHeaders(),
			AllowCredentials: true,
		}).Handler(router.MuxRouter())
	} else {
		routerHandler = router.MuxRouter()
	}

	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", opt.Port), routerHandler); err != nil {
			logger.Error(ctx, err, "serve")
		}
	}()
	logger.Info(ctx, "start server", "port", opt.Port)
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(logger.ToContext(context.Background(), logger.FromContext(ctx)), opt.Timeout)
	defer cancel()

	if err := c.Close(shutdownCtx); err != nil {
		logger.Error(ctx, err, "shutdown")
	}
	logger.Info(ctx, "stop")
}
