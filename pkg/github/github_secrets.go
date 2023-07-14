package github

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/url"

	"github.com/google/go-github/v53/github"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/oauth2"
)

func encodeWithPublicKey(text string, publicKey string) (string, error) {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return "", err
	}

	var publicKeyDecoded [32]byte
	copy(publicKeyDecoded[:], publicKeyBytes)

	encrypted, err := box.SealAnonymous(nil, []byte(text), (*[32]byte)(publicKeyBytes), rand.Reader)
	if err != nil {
		return "", err
	}

	encryptedBase64 := base64.StdEncoding.EncodeToString(encrypted)

	return encryptedBase64, nil
}

func createRepositorySecret(ctx context.Context, privateURL, owner, repo, secretName, secretValue, authToken string) error {
	client, err := newGithubClientWithToken(authToken, privateURL)
	if err != nil {
		return err
	}

	var key *github.PublicKey
	key, _, err = client.Actions.GetRepoPublicKey(ctx, owner, repo)
	if err != nil {
		return err
	}

	encryptedSecret, err := encodeWithPublicKey(secretValue, *key.Key)
	if err != nil {
		return err
	}

	secret := &github.EncryptedSecret{
		Name:           secretName,
		EncryptedValue: encryptedSecret,
	}

	_, err = client.Actions.CreateOrUpdateRepoSecret(ctx, owner, repo, secret)
	return err
}

func createRepositoryEnvSecret(ctx context.Context, privateURL, owner, repo, env, secretName, secretValue, authToken string) error {
	client, err := newGithubClientWithToken(authToken, privateURL)
	if err != nil {
		return err
	}

	var key *github.PublicKey
	var repositoryID int

	repositoryID, err = getRepoID(ctx, client, owner, repo)
	if err != nil {
		return err
	}
	key, _, err = client.Actions.GetEnvPublicKey(ctx, repositoryID, env)
	if err != nil {
		return err
	}

	encryptedSecret, err := encodeWithPublicKey(secretValue, *key.Key)
	if err != nil {
		return err
	}

	secret := &github.EncryptedSecret{
		Name:           secretName,
		EncryptedValue: encryptedSecret,
	}

	_, err = client.Actions.CreateOrUpdateEnvSecret(ctx, repositoryID, env, secret)
	return err
}

func getRepoID(ctx context.Context, client *github.Client, owner string, repo string) (int, error) {
	repository, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return 0, err
	}

	repositoryID := int(*repository.ID)
	return repositoryID, nil
}

func deleteRepositorySecret(ctx context.Context, privateURL, owner, repo, secretName, authToken string) error {
	client, err := newGithubClientWithToken(authToken, privateURL)
	if err != nil {
		return err
	}

	_, err = client.Actions.DeleteRepoSecret(ctx, owner, repo, secretName)
	return err
}

func deleteRepositoryEnvSecret(ctx context.Context, privateURL, owner, repo, env, secretName, authToken string) error {
	client, err := newGithubClientWithToken(authToken, privateURL)
	if err != nil {
		return err
	}

	repositoryID, err := getRepoID(ctx, client, owner, repo)
	if err != nil {
		return err
	}

	_, err = client.Actions.DeleteEnvSecret(ctx, repositoryID, env, secretName)
	return err
}

func newGithubClientWithToken(token, privateURL string) (*github.Client, error) {
	var httpClient *http.Client

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	httpClient = oauth2.NewClient(context.Background(), ts)

	client := github.NewClient(httpClient)
	if privateURL != "" {
		baseUrl, err := url.Parse(privateURL + "/api/v3/")
		if err != nil {
			return nil, err
		}
		client.BaseURL = baseUrl
	}
	return client, nil
}
