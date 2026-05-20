package ldap

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strings"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/nycuitsc/ldap-service/internal/domain"
	"go.uber.org/zap"
)

// ldapTLSInsecureSkipVerify reports whether self-signed LDAPS certs should be
// accepted. Controlled by the LDAP_TLS_INSECURE_SKIP_VERIFY env var. This is a
// DEV/TEST-ONLY escape hatch for the docker-compose fixture which uses
// auto-generated self-signed certs. NEVER enable in production.
func ldapTLSInsecureSkipVerify() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("LDAP_TLS_INSECURE_SKIP_VERIFY")))
	return v == "true" || v == "1" || v == "yes"
}

// ---------------------------------------------------------------------------
// Internal interfaces — for testability
// ---------------------------------------------------------------------------

// ldapConn wraps the methods used from *ldap.Conn so Pool can be unit-tested
// with a mock. Production code uses *ldap.Conn directly (it satisfies this
// interface). Tests inject a mock implementation via dialFn.
type ldapConn interface {
	Search(searchRequest *ldapv3.SearchRequest) (*ldapv3.SearchResult, error)
	Bind(username, password string) error
	Modify(modifyRequest *ldapv3.ModifyRequest) error
	Close() error
	IsClosing() bool
}

// dialFn creates a new LDAP connection bound with the pool's read-only
// credentials. Production wraps ldap.DialTLS/ldap.Dial + Bind; tests inject
// a mock that returns a mock ldapConn.
type dialFn func() (ldapConn, error)

// ---------------------------------------------------------------------------
// Pool — single LDAP server connection pool
// ---------------------------------------------------------------------------

// Pool manages a connection pool to a single LDAP server.
// Each LDAP source (internal, external) gets its own Pool instance.
type Pool struct {
	conns    chan ldapConn
	host     string
	port     int
	useTLS   bool
	bindDN   string
	bindPW   string
	baseDN   string
	source   string // domain.SourceInternal or domain.SourceExternal
	poolSize int
	dial     dialFn // used for new connections (pool init, overflow, Bind, stale replacement)
	logger   *zap.Logger
}

// NewPool creates a Pool and initializes connections.
//
// Acceptance criteria:
//   - MUST dial `poolSize` connections to host:port
//   - If useTLS is true, MUST use LDAPS (ldap.DialTLS with tls.Config{MinVersion: tls.VersionTLS12})
//   - MUST bind each connection with bindDN/bindPW (read-only account)
//   - MUST return error if any connection fails to dial or bind
//   - MUST store baseDN and source label for use in Search
//   - Implementation hint: set p.dial to a closure that dials+binds, then call p.dial() in a loop
func NewPool(host string, port int, useTLS bool, bindDN, bindPW, baseDN, source string, poolSize int, logger *zap.Logger) (*Pool, error) {
	if poolSize <= 0 {
		return nil, fmt.Errorf("invalid pool size")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	p := &Pool{
		conns:    make(chan ldapConn, poolSize),
		host:     host,
		port:     port,
		useTLS:   useTLS,
		bindDN:   bindDN,
		bindPW:   bindPW,
		baseDN:   baseDN,
		source:   source,
		poolSize: poolSize,
		logger:   logger,
	}

	p.dial = func() (ldapConn, error) {
		scheme := "ldap"
		if p.useTLS {
			scheme = "ldaps"
		}

		addr := fmt.Sprintf("%s://%s:%d", scheme, p.host, p.port)

		var (
			conn *ldapv3.Conn
			err  error
		)
		if p.useTLS {
			tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: p.host}
			if ldapTLSInsecureSkipVerify() {
				tlsCfg.InsecureSkipVerify = true
				p.logger.Warn("ldap TLS certificate verification disabled (LDAP_TLS_INSECURE_SKIP_VERIFY) — never use in production",
					zap.String("source", p.source))
			}
			conn, err = ldapv3.DialURL(addr, ldapv3.DialWithTLSConfig(tlsCfg))
		} else {
			conn, err = ldapv3.DialURL(addr)
		}
		if err != nil {
			return nil, err
		}

		if err := conn.Bind(p.bindDN, p.bindPW); err != nil {
			_ = conn.Close()
			return nil, err
		}

		return conn, nil
	}

	for i := 0; i < poolSize; i++ {
		conn, err := p.dial()
		if err != nil {
			_ = p.Close()
			return nil, err
		}
		p.conns <- conn
	}

	return p, nil
}

