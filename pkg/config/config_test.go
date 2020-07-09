package config

import (
	"strings"
	"testing"
)

const validJson = `
{
  "domain": "dev.azure.com",
  "pat": "foobar",
  "organization": "xenitab",
  "repositories": [
    {
      "name": "gitops-deployment",
      "project": "Lab",
      "token": "foobar"
    }
  ]
}
`

const invalidJson = `
{
  "domain": "dev.azure.com",
	}}
}
`

const missingParamJson = `
{
  "domain": "dev.azure.com",
  "pat": "foobar",
  "repositories": [
    {
      "name": "gitops-deployment",
      "project": "Lab",
      "token": "foobar"
    }
  ]
}
`

const minimalJson = `
{
  "pat": "foobar",
  "organization": "xenitab",
  "repositories": [
    {
      "name": "gitops-deployment",
      "project": "Lab",
      "token": "foobar"
    }
  ]
}
`

func TestValidJson(t *testing.T) {
	reader := strings.NewReader(validJson)
	_, err := LoadConfiguration(reader)
	if err != nil {
		t.Errorf("could not parse json: %v", err)
	}
}

func TestInvalidJson(t *testing.T) {
	reader := strings.NewReader(invalidJson)
	_, err := LoadConfiguration(reader)
	if err == nil {
		t.Error("error should not be nil")
	}
}

func TestMissingParam(t *testing.T) {
	reader := strings.NewReader(missingParamJson)
	_, err := LoadConfiguration(reader)
	if err == nil {
		t.Error("error should not be nil")
	}
}

func TestMinimalJson(t *testing.T) {
	reader := strings.NewReader(minimalJson)
	c, err := LoadConfiguration(reader)
	if err != nil {
		t.Errorf("could not parse json: %v", err)
	}

	if c.Domain != "dev.azure.com" {
		t.Errorf("default domain incorrect")
	}
}
