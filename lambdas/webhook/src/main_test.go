package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-go/events"
	"github.com/google/go-github/v66/github"
)

const (
	sha256Prefix string = "sha256"
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
	eventMonitor = &GitHubEventMonitor{
		webhookSecretKey: []byte(GITHUB_WEBHOOK_SECRET_DEFAULT),
	}
}

func TestInValidPayload(t *testing.T) {
	t.Parallel()

	// arrange
	invalidPayloadReq = generateAPIGatewayProxyRequest(nil, nil, false)

	// act
	resp, _ := eventMonitor.HandleRequest(context.TODO(), invalidPayloadReq)

	// assert
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "incorrect status code")
	assert.Contains(t, strings.ToLower(resp.Body), strings.ToLower("invalid payload"))
}

func TestInvalidWebhookBody(t *testing.T) {
	t.Parallel()

	// arrange
	parsedWebhookIncorrectBodyReq = generateAPIGatewayProxyRequest(nil, &[]string{"invalid body"}[0], true)

	// act
	resp, _ := eventMonitor.HandleRequest(context.TODO(), parsedWebhookIncorrectBodyReq)

	// assert
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "incorrect status code")
	assert.Contains(t, strings.ToLower(resp.Body), strings.ToLower("failed to parse webhook"))
}

func TestInvalidWebhookHeaderType(t *testing.T) {
	t.Parallel()

	// arrange
	parsedWebhookIncorrectHeaderType = generateAPIGatewayProxyRequest(&[]string{"incorrect"}[0], nil, true)

	// act
	resp, _ := eventMonitor.HandleRequest(context.TODO(), parsedWebhookIncorrectHeaderType)

	// assert
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "incorrect status code")
	assert.Contains(t, strings.ToLower(resp.Body), strings.ToLower("failed to parse webhook"))
}

func TestUnsupportedEventType(t *testing.T) {
	t.Parallel()

	// arrange
	unSupportedEventReq = generateAPIGatewayProxyRequest(&[]string{"sponsorship"}[0], nil, true)

	// act
	resp, _ := eventMonitor.HandleRequest(context.TODO(), unSupportedEventReq)

	// assert
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "incorrect status code")
	assert.Contains(t, strings.ToLower(resp.Body), strings.ToLower("unsupported event type"))
}

func TestSupportedEventType(t *testing.T) {
	t.Parallel()

	// arrange
	supportedEventReq = generateAPIGatewayProxyRequest(nil, nil, true)

	// act
	resp, _ := eventMonitor.HandleRequest(context.TODO(), supportedEventReq)

	// assert
	assert.Equal(t, http.StatusOK, resp.StatusCode, "incorrect status code")
	assert.Contains(t, strings.ToLower(resp.Body), strings.ToLower("event processed"))
}

func generateAPIGatewayProxyRequest(eventTypeHeader *string, payload *string, validateSignature bool) events.APIGatewayProxyRequest {
	if eventTypeHeader == nil {
		temp := "workflow_run"
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
			INTERNAL_MOCKING_HEADER:      "true",
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
	hmacHash := hmac.New(sha256.New, eventMonitor.webhookSecretKey)
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