// getConn borrows a connection from the pool.
//
// Acceptance criteria:
//   - MUST check connection liveness via IsClosing() before returning
//   - If connection is closing/dead, MUST close it, create a new one via p.dial(), and return the new one
//   - If pool is empty (all connections in use), MUST create a temporary overflow connection via p.dial()
//   - Overflow connections are NOT returned to the pool (caller must pass overflow=true to putConn)
func (p *Pool) getConn() (conn ldapConn, overflow bool, err error) {
	select {
	case conn = <-p.conns:
		if conn == nil || conn.IsClosing() {
			if conn != nil {
				_ = conn.Close()
			}
			conn, err = p.dial()
			if err != nil {
				return nil, false, err
			}
			return conn, false, nil
		}
		return conn, false, nil
	default:
		conn, err = p.dial()
		if err != nil {
			return nil, true, err
		}
		return conn, true, nil
	}
}

// putConn returns a connection to the pool.
//
// Acceptance criteria:
//   - If overflow is true, MUST close the connection instead of returning to pool
//   - If pool channel is full (should not normally happen), MUST close the connection
//   - If connection is nil, MUST be a no-op
func (p *Pool) putConn(conn ldapConn, overflow bool) {
	if conn == nil {
		return
	}
	if overflow {
		_ = conn.Close()
		return
	}

	select {
	case p.conns <- conn:
	default:
		_ = conn.Close()
	}
}

// Search finds an account by username on this LDAP server.
// See domain.LDAPPool.Search for acceptance criteria.
//
// Implementation requirements:
//   - MUST use ldapv3.EscapeFilter(username) when building the filter — never string concatenation
//   - Filter template: fmt.Sprintf("(cn=%s)", ldapv3.EscapeFilter(username))
//   - Search base = p.baseDN, scope = ldapv3.ScopeWholeSubtree
//   - Request only the attributes passed in (no wildcard)
//   - On len(result.Entries) == 0, return domain.ErrAccountNotFound
//   - Set Account.Source = p.source
//   - Use getConn/putConn to borrow/return the pool connection
func (p *Pool) Search(ctx context.Context, username string, attributes []string) (*domain.Account, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	conn, overflow, err := p.getConn()
	if err != nil {
		return nil, err
	}
	defer p.putConn(conn, overflow)

	filter := fmt.Sprintf("(cn=%s)", ldapv3.EscapeFilter(username))
	searchReq := ldapv3.NewSearchRequest(
		p.baseDN,
		ldapv3.ScopeWholeSubtree,
		ldapv3.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		attributes,
		nil,
	)

	result, err := conn.Search(searchReq)
	if err != nil {
		return nil, err
	}
	if len(result.Entries) == 0 {
		return nil, domain.ErrAccountNotFound
	}

	entry := result.Entries[0]
	attrMap := make(map[string]string)
	for _, attr := range attributes {
		val := entry.GetAttributeValue(attr)
		if val != "" {
			attrMap[attr] = val
		}
	}

	return &domain.Account{
		DN:         entry.DN,
		UID:        entry.GetAttributeValue("uid"),
		Attributes: attrMap,
		Source:     p.source,
	}, nil
}

// Bind attempts authentication with the given DN and password.
// See domain.LDAPPool.Bind for acceptance criteria.
//
// Implementation requirements:
//   - MUST create a NEW connection via p.dial() — do NOT use getConn()
//   - MUST call conn.Close() after the bind attempt (success OR failure) via defer
//   - MUST NOT log the password
//   - Return nil on successful bind, error on failure
func (p *Pool) Bind(ctx context.Context, dn string, password string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	conn, err := p.dial()
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	if err := ctx.Err(); err != nil {
		return err
	}

	return conn.Bind(dn, password)
}

// HealthCheck verifies that this LDAP server is reachable.
// See domain.LDAPPool.HealthCheck for acceptance criteria.
//
// Implementation requirements:
//   - MUST borrow a connection via getConn()
//   - MUST perform a lightweight search (e.g., BaseObject scope on baseDN with filter "(objectClass=*)")
//   - MUST return the connection via putConn() in defer
//   - Return nil if healthy, error if unreachable
func (p *Pool) HealthCheck(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	conn, overflow, err := p.getConn()
	if err != nil {
		return err
	}
	defer p.putConn(conn, overflow)

	searchReq := ldapv3.NewSearchRequest(
		p.baseDN,
		ldapv3.ScopeBaseObject,
		ldapv3.NeverDerefAliases,
		1,
		0,
		false,
		"(objectClass=*)",
		[]string{"dn"},
		nil,
	)

	_, err = conn.Search(searchReq)
	return err
}

// Close drains and closes all connections in this pool.
// See domain.LDAPPool.Close for acceptance criteria.
//
// Implementation requirements:
//   - MUST close all connections currently in the pool channel
//   - MUST be safe to call during graceful shutdown
//   - Subsequent calls to Close should not panic
func (p *Pool) Close() error {
	if p == nil || p.conns == nil {
		return nil
	}

	for {
		select {
		case conn := <-p.conns:
			if conn != nil {
				_ = conn.Close()
			}
		default:
			return nil
		}
	}
}

// Compile-time interface check.
var _ domain.LDAPPool = (*Pool)(nil)
