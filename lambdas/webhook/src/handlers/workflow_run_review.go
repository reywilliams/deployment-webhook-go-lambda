package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"webhook/db"
	gh "webhook/github"
	"webhook/logger"
	"webhook/util"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/google/go-github/v66/github"
	"go.uber.org/zap"
)

var (
	logInstance *zap.SugaredLogger
	tableName   string
	Current     WorkflowRun

	ghClient       *github.Client
	dynamodbClient *dynamodb.Client
)

const (
	TABLE_NAME_ENV_VAR_KEY = "DYNAMO_DB_TABLE_NAME"
	TABLE_NAME_DEFAULT     = "deployment-webhooks-table"
	REQUESTED_ACTION       = "requested"
)

type WorkflowRun struct {
	repository string
	ID         int64
	owner      string
	requester  string
}

func init() {
	logInstance = logger.GetLogger().Sugar()
	tableName = util.LookupEnv(TABLE_NAME_ENV_VAR_KEY, TABLE_NAME_DEFAULT, false)
	Current = WorkflowRun{}
}

func HandleWorkflowRunEvent(ctx context.Context, mocking bool, event *github.WorkflowRunEvent) error {
	funcLogger := logInstance.With()

	// if not mocking, set up clients. when mocking clients will be stubbed clients
	if !mocking {
		clientSetupErr := setupClients(ctx)
		if clientSetupErr != nil {
			funcLogger.Errorln("error while setting up clients")
			return clientSetupErr
		}
	}

	_, subSegment := xray.BeginSubsegment(ctx, "HandleWorkflowRunEvent")
	if subSegment != nil {
		traceID := subSegment.TraceID
		funcLogger = logInstance.With(zap.String("traceID", traceID))
		defer subSegment.Close(nil)
	}

	// we only handle request review events
	if event.GetAction() != REQUESTED_ACTION {
		funcLogger.Debug("event was not for a request", zap.String("action", event.GetAction()))
		return nil
	}

	// get the requestor and their repo
	if event.GetSender() != nil && event.GetSender().GetLogin() != "" {
		Current.requester = event.GetSender().GetLogin()
	} else {
		err := fmt.Errorf("sender or sender login from event payload is nil or empty")
		funcLogger.Errorln("invalid field", zap.Error(err))
		return err
	}
	if event.GetRepo() != nil && event.GetRepo().GetName() != "" {
		Current.repository = event.Repo.GetName()
	} else {
		err := fmt.Errorf("repo or repo name from event payload is nil or empty")
		funcLogger.Errorln("invalid field", zap.Error(err))
		return err
	}

	logInstance = logInstance.With(zap.String("requester", Current.requester), zap.String("repository", Current.repository))
	funcLogger = logInstance.With()

	pendingDeployments, err := getPendingDeployments(ctx, event)
	if err != nil {
		funcLogger.Errorln("error while fetching pending deployments to handle workflow run event")
		return err
	}

	funcLogger.Infof("Processing event: %T", event)

	// approve deployments for environments where requester has access
	for _, pendingDeployment := range pendingDeployments {

		var environment string
		if pendingDeployment.GetEnvironment() != nil && pendingDeployment.GetEnvironment().GetName() != "" {
			environment = pendingDeployment.GetEnvironment().GetName()
		} else {
			// skip invalid or empty environment names
			continue
		}

		// check if requestor (sender) has permission for repo/env
		requesterHasPerm, err := requesterHasPermission(ctx, environment)
		if err != nil {
			funcLogger.Errorln("error observed while checking if requester has permission", zap.Error(err))
			return err
		}

		// approve the pending deployment if user has permission
		if requesterHasPerm != nil && *requesterHasPerm {
			funcLogger.Info("requester has permission, will attempt to approve pending deployment")

			err := approvePendingDeployment(ctx, pendingDeployment)
			if err != nil {
				funcLogger.Error("error observed while trying to approve pending deployment", zap.Error(err))
				return err
			}
		}
	}

	// no error yielded, return nil
	return nil
}

