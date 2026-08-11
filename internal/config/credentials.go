// Package config resolves which Confluence site to talk to and loads the
// per-host credentials and optional per-project settings.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Credential is the stored auth for one Confluence host.
type Credential struct {
	Email string `yaml:"email"`
	Token string `yaml:"token"`
}

// credentialsFile is the on-disk shape: host -> credential.
type credentialsFile struct {
	Hosts map[string]Credential `yaml:"hosts"`
}

const (
	configDirName   = "acli-plus"
	credentialsName = "credentials.yaml"
	dirPerm         = 0o700
	filePerm        = 0o600
)

// CredentialStore persists per-host credentials under the user's config dir.
type CredentialStore struct {
	path string
}

// NewCredentialStore resolves the store path (honoring XDG_CONFIG_HOME, else
// ~/.config), without touching the filesystem.
func NewCredentialStore() (*CredentialStore, error) {
	base, err := configBaseDir()
	if err != nil {
		return nil, err
	}
	return &CredentialStore{path: filepath.Join(base, configDirName, credentialsName)}, nil
}

// Path returns the store's file path (useful for user-facing messages).
func (s *CredentialStore) Path() string { return s.path }

// Get returns the credential for host and whether one is stored.
func (s *CredentialStore) Get(host string) (Credential, bool, error) {
	file, err := s.load()
	if err != nil {
		return Credential{}, false, err
	}
	cred, ok := file.Hosts[host]
	return cred, ok, nil
}

// Hosts returns the registered hosts in a stable order. It is how a command
// that carries no URL of its own can still find the site to talk to.
func (s *CredentialStore) Hosts() ([]string, error) {
	file, err := s.load()
	if err != nil {
		return nil, err
	}
	hosts := make([]string, 0, len(file.Hosts))
	for host := range file.Hosts {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return hosts, nil
}

// Save upserts the credential for host, creating the store with secure
// permissions (dir 0700, file 0600) and never duplicating a host entry.
func (s *CredentialStore) Save(host string, cred Credential) error {
	file, err := s.load()
	if err != nil {
		return err
	}
	file.Hosts[host] = cred

	data, err := yaml.Marshal(file)
	if err != nil {
		return fmt.Errorf("encoding credentials: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), dirPerm); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := os.WriteFile(s.path, data, filePerm); err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}
	// Enforce 0600 even if the file already existed with looser permissions.
	if err := os.Chmod(s.path, filePerm); err != nil {
		return fmt.Errorf("securing credentials file: %w", err)
	}
	return nil
}

func (s *CredentialStore) load() (credentialsFile, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return credentialsFile{Hosts: map[string]Credential{}}, nil
	}
	if err != nil {
		return credentialsFile{}, fmt.Errorf("reading credentials: %w", err)
	}
	var file credentialsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return credentialsFile{}, fmt.Errorf("parsing credentials: %w", err)
	}
	if file.Hosts == nil {
		file.Hosts = map[string]Credential{}
	}
	return file, nil
}

func configBaseDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	return filepath.Join(home, ".config"), nil
}
