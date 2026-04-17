package ldap

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/nycuitsc/ldap-service/internal/domain"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// ---------------------------------------------------------------------------
// mockPool — implements domain.LDAPPool for Repository tests
// ---------------------------------------------------------------------------

type mockPool struct {
	searchFn      func(ctx context.Context, username string, attributes []string) (*domain.Account, error)
	bindFn        func(ctx context.Context, dn string, password string) error
	healthCheckFn func(ctx context.Context) error
	closeFn       func() error

	searchCalls    int32
	bindCalls      int32
	lastBindDN     string
	lastBindPW     string
}

func (m *mockPool) Search(ctx context.Context, username string, attributes []string) (*domain.Account, error) {
	atomic.AddInt32(&m.searchCalls, 1)
	if m.searchFn != nil {
		return m.searchFn(ctx, username, attributes)
	}
	return nil, domain.ErrAccountNotFound
}

func (m *mockPool) Bind(ctx context.Context, dn string, password string) error {
	atomic.AddInt32(&m.bindCalls, 1)
	m.lastBindDN = dn
	m.lastBindPW = password
	if m.bindFn != nil {
		return m.bindFn(ctx, dn, password)
	}
	return nil
}

func (m *mockPool) HealthCheck(ctx context.Context) error {
	if m.healthCheckFn != nil {
		return m.healthCheckFn(ctx)
	}
	return nil
}

func (m *mockPool) Close() error {
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestRepo(t *testing.T, internal, external *mockPool) (*Repository, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)
	return NewRepository(internal, external, logger), logs
}

func internalAccount(uid string) *domain.Account {
	return &domain.Account{
		DN:     "uid=" + uid + ",ou=student,o=nycu",
		UID:    uid,
		Source: domain.SourceInternal,
	}
}

func externalAccount(uid string) *domain.Account {
	return &domain.Account{
		DN:     "uid=" + uid + ",ou=alumni,o=nycu",
		UID:    uid,
		Source: domain.SourceExternal,
	}
}

// ---------------------------------------------------------------------------
// Lookup — fan-out strategy
// ---------------------------------------------------------------------------

func TestRepository_Lookup_FoundInInternal(t *testing.T) {
	want := internalAccount("110550001")
	internal := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return want, nil
	}}
	external := &mockPool{}
	repo, _ := newTestRepo(t, internal, external)

	got, err := repo.Lookup(context.Background(), "110550001", nil)
	if err != nil {
		t.Fatalf("Lookup() error = %v, want nil", err)
	}
	if got.UID != want.UID || got.Source != domain.SourceInternal {
		t.Errorf("got UID=%q Source=%q, want UID=%q Source=%q", got.UID, got.Source, want.UID, domain.SourceInternal)
	}
	if atomic.LoadInt32(&external.searchCalls) != 0 {
		t.Error("external pool was searched even though internal found the user")
	}
}

func TestRepository_Lookup_NotFoundInternalFoundExternal(t *testing.T) {
	want := externalAccount("alumni@example.com")
	internal := &mockPool{} // default returns ErrAccountNotFound
	external := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return want, nil
	}}
	repo, _ := newTestRepo(t, internal, external)

	got, err := repo.Lookup(context.Background(), "alumni@example.com", nil)
	if err != nil {
		t.Fatalf("Lookup() error = %v, want nil", err)
	}
	if got.UID != want.UID || got.Source != domain.SourceExternal {
		t.Errorf("got UID=%q Source=%q, want UID=%q Source=%q", got.UID, got.Source, want.UID, domain.SourceExternal)
	}
}

func TestRepository_Lookup_NotFoundInBothSources(t *testing.T) {
	internal := &mockPool{} // default ErrAccountNotFound
	external := &mockPool{} // default ErrAccountNotFound
	repo, _ := newTestRepo(t, internal, external)

	_, err := repo.Lookup(context.Background(), "nobody", nil)
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("Lookup() error = %v, want %v", err, domain.ErrAccountNotFound)
	}
}

