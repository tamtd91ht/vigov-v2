package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
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
// What keeps the exemption safe is that it reads only registry tables — tenant, tenant_domain
// and mini_app (mini_app.go) — none of which holds business data, and every method returns
// exactly one row or none. It never returns a list, so there is no query here that could span
// communes. A commune's own content (ho_so_hien_thi_xa) is NOT read here: that goes through the
// scoped HoSoHienThiStore.
type Directory struct {
	db *sql.DB
}

// NewDirectory is deliberately the only constructor in this package that takes a raw *sql.DB.
// Any second one appearing here is a bug: see the comment on Directory.
func NewDirectory(db *sql.DB) *Directory { return &Directory{db: db} }

// cotXa is the SELECT list every query in this file shares, in the ONE order quetXa scans.
//
// IT IS BUILT IN ONE PLACE, AND THAT IS THE POINT. There are three resolution paths here and
// they used to spell their column list out three times. A column added to two of them and
// forgotten in the third produces NO ERROR ANYWHERE: two paths return the value and one returns
// the zero value, so the province appears in the header on some requests and not on others
// depending on which path answered — and which path answers is invisible from the screen.
// `tinh_thanh` was exactly that column.
//
// A FUNCTION BECAUSE EXACTLY ONE COLUMN DIFFERS between the paths: the two Host lookups join a
// tenant_domain row that must exist, ByID left-joins one that need not. That single expression is
// the only parameter; everything else is fixed here.
func cotXa(host string) string {
	return `t.id, ` + host + `, t.ten, t.tinh_thanh, t.dang_hoat_dong`
}

// The two statements. ByHost and ByHostErr share ONE string rather than two identical ones: they
// answer the same question and differ only in what they do with a failure, so a WHERE clause that
// drifted between them would make the operator-facing variant disagree with the hot path.
var (
	truyVanTheoHost = `
		SELECT ` + cotXa("d.host") + `
		FROM tenant_domain d
		JOIN tenant t ON t.id = d.tenant_id
		WHERE d.host = $1`

	// The canonical host, so a caller building a link picks the same address every time; a
	// commune may hold several hosts after a merger. LEFT JOIN because a commune that exists
	// with no domain yet is a configuration state, not a missing commune — reporting "no such
	// commune" for it would send the operator hunting the wrong table.
	truyVanTheoID = `
		SELECT ` + cotXa("COALESCE(d.host, '')") + `
		FROM tenant t
		LEFT JOIN tenant_domain d ON d.tenant_id = t.id AND d.la_chinh
		WHERE t.id = $1`
)

// hangDoc is what both *sql.Row and *sql.Rows satisfy, so quetXa has one caller shape.
type hangDoc interface{ Scan(dest ...any) error }

// quetXa reads one row in the order cotXa names. POSITIONAL — this list and cotXa move together,
// and they are next to each other for that reason.
//
// It returns the driver's error unchanged: each caller below tells sql.ErrNoRows apart from a
// real failure differently, and collapsing the two here is precisely the mistake ByHostErr exists
// to undo.
func quetXa(hang hangDoc) (tenant.Tenant, error) {
	var out tenant.Tenant
	var id string
	if err := hang.Scan(&id, &out.Host, &out.Name, &out.Province, &out.Active); err != nil {
		return tenant.Tenant{}, err
	}
	out.ID = tenant.ID(id)
	return out, nil
}

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

	out, err := quetXa(d.db.QueryRowContext(ctx, truyVanTheoHost, h))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return tenant.Tenant{}, false
	case err != nil:
		// A database failure is NOT "commune not found". Returning false here would turn an
		// outage into a 404 and send everyone hunting a configuration problem that does not
		// exist. The caller logs this and returns 503.
		return tenant.Tenant{}, false
	}

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

	out, err := quetXa(d.db.QueryRowContext(ctx, truyVanTheoHost, h))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return tenant.Tenant{}, ErrKhongCoXa
	case err != nil:
		return tenant.Tenant{}, fmt.Errorf("directory: truy vấn host %q: %w", h, err)
	}
	if !out.Active {
		return tenant.Tenant{}, ErrXaNgungHoatDong
	}
	return out, nil
}

// ByID reads one commune by its opaque identifier.
//
// WHY IT DOES NOT REFUSE A DEACTIVATED COMMUNE, unlike ByHostErr: the two answer different
// questions. ByHostErr answers "may this request be served", and a merged commune must stop
// serving. ByID answers "what is this commune", and a merged commune still has to be
// describable — rule 7 keeps its data and its codes, and something has to be able to render
// the name attached to an archival record. The caller reads Active and decides; that is
// exactly why the field exists rather than being folded into an error.
//
// It returns exactly one commune or an error. Like the rest of this type it never returns a
// list, so there is no query here that could span communes.
func (d *Directory) ByID(ctx context.Context, id tenant.ID) (tenant.Tenant, error) {
	if !id.Valid() {
		return tenant.Tenant{}, fmt.Errorf("directory: %w", domain.ErrIDKhongHopLe)
	}

	out, err := quetXa(d.db.QueryRowContext(ctx, truyVanTheoID, id.String()))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return tenant.Tenant{}, ErrKhongCoXa
	case err != nil:
		return tenant.Tenant{}, fmt.Errorf("directory: truy vấn xã %q: %w", id, err)
	}
	return out, nil
}

var (
	ErrKhongCoXa       = errors.New("directory: không có xã nào ứng với host này")
	ErrXaNgungHoatDong = errors.New("directory: xã đã ngừng hoạt động")
)
