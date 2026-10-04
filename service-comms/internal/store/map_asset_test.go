package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// The SQL of the map asset store, over a fake driver that records every statement and answers queries
// with canned rows. What is proven: the commune is $1 and is the commune of the context, EVERY read
// carries `deleted_at IS NULL`, filter values are bound (never spliced), the points ceiling refuses
// rather than truncates, the list order is group → name → id with an anchor looked up in the SAME
// commune, and the seed writes `nguon = 'he-thong'` as a literal. What PostgreSQL does with it is
// map_asset_pg_test.go (SKIPS without VIGOV_TEST_DSN).

type assetStmt struct {
	sql  string
	args []driver.Value
}

type assetDriver struct {
	mu    sync.Mutex
	stmts []assetStmt
	// answer returns the columns and rows for a query, by inspecting its text.
	answer func(q string) ([]string, [][]driver.Value)
}

func (d *assetDriver) Connect(context.Context) (driver.Conn, error) { return &assetConn{d: d}, nil }
func (d *assetDriver) Driver() driver.Driver                        { return assetOpen{} }

type assetOpen struct{}

func (assetOpen) Open(string) (driver.Conn, error) {
	return nil, errors.New("fake driver: Connector only")
}

type assetConn struct{ d *assetDriver }

func (c *assetConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fake driver: no Prepare")
}
func (c *assetConn) Close() error              { return nil }
func (c *assetConn) Begin() (driver.Tx, error) { return assetTx{}, nil }
func (c *assetConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return assetTx{}, nil
}

func (c *assetConn) record(q string, args []driver.NamedValue) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	c.d.mu.Lock()
	c.d.stmts = append(c.d.stmts, assetStmt{sql: q, args: vals})
	c.d.mu.Unlock()
}

func (c *assetConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.record(q, args)
	return driver.RowsAffected(1), nil
}

func (c *assetConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.record(q, args)
	if c.d.answer == nil {
		return &assetRows{}, nil
	}
	cols, rows := c.d.answer(q)
	return &assetRows{cols: cols, rows: rows}, nil
}

type assetTx struct{}

func (assetTx) Commit() error   { return nil }
func (assetTx) Rollback() error { return nil }

type assetRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *assetRows) Columns() []string { return r.cols }
func (r *assetRows) Close() error      { return nil }
func (r *assetRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

func (d *assetDriver) last(sub string) assetStmt {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := len(d.stmts) - 1; i >= 0; i-- {
		if strings.Contains(d.stmts[i].sql, sub) {
			return d.stmts[i]
		}
	}
	return assetStmt{}
}

const assetCommune = tenant.ID("01JA" + "AAAAAAAAAAAAAAAAAAAAAA")

func openAssetStore(t *testing.T, d *assetDriver) (*MapAssetStore, *pkgstore.DB, context.Context) {
	t.Helper()
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	h := pkgstore.New(db)
	return NewMapAssetStore(h), h, ctxXa(assetCommune)
}

var pointCols = []string{"id", "asset_type_code", "name", "status", "verified", "lat", "lng"}

func pointRows(n int) [][]driver.Value {
	out := make([][]driver.Value, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, []driver.Value{fmt.Sprintf("id-%05d", i), "cho", "Chợ", "dang-hoat-dong", false, 15.73, 108.37})
	}
	return out
}

