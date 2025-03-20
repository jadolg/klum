package github

import (
	"context"
	"errors"

	"github.com/google/go-github/v63/github"
	"github.com/jadolg/klum/pkg/apis/klum.cattle.io/v1alpha1"
)

func createRepositorySecret(ctx context.Context, client *github.Client, syncSpec *v1alpha1.GithubSyncSpec, secretValue []byte) error {
	var key *github.PublicKey
	key, _, err := client.Actions.GetRepoPublicKey(ctx, syncSpec.Owner, syncSpec.Repository)
	if err != nil {
		return err
	}

	// Check if the secret already exists
	_, _, err = client.Actions.GetRepoSecret(ctx, syncSpec.Owner, syncSpec.Repository, syncSpec.SecretName)
	if err == nil {
		// Secret exists, don't overwrite it
		return nil
	}
	
	// If error is not 404 (not found), return the error
	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) && ghErr.Response.StatusCode != 404 {
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

	_, err = client.Actions.CreateOrUpdateRepoSecret(ctx, syncSpec.Owner, syncSpec.Repository, secret)
	return err
}

func deleteRepositorySecret(ctx context.Context, client *github.Client, syncSpec *v1alpha1.GithubSyncSpec) error {
	_, err := client.Actions.DeleteRepoSecret(ctx, syncSpec.Owner, syncSpec.Repository, syncSpec.SecretName)
	return err
}
