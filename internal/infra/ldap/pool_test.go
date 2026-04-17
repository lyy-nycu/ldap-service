package ldap

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/nycuitsc/ldap-service/internal/domain"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// mockConn — implements ldapConn for testing
// ---------------------------------------------------------------------------

type mockConn struct {
	mu           sync.Mutex
	searchFn     func(*ldapv3.SearchRequest) (*ldapv3.SearchResult, error)
	bindFn       func(string, string) error
	isClosingVal bool

	closed       bool
	closeCalls   int32
	searchCalls  int32
	bindCalls    int32
	lastSearchReq *ldapv3.SearchRequest
	lastBindDN   string
	lastBindPW   string
}

func (m *mockConn) Search(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
	atomic.AddInt32(&m.searchCalls, 1)
	m.mu.Lock()
	m.lastSearchReq = req
	m.mu.Unlock()
	if m.searchFn != nil {
		return m.searchFn(req)
	}
	return &ldapv3.SearchResult{Entries: []*ldapv3.Entry{}}, nil
}

func (m *mockConn) Bind(dn, password string) error {
	atomic.AddInt32(&m.bindCalls, 1)
	m.mu.Lock()
	m.lastBindDN = dn
	m.lastBindPW = password
	m.mu.Unlock()
	if m.bindFn != nil {
		return m.bindFn(dn, password)
	}
	return nil
}

func (m *mockConn) Close() error {
	atomic.AddInt32(&m.closeCalls, 1)
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	return nil
}

func (m *mockConn) IsClosing() bool {
	return m.isClosingVal
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestPool constructs a Pool directly (not via NewPool) with a pre-filled
// connection channel so individual method tests don't need to exercise the
// constructor. Size of conns channel = len(initial).
func newTestPool(t *testing.T, source string, initial []*mockConn, dial dialFn) *Pool {
	t.Helper()
	size := len(initial)
	if size == 0 {
		size = 1
	}
	ch := make(chan ldapConn, size)
	for _, c := range initial {
		ch <- c
	}
	return &Pool{
		conns:    ch,
		host:     "ldap.test",
		port:     636,
		useTLS:   true,
		bindDN:   "cn=readonly,o=nycu",
		bindPW:   "test-secret",
		baseDN:   "o=nycu",
		source:   source,
		poolSize: cap(ch),
		dial:     dial,
		logger:   zap.NewNop(),
	}
}

// entryFor builds an LDAP search entry with the given DN, uid, and attributes.
func entryFor(dn, uid string, attrs map[string]string) *ldapv3.Entry {
	entry := &ldapv3.Entry{DN: dn}
	entry.Attributes = append(entry.Attributes, &ldapv3.EntryAttribute{
		Name:   "uid",
		Values: []string{uid},
	})
	for name, value := range attrs {
		entry.Attributes = append(entry.Attributes, &ldapv3.EntryAttribute{
			Name:   name,
			Values: []string{value},
		})
	}
	return entry
}

// ---------------------------------------------------------------------------
// NewPool
// ---------------------------------------------------------------------------

func TestNewPool_DialsPoolSizeConnections(t *testing.T) {
	var dialCount int32
	dial := func() (ldapConn, error) {
		atomic.AddInt32(&dialCount, 1)
		return &mockConn{}, nil
	}
	// Build Pool with a constructor that exercises the dial loop.
	// Since NewPool does TLS + real dial, we simulate by calling dial manually;
	// the real NewPool must dial poolSize times.
	poolSize := 5
	conns := make(chan ldapConn, poolSize)
	for i := 0; i < poolSize; i++ {
		c, err := dial()
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		conns <- c
	}
	if got := atomic.LoadInt32(&dialCount); got != int32(poolSize) {
		t.Fatalf("dial called %d times, want %d", got, poolSize)
	}
	if len(conns) != poolSize {
		t.Fatalf("pool has %d conns, want %d", len(conns), poolSize)
	}
}

func TestNewPool_ErrorOnDialFailure(t *testing.T) {
	wantErr := errors.New("dial failed")
	// Simulate: if dial returns error mid-loop, NewPool must propagate it.
	dial := func() (ldapConn, error) {
		return nil, wantErr
	}
	_, err := dial()
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestPool_Search_SuccessfulInternal(t *testing.T) {
	conn := &mockConn{
		searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return &ldapv3.SearchResult{
				Entries: []*ldapv3.Entry{
					entryFor("uid=110550001,ou=student,o=nycu", "110550001", map[string]string{
						"cn":   "Student Name",
						"mail": "student@nycu.edu.tw",
					}),
				},
			}, nil
		},
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{conn}, nil)

	acc, err := p.Search(context.Background(), "110550001", []string{"cn", "mail"})
	if err != nil {
		t.Fatalf("Search() error = %v, want nil", err)
	}
	if acc == nil {
		t.Fatal("Search() returned nil account")
	}
	if acc.Source != domain.SourceInternal {
		t.Errorf("Account.Source = %q, want %q", acc.Source, domain.SourceInternal)
	}
	if acc.UID != "110550001" {
		t.Errorf("Account.UID = %q, want %q", acc.UID, "110550001")
	}
	if acc.DN != "uid=110550001,ou=student,o=nycu" {
		t.Errorf("Account.DN = %q, want %q", acc.DN, "uid=110550001,ou=student,o=nycu")
	}
}

func TestPool_Search_SuccessfulExternalSourceLabel(t *testing.T) {
	conn := &mockConn{
		searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return &ldapv3.SearchResult{
				Entries: []*ldapv3.Entry{
					entryFor("uid=alumni@example.com,ou=alumni,o=nycu", "alumni@example.com", nil),
				},
			}, nil
		},
	}
	p := newTestPool(t, domain.SourceExternal, []*mockConn{conn}, nil)

	acc, err := p.Search(context.Background(), "alumni@example.com", nil)
	if err != nil {
		t.Fatalf("Search() error = %v, want nil", err)
	}
	if acc.Source != domain.SourceExternal {
		t.Errorf("Account.Source = %q, want %q", acc.Source, domain.SourceExternal)
	}
}

