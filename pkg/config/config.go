package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/goodieshq/gopostal/pkg/auth"
	"gopkg.in/yaml.v3"
)

// Listener service can either listen in plaintext or explicit/implicit TLS modes
type ListenerType string

const (
	ListenerSMTP     ListenerType = "smtp"     // plaintext (optionally upgradeable if you enable STARTTLS here)
	ListenerSMTPS    ListenerType = "smtps"    // implicit TLS (465-style)
	ListenerSTARTTLS ListenerType = "starttls" // explicit TLS (587-style)
)

type AuthMode string

const (
	AuthDisabled  AuthMode = "disabled"  // no authentication required (not recommended for production)
	AuthAnonymous AuthMode = "anonymous" // allow AUTH ANONYMOUS (rarely desirable)
	AuthPlain     AuthMode = "plain"     // username/password against provided users
	AuthPlainAny  AuthMode = "plain-any" // accepts any username/password (for testing)
)

type Config struct {
	Recv RecvConfig `yaml:"recv"`
	Send SendConfig `yaml:"send"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return LoadConfigBytes(data)
}

func LoadConfigBytes(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	// Validate SendConfig
	if len(c.Recv.Listeners) == 0 {
		return errors.New("recv.listeners: at least one listener must be defined")
	}

	seenNames := make(map[string]int)
	seenPorts := make(map[uint16]string)

	for i, listener := range c.Recv.Listeners {
		prefix := fmt.Sprintf("recv.listeners[%d]: ", i)
		// validate name is valid and unique
		if listener.Name == "" {
			return errors.New(prefix + "name: must be defined")
		}
		if other, exists := seenNames[listener.Name]; exists {
			return fmt.Errorf(prefix+"name: duplicate listener name '%s' (used by recv.listeners[%d])", listener.Name, other)
		}
		seenNames[listener.Name] = i

		// validte port is valid and unique
		if listener.Port == 0 {
			return fmt.Errorf(prefix+"port: must be a valid TCP port (1-65535), got %d", listener.Port)
		}
		if other, exists := seenPorts[listener.Port]; exists {
			return fmt.Errorf(prefix+"port: duplicate port %d used by '%s'", listener.Port, other)
		}
		seenPorts[listener.Port] = listener.Name

		// validate listener type and TLS config
		switch listener.Type {
		case ListenerSMTP:
			// no TLS config required
		case ListenerSMTPS, ListenerSTARTTLS:
			if listener.TLS == nil || listener.TLS.CertFile == "" || listener.TLS.KeyFile == "" {
				return fmt.Errorf(prefix+"tls: TLS configuration must be provided for listener type '%s'", listener.Type)
			}
		default:
			return fmt.Errorf(prefix+"type: invalid listener type '%s', must be one of: 'smtp', 'smtps', or 'starttls'", listener.Type)
		}
	}

	// Validate Authentication mode and users if required
	switch c.Recv.Auth.Mode {
	case AuthDisabled, AuthAnonymous, AuthPlainAny:
		c.Recv.Authenticator = auth.NewAuthenticatorAlwaysAllow()
		// valid modes which do not require authentication
	case AuthPlain:
		creds := make(map[string]string, len(c.Recv.Auth.Credentials))
		if len(c.Recv.Auth.Credentials) == 0 {
			return errors.New("recv.auth.credentials: at least one credential must be defined for 'plain' authentication mode")
		}
		for i, cred := range c.Recv.Auth.Credentials {
			if cred.Username == "" || cred.Password == "" {
				return fmt.Errorf("recv.auth.credentials[%d]: username and password must be defined", i)
			}
			creds[cred.Username] = cred.Password
		}
		c.Recv.Authenticator = auth.NewAuthenticatorPlaintext(creds)
	default:
		return fmt.Errorf("recv.auth.mode: invalid authentication mode '%s', must be one of: 'disabled', 'anonymous', 'plain', or 'plain-any'", c.Recv.Auth.Mode)
	}

	// Validate Mail Policy (senders and recipients)
	if len(c.Recv.ValidFrom.Addresses) > 0 {
		for i, addr := range c.Recv.ValidFrom.Addresses {
			if addr == "" {
				return fmt.Errorf("recv.valid_from.addresses[%d]: address must be defined", i)
			}
			if !isValidEmail(addr) {
				return fmt.Errorf("recv.valid_from.addresses[%d]: invalid email address '%s'", i, addr)
			}
		}
	}
	if len(c.Recv.ValidFrom.Domains) > 0 {
		for i, dom := range c.Recv.ValidFrom.Domains {
			if dom == "" {
				return fmt.Errorf("recv.valid_from.domains[%d]: domain must be defined", i)
			}
			if !isValidDomain(dom) {
				return fmt.Errorf("recv.valid_from.domains[%d]: invalid domain '%s'", i, dom)
			}
		}
	}
	if len(c.Recv.ValidTo.Addresses) > 0 {
		for i, addr := range c.Recv.ValidTo.Addresses {
			if addr == "" {
				return fmt.Errorf("recv.valid_to.addresses[%d]: address must be defined", i)
			}
			if !isValidEmail(addr) {
				return fmt.Errorf("recv.valid_to.addresses[%d]: invalid email address '%s'", i, addr)
			}
		}
	}
	if len(c.Recv.ValidTo.Domains) > 0 {
		for i, dom := range c.Recv.ValidTo.Domains {
			if dom == "" {
				return fmt.Errorf("recv.valid_to.domains[%d]: domain must be defined", i)
			}
			if !isValidDomain(dom) {
				return fmt.Errorf("recv.valid_to.domains[%d]: invalid domain '%s'", i, dom)
			}
		}
	}

	// Validate AllowedIPs
	if len(c.Recv.AllowedIPs) > 0 {
		for i, ip := range c.Recv.AllowedIPs {
			if ip == "" {
				return fmt.Errorf("recv.allowed_ips[%d]: IP address or CIDR must be defined", i)
			}
			net, err := ParseNet(ip)
			if err != nil {
				return fmt.Errorf("recv.allowed_ips[%d]: invalid IP address or CIDR '%s': %v", i, ip, err)
			}
			c.Recv.AllowedNets = append(c.Recv.AllowedNets, *net)
		}
	} else {
		c.Recv.AllowedNets = []net.IPNet{
			{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)},  // allow all IPv4
			{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)}, // allow all IPv6
		}
	}

	// Validate Limits
	if c.Recv.Limits.MaxSize < 0 {
		return fmt.Errorf("recv.limits.max_size: must be a non-negative integer, got %d", c.Recv.Limits.MaxSize)
	}
	if c.Recv.Limits.MaxSize == 0 {
		c.Recv.Limits.MaxSize = 25 * 1024 * 1024 // default to 25 MiB
	}

	if c.Recv.Limits.MaxRecipients < 0 {
		return fmt.Errorf("recv.limits.max_recipients: must be a non-negative integer, got %d", c.Recv.Limits.MaxRecipients)
	}
	if c.Recv.Limits.MaxRecipients == 0 {
		c.Recv.Limits.MaxRecipients = 100 // default to 100 recipients
	}

	if c.Recv.Limits.Timeout < 0 {
		return fmt.Errorf("recv.limits.read_timeout: must be a non-negative duration, got %s", c.Recv.Limits.Timeout.String())
	}
	if c.Recv.Limits.Timeout == 0 {
		c.Recv.Limits.Timeout = 10 * time.Second // default to 10 seconds
	}

	return nil
}
