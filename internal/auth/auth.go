// Package auth provides local API bearer-token permissions without storing
// plaintext credentials.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Role string

const (
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
)

type Token struct {
	Name        string `yaml:"name"`
	TokenSHA256 string `yaml:"token_sha256"`
	Role        Role   `yaml:"role"`
}
type Config struct {
	Version int     `yaml:"version"`
	Tokens  []Token `yaml:"tokens"`
}
type Principal struct {
	Name string `json:"name"`
	Role Role   `json:"role"`
}
type Manager struct{ tokens []Token }

func Load(path string) (*Manager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	if config.Version != 1 {
		return nil, errors.New("unsupported access policy version")
	}
	for _, item := range config.Tokens {
		if !ValidRole(item.Role) || len(item.TokenSHA256) != 64 {
			return nil, errors.New("invalid access token entry")
		}
	}
	return &Manager{tokens: config.Tokens}, nil
}
func (m *Manager) Authenticate(header string) (Principal, bool) {
	scheme, value, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || value == "" {
		return Principal{}, false
	}
	candidate := HashToken(value)
	for _, item := range m.tokens {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(strings.ToLower(item.TokenSHA256))) == 1 {
			return Principal{Name: item.Name, Role: item.Role}, true
		}
	}
	return Principal{}, false
}
func HashToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func Allows(actual, required Role) bool { return rank(actual) >= rank(required) }
func rank(role Role) int {
	switch role {
	case RoleAdmin:
		return 3
	case RoleOperator:
		return 2
	case RoleViewer:
		return 1
	}
	return 0
}
func ValidRole(role Role) bool { return rank(role) > 0 }
func AddToken(path, name string, role Role) (string, error) {
	if !ValidRole(role) {
		return "", errors.New("invalid role")
	}
	config := Config{Version: 1}
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &config); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	plain := base64.RawURLEncoding.EncodeToString(raw)
	config.Tokens = append(config.Tokens, Token{Name: name, Role: role, TokenSHA256: HashToken(plain)})
	data, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}
	if err := writeAtomic(path, data); err != nil {
		return "", err
	}
	return plain, nil
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".access-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.Rename(name, path)
	} else if err != nil {
		return err
	}
	backupFile, err := os.CreateTemp(filepath.Dir(path), ".access-backup-*")
	if err != nil {
		return err
	}
	backup := backupFile.Name()
	if err := backupFile.Close(); err != nil {
		return err
	}
	if err := os.Remove(backup); err != nil {
		return err
	}
	if err := os.Rename(path, backup); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Rename(backup, path)
		return err
	}
	return os.Remove(backup)
}