func TestMapAssetPointsSQLScopedLiveBound(t *testing.T) {
	d := &assetDriver{answer: func(string) ([]string, [][]driver.Value) { return pointCols, pointRows(2) }}
	s, _, ctx := openAssetStore(t, d)
	v := false
	pts, err := s.Points(ctx, domain.MapAssetFilter{AssetTypeCodes: []string{"cho", "doanh-nghiep"},
		Status: "tam-ngung", Verified: &v, IndustryCode: "47", ResidentialUnitID: "thon-1", Query: "100%_x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 2 || pts[0].Lat != 15.73 || pts[0].Lng != 108.37 {
		t.Fatalf("points = %+v", pts)
	}
	st := d.last("FROM map_asset")
	for _, want := range []string{"WHERE tenant_id = $1", "AND deleted_at IS NULL", "asset_type_code IN ($2, $3)",
		"status = $4", "verified = $5", "industry_code = $6", "residential_unit_id = $7",
		"name ILIKE $8 ESCAPE '\\' OR address ILIKE $8 ESCAPE '\\'", "LIMIT $9"} {
		if !strings.Contains(st.sql, want) {
			t.Errorf("points SQL lacks %q:\n%s", want, st.sql)
		}
	}
	if st.args[0] != string(assetCommune) {
		t.Errorf("$1 = %v, want the context's commune", st.args[0])
	}
	if st.args[7] != `%100\%\_x%` {
		t.Errorf("q bound as %v — LIKE metacharacters not escaped", st.args[7])
	}
	if st.args[8] != int64(MapAssetPointsCeiling+1) {
		t.Errorf("LIMIT bound as %v, want ceiling+1", st.args[8])
	}
	if strings.Contains(st.sql, "Bình") || strings.Contains(st.sql, "100%") {
		t.Error("a filter value was spliced into the statement text")
	}
}

func TestMapAssetPointsRefusesPastTheCeiling(t *testing.T) {
	d := &assetDriver{answer: func(string) ([]string, [][]driver.Value) {
		return pointCols, pointRows(MapAssetPointsCeiling + 1)
	}}
	s, _, ctx := openAssetStore(t, d)
	pts, err := s.Points(ctx, domain.MapAssetFilter{})
	if !errors.Is(err, ErrTooManyMapAssetPoints) || pts != nil {
		t.Fatalf("err = %v, %d points — a truncated set must never be returned", err, len(pts))
	}
}

var assetCols = strings.Split(strings.ReplaceAll(mapAssetColumns, " ", ""), ",")

func assetRow(id, name string) []driver.Value {
	at := time.Date(2026, 10, 4, 3, 0, 0, 0, time.UTC)
	return []driver.Value{id, "doanh-nghiep", name, "Thôn 1", nil, 15.730507, 108.37811,
		"Nguyễn Văn Hùng", "0900000000", "dang-hoat-dong", true, at, "CB-00123", "0101234567", "47",
		int64(12), at, nil, []byte(`{"legal_form":"tnhh"}`), at, at}
}

func TestMapAssetListOrderAnchorAndScan(t *testing.T) {
	d := &assetDriver{answer: func(string) ([]string, [][]driver.Value) {
		return assetCols, [][]driver.Value{assetRow("a1", "A"), assetRow("a2", "B"), assetRow("a3", "C")}
	}}
	s, _, ctx := openAssetStore(t, d)
	req, err := page.New(MapAssetListSort, "", "", "2", "")
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.List(ctx, domain.MapAssetFilter{}, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 || !res.HasMore || res.NextCursor == "" {
		t.Fatalf("page = %d items, has_more=%v cursor=%q", len(res.Items), res.HasMore, res.NextCursor)
	}
	a := res.Items[0]
	if a.EmployeeCount == nil || *a.EmployeeCount != 12 || a.EstablishedOn != "2026-10-04" || a.VerifiedBy != "CB-00123" ||
		string(a.CustomValues["legal_form"]) != `"tnhh"` || a.Description != "" || a.ResidentialUnitID != "" {
		t.Errorf("scanned = %+v", a)
	}
	st := d.last("FROM map_asset")
	if !strings.Contains(st.sql, "AND deleted_at IS NULL") ||
		!strings.Contains(st.sql, "ORDER BY asset_type_code ASC, name ASC, id ASC LIMIT $2") {
		t.Errorf("list SQL:\n%s", st.sql)
	}
	// THE CURSOR CARRIES NO NAME (KindRef): decoding it yields only the anchor id.
	next, err := page.New(MapAssetListSort, "", "", "2", res.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.List(ctx, domain.MapAssetFilter{}, next); err != nil {
		t.Fatal(err)
	}
	st = d.last("FROM map_asset")
	if !strings.Contains(st.sql, "(asset_type_code, name, id) > ((SELECT r.asset_type_code FROM map_asset r WHERE r.tenant_id = $1 AND r.id = $2), (SELECT r.name FROM map_asset r WHERE r.tenant_id = $1 AND r.id = $2), $2)") {
		t.Errorf("anchor not looked up in the same commune:\n%s", st.sql)
	}
	if st.args[1] != "a2" {
		t.Errorf("anchor id = %v, want a2", st.args[1])
	}
}

func TestMapAssetReadsExcludeDeleted(t *testing.T) {
	d := &assetDriver{answer: func(q string) ([]string, [][]driver.Value) {
		if strings.Contains(q, "GROUP BY") {
			return []string{"asset_type_code", "count", "verified"}, [][]driver.Value{{"cho", int64(2), int64(1)}, {"doanh-nghiep", int64(24), int64(10)}}
		}
		return assetCols, nil
	}}
	s, _, ctx := openAssetStore(t, d)
	if _, err := s.ByID(ctx, "x"); !errors.Is(err, ErrMapAssetNotFound) {
		t.Errorf("ByID of nothing: %v", err)
	}
	if st := d.last("AND id = $2"); !strings.Contains(st.sql, "deleted_at IS NULL") || st.args[0] != string(assetCommune) {
		t.Errorf("ByID SQL: %s", st.sql)
	}
	sum, err := s.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Total != 26 || sum.Verified != 11 || len(sum.ByType) != 2 {
		t.Errorf("summary = %+v", sum)
	}
	if st := d.last("GROUP BY"); !strings.Contains(st.sql, "WHERE tenant_id = $1 AND deleted_at IS NULL") {
		t.Errorf("summary SQL: %s", st.sql)
	}
}

func TestMapAssetWriteStatementsScopedAndSigned(t *testing.T) {
	at := time.Date(2026, 10, 4, 3, 0, 0, 0, time.UTC)
	d := &assetDriver{answer: func(q string) ([]string, [][]driver.Value) {
		switch {
		case strings.Contains(q, "RETURNING created_at"):
			return []string{"created_at", "updated_at"}, [][]driver.Value{{at, at}}
		case strings.Contains(q, "RETURNING verified_at"):
			return []string{"verified_at", "updated_at"}, [][]driver.Value{{at, at}}
		case strings.Contains(q, "count(*)"):
			return []string{"count"}, [][]driver.Value{{int64(0)}}
		}
		return nil, nil
	}}
	s, h, ctx := openAssetStore(t, d)
	err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if _, err := s.Insert(ctx, tx, domain.MapAsset{ID: "n1", AssetTypeCode: "cho", Name: "Chợ", Lat: 1, Lng: 2,
			Status: "dang-hoat-dong"}, "CB-00123"); err != nil {
			return err
		}
		if _, err := s.TaxCodeTaken(ctx, tx, "0101234567", "n1"); err != nil {
			return err
		}
		a, err := s.SetConfirmation(ctx, tx, domain.MapAsset{ID: "n1"}, true, "CB-00123")
		if err != nil {
			return err
		}
		if !a.Verified || a.VerifiedBy != "CB-00123" || a.VerifiedAt == nil {
			t.Errorf("confirmation = %+v", a)
		}
		return s.SoftDelete(ctx, tx, "n1", "CB-00123", "trùng")
	})
	if err != nil {
		t.Fatal(err)
	}
	ins := d.last("INSERT INTO map_asset")
	if ins.args[0] != string(assetCommune) || ins.args[17] != "CB-00123" || ins.args[16] != "{}" {
		t.Errorf("insert args = %v", ins.args)
	}
	if st := d.last("live_tax_code"); !strings.Contains(st.sql, "tenant_id = $1") {
		t.Errorf("tax check SQL: %s", st.sql)
	}
	if st := d.last("SET verified"); !strings.Contains(st.sql, "verified_at = CASE WHEN $3 THEN now() END") ||
		!strings.Contains(st.sql, "AND deleted_at IS NULL") {
		t.Errorf("confirmation SQL: %s", st.sql)
	}
	del := d.last("SET deleted_at")
	if !strings.Contains(del.sql, "deleted_by = $3, delete_reason = $4") || !strings.Contains(del.sql, "AND deleted_at IS NULL") {
		t.Errorf("soft delete SQL: %s", del.sql)
	}
	for _, st := range d.stmts {
		if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(st.sql)), "DELETE") {
			t.Errorf("a hard DELETE was sent: %s", st.sql)
		}
	}
}

