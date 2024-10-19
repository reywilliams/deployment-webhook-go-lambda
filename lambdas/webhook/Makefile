
BINARY_NAME=bootstrap
BUILD_DIR=build
SRC_DIR=src
CONFIG_DIR=config
SCRIPTS_DIR=scripts
GOFILES=$(shell find $(SRC_DIR) -name '*.go')
TESTFILES=$(shell find $(SRC_DIR) -name '*_test.go')
LAMBDA_DOCKER_IMAGE=webhook-lambda
XRAY_DOCKER_IMAGE=xray-daemon
XRAY_IP=172.17.0.2

all: build

build: $(GOFILES)
	chmod +x $(SCRIPTS_DIR)/build_lambda.sh
	./$(SCRIPTS_DIR)/build_lambda.sh

test: $(GOFILES) $(TESTFILES)
	cd $(SRC_DIR) && go test -v ./...

docker-build:
	docker build \
	--progress=plain \
	--platform=linux/amd64 \
	-t $(LAMBDA_DOCKER_IMAGE) \
	-f Dockerfile.lambda .

docker-run: docker-build
	docker run \
	-d \
	-p 9000:8080 \
	--entrypoint /usr/local/bin/aws-lambda-rie \
	-e SERVICE_NAME="webhook-lambda" \
	-e TRACE_ENDPOINT=$(XRAY_IP):2000 \
	-e environment=dev \
	$(LAMBDA_DOCKER_IMAGE) ./bootstrap

docker-build-xray:
	docker build -t $(XRAY_DOCKER_IMAGE) -f Dockerfile.xray .

docker-run-xray: docker-build-xray
	docker run \
	-d \
	-v ~/.aws/:/root/.aws/:ro \
	-p 2000:2000/udp \
	-e AWS_REGION=us-west-2 \
	-e AWS_SDK_LOAD_CONFIG=1 \
	-e AWS_PROFILE=deployment-webhooks-dev \
	$(XRAY_DOCKER_IMAGE) --local-mode --log-level dev

docker-logs-lambda:
	docker logs $$(docker ps -q --filter ancestor=$(LAMBDA_DOCKER_IMAGE))

docker-logs-xray:
	docker logs $$(docker ps -q --filter ancestor=$(XRAY_DOCKER_IMAGE))

docker-kill-lambda:
	docker kill $$(docker ps -q --filter ancestor=$(LAMBDA_DOCKER_IMAGE))

docker-kill-xray:
	docker kill $$(docker ps -q --filter ancestor=$(XRAY_DOCKER_IMAGE))

docker-test:
	curl -X POST http://localhost:9000/2015-03-31/functions/function/invocations -d @$(CONFIG_DIR)/api_gw_sample_payload.json

.PHONY: build test docker-build docker-run docker-kill docker-test docker-run-xray docker-kill-xray docker-logs-xray docker-logs-lambda
