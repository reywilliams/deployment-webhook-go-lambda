package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
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
	log       zap.SugaredLogger
	tableName string
	Current   WorkflowRun
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
	log = *logger.GetLogger().Sugar()
	tableName = util.LookupEnv(TABLE_NAME_ENV_VAR_KEY, TABLE_NAME_DEFAULT, false)
}

func RequesterHasPermission(ctx context.Context, environment string) (*bool, error) {
	_, subSegment := xray.BeginSubsegment(ctx, "RequesterHasPermission")
	if subSegment != nil {
		traceID := subSegment.TraceID
		log = *log.With(zap.String("traceID", traceID))
		defer subSegment.Close(nil)
	}
	localLogger := log.With(zap.String("environment", environment))

	localLogger.Infoln("checking if requester has permission")

	dynamodbClient, err := db.GetDynamoClient(ctx)
	if err != nil {
		localLogger.Errorln("error observed while trying to get dynamodb client", zap.Error(err))
		return nil, err
	}

	hasAccess, err := checkRequesterAccess(ctx, dynamodbClient, Current.requester, Current.repository, environment)
	if err != nil {
		localLogger.Errorln("error observed while checking request access", zap.Error(err))
		return nil, err
	}

	return hasAccess, nil
}

func HandleWorkflowRunEvent(ctx context.Context, mocking bool, event *github.WorkflowRunEvent) error {

	_, subSegment := xray.BeginSubsegment(ctx, "HandleWorkflowRunEvent")
	if subSegment != nil {
		traceID := subSegment.TraceID
		log = *log.With(zap.String("traceID", traceID))
		defer subSegment.Close(nil)
	}

	// we only handle request review events
	if event.GetAction() != REQUESTED_ACTION {
		log.Debug("event was not for a request", zap.String("action", event.GetAction()))
		return nil
	}

	// get the requestor and their repo
	if event.GetSender() != nil && event.GetSender().GetLogin() != "" {
		Current.requester = event.GetSender().GetLogin()
	} else {
		err := fmt.Errorf("sender or sender login from event payload is nil or empty")
		log.Errorln("invalid field", zap.Error(err))
		return err
	}
	if event.GetRepo() != nil && event.GetRepo().GetName() != "" {
		Current.repository = event.Repo.GetName()
	} else {
		err := fmt.Errorf("repo or repo name from event payload is nil or empty")
		log.Errorln("invalid field", zap.Error(err))
		return err
	}

	log = *log.With(zap.String("requester", Current.requester), zap.String("repository", Current.repository))

	if mocking {
		message := fmt.Sprintf("requester %s has needs a review in %s repo!", Current.requester, Current.repository)
		log.Infof("constructed message: %s", message)
		return nil
	}

	pendingDeployments, err := getPendingDeployments(ctx, event)
	if err != nil {
		log.Errorln("error while fetching pending deployments to handle workflow run event")
		return err
	}

	log.Infof("Processing event: %T", event)

	// approve deployments for environments where requester has access
	for _, pendingDeployment := range pendingDeployments {

		environment := ""
		if pendingDeployment.GetEnvironment() != nil && pendingDeployment.GetEnvironment().GetName() != "" {
			environment = pendingDeployment.GetEnvironment().GetName()
		} else {
			// skip invalid or empty environment names
			continue
		}

		// check if requestor (sender) has permission for repo/env
		requesterHasPerm, err := RequesterHasPermission(ctx, environment)
		if err != nil {
			log.Errorln("error observed while checking if requester has permission", zap.Error(err))
			return err
		}

		// approve the pending deployment if user has permission
		if requesterHasPerm != nil && *requesterHasPerm {
			log.Info("requester has permission, will attempt to approve pending deployment")

			err := approvePendingDeployment(ctx, pendingDeployment)
			if err != nil {
				log.Error("error observed while trying to approve pending deployment", zap.Error(err))
				return err
			}
		}
	}

	// no error yielded, return nil
	return nil
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

	_, subSegment := xray.BeginSubsegment(ctx, "checkRequesterAccess")
	if subSegment != nil {
		traceID := subSegment.TraceID
		log = *log.With(zap.String("traceID", traceID))
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
		requesterHasExactAccess, err := checkAccessByInput(ctx, exactAccessInput, client)
		if err != nil {
			errChan <- err
			log.Errorln("error observed while trying to check if requester has exact access", zap.Error(err))
			return
		}
		if requesterHasExactAccess != nil && *requesterHasExactAccess {
			log.Infoln("requester has exact access")
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
		requesterHasRepoAccess, err := checkAccessByInput(ctx, repoAccessInput, client)
		if err != nil {
			errChan <- err
			log.Errorln("error observed while trying to check if requester has repo level access", zap.Error(err))
			return
		}
		if requesterHasRepoAccess != nil && *requesterHasRepoAccess {
			log.Infoln("requester has repo access")
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
		requesterHasOrgAccess, err := checkAccessByInput(ctx, orgAccessInput, client)
		if err != nil {
			errChan <- err
			log.Errorln("error observed while trying to check if requester has org access", zap.Error(err))
			return
		}
		if requesterHasOrgAccess != nil && *requesterHasOrgAccess {
			log.Infoln("requester has org access")
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
		requesterHasRepoAccess, err := checkAccessByInput(ctx, repoAccessInput, client)
		if err != nil {
			errChan <- err
			log.Errorln("error observed while trying to check if requester has environment level access", zap.Error(err))
			return
		}
		if requesterHasRepoAccess != nil && *requesterHasRepoAccess {
			log.Infoln("requester has environment access")
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
				log.Infoln("requester has access")
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
	log.Infoln("requester did not have access")
	return &requesterAccess, nil
}

func checkAccessByInput(ctx context.Context, input *dynamodb.GetItemInput, client *dynamodb.Client) (*bool, error) {

	_, subSegment := xray.BeginSubsegment(ctx, "checkAccessByInput")
	if subSegment != nil {
		traceID := subSegment.TraceID
		log = *log.With(zap.String("traceID", traceID))
		defer subSegment.Close(nil)
	}

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
approves the pending deployment passed as user has access
*/
func approvePendingDeployment(ctx context.Context, pendingDeployment *github.PendingDeployment) error {

	_, subSegment := xray.BeginSubsegment(ctx, "approvePendingDeployment")
	if subSegment != nil {
		traceID := subSegment.TraceID
		log = *log.With(zap.String("traceID", traceID))
		defer subSegment.Close(nil)
	}

	envID := int64(0)
	if pendingDeployment.GetEnvironment() != nil && pendingDeployment.GetEnvironment().GetID() != 0 {
		envID = pendingDeployment.GetEnvironment().GetID()
	} else {
		err := fmt.Errorf("environment or environment ID from pending deployment payload is nil or empty")
		log.Errorln("invalid field", zap.Error(err))
		return err
	}

	localLogger := log.With(zap.Int64("envID", envID))

	ghClient, err := gh.GetGitHubClient(ctx)
	if err != nil {
		localLogger.Errorln("error while getting github client to approve pending deployments", zap.Error(err))
		return err
	}

	req := github.PendingDeploymentsRequest{EnvironmentIDs: []int64{envID}, State: "approved", Comment: "Approved via Go GitHub Webhook Lambda! 🚀"}

	approvedDeployments, approvalResp, approvalErr := ghClient.Actions.PendingDeployments(ctx, Current.owner, Current.repository, Current.ID, &req)
	if approvalResp.Response.StatusCode != http.StatusOK || approvalErr != nil {
		localLogger.Error("error or incorrect status code observed while approving deployments", zap.Error(approvalErr), zap.Int("status_code", approvalResp.Response.StatusCode))
		return approvalErr
	}

	var approvedDeploymentsURLs []string
	for _, approvedDeployment := range approvedDeployments {
		if approvedDeployment.GetURL() != "" {
			approvedDeploymentsURLs = append(approvedDeploymentsURLs, approvedDeployment.GetURL())
		}
	}

	localLogger.Infoln("approved deployments", zap.Strings("deployment_urls", approvedDeploymentsURLs))

	return nil
}

func getPendingDeployments(ctx context.Context, event *github.WorkflowRunEvent) ([]*github.PendingDeployment, error) {
	_, subSegment := xray.BeginSubsegment(ctx, "getPendingDeployments")
	if subSegment != nil {
		traceID := subSegment.TraceID
		log = *log.With(zap.String("traceID", traceID))
		defer subSegment.Close(nil)
	}

	if event.GetRepo() != nil && event.GetRepo().GetOwner() != nil && event.GetRepo().GetOwner().GetLogin() != "" {
		Current.owner = event.GetRepo().GetOwner().GetLogin()
	} else {
		err := fmt.Errorf("repo or repo owner or repo owner login from pending deployment is nil or empty")
		log.Errorln("invalid field", zap.Error(err))
		return nil, err
	}

	if event.GetWorkflowRun() != nil && event.GetWorkflowRun().GetWorkflowID() != 0 {
		Current.ID = event.GetWorkflowRun().GetWorkflowID()
	} else {
		err := fmt.Errorf("workflow run or workflow run ID from pending deployment is nil or empty")
		log.Errorln("invalid field", zap.Error(err))
		return nil, err
	}

	localLogger := log.With(zap.String("owner", Current.owner), zap.Int64("runID", Current.ID))

	ghClient, err := gh.GetGitHubClient(ctx)
	if err != nil {
		localLogger.Errorln("error while github client to fetch pending deployments", zap.Error(err))
		return nil, err
	}

	pendingDeployments, resp, err := ghClient.Actions.GetPendingDeployments(ctx, Current.owner, Current.repository, Current.ID)
	if err != nil || resp.StatusCode != http.StatusOK {
		localLogger.Errorln("error or incorrect status code while fetching pending deployments", zap.Error(err))
		return nil, err
	}

	return pendingDeployments, nil
}
