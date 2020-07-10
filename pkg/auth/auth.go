package auth

import (
	"fmt"
	"regexp"

	"github.com/xenitab/azdo-proxy/pkg/config"
)

type Authorization struct {
	Endpoints []Endpoint
}

type Endpoint struct {
	Token   string
	Regexes []*regexp.Regexp
}

// GeneGenerateAuthorization creates regex resources to validate tokens and endpoints
func GenerateAuthorization(c config.Configuration) (*Authorization, error) {
	baseApi, err := regexp.Compile(`/` + c.Organization + `/_apis\b`)
	if err != nil {
		return nil, err
	}

	auth := &Authorization{Endpoints: []Endpoint{}}
	for _, r := range c.Repositories {
		git, err := regexp.Compile(`/` + c.Organization + `/` + r.Project + `/_git/` + r.Name + `(/.*)?\b`)
		if err != nil {
			return nil, err
		}

		api, err := regexp.Compile(`/` + c.Organization + `/` + r.Project + `/_apis/git/repositories/` + r.Name + `(/.*)?\b`)
		if err != nil {
			return nil, err
		}

		endpoint := Endpoint{
			Token:   r.Token,
			Regexes: []*regexp.Regexp{baseApi, git, api},
		}
		auth.Endpoints = append(auth.Endpoints, endpoint)
	}

	return auth, nil
}

// IsPermitted checks if a specific user is permitted to access a path
func IsPermitted(a *Authorization, p string, t string) error {
	for _, e := range a.Endpoints {
		// Only check regex for matching tokens
		if e.Token != t {
			continue
		}

		// Return of a regex matches the path
		for _, r := range e.Regexes {
			if r.MatchString(p) {
				fmt.Println(p)
				fmt.Printf("Path %v matches %v regex\n", p, r.String())
				return nil
			}
		}
	}

	return fmt.Errorf("Could not find any matching configured repositories: %v", p)
}