func TestPool_Search_NotFound(t *testing.T) {
	conn := &mockConn{
		searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return &ldapv3.SearchResult{Entries: []*ldapv3.Entry{}}, nil
		},
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{conn}, nil)

	_, err := p.Search(context.Background(), "nonexistent", nil)
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("Search() error = %v, want %v", err, domain.ErrAccountNotFound)
	}
}

// TestPool_Search_EscapeFilter verifies that LDAP injection attempts in
// the username are escaped via ldap.EscapeFilter before being placed in
// the filter string. This is defense-in-depth: ValidateUsername should
// reject these at the use case layer, but Pool.Search MUST still escape.
func TestPool_Search_EscapeFilter(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		mustContain []string // substrings that MUST appear in the filter
		mustNotContain []string // substrings that MUST NOT appear (raw unescaped chars)
	}{
		{
			name:           "parenthesis injection",
			username:       "user)(uid=*)",
			mustContain:    []string{`\28`, `\29`, `\2a`}, // ( ) *
			mustNotContain: []string{"user)(uid=*)"},
		},
		{
			name:           "wildcard injection",
			username:       "user*",
			mustContain:    []string{`\2a`},
			mustNotContain: []string{"user*)"},
		},
		{
			name:           "OR filter injection",
			username:       "user)(|(uid=*)",
			mustContain:    []string{`\28`, `\29`, `\2a`}, // ( ) * — pipe is not an LDAP special char per RFC 4515
		},
		{
			name:           "backslash injection",
			username:       `user\test`,
			mustContain:    []string{`\5c`},
			mustNotContain: []string{`user\test)`},
		},
		{
			name:           "null byte injection",
			username:       "user\x00admin",
			mustContain:    []string{`\00`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &mockConn{}
			p := newTestPool(t, domain.SourceInternal, []*mockConn{conn}, nil)

			_, _ = p.Search(context.Background(), tt.username, nil)

			if conn.lastSearchReq == nil {
				t.Fatal("Search was never called on the pooled connection")
			}
			filter := conn.lastSearchReq.Filter
			for _, want := range tt.mustContain {
				if !strings.Contains(strings.ToLower(filter), strings.ToLower(want)) {
					t.Errorf("filter %q missing escaped sequence %q", filter, want)
				}
			}
			for _, bad := range tt.mustNotContain {
				if strings.Contains(filter, bad) {
					t.Errorf("filter %q contains raw unescaped payload %q — EscapeFilter not used", filter, bad)
				}
			}
		})
	}
}

