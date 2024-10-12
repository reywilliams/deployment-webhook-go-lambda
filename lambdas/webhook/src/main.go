package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"webhook/logger"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/go-github/v66/github"
)

var (
	log                   logger.Logger
	GITHUB_WEBHOOK_SECRET string
)

const (
	GITHUB_WEBHOOK_SECRET_DEFAULT     = "FALLBACK_SECRET_NOT_IMPL"
	GITHUB_WEBHOOK_SECRET_ENV_VAR_KEY = "GITHUB_WEBHOOK_SECRET"

	CONTENT_TYPE_HEADER = "Content-Type"
)

func init() {
	source_github_webhook_secret() // sources GITHUB_WEBHOOK_SECRET env. variable
}

type GitHubEventMonitor struct {
	webhookSecretKey []byte
}

func (s *GitHubEventMonitor) HandleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.INFO("HandleRequest() called")
	logAPIGatewayRequest(request)

	payload, err := github.ValidatePayloadFromBody(request.Headers[CONTENT_TYPE_HEADER], strings.NewReader(request.Body), request.Headers[github.SHA256SignatureHeader], s.webhookSecretKey)
	if err != nil {
		log.ERROR("invalid payload: %s", err)
		err := fmt.Errorf("invalid payload: %w", err)
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "invalid payload"}, err
	}

	event, err := github.ParseWebHook(request.Headers[github.EventTypeHeader], payload)
	if err != nil {
		log.ERROR("failed to parse webhook: %s", err)
		err := fmt.Errorf("failed to parse webhook: %w", err)
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "failed to parse webhook"}, err
	}

	switch event := event.(type) {
	case *github.DeploymentReviewEvent:
		handleDeploymentReviewEvent(event)
	default:
		log.ERROR("unsupported event type: %T", event)
		err := fmt.Errorf("unsupported event type: %T", event)
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "unsupported event type"}, err
	}

	return events.APIGatewayProxyResponse{StatusCode: 200, Body: "event processed"}, nil
}

func handleDeploymentReviewEvent(event *github.DeploymentReviewEvent) {
	log.INFO("Processing event: %T", event)

	var message string

	if event.Requester != nil && event.Requester.Name != nil &&
		event.Environment != nil && event.Repo != nil && event.Repo.Name != nil {
		message = fmt.Sprintf("User %s has requested a review for %s environment in %s repo!", *event.Requester.Name, *event.Environment, *event.Repo.Name)
	} else {
		message = "fall back message as there were null pointers."
	}
	log.INFO("Constructed message %s", message)
}

func main() {
	log.INFO("main() func called")

	eventMonitor := &GitHubEventMonitor{
		webhookSecretKey: []byte(GITHUB_WEBHOOK_SECRET),
	}
	lambda.Start(eventMonitor.HandleRequest)
}

/*
Retrieves GITHUB_WEBHOOK_SECRET environment variable, sets global var (GITHUB_WEBHOOK_SECRET) to:
-> GITHUB_WEBHOOK_SECRET env var value if it is not empty
-> GITHUB_WEBHOOK_SECRET_DEFAULT if empty
*/
func source_github_webhook_secret() {
	log.INFO("sourcing Github Webhook Secret, env. var key: %s", GITHUB_WEBHOOK_SECRET_ENV_VAR_KEY)
	sourcedWebHookSecret := os.Getenv(GITHUB_WEBHOOK_SECRET_ENV_VAR_KEY)

	if sourcedWebHookSecret == "" {
		log.WARN("Github Webhook Secret, falling back to default: %s", GITHUB_WEBHOOK_SECRET_DEFAULT)
		GITHUB_WEBHOOK_SECRET = GITHUB_WEBHOOK_SECRET_DEFAULT
	} else {
		GITHUB_WEBHOOK_SECRET = sourcedWebHookSecret
	}
}

func logAPIGatewayRequest(req events.APIGatewayProxyRequest) {
	log.INFO("========================================")
	log.INFO("API_LOG:           API Gateway Request")
	log.INFO("========================================")
	log.INFO("API_LOG: HTTP Method: %s", req.HTTPMethod)
	log.INFO("API_LOG: Path: %s", req.Path)
	log.INFO("API_LOG: Headers: %v", req.Headers)
	log.INFO("API_LOG: Query String Parameters: %v", req.QueryStringParameters)
	log.INFO("API_LOG: Request Body: %s", req.Body)
	log.INFO("API_LOG: Is Base64 Encoded: %v", req.IsBase64Encoded)
	log.INFO("========================================")
}
