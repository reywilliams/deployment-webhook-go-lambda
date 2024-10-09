package main

import (
	"context"
	"encoding/json"
	"fmt"
	"webhook/logger"

	"github.com/aws/aws-lambda-go/lambda"
	github "github.com/google/go-github/v66/github"
)

var log logger.Logger

type event interface{}

func HandleRequest(ctx context.Context, event event) (*string, error) {
	log.INFO("lambda handler execution has begun")

	log.INFO("Consumed event: %+v\n", event)

	switch githubEvent := event.(type) {
	case *github.DeploymentReviewEvent:
		log.INFO("Event if of type: %T", event)
		message := fmt.Sprintf("User %s has requested a review for %s environment in %s repo!", *githubEvent.Requester.Name, *githubEvent.Environment, *githubEvent.Repo.Name)
		log.INFO("Constructed message %s", message)
		return &message, nil
	default:
		log.INFO("Switched to default case - event if of type: %T", event)
		// try to get json from event and log event.
		eventJSON, err := json.Marshal(event)
		if err != nil {
			log.ERROR("failed to marshal event: %s", err.Error())
			return nil, fmt.Errorf("failed to marshal event: %w", err)
		}

		eventJSONString := string(eventJSON)
		log.INFO("Received event: %s", string(eventJSONString))
		return &eventJSONString, nil
	}
}

func main() {
	log.INFO("main function called")
	lambda.Start(HandleRequest)
}
