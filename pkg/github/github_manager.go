package github

import (
	"fmt"
	"regexp"

	"github.com/ghodss/yaml"
	klum "github.com/jadolg/klum/pkg/apis/klum.cattle.io/v1alpha1"
	log "github.com/sirupsen/logrus"
)

func UploadGithubSecret(kubeconfig *klum.Kubeconfig, user *klum.User, githubURL string, githubToken string) error {
	owner, repo, env, secretName, done := getGitHubData(user)
	if done {
		log.Info("Not enough github data to be able to create a secret")
		return nil
	}
	log.Infof("Adding secret (%s) to GitHub for user %s to %s/%s", secretName, kubeconfig.Name, owner, repo)
	kubeconfigYAML, err := toYAMLString(kubeconfig.Spec)
	if err != nil {
		return err
	}

	return createRepositorySecret(githubURL, owner, repo, env, secretName, kubeconfigYAML, githubToken)
}

func DeleteGithubSecret(user *klum.User, githubURL string, githubToken string) error {
	owner, repo, env, secretName, done := getGitHubData(user)
	if done {
		log.Info("Not enough github data to be able to get any secret")
		return nil
	}
	log.Infof("Deleting secret (%s) from GitHub for user %s in %s/%s", secretName, user.Name, owner, repo)

	return deleteRepositorySecret(githubURL, owner, repo, env, secretName, githubToken)
}

func getGitHubData(user *klum.User) (string, string, string, string, bool) {
	owner, present := user.Annotations["github/owner"]
	if !present {
		return "", "", "", "", true
	}
	repo, present := user.Annotations["github/repo"]
	if !present {
		return "", "", "", "", true
	}
	env, _ := user.Annotations["github/env"]

	secretName, present := user.Annotations["github/name"]
	if !present {
		secretName = fmt.Sprintf("%s_KUBECONFIG", clearString(user.Name))
	}
	return owner, repo, env, secretName, false
}

func toYAMLString(x interface{}) (string, error) {
	b, err := yaml.Marshal(x)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func clearString(str string) string {
	nonAlphanumericRegex := regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}
