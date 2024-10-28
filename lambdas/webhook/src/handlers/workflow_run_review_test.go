package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/go-github/v66/github"
	"github.com/stretchr/testify/assert"

	"github.com/awsdocs/aws-doc-sdk-examples/gov2/testtools"
	ghMock "github.com/migueleliasweb/go-github-mock/src/mock"
)

var (
	stubber *testtools.AwsmStubber
)

const (
	repo_name      = "test-repo"
	env_name       = "test-env"
	requester_name = "github-requester"
	owner_name     = "github-owner"
	run_id         = int64(123456)
)

func init() {
	// set up dynamodb client from stubber
	stubber = testtools.NewStubber()
	dynamodbClient = dynamodb.NewFromConfig(*stubber.SdkConfig)
}

/*
Test for case where user has no access based on
*/
func TestNoAccess(t *testing.T) {
	// arrange
	event := createdWorkflowRunEvent(repo_name, owner_name, requester_name, run_id)

	ghClient = getMockedGhClient(run_id, env_name)
	stubGetItem(requester_name, repo_name, env_name, false)

	// act
	err := HandleWorkflowRunEvent(context.TODO(), true, event)

	// assert
	assert.Nil(t, err)
}

/*
Test for case where user has access based on
table entry of <repo>#<env>
*/
func TestExactAccess(t *testing.T) {
	// arrange
	event := createdWorkflowRunEvent(repo_name, owner_name, requester_name, run_id)

	ghClient = getMockedGhClient(run_id, env_name)
	stubGetItem(requester_name, repo_name, env_name, true)

	// act
	err := HandleWorkflowRunEvent(context.TODO(), true, event)

	// assert
	assert.Nil(t, err)
}

/*
Test for case where user has access based on
table entry of *#*
*/
func TestOrgAccess(t *testing.T) {
	// arrange
	repo_name := "*"
	env_name := "*"

	event := createdWorkflowRunEvent(repo_name, owner_name, requester_name, run_id)

	ghClient = getMockedGhClient(run_id, env_name)
	stubGetItem(requester_name, repo_name, env_name, true)

	// act
	err := HandleWorkflowRunEvent(context.TODO(), true, event)

	// assert
	assert.Nil(t, err)
}

/*
Test for case where user has access based on
table entry of <repo>#*
*/
func TestRepoAccess(t *testing.T) {
	// arrange
	env_name := "*"

	event := createdWorkflowRunEvent(repo_name, owner_name, requester_name, run_id)

	ghClient = getMockedGhClient(run_id, env_name)
	stubGetItem(requester_name, repo_name, env_name, true)

	// act
	err := HandleWorkflowRunEvent(context.TODO(), true, event)

	// assert
	assert.Nil(t, err)
}

/*
Test for case where user has access based on
table entry of *#<env>
*/
func TestEnvAccess(t *testing.T) {
	// arrange
	repo_name := "*"

	event := createdWorkflowRunEvent(repo_name, owner_name, requester_name, run_id)

	ghClient = getMockedGhClient(run_id, env_name)
	stubGetItem(requester_name, repo_name, env_name, true)

	// act
	err := HandleWorkflowRunEvent(context.TODO(), true, event)

	// assert
	assert.Nil(t, err)
}

func stubGetItem(requester string, repo string, env string, itemExists bool) {

	entry := map[string]types.AttributeValue{
		"login":    &types.AttributeValueMemberS{Value: requester},
		"repo-env": &types.AttributeValueMemberS{Value: strings.Join([]string{repo, env}, "#")},
	}

	tableName := TABLE_NAME_DEFAULT
	input := &dynamodb.GetItemInput{TableName: &tableName, Key: entry}

	var output *dynamodb.GetItemOutput = nil
	if itemExists {
		output = &dynamodb.GetItemOutput{Item: entry}
	}

	stubber.Add(testtools.Stub{
		OperationName: "GetItem",
		Input:         input,
		Output:        output,
		SkipErrorTest: true,
		Error:         nil,
	})
}

func getMockedGhClient(runID int64, envName string) *github.Client {

	deploymentURL := "example.com"

	pendingDeployments := []*github.PendingDeployment{{
		Environment: &github.PendingDeploymentEnvironment{ID: &runID, Name: &envName},
	}}
	approvedDeployments := []*github.Deployment{{
		URL: &deploymentURL,
	}}

	mockedHTTPClient := ghMock.NewMockedHTTPClient(
		// this func implements a FIFO for requests of the pattern
		// you must put a response for each time you will call.
		ghMock.WithRequestMatch(
			ghMock.GetReposActionsRunsPendingDeploymentsByOwnerByRepoByRunId,
			pendingDeployments),
		ghMock.WithRequestMatch(ghMock.PostReposActionsRunsPendingDeploymentsByOwnerByRepoByRunId,
			approvedDeployments,
		),
	)

	return github.NewClient(mockedHTTPClient)
}

func createdWorkflowRunEvent(repoName, ownerName, requesterName string, runID int64) *github.WorkflowRunEvent {
	action := REQUESTED_ACTION

	return &github.WorkflowRunEvent{
		Repo:   &github.Repository{Name: &repoName, Owner: &github.User{Login: &ownerName}},
		Sender: &github.User{Login: &requesterName},
		WorkflowRun: &github.WorkflowRun{
			ID: &runID,
		},
		Action: &action,
	}
}
