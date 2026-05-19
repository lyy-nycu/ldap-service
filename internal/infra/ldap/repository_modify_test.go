package ldap

import (
	"context"
	"errors"
	"testing"

	"github.com/nycuitsc/ldap-service/internal/domain"
)

// Repository.Modify — locked fan-out tests.

func TestRepository_Modify_InternalHit(t *testing.T) {
	internal := &mockPool{
		modifyFn: func(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error {
			return nil
		},
	}
	external := &mockPool{}
	repo, _ := newTestRepo(t, internal, external)

	err := repo.Modify(context.Background(), "0856001", []domain.ModifyAttr{
		{Name: "disable", Value: "0"},
	})
	if err != nil {
		t.Fatalf("Modify() err = %v, want nil", err)
	}
	if internal.modifyCalls != 1 {
		t.Errorf("internal.Modify called %d times, want 1", internal.modifyCalls)
	}
	if external.modifyCalls != 0 {
		t.Errorf("external.Modify called %d times, want 0 (no fan-out needed when internal succeeds)", external.modifyCalls)
	}
}

func TestRepository_Modify_InternalNotFound_FallsOverToExternal(t *testing.T) {
	internal := &mockPool{
		modifyFn: func(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error {
			return domain.ErrAccountNotFound
		},
	}
	external := &mockPool{
		modifyFn: func(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error {
			return nil
		},
	}
	repo, _ := newTestRepo(t, internal, external)

	err := repo.Modify(context.Background(), "alumni@example.com", []domain.ModifyAttr{
		{Name: "userpassword", Value: "{SSHA}xyz"},
	})
	if err != nil {
		t.Fatalf("Modify() err = %v, want nil", err)
	}
	if external.modifyCalls != 1 {
		t.Errorf("external.Modify must be called when internal returns NotFound; got %d", external.modifyCalls)
	}
}

func TestRepository_Modify_BothNotFound(t *testing.T) {
	internal := &mockPool{
		modifyFn: func(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error {
			return domain.ErrAccountNotFound
		},
	}
	external := &mockPool{
		modifyFn: func(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error {
			return domain.ErrAccountNotFound
		},
	}
	repo, _ := newTestRepo(t, internal, external)

	err := repo.Modify(context.Background(), "ghost", []domain.ModifyAttr{
		{Name: "disable", Value: "0"},
	})
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("Modify() err = %v, want ErrAccountNotFound", err)
	}
}

func TestRepository_Modify_SchemaViolationIsAuthoritative_NoFanover(t *testing.T) {
	// Schema rejection from internal must NOT cause a retry on external.
	// The schema is per-server, but legacy production semantics treat
	// 409 as a stop sign — the consumer expects to surface it directly.
	internal := &mockPool{
		modifyFn: func(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error {
			return domain.ErrSchemaViolation
		},
	}
	external := &mockPool{
		modifyFn: func(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error {
			t.Fatal("external.Modify must NOT be called on schema violation")
			return nil
		},
	}
	repo, _ := newTestRepo(t, internal, external)

	err := repo.Modify(context.Background(), "0856001", []domain.ModifyAttr{
		{Name: "userpassword", Value: "{SSHA}reused"},
	})
	if !errors.Is(err, domain.ErrSchemaViolation) {
		t.Fatalf("Modify() err = %v, want ErrSchemaViolation", err)
	}
}

func TestRepository_Modify_BothConnectionErrors_ReturnsServiceUnavailable(t *testing.T) {
	connErr := errors.New("dial tcp: connection refused")
	internal := &mockPool{
		modifyFn: func(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error {
			return connErr
		},
	}
	external := &mockPool{
		modifyFn: func(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error {
			return connErr
		},
	}
	repo, _ := newTestRepo(t, internal, external)

	err := repo.Modify(context.Background(), "0856001", []domain.ModifyAttr{
		{Name: "disable", Value: "0"},
	})
	if !errors.Is(err, domain.ErrServiceUnavailable) {
		t.Fatalf("Modify() err = %v, want ErrServiceUnavailable", err)
	}
}

// TestRepository_Modify_InternalTransportError_FallsOverToExternal locks
// in the documented partial-outage invariant: if the internal pool
// returns a transport error (anything that is not ErrAccountNotFound and
// not ErrSchemaViolation) but the external pool succeeds, the request
// must succeed. A green implementation that short-circuits to
// ErrServiceUnavailable on the first transport failure would pass every
// other fan-out test in this file but silently break the most common
// real-world degraded-mode scenario.
func TestRepository_Modify_InternalTransportError_FallsOverToExternal(t *testing.T) {
	internal := &mockPool{
		modifyFn: func(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error {
			return errors.New("dial tcp: i/o timeout")
		},
	}
	external := &mockPool{
		modifyFn: func(ctx context.Context, subjectID string, attrs []domain.ModifyAttr) error {
			return nil
		},
	}
	repo, _ := newTestRepo(t, internal, external)

	err := repo.Modify(context.Background(), "alumni@example.com", []domain.ModifyAttr{
		{Name: "userpassword", Value: "{SSHA}xyz"},
	})
	if err != nil {
		t.Fatalf("Modify() err = %v, want nil (external must absorb internal transport failure)", err)
	}
	if internal.modifyCalls != 1 {
		t.Errorf("internal.Modify called %d times, want 1", internal.modifyCalls)
	}
	if external.modifyCalls != 1 {
		t.Errorf("external.Modify called %d times, want 1 (transport error MUST fan-over)", external.modifyCalls)
	}
}
