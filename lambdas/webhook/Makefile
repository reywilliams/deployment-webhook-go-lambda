# go binary
BINARY_NAME=bootstrap

# docker image
LAMBDA_DOCKER_IMAGE=webhook-lambda

# dir names
BUILD_DIR=build
SRC_DIR=src
CONFIG_DIR=config
SCRIPTS_DIR=scripts

# go and test files
GOFILES=$(shell find $(SRC_DIR) -name '*.go')
TESTFILES=$(shell find $(SRC_DIR) -name '*_test.go')

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
	-e environment=dev \
	$(LAMBDA_DOCKER_IMAGE) ./bootstrap

docker-logs:
	docker logs $$(docker ps -q --filter ancestor=$(LAMBDA_DOCKER_IMAGE))

docker-logs-f:
	docker logs -f $$(docker ps -q --filter ancestor=$(LAMBDA_DOCKER_IMAGE))

docker-kill:
	docker kill $$(docker ps -q --filter ancestor=$(LAMBDA_DOCKER_IMAGE))

docker-test:
	curl -X POST http://localhost:9000/2015-03-31/functions/function/invocations -d @$(CONFIG_DIR)/api_gw_sample_payload.json

.PHONY: build test docker-build docker-run docker-kill docker-test docker-logs docker-logs-f
