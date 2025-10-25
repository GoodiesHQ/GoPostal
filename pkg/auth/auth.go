package auth

import (
	"golang.org/x/crypto/bcrypt"
)

// This file defines the authentication mechanism for the SMTP server.
type Authenticator interface {
	Check(username, password string) bool
}

// Authenticator that uses plaintext credentials
type AuthenticatorPlaintext struct {
	credentials map[string]string
}

func (a *AuthenticatorPlaintext) Check(username, password string) bool {
	if pw, found := a.credentials[username]; found {
		return password == pw
	}
	return false
}

func NewAuthenticatorPlaintext(creds map[string]string) *AuthenticatorPlaintext {
	return &AuthenticatorPlaintext{
		credentials: creds,
	}
}

// Authenticator that uses bcrypt hashed passwords for authentication.
type AuthenticatorHashed struct {
	credentials map[string]string
}

func (a *AuthenticatorHashed) Check(username, password string) bool {
	if pw, found := a.credentials[username]; found {
		err := bcrypt.CompareHashAndPassword([]byte(pw), []byte(password))
		return err == nil
	}
	return false
}

func NewAuthenticatorHashed(creds map[string]string) *AuthenticatorPlaintext {
	return &AuthenticatorPlaintext{
		credentials: creds,
	}
}

// Authenticator that allows any username/password combination (for testing purposes only).
type AuthenticatorAlwaysAllow struct{}

func (a *AuthenticatorAlwaysAllow) Check(username, password string) bool {
	return true
}

func NewAuthenticatorAlwaysAllow() *AuthenticatorAlwaysAllow {
	return &AuthenticatorAlwaysAllow{}
}
