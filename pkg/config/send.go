package config

type SendConfig struct {
	TenantID     string `yaml:"tenant_id"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RunIfInvalid bool   `yaml:"run_if_invalid,omitempty"`
}
