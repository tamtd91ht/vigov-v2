package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"

	"github.com/vihat/vigov/core/audit"
	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	docstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The REAL use case over the REAL NhanLinhVucStore over a fake database/sql driver that records every
// statement and every transaction boundary — the approach of nhan_trang_thai_nhiem_vu_test.go, because
// "one transaction", "a refusal writes nothing" and "the commune is $1" are properties of the SQL.
//
// NOT PROVED HERE: anything PostgreSQL does — the CHECKs of migration 0022, ON CONFLICT against the
// real unique key, the `WHERE deleted_at IS NULL` branch returning zero rows. Those need VIGOV_TEST_DSN.

var communeFields = tenant.ID("01JA" + strings.Repeat("A", 22))

type fieldStmt struct {
	sql  string
	args []driver.Value
}

type fieldDB struct {
	mu    sync.Mutex
	stmts []fieldStmt

	begun, committed, rolledBack int

	// row is what the FOR UPDATE read returns; nil = no override for the code.
	row     *domain.NhanLinhVuc
	deleted bool
	// upsertTouchesNothing makes the upsert report 0 rows — the soft-deleted conflict branch.
	upsertTouchesNothing bool
	// failOn fails the first statement containing this substring.
	failOn string
}

func (k *fieldDB) record(q string, args []driver.NamedValue) error {
	v := make([]driver.Value, 0, len(args))
	for _, a := range args {
		v = append(v, a.Value)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.stmts = append(k.stmts, fieldStmt{sql: q, args: v})
	if k.failOn != "" && strings.Contains(q, k.failOn) {
		k.failOn = ""
		return errors.New("fake driver: statement built to fail")
	}
	return nil
}

func (k *fieldDB) with(sub string) []fieldStmt {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []fieldStmt
	for _, s := range k.stmts {
		if strings.Contains(s.sql, sub) {
			out = append(out, s)
		}
	}
	return out
}

func (k *fieldDB) Connect(context.Context) (driver.Conn, error) { return &fieldConn{k: k}, nil }
func (k *fieldDB) Driver() driver.Driver                        { return fieldDriver{} }

type fieldDriver struct{}

func (fieldDriver) Open(string) (driver.Conn, error) { return nil, errors.New("connector only") }

type fieldConn struct{ k *fieldDB }

func (c *fieldConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no Prepare") }
func (c *fieldConn) Close() error                        { return nil }
func (c *fieldConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *fieldConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.begun++
	c.k.mu.Unlock()
	return &fieldTx{k: c.k}, nil
}

func (c *fieldConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.k.record(q, args); err != nil {
		return nil, err
	}
	if strings.Contains(q, "INSERT INTO nhan_linh_vuc") && c.k.upsertTouchesNothing {
		return driver.RowsAffected(0), nil
	}
	return driver.RowsAffected(1), nil
}

func (c *fieldConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.k.record(q, args); err != nil {
		return nil, err
	}
	if !strings.Contains(q, "FOR UPDATE") {
		return nil, fmt.Errorf("fake driver: no answer for %q", q)
	}
	r := &fieldRows{}
	if h := c.k.row; h != nil {
		var label driver.Value
		if h.Nhan != "" {
			label = h.Nhan
		}
		r.rows = [][]driver.Value{{h.ID, h.Ma, label, int64(h.SortOrder), h.Enabled, c.k.deleted}}
	}
	return r, nil
}

type fieldTx struct{ k *fieldDB }

func (t *fieldTx) Commit() error   { t.k.mu.Lock(); t.k.committed++; t.k.mu.Unlock(); return nil }
func (t *fieldTx) Rollback() error { t.k.mu.Lock(); t.k.rolledBack++; t.k.mu.Unlock(); return nil }

type fieldRows struct {
	rows [][]driver.Value
	i    int
}

// Columns mirror the store's SELECT list in ORDER, so a reorder there turns this red.
func (r *fieldRows) Columns() []string {
	return []string{"id", "ma", "nhan", "thu_tu", "enabled", "deleted"}
}
func (r *fieldRows) Close() error { return nil }
func (r *fieldRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

// tier1Fake is tier 1. Two codes: an active one and a retired one.
type tier1Fake struct {
	err   error
	calls int
}

func (f *tier1Fake) Fields(ctx context.Context) ([]domain.FieldDefault, error) {
	f.calls++
	_ = tenant.MustFrom(ctx)
	if f.err != nil {
		return nil, f.err
	}
	return []domain.FieldDefault{
		{Code: "rac-thai", DefaultLabel: "Rác thải – Vệ sinh môi trường", SortOrder: 2, Icon: "Trash2", Tone: "green", Active: true},
		{Code: "ma-cu", DefaultLabel: "Mã đã ngừng", SortOrder: 13, Active: false},
	}, nil
}

func buildFieldCatalogue(t *testing.T, k *fieldDB, t1 *tier1Fake) (*PetitionFieldCatalogue, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	s := store.New(db)
	return NewPetitionFieldCatalogue(s, t1, docstore.NewNhanLinhVucStore(s)),
		tenant.Into(context.Background(), communeFields)
}

func fieldActor() audit.Actor { return audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"} }

func strPtr(s string) *string { return &s }
func intPtr(n int) *int       { return &n }
func boolPtr(b bool) *bool    { return &b }

// --- rule 6, invariant 3: override and trail in ONE transaction ------------------------------------

func TestFieldEditWritesOverrideAndTrailInOneTransaction(t *testing.T) {
	k := &fieldDB{}
	uc, ctx := buildFieldCatalogue(t, k, &tier1Fake{})

	after, err := uc.Edit(ctx, "rac-thai", domain.PetitionFieldEdit{
		Label: strPtr("Rác thải xã A"), Enabled: boolPtr(false)}, fieldActor())
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if k.begun != 1 || k.committed != 1 || k.rolledBack != 0 {
		t.Errorf("tx begun=%d committed=%d rolledBack=%d, want 1/1/0", k.begun, k.committed, k.rolledBack)
	}
	lock := k.with("FOR UPDATE")
	up := k.with("INSERT INTO nhan_linh_vuc")
	trail := k.with("INSERT INTO audit_log")
	if len(lock) != 1 || len(up) != 1 || len(trail) != 1 {
		t.Fatalf("lock=%d upsert=%d trail=%d, want 1/1/1", len(lock), len(up), len(trail))
	}
	// Rule 1: the commune is $1, from the context; the code is $2.
	if lock[0].args[0] != string(communeFields) || lock[0].args[1] != "rac-thai" {
		t.Errorf("lock args = %v", lock[0].args)
	}
	// Upsert: commune, a fresh id, code, label, 0 = inherit order, enabled=false.
	a := up[0].args
	if a[0] != string(communeFields) || a[2] != "rac-thai" || a[3] != "Rác thải xã A" || a[4] != int64(0) || a[5] != false {
		t.Errorf("upsert args = %v", a)
	}
	if id, _ := a[1].(string); len(id) != 26 {
		t.Errorf("row id %v is not a ULID", a[1])
	}
	if !strings.Contains(up[0].sql, "ON CONFLICT (tenant_id, ma) DO UPDATE") ||
		!strings.Contains(up[0].sql, "WHERE nhan_linh_vuc.deleted_at IS NULL") {
		t.Errorf("not the guarded upsert: %q", up[0].sql)
	}
	// Trail: commune, business-code actor, verb, subject = field code, effective before/after.
	v := trail[0].args
	if v[0] != string(communeFields) || v[1] != "CB-00123" || v[4] != ActionUpdatePetitionField || v[5] != "rac-thai" {
		t.Errorf("trail = %v", v[:6])
	}
	var delta map[string]map[string]any
	if err := json.Unmarshal(v[7].([]byte), &delta); err != nil {
		t.Fatalf("delta: %v", err)
	}
	if delta["truoc"]["label"] != "Rác thải – Vệ sinh môi trường" || delta["sau"]["label"] != "Rác thải xã A" ||
		delta["truoc"]["enabled"] != true || delta["sau"]["enabled"] != false {
		t.Errorf("delta = %v", delta)
	}
	if _, moved := delta["sau"]["order"]; moved {
		t.Errorf("order did not move but is in the delta: %v", delta)
	}
	if after.Label != "Rác thải xã A" || after.Enabled || !after.Customised {
		t.Errorf("returned view = %+v", after)
	}
}

func TestFieldEditSendingTheDefaultsStoresInherit(t *testing.T) {
	k := &fieldDB{row: &domain.NhanLinhVuc{ID: "nlv-1", Ma: "rac-thai", Nhan: "Rác thải xã A", SortOrder: 5, Enabled: true}}
	uc, ctx := buildFieldCatalogue(t, k, &tier1Fake{})

	after, err := uc.Edit(ctx, "rac-thai", domain.PetitionFieldEdit{
		Label: strPtr("Rác thải – Vệ sinh môi trường"), Order: intPtr(2)}, fieldActor())
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	a := k.with("INSERT INTO nhan_linh_vuc")[0].args
	// NULL label and 0 order: a later correction of the platform default reaches this commune.
	if a[3] != nil || a[4] != int64(0) {
		t.Errorf("defaults stored as copies, not inherit: label=%v order=%v", a[3], a[4])
	}
	if after.Customised {
		t.Errorf("a code back on every default is still Customised: %+v", after)
	}
}

func TestFieldEditNoOpWritesNothing(t *testing.T) {
	k := &fieldDB{row: &domain.NhanLinhVuc{ID: "nlv-1", Ma: "rac-thai", Nhan: "Rác thải xã A", Enabled: true}}
	uc, ctx := buildFieldCatalogue(t, k, &tier1Fake{})

	if _, err := uc.Edit(ctx, "rac-thai", domain.PetitionFieldEdit{Label: strPtr("Rác thải xã A")}, fieldActor()); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if len(k.with("INSERT INTO nhan_linh_vuc")) != 0 || len(k.with("INSERT INTO audit_log")) != 0 {
		t.Error("a no-op wrote a row or a trail entry — idem.KhongCan rests on it writing nothing")
	}
}

func TestFieldEditRefusalsOpenNoTransaction(t *testing.T) {
	for name, tc := range map[string]struct {
		code  string
		edit  domain.PetitionFieldEdit
		actor audit.Actor
		t1err error
		want  error
	}{
		"code not in tier 1": {code: "khong-co", edit: domain.PetitionFieldEdit{Enabled: boolPtr(false)},
			actor: fieldActor(), want: docstore.ErrDanhMucKhongTonTai},
		"platform unreachable": {code: "rac-thai", edit: domain.PetitionFieldEdit{Enabled: boolPtr(false)},
			actor: fieldActor(), t1err: fmt.Errorf("%w: x", ErrFieldCatalogueUnavailable), want: ErrFieldCatalogueUnavailable},
		"blank label": {code: "rac-thai", edit: domain.PetitionFieldEdit{Label: strPtr("  ")},
			actor: fieldActor(), want: domain.ErrNhanTrong},
		"order zero": {code: "rac-thai", edit: domain.PetitionFieldEdit{Order: intPtr(0)},
			actor: fieldActor(), want: domain.ErrThuTuNgoaiKhoang},
		"empty edit": {code: "rac-thai", actor: fieldActor(), want: domain.ErrFieldEditEmpty},
	} {
		t.Run(name, func(t *testing.T) {
			k := &fieldDB{}
			uc, ctx := buildFieldCatalogue(t, k, &tier1Fake{err: tc.t1err})
			_, err := uc.Edit(ctx, tc.code, tc.edit, tc.actor)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if k.begun != 0 {
				t.Error("a refusal opened a transaction")
			}
		})
	}
	t.Run("no actor business code", func(t *testing.T) {
		k := &fieldDB{}
		uc, ctx := buildFieldCatalogue(t, k, &tier1Fake{})
		if _, err := uc.Edit(ctx, "rac-thai", domain.PetitionFieldEdit{Enabled: boolPtr(false)}, audit.Actor{}); err == nil {
			t.Fatal("an edit nobody can be named for was accepted")
		}
		if k.begun != 0 {
			t.Error("opened a transaction for an unnamed actor")
		}
	})
}

func TestFieldEditRetiredCodeIsStillEditable(t *testing.T) {
	// Retiring decides INTAKE; the label still shows on old petitions (ADR 0060 §4).
	k := &fieldDB{}
	uc, ctx := buildFieldCatalogue(t, k, &tier1Fake{})
	if _, err := uc.Edit(ctx, "ma-cu", domain.PetitionFieldEdit{Label: strPtr("Mã cũ của xã")}, fieldActor()); err != nil {
		t.Fatalf("Edit on a retired code: %v", err)
	}
}

func TestFieldEditTrailFailureRollsBack(t *testing.T) {
	k := &fieldDB{failOn: "INSERT INTO audit_log"}
	uc, ctx := buildFieldCatalogue(t, k, &tier1Fake{})
	if _, err := uc.Edit(ctx, "rac-thai", domain.PetitionFieldEdit{Enabled: boolPtr(false)}, fieldActor()); err == nil {
		t.Fatal("trail failure swallowed")
	}
	if k.committed != 0 || k.rolledBack != 1 {
		t.Errorf("committed=%d rolledBack=%d — the override must not survive without its trail", k.committed, k.rolledBack)
	}
}

func TestFieldEditRefusesToReviveASoftDeletedRow(t *testing.T) {
	t.Run("seen by the locked read", func(t *testing.T) {
		k := &fieldDB{row: &domain.NhanLinhVuc{ID: "nlv-1", Ma: "rac-thai", Enabled: true}, deleted: true}
		uc, ctx := buildFieldCatalogue(t, k, &tier1Fake{})
		_, err := uc.Edit(ctx, "rac-thai", domain.PetitionFieldEdit{Enabled: boolPtr(false)}, fieldActor())
		if !errors.Is(err, docstore.ErrFieldOverrideSoftDeleted) {
			t.Fatalf("err = %v", err)
		}
		if len(k.with("INSERT INTO nhan_linh_vuc")) != 0 || k.committed != 0 {
			t.Error("wrote over a soft-deleted row")
		}
	})
	t.Run("upsert touched no row", func(t *testing.T) {
		k := &fieldDB{upsertTouchesNothing: true}
		uc, ctx := buildFieldCatalogue(t, k, &tier1Fake{})
		_, err := uc.Edit(ctx, "rac-thai", domain.PetitionFieldEdit{Enabled: boolPtr(false)}, fieldActor())
		if !errors.Is(err, docstore.ErrFieldOverrideSoftDeleted) {
			t.Fatalf("err = %v", err)
		}
		if k.committed != 0 || len(k.with("INSERT INTO audit_log")) != 0 {
			t.Error("committed or audited an upsert that wrote nothing")
		}
	})
}

// --- the platform adapter ------------------------------------------------------------------------

type platformFieldsFake struct {
	err    error
	fields []*platformv1.PetitionField
}

func (p *platformFieldsFake) ListPetitionFields(context.Context, *platformv1.ListPetitionFieldsRequest,
	...grpc.CallOption) (*platformv1.ListPetitionFieldsResponse, error) {
	if p.err != nil {
		return nil, p.err
	}
	return &platformv1.ListPetitionFieldsResponse{Fields: p.fields}, nil
}

func TestTier1AdapterMapsUnavailableTo503Sentinel(t *testing.T) {
	a := NewTier1Fields(platformclient.NewPetitionFields(&platformFieldsFake{err: errors.New("unavailable")}, nil))
	_, err := a.Fields(tenant.Into(context.Background(), communeFields))
	if !errors.Is(err, ErrFieldCatalogueUnavailable) {
		t.Fatalf("err = %v, want ErrFieldCatalogueUnavailable", err)
	}
}

func TestTier1AdapterCarriesEveryField(t *testing.T) {
	a := NewTier1Fields(platformclient.NewPetitionFields(&platformFieldsFake{fields: []*platformv1.PetitionField{
		{Code: "rac-thai", DefaultLabel: "Rác thải", SortOrder: 2, Icon: "Trash2", Tone: "green", Active: true},
		{Code: "ma-cu", DefaultLabel: "Mã cũ", SortOrder: 13, Active: false},
	}}, nil))
	got, err := a.Fields(tenant.Into(context.Background(), communeFields))
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.FieldDefault{
		{Code: "rac-thai", DefaultLabel: "Rác thải", SortOrder: 2, Icon: "Trash2", Tone: "green", Active: true},
		{Code: "ma-cu", DefaultLabel: "Mã cũ", SortOrder: 13, Active: false},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}
