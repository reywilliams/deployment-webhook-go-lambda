package db

import (
	"context"
	"os"
	"sync"
	"webhook/logger"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-xray-sdk-go/instrumentation/awsv2"
	"go.uber.org/zap"
)

var (
	dynamoDbClientInstance *dynamodb.Client

	log zap.SugaredLogger

	once sync.Once
)

func init() {
	log = *logger.GetLogger().Sugar()
}

func GetDynamoClient(ctx context.Context) (*dynamodb.Client, error) {
	if err := configureDynamoDbClient(ctx); err != nil {
		log.Errorln("error observed while trying to get dynamodb client", zap.Error(err))
		return nil, err
	}
	return dynamoDbClientInstance, nil
}

func configureDynamoDbClient(ctx context.Context) error {
	var returnedErr error

	// ensures only one dynamodb client instance is created
	once.Do(func() {
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("AWS_REGION")))
		if err != nil {
			returnedErr = err
			log.Errorln("unable to load default SDK config for db client", zap.Error(err))
			return
		}
		awsv2.AWSV2Instrumentor(&cfg.APIOptions)
		dynamoDbClientInstance = dynamodb.NewFromConfig(cfg)
	})

	return returnedErr
}
