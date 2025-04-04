package github

import (
	"context"
	"errors"
	"strings"

	"github.com/google/go-github/v63/github"
	"github.com/jadolg/klum/pkg/apis/klum.cattle.io/v1alpha1"
	log "github.com/sirupsen/logrus"
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
		// Secret exists, check if it's managed by Klum
		isKlumManaged, err := isSecretManagedByKlum(ctx, client, syncSpec.Owner, syncSpec.Repository, syncSpec.SecretName)
		if err != nil {
			log.WithFields(log.Fields{
				"repository": syncSpec.Repository,
				"secret":     syncSpec.SecretName,
				"error":      err,
			}).Warn("Failed to check if secret is managed by Klum")
		}

		if !isKlumManaged {
			log.WithFields(log.Fields{
				"repository": syncSpec.Repository,
				"secret":     syncSpec.SecretName,
			}).Info("Secret exists and is not managed by Klum, skipping")
			return nil
		}

		log.WithFields(log.Fields{
			"repository": syncSpec.Repository,
			"secret":     syncSpec.SecretName,
		}).Info("Secret exists and is managed by Klum, updating")
	} else {
		// If error is not 404 (not found), return the error
		var ghErr *github.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response.StatusCode != 404 {
			return err
		}

		// Secret doesn't exist, mark it as managed by Klum
		err = markSecretAsManagedByKlum(ctx, client, syncSpec.Owner, syncSpec.Repository, syncSpec.SecretName)
		if err != nil {
			log.WithFields(log.Fields{
				"repository": syncSpec.Repository,
				"secret":     syncSpec.SecretName,
				"error":      err,
			}).Warn("Failed to mark secret as managed by Klum")
		}
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

// isSecretManagedByKlum checks if a secret is managed by Klum by looking at the repository variables
func isSecretManagedByKlum(ctx context.Context, client *github.Client, owner, repo, secretName string) (bool, error) {
	// Get the repository variables
	vars, _, err := client.Actions.ListRepoVariables(ctx, owner, repo, nil)
	if err != nil {
		return false, err
	}

	// Look for the KLUM_MANAGED_SECRETS variable
	for _, v := range vars.Variables {
		if v.Name == KlumManagedSecretVar {
			// Check if the secret name is in the list of managed secrets
			managedSecrets := strings.Split(v.Value, ",")
			for _, s := range managedSecrets {
				if strings.TrimSpace(s) == secretName {
					return true, nil
				}
			}
			break
		}
	}

	return false, nil
}

// markSecretAsManagedByKlum adds a secret to the list of Klum-managed secrets
func markSecretAsManagedByKlum(ctx context.Context, client *github.Client, owner, repo, secretName string) error {
	// Get the repository variables
	vars, _, err := client.Actions.ListRepoVariables(ctx, owner, repo, nil)
	if err != nil {
		var ghErr *github.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response.StatusCode != 404 {
			return err
		}
	}

	// Check if the KLUM_MANAGED_SECRETS variable exists
	var managedSecrets []string
	var exists bool

	for _, v := range vars.Variables {
		if v.Name == KlumManagedSecretVar {
			managedSecrets = strings.Split(v.Value, ",")
			exists = true
			break
		}
	}

	// Add the secret to the list if it's not already there
	found := false
	for _, s := range managedSecrets {
		if strings.TrimSpace(s) == secretName {
			found = true
			break
		}
	}

	if !found {
		managedSecrets = append(managedSecrets, secretName)
	} else {
		// Secret is already in the list, nothing to do
		return nil
	}

	// Create or update the variable
	value := strings.Join(managedSecrets, ",")
	variable := &github.ActionsVariable{
		Name:  KlumManagedSecretVar,
		Value: value,
	}

	if exists {
		_, err = client.Actions.UpdateRepoVariable(ctx, owner, repo, variable)
	} else {
		_, err = client.Actions.CreateRepoVariable(ctx, owner, repo, variable)
	}

	return err
}
