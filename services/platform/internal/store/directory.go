package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/pkg/tenant"
	"github.com/vihat/vigov/services/platform/internal/domain"
)

// Directory resolves a Host to a commune.
//
// THIS IS THE ONE SANCTIONED UNSCOPED READER IN THE SYSTEM, and it needs stating plainly
// because everything else in pkg/store exists to make unscoped reads impossible.
//
// It is exempt for a reason that cannot be designed away: this lookup is what ESTABLISHES the
// commune. Scoping it would require knowing the commune in order to find the commune. Every
// other repository is built from *store.Scoped and inherits WHERE tenant_id = $1; this one
// holds a raw handle instead.
//
// What keeps the exemption safe is that it reads exactly two tables — tenant and
// tenant_domain — neither of which holds business data, and it returns exactly one commune or
// none. It never returns a list, so there is no query here that could span communes.
type Directory struct {
	db *sql.DB
}

// NewDirectory is deliberately the only constructor in this package that takes a raw *sql.DB.
// Any second one appearing here is a bug: see the comment on Directory.
func NewDirectory(db *sql.DB) *Directory { return &Directory{db: db} }

// ByHost implements tenant.Directory.
//
// A Host matching nothing returns ok=false, and the caller turns that into 404 — never into a
// fallback commune. "Fail closed" is the whole isolation model: a default here would serve one
// commune's data under another commune's address, silently, with every test still green.
func (d *Directory) ByHost(ctx context.Context, host string) (tenant.Tenant, bool) {
	h, err := domain.NormaliseHost(host)
	if err != nil {
		return tenant.Tenant{}, false
	}

	const q = `
		SELECT t.id, d.host, t.ten, t.dang_hoat_dong
		FROM tenant_domain d
		JOIN tenant t ON t.id = d.tenant_id
		WHERE d.host = $1`

	var out tenant.Tenant
	var id string
	err = d.db.QueryRowContext(ctx, q, h).Scan(&id, &out.Host, &out.Name, &out.Active)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return tenant.Tenant{}, false
	case err != nil:
		// A database failure is NOT "commune not found". Returning false here would turn an
		// outage into a 404 and send everyone hunting a configuration problem that does not
		// exist. The caller logs this and returns 503.
		return tenant.Tenant{}, false
	}
	out.ID = tenant.ID(id)

	// A deactivated commune (merged, dissolved) keeps its data and its address by rule 7, but
	// stops serving requests. Its rows are never discarded; they are simply no longer reachable
	// through this host.
	if !out.Active {
		return tenant.Tenant{}, false
	}
	return out, true
}

// ByHostErr is ByHost with the failure reason preserved.
//
// WHY BOTH EXIST: tenant.Directory is the interface every service depends on, and it returns
// only (Tenant, bool) — that is right for the hot path, where the edge needs a yes or no. But
// collapsing "no such commune" and "the database is down" into one false is how an outage gets
// misdiagnosed as a misconfigured domain. The platform service's own admin paths use this
// variant so the operator sees which one actually happened.
func (d *Directory) ByHostErr(ctx context.Context, host string) (tenant.Tenant, error) {
	h, err := domain.NormaliseHost(host)
	if err != nil {
		return tenant.Tenant{}, fmt.Errorf("directory: %w", err)
	}

	const q = `
		SELECT t.id, d.host, t.ten, t.dang_hoat_dong
		FROM tenant_domain d
		JOIN tenant t ON t.id = d.tenant_id
		WHERE d.host = $1`

	var out tenant.Tenant
	var id string
	err = d.db.QueryRowContext(ctx, q, h).Scan(&id, &out.Host, &out.Name, &out.Active)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return tenant.Tenant{}, ErrKhongCoXa
	case err != nil:
		return tenant.Tenant{}, fmt.Errorf("directory: truy vấn host %q: %w", h, err)
	}
	out.ID = tenant.ID(id)
	if !out.Active {
		return tenant.Tenant{}, ErrXaNgungHoatDong
	}
	return out, nil
}

var (
	ErrKhongCoXa       = errors.New("directory: không có xã nào ứng với host này")
	ErrXaNgungHoatDong = errors.New("directory: xã đã ngừng hoạt động")
)
