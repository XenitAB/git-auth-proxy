package auth

import (
	"testing"

	"github.com/xenitab/azdo-proxy/pkg/config"
)

func auth() *Authorization {
	config := config.Configuration{
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
			{
				Project: "proj%20space",
				Name:    "repo%20space",
				Token:   "whitespace",
			},
		},
	}

	auth, err := GenerateAuthorization(config)
	if err != nil {
		panic(err)
	}

	return auth
}

func TestPermitted(t *testing.T) {
	auth := auth()
	token := "token"
	path := "/org/proj/_git/repo"

	err := IsPermitted(auth, path, token)
	if err != nil {
		t.Errorf("Token should be permitted: %v", err)
	}
}

func TestPermittedExtraPath(t *testing.T) {
	auth := auth()
	token := "token"
	path := "/org/proj/_git/repo/foobar/foobar"

	err := IsPermitted(auth, path, token)
	if err != nil {
		t.Errorf("Token should be permitted: %v", err)
	}
}

func TestWrongOrg(t *testing.T) {
	auth := auth()
	token := "token"
	path := "/org1/proj/_git/repo"

	err := IsPermitted(auth, path, token)
	if err == nil {
		t.Error("Token should not be permitted")
	}
}

func TestToShortPath(t *testing.T) {
	auth := auth()
	token := "token"
	path := "/foobar/foobar/foobar"

	err := IsPermitted(auth, path, token)
	if err == nil {
		t.Error("Token should not be permitted")
	}
}

func TestMisssingProject(t *testing.T) {
	auth := auth()
	token := "token"
	path := "/org/proj1/_git/repo"

	err := IsPermitted(auth, path, token)
	if err == nil {
		t.Error("Token should not be permitted")
	}
}

func TestMisssingRepo(t *testing.T) {
	auth := auth()
	token := "token"
	path := "/org/proj/_git/repo123"

	err := IsPermitted(auth, path, token)
	if err == nil {
		t.Error("Token should not be permitted")
	}
}

func TestInvalidToken(t *testing.T) {
	auth := auth()
	token := "token1"
	path := "/org/proj/_git/repo"

	err := IsPermitted(auth, path, token)
	if err == nil {
		t.Error("Token should not be permitted")
	}
}

func TestWhitespace(t *testing.T) {
	auth := auth()
	token := "whitespace"
	path := "/org/proj%20space/_git/repo%20space"

	err := IsPermitted(auth, path, token)
	if err != nil {
		t.Errorf("Token should be permitted: %v", err)
	}
}

func TestBaseApi(t *testing.T) {
	auth := auth()
	token := "foobar"
	path := "/org/_apis"

	err := IsPermitted(auth, path, token)
	if err != nil {
		t.Errorf("Token should be permitted: %v", err)
	}
}

func TestApiPath(t *testing.T) {
	auth := auth()
	token := "token"
	path := "/org/proj/_apis/git/repositories/repo/commits"

	err := IsPermitted(auth, path, token)
	if err != nil {
		t.Errorf("Token should be permitted: %v", err)
	}
}
