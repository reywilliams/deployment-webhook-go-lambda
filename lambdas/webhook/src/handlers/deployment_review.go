package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	db "webhook/dynamodb"
	"webhook/logger"
	"webhook/util"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/go-github/v66/github"
	"go.uber.org/zap"
)

var (
	log       zap.SugaredLogger
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

func RequesterHasPermission(ctx context.Context, requester string, repository string, environment string) (*bool, error) {
	log.Infoln("checking if requester has permission", zap.String("requester", requester), zap.String("repository", repository), zap.String("environment", environment))

	if util.AnyStringsEmpty(requester, repository, environment) {
		log.Infoln("an input string was empty", zap.String("requester", requester), zap.String("repository", repository), zap.String("environment", environment))
		return nil, errors.New("an input was string was empty")
	}

	dynamodbClient, err := db.GetDynamoClient(ctx)
	if err != nil {
		log.Errorln("error observed while trying to get dynamodb client", zap.Error(err))
		return nil, err
	}

	hasAccess, err := checkRequesterAccess(ctx, dynamodbClient, requester, repository, environment)
	if err != nil {
		log.Errorln("error observed while checking request access", zap.Error(err))
		return nil, err
	}

	return hasAccess, nil
}

func HandleDeploymentReviewEvent(ctx context.Context, mocking bool, event *github.DeploymentReviewEvent) error {
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

		return nil
	} else {
		requesterHasPerm, err := RequesterHasPermission(ctx, requester, repository, environment)
		if err != nil {
			log.Errorln("error observed while checking if requester has permission", zap.Error(err))
			return err
		}

		if requesterHasPerm != nil && *requesterHasPerm {
			log.Info("requester has permission", zap.String("requester", requester), zap.String("repository", repository), zap.String("environment", environment))
		}
		return nil
	}

}

/*
*
concurrently checks requester access across three levels
Exact access -> requester has access to the exact repo and environment
Repo access -> requester has access to a repo and all its environments (<repo>#<env> -> <repo>#*)
Org access -> requester has access to a org, so all repos and all environments (<repo>#<env> -> *#*)
*
*/
func checkRequesterAccess(ctx context.Context, client *dynamodb.Client, requester string, repository string, environment string) (*bool, error) {

	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // used to cancel other checks once access is found

	accessCheck := make(chan *bool) // channel to store access check(s)
	errChan := make(chan error, 3)  // chanel to store errors

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		// checks if requester has exact access to a specific environment in a repository
		exactAccessInput := &dynamodb.GetItemInput{
			TableName: &tableName,
			Key: map[string]types.AttributeValue{
				"email":    &types.AttributeValueMemberS{Value: requester},
				"repo-env": &types.AttributeValueMemberS{Value: strings.Join([]string{repository, environment}, "#")},
			},
		}
		requesterHasExactAccess, err := checkAccessByInput(ctx, exactAccessInput, client)
		if err != nil {
			errChan <- err
			log.Errorln("error observed while trying to check if requester has exact access", zap.Error(err))
			return
		}
		if requesterHasExactAccess != nil && *requesterHasExactAccess {
			log.Infoln("requester has exact access", zap.String("requester", requester), zap.String("repository", repository), zap.String("environment", environment))
			accessCheck <- requesterHasExactAccess
			cancel()
		}

	}()

	go func() {
		defer wg.Done()

		// checks if requester has repo access (and thus access to all environments)
		repoAccessInput := &dynamodb.GetItemInput{
			TableName: &tableName,
			Key: map[string]types.AttributeValue{
				"email":    &types.AttributeValueMemberS{Value: requester},
				"repo-env": &types.AttributeValueMemberS{Value: strings.Join([]string{repository, "*"}, "#")},
			},
		}
		requesterHasRepoAccess, err := checkAccessByInput(ctx, repoAccessInput, client)
		if err != nil {
			errChan <- err
			log.Errorln("error observed while trying to check if requester has repo level access", zap.Error(err))
			return
		}
		if requesterHasRepoAccess != nil && *requesterHasRepoAccess {
			log.Infoln("requester has repo access", zap.String("requester", requester), zap.String("repository", repository), zap.String("environment", environment))
			accessCheck <- requesterHasRepoAccess
			cancel()
		}
	}()

	go func() {
		defer wg.Done()

		// checks if requester has org access (access to all repos and all environments)
		orgAccessInput := &dynamodb.GetItemInput{
			TableName: &tableName,
			Key: map[string]types.AttributeValue{
				"email":    &types.AttributeValueMemberS{Value: requester},
				"repo-env": &types.AttributeValueMemberS{Value: strings.Join([]string{"*", "*"}, "#")},
			},
		}
		requesterHasOrgAccess, err := checkAccessByInput(ctx, orgAccessInput, client)
		if err != nil {
			errChan <- err
			log.Errorln("error observed while trying to check if requester has org access", zap.Error(err))
			return
		}
		if requesterHasOrgAccess != nil && *requesterHasOrgAccess {
			log.Infoln("requester has org access", zap.String("requester", requester), zap.String("repository", repository), zap.String("environment", environment))
			accessCheck <- requesterHasOrgAccess
			cancel()
		}
	}()

	go func() {
		wg.Wait()
		close(accessCheck)
		close(errChan)
	}()

	var firstErr error
	for {
		select { // read from error and access chanel
		case result, ok := <-accessCheck:
			if ok && result != nil && *result {
				return result, nil // determined access, exit early
			}
		case err, ok := <-errChan:
			if ok && err != nil {
				if firstErr == nil {
					firstErr = err // save first err as err to return
				}
			}
		}

		// if we've reviewed all the access we can
		// and we've reviewed all the errors OR just got one
		// then break
		if len(accessCheck) == 0 && (len(errChan) == 0 || firstErr != nil) {
			break
		}
	}

	for result := range accessCheck {
		if result != nil && *result {
			return result, nil
		}
	}

	// user did not have any access, return false
	userAccess := false
	return &userAccess, nil
}

func checkAccessByInput(ctx context.Context, input *dynamodb.GetItemInput, client *dynamodb.Client) (*bool, error) {
	result, err := client.GetItem(ctx, input)
	if err != nil {
		log.Errorln("error observed while trying to get dynamodb item", zap.Error(err), zap.Any("input", *input))
		return nil, fmt.Errorf("observed an error while trying to get dynamodb item")
	}
	hasAccess := (result.Item != nil)
	return &hasAccess, nil
}
