package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const credentialsFile = "credentials.json"

// FileStore stores credentials as a JSON file with restrictive permissions
// (0600). Suitable as a fallback when the OS keychain is unavailable.
//
// The file is stored at <configDir>/credentials.json.
type FileStore struct {
	path string
	mu   sync.Mutex
}

// NewFileStore creates a FileStore that persists credentials to the given
// config directory.
func NewFileStore(configDir string) *FileStore {
	return &FileStore{
		path: filepath.Join(configDir, credentialsFile),
	}
}

// Get retrieves a token from the credentials file.
func (f *FileStore) Get(serverAlias string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	creds, err := f.load()
	if err != nil {
		return "", err
	}

	token, ok := creds[serverAlias]
	if !ok {
		return "", ErrNotFound
	}
	return token, nil
}

// Set stores a token in the credentials file.
func (f *FileStore) Set(serverAlias, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	creds, err := f.load()
	if err != nil {
		return err
	}

	creds[serverAlias] = token
	return f.save(creds)
}

// Delete removes a token from the credentials file.
func (f *FileStore) Delete(serverAlias string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	creds, err := f.load()
	if err != nil {
		return err
	}

	delete(creds, serverAlias)
	return f.save(creds)
}

// List returns all server aliases with stored tokens.
func (f *FileStore) List() ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	creds, err := f.load()
	if err != nil {
		return nil, err
	}

	aliases := make([]string, 0, len(creds))
	for alias := range creds {
		aliases = append(aliases, alias)
	}
	return aliases, nil
}

func (f *FileStore) load() (map[string]string, error) {
	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return make(map[string]string), nil
	}
	if err != nil {
		return nil, err
	}

	var creds map[string]string
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return creds, nil
}

func (f *FileStore) save(creds map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(f.path, data, 0o600)
}
