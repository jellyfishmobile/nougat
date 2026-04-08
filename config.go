package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileConfig is nougat.config.json (secrets on disk — restrict file permissions).
type FileConfig struct {
	Auth    *ConfigAuth  `json:"auth,omitempty"`
	APIKeys []APIKeySpec `json:"api_keys,omitempty"`
}

type ConfigAuth struct {
	// RequireAPIKey forces Bearer auth on /v1/* and /dashboard/api/* even with no keys in file.
	RequireAPIKey bool `json:"require_api_key,omitempty"`
}

type APIKeySpec struct {
	ID        string `json:"id"`
	Secret    string `json:"secret"`
	Label     string `json:"label,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// GenerateAPIKeySecret returns a new sk-nougat-… secret suitable for Bearer auth.
func GenerateAPIKeySecret() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "sk-nougat-" + hex.EncodeToString(b[:]), nil
}

// NewAPIKeySpec creates a labeled API key row with unique id and fresh secret.
func NewAPIKeySpec(label string) (APIKeySpec, error) {
	secret, err := GenerateAPIKeySecret()
	if err != nil {
		return APIKeySpec{}, err
	}
	var idb [8]byte
	if _, err := rand.Read(idb[:]); err != nil {
		return APIKeySpec{}, err
	}
	id := "key_" + hex.EncodeToString(idb[:])
	return APIKeySpec{
		ID:        id,
		Secret:    secret,
		Label:     strings.TrimSpace(label),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func loadConfig(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileConfig{}, nil
		}
		return nil, err
	}
	var c FileConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return &c, nil
}

func saveConfig(path string, c *FileConfig) error {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func defaultConfigPath(wd string) string {
	if p := strings.TrimSpace(os.Getenv("NOUGAT_CONFIG")); p != "" {
		return p
	}
	return filepath.Join(wd, "nougat.config.json")
}

// mergeEnvAPIKey adds NOUGAT_API_KEY as key_env when set and not already present.
func (c *FileConfig) mergeEnvAPIKey() {
	env := strings.TrimSpace(os.Getenv("NOUGAT_API_KEY"))
	if env == "" {
		return
	}
	for _, k := range c.APIKeys {
		if k.Secret == env {
			return
		}
	}
	c.APIKeys = append(c.APIKeys, APIKeySpec{
		ID:        "key_env",
		Secret:    env,
		Label:     "NOUGAT_API_KEY",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func (c *FileConfig) authRequiredForAPI() bool {
	if c == nil {
		return strings.TrimSpace(os.Getenv("NOUGAT_API_KEY")) != ""
	}
	if c.Auth != nil && c.Auth.RequireAPIKey {
		return true
	}
	if len(c.APIKeys) > 0 {
		return true
	}
	return strings.TrimSpace(os.Getenv("NOUGAT_API_KEY")) != ""
}

func (c *FileConfig) lookupSecret(secret string) (id, label string, ok bool) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", "", false
	}
	for _, k := range c.APIKeys {
		if k.Secret == secret {
			return k.ID, k.Label, true
		}
	}
	return "", "", false
}

// runGenAPIKey creates a key, appends to config file, prints secret, exits path from main.
func runGenAPIKey(configPath, label string) error {
	c, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	c.mergeEnvAPIKey()
	row, err := NewAPIKeySpec(label)
	if err != nil {
		return err
	}
	c.APIKeys = append(c.APIKeys, row)
	if err := saveConfig(configPath, c); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Added API key %s (%s)\n", row.ID, row.Label)
	fmt.Fprintf(os.Stdout, "Secret (save it — not shown again):\n%s\n", row.Secret)
	return nil
}
