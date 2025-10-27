package config

import (
	"time"

	"github.com/goodieshq/gopostal/pkg/sender"
)

type SendConfig struct {
	Graph                  GraphSenderConfig `yaml:"graph"`
	Sender                 sender.Sender     `yaml:"-"`
	AllowStartWithoutGraph bool              `yaml:"allow_start_without_graph,omitempty"`
	Timeout                time.Duration     `yaml:"timeout"`
	Retries                int               `yaml:"retries"`
	Backoff                time.Duration     `yaml:"backoff"`
}

type GraphSenderConfig struct {
	Mailbox         string `yaml:"mailbox,omitempty"`
	TenantID        string `yaml:"tenant_id"`
	ClientID        string `yaml:"client_id"`
	ClientSecretEnv string `yaml:"client_secret_env"`
	ClientSecret    string `yaml:"-"`
}