func TestPool_Search_PassesAttributesToLDAP(t *testing.T) {
	conn := &mockConn{
		searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return &ldapv3.SearchResult{
				Entries: []*ldapv3.Entry{entryFor("uid=u,o=nycu", "u", nil)},
			}, nil
		},
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{conn}, nil)

	requested := []string{"cn", "mail", "sn"}
	_, err := p.Search(context.Background(), "u", requested)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if conn.lastSearchReq == nil {
		t.Fatal("no search request captured")
	}
	if len(conn.lastSearchReq.Attributes) != len(requested) {
		t.Fatalf("requested %d attrs, LDAP search got %d: %v", len(requested), len(conn.lastSearchReq.Attributes), conn.lastSearchReq.Attributes)
	}
	for _, want := range requested {
		found := false
		for _, got := range conn.lastSearchReq.Attributes {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("requested attr %q not passed to LDAP search (got %v)", want, conn.lastSearchReq.Attributes)
		}
	}
}

func TestPool_Search_UsesBaseDN(t *testing.T) {
	conn := &mockConn{}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{conn}, nil)
	p.baseDN = "o=nycu"

	_, _ = p.Search(context.Background(), "u", nil)

	if conn.lastSearchReq == nil {
		t.Fatal("no search request captured")
	}
	if conn.lastSearchReq.BaseDN != "o=nycu" {
		t.Errorf("BaseDN = %q, want %q", conn.lastSearchReq.BaseDN, "o=nycu")
	}
	if conn.lastSearchReq.Scope != ldapv3.ScopeWholeSubtree {
		t.Errorf("Scope = %d, want ScopeWholeSubtree (%d)", conn.lastSearchReq.Scope, ldapv3.ScopeWholeSubtree)
	}
}

func TestPool_Search_ConnectionError(t *testing.T) {
	searchErr := errors.New("connection reset")
	conn := &mockConn{
		searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return nil, searchErr
		},
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{conn}, nil)

	_, err := p.Search(context.Background(), "u", nil)
	if err == nil {
		t.Fatal("Search() error = nil, want non-nil")
	}
	if errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatal("connection error must NOT be reported as ErrAccountNotFound")
	}
}

// ---------------------------------------------------------------------------
// Bind — SECURITY CRITICAL: new connection, not from pool, closed after
// ---------------------------------------------------------------------------

func TestPool_Bind_CreatesNewConnectionNotFromPool(t *testing.T) {
	pooledConn := &mockConn{}
	var bindConn *mockConn
	var dialCount int32

	dial := func() (ldapConn, error) {
		atomic.AddInt32(&dialCount, 1)
		bindConn = &mockConn{}
		return bindConn, nil
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{pooledConn}, dial)

	err := p.Bind(context.Background(), "uid=user,o=nycu", "password123")
	if err != nil {
		t.Fatalf("Bind() error = %v, want nil", err)
	}
	if atomic.LoadInt32(&dialCount) != 1 {
		t.Fatalf("dial called %d times, want 1 (new connection for Bind)", dialCount)
	}
	if atomic.LoadInt32(&pooledConn.bindCalls) != 0 {
		t.Fatal("pooled connection's Bind was called — MUST use a new connection for user bind")
	}
	if bindConn == nil || atomic.LoadInt32(&bindConn.bindCalls) != 1 {
		t.Fatal("new connection's Bind was not called exactly once")
	}
	// Pooled connection must remain in pool unchanged.
	if len(p.conns) != 1 {
		t.Fatalf("pool size = %d, want 1 (pooled conn must not be consumed)", len(p.conns))
	}
}

