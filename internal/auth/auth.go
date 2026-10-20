package auth

import (
	"encoding/base64"
	"net/http"
	"strings"
)

func IsAuthorized(r *http.Request, username, password string) bool {
	if username == "" || password == "" {
		return false
	}

	header := r.Header.Get("Proxy-Authorization")
	if header == "" {
		header = r.Header.Get("Authorization")
	}
	if !strings.HasPrefix(header, "Basic ") {
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(header, "Basic "))
	if err != nil {
		return false
	}
	user, pass, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return false
	}
	return user == username && pass == password
}
