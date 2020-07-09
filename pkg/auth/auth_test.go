package auth

import (
	"testing"

	"github.com/xenitab/azdo-proxy/pkg/config"
)

func baseConf() config.Configuration {
	return config.Configuration{
		Domain:       "",
		Pat:          "",
		Organization: "org",
		Repositories: []config.Repository{
			{
				Project: "proj",
				Name:    "repo",
				Token:   "token",
			},
			{
				Project: "foobar",
				Name:    "foobar",
				Token:   "foobar",
			},
		},
	}
}

func TestPermitted(t *testing.T) {
	c := baseConf()
	token := "token"
	path := "/org/proj/_git/repo"

	err := IsPermitted(&c, path, token)
	if err != nil {
		t.Errorf("Token should be permitted: %v", err)
	}
}

func TestWrongOrg(t *testing.T) {
	token := "token"
	path := "/org1/proj/_git/repo"
	c := baseConf()

	err := IsPermitted(&c, path, token)
	if err == nil {
		t.Error("Token should not be permitted")
	}
}

func TestToShortPath(t *testing.T) {
	token := "token"
	path := "/foobar/foobar/foobar"
	c := baseConf()

	err := IsPermitted(&c, path, token)
	if err == nil {
		t.Error("Token should not be permitted")
	}
}

func TestMisssingProject(t *testing.T) {
	token := "token"
	path := "/org/proj1/_git/repo"
	c := baseConf()

	err := IsPermitted(&c, path, token)
	if err == nil {
		t.Error("Token should not be permitted")
	}
}

func TestMisssingRepo(t *testing.T) {
	token := "token"
	path := "/org/proj/_git/repo1"
	c := baseConf()

	err := IsPermitted(&c, path, token)
	if err == nil {
		t.Error("Token should not be permitted")
	}
}

func TestInvalidToken(t *testing.T) {
	token := "token1"
	path := "/org/proj/_git/repo"
	c := baseConf()

	err := IsPermitted(&c, path, token)
	if err == nil {
		t.Error("Token should not be permitted")
	}
}
