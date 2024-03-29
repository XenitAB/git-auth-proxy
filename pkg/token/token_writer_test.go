package token

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/xenitab/git-auth-proxy/pkg/auth"
	"github.com/xenitab/git-auth-proxy/pkg/config"
)

func TestBasic(t *testing.T) {
	cfg := &config.Configuration{
		Organizations: []*config.Organization{
			{
				Provider: "azuredevops",
				Name:     "org",
				Host:     "foo",
				AzureDevOps: config.AzureDevOps{
					Pat: "foo",
				},
				Repositories: []*config.Repository{
					{
						Project:            "proj",
						Name:               "repo",
						Namespaces:         []string{"foo", "bar"},
						SecretNameOverride: "git-auth",
					},
				},
			},
		},
	}
	authz, err := auth.NewAuthorizer(cfg)
	require.NoError(t, err)
	client := fake.NewSimpleClientset()
	tokenWriter := NewTokenWriter(client, authz)
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	go func() {
		err := tokenWriter.Start(ctx)
		if err != nil {
			panic(err)
		}
	}()

	endpoint, err := authz.GetEndpointById("foo-org-proj-repo")
	require.NoError(t, err)
	require.Eventuallyf(t, func() bool {
		secret, err := client.CoreV1().Secrets("foo").Get(ctx, "git-auth", v1.GetOptions{})
		if err != nil {
			return false
		}
		val, ok := secret.StringData["token"]
		if !ok {
			return false
		}
		if val != endpoint.Token {
			return false
		}
		return true
	}, 5*time.Second, 1*time.Second, "secret git-auth not found in namespace foo")

	secret, err := client.CoreV1().Secrets("bar").Get(ctx, "git-auth", v1.GetOptions{})
	require.NoError(t, err)
	secret.StringData["token"] = "stuff"
	_, err = client.CoreV1().Secrets("bar").Update(ctx, secret, v1.UpdateOptions{})
	require.NoError(t, err)
	require.Eventuallyf(t, func() bool {
		secret, err := client.CoreV1().Secrets("bar").Get(ctx, "git-auth", v1.GetOptions{})
		if err != nil {
			return false
		}
		val, ok := secret.StringData["token"]
		if !ok {
			return false
		}
		if val != endpoint.Token {
			return false
		}
		return true
	}, 15*time.Second, 1*time.Second, "secret git-auth does not have correct value in namespace bar")

	err = client.CoreV1().Secrets("bar").Delete(ctx, "git-auth", v1.DeleteOptions{})
	require.NoError(t, err)
	require.Eventuallyf(t, func() bool {
		secret, err := client.CoreV1().Secrets("bar").Get(ctx, "git-auth", v1.GetOptions{})
		if err != nil {
			return false
		}
		val, ok := secret.StringData["token"]
		if !ok {
			return false
		}
		if val != endpoint.Token {
			return false
		}
		return true
	}, 5*time.Second, 1*time.Second, "secret git-auth not found in namespace bar")
}
