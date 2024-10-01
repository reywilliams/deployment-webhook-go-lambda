# Context on these commands can be found here:
# https://docs.aws.amazon.com/lambda/latest/dg/golang-package.html#golang-package-mac-linux

# Download go modules
go mod tidy

# compiles go program
GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o archive/bootstrap main.go 

# zip the go executable in a zip file (Terraform will reference this)
zip -j archive/lambda.zip archive/bootstrap