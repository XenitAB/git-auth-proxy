package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"

	"github.com/xenitab/azdo-proxy/pkg/config"
)

const tokenLenght = 64

type Authorization struct {
	endpoints map[string]*Endpoint
}

type Endpoint struct {
	Pat        string
	Domain     string
	Scheme     string
	Token      string
	Namespaces []string
	SecretName string

	// metadata used for reverse lookup
	Organization string
	Project      string
	Repository   string

	regexes []*regexp.Regexp
}

// NewAuthorization cretes a new authorization from a give configuration.
func NewAuthorization(cfg *config.Configuration) (Authorization, error) {
	authz := Authorization{endpoints: map[string]*Endpoint{}}
	for _, o := range cfg.Organizations {
		baseApi, err := regexp.Compile(fmt.Sprintf(`/%s/_apis\b`, o.Name))
		if err != nil {
			return Authorization{}, fmt.Errorf("invalid base api regex: %w", err)
		}

		for _, r := range o.Repositories {
			git, err := regexp.Compile(fmt.Sprintf(`/%s/%s/_git/%s(/.*)?\b`, o.Name, r.Project, r.Name))
			if err != nil {
				return Authorization{}, err
			}
			api, err := regexp.Compile(fmt.Sprintf(`/%s/%s/_apis/git/repositories/%s(/.*)?\b`, o.Name, r.Project, r.Name))
			if err != nil {
				return Authorization{}, err
			}
			token, err := randomSecureToken()
			if err != nil {
				return Authorization{}, fmt.Errorf("could not generate random token: %w", err)
			}

			e := Endpoint{
				Pat:          o.Pat,
				Domain:       o.Domain,
				Scheme:       o.Scheme,
				Token:        token,
				Namespaces:   r.Namespaces,
				SecretName:   o.GetSecretName(r),
				Organization: o.Name,
				Project:      r.Project,
				Repository:   r.Name,
				regexes:      []*regexp.Regexp{baseApi, git, api},
			}
			authz.endpoints[token] = &e
		}
	}

	return authz, nil
}

func (a *Authorization) GetEndpoints() map[string]*Endpoint {
	return a.endpoints
}

// LookupEndpoint returns the endpoint with the matching organization, project and repository.
func (a *Authorization) LookupEndpoint(domain, org, proj, repo string) (*Endpoint, error) {
	for _, e := range a.endpoints {
		if e.Domain == domain && e.Organization == org && e.Project == proj && e.Repository == repo {
			return e, nil
		}
	}
	return nil, errors.New("endpoint not found")
}

// PatForToken returns the pat associated with the token.
func (a *Authorization) GetPatForToken(token string) (string, error) {
	e, ok := a.endpoints[token]
	if !ok {
		return "", errors.New("invalid token")
	}
	return e.Pat, nil
}

// TargetForToken returns the target url which matches the given token.
func (a *Authorization) GetTargetForToken(token string) (*url.URL, error) {
	e, ok := a.endpoints[token]
	if !ok {
		return nil, errors.New("invalid token")
	}
	target, err := url.Parse(fmt.Sprintf("%s://%s", e.Scheme, e.Domain))
	if err != nil {
		return nil, fmt.Errorf("invalid url format: %w", err)
	}
	return target, nil
}

// IsPermitted checks if a specific token is permitted to access a path.
func (a *Authorization) IsPermitted(path string, token string) error {
	e, ok := a.endpoints[token]
	if !ok {
		return errors.New("invalid token")
	}
	for _, r := range e.regexes {
		if r.MatchString(path) {
			return nil
		}
	}
	return fmt.Errorf("invalid token")
}

func randomSecureToken() (string, error) {
	b := make([]byte, tokenLenght)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	randStr := base64.URLEncoding.EncodeToString(b)
	return randStr, nil
}
