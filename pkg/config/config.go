package config

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"

	validate "github.com/go-playground/validator/v10"
)

type Repository struct {
	Project string `json:"project" validate:"required"`
	Name    string `json:"name" validate:"required"`
	Token   string `json:"token" validate:"required"`
}

type Configuration struct {
	Domain       string       `json:"domain" validate:"required"`
	Pat          string       `json:"pat" validate:"required"`
	Organization string       `json:"organization" validate:"required"`
	Repositories []Repository `json:"repositories" validate:"required"`
}

// LoadConfiguration reads json from the given path and parses it.
func LoadConfigurationFromPath(path string) (*Configuration, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return LoadConfiguration(f)
}

// LoadConfiguration reads json from the given reader and parses it.
func LoadConfiguration(src io.Reader) (*Configuration, error) {
	b, err := ioutil.ReadAll(src)
	if err != nil {
		return nil, err
	}

	c := &Configuration{}
	err = json.Unmarshal(b, c)
	if err != nil {
		return nil, err
	}

	err = validate.New().Struct(c)
	if err != nil {
		return nil, err
	}

	return c, nil
}
