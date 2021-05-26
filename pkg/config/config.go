package config

import (
	"fmt"
	"net/url"
)

type Configuration struct {
	Organizations []Organization `json:"organizations" validate:"required,dive"`
}

type Organization struct {
	Name         string       `json:"name" validate:"required"`
	Domain       string       `json:"domain,omitempty" validate:"required"`
	Scheme       string       `json:"scheme,omitempty" validate:"required"`
	Pat          string       `json:"pat" validate:"required"`
	Repositories []Repository `json:"repositories" validate:"required,dive"`
}

func (o Organization) GetTarget() (*url.URL, error) {
	u, err := url.Parse(fmt.Sprintf("%s://%s", o.Scheme, o.Domain))
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (o Organization) GetSecretName(r Repository) string {
	if r.SecretNameOverride != "" {
		return r.SecretNameOverride
	}
	return fmt.Sprintf("%s-%s-%s", o.Name, r.Project, r.Name)
}

type Repository struct {
	Project            string   `json:"project" validate:"required"`
	Name               string   `json:"name" validate:"required"`
	Namespaces         []string `json:"namespaces" validate:"required"`
	SecretNameOverride string   `json:"secretNameOverride,omitempty"`
}
