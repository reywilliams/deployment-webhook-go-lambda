package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	db "webhook/dynamodb"
	"webhook/logger"
	"webhook/util"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/go-github/v66/github"
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
	log = *logger.GetLogger().Sugar()
	tableName = util.LookupEnv(TABLE_NAME_ENV_VAR_KEY, TABLE_NAME_DEFAULT, false)
}

func RequesterHasPermission(ctx context.Context, requesterEmail string, repository string, environment string) (*bool, error) {
	log.Infoln("checking if requester has permission", zap.String("email", requesterEmail), zap.String("repository", repository), zap.String("environment", environment))

	if util.AnyStringsEmpty(requesterEmail, repository, environment) {
		log.Infoln("an input string was empty")
		return nil, errors.New("an input was string was empty")
	}

	partitionKey := requesterEmail // (hash_key in tf)
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

	requesterHasPerm := (result.Item != nil)

	return &requesterHasPerm, nil
}

func HandleDeploymentReviewEvent(ctx context.Context, mocking bool, event *github.DeploymentReviewEvent) {
	log.Infof("Processing event: %T", event)

	requester := event.Requester.GetEmail()
	repository := event.Repo.GetName()
	environment := event.GetEnvironment()

	if mocking {
		var message string

		if util.AnyStringsEmpty(requester, repository, environment) {
			log.Infoln("an input string was empty, using fall back message for deployment_review event")
			message = "fall back message as there were null pointers."
		} else {
			message = fmt.Sprintf("requester %s has needs a review for %s environment in %s repo!", *event.Requester.Name, *event.Environment, *event.Repo.Name)
		}
		log.Infof("constructed message: %s", message)
	} else {
		requesterHasPerm, err := RequesterHasPermission(ctx, requester, repository, environment)
		if err != nil {
			log.Errorln("error observed while checking if requester has permission", zap.Error(err))
		}

		if requesterHasPerm != nil && *requesterHasPerm {
			log.Info("requester has permission", zap.String("requester", requester), zap.String("repository", repository), zap.String("environment", environment))
		}

	}

}
