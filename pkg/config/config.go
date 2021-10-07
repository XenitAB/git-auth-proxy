package config

import (
	"strings"
)

type ProviderType string

const (
	AzureDevOpsProviderType = "azuredevops"
	GitHubProviderType      = "github"
)

type Configuration struct {
	Organizations []*Organization `json:"organizations" validate:"required,dive"`
}

type Organization struct {
	Provider     ProviderType  `json:"provider" validate:"required"`
	AzureDevOps  AzureDevOps   `json:"azuredevops"`
	GitHub       GitHub        `json:"github"`
	Host         string        `json:"host,omitempty" validate:"required"`
	Scheme       string        `json:"scheme,omitempty" validate:"required"`
	Name         string        `json:"name" validate:"required"`
	Repositories []*Repository `json:"repositories" validate:"required,dive"`
}

type AzureDevOps struct {
	Pat string `json:"pat"`
}

type GitHub struct {
	AppID          int64  `json:"appID"`
	InstallationID int64  `json:"installationID"`
	PrivateKey     string `json:"privateKey"`
}

func (o *Organization) GetSecretName(r *Repository) string {
	if r.SecretNameOverride != "" {
		return r.SecretNameOverride
	}

	comps := []string{}
	comps = append(comps, o.Name)
	if r.Project != "" {
		comps = append(comps, r.Project)
	}
	comps = append(comps, r.Name)
	return strings.Join(comps, "-")
}

type Repository struct {
	Project            string   `json:"project" validate:"required"`
	Name               string   `json:"name" validate:"required"`
	Namespaces         []string `json:"namespaces" validate:"required"`
	SecretNameOverride string   `json:"secretNameOverride,omitempty"`
}
