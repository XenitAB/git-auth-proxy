package config

import (
	"encoding/json"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/afero"
)

const (
	defaultScheme = "https"
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
	Provider     ProviderType  `json:"provider" validate:"required,oneof='azuredevops' 'github'"`
	AzureDevOps  AzureDevOps   `json:"azuredevops"`
	GitHub       GitHub        `json:"github"`
	Host         string        `json:"host,omitempty" validate:"required,hostname"`
	Scheme       string        `json:"scheme,omitempty" validate:"required"`
	Name         string        `json:"name" validate:"required"`
	Repositories []*Repository `json:"repositories" validate:"required,dive"`
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

type AzureDevOps struct {
	Pat string `json:"pat"`
}

type GitHub struct {
	AppID          int64  `json:"appID"`
	InstallationID int64  `json:"installationID"`
	PrivateKey     string `json:"privateKey"`
}

type Repository struct {
	Project            string   `json:"project"`
	Name               string   `json:"name" validate:"required"`
	Namespaces         []string `json:"namespaces" validate:"required"`
	SecretNameOverride string   `json:"secretNameOverride,omitempty"`
}

func setConfigurationDefaults(cfg *Configuration) *Configuration {
	for i, o := range cfg.Organizations {
		if o.Scheme == "" {
			cfg.Organizations[i].Scheme = defaultScheme
		}
	}
	return cfg
}

// LoadConfiguration parses and validates the configuration file at a given path.
func LoadConfiguration(fs afero.Fs, path string) (*Configuration, error) {
	b, err := afero.ReadFile(fs, path)
	if err != nil {
		return nil, err
	}

	cfg := &Configuration{}
	err = json.Unmarshal(b, &cfg)
	if err != nil {
		return nil, err
	}
	cfg = setConfigurationDefaults(cfg)

	validate := validator.New()
	err = validate.Struct(cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
