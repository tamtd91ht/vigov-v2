package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// OperatorRegistry is the operator console's READ side of the commune registry (ADR 0048 §01/10 #4).
//
// A RAW HANDLE, LIKE Directory, FOR THE SAME REASON: the registry tables define what a commune is.
// The list below spans every commune by design and carries ONLY registry metadata — id, name,
// province, state, hosts (ADR 0003; ADR 0048 §30/09 #5). There is no path from here to a commune's
// business data: this service owns none.
type OperatorRegistry struct {
	db *sql.DB
}

// NewOperatorRegistry is the third raw-handle constructor of this package (Directory and
// UploadPolicyStore are the others). Its tables are tenant, tenant_domain, mini_app and tinh_thanh —
// the registry, never a commune's content.
func NewOperatorRegistry(db *sql.DB) *OperatorRegistry { return &OperatorRegistry{db: db} }

// CommuneOrder is the one sort the commune list offers: creation time, ULID as the tie-break.
// Sorting by name would put a Vietnamese collation in the ORDER BY, which differs between database
// installations (the tinh_thanh migration's argument).
var CommuneOrder = page.NewAllowlist(page.Asc, page.Col("created_at", "tao_luc", page.KindTime))

// domainsAgg lists a commune's hosts, primary first. string_agg, not array_agg: a host is LDH-only
// (domain.ParseCommuneHost, migration 0001's lower-case CHECK), so it never contains a comma, and
// a text column scans through database/sql without a driver-specific array type.
const domainsAgg = `COALESCE(string_agg(d.host, ',' ORDER BY d.la_chinh DESC, d.host), '')`

