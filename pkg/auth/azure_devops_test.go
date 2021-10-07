package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/xenitab/azdo-proxy/pkg/config"
)

func getAzureDevOpsAuthorizer() *Authorizer {
	cfg := &config.Configuration{
		Organizations: []*config.Organization{
			{
				Provider: config.AzureDevOpsProviderType,
				Host:     "foo",
				Name:     "org",
				Repositories: []*config.Repository{
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
	auth, err := NewAuthorizer(cfg)
	if err != nil {
		panic(err)
	}
	return auth
}

func TestAzureDevOpsPermitted(t *testing.T) {
	authz := getAzureDevOpsAuthorizer()
	endpoint, err := authz.GetEndpointById("foo-org-proj-repo")
	require.NoError(t, err)
	path := "/org/proj/_git/repo"
	err = authz.IsPermitted(path, endpoint.Token)
	require.NoError(t, err, "token should be permitted")
}

func TestAzureDevOpsPermittedExtraPath(t *testing.T) {
	authz := getAzureDevOpsAuthorizer()
	endpoint, err := authz.GetEndpointById("foo-org-proj-repo")
	require.NoError(t, err)
	path := "/org/proj/_git/repo/foobar/foobar"
	err = authz.IsPermitted(path, endpoint.Token)
	require.NoError(t, err, "token should be permitted")
}

func TestAzureDevOpsWrongOrg(t *testing.T) {
	authz := getAzureDevOpsAuthorizer()
	endpoint, err := authz.GetEndpointById("foo-org-proj-repo")
	require.NoError(t, err)
	path := "/org1/proj/_git/repo"
	err = authz.IsPermitted(path, endpoint.Token)
	require.Error(t, err, "token should not be permitted")
}

func TestAzureDevOpsToShortPath(t *testing.T) {
	authz := getAzureDevOpsAuthorizer()
	endpoint, err := authz.GetEndpointById("foo-org-foobar-foobar")
	require.NoError(t, err)
	path := "/foobar/foobar/foobar"
	err = authz.IsPermitted(path, endpoint.Token)
	require.Error(t, err, "token should not be permitted")
}

func TestAzureDevOpsWrongProject(t *testing.T) {
	authz := getAzureDevOpsAuthorizer()
	endpoint, err := authz.GetEndpointById("foo-org-proj-repo")
	require.NoError(t, err)
	path := "/org/proj1/_git/repo"
	err = authz.IsPermitted(path, endpoint.Token)
	require.Error(t, err, "token should not be permitted")
}

func TestAzureDevOpsWrongRepo(t *testing.T) {
	authz := getAzureDevOpsAuthorizer()
	endpoint, err := authz.GetEndpointById("foo-org-proj-repo")
	require.NoError(t, err)
	path := "/org/proj/_git/repo123"
	err = authz.IsPermitted(path, endpoint.Token)
	require.Error(t, err, "token should not be permitted")
}

func TestAzureDevOpsInvalidToken(t *testing.T) {
	authz := getAzureDevOpsAuthorizer()
	token := "token1"
	path := "/org/proj/_git/repo"
	err := authz.IsPermitted(path, token)
	require.Error(t, err, "token should not be permitted")
}

func TestAzureDevOpsWhitespace(t *testing.T) {
	authz := getAzureDevOpsAuthorizer()
	endpoint, err := authz.GetEndpointById("foo-org-proj%20space-repo%20space")
	require.NoError(t, err)
	path := "/org/proj%20space/_git/repo%20space"
	err = authz.IsPermitted(path, endpoint.Token)
	require.NoError(t, err, "token should be permitted")
}

func TestAzureDevOpsBaseApi(t *testing.T) {
	authz := getAzureDevOpsAuthorizer()
	endpoint, err := authz.GetEndpointById("foo-org-proj-repo")
	require.NoError(t, err)
	path := "/org/_apis"
	err = authz.IsPermitted(path, endpoint.Token)
	require.NoError(t, err, "token should be permitted")
}

func TestAzureDevOpsApiPath(t *testing.T) {
	authz := getAzureDevOpsAuthorizer()
	endpoint, err := authz.GetEndpointById("foo-org-proj-repo")
	require.NoError(t, err)
	path := "/org/proj/_apis/git/repositories/repo/commits"
	err = authz.IsPermitted(path, endpoint.Token)
	require.NoError(t, err, "token should be permitted")
}
