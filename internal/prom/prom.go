package prom

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
	"net/http"
)

type Metrics struct {
	MaxModifiedFileLifetime *prometheus.GaugeVec
	MinModifiedFileLifetime *prometheus.GaugeVec
	CountFiles              *prometheus.GaugeVec
	RsyncCSVStartTime       *prometheus.GaugeVec
	RsyncCSVStopTime        *prometheus.GaugeVec
	RsyncEXfileStartTime    *prometheus.GaugeVec
	RsyncEXfileStopTime     *prometheus.GaugeVec
	RsyncCSVExitCode        *prometheus.GaugeVec
	RsyncEXfileExitCode     *prometheus.GaugeVec
	Info                    *prometheus.GaugeVec
}

func pullcsvMetrics() (http.Handler, *Metrics) {
	m := &Metrics{
		MaxModifiedFileLifetime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pullcsv",
			Name:      "folder_sentry_max_modified_file_lifetime",
			Help:      "Unix time of the oldest file in DOWNLOAD_TO folder.",
		},
			[]string{"path", "stand_name", "pod_name"}),
		MinModifiedFileLifetime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pullcsv",
			Name:      "folder_sentry_min_modified_file_lifetime",
			Help:      "Unix time of the newest file in DOWNLOAD_TO folder.",
		},
			[]string{"path", "stand_name", "pod_name"}),
		CountFiles: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pullcsv",
			Name:      "folder_sentry_file_count",
			Help:      "How many files in DOWNLOAD_TO folder.",
		},
			[]string{"path", "stand_name", "pod_name"}),
		RsyncCSVStartTime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pullcsv",
			Name:      "rsync_download_csv_start_time",
			Help:      "Rsync start time (pulling CSV files).",
		},
			[]string{"path", "stand_name", "pod_name"}),
		RsyncCSVStopTime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pullcsv",
			Name:      "rsync_download_csv_stop_time",
			Help:      "Rsync stop time (pulling CSV files).",
		},
			[]string{"path", "stand_name", "pod_name"}),
		RsyncCSVExitCode: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pullcsv",
			Name:      "rsync_download_csv_exit_code",
			Help:      "Rsync exit code (pulling CSV files).",
		},
			[]string{"path", "stand_name", "pod_name"}),
		RsyncEXfileStartTime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pullcsv",
			Name:      "rsync_upload_exclude_file_start_time",
			Help:      "Rsync start time (uploading exclude file).",
		},
			[]string{"path", "stand_name", "pod_name"}),
		RsyncEXfileStopTime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pullcsv",
			Name:      "rsync_upload_exclude_file_stop_time",
			Help:      "Rsync stop time (uploading exclude file).",
		},
			[]string{"path", "stand_name", "pod_name"}),
		RsyncEXfileExitCode: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pullcsv",
			Name:      "rsync_upload_exclude_file_exit_code",
			Help:      "Rsync exit code (uploading exclude file).",
		},
			[]string{"path", "stand_name", "pod_name"}),
		Info: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pullcsv",
			Name:      "info",
			Help:      "Information about the pullcsv's version.",
		},
			[]string{"version", "stand_name", "pod_name"}),
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		m.MaxModifiedFileLifetime,
		m.MinModifiedFileLifetime,
		m.CountFiles,
		m.RsyncCSVStartTime,
		m.RsyncCSVStopTime,
		m.RsyncCSVExitCode,
		m.RsyncEXfileStartTime,
		m.RsyncEXfileStopTime,
		m.RsyncEXfileExitCode,
		m.Info,
	)

	metricsHandler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})

	return metricsHandler, m
}

func WithPromFx() fx.Option {
	return fx.Options(
		fx.Provide(pullcsvMetrics),
	)
}
