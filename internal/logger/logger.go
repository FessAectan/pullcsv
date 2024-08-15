package logger

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var zapLog *zap.Logger

func WithZapLoggerFx() fx.Option {
	return fx.Options(
		fx.Provide(func() *zap.Logger {
			var err error
			//config := zap.NewProductionConfig() // uncomment this line and comment out next line in order to use JSON logs
			config := zap.NewDevelopmentConfig()
			enccoderConfig := zap.NewProductionEncoderConfig()
			enccoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
			enccoderConfig.StacktraceKey = "" // to hide stacktrace info
			config.EncoderConfig = enccoderConfig

			zapLog, err = config.Build(zap.AddCallerSkip(1))
			if err != nil {
				panic(err)
			}
			defer zapLog.Sync()

			return zapLog
		}),
	)
}

func Info(message string, fields ...zap.Field) {
	zapLog.Info(message, fields...)
}

func Warn(message string, fields ...zap.Field) {
	zapLog.Warn(message, fields...)
}

func Debug(message string, fields ...zap.Field) {
	zapLog.Debug(message, fields...)
}

func Error(message string, fields ...zap.Field) {
	zapLog.Error(message, fields...)
}

func Fatal(message string, fields ...zap.Field) {
	zapLog.Fatal(message, fields...)
}
