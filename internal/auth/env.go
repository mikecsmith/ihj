package auth

import (
	"errors"
	"os"
	"strings"
)

// EnvStore is a read-only CredentialStore that looks up tokens from
// environment variables. The naming convention is IHJ_TOKEN_<ALIAS>
// where the alias is uppercased with hyphens replaced by underscores.
//
// Example: server alias "mycompany-jira" checks IHJ_TOKEN_MYCOMPANY_JIRA.
type EnvStore struct{}

// Get checks the environment for a server-specific token.
func (e *EnvStore) Get(serverAlias string) (string, error) {
	envKey := "IHJ_TOKEN_" + strings.ToUpper(strings.ReplaceAll(serverAlias, "-", "_"))
	if v := os.Getenv(envKey); v != "" {
		return v, nil
	}
	return "", ErrNotFound
}

// Set is not supported for environment variables.
func (e *EnvStore) Set(_, _ string) error {
	return errors.New("cannot store credentials in environment variables")
}

// Delete is not supported for environment variables.
func (e *EnvStore) Delete(_ string) error {
	return errors.New("cannot delete credentials from environment variables")
}

// List always returns nil — environment scanning is not supported.
func (e *EnvStore) List() ([]string, error) {
	return nil, nil
}