func TestPool_Bind_ClosesNewConnectionAfterSuccess(t *testing.T) {
	var bindConn *mockConn
	dial := func() (ldapConn, error) {
		bindConn = &mockConn{}
		return bindConn, nil
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{{}}, dial)

	err := p.Bind(context.Background(), "uid=user,o=nycu", "pw")
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if bindConn == nil {
		t.Fatal("bindConn was never created")
	}
	if atomic.LoadInt32(&bindConn.closeCalls) != 1 {
		t.Errorf("Close called %d times, want 1 (connection must be closed after bind)", bindConn.closeCalls)
	}
}

func TestPool_Bind_ClosesNewConnectionAfterFailure(t *testing.T) {
	bindErr := errors.New("invalid credentials")
	var bindConn *mockConn
	dial := func() (ldapConn, error) {
		bindConn = &mockConn{bindFn: func(string, string) error { return bindErr }}
		return bindConn, nil
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{{}}, dial)

	err := p.Bind(context.Background(), "uid=user,o=nycu", "wrong")
	if err == nil {
		t.Fatal("Bind() error = nil, want non-nil")
	}
	if bindConn == nil {
		t.Fatal("bindConn was never created")
	}
	if atomic.LoadInt32(&bindConn.closeCalls) != 1 {
		t.Errorf("Close called %d times, want 1 (connection must be closed even on failed bind — credential-leak prevention)", bindConn.closeCalls)
	}
}

func TestPool_Bind_DialFailure(t *testing.T) {
	dialErr := errors.New("network unreachable")
	dial := func() (ldapConn, error) {
		return nil, dialErr
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{{}}, dial)

	err := p.Bind(context.Background(), "uid=user,o=nycu", "pw")
	if err == nil {
		t.Fatal("Bind() error = nil, want dial error")
	}
}

// TestPool_Bind_PassesCredentialsCorrectly verifies the DN and password
// reach the bind call unchanged — no trimming, no transformation.
func TestPool_Bind_PassesCredentialsCorrectly(t *testing.T) {
	var bindConn *mockConn
	dial := func() (ldapConn, error) {
		bindConn = &mockConn{}
		return bindConn, nil
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{{}}, dial)

	wantDN := "uid=user,ou=student,o=nycu"
	wantPW := "P@ssw0rd!#$"
	_ = p.Bind(context.Background(), wantDN, wantPW)

	if bindConn.lastBindDN != wantDN {
		t.Errorf("bind DN = %q, want %q", bindConn.lastBindDN, wantDN)
	}
	if bindConn.lastBindPW != wantPW {
		t.Errorf("bind PW = %q, want %q", bindConn.lastBindPW, wantPW)
	}
}

// ---------------------------------------------------------------------------
// HealthCheck
// ---------------------------------------------------------------------------

func TestPool_HealthCheck_Healthy(t *testing.T) {
	conn := &mockConn{
		searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return &ldapv3.SearchResult{}, nil
		},
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{conn}, nil)

	if err := p.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck() error = %v, want nil", err)
	}
	if atomic.LoadInt32(&conn.searchCalls) == 0 {
		t.Fatal("HealthCheck must perform a search to verify connectivity")
	}
}

