package config

import (
	"encoding/json"
	iofs "io/fs"

	"github.com/go-playground/validator/v10"
)

const (
	defaultDomain = "dev.azure.com"
	defaultScheme = "https"
)

// LoadConfiguration parses and validates the configuration file at a given path.
func LoadConfiguration(fs iofs.FS, path string) (*Configuration, error) {
	b, err := iofs.ReadFile(fs, path)
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

func setConfigurationDefaults(cfg *Configuration) *Configuration {
	for i, o := range cfg.Organizations {
		if o.Domain == "" {
			cfg.Organizations[i].Domain = defaultDomain
		}
		if o.Scheme == "" {
			cfg.Organizations[i].Scheme = defaultScheme
		}
	}
	return cfg
}
