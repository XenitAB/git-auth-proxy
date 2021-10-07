package config

import (
	"encoding/json"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/afero"
)

const (
	defaultScheme = "https"
)

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

func setConfigurationDefaults(cfg *Configuration) *Configuration {
	for i, o := range cfg.Organizations {
		if o.Scheme == "" {
			cfg.Organizations[i].Scheme = defaultScheme
		}
	}
	return cfg
}
