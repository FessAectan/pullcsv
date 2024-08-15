package http

import (
	"context"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net"
	"net/http"
)

func pullcsvServeMux(metricsHandler http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/metrics", metricsHandler)

	return mux
}

func pullcsvHTTPServer(lc fx.Lifecycle, mux *http.ServeMux, logger *zap.Logger) *http.Server {
	srv := &http.Server{Addr: ":8080", Handler: mux}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}
			logger.Info("Starting HTTP server at " + srv.Addr)
			go srv.Serve(ln)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
	return srv
}

func WithHttpServiceFx() fx.Option {
	return fx.Options(
		fx.Provide(pullcsvServeMux),
		fx.Provide(pullcsvHTTPServer),
		fx.Invoke(func(*http.Server) {}),
	)
}
