package logger

import (
	"runtime"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var instance *zap.Logger
var once sync.Once

func GetLogger() *zap.Logger {
	once.Do(func() {
		prodEncoderConfig := zap.NewProductionEncoderConfig()
		prodEncoderConfig.TimeKey = "timestamp"
		prodEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		config := zap.Config{
			Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
			Sampling:         nil,
			Encoding:         "json",
			EncoderConfig:    prodEncoderConfig,
			OutputPaths:      []string{"stderr"},
			ErrorOutputPaths: []string{"stderr"},
			InitialFields:    map[string]interface{}{"go_version": runtime.Version()},
		}

		instance = zap.Must(config.Build())
	})

	return instance
}
