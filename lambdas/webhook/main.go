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

func HandleRequest(ctx context.Context, inputEvent interface{}) (*string, error) {
	log.INFO("lambda handler execution has begun")
	log.INFO("Consumed event: %+v\n", inputEvent)

	// TODO: use parse webhook to actually unmarshall event payload
	// and recognize GitHub Types
	switch event := inputEvent.(type) {
	case *github.DeploymentReviewEvent:
		message, err := handleDeploymentReviewEvent(event)
		return message, err
	default:
		log.INFO("switched to default case as an unknown type encountered.")
		message, err := handleDefaultCase(event)
		return message, err

	}
}

func handleDeploymentReviewEvent(event *github.DeploymentReviewEvent) (*string, error) {
	log.INFO("Event if of type: %T", event)
	message := fmt.Sprintf("User %s has requested a review for %s environment in %s repo!", *event.Requester.Name, *event.Environment, *event.Repo.Name)
	log.INFO("Constructed message %s", message)
	return &message, nil
}

func handleDefaultCase(event interface{}) (*string, error) {
	log.INFO("default case event is of type: %T", event)

	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.ERROR("failed to marshal event: %s", err.Error())
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	eventJSONString := string(eventJSON)
	log.INFO("Received event: %s", string(eventJSONString))
	return &eventJSONString, nil
}

func main() {
	log.INFO("main function called")
	lambda.Start(HandleRequest)
}
