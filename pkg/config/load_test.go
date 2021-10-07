package config

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

const path = "config.json"

const validJson = `
{
	"organizations": [
		{
      "provider": "azuredevops",
			"host": "dev.azure.com",
			"name": "xenitab",
			"azuredevops": {
        "pat": "foobar"
      },
			"repositories": [
				{
					"name": "gitops-deployment",
					"project": "Lab",
					"namespaces": ["foo"]
				}
			]
		}
	]
}
`

const invalidJson = `
{
  "host": "dev.azure.com",
	}}
}
`

const missingPatJson = `
{
	"organizations": [
		{
      "provider": "azuredevops",
			"host": "dev.azure.com",
			"name": "xenitab",
			"repositories": [
				{
					"name": "gitops-deployment",
					"project": "Lab"
				}
			]
		}
	]
}
`

const missingRepositoriesJson = `
{
	"organizations": [
		{
      "provider": "azuredevops",
			"host": "dev.azure.com",
			"name": "xenitab",
		}
	]
}
`

func fsWithContent(path string, content string) (afero.Fs, error) {
	fs := afero.NewMemMapFs()
	file, err := fs.Create(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	_, err = file.WriteString(content)
	if err != nil {
		return nil, err
	}
	return fs, nil
}

func TestValidJson(t *testing.T) {
	fs, err := fsWithContent(path, validJson)
	require.NoError(t, err)
	cfg, err := LoadConfiguration(fs, path)
	require.NoError(t, err)
	require.NotEmpty(t, cfg.Organizations)
	require.Equal(t, "dev.azure.com", cfg.Organizations[0].Host)
	require.Equal(t, "https", cfg.Organizations[0].Scheme)
	require.Equal(t, "xenitab", cfg.Organizations[0].Name)
	require.Equal(t, "foobar", cfg.Organizations[0].AzureDevOps.Pat)
	require.NotEmpty(t, cfg.Organizations[0].Repositories)
	require.Equal(t, "gitops-deployment", cfg.Organizations[0].Repositories[0].Name)
	require.Equal(t, "Lab", cfg.Organizations[0].Repositories[0].Project)
}

func TestInvalidJson(t *testing.T) {
	fs, err := fsWithContent(path, invalidJson)
	require.NoError(t, err)
	_, err = LoadConfiguration(fs, path)
	require.Error(t, err)
}

func TestMissingPat(t *testing.T) {
	fs, err := fsWithContent(path, missingPatJson)
	require.NoError(t, err)
	_, err = LoadConfiguration(fs, path)
	require.Error(t, err)
}

func TestMissingRepositories(t *testing.T) {
	fs, err := fsWithContent(path, missingRepositoriesJson)
	require.NoError(t, err)
	_, err = LoadConfiguration(fs, path)
	require.Error(t, err)
}
