package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	db "webhook/dynamodb"
	gh "webhook/github"
	"webhook/logger"
	"webhook/util"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/go-github/v66/github"
	"go.opentelemetry.io/otel"
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

	// start trace for function
	tracer := otel.Tracer("application")
	ctx, span := tracer.Start(ctx, "RequesterHasPermission")
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()
	log = *logger.WithTraceContext(log, traceID, spanID)
	defer span.End()

	localLogger := log.With(zap.String("requester", requester), zap.String("repository", repository), zap.String("environment", environment))

	localLogger.Infoln("checking if requester has permission")

	dynamodbClient, err := db.GetDynamoClient(ctx)
	if err != nil {
		localLogger.Errorln("error observed while trying to get dynamodb client", zap.Error(err))
		return nil, err
	}

	hasAccess, err := checkRequesterAccess(ctx, dynamodbClient, requester, repository, environment)
	if err != nil {
		localLogger.Errorln("error observed while checking request access", zap.Error(err))
		return nil, err
	}

	return hasAccess, nil
}

func HandleDeploymentReviewEvent(ctx context.Context, mocking bool, event *github.DeploymentReviewEvent) error {
	// start trace for function
	tracer := otel.Tracer("application")
	ctx, span := tracer.Start(ctx, "HandleDeploymentReviewEvent")
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()
	log = *logger.WithTraceContext(log, traceID, spanID)
	defer span.End()

	// we only handle request review events
	if event.GetAction() != "requested" {
		log.Debug("deployment review event was not for a request", zap.String("action", event.GetAction()))
		return nil
	}

	requester := event.Requester.GetEmail()
	repository := event.Repo.GetName()
	environment := event.GetEnvironment()

	localLogger := log.With(zap.String("requester", requester), zap.String("repository", repository), zap.String("environment", environment))

	localLogger.Infof("Processing event: %T", event)

	if util.AnyStringsEmpty(requester, repository, environment) {
		localLogger.Infoln("an input string was empty")
		return errors.New("an input was string was empty")
	}

	if mocking {
		message := fmt.Sprintf("requester %s has needs a review for %s environment in %s repo!", requester, environment, repository)
		localLogger.Infof("constructed message: %s", message)
		return nil
	} else {
		requesterHasPerm, err := RequesterHasPermission(ctx, requester, repository, environment)
		if err != nil {
			localLogger.Errorln("error observed while checking if requester has permission", zap.Error(err))
			return err
		}

		if requesterHasPerm != nil && *requesterHasPerm {
			localLogger.Info("requester has permission, will attempt to approve deployments")

			err := approveDeploymentReview(ctx, event)
			if err != nil {
				localLogger.Error("error observed while trying to approve deployment review", zap.Error(err))
			}
		}

		// no error yielded, return nil
		return nil
	}
}

/*
*
concurrently checks requester access across three levels
Exact access -> requester has access to the exact repo and environment
Repo access -> requester has access to a repo and all its environments (<repo>#<env> -> <repo>#*)
Env access -> requester has access to an env across all repos (<repo>#<env> -> *#<env>)
Org access -> requester has access to an org, so all repos and all environments (<repo>#<env> -> *#*)
*
*/
func checkRequesterAccess(ctx context.Context, client *dynamodb.Client, requester string, repository string, environment string) (*bool, error) {

	// start trace for function
	tracer := otel.Tracer("application")
	ctx, span := tracer.Start(ctx, "checkRequesterAccess")
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()
	log = *logger.WithTraceContext(log, traceID, spanID)
	defer span.End()

	localLogger := log.With(zap.String("requester", requester), zap.String("repository", repository), zap.String("environment", environment))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // used to cancel other checks once access is found

	accessCheck := make(chan *bool) // channel to store access check(s)
	errChan := make(chan error, 4)  // chanel to store errors

	var wg sync.WaitGroup
	wg.Add(4)

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
			localLogger.Errorln("error observed while trying to check if requester has exact access", zap.Error(err))
			return
		}
		if requesterHasExactAccess != nil && *requesterHasExactAccess {
			localLogger.Infoln("requester has exact access")
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
			localLogger.Errorln("error observed while trying to check if requester has repo level access", zap.Error(err))
			return
		}
		if requesterHasRepoAccess != nil && *requesterHasRepoAccess {
			localLogger.Infoln("requester has repo access")
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
			localLogger.Errorln("error observed while trying to check if requester has org access", zap.Error(err))
			return
		}
		if requesterHasOrgAccess != nil && *requesterHasOrgAccess {
			localLogger.Infoln("requester has org access")
			accessCheck <- requesterHasOrgAccess
			cancel()
		}
	}()

	go func() {
		defer wg.Done()

		// checks if requester has environment access (across all repos)
		repoAccessInput := &dynamodb.GetItemInput{
			TableName: &tableName,
			Key: map[string]types.AttributeValue{
				"email":    &types.AttributeValueMemberS{Value: requester},
				"repo-env": &types.AttributeValueMemberS{Value: strings.Join([]string{"*", environment}, "#")},
			},
		}
		requesterHasRepoAccess, err := checkAccessByInput(ctx, repoAccessInput, client)
		if err != nil {
			errChan <- err
			localLogger.Errorln("error observed while trying to check if requester has environment level access", zap.Error(err))
			return
		}
		if requesterHasRepoAccess != nil && *requesterHasRepoAccess {
			localLogger.Infoln("requester has environment access")
			accessCheck <- requesterHasRepoAccess
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
			localLogger.Infoln("requester has access")
			return result, nil
		}
	}

	// requester did not have any access, return false
	requesterAccess := false
	localLogger.Infoln("requester did not have access")
	return &requesterAccess, nil
}

