package auth

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"github.com/xenitab/azdo-proxy/pkg/config"
)

type Provider interface {
	getPathRegex(organization, project, repository string) ([]*regexp.Regexp, error)
	getAuthorizationHeader(ctx context.Context, path string) (string, error)
	getHost(e *Endpoint, path string) string
	getPath(e *Endpoint, path string) string
}

type Authorizer struct {
	providers        map[string]Provider
	endpoints        []*Endpoint
	endpointsByID    map[string]*Endpoint
	endpointsByToken map[string]*Endpoint
}

func NewAuthorizer(cfg *config.Configuration) (*Authorizer, error) {
	providers := map[string]Provider{}
	endpoints := []*Endpoint{}
	endpointsByID := map[string]*Endpoint{}
	endpointsByToken := map[string]*Endpoint{}

	for _, o := range cfg.Organizations {
		// Get the correct provider for the organization
		var provider Provider
		switch o.Provider {
		case config.AzureDevOpsProviderType:
			provider = newAzureDevops(o.AzureDevOps.Pat)
		case config.GitHubProviderType:
			pemData, err := b64.URLEncoding.DecodeString(o.GitHub.PrivateKey)
			if err != nil {
				return nil, err
			}
			provider, err = newGithub(o.GitHub.AppID, o.GitHub.InstallationID, pemData)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("invalid provider type %s", o.Provider)
		}

		// Create endpoints for the repositories
		for _, r := range o.Repositories {
			pathRegex, err := provider.getPathRegex(o.Name, r.Project, r.Name)
			if err != nil {
				return nil, fmt.Errorf("could not get path regex: %w", err)
			}

			token, err := randomSecureToken()
			if err != nil {
				return nil, fmt.Errorf("could not generate random token: %w", err)
			}

			e := &Endpoint{
				host:         o.Host,
				scheme:       o.Scheme,
				organization: o.Name,
				project:      r.Project,
				repository:   r.Name,
				regexes:      pathRegex,
				Token:        token,
				Namespaces:   r.Namespaces,
				SecretName:   o.GetSecretName(r),
			}

			providers[e.ID()] = provider
			endpoints = append(endpoints, e)
			endpointsByID[e.ID()] = e
			endpointsByToken[e.Token] = e
		}
	}

	authz := &Authorizer{
		providers:        providers,
		endpoints:        endpoints,
		endpointsByID:    endpointsByID,
		endpointsByToken: endpointsByToken,
	}
	return authz, nil
}

func (a *Authorizer) GetEndpoints() []*Endpoint {
	return a.endpoints
}

func (a *Authorizer) GetEndpointById(id string) (*Endpoint, error) {
	e, ok := a.endpointsByID[id]
	if !ok {
		return nil, fmt.Errorf("endpoint not found for id %s", id)
	}
	return e, nil
}

func (a *Authorizer) GetEndpointByToken(token string) (*Endpoint, error) {
	e, ok := a.endpointsByToken[token]
	if !ok {
		return nil, fmt.Errorf("endpoint not found for given token")
	}
	return e, nil
}

func (a *Authorizer) IsPermitted(path string, token string) error {
	e, err := a.GetEndpointByToken(token)
	if err != nil {
		return err
	}
	for _, r := range e.regexes {
		if r.MatchString(path) {
			return nil
		}
	}
	return fmt.Errorf("token not permitted for path %s", path)
}

func (a *Authorizer) UpdateRequest(ctx context.Context, req *http.Request, token string) (*http.Request, *url.URL, error) {
	e, err := a.GetEndpointByToken(token)
	if err != nil {
		return nil, nil, err
	}
	provider, ok := a.providers[e.ID()]
	if !ok {
		return nil, nil, fmt.Errorf("provider not found for id %s", e.ID())
	}

	host := provider.getHost(e, req.URL.Path)
	path := provider.getPath(e, req.URL.Path)
	authorizationHeader, err := provider.getAuthorizationHeader(ctx, req.URL.Path)
	if err != nil {
		return nil, nil, err
	}
	url, err := url.Parse(fmt.Sprintf("%s://%s", e.scheme, host))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid url format: %w", err)
	}

	req.Host = host
	req.URL.Path = path
	req.Header.Del("Authorization")
	req.Header.Add("Authorization", authorizationHeader)
	return req, url, nil
}
