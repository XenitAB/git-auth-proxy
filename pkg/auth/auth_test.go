package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xenitab/azdo-proxy/pkg/config"
)

func auth() Authorization {
	config := config.Configuration{
		Organizations: []config.Organization{
			{
				Domain: "",
				Pat:    "",
				Name:   "org",
				Repositories: []config.Repository{
					{
						Project: "proj",
						Name:    "repo",
					},
					{
						Project: "foobar",
						Name:    "foobar",
					},
					{
						Project: "proj%20space",
						Name:    "repo%20space",
					},
				},
			},
		},
	}
	auth, err := NewAuthorization(config)
	if err != nil {
		panic(err)
	}
	return auth
}

func TestPermitted(t *testing.T) {
	authz := auth()
	endpoint, err := authz.LookupEndpoint("org", "proj", "repo")
	require.NoError(t, err)
	path := "/org/proj/_git/repo"
	err = authz.IsPermitted(path, endpoint.Token)
	require.NoError(t, err, "token should be permitted")
}

func TestPermittedExtraPath(t *testing.T) {
	authz := auth()
	endpoint, err := authz.LookupEndpoint("org", "proj", "repo")
	require.NoError(t, err)
	path := "/org/proj/_git/repo/foobar/foobar"
	err = authz.IsPermitted(path, endpoint.Token)
	require.NoError(t, err, "token should be permitted")
}

func TestWrongOrg(t *testing.T) {
	authz := auth()
	endpoint, err := authz.LookupEndpoint("org", "proj", "repo")
	require.NoError(t, err)
	path := "/org1/proj/_git/repo"
	err = authz.IsPermitted(path, endpoint.Token)
	require.Error(t, err, "token should not be permitted")
}

func TestToShortPath(t *testing.T) {
	authz := auth()
	endpoint, err := authz.LookupEndpoint("org", "foobar", "foobar")
	require.NoError(t, err)
	path := "/foobar/foobar/foobar"
	err = authz.IsPermitted(path, endpoint.Token)
	require.Error(t, err, "token should not be permitted")
}

func TestWrongProject(t *testing.T) {
	authz := auth()
	endpoint, err := authz.LookupEndpoint("org", "proj", "repo")
	require.NoError(t, err)
	path := "/org/proj1/_git/repo"
	err = authz.IsPermitted(path, endpoint.Token)
	require.Error(t, err, "token should not be permitted")
}

func TestWrongRepo(t *testing.T) {
	authz := auth()
	endpoint, err := authz.LookupEndpoint("org", "proj", "repo")
	require.NoError(t, err)
	path := "/org/proj/_git/repo123"
	err = authz.IsPermitted(path, endpoint.Token)
	require.Error(t, err, "token should not be permitted")
}

func TestInvalidToken(t *testing.T) {
	authz := auth()
	token := "token1"
	path := "/org/proj/_git/repo"
	err := authz.IsPermitted(path, token)
	require.Error(t, err, "token should not be permitted")
}

func TestWhitespace(t *testing.T) {
	authz := auth()
	endpoint, err := authz.LookupEndpoint("org", "proj%20space", "repo%20space")
	require.NoError(t, err)
	path := "/org/proj%20space/_git/repo%20space"
	err = authz.IsPermitted(path, endpoint.Token)
	require.NoError(t, err, "token should be permitted")
}

func TestBaseApi(t *testing.T) {
	authz := auth()
	endpoint, err := authz.LookupEndpoint("org", "proj", "repo")
	require.NoError(t, err)
	path := "/org/_apis"
	err = authz.IsPermitted(path, endpoint.Token)
	require.NoError(t, err, "token should be permitted")
}

func TestApiPath(t *testing.T) {
	authz := auth()
	endpoint, err := authz.LookupEndpoint("org", "proj", "repo")
	require.NoError(t, err)
	path := "/org/proj/_apis/git/repositories/repo/commits"
	err = authz.IsPermitted(path, endpoint.Token)
	require.NoError(t, err, "token should be permitted")
}