func splitDomains(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

// ListCommunes is one page of every commune.
//
// NOT AUDITED, by the user's decision of 2026-09-30 (ADR 0048 §30/09 #5): an explicit, bounded
// exception to rule 6 invariant 7 that covers ONLY registry metadata. It still carries the mark.
func (r *OperatorRegistry) ListCommunes(ctx context.Context, req page.Request) (page.Result[domain.Commune], error) {
	cmp, ord := ">", "ASC"
	if req.Dir() == page.Desc {
		cmp, ord = "<", "DESC"
	}
	var (
		afterAt any
		afterID string
	)
	if a, ok := req.After(); ok {
		afterAt, afterID = a.Key.Time(), a.ID
	}
	q := `SELECT t.id, t.ten, t.tinh_thanh, t.dang_hoat_dong, t.tao_luc, ` + domainsAgg + `
		FROM tenant t LEFT JOIN tenant_domain d ON d.tenant_id = t.id
		WHERE ($1::timestamptz IS NULL OR (t.tao_luc, t.id) ` + cmp + ` ($1::timestamptz, $2))
		GROUP BY t.id
		ORDER BY t.tao_luc ` + ord + `, t.id ` + ord + `
		LIMIT $3`
	// @cross-tenant: the operator console's commune list — registry metadata of every commune
	// (name, province, state, hosts), unaudited by the user's decision of 2026-09-30 (ADR 0048 §30/09 #5).
	rows, err := r.db.QueryContext(ctx, q, afterAt, afterID, req.Limit()+1)
	if err != nil {
		return page.Result[domain.Commune]{}, fmt.Errorf("operator registry: list communes: %w", err)
	}
	defer rows.Close()

	out := page.NewResult[domain.Commune]()
	for rows.Next() {
		var (
			c       domain.Commune
			domains string
		)
		if err := rows.Scan(&c.ID, &c.Name, &c.Province, &c.Active, &c.CreatedAt, &domains); err != nil {
			return page.Result[domain.Commune]{}, fmt.Errorf("operator registry: scan commune: %w", err)
		}
		c.Domains = splitDomains(domains)
		out.Items = append(out.Items, c)
	}
	if err := rows.Err(); err != nil {
		return page.Result[domain.Commune]{}, fmt.Errorf("operator registry: list communes: %w", err)
	}
	if len(out.Items) > req.Limit() {
		out.Items = out.Items[:req.Limit()]
		last := out.Items[len(out.Items)-1]
		out.HasMore = true
		out.NextCursor = page.Encode(req.Column(), req.Dir(),
			page.Anchor{Key: page.TimeKey(last.CreatedAt), ID: last.ID})
	}
	return out, nil
}

// ErrCommuneNotFound — no commune with this id.
var ErrCommuneNotFound = errors.New("operator registry: không có xã với id này")

// Commune reads one commune and its dedicated mini_app rows. The id comes from the path, already
// checked to be ULID-shaped by the caller.
func (r *OperatorRegistry) Commune(ctx context.Context, id string) (domain.Commune, []domain.CommuneMiniApp, error) {
	const q = `SELECT t.id, t.ten, t.tinh_thanh, t.dang_hoat_dong, t.tao_luc, ` + domainsAgg + `
		FROM tenant t LEFT JOIN tenant_domain d ON d.tenant_id = t.id
		WHERE t.id = $1
		GROUP BY t.id`
	var (
		c       domain.Commune
		domains string
	)
	err := r.db.QueryRowContext(ctx, q, id).
		Scan(&c.ID, &c.Name, &c.Province, &c.Active, &c.CreatedAt, &domains)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return domain.Commune{}, nil, ErrCommuneNotFound
	case err != nil:
		return domain.Commune{}, nil, fmt.Errorf("operator registry: read commune: %w", err)
	}
	c.Domains = splitDomains(domains)

	// Soft-deleted rows are excluded (rule 7 invariant 2). The tenant_id filter is the commune of
	// the path — one commune's rows, never a list across communes.
	const qApps = `SELECT app_id, che_do, dang_hoat_dong, tao_luc, tao_boi
		FROM mini_app WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY tao_luc, app_id`
	rows, err := r.db.QueryContext(ctx, qApps, id)
	if err != nil {
		return domain.Commune{}, nil, fmt.Errorf("operator registry: read mini apps: %w", err)
	}
	defer rows.Close()
	apps := []domain.CommuneMiniApp{}
	for rows.Next() {
		var (
			a    domain.CommuneMiniApp
			mode string
		)
		if err := rows.Scan(&a.AppID, &mode, &a.Active, &a.CreatedAt, &a.CreatedBy); err != nil {
			return domain.Commune{}, nil, fmt.Errorf("operator registry: scan mini app: %w", err)
		}
		a.Mode = domain.CheDoMiniApp(mode)
		apps = append(apps, a)
	}
	if err := rows.Err(); err != nil {
		return domain.Commune{}, nil, fmt.Errorf("operator registry: read mini apps: %w", err)
	}
	return c, apps, nil
}

// HeldMiniApp answers whether appID is, or WAS, a dedicated (`rieng`) App ID of commune id —
// soft-deleted rows INCLUDED, on purpose and only here. Its one use is retiring a secret left live
// under an App ID the commune no longer holds (an automatic retirement after "Đổi App ID" / "Gỡ khỏi
// xã" that failed): Commune hides that row (rule 7 invariant 2), yet the secret under it is exactly
// what the operator must still be able to end. Nothing else is read from the deleted row.
func (r *OperatorRegistry) HeldMiniApp(ctx context.Context, id, appID string) (bool, error) {
	var one int
	// The tenant_id filter is the commune of the path — one commune's history, never another's.
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM mini_app WHERE tenant_id = $1 AND app_id = $2 AND che_do = 'rieng'`, id, appID).Scan(&one)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("operator registry: held mini app: %w", err)
	}
	return true, nil
}

// Provinces is the active tinh_thanh catalogue in its fixed display order. Bounded by nature (34 rows
// after the 2025 reorganisation), so it is returned whole, not paginated.
func (r *OperatorRegistry) Provinces(ctx context.Context) ([]domain.Province, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, ten FROM tinh_thanh WHERE dang_hoat_dong ORDER BY thu_tu, ten`)
	if err != nil {
		return nil, fmt.Errorf("operator registry: provinces: %w", err)
	}
	defer rows.Close()
	out := []domain.Province{}
	for rows.Next() {
		var p domain.Province
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, fmt.Errorf("operator registry: scan province: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("operator registry: provinces: %w", err)
	}
	return out, nil
}
