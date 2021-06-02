package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

const testToken = "foobar"

func basicAuthHeader(username, password string) http.Header {
	basicAuth := fmt.Sprintf("%s:%s", username, password)
	b64TestToken := base64.StdEncoding.EncodeToString([]byte(basicAuth))
	tokenData := fmt.Sprintf("Basic %s", b64TestToken)
	return http.Header{
		"Authorization": {tokenData},
	}
}

func TestTokenFromRequestUsername(t *testing.T) {
	r := http.Request{
		Header: basicAuthHeader(testToken, ""),
	}
	token, err := tokenFromRequest(&r)
	require.NoError(t, err)
	require.Equal(t, testToken, token)
}

func TestTokenFromRequestPassword(t *testing.T) {
	r := http.Request{
		Header: basicAuthHeader("", testToken),
	}
	token, err := tokenFromRequest(&r)
	require.NoError(t, err)
	require.Equal(t, testToken, token)
}

func TestTokenFromRequestBoth(t *testing.T) {
	r := http.Request{
		Header: basicAuthHeader(testToken, testToken),
	}
	token, err := tokenFromRequest(&r)
	require.NoError(t, err)
	require.Equal(t, testToken, token)
}

func TestTokenFromRequestNone(t *testing.T) {
	r := http.Request{
		Header: basicAuthHeader("", ""),
	}
	_, err := tokenFromRequest(&r)
	require.Error(t, err)
	require.EqualError(t, err, "token cannot be empty")
}

func TestTokenFromRequestWrongFormat(t *testing.T) {
	r := http.Request{
		Header: http.Header{
			"Authorization": {"foobar"},
		},
	}
	_, err := tokenFromRequest(&r)
	require.Error(t, err)
	require.EqualError(t, err, "basic auth not present")
}
