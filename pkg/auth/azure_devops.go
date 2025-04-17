package auth

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"regexp"
)

type azureDevops struct {
	pat string
}

func newAzureDevops(pat string) *azureDevops {
	return &azureDevops{
		pat: pat,
	}
}

//nolint:staticcheck // ignore this
func (a *azureDevops) getPathRegex(organization, project, repository string) ([]*regexp.Regexp, error) {
	baseApi, err := regexp.Compile(fmt.Sprintf(`(?i)/%s/_apis\b`, organization))
	if err != nil {
		return nil, fmt.Errorf("invalid base api regex: %w", err)
	}
	git, err := regexp.Compile(fmt.Sprintf(`(?i)/%s/%s/_git/%s(/.*)?\b`, organization, project, repository))
	if err != nil {
		return nil, err
	}
	api, err := regexp.Compile(fmt.Sprintf(`(?i)/%s/%s/_apis/git/repositories/%s(/.*)?\b`, organization, project, repository))
	if err != nil {
		return nil, err
	}
	return []*regexp.Regexp{baseApi, git, api}, nil
}

func (a *azureDevops) getAuthorizationHeader(ctx context.Context, path string) (string, error) {
	tokenB64 := b64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("pat:%s", a.pat)))
	return fmt.Sprintf("Basic %s", tokenB64), nil
}

func (a *azureDevops) getHost(e *Endpoint, path string) string {
	return e.host
}

func (a *azureDevops) getPath(e *Endpoint, path string) string {
	return path
}
