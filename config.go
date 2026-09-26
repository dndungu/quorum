package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Seat struct {
	Name       string `yaml:"name"`
	BaseURL    string `yaml:"base_url"`
	BaseURLEnv string `yaml:"base_url_env"`
	Model      string `yaml:"model"`
	APIKeyEnv  string `yaml:"api_key_env"`
	Enabled    *bool  `yaml:"enabled,omitempty"`
}

func (s Seat) isEnabled() bool { return s.Enabled == nil || *s.Enabled }

type Config struct {
	Defaults struct {
		BaseURL     string  `yaml:"base_url"`
		BaseURLEnv  string  `yaml:"base_url_env"`
		APIKeyEnv   string  `yaml:"api_key_env"`
		Timeout     string  `yaml:"timeout"`
		Parallel    int     `yaml:"parallel"`
		Temperature float64 `yaml:"temperature"`
	} `yaml:"defaults"`
	Seats []Seat `yaml:"seats"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no config at %s; create it (see README) or pass --config", path)
		}
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	// Normalize: materialize the Enabled pointer so selectSeats sees it.
	for i := range cfg.Seats {
		if cfg.Seats[i].BaseURL == "" && cfg.Seats[i].BaseURLEnv == "" {
			cfg.Seats[i].BaseURL = cfg.Defaults.BaseURL
			cfg.Seats[i].BaseURLEnv = cfg.Defaults.BaseURLEnv
		}
		if cfg.Seats[i].APIKeyEnv == "" {
			cfg.Seats[i].APIKeyEnv = cfg.Defaults.APIKeyEnv
		}
		if cfg.Seats[i].BaseURLEnv != "" {
			baseURL, ok := os.LookupEnv(cfg.Seats[i].BaseURLEnv)
			if !ok || baseURL == "" {
				return nil, fmt.Errorf("seat %q base URL env %s is not set", cfg.Seats[i].Name, cfg.Seats[i].BaseURLEnv)
			}
			cfg.Seats[i].BaseURL = baseURL
		}
		if cfg.Seats[i].Enabled == nil {
			t := true
			cfg.Seats[i].Enabled = &t
		}
		if cfg.Seats[i].Name == "" || cfg.Seats[i].BaseURL == "" || cfg.Seats[i].Model == "" {
			return nil, fmt.Errorf("seat %d missing name/base_url/model", i)
		}
	}
	return &cfg, nil
}