func TestRepository_Lookup_InternalConnErrorFallsBackToExternal(t *testing.T) {
	connErr := errors.New("connection reset")
	want := externalAccount("user1")
	internal := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return nil, connErr
	}}
	external := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return want, nil
	}}
	repo, logs := newTestRepo(t, internal, external)

	got, err := repo.Lookup(context.Background(), "user1", nil)
	if err != nil {
		t.Fatalf("Lookup() error = %v, want nil (should fallback to external)", err)
	}
	if got.Source != domain.SourceExternal {
		t.Errorf("Source = %q, want %q", got.Source, domain.SourceExternal)
	}
	// Internal connection error MUST be logged at WARN level.
	if logs.Len() == 0 {
		t.Error("internal connection error was not logged — must log before falling back to external")
	}
}

func TestRepository_Lookup_BothSourcesConnError_ReturnsServiceUnavailable(t *testing.T) {
	connErr := errors.New("connection reset")
	internal := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return nil, connErr
	}}
	external := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return nil, connErr
	}}
	repo, _ := newTestRepo(t, internal, external)

	_, err := repo.Lookup(context.Background(), "user1", nil)
	if !errors.Is(err, domain.ErrServiceUnavailable) {
		t.Fatalf("Lookup() error = %v, want %v", err, domain.ErrServiceUnavailable)
	}
}

func TestRepository_Lookup_InternalConnErrorExternalNotFound(t *testing.T) {
	connErr := errors.New("connection reset")
	internal := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return nil, connErr
	}}
	external := &mockPool{} // default ErrAccountNotFound
	repo, _ := newTestRepo(t, internal, external)

	_, err := repo.Lookup(context.Background(), "user1", nil)
	// External returned not found — that's a legitimate answer even though internal was down.
	// This tests that not-found from external is propagated, not masked as ServiceUnavailable.
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("Lookup() error = %v, want %v (external not-found should propagate)", err, domain.ErrAccountNotFound)
	}
}

func TestRepository_Lookup_PassesAttributesThrough(t *testing.T) {
	var capturedAttrs []string
	internal := &mockPool{searchFn: func(_ context.Context, _ string, attrs []string) (*domain.Account, error) {
		capturedAttrs = attrs
		return internalAccount("u"), nil
	}}
	repo, _ := newTestRepo(t, internal, &mockPool{})

	want := []string{"cn", "mail", "ou"}
	_, _ = repo.Lookup(context.Background(), "u", want)

	if len(capturedAttrs) != len(want) {
		t.Fatalf("attributes passed = %v, want %v", capturedAttrs, want)
	}
	for i, a := range want {
		if capturedAttrs[i] != a {
			t.Errorf("attr[%d] = %q, want %q", i, capturedAttrs[i], a)
		}
	}
}

// ---------------------------------------------------------------------------
// LookupBatch
// ---------------------------------------------------------------------------

