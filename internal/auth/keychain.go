package auth

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const keychainService = "ihj"

// KeychainStore stores credentials in the OS keychain (macOS Keychain,
// Linux libsecret/kwallet, Windows Credential Manager) via go-keyring.
// The service name is "ihj" and the "user" is the server alias.
type KeychainStore struct{}

// Get retrieves a token from the OS keychain.
func (k *KeychainStore) Get(serverAlias string) (string, error) {
	token, err := keyring.Get(keychainService, serverAlias)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return token, nil
}

// Set stores a token in the OS keychain.
func (k *KeychainStore) Set(serverAlias, token string) error {
	return keyring.Set(keychainService, serverAlias, token)
}

// Delete removes a token from the OS keychain.
func (k *KeychainStore) Delete(serverAlias string) error {
	err := keyring.Delete(keychainService, serverAlias)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil // already absent
	}
	return err
}

// List is not efficiently supported by go-keyring; returns nil.
// Use the config's server list to drive status checks instead.
func (k *KeychainStore) List() ([]string, error) {
	return nil, nil
}
