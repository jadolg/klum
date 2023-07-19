package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	klum "github.com/jadolg/klum/pkg/apis/klum.cattle.io/v1alpha1"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

func UploadKubeconfig(userSync *klum.UserSync, kubeconfig *klum.Kubeconfig, githubURL string, githubToken string) error {
	githubSync := userSync.Spec.Github
	if !githubSync.Valid() {
		return fmt.Errorf("not enough github data to be able to create a GitHub secret")
	}

	kubeconfigYAML, err := toYAMLString(kubeconfig.Spec)
	if err != nil {
		return err
	}

	kubeconfigYAMLb64 := base64.StdEncoding.EncodeToString([]byte(kubeconfigYAML))

	latestKubeconfigUploaded, present := userSync.Annotations["klum.cattle.io/lastest.upload.github"]
	if present && latestKubeconfigUploaded == kubeconfigYAMLb64 {
		return nil
	}
	userSync.Annotations["klum.cattle.io/lastest.upload.github"] = kubeconfigYAMLb64

	ctx := context.Background()
	time.Sleep(time.Second) // Calling GitHub continuously creates problems. This adds a buffer so all operations succeed.

	log.Infof("Adding secret (%s) to GitHub for user %s to %s/%s %s", githubSync.SecretName, kubeconfig.Name, githubSync.Owner, githubSync.Repository, githubSync.Environment)

	client, err := newGithubClientWithToken(githubURL, githubToken)
	if err != nil {
		return err
	}

	if githubSync.Environment == "" {
		return createRepositorySecret(
			ctx,
			client,
			githubSync,
			kubeconfigYAML,
		)
	} else {
		return createRepositoryEnvSecret(
			ctx,
			client,
			githubSync,
			kubeconfigYAML,
		)
	}
}

func DeleteKubeconfig(userSync *klum.UserSync, githubURL string, githubToken string) error {
	githubSync := userSync.Spec.Github
	if !githubSync.Valid() {
		log.Info("Not enough github data to be able to remove a GitHub secret")
		return nil
	}

	log.Infof("Deleting secret (%s) from GitHub for user %s in %s/%s %s", githubSync.SecretName, userSync.Spec.User, githubSync.Owner, githubSync.Repository, githubSync.Environment)

	client, err := newGithubClientWithToken(githubURL, githubToken)
	if err != nil {
		return err
	}

	ctx := context.Background()
	if githubSync.Environment == "" {
		return deleteRepositorySecret(
			ctx,
			client,
			githubSync,
		)
	} else {
		return deleteRepositoryEnvSecret(
			ctx,
			client,
			githubSync,
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
