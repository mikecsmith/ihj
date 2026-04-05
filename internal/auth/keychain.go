package auth

import (
	"errors"
	"strings"

	"github.com/zalando/go-keyring"
)

const keychainService = "ihj"

// KeychainStore stores credentials in the OS keychain (macOS Keychain,
// Linux libsecret/kwallet, Windows Credential Manager) via go-keyring.
// The service name is "ihj" and the "user" is the server alias.
type KeychainStore struct{}

// Get retrieves a token from the OS keychain.
//
// When the underlying keychain backend is unavailable (e.g. Linux without
// a running secret-service/D-Bus, WSL, or sandboxed CI containers) we
// translate the failure to ErrNotFound so the ChainStore can fall through
// to the next backend instead of short-circuiting the whole lookup.
func (k *KeychainStore) Get(serverAlias string) (string, error) {
	token, err := keyring.Get(keychainService, serverAlias)
	if err == nil {
		return token, nil
	}
	if errors.Is(err, keyring.ErrNotFound) || backendUnavailable(err) {
		return "", ErrNotFound
	}
	return "", err
}

// backendUnavailable reports whether an error from go-keyring indicates
// the platform keychain is not reachable at all (as opposed to the
// requested item simply not existing).
func backendUnavailable(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "org.freedesktop.secrets") ||
		strings.Contains(msg, "The name is not activatable") ||
		strings.Contains(msg, "dbus") ||
		strings.Contains(msg, "D-Bus")
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
