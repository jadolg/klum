package github

import (
	"fmt"

	"github.com/ghodss/yaml"
	klum "github.com/jadolg/klum/pkg/apis/klum.cattle.io/v1alpha1"
	log "github.com/sirupsen/logrus"
)

func UploadGithubSecret(kubeconfig *klum.Kubeconfig, user *klum.User, githubURL string, githubToken string) error {
	if user.Spec.Sync.Github == nil {
		return nil
	}
	if !user.Spec.Sync.Github.Valid() {
		return fmt.Errorf("not enough github data to be able to create a GitHub secret")
	}

	log.Infof("Adding secret (%s) to GitHub for user %s to %s/%s", user.Spec.Sync.Github.SecretName, kubeconfig.Name, user.Spec.Sync.Github.Owner, user.Spec.Sync.Github.Repository)

	kubeconfigYAML, err := toYAMLString(kubeconfig.Spec)
	if err != nil {
		return err
	}

	return createRepositorySecret(
		githubURL,
		user.Spec.Sync.Github.Owner,
		user.Spec.Sync.Github.Repository,
		user.Spec.Sync.Github.Environment,
		user.Spec.Sync.Github.SecretName,
		kubeconfigYAML,
		githubToken,
	)
}

func DeleteGithubSecret(user *klum.User, githubURL string, githubToken string) error {
	if !user.Spec.Sync.Github.Valid() {
		log.Info("Not enough github data to be able to remove a GitHub secret")
		return nil
	}

	log.Infof("Deleting secret (%s) from GitHub for user %s in %s/%s", user.Spec.Sync.Github.SecretName, user.Name, user.Spec.Sync.Github.Owner, user.Spec.Sync.Github.Repository)

	return deleteRepositorySecret(
		githubURL,
		user.Spec.Sync.Github.Owner,
		user.Spec.Sync.Github.Repository,
		user.Spec.Sync.Github.Environment,
		user.Spec.Sync.Github.SecretName,
		githubToken,
	)
}

func toYAMLString(x interface{}) (string, error) {
	b, err := yaml.Marshal(x)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
