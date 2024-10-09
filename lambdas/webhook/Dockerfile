# we use a multi stage build so the final
# artifact is smaller. so we build using go image as build env.
# and then use aws lambda image for runtime env.
FROM golang:1.23 AS builder

WORKDIR /lambda

# move go mod and go sum (for deps.)
COPY go.mod go.sum ./

# install deps.
RUN go mod download

# copy source code into the container
COPY . .

# Build the binary using similar command from readme
RUN GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap .

# get runtime environment (Amazon Linux 2023 image)
FROM public.ecr.aws/lambda/provided:al2023 AS lambda-runtime

# copy go binary from the builder stage
# context: $LAMBDA_TASK_ROOT is env variable from
# AWS Lambda runtime where the Lambda function code is run
COPY --from=builder /lambda/bootstrap ${LAMBDA_TASK_ROOT}

ENTRYPOINT [ "./bootstrap" ]