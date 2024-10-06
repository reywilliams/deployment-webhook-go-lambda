package main

import (
	"context"
	"encoding/json"
	"fmt"
	"webhook/logger"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/go-github/v65/github"
)

var log logger.Logger

// This type is introduced in a PR but has not yet been included in the latest version
// https://github.com/google/go-github/pull/3254
// TODO: bump go-github version and use included type (when v66 is out)
type DeploymentReviewEvent struct {
	// The action performed. Possible values are: "requested", "approved", or "rejected".
	Action *string `json:"action,omitempty"`

	// The following will be populated only if requested.
	Requester   *github.User `json:"requester,omitempty"`
	Environment *string      `json:"environment,omitempty"`

	// The following will be populated only if approved or rejected.
	Approver        *github.User      `json:"approver,omitempty"`
	Comment         *string           `json:"comment,omitempty"`
	WorkflowJobRuns []*WorkflowJobRun `json:"workflow_job_runs,omitempty"`

	Enterprise     *github.Enterprise         `json:"enterprise,omitempty"`
	Installation   *github.Installation       `json:"installation,omitempty"`
	Organization   *github.Organization       `json:"organization,omitempty"`
	Repo           *github.Repository         `json:"repository,omitempty"`
	Reviewers      []*github.RequiredReviewer `json:"reviewers,omitempty"`
	Sender         *github.User               `json:"sender,omitempty"`
	Since          *string                    `json:"since,omitempty"`
	WorkflowJobRun *WorkflowJobRun            `json:"workflow_job_run,omitempty"`
	WorkflowRun    *github.WorkflowRun        `json:"workflow_run,omitempty"`
}

type WorkflowJobRun struct {
	Conclusion  *string           `json:"conclusion,omitempty"`
	CreatedAt   *github.Timestamp `json:"created_at,omitempty"`
	Environment *string           `json:"environment,omitempty"`
	HTMLURL     *string           `json:"html_url,omitempty"`
	ID          *int64            `json:"id,omitempty"`
	Name        *string           `json:"name,omitempty"`
	Status      *string           `json:"status,omitempty"`
	UpdatedAt   *github.Timestamp `json:"updated_at,omitempty"`
}

type Event interface{}

func HandleRequest(ctx context.Context, event Event) (*string, error) {

	// attempt to assert that this is a DeploymentReviewEvent
	deploymentEvent, ok := event.(*DeploymentReviewEvent)
	if !ok {
		return nil, fmt.Errorf("unexpected event type: %T", event)
	}
	// ensure the DeploymentReviewEvent is not null/nil
	if deploymentEvent == nil {
		return nil, fmt.Errorf("received nil event")
	}

	// try to get json from event and log event.
	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.ERROR("failed to marshal event: %s", err.Error())
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}
	log.INFO("Received event: %s", string(eventJSON))

	message := fmt.Sprintf("User %s has requested a review for %s environment in %s repo!", *deploymentEvent.Requester.Name, *deploymentEvent.Environment, *deploymentEvent.Repo.Name)
	log.INFO("Constructed message %s", message)
	return &message, nil
}

func main() {
	lambda.Start(HandleRequest)
}