func TestRepository_LookupBatch_MixedResults(t *testing.T) {
	internal := &mockPool{searchFn: func(_ context.Context, username string, _ []string) (*domain.Account, error) {
		if username == "110550001" {
			return internalAccount("110550001"), nil
		}
		return nil, domain.ErrAccountNotFound
	}}
	external := &mockPool{searchFn: func(_ context.Context, username string, _ []string) (*domain.Account, error) {
		if username == "alumni@example.com" {
			return externalAccount("alumni@example.com"), nil
		}
		return nil, domain.ErrAccountNotFound
	}}
	repo, _ := newTestRepo(t, internal, external)

	found, notFound, err := repo.LookupBatch(context.Background(), []string{"110550001", "alumni@example.com", "nobody"}, nil)
	if err != nil {
		t.Fatalf("LookupBatch() error = %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("found %d accounts, want 2", len(found))
	}
	if len(notFound) != 1 || notFound[0] != "nobody" {
		t.Fatalf("notFound = %v, want [nobody]", notFound)
	}
}

func TestRepository_LookupBatch_AllNotFound(t *testing.T) {
	internal := &mockPool{}
	external := &mockPool{}
	repo, _ := newTestRepo(t, internal, external)

	found, notFound, err := repo.LookupBatch(context.Background(), []string{"a", "b"}, nil)
	if err != nil {
		t.Fatalf("LookupBatch() error = %v", err)
	}
	if len(found) != 0 {
		t.Errorf("found %d accounts, want 0", len(found))
	}
	if len(notFound) != 2 {
		t.Errorf("notFound has %d entries, want 2", len(notFound))
	}
}

func TestRepository_LookupBatch_ConnErrorAborts(t *testing.T) {
	connErr := errors.New("connection reset")
	internal := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return nil, connErr
	}}
	external := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return nil, connErr
	}}
	repo, _ := newTestRepo(t, internal, external)

	_, _, err := repo.LookupBatch(context.Background(), []string{"a", "b"}, nil)
	if err == nil {
		t.Fatal("LookupBatch() error = nil, want non-nil (both sources had conn errors)")
	}
}

func TestRepository_LookupBatch_EmptyInput(t *testing.T) {
	repo, _ := newTestRepo(t, &mockPool{}, &mockPool{})

	found, notFound, err := repo.LookupBatch(context.Background(), []string{}, nil)
	if err != nil {
		t.Fatalf("LookupBatch() error = %v", err)
	}
	if len(found) != 0 || len(notFound) != 0 {
		t.Errorf("expected empty results, got found=%d notFound=%d", len(found), len(notFound))
	}
}

// ---------------------------------------------------------------------------
// Authenticate — SECURITY CRITICAL
// ---------------------------------------------------------------------------

func TestRepository_Authenticate_InternalUserSuccess(t *testing.T) {
	acc := internalAccount("110550001")
	internal := &mockPool{
		searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
			return acc, nil
		},
		// Bind succeeds (default)
	}
	external := &mockPool{}
	repo, _ := newTestRepo(t, internal, external)

	ok, err := repo.Authenticate(context.Background(), "110550001", "correct-pw")
	if err != nil {
		t.Fatalf("Authenticate() error = %v, want nil", err)
	}
	if !ok {
		t.Fatal("Authenticate() = false, want true")
	}
	// Bind MUST be called on the internal pool (same pool that found the user).
	if atomic.LoadInt32(&internal.bindCalls) != 1 {
		t.Errorf("internal.Bind calls = %d, want 1", internal.bindCalls)
	}
	if atomic.LoadInt32(&external.bindCalls) != 0 {
		t.Error("external.Bind was called — must bind against the SAME pool that found the user")
	}
	if internal.lastBindDN != acc.DN {
		t.Errorf("Bind DN = %q, want %q (user's DN from search)", internal.lastBindDN, acc.DN)
	}
}

func TestRepository_Authenticate_ExternalUserSuccess(t *testing.T) {
	acc := externalAccount("alumni@example.com")
	internal := &mockPool{} // default not found
	external := &mockPool{
		searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
			return acc, nil
		},
	}
	repo, _ := newTestRepo(t, internal, external)

	ok, err := repo.Authenticate(context.Background(), "alumni@example.com", "correct-pw")
	if err != nil {
		t.Fatalf("Authenticate() error = %v, want nil", err)
	}
	if !ok {
		t.Fatal("Authenticate() = false, want true")
	}
	// Bind MUST be on external pool — the pool that found the user.
	if atomic.LoadInt32(&external.bindCalls) != 1 {
		t.Errorf("external.Bind calls = %d, want 1", external.bindCalls)
	}
	if atomic.LoadInt32(&internal.bindCalls) != 0 {
		t.Error("internal.Bind was called — must bind against the SAME pool that found the user")
	}
	if external.lastBindDN != acc.DN {
		t.Errorf("Bind DN = %q, want %q", external.lastBindDN, acc.DN)
	}
}

