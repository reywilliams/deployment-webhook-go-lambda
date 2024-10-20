package logger

import (
	"fmt"
	"os"
	"runtime"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/aws/aws-xray-sdk-go/xraylog"
)

var (
	instance *zap.Logger
	once     sync.Once
)

type (
	xrayZapLogger struct{}
)

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

func InitializeXRay() {

	devEnv := os.Getenv("environment") == "dev"

	if devEnv {
		os.Setenv("AWS_XRAY_SDK_DISABLED", "true")
	}
	
	xray.Configure(xray.Config{
		ServiceVersion: "1.0.0",
	})

	xray.SetLogger(&xrayZapLogger{})
}

func (x *xrayZapLogger) Log(ll xraylog.LogLevel, msg fmt.Stringer) {
	logger := GetLogger().Sugar()
	switch ll {
	case xraylog.LogLevelDebug:
		logger.Debugln(msg.String())
	case xraylog.LogLevelInfo:
		logger.Infoln(msg.String())
	case xraylog.LogLevelWarn:
		logger.Warnln(msg.String())
	case xraylog.LogLevelError:
		logger.Errorln(msg.String())
	}
}
