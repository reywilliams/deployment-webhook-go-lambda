package dynamodb

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"webhook/logger"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	DYNAMO_DB_TABLE_NAME   string
	AWS_REGION             string
	dynamoDbClientInstance *dynamodb.Client

	log logger.Logger

	once sync.Once
)

const (
	DYNAMO_DB_TABLE_NAME_ENV_VAR_KEY = "DYNAMO_DB_TABLE_NAME"
	DYNAMO_DB_TABLE_NAME_DEFAULT     = "deployment-webhooks-table"

	AWS_REGION_ENV_VAR_KEY = "AWS_REGION"
	AWS_REGION_DEFAULT     = "us-west-2"
)

func init() {
	source_aws_region()           // sources AWS_REGION env. variable
	source_dynamo_db_table_name() // sources DYNAMO_DB_TABLE_NAME env. variable
}

func UserHasPermission(ctx context.Context, userEmail string, repository string, environment string) (*bool, error) {
	log.INFO("UserHasPermission() method - inputs: userEmail: %s, repository %s", userEmail, repository)

	partitionKey := userEmail // (hash_key in tf)
	sortKey := strings.Join([]string{repository, environment}, "#")

	dynamodbClient, err := getDynamoClient(ctx)
	if err != nil {
		log.ERROR("error observed while trying to get dynamodb client")
		return nil, err
	}

	input := dynamodb.GetItemInput{
		TableName: &DYNAMO_DB_TABLE_NAME,
		Key: map[string]types.AttributeValue{
			"email":    &types.AttributeValueMemberS{Value: partitionKey},
			"repo-env": &types.AttributeValueMemberS{Value: sortKey},
		},
	}

	result, err := dynamodbClient.GetItem(ctx, &input)
	if err != nil {
		log.ERROR("observed an error while trying to get entry from %s dynamodb table - PK: %s, SK: %s", DYNAMO_DB_TABLE_NAME, partitionKey, sortKey)
		return nil, fmt.Errorf("observed an error while trying to get entry from %s dynamodb table - (PK: %s, SK: %s): %w", DYNAMO_DB_TABLE_NAME, sortKey, sortKey, err)
	}

	userHasPerm := (result.Item != nil)

	return &userHasPerm, nil
}

func getDynamoClient(ctx context.Context) (*dynamodb.Client, error) {
	if err := configureDynamoDbClient(ctx); err != nil {
		return nil, err
	}
	return dynamoDbClientInstance, nil
}

func configureDynamoDbClient(ctx context.Context) error {
	var err error

	// ensures only one dynamodb client instance is created
	once.Do(func() {
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(AWS_REGION))
		if err != nil {
			log.ERROR("unable to load SDK config, %s", err.Error())
			return
		}
		dynamoDbClientInstance = dynamodb.NewFromConfig(cfg)
	})

	return err
}

/*
Retrieves AWS_REGION environment variable, sets global var (AWS_REGION) to:
-> AWS_REGION env var value if it is not empty and valid (per regex pattern)
-> AWS_REGION_DEFAULT if the above conditions are not met
*/
func source_aws_region() {
	envSourcedRegion := os.Getenv(AWS_REGION_ENV_VAR_KEY)
	log.INFO("AWS region environment variable (%s) retrieved with value: %s", AWS_REGION_ENV_VAR_KEY, envSourcedRegion)

	if envSourcedRegion == "" {
		log.WARN("AWS_REGION environment variable was empty, falling back to default: %s", AWS_REGION_DEFAULT)
		AWS_REGION = AWS_REGION_DEFAULT
	} else if !isValidAWSRegion(AWS_REGION) {
		log.WARN("AWS_REGION environment variable (%s) was invalid, falling back to default: %s", AWS_REGION, AWS_REGION_DEFAULT)
		AWS_REGION = AWS_REGION_DEFAULT
	} else {
		AWS_REGION = envSourcedRegion
	}
}

/*
Retrieves DYNAMO_DB_TABLE_NAME environment variable, sets global var (DYNAMO_DB_TABLE_NAME) to:
-> DYNAMO_DB_TABLE_NAME env var value if it is not empty
-> DYNAMO_DB_TABLE_NAME_DEFAULT if empty
*/
func source_dynamo_db_table_name() {
	envSourcedTableName := os.Getenv(DYNAMO_DB_TABLE_NAME_ENV_VAR_KEY)
	log.INFO("DynamoDB Table Name environment variable (%s) retrieved with value: %s", DYNAMO_DB_TABLE_NAME_ENV_VAR_KEY, envSourcedTableName)

	if envSourcedTableName == "" {
		log.WARN("DynamoDB Table Name environment variable was empty, falling back to default: %s", DYNAMO_DB_TABLE_NAME_DEFAULT)
		DYNAMO_DB_TABLE_NAME = DYNAMO_DB_TABLE_NAME_DEFAULT
	} else {
		DYNAMO_DB_TABLE_NAME = envSourcedTableName
	}
}

/*
*
Uses regex to determine if AWS region string
is valid
*/
func isValidAWSRegion(region string) bool {
	/**
	Regex pattern for AWS regions
	There are three (3) groups separated by dashes (-)
	<country_code>-<geo_location>-<region_number>

	For example:
	us-west-2
	**/
	pattern := `^(us|eu|ap|ca|sa|me|af)-(west|east|central|north|south|northeast|southeast|west|southwest|south|east)-(1|2)$`

	// Compile the regex
	re := regexp.MustCompile(pattern)

	// Check if the region matches the pattern
	return re.MatchString(region)
}
