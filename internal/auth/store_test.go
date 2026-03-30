package auth_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mikecsmith/ihj/internal/auth"
)

// mockStore is a simple in-memory CredentialStore for testing.
type mockStore struct {
	tokens map[string]string
	err    error // if set, all operations return this error
}

func newMockStore() *mockStore {
	return &mockStore{tokens: make(map[string]string)}
}

func (m *mockStore) Get(alias string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	token, ok := m.tokens[alias]
	if !ok {
		return "", auth.ErrNotFound
	}
	return token, nil
}

func (m *mockStore) Set(alias, token string) error {
	if m.err != nil {
		return m.err
	}
	m.tokens[alias] = token
	return nil
}

func (m *mockStore) Delete(alias string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.tokens, alias)
	return nil
}

func (m *mockStore) List() ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	out := make([]string, 0, len(m.tokens))
	for k := range m.tokens {
		out = append(out, k)
	}
	return out, nil
}

// ── ChainStore Tests ────────────────────────────────────────────

func TestChainStore_FirstHit(t *testing.T) {
	s1 := newMockStore()
	s1.tokens["server-a"] = "token-from-first"
	s2 := newMockStore()
	s2.tokens["server-a"] = "token-from-second"

	chain := auth.NewChainStore(s1, s2)

	got, err := chain.Get("server-a")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got != "token-from-first" {
		t.Errorf("Get() = %q; want %q", got, "token-from-first")
	}
}

func TestChainStore_Fallthrough(t *testing.T) {
	s1 := newMockStore()
	s2 := newMockStore()
	s2.tokens["server-a"] = "token-from-second"

	chain := auth.NewChainStore(s1, s2)

	got, err := chain.Get("server-a")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got != "token-from-second" {
		t.Errorf("Get() = %q; want %q", got, "token-from-second")
	}
}

func TestChainStore_AllMiss(t *testing.T) {
	s1 := newMockStore()
	s2 := newMockStore()

	chain := auth.NewChainStore(s1, s2)

	_, err := chain.Get("server-a")
	if !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("Get() error = %v; want ErrNotFound", err)
	}
}

func TestChainStore_PropagatesRealErrors(t *testing.T) {
	realErr := errors.New("keychain locked")
	s1 := newMockStore()
	s1.err = realErr
	s2 := newMockStore()
	s2.tokens["server-a"] = "fallback"

	chain := auth.NewChainStore(s1, s2)

	_, err := chain.Get("server-a")
	if !errors.Is(err, realErr) {
		t.Errorf("Get() error = %v; want %v", err, realErr)
	}
}

func TestChainStore_SetUsesFirst(t *testing.T) {
	s1 := newMockStore()
	s2 := newMockStore()

	chain := auth.NewChainStore(s1, s2)

	if err := chain.Set("server-a", "my-token"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	if s1.tokens["server-a"] != "my-token" {
		t.Error("Set() should store in first backend")
	}
	if _, ok := s2.tokens["server-a"]; ok {
		t.Error("Set() should not store in second backend")
	}
}

func TestChainStore_DeleteUsesFirst(t *testing.T) {
	s1 := newMockStore()
	s1.tokens["server-a"] = "token"

	chain := auth.NewChainStore(s1)

	if err := chain.Delete("server-a"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if _, ok := s1.tokens["server-a"]; ok {
		t.Error("Delete() should remove from first backend")
	}
}

func TestChainStore_EmptyChain(t *testing.T) {
	chain := auth.NewChainStore()

	_, err := chain.Get("server-a")
	if !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("Get() on empty chain: error = %v; want ErrNotFound", err)
	}
	if err := chain.Set("server-a", "token"); err == nil {
		t.Error("Set() on empty chain should return error")
	}
}

// ── EnvStore Tests ──────────────────────────────────────────────

func TestEnvStore_ServerSpecific(t *testing.T) {
	t.Setenv("IHJ_TOKEN_MYCOMPANY_JIRA", "env-token")

	store := &auth.EnvStore{}
	got, err := store.Get("mycompany-jira")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got != "env-token" {
		t.Errorf("Get() = %q; want %q", got, "env-token")
	}
}

func TestEnvStore_NotFound(t *testing.T) {
	store := &auth.EnvStore{}
	_, err := store.Get("nonexistent-server")
	if !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("Get() error = %v; want ErrNotFound", err)
	}
}

func TestEnvStore_SetReturnsError(t *testing.T) {
	store := &auth.EnvStore{}
	if err := store.Set("alias", "token"); err == nil {
		t.Error("Set() should return error for env store")
	}
}

// ── FileStore Tests ─────────────────────────────────────────────

func TestFileStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := auth.NewFileStore(dir)

	// Initially empty.
	_, err := store.Get("server-a")
	if !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("Get() on empty store: error = %v; want ErrNotFound", err)
	}

	// Set and get.
	if err := store.Set("server-a", "my-token"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	got, err := store.Get("server-a")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got != "my-token" {
		t.Errorf("Get() = %q; want %q", got, "my-token")
	}

	// Delete.
	if err := store.Delete("server-a"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	_, err = store.Get("server-a")
	if !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("Get() after Delete: error = %v; want ErrNotFound", err)
	}
}

func TestFileStore_Permissions(t *testing.T) {
	dir := t.TempDir()
	store := auth.NewFileStore(dir)

	if err := store.Set("server-a", "secret"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "credentials.json"))
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o; want 0600", perm)
	}
}

func TestFileStore_List(t *testing.T) {
	dir := t.TempDir()
	store := auth.NewFileStore(dir)

	_ = store.Set("server-a", "token-a")
	_ = store.Set("server-b", "token-b")

	aliases, err := store.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(aliases) != 2 {
		t.Errorf("List() returned %d aliases; want 2", len(aliases))
	}
}

func TestFileStore_MultipleKeys(t *testing.T) {
	dir := t.TempDir()
	store := auth.NewFileStore(dir)

	_ = store.Set("server-a", "token-a")
	_ = store.Set("server-b", "token-b")

	gotA, _ := store.Get("server-a")
	gotB, _ := store.Get("server-b")

	if gotA != "token-a" {
		t.Errorf("Get(server-a) = %q; want %q", gotA, "token-a")
	}
	if gotB != "token-b" {
		t.Errorf("Get(server-b) = %q; want %q", gotB, "token-b")
	}
}