// TestRepository_Authenticate_WrongPassword — MUST return (false, nil), NOT
// (false, error). The caller must not be able to distinguish wrong password
// from user not found.
func TestRepository_Authenticate_WrongPassword(t *testing.T) {
	acc := internalAccount("110550001")
	internal := &mockPool{
		searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
			return acc, nil
		},
		bindFn: func(_ context.Context, _ string, _ string) error {
			return errors.New("invalid credentials")
		},
	}
	repo, _ := newTestRepo(t, internal, &mockPool{})

	ok, err := repo.Authenticate(context.Background(), "110550001", "wrong-pw")
	if ok {
		t.Fatal("Authenticate() = true, want false (wrong password)")
	}
	// SECURITY: must return (false, nil) — no error that reveals the reason.
	if err != nil {
		t.Fatalf("Authenticate() error = %v, want nil (must not reveal failure reason)", err)
	}
}

// TestRepository_Authenticate_UserNotFound — MUST return (false, nil),
// identical to wrong password response.
func TestRepository_Authenticate_UserNotFound(t *testing.T) {
	internal := &mockPool{} // default not found
	external := &mockPool{} // default not found
	repo, _ := newTestRepo(t, internal, external)

	ok, err := repo.Authenticate(context.Background(), "nobody", "any-pw")
	if ok {
		t.Fatal("Authenticate() = true, want false (user not found)")
	}
	// SECURITY: must return (false, nil) — identical to wrong password.
	if err != nil {
		t.Fatalf("Authenticate() error = %v, want nil (must not reveal failure reason)", err)
	}
}

// TestRepository_Authenticate_InternalConnErrorFallsBackToExternal verifies
// that when internal pool has a connection error, external is still tried.
func TestRepository_Authenticate_InternalConnErrorFallsBackToExternal(t *testing.T) {
	connErr := errors.New("connection reset")
	acc := externalAccount("user1")
	internal := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return nil, connErr
	}}
	external := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return acc, nil
	}}
	repo, _ := newTestRepo(t, internal, external)

	ok, err := repo.Authenticate(context.Background(), "user1", "correct-pw")
	if err != nil {
		t.Fatalf("Authenticate() error = %v, want nil", err)
	}
	if !ok {
		t.Fatal("Authenticate() = false, want true (found in external after internal conn error)")
	}
	if atomic.LoadInt32(&external.bindCalls) != 1 {
		t.Error("external.Bind was not called — must bind against external pool")
	}
}

// TestRepository_Authenticate_BothSourcesDown — MUST return (false, nil),
// NOT (false, ErrServiceUnavailable). Auth must never leak infrastructure state.
func TestRepository_Authenticate_BothSourcesDown(t *testing.T) {
	connErr := errors.New("connection reset")
	internal := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return nil, connErr
	}}
	external := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return nil, connErr
	}}
	repo, _ := newTestRepo(t, internal, external)

	ok, err := repo.Authenticate(context.Background(), "user1", "pw")
	if ok {
		t.Fatal("Authenticate() = true, want false")
	}
	// SECURITY: Even service-unavailable must be masked as (false, nil).
	if err != nil {
		t.Fatalf("Authenticate() error = %v, want nil (must not reveal infrastructure state)", err)
	}
}

// TestRepository_Authenticate_NeverLogsPassword checks that no WARN-level
// log entry contains the password string.
func TestRepository_Authenticate_NeverLogsPassword(t *testing.T) {
	password := "SuperSecret123!"
	internal := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return internalAccount("u"), nil
	}, bindFn: func(_ context.Context, _ string, _ string) error {
		return errors.New("invalid credentials")
	}}
	repo, logs := newTestRepo(t, internal, &mockPool{})

	_, _ = repo.Authenticate(context.Background(), "u", password)

	for _, entry := range logs.All() {
		msg := entry.Message
		for _, field := range entry.ContextMap() {
			if str, ok := field.(string); ok {
				msg += " " + str
			}
		}
		if containsPassword(msg, password) {
			t.Fatalf("log entry contains password: %q", entry.Message)
		}
	}
}

