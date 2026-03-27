package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths holds the standard filesystem paths for config and cache.
type Paths struct {
	ConfigDir  string
	ConfigFile string
	CacheDir   string
}

// DefaultPaths returns XDG-compliant paths for ihj.
func DefaultPaths() Paths {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "ihj")
	cacheDir := filepath.Join(home, ".local", "state", "ihj")
	return Paths{
		ConfigDir:  configDir,
		ConfigFile: filepath.Join(configDir, "config.yaml"),
		CacheDir:   cacheDir,
	}
}

// EnsureDirs creates the config and cache directories if they don't exist.
func (p Paths) EnsureDirs() error {
	if err := os.MkdirAll(p.ConfigDir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := os.MkdirAll(p.CacheDir, 0o755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}
	return nil
}
