package secrets

import (
	"context"
	"os"
	"sync"
	"webhook/logger"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-secretsmanager-caching-go/v2/secretcache"
	"github.com/aws/aws-xray-sdk-go/instrumentation/awsv2"
	"github.com/aws/aws-xray-sdk-go/xray"
	"go.uber.org/zap"
)

var (
	secretCacheInstance  *secretcache.Cache
	secretClientInstance *secretsmanager.Client

	logInstance *zap.SugaredLogger

	once sync.Once
)

func init() {
	logInstance = logger.GetLogger().Sugar()
}

func getSecretCache(ctx context.Context) (*secretcache.Cache, error) {
	if err := configureSecretCache(ctx); err != nil {
		logInstance.Errorln("error observed while trying to get secretsmanager client", zap.Error(err))
		return nil, err
	}
	return secretCacheInstance, nil
}

func configureSecretCache(ctx context.Context) error {
	var returnedErr error

	// ensures only one secret client (and cache) instance is created
	once.Do(func() {

		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("AWS_REGION")))
		if err != nil {
			returnedErr = err
			logInstance.Errorln("unable to load default SDK config for secret client", zap.Error(err))
			return
		}
		awsv2.AWSV2Instrumentor(&cfg.APIOptions)
		secretClientInstance = secretsmanager.NewFromConfig(cfg)

		config := secretcache.CacheConfig{
			MaxCacheSize: 10,
			VersionStage: secretcache.DefaultVersionStage,
			CacheItemTTL: secretcache.DefaultCacheItemTTL,
		}

		cache, err := secretcache.New(
			func(c *secretcache.Cache) { c.CacheConfig = config },
			func(c *secretcache.Cache) { c.Client = secretClientInstance },
		)
		if err != nil {
			logInstance.Errorln("unable to construct a secret cache instance", zap.Error(err))
			returnedErr = err
			return
		}
		secretCacheInstance = cache
	})

	return returnedErr
}

func GetSecretValue(ctx context.Context, secretName string) (*string, error) {
	funcLogger := logInstance.With(zap.String("secret_name", secretName))
	funcLogger.Infoln("getting secret value")

	_, subSegment := xray.BeginSubsegment(ctx, "RequesterHasPermission")
	if subSegment != nil {
		traceID := subSegment.TraceID
		funcLogger = logInstance.With(zap.String("traceID", traceID))
		defer subSegment.Close(nil)
	}

	cache, err := getSecretCache(ctx)
	if err != nil {
		funcLogger.Errorln("error while string to get secret client", zap.Error(err))
		return nil, err
	}

	secretValue, err := cache.GetSecretStringWithContext(ctx, secretName)
	if err != nil {
		funcLogger.Errorln("error while string to get secret string", zap.Error(err))
		return nil, err
	}

	return &secretValue, nil
}
