package store

// The commune's map asset register — migrations/0015_map_asset.sql (ADR 0072).
//
// FOUR THINGS HOLD ACROSS EVERY METHOD, the same four as external_contact.go:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from Scoped / tx.TenantID(), never a parameter.
//  2. `tenant_id`, `id`, `created_at` AND `created_by` APPEAR IN NO UPDATE; the guard trigger refuses
//     a change too, and their absence here keeps that floor unreachable from this service.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS (rule 7, invariant 2) — points, list, detail, summary and
//     the tax-code check alike. The one lookup that does not is the list cursor's anchor (QueryPage's
//     KindRef argument: an anchor deleted between two pages still marks where the walk stopped).
//  4. NOTHING HERE OPENS A TRANSACTION. The use case opens it and writes the audit entry inside.
//
// PERSONAL DATA (0015 §PERSONAL DATA): nothing here logs a row or puts a value into an error message,
// and the search term `q` is a bound parameter, never part of the statement text.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// MapAssetStore reads and writes map_asset. Built from *store.DB and reaching the database only
// through Scoped / ScopedTx (rule 1, invariant 5).
type MapAssetStore struct {
	db *store.DB
}

func NewMapAssetStore(db *store.DB) *MapAssetStore {
	return &MapAssetStore{db: db}
}

// MapAssetPointsCeiling bounds GET /api/v1/map-asset-points. The map draws every point of the filter
// at once (browser-side clustering, ADR 0072 §4), so the bound cannot be a page; past it the route
// REFUSES rather than truncating — a silently missing pin is a place nobody knows is missing.
//
// 5000 is the register's "few thousand per commune at most" (0015, question 1) with headroom; a
// commune past it narrows the filter (by group) and is told so.
const MapAssetPointsCeiling = 5000

// MapAssetListSort is the ONE order of the `Sổ địa điểm` table (spec §7: grouped by type): group, then
// name, then id as the tie-break. KindRef because the name of a household business is often its
// owner's name, and a cursor carrying it would put personal data in a URL (rule 3, forbidden #4) — the
// cursor carries only the anchor row's id and the key is looked up server-side.
var MapAssetListSort = page.NewAllowlist(page.Asc,
	page.Col("asset_type_code", "asset_type_code", page.KindRef))

var (
	// ErrMapAssetNotFound — no LIVE row with that id in THIS commune: one answer for "another commune's
	// row", "deleted" and "no such row" (rule 4, forbidden #2 on the commune axis).
	ErrMapAssetNotFound = errors.New("map_asset: không tồn tại trong xã này")

	// ErrMapAssetTypeUnavailable — the type code is not a live, in-use row of THIS commune's catalogue.
	ErrMapAssetTypeUnavailable = errors.New("map_asset: nhóm tài nguyên không có hoặc đã tắt trong danh mục của xã")

	// ErrMapAssetTaxCodeTaken — another LIVE asset of this commune holds the tax code (0015's
	// UNIQUE (tenant_id, live_tax_code)).
	ErrMapAssetTaxCodeTaken = errors.New("map_asset: mã số thuế đã có ở một đối tượng khác của xã")

	// ErrTooManyMapAssetPoints — the filtered set exceeds MapAssetPointsCeiling. Refused, never cut.
	ErrTooManyMapAssetPoints = errors.New("map_asset: vượt trần số điểm trên bản đồ")
)

