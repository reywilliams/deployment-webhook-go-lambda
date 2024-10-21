package github

import (
	"context"
	"sync"
	"webhook/logger"
	"webhook/secrets"
	"webhook/util"

	"github.com/google/go-github/v66/github"
	"go.uber.org/zap"
)

var (
	githubClientInstance *github.Client

	once sync.Once

	log zap.SugaredLogger

	githubPAT     string
	sourcingError error
)

const (
	GITHUB_PAT_SECRET_NAME_ENV_VAR_KEY = "GITHUB_PAT_SECRET_NAME"
	GITHUB_PAT_SECRET_NAME_DEFAULT     = "GITHUB_PAT_SECRET"
)

func init() {
	log = *logger.GetLogger().Sugar()
}

func GetGitHubClient(ctx context.Context) (*github.Client, error) {

	// see if we already got a sourcing error
	if sourcingError != nil {
		log.Errorln("cannot source github PAT, cannot create new Github client instance", zap.Error(sourcingError))
		return nil, sourcingError
	}

	// only source PAT and setup instance once
	once.Do(func() {
		sourcePATSecret(ctx)
		githubClientInstance = github.NewClient(nil).WithAuthToken(githubPAT)
	})

	return githubClientInstance, nil
}

/*
We attempt to get the secret name from environment variables
using the secret name sourced from GITHUB_PAT_SECRET_NAME_ENV_VAR_KEY
or GITHUB_PAT_SECRET_NAME_DEFAULT if that environment variable is not found
*/
func getWebhookPAT(ctx context.Context) (*string, error) {

	ghPATSecretName := util.LookupEnv(GITHUB_PAT_SECRET_NAME_ENV_VAR_KEY, GITHUB_PAT_SECRET_NAME_DEFAULT, false)
	ghPAT, err := secrets.GetSecretValue(ctx, ghPATSecretName)
	if ghPAT == nil || err != nil {
		log.Errorln("error while getting github PAT secret value")
		return nil, err
	}

	return ghPAT, nil
}

/*
Sources Github PAT secret and sets secretPAT it if sourced.
Returns error and leaves var secretPAT unset if not sourced successfully.
*/
func sourcePATSecret(ctx context.Context) {

	secretPAT, err := getWebhookPAT(ctx)

	if err != nil {
		sourcingError = err
		return
	}

	githubPAT = *secretPAT
}
