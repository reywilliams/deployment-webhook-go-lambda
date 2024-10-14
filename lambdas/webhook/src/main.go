package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"webhook/handlers"
	"webhook/logger"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/go-github/v66/github"
	"go.uber.org/zap"
)

var (
	log zap.SugaredLogger

	GITHUB_WEBHOOK_SECRET string
	mocking               bool
)

const (
	GITHUB_WEBHOOK_SECRET_DEFAULT     = "FALLBACK_SECRET_NOT_IMPL"
	GITHUB_WEBHOOK_SECRET_ENV_VAR_KEY = "GITHUB_WEBHOOK_SECRET"

	CONTENT_TYPE_HEADER     = "Content-Type"
	INTERNAL_MOCKING_HEADER = "X-Mock-Enabled"
)

func init() {
	log = *logger.GetLogger().Sugar()
	source_github_webhook_secret() // sources GITHUB_WEBHOOK_SECRET env. variable
}

type GitHubEventMonitor struct {
	webhookSecretKey []byte
}

func (s *GitHubEventMonitor) HandleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	defer log.Sync()
	logAPIGatewayRequest(request)
	log = addAPIGatewayRequestToLogContext(request)
	mocking = ShouldUseMock(&request.Headers)

	payload, err := github.ValidatePayloadFromBody(request.Headers[CONTENT_TYPE_HEADER], strings.NewReader(request.Body), request.Headers[github.SHA256SignatureHeader], s.webhookSecretKey)
	if err != nil {
		errMsg := fmt.Sprintf("invalid payload; %s", err)
		log.Errorln("invalid payload", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: buildResponseBody(errMsg, http.StatusBadRequest)}, nil
	}

	event, err := github.ParseWebHook(request.Headers[github.EventTypeHeader], payload)
	if err != nil {
		errMsg := fmt.Sprintf("failed to parse webhook; %s", err)
		log.Errorln("failed to parse webhook", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: buildResponseBody(errMsg, http.StatusBadRequest)}, nil
	}

	switch event := event.(type) {
	case *github.DeploymentReviewEvent:
		handlers.HandleDeploymentReviewEvent(ctx, mocking, event)
	default:
		errMsg := fmt.Sprintf("unsupported event type %T", event)
		log.Errorln("unsupported event type", fmt.Errorf("unsupported event type %T", event))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: buildResponseBody(errMsg, http.StatusBadRequest)}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: buildResponseBody("event processed", http.StatusOK)}, nil
}

func main() {
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
	log.Debugln("sourcing github webhook secret")
	sourcedWebHookSecret := os.Getenv(GITHUB_WEBHOOK_SECRET_ENV_VAR_KEY)

	if sourcedWebHookSecret == "" {
		log.Debugln("sourced github webhook secret empty, falling back to default")
		GITHUB_WEBHOOK_SECRET = GITHUB_WEBHOOK_SECRET_DEFAULT
	} else {
		GITHUB_WEBHOOK_SECRET = sourcedWebHookSecret
	}
}

func logAPIGatewayRequest(req events.APIGatewayProxyRequest) {
	log.Infoln("received API gateway proxy request",
		zap.Any("request", map[string]interface{}{
			"HTTPMethod":            req.HTTPMethod,
			"Path":                  req.Path,
			"Headers":               req.Headers,
			"QueryStringParameters": req.QueryStringParameters,
			"RequestBody":           req.Body,
			"IsBase64Encoded":       req.IsBase64Encoded,
		}),
	)
}

func addAPIGatewayRequestToLogContext(req events.APIGatewayProxyRequest) zap.SugaredLogger {
	return *log.With("request", map[string]interface{}{
		"HTTPMethod":            req.HTTPMethod,
		"Path":                  req.Path,
		"Headers":               req.Headers,
		"QueryStringParameters": req.QueryStringParameters,
		"RequestBody":           req.Body,
		"IsBase64Encoded":       req.IsBase64Encoded,
	})
}

func ShouldUseMock(headers *map[string]string) bool {
	log.Debugln("checking if mocking header is present")
	if headers != nil {
		val, exists := (*headers)[INTERNAL_MOCKING_HEADER]

		if exists {
			log.Debugln("mocking header present")

			if strings.ToLower(val) == "true" {
				log.Debugln("mocking is enabled through mocking header")
			}
		}
		return exists && strings.ToLower(val) == "true"
	} else {
		log.Debugln("mocking header not present")
		return false
	}
}

/*
*
Builds a response body using the message and status code's string representation
ex. Bad Request: unsupported event type
*/
func buildResponseBody(msg string, statusCode int) string {
	return strings.Join([]string{http.StatusText(statusCode), msg}, ": ")
}