// mapAssetFilterSQL turns a NORMALISED filter into the tail of a statement whose $1 is the commune
// (tenant_id, bound by Scoped.Query), with placeholders from $next on. One table, no join.
//
// THE VALUES ARE ALWAYS BOUND, NEVER SPLICED — including the type codes (one placeholder each) and the
// search term, whose `%` / `_` / `\` are escaped so a name containing "100%" does not match everybody.
func mapAssetFilterSQL(f domain.MapAssetFilter, next int) (string, []any) {
	var b strings.Builder
	var args []any
	b.WriteString(" AND deleted_at IS NULL")
	ph := func(v any) string {
		args = append(args, v)
		s := fmt.Sprintf("$%d", next)
		next++
		return s
	}
	if len(f.AssetTypeCodes) > 0 {
		parts := make([]string, 0, len(f.AssetTypeCodes))
		for _, c := range f.AssetTypeCodes {
			parts = append(parts, ph(c))
		}
		b.WriteString(" AND asset_type_code IN (" + strings.Join(parts, ", ") + ")")
	}
	if f.Status != "" {
		b.WriteString(" AND status = " + ph(f.Status))
	}
	if f.Verified != nil {
		b.WriteString(" AND verified = " + ph(*f.Verified))
	}
	if f.IndustryCode != "" {
		b.WriteString(" AND industry_code = " + ph(f.IndustryCode))
	}
	if f.ResidentialUnitID != "" {
		b.WriteString(" AND residential_unit_id = " + ph(f.ResidentialUnitID))
	}
	if f.Query != "" {
		p := ph("%" + escapeLike(f.Query) + "%")
		b.WriteString(" AND (name ILIKE " + p + " ESCAPE '\\' OR address ILIKE " + p + " ESCAPE '\\')")
	}
	return b.String(), args
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// Points reads the live assets of the filter as map points — five light columns and the coordinates.
// LIMIT is the ceiling PLUS ONE, so "too many" is detectable rather than indistinguishable from a
// complete set of exactly that size.
func (s *MapAssetStore) Points(ctx context.Context, f domain.MapAssetFilter) ([]domain.MapAssetPoint, error) {
	tail, args := mapAssetFilterSQL(f, 2)
	args = append(args, MapAssetPointsCeiling+1)
	tail += fmt.Sprintf(" ORDER BY asset_type_code, id LIMIT $%d", len(args)+1)

	rows, err := s.db.For(ctx).Query(ctx, `id, asset_type_code, name, status, verified, lat, lng`,
		"map_asset", tail, args...)
	if err != nil {
		return nil, fmt.Errorf("map_asset: đọc điểm bản đồ: %w", err)
	}
	defer rows.Close()

	out := make([]domain.MapAssetPoint, 0, 64)
	for rows.Next() {
		var p domain.MapAssetPoint
		if err := rows.Scan(&p.ID, &p.AssetTypeCode, &p.Name, &p.Status, &p.Verified, &p.Lat, &p.Lng); err != nil {
			return nil, fmt.Errorf("map_asset: đọc điểm: %w", err)
		}
		out = append(out, p)
		if len(out) > MapAssetPointsCeiling {
			// The rows already read are DROPPED, not returned trimmed: a caller rendering them anyway
			// is how a refusal turns back into a silent truncation.
			return nil, ErrTooManyMapAssetPoints
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("map_asset: duyệt điểm: %w", err)
	}
	return out, nil
}

// mapAssetColumns IS READ BY POSITION in scanMapAsset. Long runs of adjacent TEXT columns: a swap
// produces no error, only a phone number printed as an address.
const mapAssetColumns = `id, asset_type_code, name, address, residential_unit_id, lat, lng, ` +
	`representative, phone, status, verified, verified_at, verified_by, tax_code, industry_code, ` +
	`employee_count, established_on, description, custom_values, created_at, updated_at`

func scanMapAsset(scan func(...any) error) (domain.MapAsset, error) {
	var (
		a                                                         domain.MapAsset
		address, unit, rep, phone, verifiedBy, tax, industry, des sql.NullString
		verifiedAt, established                                   sql.NullTime
		employees                                                 sql.NullInt64
		custom                                                    []byte
	)
	if err := scan(&a.ID, &a.AssetTypeCode, &a.Name, &address, &unit, &a.Lat, &a.Lng,
		&rep, &phone, &a.Status, &a.Verified, &verifiedAt, &verifiedBy, &tax, &industry,
		&employees, &established, &des, &custom, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return domain.MapAsset{}, err
	}
	a.Address, a.ResidentialUnitID, a.Representative, a.Phone = address.String, unit.String, rep.String, phone.String
	a.VerifiedBy, a.TaxCode, a.IndustryCode, a.Description = verifiedBy.String, tax.String, industry.String, des.String
	if verifiedAt.Valid {
		t := verifiedAt.Time
		a.VerifiedAt = &t
	}
	if established.Valid {
		a.EstablishedOn = established.Time.Format(time.DateOnly)
	}
	if employees.Valid {
		n := int(employees.Int64)
		a.EmployeeCount = &n
	}
	a.CustomValues = map[string]json.RawMessage{}
	if len(custom) > 0 {
		if err := json.Unmarshal(custom, &a.CustomValues); err != nil {
			return domain.MapAsset{}, fmt.Errorf("map_asset: đọc custom_values: %w", err)
		}
	}
	return a, nil
}

// List reads one page of the `Sổ địa điểm` table: live assets of the filter, ordered by group, name
// and id (MapAssetListSort).
//
// NOT store.QueryPage, AND THE REASON IS THE ORDER: QueryPage pages on ONE sort column plus the id, and
// the table is grouped by type THEN sorted by name inside the group (spec §7). The keyset here is the
// same construction with three columns — `(asset_type_code, name, id) > (anchor's own values)` — the
// anchor looked up from its row in the same statement and the same commune ($1), exactly QueryPage's
// KindRef branch. A forged cursor naming another commune's row reads NULL and returns an empty page.
func (s *MapAssetStore) List(ctx context.Context, f domain.MapAssetFilter, req page.Request) (page.Result[domain.MapAsset], error) {
	out := page.NewResult[domain.MapAsset]()
	col := req.Column()
	if col.Param != "asset_type_code" {
		return out, fmt.Errorf("map_asset: yêu cầu phân trang chưa qua page.Parse với MapAssetListSort")
	}

	tail, args := mapAssetFilterSQL(f, 2)
	op, dir := ">", "ASC"
	if req.Dir() == page.Desc {
		op, dir = "<", "DESC"
	}
	if a, ok := req.After(); ok {
		args = append(args, a.ID)
		n := len(args) + 1
		tail += fmt.Sprintf(" AND (asset_type_code, name, id) %s ("+
			"(SELECT r.asset_type_code FROM map_asset r WHERE r.tenant_id = $1 AND r.id = $%d), "+
			"(SELECT r.name FROM map_asset r WHERE r.tenant_id = $1 AND r.id = $%d), $%d)", op, n, n, n)
	}
	args = append(args, req.Limit()+1)
	tail += fmt.Sprintf(" ORDER BY asset_type_code %s, name %s, id %s LIMIT $%d", dir, dir, dir, len(args)+1)

	rows, err := s.db.For(ctx).Query(ctx, mapAssetColumns, "map_asset", tail, args...)
	if err != nil {
		return out, fmt.Errorf("map_asset: đọc sổ địa điểm: %w", err)
	}
	defer rows.Close()

	var last string
	for rows.Next() {
		if len(out.Items) == req.Limit() {
			out.HasMore = true
			break
		}
		a, err := scanMapAsset(rows.Scan)
		if err != nil {
			return page.NewResult[domain.MapAsset](), fmt.Errorf("map_asset: đọc dòng: %w", err)
		}
		out.Items = append(out.Items, a)
		last = a.ID
	}
	if err := rows.Err(); err != nil {
		return page.NewResult[domain.MapAsset](), fmt.Errorf("map_asset: duyệt sổ địa điểm: %w", err)
	}
	if out.HasMore {
		out.NextCursor = page.Encode(col, req.Dir(), page.Anchor{Key: page.RefKey(), ID: last})
	}
	return out, nil
}

// ByID reads one LIVE asset.
func (s *MapAssetStore) ByID(ctx context.Context, id string) (domain.MapAsset, error) {
	rows, err := s.db.For(ctx).Query(ctx, mapAssetColumns, "map_asset",
		`AND id = $2 AND deleted_at IS NULL`, id)
	if err != nil {
		return domain.MapAsset{}, fmt.Errorf("map_asset: đọc một đối tượng: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.MapAsset{}, fmt.Errorf("map_asset: đọc một đối tượng: %w", err)
		}
		return domain.MapAsset{}, ErrMapAssetNotFound
	}
	a, err := scanMapAsset(rows.Scan)
	if err != nil {
		return domain.MapAsset{}, fmt.Errorf("map_asset: đọc một đối tượng: %w", err)
	}
	return a, nil
}

// Summary counts the live assets per group, and how many of them are verified (spec §1, §4.1, §12.3).
// Groups with no asset are absent; the client lays the counts over the catalogue it already holds.
func (s *MapAssetStore) Summary(ctx context.Context) (domain.MapAssetSummary, error) {
	rows, err := s.db.For(ctx).Query(ctx, `asset_type_code, count(*), count(*) FILTER (WHERE verified)`,
		"map_asset", `AND deleted_at IS NULL GROUP BY asset_type_code ORDER BY asset_type_code`)
	if err != nil {
		return domain.MapAssetSummary{}, fmt.Errorf("map_asset: đếm theo nhóm: %w", err)
	}
	defer rows.Close()

	out := domain.MapAssetSummary{ByType: []domain.MapAssetTypeCount{}}
	for rows.Next() {
		var c domain.MapAssetTypeCount
		if err := rows.Scan(&c.AssetTypeCode, &c.Count, &c.Verified); err != nil {
			return domain.MapAssetSummary{}, fmt.Errorf("map_asset: đọc số đếm: %w", err)
		}
		out.ByType = append(out.ByType, c)
		out.Total += c.Count
		out.Verified += c.Verified
	}
	if err := rows.Err(); err != nil {
		return domain.MapAssetSummary{}, fmt.Errorf("map_asset: duyệt số đếm: %w", err)
	}
	return out, nil
}

// --- the write path ------------------------------------------------------------------------------

// AssetTypeAvailable reports whether the code is a live AND in-use (`dang_dung`) row of THIS commune's
// type catalogue, and share-locks it until the transaction ends — a concurrent soft delete of the type
// waits instead of racing this write. A disabled group takes no NEW asset: it was "taken out of use".
// An asset already filed under it keeps it (the use case only asks when the type is being set).
func (s *MapAssetStore) AssetTypeAvailable(ctx context.Context, tx *store.ScopedTx, code string) (bool, error) {
	const stmt = `SELECT 1 FROM loai_tai_nguyen_ban_do ` +
		`WHERE tenant_id = $1 AND ma = $2 AND deleted_at IS NULL AND dang_dung FOR SHARE`

	var one int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), code).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("map_asset: kiểm tra nhóm tài nguyên: %w", err)
	}
	return true, nil
}