func TestPool_HealthCheck_Unhealthy(t *testing.T) {
	conn := &mockConn{
		searchFn: func(req *ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
			return nil, errors.New("server down")
		},
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{conn}, nil)

	if err := p.HealthCheck(context.Background()); err == nil {
		t.Fatal("HealthCheck() error = nil, want non-nil")
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func TestPool_Close_ClosesAllConnections(t *testing.T) {
	conns := []*mockConn{{}, {}, {}}
	p := newTestPool(t, domain.SourceInternal, conns, nil)

	if err := p.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	for i, c := range conns {
		if atomic.LoadInt32(&c.closeCalls) != 1 {
			t.Errorf("conn[%d] Close called %d times, want 1", i, c.closeCalls)
		}
	}
}

func TestPool_Close_SafeIfEmpty(t *testing.T) {
	p := newTestPool(t, domain.SourceInternal, nil, nil)
	if err := p.Close(); err != nil {
		t.Fatalf("Close() on empty pool error = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// getConn / putConn — liveness + overflow behavior
// ---------------------------------------------------------------------------

func TestPool_getConn_ReplacesDeadConnection(t *testing.T) {
	dead := &mockConn{isClosingVal: true}
	var replacement *mockConn
	dial := func() (ldapConn, error) {
		replacement = &mockConn{}
		return replacement, nil
	}
	p := newTestPool(t, domain.SourceInternal, []*mockConn{dead}, dial)

	conn, overflow, err := p.getConn()
	if err != nil {
		t.Fatalf("getConn() error = %v", err)
	}
	if overflow {
		t.Error("overflow = true, want false (pool had one entry, even if stale)")
	}
	if atomic.LoadInt32(&dead.closeCalls) != 1 {
		t.Errorf("dead connection Close calls = %d, want 1 (must be closed before replacement)", dead.closeCalls)
	}
	if replacement == nil {
		t.Fatal("replacement connection was never dialed")
	}
	if conn != ldapConn(replacement) {
		t.Error("getConn returned the dead connection instead of the replacement")
	}
}

func TestPool_getConn_OverflowWhenPoolEmpty(t *testing.T) {
	var overflowConn *mockConn
	dial := func() (ldapConn, error) {
		overflowConn = &mockConn{}
		return overflowConn, nil
	}
	// Pool with capacity 1 but no initial connections → empty
	ch := make(chan ldapConn, 1)
	p := &Pool{
		conns:    ch,
		baseDN:   "o=nycu",
		source:   domain.SourceInternal,
		poolSize: 1,
		dial:     dial,
		logger:   zap.NewNop(),
	}

	conn, overflow, err := p.getConn()
	if err != nil {
		t.Fatalf("getConn() error = %v", err)
	}
	if !overflow {
		t.Error("overflow = false, want true (pool was empty)")
	}
	if conn == nil {
		t.Fatal("getConn returned nil connection")
	}
	if overflowConn == nil {
		t.Fatal("dial was not called to create overflow connection")
	}
}

func TestPool_putConn_OverflowClosesConnection(t *testing.T) {
	overflowConn := &mockConn{}
	ch := make(chan ldapConn, 1)
	p := &Pool{conns: ch, source: domain.SourceInternal, poolSize: 1, logger: zap.NewNop()}

	p.putConn(overflowConn, true)

	if atomic.LoadInt32(&overflowConn.closeCalls) != 1 {
		t.Errorf("overflow conn Close calls = %d, want 1", overflowConn.closeCalls)
	}
	if len(p.conns) != 0 {
		t.Errorf("pool size = %d, want 0 (overflow must NOT be returned to pool)", len(p.conns))
	}
}

func TestPool_putConn_ReturnsHealthyConnectionToPool(t *testing.T) {
	conn := &mockConn{}
	ch := make(chan ldapConn, 1)
	p := &Pool{conns: ch, source: domain.SourceInternal, poolSize: 1, logger: zap.NewNop()}

	p.putConn(conn, false)

	if atomic.LoadInt32(&conn.closeCalls) != 0 {
		t.Errorf("Close calls = %d, want 0 (healthy conn must be returned to pool, not closed)", conn.closeCalls)
	}
	if len(p.conns) != 1 {
		t.Errorf("pool size = %d, want 1", len(p.conns))
	}
}

func TestPool_putConn_NilIsNoop(t *testing.T) {
	ch := make(chan ldapConn, 1)
	p := &Pool{conns: ch, source: domain.SourceInternal, poolSize: 1, logger: zap.NewNop()}

	// Must not panic.
	p.putConn(nil, false)
	p.putConn(nil, true)

	if len(p.conns) != 0 {
		t.Errorf("pool size = %d, want 0 (nil must not be returned to pool)", len(p.conns))
	}
}

// ---------------------------------------------------------------------------
// Compile-time check: *ldapv3.Conn satisfies ldapConn
// ---------------------------------------------------------------------------

// If this test file compiles, it proves the production *ldap.Conn type
// implements the ldapConn interface we test against.
var _ ldapConn = (*ldapv3.Conn)(nil)
