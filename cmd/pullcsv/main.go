package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"os"
	"pullcsv/internal/http"
	"pullcsv/internal/logger"
	"pullcsv/internal/prom"
	"pullcsv/internal/pullcsv"
)

const DISPLAY_VERSION = "2.0.1"

func main() {
	fx.New(
		logger.WithZapLoggerFx(),
		prom.WithPromFx(),
		http.WithHttpServiceFx(),
		fx.Invoke(func(logger *zap.Logger, metrics *prom.Metrics) {
			logger.Info("running PullCSV version " + DISPLAY_VERSION)
			metrics.Info.With(prometheus.Labels{"version": DISPLAY_VERSION, "stand_name": os.Getenv("STAND_NAME"), "pod_name": os.Getenv("POD_NAME")}).Set(1)
		}),
		fx.Invoke(pullcsv.Pullcsv),
	).Run()
}