// FieldsOfType reads the LIVE map_field_schema rows of one type — active and disabled — inside the
// write's transaction, so the custom values are judged against the schema as it is at commit time.
func (s *MapAssetStore) FieldsOfType(ctx context.Context, tx *store.ScopedTx, code string) ([]domain.MapFieldSchema, error) {
	// ScopedTx.Query writes `WHERE tenant_id = $1` itself and binds tx.TenantID().
	rows, err := tx.Query(ctx, mapFieldSchemaColumns, "map_field_schema",
		`AND asset_type_code = $2 AND deleted_at IS NULL ORDER BY sort_order, field_code LIMIT $3`,
		code, MapFieldSchemaCeiling+1)
	if err != nil {
		return nil, fmt.Errorf("map_asset: đọc cấu hình trường: %w", err)
	}
	defer rows.Close()

	out := make([]domain.MapFieldSchema, 0, 8)
	for rows.Next() {
		m, err := scanMapFieldSchema(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("map_asset: đọc trường: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("map_asset: duyệt cấu hình trường: %w", err)
	}
	if len(out) > MapFieldSchemaCeiling {
		// Judging values against a truncated schema would refuse a valid key as unknown.
		return nil, ErrTooManyMapFieldSchemas
	}
	return out, nil
}

// TaxCodeTaken reports whether another LIVE asset of this commune holds the tax code. The unique key is
// the real guard (two concurrent writes can both pass this); this turns the ordinary case into a 409.
func (s *MapAssetStore) TaxCodeTaken(ctx context.Context, tx *store.ScopedTx, taxCode, exceptID string) (bool, error) {
	const stmt = `SELECT count(*) FROM map_asset WHERE tenant_id = $1 AND live_tax_code = $2 AND id <> $3`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), taxCode, exceptID).Scan(&n); err != nil {
		return false, fmt.Errorf("map_asset: kiểm tra mã số thuế: %w", err)
	}
	return n > 0, nil
}

