cd src/

go mod tidy

# compiles go program
# Context on these commands can be found here:
# https://docs.aws.amazon.com/lambda/latest/dg/golang-package.html#golang-package-mac-linux
GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o ./../build/bootstrap

# zip the go executable in a zip file (Terraform will reference this)
zip -j ./../build/lambda.zip ./../build/bootstrap

cd - 