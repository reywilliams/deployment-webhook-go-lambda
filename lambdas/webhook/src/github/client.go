package github

import (
	"sync"
	"webhook/util"

	"github.com/google/go-github/v66/github"
)

var (
	githubClientInstance *github.Client

	once sync.Once
)

const (
	GITHUB_PAT_ENV_VAR_KEY = "GITHUB_PAT"
)

func init() {
}

func GetGitHubClient() *github.Client {
	once.Do(func() {
		githubClientInstance = github.NewClient(nil).WithAuthToken(util.LookupEnv(GITHUB_PAT_ENV_VAR_KEY, "", true))
	})
	return githubClientInstance
}
