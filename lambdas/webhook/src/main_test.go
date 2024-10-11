package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/google/go-github/v66/github"
)

const (
	APP_JSON_HEADER                    string = "application/json"
	DEPLOY_REVIEW_GH_EVENT_HEADER_TYPE string = "deployment_review"

	GH_SAMPLE_SECRET_KEY string = "secret_key_shh"

	sha256Prefix string = "sha256"

	STATUS_CODE_200 int = 200
	STATUS_CODE_400 int = 400
)

var (
	supportedEventReq                events.APIGatewayProxyRequest
	invalidPayloadReq                events.APIGatewayProxyRequest
	parsedWebhookIncorrectBodyReq    events.APIGatewayProxyRequest
	unSupportedEventReq              events.APIGatewayProxyRequest
	parsedWebhookIncorrectHeaderType events.APIGatewayProxyRequest

	eventMonitor *GitHubEventMonitor

	validPayloadBody string = "{\"key\":\"value\"}"
)

func init() {
	initReqs()
}

func initReqs() {
	eventMonitor = &GitHubEventMonitor{
		webhookSecretKey: []byte(GH_SAMPLE_SECRET_KEY),
	}

	invalidPayloadReq = events.APIGatewayProxyRequest{
		Headers: map[string]string{
			CONTENT_TYPE_HEADER:          APP_JSON_HEADER,
			github.EventTypeHeader:       DEPLOY_REVIEW_GH_EVENT_HEADER_TYPE,
			github.SHA256SignatureHeader: generateSignatureHeader(validPayloadBody, false),
		},
		Body: validPayloadBody,
	}

	parsedWebhookIncorrectBodyReq = events.APIGatewayProxyRequest{
		Headers: map[string]string{
			CONTENT_TYPE_HEADER:          APP_JSON_HEADER,
			github.EventTypeHeader:       DEPLOY_REVIEW_GH_EVENT_HEADER_TYPE,
			github.SHA256SignatureHeader: generateSignatureHeader("invalid body", true),
		},
		Body: "invalid body",
	}

	parsedWebhookIncorrectHeaderType = events.APIGatewayProxyRequest{
		Headers: map[string]string{
			CONTENT_TYPE_HEADER:          APP_JSON_HEADER,
			github.EventTypeHeader:       "incorrect-header-type",
			github.SHA256SignatureHeader: generateSignatureHeader(validPayloadBody, true),
		},
		Body: validPayloadBody,
	}

	unSupportedEventReq = events.APIGatewayProxyRequest{
		Headers: map[string]string{
			CONTENT_TYPE_HEADER:          APP_JSON_HEADER,
			github.EventTypeHeader:       "sponsorship",
			github.SHA256SignatureHeader: generateSignatureHeader(validPayloadBody, true),
		},
		Body: validPayloadBody,
	}

	supportedEventReq = events.APIGatewayProxyRequest{
		Headers: map[string]string{
			CONTENT_TYPE_HEADER:          APP_JSON_HEADER,
			github.EventTypeHeader:       DEPLOY_REVIEW_GH_EVENT_HEADER_TYPE,
			github.SHA256SignatureHeader: generateSignatureHeader(validPayloadBody, true),
		},
		Body: validPayloadBody,
	}
}

func TestInValidPayload(t *testing.T) {
	resp, err := eventMonitor.HandleRequest(context.TODO(), invalidPayloadReq)
	if err == nil {
		t.Errorf("expected an error due to invalid payload but got nil")
	} else if resp.StatusCode != STATUS_CODE_400 {
		t.Errorf("expected %d status code but got %d", STATUS_CODE_400, resp.StatusCode)
	} else if !strings.Contains(strings.ToLower(resp.Body), strings.ToLower("Invalid payload")) {
		t.Errorf("the expected response was not returned")
	}
}

func TestInvalidWebhookBody(t *testing.T) {
	resp, err := eventMonitor.HandleRequest(context.TODO(), parsedWebhookIncorrectBodyReq)
	if err == nil {
		t.Errorf("expected an error due to invalid payload but got nil")
	} else if resp.StatusCode != STATUS_CODE_400 {
		t.Errorf("expected %d status code but got %d", STATUS_CODE_400, resp.StatusCode)
	} else if !strings.Contains(strings.ToLower(resp.Body), strings.ToLower("Failed to parse webhook")) {
		t.Errorf("the expected response was not returned")
	} else if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower("invalid character")) {
		t.Errorf("the expected response was not returned")
	}
}

func TestInvalidWebhookHeaderType(t *testing.T) {
	resp, err := eventMonitor.HandleRequest(context.TODO(), parsedWebhookIncorrectHeaderType)
	if err == nil {
		t.Errorf("expected an error due to invalid payload but got nil")
	} else if resp.StatusCode != STATUS_CODE_400 {
		t.Errorf("expected %d status code but got %d", STATUS_CODE_400, resp.StatusCode)
	} else if !strings.Contains(strings.ToLower(resp.Body), strings.ToLower("Failed to parse webhook")) {
		t.Errorf("the expected response was not returned")
	} else if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower("unknown X-Github-Event in message")) {
		t.Errorf("the expected response was not returned")
	}
}

func TestUnsupportedEventType(t *testing.T) {
	resp, err := eventMonitor.HandleRequest(context.TODO(), unSupportedEventReq)
	if err == nil {
		t.Errorf("expected an error due to invalid payload but got nil")
	} else if resp.StatusCode != STATUS_CODE_400 {
		t.Errorf("expected %d status code but got %d", STATUS_CODE_400, resp.StatusCode)
	} else if !strings.Contains(strings.ToLower(resp.Body), strings.ToLower("unsupported event type")) {
		t.Errorf("the expected response was not returned")
	}
}

func TestSupportedEventType(t *testing.T) {
	resp, err := eventMonitor.HandleRequest(context.TODO(), supportedEventReq)
	if err != nil {
		t.Errorf("did not expect an error but got one: %s", err.Error())
	} else if resp.StatusCode != STATUS_CODE_200 {
		t.Errorf("expected %d status code but got %d", STATUS_CODE_400, resp.StatusCode)
	} else if !strings.Contains(strings.ToLower(resp.Body), strings.ToLower("Event processed")) {
		t.Errorf("the expected response was not returned")
	}
}

/*
*
Generates a sha256 signature for the payload body
See more about payload validation here:
https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries#testing-the-webhook-payload-validation
*/
func generateSignature(body string) string {
	// create a new HMAC sha256 hash using the sample key
	hmacHash := hmac.New(sha256.New, []byte(GH_SAMPLE_SECRET_KEY))
	// write the payload body to hmac
	hmacHash.Write([]byte(body))
	// return the hex encoded hmac body to get signature
	return hex.EncodeToString(hmacHash.Sum(nil))
}

func generateSignatureHeader(signature string, valid bool) string {
	if valid {
		signature = generateSignature(signature)
	}

	return strings.Join([]string{sha256Prefix, signature}, "=")
}
