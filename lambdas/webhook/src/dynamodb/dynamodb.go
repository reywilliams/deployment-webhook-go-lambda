package dynamodb

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"webhook/logger"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

var (
	DYNAMO_DB_TABLE_NAME   string
	dynamoDbClientInstance *dynamodb.Client

	log zap.SugaredLogger

	once sync.Once
)

const (
	DYNAMO_DB_TABLE_NAME_ENV_VAR_KEY = "DYNAMO_DB_TABLE_NAME"
	DYNAMO_DB_TABLE_NAME_DEFAULT     = "deployment-webhooks-table"
)

func init() {
	log = *logger.GetLogger().Sugar()
	source_dynamo_db_table_name() // sources DYNAMO_DB_TABLE_NAME env. variable
}

func UserHasPermission(ctx context.Context, userEmail string, repository string, environment string) (*bool, error) {
	log.Infoln("checking if user has permission", zap.String("email", userEmail), zap.String("repository", repository), zap.String("environment", environment))

	partitionKey := userEmail // (hash_key in tf)
	sortKey := strings.Join([]string{repository, environment}, "#")

	log.Infoln("constructed keys to search DB", zap.String("partitionKey", partitionKey), zap.String("sortKey", sortKey))

	dynamodbClient, err := getDynamoClient(ctx)
	if err != nil {
		log.Errorln("error observed while trying to get dynamodb client", zap.Error(err))
		return nil, err
	}

	input := dynamodb.GetItemInput{
		TableName: &DYNAMO_DB_TABLE_NAME,
		Key: map[string]types.AttributeValue{
			"email":    &types.AttributeValueMemberS{Value: partitionKey},
			"repo-env": &types.AttributeValueMemberS{Value: sortKey},
		},
	}

	result, err := dynamodbClient.GetItem(ctx, &input)
	if err != nil {
		log.Errorln("error observed while trying to get dynamodb item", zap.Error(err))
		return nil, fmt.Errorf("observed an error while trying to get dynamodb item")
	}

	userHasPerm := (result.Item != nil)

	return &userHasPerm, nil
}

func getDynamoClient(ctx context.Context) (*dynamodb.Client, error) {
	if err := configureDynamoDbClient(ctx); err != nil {
		log.Errorln("error observed while trying to get dynamodb client", zap.Error(err))
		return nil, err
	}
	return dynamoDbClientInstance, nil
}

func configureDynamoDbClient(ctx context.Context) error {
	var err error

	// ensures only one dynamodb client instance is created
	once.Do(func() {
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("AWS_REGION")))
		if err != nil {
			log.Errorln("unable to load default SDK config", zap.Error(err))
			return
		}
		dynamoDbClientInstance = dynamodb.NewFromConfig(cfg)
	})

	return err
}

/*
Retrieves DYNAMO_DB_TABLE_NAME environment variable, sets global var (DYNAMO_DB_TABLE_NAME) to:
-> DYNAMO_DB_TABLE_NAME env var value if it is not empty
-> DYNAMO_DB_TABLE_NAME_DEFAULT if empty
*/
func source_dynamo_db_table_name() {
	envSourcedTableName := os.Getenv(DYNAMO_DB_TABLE_NAME_ENV_VAR_KEY)
	log.Debugln("sourced dynamodb table name environment variable: %s", envSourcedTableName)

	if envSourcedTableName == "" {
		log.Debugln("dynamodb table name falling back to default value: %s", DYNAMO_DB_TABLE_NAME_DEFAULT)
		DYNAMO_DB_TABLE_NAME = DYNAMO_DB_TABLE_NAME_DEFAULT
	} else {
		DYNAMO_DB_TABLE_NAME = envSourcedTableName
	}
}
