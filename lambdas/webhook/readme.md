# Webhook Go Lambda

## General Info on Making a Go Lambda

You can find information on creating a Go lambda function here: Define [Lambda function handler in Go](https://docs.aws.amazon.com/lambda/latest/dg/golang-handler.html)

### Go Lambda Runtimes (OS Runtimes)

Per the AWS [docs](https://docs.aws.amazon.com/lambda/latest/dg/golang-handler.html#golang-handler-naming):

> For Go functions that use the provided.al2 or provided.al2023 runtime in a .zip deployment package, the executable file that contains your function code must be named bootstrap.

### Creating your `.zip` archive/deployment package For Your Lambda

See this AWS doc here for information on [creating a .zip file on macOS and Linux](https://docs.aws.amazon.com/lambda/latest/dg/golang-package.html#golang-package-mac-linux).

For this project, the command I have used the following command. Note that these commands are also available in the shell script [build_lambda.sh](scripts/build_lambda.sh).

```shell
make build
```

Note that I chose ARM 64 due to the advantages outlined in [Selecting and configuring an instruction set architecture for your Lambda function](https://docs.aws.amazon.com/lambda/latest/dg/foundation-arch.html#foundation-arch-adv).

The [TL;DR](https://www.merriam-webster.com/dictionary/TL%3BDR) is:

> Lambda functions that use arm64 architecture (AWS Graviton2 processor) can achieve significantly better price and performance than the equivalent function running on x86_64 architecture

# Testing Lambda

# Local Invoke

> More can be found on this process [here](https://docs.aws.amazon.com/lambda/latest/dg/go-image.html) in the AWS docs.

## Build The Image

Build the docker image

```shell
make docker-build
```

## Run the Image

Run the image/container that was just built

```shell
make docker-run
```

### Follow Logs While Running

Follow & view the logs for your container and verify the behavior

```shell
make docker-logs
```

### Test The Running Image

Test the lambda using cURL and a sample payload (I have made the sample payload file [lambda_sample_payload.json](config/lambda_sample_payload.json)) that is used to build a valid API Gateway Proxy Request in [api_gw_sample_payload.json](config/api_gw_sample_payload.json)

```shell
make docker-test
```

You should get a response like the following:

```shell
{"statusCode":200,"headers":null,"multiValueHeaders":null,"body":"event processed"}%
```

> **Note:** Editing this sample payload will cause the validation to fail due to the signature in the `X-Hub-Signature-256` header now being invalid.

## Clean Up

Kill the container once you are done to free up that port again

```shell
make docker-kill
```

# Live Invoke

I used the following commands to invoke the lambda and test it using the sample event

**Note**: Remember to set your profile if you're using AWS CLI profiles

```shell
aws lambda invoke \
    --function-name <function_name> \
    --payload </path/to>file://api_gw_sample_payload.json \
    --cli-binary-format raw-in-base64-out \
    --region us-west-2 \
    --profile <profile> \
    /dev/stdout
```