func containsPassword(haystack, password string) bool {
	return len(password) > 0 && len(haystack) >= len(password) &&
		// Simple substring check.
		stringContains(haystack, password)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestRepository_Authenticate_BindUsesUserDN verifies the DN from search
// (not the username) is passed to Bind.
func TestRepository_Authenticate_BindUsesUserDN(t *testing.T) {
	acc := &domain.Account{
		DN:     "uid=T1234,ou=employee,o=nycu",
		UID:    "T1234",
		Source: domain.SourceInternal,
	}
	internal := &mockPool{searchFn: func(_ context.Context, _ string, _ []string) (*domain.Account, error) {
		return acc, nil
	}}
	repo, _ := newTestRepo(t, internal, &mockPool{})

	_, _ = repo.Authenticate(context.Background(), "T1234", "pw")

	if internal.lastBindDN != acc.DN {
		t.Errorf("Bind was called with DN %q, want %q (must use DN from search, not username)", internal.lastBindDN, acc.DN)
	}
}

// ---------------------------------------------------------------------------
// HealthCheck
// ---------------------------------------------------------------------------

func TestRepository_HealthCheck_BothHealthy(t *testing.T) {
	repo, _ := newTestRepo(t, &mockPool{}, &mockPool{})

	if err := repo.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck() error = %v, want nil", err)
	}
}

func TestRepository_HealthCheck_InternalUnhealthy(t *testing.T) {
	internal := &mockPool{healthCheckFn: func(_ context.Context) error {
		return errors.New("internal down")
	}}
	repo, _ := newTestRepo(t, internal, &mockPool{})

	err := repo.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("HealthCheck() error = nil, want non-nil (internal unhealthy)")
	}
	if !errors.Is(err, domain.ErrServiceUnavailable) {
		t.Errorf("HealthCheck() error = %v, want %v", err, domain.ErrServiceUnavailable)
	}
}

func TestRepository_HealthCheck_ExternalUnhealthy(t *testing.T) {
	external := &mockPool{healthCheckFn: func(_ context.Context) error {
		return errors.New("external down")
	}}
	repo, _ := newTestRepo(t, &mockPool{}, external)

	err := repo.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("HealthCheck() error = nil, want non-nil (external unhealthy)")
	}
	if !errors.Is(err, domain.ErrServiceUnavailable) {
		t.Errorf("HealthCheck() error = %v, want %v", err, domain.ErrServiceUnavailable)
	}
}

func TestRepository_HealthCheck_BothUnhealthy(t *testing.T) {
	internal := &mockPool{healthCheckFn: func(_ context.Context) error {
		return errors.New("internal down")
	}}
	external := &mockPool{healthCheckFn: func(_ context.Context) error {
		return errors.New("external down")
	}}
	repo, _ := newTestRepo(t, internal, external)

	err := repo.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("HealthCheck() error = nil, want non-nil")
	}
	if !errors.Is(err, domain.ErrServiceUnavailable) {
		t.Errorf("HealthCheck() error = %v, want %v", err, domain.ErrServiceUnavailable)
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func TestRepository_Close_ClosesBothPools(t *testing.T) {
	var intClosed, extClosed bool
	internal := &mockPool{closeFn: func() error { intClosed = true; return nil }}
	external := &mockPool{closeFn: func() error { extClosed = true; return nil }}
	repo, _ := newTestRepo(t, internal, external)

	if err := repo.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !intClosed {
		t.Error("internal pool was not closed")
	}
	if !extClosed {
		t.Error("external pool was not closed")
	}
}
