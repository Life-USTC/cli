// Package config manages CLI configuration and credentials.
//
// Stores config and credentials per-server under the user's config directory
// (~/.config/life-ustc on Linux/Mac).  Credentials are written atomically
// with 0600 permissions.
package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	AppName       = "life-ustc"
	DefaultServer = "https://life-ustc.tiankaima.dev"
)

var (
	configDir     string
	configDirOnce sync.Once
)

// Dir returns the configuration directory, creating it if needed.
func Dir() string {
	configDirOnce.Do(func() {
		if d := os.Getenv("LIFE_USTC_CONFIG_DIR"); d != "" {
			configDir = d
		} else {
			base, err := os.UserConfigDir()
			if err != nil {
				base = filepath.Join(os.Getenv("HOME"), ".config")
			}
			configDir = filepath.Join(base, AppName)
		}
		_ = os.MkdirAll(configDir, 0o755)
	})
	return configDir
}

// serverKey normalises a server URL into a stable cache key.
func serverKey(server string) string {
	u, err := url.Parse(server)
	if err != nil {
		return server
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "https"
	}
	host := u.Hostname()
	if host == "" {
		host = "localhost"
	}
	port := u.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return fmt.Sprintf("%s://%s:%s", scheme, host, port)
}

// atomicWriteJSON writes data as JSON to path atomically.
func atomicWriteJSON(path string, data any, mode os.FileMode) error {
	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, 0o755)

	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := f.Name()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpName)
		return err
	}
	_ = f.Close()

	if err := os.Chmod(tmpName, mode); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// --- Global config ---

type Config struct {
	Server         string   `json:"server,omitempty"`
	SchoolPrograms []string `json:"schoolPrograms,omitempty"`
}

func LoadConfig() (*Config, error) {
	p := filepath.Join(Dir(), "config.json")
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	return atomicWriteJSON(filepath.Join(Dir(), "config.json"), cfg, 0o644)
}

// GetDefaultServer returns the server URL using this precedence:
//  1. LIFE_USTC_SERVER environment variable
//  2. Config file server field
//  3. Built-in DefaultServer
//
// The --server flag overrides all of these at the command level.
func GetDefaultServer() string {
	if server := os.Getenv("LIFE_USTC_SERVER"); server != "" {
		return server
	}
	cfg, err := LoadConfig()
	if err != nil || cfg.Server == "" {
		return DefaultServer
	}
	return cfg.Server
}

func SetDefaultServer(server string) error {
	cfg, _ := LoadConfig()
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.Server = server
	return SaveConfig(cfg)
}

func GetSchoolPrograms() []string {
	cfg, err := LoadConfig()
	if err != nil || cfg == nil {
		return nil
	}
	return append([]string(nil), cfg.SchoolPrograms...)
}

func SetSchoolPrograms(programs []string) error {
	cfg, _ := LoadConfig()
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.SchoolPrograms = append([]string(nil), programs...)
	return SaveConfig(cfg)
}

// --- Credentials (per server) ---

type Credential struct {
	ClientID     string  `json:"client_id"`
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token,omitempty"`
	TokenType    string  `json:"token_type"`
	ExpiresAt    float64 `json:"expires_at"`
	Scope        string  `json:"scope,omitempty"`
	Resource     string  `json:"resource,omitempty"`
}

func credsPath() string {
	return filepath.Join(Dir(), "credentials.json")
}

func loadAllCreds() (map[string]*Credential, error) {
	data, err := os.ReadFile(credsPath())
	if os.IsNotExist(err) {
		return make(map[string]*Credential), nil
	}
	if err != nil {
		return nil, err
	}
	var creds map[string]*Credential
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	if creds == nil {
		creds = make(map[string]*Credential)
	}
	return creds, nil
}

func saveAllCreds(creds map[string]*Credential) error {
	return atomicWriteJSON(credsPath(), creds, 0o600)
}

func LoadCredentials(server string) (*Credential, error) {
	key := serverKey(server)
	all, err := loadAllCreds()
	if err != nil {
		return nil, err
	}
	return all[key], nil
}

func SaveCredentials(server string, cred *Credential) error {
	key := serverKey(server)
	all, err := loadAllCreds()
	if err != nil {
		all = make(map[string]*Credential)
	}
	all[key] = cred
	return saveAllCreds(all)
}

func RemoveCredentials(server string) (bool, error) {
	key := serverKey(server)
	all, err := loadAllCreds()
	if err != nil {
		return false, err
	}
	if _, ok := all[key]; !ok {
		return false, nil
	}
	delete(all, key)
	return true, saveAllCreds(all)
}

func IsTokenExpired(cred *Credential) bool {
	return float64(time.Now().Unix()) >= (cred.ExpiresAt - 60)
}
