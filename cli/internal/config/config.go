package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	RelayURL   string   `json:"relay_url"`
	Token      string   `json:"token"`
	PublicBase string   `json:"public_base,omitempty"`
	Tunnels    []Tunnel `json:"tunnels"`
}

type Tunnel struct {
	Name         string   `json:"name"`
	Protocol     string   `json:"protocol"`
	Subdomain    string   `json:"subdomain,omitempty"`
	Allowlist    []string `json:"allowlist,omitempty"`
	ExternalPort int      `json:"external_port,omitempty"`
	LocalURL     string   `json:"local_url,omitempty"`
	LocalHost    string   `json:"local_host,omitempty"`
	LocalPort    int      `json:"local_port,omitempty"`
}

func Load(path string) (Config, error) {
	var cfg Config
	if strings.TrimSpace(path) == "" {
		return cfg, errors.New("config path required")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(contents, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "portopener.json"
	}
	return filepath.Join(home, ".portopener", "config.json")
}

func Save(path string, cfg Config) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("config path required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o600)
}

func (c *Config) Validate() error {
	if strings.TrimSpace(c.RelayURL) == "" {
		return fmt.Errorf("relay_url required")
	}
	if len(c.Tunnels) == 0 {
		return fmt.Errorf("at least one tunnel required")
	}
	for idx, tunnel := range c.Tunnels {
		proto := strings.ToLower(strings.TrimSpace(tunnel.Protocol))
		if proto == "" {
			return fmt.Errorf("tunnels[%d].protocol required", idx)
		}
		switch proto {
		case "http":
			if strings.TrimSpace(tunnel.Subdomain) == "" {
				return fmt.Errorf("tunnels[%d].subdomain required", idx)
			}
			if strings.TrimSpace(tunnel.LocalURL) == "" {
				return fmt.Errorf("tunnels[%d].local_url required", idx)
			}
		case "tcp", "udp":
			if tunnel.ExternalPort == 0 {
				return fmt.Errorf("tunnels[%d].external_port required", idx)
			}
			if strings.TrimSpace(tunnel.LocalHost) == "" {
				return fmt.Errorf("tunnels[%d].local_host required", idx)
			}
			if tunnel.LocalPort == 0 {
				return fmt.Errorf("tunnels[%d].local_port required", idx)
			}
		default:
			return fmt.Errorf("tunnels[%d].protocol invalid", idx)
		}
	}
	return nil
}