func requesterHasPermission(ctx context.Context, environment string) (*bool, error) {
	funcLogger := logInstance.With(zap.String("environment", environment))
	funcLogger.Infoln("checking if requester has permission")

	_, subSegment := xray.BeginSubsegment(ctx, "RequesterHasPermission")
	if subSegment != nil {
		traceID := subSegment.TraceID
		funcLogger = logInstance.With(zap.String("traceID", traceID))
		defer subSegment.Close(nil)
	}

	hasAccess, err := checkRequesterAccess(ctx, strings.ToLower(Current.requester), strings.ToLower(Current.repository), strings.ToLower(environment))
	if err != nil {
		funcLogger.Errorln("error observed while checking request access", zap.Error(err))
		return nil, err
	}

	return hasAccess, nil
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
func checkRequesterAccess(ctx context.Context, requester string, repository string, environment string) (*bool, error) {
	funcLogger := logInstance.With()

	_, subSegment := xray.BeginSubsegment(ctx, "checkRequesterAccess")
	if subSegment != nil {
		traceID := subSegment.TraceID
		funcLogger = logInstance.With(zap.String("traceID", traceID))
		defer subSegment.Close(nil)
	}

	accessCheck := make(chan *bool, 4) // channel to store access checks
	errChan := make(chan error, 4)     // chanel to store errors

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		// checks if requester has exact access to a specific environment in a repository
		exactAccessInput := &dynamodb.GetItemInput{
			TableName: &tableName,
			Key: map[string]types.AttributeValue{
				"login":    &types.AttributeValueMemberS{Value: requester},
				"repo-env": &types.AttributeValueMemberS{Value: strings.Join([]string{repository, environment}, "#")},
			},
		}
		requesterHasExactAccess, err := checkAccessByInput(ctx, exactAccessInput)
		if err != nil {
			errChan <- err
			funcLogger.Errorln("error observed while trying to check if requester has exact access", zap.Error(err))
			return
		}
		if requesterHasExactAccess != nil && *requesterHasExactAccess {
			funcLogger.Infoln("requester has exact access")
			accessCheck <- requesterHasExactAccess
		}

	}()

	go func() {
		defer wg.Done()

		// checks if requester has repo access (and thus access to all environments)
		repoAccessInput := &dynamodb.GetItemInput{
			TableName: &tableName,
			Key: map[string]types.AttributeValue{
				"login":    &types.AttributeValueMemberS{Value: requester},
				"repo-env": &types.AttributeValueMemberS{Value: strings.Join([]string{repository, "*"}, "#")},
			},
		}
		requesterHasRepoAccess, err := checkAccessByInput(ctx, repoAccessInput)
		if err != nil {
			errChan <- err
			funcLogger.Errorln("error observed while trying to check if requester has repo level access", zap.Error(err))
			return
		}
		if requesterHasRepoAccess != nil && *requesterHasRepoAccess {
			funcLogger.Infoln("requester has repo access")
			accessCheck <- requesterHasRepoAccess
		}
	}()

	go func() {
		defer wg.Done()

		// checks if requester has org access (access to all repos and all environments)
		orgAccessInput := &dynamodb.GetItemInput{
			TableName: &tableName,
			Key: map[string]types.AttributeValue{
				"login":    &types.AttributeValueMemberS{Value: requester},
				"repo-env": &types.AttributeValueMemberS{Value: strings.Join([]string{"*", "*"}, "#")},
			},
		}
		requesterHasOrgAccess, err := checkAccessByInput(ctx, orgAccessInput)
		if err != nil {
			errChan <- err
			funcLogger.Errorln("error observed while trying to check if requester has org access", zap.Error(err))
			return
		}
		if requesterHasOrgAccess != nil && *requesterHasOrgAccess {
			funcLogger.Infoln("requester has org access")
			accessCheck <- requesterHasOrgAccess
		}
	}()

	go func() {
		defer wg.Done()

		// checks if requester has environment access (across all repos)
		repoAccessInput := &dynamodb.GetItemInput{
			TableName: &tableName,
			Key: map[string]types.AttributeValue{
				"login":    &types.AttributeValueMemberS{Value: requester},
				"repo-env": &types.AttributeValueMemberS{Value: strings.Join([]string{"*", environment}, "#")},
			},
		}
		requesterHasRepoAccess, err := checkAccessByInput(ctx, repoAccessInput)
		if err != nil {
			errChan <- err
			funcLogger.Errorln("error observed while trying to check if requester has environment level access", zap.Error(err))
			return
		}
		if requesterHasRepoAccess != nil && *requesterHasRepoAccess {
			funcLogger.Infoln("requester has environment access")
			accessCheck <- requesterHasRepoAccess
		}
	}()

	wg.Wait()
	close(accessCheck)
	close(errChan)

	var firstErr error
	for {
		select { // read from error and access chanel
		case result, ok := <-accessCheck:
			if ok && result != nil && *result {
				funcLogger.Infoln("requester has access")
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
		// AND we've reviewed all the errors
		// then break
		if len(accessCheck) == 0 && (len(errChan) == 0) {
			break
		}
	}

	// requester did not have any access, return false
	requesterAccess := false
	funcLogger.Infoln("requester did not have access")
	return &requesterAccess, nil
}

func checkAccessByInput(ctx context.Context, input *dynamodb.GetItemInput) (*bool, error) {
	funcLogger := logInstance.With()

	_, subSegment := xray.BeginSubsegment(ctx, "checkAccessByInput")
	if subSegment != nil {
		traceID := subSegment.TraceID
		funcLogger = logInstance.With(zap.String("traceID", traceID))
		defer subSegment.Close(nil)
	}

	result, err := dynamodbClient.GetItem(ctx, input)
	if err != nil {
		errMsg := "error observed while trying to get dynamodb item"
		funcLogger.Errorln(errMsg, zap.Error(err), zap.Any("input", *input))
		return nil, err
	}
	hasAccess := (result.Item != nil)
	return &hasAccess, nil
}

/*
*
approves the pending deployment passed as user has access
*/
func approvePendingDeployment(ctx context.Context, pendingDeployment *github.PendingDeployment) error {
	funcLogger := logInstance.With()

	_, subSegment := xray.BeginSubsegment(ctx, "approvePendingDeployment")
	if subSegment != nil {
		traceID := subSegment.TraceID
		funcLogger = logInstance.With(zap.String("traceID", traceID))
		defer subSegment.Close(nil)
	}

	envID := int64(0)
	if pendingDeployment.GetEnvironment() != nil && pendingDeployment.GetEnvironment().GetID() != 0 {
		envID = pendingDeployment.GetEnvironment().GetID()
	} else {
		errMsg := "environment or environment ID from pending deployment payload is nil or empty"
		err := errors.New(errMsg)
		funcLogger.Errorln(errMsg, zap.Error(err))
		return err
	}

	funcLogger = funcLogger.With(zap.Int64("envID", envID))

	req := github.PendingDeploymentsRequest{EnvironmentIDs: []int64{envID}, State: "approved", Comment: "Approved via Go GitHub Webhook Lambda! 🚀"}

	approvedDeployments, approvalResp, approvalErr := ghClient.Actions.PendingDeployments(ctx, Current.owner, Current.repository, Current.ID, &req)
	if approvalResp.Response.StatusCode != http.StatusOK || approvalErr != nil {
		funcLogger.Error("error or incorrect status code observed while approving deployments", zap.Error(approvalErr), zap.Int("status_code", approvalResp.Response.StatusCode))
		return approvalErr
	}

	var approvedDeploymentsURLs []string
	for _, approvedDeployment := range approvedDeployments {
		if approvedDeployment.GetURL() != "" {
			approvedDeploymentsURLs = append(approvedDeploymentsURLs, approvedDeployment.GetURL())
		}
	}

	funcLogger.Infoln("approved deployments", zap.Strings("deployment_urls", approvedDeploymentsURLs))

	return nil
}

func getPendingDeployments(ctx context.Context, event *github.WorkflowRunEvent) ([]*github.PendingDeployment, error) {
	funcLogger := logInstance.With()

	_, subSegment := xray.BeginSubsegment(ctx, "getPendingDeployments")
	if subSegment != nil {
		traceID := subSegment.TraceID
		funcLogger = logInstance.With(zap.String("traceID", traceID))
		defer subSegment.Close(nil)
	}

	if event.GetRepo() != nil && event.GetRepo().GetOwner() != nil && event.GetRepo().GetOwner().GetLogin() != "" {
		Current.owner = event.GetRepo().GetOwner().GetLogin()
	} else {
		err := fmt.Errorf("repo or repo owner or repo owner login from pending deployment is nil or empty")
		funcLogger.Errorln(err.Error())
		return nil, err
	}

	if event.GetWorkflowRun() != nil && event.GetWorkflowRun().GetID() != 0 {
		Current.ID = event.GetWorkflowRun().GetID()
	} else {
		err := fmt.Errorf("workflow run or workflow run ID from pending deployment is nil or empty")
		funcLogger.Errorln(err.Error())
		return nil, err
	}

	funcLogger = funcLogger.With(zap.String("owner", Current.owner), zap.Int64("runID", Current.ID))

	maxRetries := 3
	retryDelay := 1

	for i := 0; i < maxRetries; i++ {

		pendingDeployments, resp, err := ghClient.Actions.GetPendingDeployments(ctx, Current.owner, Current.repository, Current.ID)
		if err != nil || resp.StatusCode != http.StatusOK {
			funcLogger.Errorln("error or incorrect status code while fetching pending deployments", zap.Error(err))
			return nil, err
		}

		if len(pendingDeployments) > 0 {
			return pendingDeployments, nil
		}

		funcLogger.Warnln("No pending deployments found, retrying...", zap.Int("attempt", i+1), zap.Int("nextDelay", retryDelay))

		time.Sleep(time.Second * time.Duration(retryDelay))
		retryDelay ^= 2 // exponential backoff
	}

	return nil, fmt.Errorf("no pending deployments found after %d retries", maxRetries)
}

func setupClients(ctx context.Context) error {
	ghClientErr := setGhClient(ctx)
	if ghClientErr != nil {
		return ghClientErr
	}

	dbClientErr := setDbClient(ctx)
	if dbClientErr != nil {
		return dbClientErr
	}

	return nil
}

func setGhClient(ctx context.Context) error {
	client, err := gh.GetGitHubClient(ctx)
	if err != nil {
		logInstance.Errorln("error while getting github client to handle workflow run review", zap.Error(err))
		return err
	}

	ghClient = client
	return nil
}

func setDbClient(ctx context.Context) error {
	client, err := db.GetDynamoClient(ctx)
	if err != nil {
		logInstance.Errorln("error observed while trying to get dynamodb client", zap.Error(err))
		return err
	}

	dynamodbClient = client
	return nil
}
