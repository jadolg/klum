package github

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"

	"github.com/google/go-github/v53/github"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/oauth2"
)

type Config struct {
	GithubURL            string
	GithubToken          string
	GithubPrivateKeyFile string
	GithubAppID          int64
	GithubInstallationID int64
}

func (c *Config) Enabled() bool {
	return c.GithubToken != "" || (c.GithubInstallationID != 0 && c.GithubAppID != 0 && c.GithubPrivateKeyFile != "")
}

func newGithubClient(cfg Config) (*github.Client, error) {
	if cfg.GithubToken != "" {
		return newGithubClientWithToken(cfg.GithubToken, cfg.GithubURL)
	} else if cfg.GithubInstallationID != 0 && cfg.GithubAppID != 0 && cfg.GithubPrivateKeyFile != "" {
		return newGithubClientWithApp(cfg.GithubPrivateKeyFile, cfg.GithubAppID, cfg.GithubInstallationID, cfg.GithubURL)
	}
	return nil, fmt.Errorf("insufficient information provided. Github client can't be created")
}

func newGithubClientWithToken(token, privateURL string) (*github.Client, error) {
	var httpClient *http.Client

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	httpClient = oauth2.NewClient(context.Background(), ts)

	client := github.NewClient(httpClient)
	err := injectGithubClientPrivateURL(privateURL, client)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func newGithubClientWithApp(privateKeyFile string, appID int64, installationID int64, privateURL string) (*github.Client, error) {
	itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appID, installationID, privateKeyFile)
	if err != nil {
		return nil, err
	}
	err = injectTransportPrivateURL(privateURL, itr)
	if err != nil {
		return nil, err
	}

	client := github.NewClient(&http.Client{Transport: itr})

	err = injectGithubClientPrivateURL(privateURL, client)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func injectGithubClientPrivateURL(privateURL string, client *github.Client) error {
	if privateURL != "" {
		baseURL, err := getBaseURL(privateURL)
		if err != nil {
			return err
		}
		client.BaseURL = baseURL
	}
	return nil
}

func injectTransportPrivateURL(privateURL string, transport *ghinstallation.Transport) error {
	if privateURL != "" {
		baseURL, err := getBaseURL(privateURL)
		if err != nil {
			return err
		}
		transport.BaseURL = baseURL.String()
	}
	return nil
}

func getBaseURL(privateURL string) (*url.URL, error) {
	baseURL, err := url.Parse(privateURL)
	if err != nil {
		return nil, err
	}
	baseURL.Path = strings.TrimSuffix(baseURL.Path, "/") + "/api/v3/"
	return baseURL, nil
}

func getRepoID(ctx context.Context, client *github.Client, owner string, repo string) (int, error) {
	repository, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return 0, err
	}

	repositoryID := int(*repository.ID)
	return repositoryID, nil
}

func encodeWithPublicKey(text []byte, publicKey string) (string, error) {
	publicKeyDecoded, err := decodeKeyString(publicKey)
	if err != nil {
		return "", err
	}

	encrypted, err := box.SealAnonymous(nil, text, publicKeyDecoded, rand.Reader)
	if err != nil {
		return "", err
	}

	encryptedBase64 := base64.StdEncoding.EncodeToString(encrypted)

	return encryptedBase64, nil
}

func decodeKeyString(publicKey string) (*[32]byte, error) {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return nil, err
	}

	var publicKeyDecoded [32]byte
	if copy(publicKeyDecoded[:], publicKeyBytes) < 32 {
		return nil, fmt.Errorf("not a full length key, want 32 bytes, got %d bytes: %q", len(publicKeyBytes), publicKey)
	}

	return &publicKeyDecoded, nil
}
