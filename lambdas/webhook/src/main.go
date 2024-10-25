package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"webhook/handlers"
	"webhook/logger"
	"webhook/secrets"
	"webhook/util"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/awslabs/aws-lambda-go-api-proxy/core"
	"github.com/google/go-github/v66/github"
	"go.uber.org/zap"
)

var (
	log zap.SugaredLogger

	Mocking bool
)

const (
	GITHUB_WEBHOOK_SECRET_DEFAULT          = "FALLBACK_SECRET_NOT_IMPL"
	GITHUB_WEBHOOK_SECRET_NAME_DEFAULT     = "GITHUB_WEBHOOK_SECRET"
	GITHUB_WEBHOOK_SECRET_NAME_ENV_VAR_KEY = "GITHUB_WEBHOOK_SECRET_NAME"

	CONTENT_TYPE_HEADER     = "Content-Type"
	INTERNAL_MOCKING_HEADER = "X-Mock-Enabled"
)

func init() {
	log = *logger.GetLogger().Sugar()
	logger.InitializeXRay()
}

type GitHubEventMonitor struct {
	webhookSecretKey []byte
}

func (s *GitHubEventMonitor) HandleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	Mocking = ShouldUseMock(&request.Headers)
	webhookSecretErr := s.sourceSecret(ctx)
	if webhookSecretErr != nil {
		errMsg := "a webhook secret has not been configured"
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: buildResponseBody(errMsg, http.StatusInternalServerError)}, nil
	}

	_, subSegment := xray.BeginSubsegment(ctx, "HandleRequest")
	if subSegment != nil {
		traceID := subSegment.TraceID
		log = *log.With(zap.String("traceID", traceID))
		defer subSegment.Close(nil)
	}

	logAPIGatewayRequest(request)
	log = addAPIGatewayRequestToLogContext(request)

	reqAccessor := core.RequestAccessor{}
	httpReq, err := reqAccessor.ProxyEventToHTTPRequest(request)
	if err != nil {
		errMsg := "error while transforming proxy event to http req"
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: buildResponseBody(errMsg, http.StatusInternalServerError)}, nil
	}

	fmt.Printf("webhook secret: %s", s.webhookSecretKey)
	payload, err := github.ValidatePayload(httpReq, s.webhookSecretKey)
	if err != nil {
		errMsg := fmt.Sprintf("invalid payload; %s", err)
		log.Errorln("invalid payload", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: buildResponseBody(errMsg, http.StatusUnauthorized)}, nil
	}
	event, err := github.ParseWebHook(github.WebHookType(httpReq), payload)
	if err != nil {
		errMsg := fmt.Sprintf("failed to parse webhook; %s", err)
		log.Errorln("failed to parse webhook", zap.Error(err))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: buildResponseBody(errMsg, http.StatusBadRequest)}, nil
	}

	switch event := event.(type) {
	case *github.WorkflowRunEvent:
		err := handlers.HandleWorkflowRunEvent(ctx, Mocking, event)
		if err != nil {
			log.Errorln("error while handling event", zap.Error(err), zap.String("event_type", "deployment_status"))
		}
	default:
		errMsg := fmt.Sprintf("unsupported event type %T", event)
		log.Errorln("unsupported event type", fmt.Errorf("unsupported event type %T", event))
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: buildResponseBody(errMsg, http.StatusBadRequest)}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: buildResponseBody("event processed", http.StatusOK)}, nil
}

func main() {
	defer log.Sync()

	eventMonitor := &GitHubEventMonitor{}

	lambda.Start(eventMonitor.HandleRequest)
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
			return exists && strings.ToLower(val) == "true"

		} else {
			log.Debugln("mocking header not present")
			return false
		}
	}

	return false
}

/*
*
Builds a response body using the message and status code's string representation
ex. Bad Request: unsupported event type
*/
func buildResponseBody(msg string, statusCode int) string {
	return strings.Join([]string{http.StatusText(statusCode), msg}, ": ")
}

/*
If we are mocking, we return GITHUB_WEBHOOK_SECRET_DEFAULT,
otherwise we attempt to get the secret name from environment variables.
We using the secret name sourced from GITHUB_WEBHOOK_SECRET_NAME_ENV_VAR_KEY
or GITHUB_WEBHOOK_SECRET_NAME_DEFAULT if that environment variable is not found
*/
func getWebhookSecret(ctx context.Context) (*string, error) {

	var secretDefault = string(GITHUB_WEBHOOK_SECRET_DEFAULT)

	if Mocking {
		return &secretDefault, nil
	}

	webhookSecretName := util.LookupEnv(GITHUB_WEBHOOK_SECRET_NAME_ENV_VAR_KEY, GITHUB_WEBHOOK_SECRET_NAME_DEFAULT, false)
	webhookSecret, err := secrets.GetSecretValue(ctx, webhookSecretName)
	if webhookSecret == nil || err != nil {
		log.Errorln("error while getting webhook secret value")
		return nil, err
	}

	return webhookSecret, nil
}

/*
Sources Github Webhook secret and sets it if sourced.
Returns error and leaves property unset if not sourced successfully.
*/
func (s *GitHubEventMonitor) sourceSecret(ctx context.Context) error {

	secret, err := getWebhookSecret(ctx)
	if err != nil {
		return err
	}
	s.webhookSecretKey = []byte(*secret)
	return nil
}
