package github

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"github.com/google/go-github/v53/github"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
)

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

func getRepoID(ctx context.Context, client *github.Client, owner string, repo string) (int, error) {
	repository, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return 0, err
	}

	repositoryID := int(*repository.ID)
	return repositoryID, nil
}

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