func TestSeedSystemRowIsALiteralTier(t *testing.T) {
	d := &assetDriver{answer: func(string) ([]string, [][]driver.Value) {
		return []string{"ma", "deleted"}, [][]driver.Value{{"ocop", true}}
	}}
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	h := pkgstore.New(db)
	cat := NewLoaiTaiNguyenBanDoStore(h)
	ctx := ctxXa(assetCommune)
	err := h.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if err := cat.LockCatalogueForSeed(ctx, tx); err != nil {
			return err
		}
		states, err := cat.CodeStates(ctx, tx, []string{"ocop", "cho"})
		if err != nil {
			return err
		}
		if deleted, ok := states["ocop"]; !ok || !deleted {
			t.Errorf("states = %v", states)
		}
		return cat.InsertSystemRow(ctx, tx, "id-1", domain.MapAssetTypeDefault{Code: "cho", Label: "Chợ, trung tâm thương mại", Order: 4})
	})
	if err != nil {
		t.Fatal(err)
	}
	ins := d.last("INSERT INTO loai_tai_nguyen_ban_do")
	if !strings.Contains(ins.sql, "'he-thong', false)") || len(ins.args) != 5 || ins.args[0] != string(assetCommune) {
		t.Errorf("insert = %s %v — nguon must be a LITERAL, never a parameter", ins.sql, ins.args)
	}
	// CodeStates must SEE soft-deleted rows: an issued code is never reissued.
	if st := d.last("deleted_at IS NOT NULL"); strings.Contains(st.sql, "deleted_at IS NULL") || !strings.Contains(st.sql, "WHERE tenant_id = $1") {
		t.Errorf("code states SQL: %s", st.sql)
	}
	if st := d.last("pg_advisory_xact_lock"); st.args[0] != string(assetCommune) {
		t.Errorf("lock keyed on %v", st.args)
	}
}