func checkAccessByInput(ctx context.Context, input *dynamodb.GetItemInput, client *dynamodb.Client) (*bool, error) {
	
	// start trace for function
	tracer := otel.Tracer("application")
	ctx, span := tracer.Start(ctx, "checkAccessByInput")
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()
	log = *logger.WithTraceContext(log, traceID, spanID)
	defer span.End()

	result, err := client.GetItem(ctx, input)
	if err != nil {
		log.Errorln("error observed while trying to get dynamodb item", zap.Error(err), zap.Any("input", *input))
		return nil, fmt.Errorf("observed an error while trying to get dynamodb item")
	}
	hasAccess := (result.Item != nil)
	return &hasAccess, nil
}

/*
*
uses event attributes (owner, repo, runID) to get environment IDs and then use those
environment IDs to approve all pending deployments
*/
func approveDeploymentReview(ctx context.Context, event *github.DeploymentReviewEvent) error {

	// start trace for function
	tracer := otel.Tracer("application")
	ctx, span := tracer.Start(ctx, "approveDeploymentReview")
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()
	log = *logger.WithTraceContext(log, traceID, spanID)
	defer span.End()

	ghClient := gh.GetGitHubClient()

	owner := event.GetOrganization().GetName()
	repo := event.GetRepo().GetName()
	runID := event.WorkflowJobRun.GetID()

	localLogger := log.With(zap.String("owner", owner), zap.String("repo", repo), zap.Int64("runID", runID))

	if util.AnyStringsEmpty(owner, repo) || runID == 0 {
		err := fmt.Errorf("event values continued zero values")
		localLogger.Errorln("error observed while getting values to approve review", zap.Error(err))
		return err
	}

	gotPendingDeployments, getPendDeployResp, gotPendingDeploymentsErr := ghClient.Actions.GetPendingDeployments(ctx, owner, repo, runID)
	if getPendDeployResp.Response.StatusCode != http.StatusOK || gotPendingDeploymentsErr != nil {
		localLogger.Errorln("error or incorrect status code observed while getting pending deployments", zap.Error(gotPendingDeploymentsErr), zap.Int("status_code", getPendDeployResp.Response.StatusCode))
		return gotPendingDeploymentsErr
	}

	var envIDs []int64
	for _, pendingDeployment := range gotPendingDeployments {
		envIDs = append(envIDs, pendingDeployment.GetEnvironment().GetID())
	}

	approvedDeployments, approvalResp, approvalErr := ghClient.Actions.PendingDeployments(ctx, owner, repo, runID, &github.PendingDeploymentsRequest{EnvironmentIDs: envIDs, State: "approved", Comment: "Approved via Go GitHub Webhook Lambda! 🚀"})
	if approvalResp.Response.StatusCode != http.StatusOK || approvalErr != nil {
		localLogger.Error("error or incorrect status code observed while approving deployments", zap.Error(approvalErr), zap.Int("status_code", approvalResp.Response.StatusCode), zap.Int64s("env_IDs", envIDs))
		return approvalErr
	}

	var approvedDeploymentsURLs []string

	for _, approvedDeployment := range approvedDeployments {
		approvedDeploymentsURLs = append(approvedDeploymentsURLs, approvedDeployment.GetURL())
	}
	localLogger.Infoln("approved deployments", zap.Strings("deployment_urls", approvedDeploymentsURLs))

	return nil
}
