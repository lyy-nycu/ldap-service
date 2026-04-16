package ldap

import (
	"context"

	"github.com/nycuitsc/ldap-service/internal/domain"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Pool — single LDAP server connection pool
// ---------------------------------------------------------------------------

// Pool manages a connection pool to a single LDAP server.
// Each LDAP source (internal, external) gets its own Pool instance.
//
// Internal design (for implementer):
//   - Use a buffered channel of *ldap.Conn as the pool
//   - Store: host, port, useTLS, bindDN, bindPW, baseDN, source label, logger
//   - Channel capacity = configured pool size
type Pool struct {
	// TODO(copilot): add fields
	//   - conns    chan *ldap.Conn  (buffered channel, capacity = pool size)
	//   - host     string
	//   - port     int
	//   - useTLS   bool
	//   - bindDN   string
	//   - bindPW   string
	//   - baseDN   string
	//   - source   string          (domain.SourceInternal or domain.SourceExternal)
	//   - logger   *zap.Logger
}

// NewPool creates a Pool and initializes connections.
//
// Acceptance criteria:
//   - MUST dial `poolSize` connections to host:port
//   - If useTLS is true, MUST use LDAPS (crypto/tls)
//   - MUST bind each connection with bindDN/bindPW (read-only account)
//   - MUST return error if any connection fails to dial or bind
//   - MUST store baseDN and source label for use in Search
func NewPool(host string, port int, useTLS bool, bindDN, bindPW, baseDN, source string, poolSize int, logger *zap.Logger) (*Pool, error) {
	panic("not implemented")
}

// getConn borrows a connection from the pool.
//
// Acceptance criteria:
//   - MUST check connection liveness (simple search) before returning
//   - If connection is dead, MUST close it, create a new one, bind, and return the new one
//   - If pool is empty (all connections in use), MUST create a temporary overflow connection
//   - Overflow connections are NOT returned to the pool (caller must close them)
func (p *Pool) getConn() (conn interface{}, overflow bool, err error) {
	panic("not implemented")
}

// putConn returns a connection to the pool.
//
// Acceptance criteria:
//   - If overflow is true, MUST close the connection instead of returning to pool
//   - If pool channel is full, MUST close the connection
func (p *Pool) putConn(conn interface{}, overflow bool) {
	panic("not implemented")
}

// Search finds an account by username on this LDAP server.
// See domain.LDAPPool.Search for acceptance criteria.
func (p *Pool) Search(ctx context.Context, username string, attributes []string) (*domain.Account, error) {
	panic("not implemented")
}

// Bind attempts authentication with the given DN and password.
// See domain.LDAPPool.Bind for acceptance criteria.
func (p *Pool) Bind(ctx context.Context, dn string, password string) error {
	panic("not implemented")
}

// HealthCheck verifies that this LDAP server is reachable.
// See domain.LDAPPool.HealthCheck for acceptance criteria.
func (p *Pool) HealthCheck(ctx context.Context) error {
	panic("not implemented")
}

// Close drains and closes all connections in this pool.
// See domain.LDAPPool.Close for acceptance criteria.
func (p *Pool) Close() error {
	panic("not implemented")
}

// Compile-time interface check.
var _ domain.LDAPPool = (*Pool)(nil)
