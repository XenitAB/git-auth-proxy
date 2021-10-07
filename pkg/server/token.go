package server

import (
	b64 "encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

const (
	headerKey = "Authorization"
	basicKey  = "Basic "
	bearerKey = "Bearer "
)

func getTokenFromRequest(req *http.Request) (string, error) {
	headerValue := req.Header.Get(headerKey)
	if headerValue == "" {
		return "", fmt.Errorf("Header %s not found in request", headerKey)
	}
	encodedToken, isBearer, err := extractEncodedToken(headerValue)
	if err != nil {
		return "", err
	}
	if isBearer {
		return encodedToken, nil
	}
	decodedToken, err := b64.URLEncoding.DecodeString(encodedToken)
	if err != nil {
		return "", err
	}
	comps := strings.Split(string(decodedToken), ":")
	if len(comps) != 2 {
		return "", fmt.Errorf("decoded token does not contain enough components")
	}
	if comps[1] != "" {
		return comps[1], nil
	}
	if comps[0] != "" {
		return comps[0], nil
	}
	return "", fmt.Errorf("username component and password component cannot be empty")
}

func extractEncodedToken(value string) (string, bool, error) {
	if strings.HasPrefix(value, basicKey) {
		return strings.TrimPrefix(value, basicKey), false, nil
	}
	if strings.HasPrefix(value, bearerKey) {
		return strings.TrimPrefix(value, bearerKey), true, nil
	}
	return "", false, fmt.Errorf("Missing either %s or %s", basicKey, bearerKey)
}
