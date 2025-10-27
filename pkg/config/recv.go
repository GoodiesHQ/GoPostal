package config

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/goodieshq/gopostal/pkg/auth"
)

type RecvConfig struct {
	RecvGlobalConfig `yaml:",inline"`
	Listeners        []ListenerConfig `yaml:"listeners"`
}

type RecvGlobalConfig struct {
	Domain        string             `yaml:"domain,omitempty"`
	AllowedIPs    []string           `yaml:"allowed_ips"`
	AllowedNets   []net.IPNet        `yaml:"-"`
	Auth          AuthRule           `yaml:"auth"`
	Authenticator auth.Authenticator `yaml:"-"`
	ValidFrom     MailPolicy         `yaml:"valid_from"`
	ValidTo       MailPolicy         `yaml:"valid_to"`
	Limits        RecvLimits         `yaml:"limits,omitempty"`
}

type ListenerConfig struct {
	Name        string       `yaml:"name"`
	Port        uint16       `yaml:"port"`
	Type        ListenerType `yaml:"type"`
	RequireAuth bool         `yaml:"require_auth"`
	TLS         *TLSConfig   `yaml:"tls,omitempty"`
	TLSConfig   *tls.Config  `yaml:"-"`
}

type TLSConfig struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type AuthRule struct {
	Mode        AuthMode     `yaml:"mode"`
	Credentials []Credential `yaml:"credentials,omitempty"`
}

// Represents a username and a BCrypt hashed password for authentication.
type Credential struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type MailPolicy struct {
	Addresses []string `yaml:"addresses,omitempty"`
	Domains   []string `yaml:"domains,omitempty"`
}

type RecvLimits struct {
	MaxSize       int           `yaml:"max_size,omitempty"`       // Maximum message size in bytes
	MaxRecipients int           `yaml:"max_recipients,omitempty"` // Maximum number of recipients per message
	Timeout       time.Duration `yaml:"timeout,omitempty"`        // Read timeout duration (e.g., "10s")
}
