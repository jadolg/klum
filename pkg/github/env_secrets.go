package github

import (
	"context"
	"errors"
	"strings"

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
		// Secret exists, check if it's managed by Klum
		isKlumManaged, err := isEnvSecretManagedByKlum(ctx, client, syncSpec.Owner, syncSpec.Repository, syncSpec.Environment, syncSpec.SecretName)
		if err != nil {
			log.WithFields(log.Fields{
				"repository":  syncSpec.Repository,
				"environment": syncSpec.Environment,
				"secret":      syncSpec.SecretName,
				"error":       err,
			}).Warn("Failed to check if environment secret is managed by Klum")
		}

		if !isKlumManaged {
			log.WithFields(log.Fields{
				"repository":  syncSpec.Repository,
				"environment": syncSpec.Environment,
				"secret":      syncSpec.SecretName,
			}).Info("Environment secret exists and is not managed by Klum, skipping")
			return nil
		}

		log.WithFields(log.Fields{
			"repository":  syncSpec.Repository,
			"environment": syncSpec.Environment,
			"secret":      syncSpec.SecretName,
		}).Info("Environment secret exists and is managed by Klum, updating")
	} else {
		// If error is not 404 (not found), return the error
		var ghErr *github.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response.StatusCode != 404 {
			return err
		}

		// Secret doesn't exist, mark it as managed by Klum
		err = markEnvSecretAsManagedByKlum(ctx, client, syncSpec.Owner, syncSpec.Repository, syncSpec.Environment, syncSpec.SecretName)
		if err != nil {
			log.WithFields(log.Fields{
				"repository":  syncSpec.Repository,
				"environment": syncSpec.Environment,
				"secret":      syncSpec.SecretName,
				"error":       err,
			}).Warn("Failed to mark environment secret as managed by Klum")
		}
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

// isEnvSecretManagedByKlum checks if an environment secret is managed by Klum by looking at the environment variables
func isEnvSecretManagedByKlum(ctx context.Context, client *github.Client, owner, repo, environment, secretName string) (bool, error) {
	repositoryID, err := getRepoID(ctx, client, owner, repo)
	if err != nil {
		return false, err
	}

	// Get the environment variables
	vars, _, err := client.Actions.ListEnvVariables(ctx, repositoryID, environment, nil)
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

// markEnvSecretAsManagedByKlum adds an environment secret to the list of Klum-managed secrets
func markEnvSecretAsManagedByKlum(ctx context.Context, client *github.Client, owner, repo, environment, secretName string) error {
	repositoryID, err := getRepoID(ctx, client, owner, repo)
	if err != nil {
		return err
	}

	// Get the environment variables
	vars, _, err := client.Actions.ListEnvVariables(ctx, repositoryID, environment, nil)
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
		_, err = client.Actions.UpdateEnvVariable(ctx, repositoryID, environment, variable)
	} else {
		_, err = client.Actions.CreateEnvVariable(ctx, repositoryID, environment, variable)
	}

	return err
}
