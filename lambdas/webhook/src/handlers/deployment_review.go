package handlers

import (
	"context"
	"fmt"
	"strings"
	db "webhook/dynamodb"
	"webhook/util"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

var (
	log zap.SugaredLogger

	tableName string
)

const (
	TABLE_NAME_ENV_VAR_KEY = "DYNAMO_DB_TABLE_NAME"
	TABLE_NAME_DEFAULT     = "deployment-webhooks-table"
)

func init() {
	tableName = util.LookupEnv(TABLE_NAME_ENV_VAR_KEY, TABLE_NAME_DEFAULT, false)
}

func UserHasPermission(ctx context.Context, userEmail string, repository string, environment string) (*bool, error) {
	log.Infoln("checking if user has permission", zap.String("email", userEmail), zap.String("repository", repository), zap.String("environment", environment))

	partitionKey := userEmail // (hash_key in tf)
	sortKey := strings.Join([]string{repository, environment}, "#")

	PKzapField := zap.String("partitionKey", partitionKey)
	SKzapField := zap.String("sortKey", sortKey)

	log.Infoln("constructed keys to search DB", PKzapField, SKzapField)

	dynamodbClient, err := db.GetDynamoClient(ctx)
	if err != nil {
		log.Errorln("error observed while trying to get dynamodb client", zap.Error(err), PKzapField, SKzapField)
		return nil, err
	}

	input := &dynamodb.GetItemInput{
		TableName: &tableName,
		Key: map[string]types.AttributeValue{
			"email":    &types.AttributeValueMemberS{Value: partitionKey},
			"repo-env": &types.AttributeValueMemberS{Value: sortKey},
		},
	}

	result, err := dynamodbClient.GetItem(ctx, input)
	if err != nil {
		log.Errorln("error observed while trying to get dynamodb item", zap.Error(err), zap.Any("input", *input))
		return nil, fmt.Errorf("observed an error while trying to get dynamodb item")
	}

	userHasPerm := (result.Item != nil)

	return &userHasPerm, nil
}
