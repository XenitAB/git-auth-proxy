package auth

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
)

const standardGitHub = "github.com"

type GitHubTokenSource interface {
	Token(ctx context.Context) (string, error)
}

type github struct {
	itr GitHubTokenSource
}

func newGithub(appID, installationID int64, privateKey []byte) (*github, error) {
	itr, err := ghinstallation.New(http.DefaultTransport, appID, installationID, privateKey)
	if err != nil {
		return nil, err
	}
	return &github{itr: itr}, nil
}

func (g *github) getPathRegex(organization, project, repository string) ([]*regexp.Regexp, error) {
	git, err := regexp.Compile(fmt.Sprintf(`(?i)/%s/%s(/.*)?\b`, organization, repository))
	if err != nil {
		return nil, err
	}
	api, err := regexp.Compile(fmt.Sprintf(`(?i)/api/v3/(.*)/%s/%s/(/.*)?\b`, organization, repository))
	if err != nil {
		return nil, err
	}
	return []*regexp.Regexp{git, api}, nil
}

func (g *github) getAuthorizationHeader(ctx context.Context, path string) (string, error) {
	token, err := g.itr.Token(ctx)
	if err != nil {
		return "", fmt.Errorf("error when fetching GitHub JWT token: %w", err)
	}

	if strings.HasPrefix(path, "/api/v3/") {
		return fmt.Sprintf("Bearer %s", token), nil
	}
	tokenB64 := b64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("x-access-token:%s", token)))
	return fmt.Sprintf("Basic %s", tokenB64), nil
}

func (g *github) getHost(e *Endpoint, path string) string {
	if e.host != standardGitHub {
		return e.host
	}
	if strings.HasPrefix(path, "/api/v3/") {
		return fmt.Sprintf("api.%s", e.host)
	}
	return e.host
}

func (g *github) getPath(e *Endpoint, path string) string {
	if e.host != standardGitHub {
		return path
	}
	newPath := strings.TrimPrefix(path, "/api/v3")
	return newPath
}
