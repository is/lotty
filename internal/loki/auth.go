package loki

import (
	"encoding/base64"
	"fmt"
	"net/http"
)

// Auth handles Basic Authentication for Loki
type Auth struct {
	username string
	password string
}

// NewAuth creates a new Auth instance
func NewAuth(username, password string) *Auth {
	return &Auth{
		username: username,
		password: password,
	}
}

// SetHeader sets the Authorization header on the request
func (a *Auth) SetHeader(req *http.Request) {
	if a.username != "" || a.password != "" {
		credentials := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", a.username, a.password)))
		req.Header.Set("Authorization", fmt.Sprintf("Basic %s", credentials))
	}
}

// Validate checks if the credentials are set
func (a *Auth) Validate() error {
	if a.username == "" && a.password == "" {
		return fmt.Errorf("no credentials provided")
	}
	return nil
}

// GetUsername returns the username
func (a *Auth) GetUsername() string {
	return a.username
}

// GetPassword returns the password
func (a *Auth) GetPassword() string {
	return a.password
}