const insertMapAsset = `INSERT INTO map_asset
	(tenant_id, id, asset_type_code, name, address, residential_unit_id, lat, lng, representative, phone,
	 status, tax_code, industry_code, employee_count, established_on, description, custom_values,
	 created_by, updated_by)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15::date, $16, $17::jsonb, $18, $18)
	RETURNING created_at, updated_at`

// Insert adds one row, NOT verified — confirmation is its own act with its own trail. `by` is the
// actor's BUSINESS CODE (rule 6, invariant 8). Returns the row with created_at / updated_at as stored.
func (s *MapAssetStore) Insert(ctx context.Context, tx *store.ScopedTx, a domain.MapAsset, by string) (domain.MapAsset, error) {
	custom, err := encodeCustomValues(a.CustomValues)
	if err != nil {
		return domain.MapAsset{}, err
	}
	err = tx.Underlying().QueryRowContext(ctx, insertMapAsset, string(tx.TenantID()), a.ID, a.AssetTypeCode,
		a.Name, nullText(a.Address), nullText(a.ResidentialUnitID), a.Lat, a.Lng, nullText(a.Representative),
		nullText(a.Phone), a.Status, nullText(a.TaxCode), nullText(a.IndustryCode), nullOrder(a.EmployeeCount),
		nullText(a.EstablishedOn), nullText(a.Description), custom, by).Scan(&a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.MapAsset{}, ErrMapAssetTaxCodeTaken
		}
		return domain.MapAsset{}, fmt.Errorf("map_asset: chèn: %w", err)
	}
	a.Verified, a.VerifiedAt, a.VerifiedBy = false, nil, ""
	return a, nil
}

