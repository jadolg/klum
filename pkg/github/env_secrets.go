package github

import (
	"context"
	"errors"

	"github.com/google/go-github/v63/github"
	"github.com/jadolg/klum/pkg/apis/klum.cattle.io/v1alpha1"
	log "github.com/sirupsen/logrus"
)

func createRepositoryEnvSecret(ctx context.Context, client *github.Client, syncSpec *v1alpha1.GithubSyncSpec, secretValue []byte) error {
	var key *github.PublicKey
	var repositoryID int

	repositoryID, err := getRepoID(ctx, client, syncSpec.Owner, syncSpec.Repository)
	if err != nil {
		return err
	}

	_, _, err = client.Repositories.GetEnvironment(ctx, syncSpec.Owner, syncSpec.Repository, syncSpec.Environment)
	if err != nil {
		var ghErr *github.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response.StatusCode == 404 {
			log.WithFields(log.Fields{"environment": syncSpec.Environment, "repository": syncSpec.Repository}).Warn("Environment not found. Creating new environment.")
			_, _, err = client.Repositories.CreateUpdateEnvironment(ctx, syncSpec.Owner, syncSpec.Repository, syncSpec.Environment, &github.CreateUpdateEnvironment{})
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Check if the secret already exists
	_, _, err = client.Actions.GetEnvSecret(ctx, repositoryID, syncSpec.Environment, syncSpec.SecretName)
	if err == nil {
		// Secret exists, don't overwrite it
		return nil
	}
	
	// If error is not 404 (not found), return the error
	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) && ghErr.Response.StatusCode != 404 {
		return err
	}

	key, _, err = client.Actions.GetEnvPublicKey(ctx, repositoryID, syncSpec.Environment)
	if err != nil {
		return err
	}

	encryptedSecret, err := encodeWithPublicKey(secretValue, key.GetKey())
	if err != nil {
		return err
	}

	secret := &github.EncryptedSecret{
		Name:           syncSpec.SecretName,
		EncryptedValue: encryptedSecret,
	}

	_, err = client.Actions.CreateOrUpdateEnvSecret(ctx, repositoryID, syncSpec.Environment, secret)
	return err
}

func deleteRepositoryEnvSecret(ctx context.Context, client *github.Client, syncSpec *v1alpha1.GithubSyncSpec) error {
	repositoryID, err := getRepoID(ctx, client, syncSpec.Owner, syncSpec.Repository)
	if err != nil {
		return err
	}

	_, err = client.Actions.DeleteEnvSecret(ctx, repositoryID, syncSpec.Environment, syncSpec.SecretName)
	return err
}
