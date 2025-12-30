package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// TestTarget represents a single test target URL
type TestTarget struct {
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	Method  string `yaml:"method"`
	Timeout string `yaml:"timeout"`
}

// ProxyConfig represents proxy server configuration
type ProxyConfig struct {
	Socks5   string `yaml:"socks5"`
	Name     string `yaml:"name"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// Scenario represents a test scenario
type Scenario struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"` // "single" or "concurrent"
	Count       int    `yaml:"count"`
	Concurrency int    `yaml:"concurrency"`
	Enabled     bool   `yaml:"enabled"`
}

// Settings represents general settings
type Settings struct {
	RequestTimeout  string `yaml:"request_timeout"`
	MaxRetries      int    `yaml:"max_retries"`
	RequestInterval string `yaml:"request_interval"`
	OutputDir       string `yaml:"output_dir"`
	Verbose         bool   `yaml:"verbose"`
}

// Config represents the entire configuration
type Config struct {
	Targets   []TestTarget           `yaml:"targets"`
	Proxies   map[string]ProxyConfig `yaml:"proxies"`
	Scenarios []Scenario             `yaml:"scenarios"`
	Settings  Settings               `yaml:"settings"`
}

// LoadConfig loads configuration from YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if len(c.Targets) == 0 {
		return fmt.Errorf("no targets defined")
	}

	if len(c.Proxies) == 0 {
		return fmt.Errorf("no proxies defined")
	}

	// Validate timeout parsing
	if _, err := time.ParseDuration(c.Settings.RequestTimeout); err != nil {
		return fmt.Errorf("invalid request_timeout: %w", err)
	}

	if _, err := time.ParseDuration(c.Settings.RequestInterval); err != nil {
		return fmt.Errorf("invalid request_interval: %w", err)
	}

	return nil
}

// GetEnabledScenarios returns only enabled scenarios
func (c *Config) GetEnabledScenarios() []Scenario {
	var enabled []Scenario
	for _, scenario := range c.Scenarios {
		if scenario.Enabled {
			enabled = append(enabled, scenario)
		}
	}
	return enabled
}
