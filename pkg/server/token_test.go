package server

import (
	b64 "encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		username string
		password string
		expected string
	}{
		{
			name:     "basic only username",
			key:      basicKey,
			username: "foo",
			password: "",
			expected: "foo",
		},
		{
			name:     "basic only password",
			key:      basicKey,
			username: "",
			password: "bar",
			expected: "bar",
		},
		{
			name:     "basic username and password",
			key:      basicKey,
			username: "foo",
			password: "bar",
			expected: "bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{Header: http.Header{}}
			combo := fmt.Sprintf("%s:%s", tt.username, tt.password)
			headerValue := fmt.Sprintf("%s%s", tt.key, b64.URLEncoding.EncodeToString([]byte(combo)))
			req.Header.Set(headerKey, headerValue)
			token, err := getTokenFromRequest(req)
			require.NoError(t, err)
			require.Equal(t, tt.expected, token)
		})
	}
}

func TestBearer(t *testing.T) {
	req := &http.Request{Header: http.Header{}}
	headerValue := fmt.Sprintf("%s%s", bearerKey, "token")
	req.Header.Set(headerKey, headerValue)
	token, err := getTokenFromRequest(req)
	require.NoError(t, err)
	require.Equal(t, "token", token)
}
