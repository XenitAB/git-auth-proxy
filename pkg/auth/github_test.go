package auth

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xenitab/git-auth-proxy/pkg/config"
)

type MockGitHubTokenSource struct {
}

func (*MockGitHubTokenSource) Token(ctx context.Context) (string, error) {
	return "foo", nil
}

func getGitHubAuthorizer() *Authorizer {
	cfg := &config.Configuration{
		Organizations: []*config.Organization{
			{
				Provider: config.GitHubProviderType,
				GitHub: config.GitHub{
					AppID:          123,
					InstallationID: 123,
					//nolint:lll //only for testing
					PrivateKey: "LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBMUtXT08wVzNlbDdQQkhBK0VDcFBXREIrWjVxdnJXZDN2MU9QbG81cjFMZ3U5bkRqCkFkVDRwZkR1aE92SzhZMGlaWTB0bGJBV0dpOUpFMTFFeXV1T0RoWTloYTlKaHRYOGx6UDlwL3pOdHRlNGlINkYKQWZSQjFWbk9CZ1JzOGJYUWxvL0RDRXpZdWJUZjgycTJRZ2JJWkp1UlJZOEZ3KzVYS2c2VVRKQzI1RjBmN2M0awpoNWdaS1FiMXpVR3FvQTl5a0M5Z1N0YmVRMys5Mlh2NlhpdjVnNjU4ZGN2djFpY1RPdlE4YXdORWVWd3FhbXEyClJQQ2lwWGluaS9kTHlFTVFWSm45OVFvTVZXa1krZnd2aGhQWXRtbVFrOE13QlNYMFF1dDRJdmNZdkZ5Tk83MEkKV0V4dk92R3JubTVhTjg4QUFZTkpNcHVnV1M1M3ZUTlpCeHF0L1FJREFRQUJBb0lCQUYyd3I5RUhyNFpYL1dnYwpPQXdSU0RJMzg0bWNTdWpnM0k3TXQwZ0RhaGtvS1hEbFhlOXhzVGdUeGxPRVBEOWZDcGVwc3pydmdWMTZGZjFWCks3a29QY2VSSHZ3bXRnT1ZocHZzQ1VlWmg5MldnRFNMWWZqeGNJd2E3RDRVZHhlc0hzSW5oeXZDQi84U1pWV3YKWDZ3SnB3TkUwNlhORlNJMWdld0N6bTVKbUh0V2lxbFZZcjZaYms0ekZnOTVPM21tS2hLd2lUM0ljb0VZK01rcQpWeENDNlFTRUcvc3M0dTBLSkZGVWJPdndJVDdpQ0RIUlh6YURUSVd3dlpRbWVNeEJxdFpWNFZEbEpTS2FIRHBQCnhmVjIxNFRYQkk3QlZGaFpmR0pNd2lBNXdyVjNoUkw0N1JkbURYaUxIRW1jSjRPZmRXV0RvMm5NRjlMRjVvUG4KaFNLTnRDRUNnWUVBOUQyNG45ZTgzeHFKaHlIaGhlV00yTkozRWF3Rk5oTVZDNkNnNjY0ZUZmL25YbG1TN2x4Ygo0L1o0T2w5UWwxUW9JYWNKQjRaYnIrUlpnZ3lGeDRTT0M0MDNZWkFneXJDUURiMVczUkxLSnlEQkZuWXArL0I4CjlSZ0pESVNaNTFpeFYrUk9YWWR3UUJpZzVqTHJNcVFhYndsUUJxSzBaek05SW5OUGtqdmJOeGtDZ1lFQTN1SnUKdWEyTy9aaFpoZ1Jab3V6b0ZMcVJNMTk1d0FHM1pTK1pReUZYZUY0bWRMNk1QS3hob0JBYmh6S3I2MEpPcGcvRApxSVdOYmdRRlNwb24xNkhMdEpad2lzd3RDUUx0OWJkQ2dxU0tCaXJMT2pqSU1QaStSS2FVTDA0QlkxMGtPd215CmEweGkwa0p1WUNYOTNYTzFlckxwNTY4Tm0wMThrNjNUSXBTM1BvVUNnWUFuR0FJSFE4YnRoeGZnVTJIL3hxQm0KekRsVzBNdjh2YzB1a1VWd3Mrd0k1VzhwUVBrdHdnYkxWRlltTWI5Nm1YUGEveHVJNHM2bU5zekU3akF6b1ZvRApLMVZqL21maFNhV2xMVnRNQTRmci8yZ29xajFLSUZKQUFOcmg4QSthWWkzd3ZaQjFsQW81bURlWTRTbVliMy96CnFlL3ZQL2ZVVlBWQ0lHYnFKejZOY1FLQmdRQ21DZDBlcWJMYUxJS1VtZTBFdUtQenZVQ3FHcmdpVjZUOTFrWEEKZ3JnY3pWYXNwYjdtL0N3R0I3bmFMOTl1OVFpT0lUUksrS0x4a0VFNDREcEtJeGdUd2ZhNUQzMkZOdzk2ZXprcgpCZFJrMzhCaDhTY0JoR3lKeSthY2p1bnQwZGRKdStHVW1XVU02Ync4R0ZGVWhmeHVHWmF5cCsvbEFBYU1KWFFpClVOTnAyUUtCZ0FSMGtyaFlSejJJc3gwRDF5UXM3a2dMajdtMWY2YkFxbE5IZ1ZMYTJicy9NQVc3SHpJeWp4dDMKV1dFeWxldmJPTzRCRXFnL1FCMTRwNWFNbVhFbjlHYXg1Z2RhM2YxeURTemR5M2t5My9GU3Q0dzJteVpOaFViWApmdEU4TG01OHc3eEh2MFZBc0tXRk01RFZXZVVyaDJLNFNxczdrODY1R1JKQS9xWnRqMUNrCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==",
				},
				Host: "github.com",
				Name: "org",
				Repositories: []*config.Repository{
					{
						Name: "repo",
					},
					{
						Name: "foobar",
					},
					{
						Name: "repo%20space",
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

func TestGitHubAuthorization(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		allow bool
	}{
		{
			name:  "allow repo",
			path:  "/org/repo",
			allow: true,
		},
		{
			name:  "allow repo",
			path:  "/Org/repO",
			allow: true,
		},
		{
			name:  "allow api",
			path:  "/api/v3/org/repo",
			allow: true,
		},
		{
			name:  "disallow wrong repo",
			path:  "/org/foo",
			allow: false,
		},
		{
			name:  "disallow wront repo in api",
			path:  "/api/v3/org/foo",
			allow: false,
		},
		{
			name:  "disallow wrong org",
			path:  "/foo/repo",
			allow: false,
		},
		{
			name:  "disallow wront org in api",
			path:  "/api/v3/foo/repo",
			allow: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authz := getGitHubAuthorizer()
			endpoint, err := authz.GetEndpointById("github.com-org-repo")
			require.NoError(t, err)
			err = authz.IsPermitted(tt.path, endpoint.Token)

			if tt.allow {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestGithubApiGetAuthorization(t *testing.T) {
	gh := &github{itr: &MockGitHubTokenSource{}}
	authorization, err := gh.getAuthorizationHeader(context.TODO(), "/api/v3/test")
	require.NoError(t, err)
	require.Equal(t, "Bearer foo", authorization)
}

func TestGithubGitGetAuthorization(t *testing.T) {
	gh := &github{itr: &MockGitHubTokenSource{}}
	authorization, err := gh.getAuthorizationHeader(context.TODO(), "/org/repo")
	require.NoError(t, err)
	require.Equal(t, "Basic eC1hY2Nlc3MtdG9rZW46Zm9v", authorization)
}

func TestGetHost(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		path     string
		expected string
	}{
		{
			name:     "api path standard github",
			host:     standardGitHub,
			path:     "/api/v3/test",
			expected: fmt.Sprintf("api.%s", standardGitHub),
		},
		{
			name:     "repo path standard github",
			host:     standardGitHub,
			path:     "/foo/bar",
			expected: standardGitHub,
		},
		{
			name:     "api path enterprise github",
			host:     "example.com",
			path:     "/api/v3/test",
			expected: "example.com",
		},
		{
			name:     "repo path enterprise github",
			host:     "example.com",
			path:     "/foo/bar",
			expected: "example.com",
		},
	}
	gh := &github{itr: &MockGitHubTokenSource{}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Endpoint{
				host: tt.host,
			}
			host := gh.getHost(e, tt.path)
			require.Equal(t, tt.expected, host)
		})
	}
}

func TestGetPath(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		path     string
		expected string
	}{
		{
			name:     "api path standard github",
			host:     standardGitHub,
			path:     "/api/v3/test",
			expected: "/test",
		},
		{
			name:     "repo path standard github",
			host:     standardGitHub,
			path:     "/foo/bar",
			expected: "/foo/bar",
		},
		{
			name:     "api path enterprise github",
			host:     "example.com",
			path:     "/api/v3/test",
			expected: "/api/v3/test",
		},
		{
			name:     "repo path enterprise github",
			host:     "example.com",
			path:     "/foo/bar",
			expected: "/foo/bar",
		},
	}
	gh := &github{itr: &MockGitHubTokenSource{}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Endpoint{
				host: tt.host,
			}
			path := gh.getPath(e, tt.path)
			require.Equal(t, tt.expected, path)
		})
	}
}
