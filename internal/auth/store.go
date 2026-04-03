// Package auth provides credential storage and retrieval for server access
// tokens. It abstracts the underlying storage mechanism (OS keychain,
// environment variables, encrypted file) behind a CredentialStore interface,
// allowing the rest of the application to remain agnostic to how tokens are
// persisted.
package auth

import "errors"

// ErrNotFound is returned when no credential exists for the given server alias.
var ErrNotFound = errors.New("credential not found")

// CredentialStore abstracts secure token storage. Implementations may use
// the OS keychain, environment variables, or encrypted files. The interface
// is intentionally simple to support future use cases (e.g., OAuth refresh
// tokens) without modification.
type CredentialStore interface {
	// Get retrieves the token for a server alias.
	// Returns ErrNotFound if no credential is stored.
	Get(serverAlias string) (string, error)

	// Set stores a token for a server alias, overwriting any existing value.
	Set(serverAlias, token string) error

	// Delete removes the token for a server alias. Returns nil if absent.
	Delete(serverAlias string) error

	// List returns all server aliases that have stored tokens.
	List() ([]string, error)
}

// ChainStore tries multiple CredentialStore backends in order, returning the
// first successful result. Writes go to the first store only.
type ChainStore struct {
	stores []CredentialStore
}

// NewChainStore creates a ChainStore that queries backends in order.
// The first store is used for Set and Delete operations.
func NewChainStore(stores ...CredentialStore) *ChainStore {
	return &ChainStore{stores: stores}
}

// Get returns the first successful token lookup across all stores.
// Only ErrNotFound is treated as "try next"; real errors are propagated.
func (c *ChainStore) Get(serverAlias string) (string, error) {
	for _, s := range c.stores {
		token, err := s.Get(serverAlias)
		if err == nil {
			return token, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return "", err
		}
	}
	return "", ErrNotFound
}

// Set attempts to store the token, falling back to the next store if one fails.
func (c *ChainStore) Set(serverAlias, token string) error {
	if len(c.stores) == 0 {
		return errors.New("no credential stores configured")
	}

	var lastErr error
	for _, s := range c.stores {
		err := s.Set(serverAlias, token)
		if err == nil {
			return nil // Success! Token is saved.
		}
		// Track the error and try the next store in the chain
		lastErr = err
	}

	// If it loops through everything and they all fail, return the last error
	return lastErr
}

// Delete attempts to remove the token from all stores
func (c *ChainStore) Delete(serverAlias string) error {
	if len(c.stores) == 0 {
		return errors.New("no credential stores configured")
	}

	var lastErr error
	deletedFromAny := false

	for _, s := range c.stores {
		err := s.Delete(serverAlias)
		if err == nil {
			deletedFromAny = true
		} else if !errors.Is(err, ErrNotFound) {
			lastErr = err
		}
	}

	if !deletedFromAny && lastErr != nil {
		return lastErr
	}
	return nil
}

// List returns aliases from the first (primary) store.
func (c *ChainStore) List() ([]string, error) {
	if len(c.stores) == 0 {
		return nil, nil
	}
	return c.stores[0].List()
}
