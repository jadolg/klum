package github

import (
	"context"
	"fmt"
	"time"

	"github.com/ghodss/yaml"
	klum "github.com/jadolg/klum/pkg/apis/klum.cattle.io/v1alpha1"
	log "github.com/sirupsen/logrus"
)

func UploadGithubSecret(userSync *klum.UserSync, kubeconfig *klum.Kubeconfig, githubURL string, githubToken string) error {
	githubSync := userSync.Spec.Github
	if !githubSync.Valid() {
		return fmt.Errorf("not enough github data to be able to create a GitHub secret")
	}

	log.Infof("Adding secret (%s) to GitHub for user %s to %s/%s %s", githubSync.SecretName, kubeconfig.Name, githubSync.Owner, githubSync.Repository, githubSync.Environment)

	kubeconfigYAML, err := toYAMLString(kubeconfig.Spec)
	if err != nil {
		return err
	}

	ctx := context.Background()

	time.Sleep(time.Second) // Calling GitHub continuously creates problems. This adds a buffer so all operations succeed.

	if githubSync.Environment == "" {
		return createRepositorySecret(
			ctx,
			githubURL,
			githubSync.Owner,
			githubSync.Repository,
			githubSync.SecretName,
			kubeconfigYAML,
			githubToken,
		)
	} else {
		return createRepositoryEnvSecret(
			ctx,
			githubURL,
			githubSync.Owner,
			githubSync.Repository,
			githubSync.Environment,
			githubSync.SecretName,
			kubeconfigYAML,
			githubToken,
		)
	}
}

func DeleteGithubSecret(userSync *klum.UserSync, githubURL string, githubToken string) error {
	githubSync := userSync.Spec.Github
	if !githubSync.Valid() {
		log.Info("Not enough github data to be able to remove a GitHub secret")
		return nil
	}

	log.Infof("Deleting secret (%s) from GitHub for user %s in %s/%s %s", githubSync.SecretName, userSync.Spec.User, githubSync.Owner, githubSync.Repository, githubSync.Environment)
	ctx := context.Background()
	if githubSync.Environment == "" {
		return deleteRepositorySecret(
			ctx,
			githubURL,
			githubSync.Owner,
			githubSync.Repository,
			githubSync.SecretName,
			githubToken,
		)
	} else {
		return deleteRepositoryEnvSecret(
			ctx,
			githubURL,
			githubSync.Owner,
			githubSync.Repository,
			githubSync.Environment,
			githubSync.SecretName,
			githubToken,
		)
	}
}

func toYAMLString(x interface{}) (string, error) {
	b, err := yaml.Marshal(x)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
