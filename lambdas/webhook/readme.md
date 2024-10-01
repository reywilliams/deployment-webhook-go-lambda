# Relevant Info

## General Info on Making a Go Lambda
You can find information on creating a Go lambda function here: Define [Lambda function handler in Go](https://docs.aws.amazon.com/lambda/latest/dg/golang-handler.html)

### Go Lambda Runtimes (OS Runtimes)
Per the AWS [docs](https://docs.aws.amazon.com/lambda/latest/dg/golang-handler.html#golang-handler-naming):
> For Go functions that use the provided.al2 or provided.al2023 runtime in a .zip deployment package, the executable file that contains your function code must be named bootstrap.

### Creating your `.zip` archive/deployment package For Your Lambda
See this AWS doc here for information on [creating a .zip file on macOS and Linux](https://docs.aws.amazon.com/lambda/latest/dg/golang-package.html#golang-package-mac-linux).

For this project, the command I have used the following command. Note that these commands are also available in the shell script [build_lambda.sh](build_lambda.sh).

```shell
GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o archive/bootstrap main.go && zip archive/lambda.zip archive/bootstrap
```
or
```shell
chmod +x build_lamda.sh
./build_lambda.sh
```

Note that I chose ARM 64 due to the advantages outlined in [Selecting and configuring an instruction set architecture for your Lambda function](https://docs.aws.amazon.com/lambda/latest/dg/foundation-arch.html#foundation-arch-adv). 

The [TL;DR](https://www.merriam-webster.com/dictionary/TL%3BDR) is:

> Lambda functions that use arm64 architecture (AWS Graviton2 processor) can achieve significantly better price and performance than the equivalent function running on x86_64 architecture

