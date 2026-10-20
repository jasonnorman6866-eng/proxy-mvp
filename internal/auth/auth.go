package auth

import (
	"encoding/base64"
	"net/http"
	"strings"
)

func IsAuthorized(r *http.Request, enabled bool, username, password string) bool {
	if !enabled {
		return true
	}
	if username == "" || password == "" {
		return false
	}

	header := r.Header.Get("Proxy-Authorization")
	if header == "" {
		header = r.Header.Get("Authorization")
	}
	if header == "" {
		return false
	}
	if !strings.HasPrefix(header, "Basic ") {
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(header, "Basic "))
	if err != nil {
		return false
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return false
	}
	return parts[0] == username && parts[1] == password
}