// ByIDForUpdate reads one LIVE row and locks it until the transaction ends: every write is
// read-decide-write, and without the lock the audit entry of the second editor records a "before" that
// was never the row's state.
func (s *MapAssetStore) ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.MapAsset, error) {
	const stmt = `SELECT ` + mapAssetColumns + ` FROM map_asset ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	a, err := scanMapAsset(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MapAsset{}, ErrMapAssetNotFound
	}
	if err != nil {
		return domain.MapAsset{}, fmt.Errorf("map_asset: đọc dòng để sửa: %w", err)
	}
	return a, nil
}

// updateMapAsset — every field a commune may change, and who changed it (point 2 above). Confirmation
// is NOT here: it moves only through SetConfirmation, so an edit can never forge or erase it.
const updateMapAsset = `UPDATE map_asset SET asset_type_code = $3, name = $4, address = $5, ` +
	`residential_unit_id = $6, lat = $7, lng = $8, representative = $9, phone = $10, status = $11, ` +
	`tax_code = $12, industry_code = $13, employee_count = $14, established_on = $15::date, ` +
	`description = $16, custom_values = $17::jsonb, updated_at = now(), updated_by = $18 ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL RETURNING updated_at`

func (s *MapAssetStore) Update(ctx context.Context, tx *store.ScopedTx, a domain.MapAsset, by string) (domain.MapAsset, error) {
	custom, err := encodeCustomValues(a.CustomValues)
	if err != nil {
		return domain.MapAsset{}, err
	}
	err = tx.Underlying().QueryRowContext(ctx, updateMapAsset, string(tx.TenantID()), a.ID, a.AssetTypeCode,
		a.Name, nullText(a.Address), nullText(a.ResidentialUnitID), a.Lat, a.Lng, nullText(a.Representative),
		nullText(a.Phone), a.Status, nullText(a.TaxCode), nullText(a.IndustryCode), nullOrder(a.EmployeeCount),
		nullText(a.EstablishedOn), nullText(a.Description), custom, by).Scan(&a.UpdatedAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return domain.MapAsset{}, ErrMapAssetNotFound
	case err != nil && isUniqueViolation(err):
		return domain.MapAsset{}, ErrMapAssetTaxCodeTaken
	case err != nil:
		return domain.MapAsset{}, fmt.Errorf("map_asset: cập nhật: %w", err)
	}
	return a, nil
}

// setMapAssetConfirmation sets or clears the verified flag with BOTH of its facts (0015's
// `map_asset_verified_complete`): set → now() and `by`; cleared → NULL and NULL. The audit entry keeps
// who had verified and when (rule 6).
const setMapAssetConfirmation = `UPDATE map_asset SET verified = $3, ` +
	`verified_at = CASE WHEN $3 THEN now() END, verified_by = CASE WHEN $3 THEN $4 END, ` +
	`updated_at = now(), updated_by = $4 ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL RETURNING verified_at, updated_at`

// SetConfirmation applies the flag to the row the caller read and locked.
func (s *MapAssetStore) SetConfirmation(ctx context.Context, tx *store.ScopedTx, a domain.MapAsset, verified bool, by string) (domain.MapAsset, error) {
	var at sql.NullTime
	err := tx.Underlying().QueryRowContext(ctx, setMapAssetConfirmation, string(tx.TenantID()), a.ID, verified, by).
		Scan(&at, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MapAsset{}, ErrMapAssetNotFound
	}
	if err != nil {
		return domain.MapAsset{}, fmt.Errorf("map_asset: đặt xác minh: %w", err)
	}
	a.Verified = verified
	a.VerifiedAt, a.VerifiedBy = nil, ""
	if verified && at.Valid {
		t := at.Time
		a.VerifiedAt, a.VerifiedBy = &t, by
	}
	return a, nil
}

// softDeleteMapAsset writes all three columns of rule 7 invariant 1 in one statement. `AND deleted_at
// IS NULL` makes a second delete a 404 rather than a rewrite of who deleted it; the generated
// `live_tax_code` goes NULL with it, so the same enterprise may be entered again (0015).
const softDeleteMapAsset = `UPDATE map_asset ` +
	`SET deleted_at = now(), deleted_by = $3, delete_reason = $4, updated_at = now(), updated_by = $3 ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// SoftDelete retires one row. There is no hard delete in this package; the trigger refuses one.
func (s *MapAssetStore) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error {
	res, err := tx.Exec(ctx, softDeleteMapAsset, string(tx.TenantID()), id, by, reason)
	if err != nil {
		return fmt.Errorf("map_asset: xoá mềm: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("map_asset: xoá mềm: đọc số dòng: %w", err)
	}
	if n == 0 {
		return ErrMapAssetNotFound
	}
	return nil
}

func encodeCustomValues(v map[string]json.RawMessage) (string, error) {
	if v == nil {
		return "{}", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("map_asset: mã hoá custom_values: %w", err)
	}
	return string(b), nil
}
