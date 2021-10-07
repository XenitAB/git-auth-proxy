package auth

import (
	"crypto/rand"
	"encoding/base64"
)

const tokenLenght = 64

func randomSecureToken() (string, error) {
	b := make([]byte, tokenLenght)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	randStr := base64.URLEncoding.EncodeToString(b)
	return randStr, nil
}
