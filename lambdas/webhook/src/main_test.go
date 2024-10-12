package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-go/events"
	"github.com/google/go-github/v66/github"
)

const (
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
)

func init() {
	initReqs()
}

func initReqs() {
	eventMonitor = &GitHubEventMonitor{
		webhookSecretKey: []byte(GH_SAMPLE_SECRET_KEY),
	}

	invalidPayloadReq = generateAPIGatewayProxyRequest(nil, nil, false)
	parsedWebhookIncorrectBodyReq = generateAPIGatewayProxyRequest(nil, &[]string{"invalid body"}[0], true)
	parsedWebhookIncorrectHeaderType = generateAPIGatewayProxyRequest(&[]string{"incorrect"}[0], nil, true)
	unSupportedEventReq = generateAPIGatewayProxyRequest(&[]string{"sponsorship"}[0], nil, true)
	supportedEventReq = generateAPIGatewayProxyRequest(nil, nil, true)
}

func TestInValidPayload(t *testing.T) {
	resp, err := eventMonitor.HandleRequest(context.TODO(), invalidPayloadReq)

	assert.Error(t, err, "expected an error due to invalid payload but got nil")
	assert.Equal(t, STATUS_CODE_400, resp.StatusCode, fmt.Sprintf("expected %d status code but got %d", STATUS_CODE_400, resp.StatusCode))
	assert.Contains(t, strings.ToLower(resp.Body), strings.ToLower("invalid payload"))
	assert.ErrorContains(t, err, "invalid payload")
}

func TestInvalidWebhookBody(t *testing.T) {
	resp, err := eventMonitor.HandleRequest(context.TODO(), parsedWebhookIncorrectBodyReq)

	assert.Error(t, err, "expected an error due to invalid payload but got nil")
	assert.Equal(t, STATUS_CODE_400, resp.StatusCode, fmt.Sprintf("expected %d status code but got %d", STATUS_CODE_400, resp.StatusCode))
	assert.Contains(t, strings.ToLower(resp.Body), strings.ToLower("failed to parse webhook"))
	assert.ErrorContains(t, err, "invalid character")
}

func TestInvalidWebhookHeaderType(t *testing.T) {
	resp, err := eventMonitor.HandleRequest(context.TODO(), parsedWebhookIncorrectHeaderType)

	assert.Error(t, err, "expected an error due to invalid payload but got nil")
	assert.Equal(t, STATUS_CODE_400, resp.StatusCode, fmt.Sprintf("expected %d status code but got %d", STATUS_CODE_400, resp.StatusCode))
	assert.Contains(t, strings.ToLower(resp.Body), strings.ToLower("failed to parse webhook"))
	assert.ErrorContains(t, err, "unknown X-Github-Event in message")
}

func TestUnsupportedEventType(t *testing.T) {
	resp, err := eventMonitor.HandleRequest(context.TODO(), unSupportedEventReq)

	assert.Error(t, err, "expected an error due to invalid payload but got nil")
	assert.Equal(t, STATUS_CODE_400, resp.StatusCode, fmt.Sprintf("expected %d status code but got %d", STATUS_CODE_400, resp.StatusCode))
	assert.Contains(t, strings.ToLower(resp.Body), strings.ToLower("unsupported event type"))
	assert.ErrorContains(t, err, "unsupported event type")
}

func TestSupportedEventType(t *testing.T) {
	resp, err := eventMonitor.HandleRequest(context.TODO(), supportedEventReq)

	assert.NoError(t, err)
	assert.Equal(t, STATUS_CODE_200, resp.StatusCode, fmt.Sprintf("expected %d status code but got %d", STATUS_CODE_400, resp.StatusCode))
	assert.Contains(t, strings.ToLower(resp.Body), strings.ToLower("event processed"))
}

func generateAPIGatewayProxyRequest(eventTypeHeader *string, payload *string, validateSignature bool) events.APIGatewayProxyRequest {
	if eventTypeHeader == nil {
		temp := "deployment_review"
		eventTypeHeader = &temp
	}

	if payload == nil {
		temp := "{\"key\":\"value\"}"
		payload = &temp
	}

	return events.APIGatewayProxyRequest{
		Headers: map[string]string{
			CONTENT_TYPE_HEADER:          "application/json",
			github.EventTypeHeader:       *eventTypeHeader,
			github.SHA256SignatureHeader: generateSignatureHeader(*payload, validateSignature),
		},
		Body: *payload,
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

/*
*
generated a signature header, if valid is true, the string
returned is properly encoded with the secret key
*/
func generateSignatureHeader(signature string, valid bool) string {
	if valid {
		signature = generateSignature(signature)
	}
	return strings.Join([]string{sha256Prefix, signature}, "=")
}
