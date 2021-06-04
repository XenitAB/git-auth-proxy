package token

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/xenitab/azdo-proxy/pkg/auth"
	"github.com/xenitab/azdo-proxy/pkg/config"
)

func TestBasic(t *testing.T) {
	cfg := &config.Configuration{
		Organizations: []*config.Organization{
			{
				Name:   "org",
				Domain: "foo",
				Pat:    "test",
				Repositories: []*config.Repository{
					{
						Project:            "proj",
						Name:               "repo",
						Namespaces:         []string{"foo", "bar"},
						SecretNameOverride: "azdo",
					},
				},
			},
		},
	}
	authz, err := auth.NewAuthorization(cfg)
	require.NoError(t, err)
	logger := logr.Discard()
	client := fake.NewSimpleClientset()
	tokenWriter := NewTokenWriter(logger, client, authz)
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	go func() {
		err := tokenWriter.Start(ctx)
		if err != nil {
			panic(err)
		}
	}()

	endpoint, err := authz.LookupEndpoint("foo", "org", "proj", "repo")
	require.NoError(t, err)
	require.Eventuallyf(t, func() bool {
		secret, err := client.CoreV1().Secrets("foo").Get(ctx, "azdo", v1.GetOptions{})
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
	}, 5*time.Second, 1*time.Second, "secret azdo not found in namespace foo")

	secret, err := client.CoreV1().Secrets("bar").Get(ctx, "azdo", v1.GetOptions{})
	require.NoError(t, err)
	secret.StringData["token"] = "stuff"
	_, err = client.CoreV1().Secrets("bar").Update(ctx, secret, v1.UpdateOptions{})
	require.NoError(t, err)
	require.Eventuallyf(t, func() bool {
		secret, err := client.CoreV1().Secrets("bar").Get(ctx, "azdo", v1.GetOptions{})
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
	}, 5*time.Second, 1*time.Second, "secret azdo does not have correct value in namespace bar")

	err = client.CoreV1().Secrets("bar").Delete(ctx, "azdo", v1.DeleteOptions{})
	require.NoError(t, err)
	require.Eventuallyf(t, func() bool {
		secret, err := client.CoreV1().Secrets("bar").Get(ctx, "azdo", v1.GetOptions{})
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
	}, 5*time.Second, 1*time.Second, "secret azdo not found in namespace bar")
}
