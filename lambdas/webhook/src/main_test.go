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
	happyPathReq          events.APIGatewayProxyRequest
	invalidPayloadReq     events.APIGatewayProxyRequest
	failedWebhookParseReq events.APIGatewayProxyRequest

	eventMonitor *GitHubEventMonitor
)

func init() {
	initReqs()
}

func initReqs() {
	eventMonitor = &GitHubEventMonitor{
		webhookSecretKey: []byte(GH_SAMPLE_SECRET_KEY),
	}

	happyPathBody := "{\"key\":\"value\"}"
	happyPathReq = events.APIGatewayProxyRequest{
		Headers: map[string]string{
			CONTENT_TYPE_HEADER:          APP_JSON_HEADER,
			github.EventTypeHeader:       DEPLOY_REVIEW_GH_EVENT_HEADER_TYPE,
			github.SHA256SignatureHeader: generateSignatureHeader(happyPathBody, true),
		},
		Body: happyPathBody,
	}

	invalidPayloadReq = events.APIGatewayProxyRequest{
		Headers: map[string]string{
			CONTENT_TYPE_HEADER:          APP_JSON_HEADER,
			github.EventTypeHeader:       DEPLOY_REVIEW_GH_EVENT_HEADER_TYPE,
			github.SHA256SignatureHeader: generateSignatureHeader("invalid", false),
		},
		Body: "Hello World!",
	}

	failedWebhookParseReq = events.APIGatewayProxyRequest{
		Headers: map[string]string{
			CONTENT_TYPE_HEADER:          APP_JSON_HEADER,
			github.EventTypeHeader:       DEPLOY_REVIEW_GH_EVENT_HEADER_TYPE,
			github.SHA256SignatureHeader: generateSignatureHeader("Hello World!", true),
		},
		Body: "Hello World!",
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

func TestInvalidWebhookTypeHeader(t *testing.T) {
	resp, err := eventMonitor.HandleRequest(context.TODO(), failedWebhookParseReq)
	if err == nil {
		t.Errorf("expected an error due to invalid payload but got nil")
	} else if resp.StatusCode != STATUS_CODE_400 {
		t.Errorf("expected %d status code but got %d", STATUS_CODE_400, resp.StatusCode)
	} else if !strings.Contains(strings.ToLower(resp.Body), strings.ToLower("Failed to parse webhook")) {
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
