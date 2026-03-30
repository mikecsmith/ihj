package testutil

import "github.com/mikecsmith/ihj/internal/auth"

// MockCredentialStore is an in-memory auth.CredentialStore for testing.
type MockCredentialStore struct {
	Tokens map[string]string
}

// NewMockCredentialStore creates a MockCredentialStore with no stored tokens.
func NewMockCredentialStore() *MockCredentialStore {
	return &MockCredentialStore{Tokens: make(map[string]string)}
}

func (m *MockCredentialStore) Get(serverAlias string) (string, error) {
	token, ok := m.Tokens[serverAlias]
	if !ok {
		return "", auth.ErrNotFound
	}
	return token, nil
}

func (m *MockCredentialStore) Set(serverAlias, token string) error {
	m.Tokens[serverAlias] = token
	return nil
}

func (m *MockCredentialStore) Delete(serverAlias string) error {
	delete(m.Tokens, serverAlias)
	return nil
}

func (m *MockCredentialStore) List() ([]string, error) {
	out := make([]string, 0, len(m.Tokens))
	for k := range m.Tokens {
		out = append(out, k)
	}
	return out, nil
}
