package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The map asset use cases over a RECORDING driver (transaction boundaries and the audit INSERT, which
// core/audit.Write sends through the transaction) and an IN-MEMORY repo that applies the store's
// predicates (live rows only, tax code unique among live rows). What is proven here: the write and its
// entry share ONE transaction, a refusal commits nothing and audits nothing, the actor is the business
// code, the delta masks personal data, a no-op writes nothing. The SQL itself — the commune as $1, the
// soft-delete predicate in every read — is internal/store/map_asset_test.go; PostgreSQL's half (the
// guard trigger, the generated tax-code key) is internal/store/map_asset_pg_test.go.

// --- recording driver ----------------------------------------------------------------------------

type recStmt struct {
	sql  string
	args []driver.Value
}

type recDB struct {
	mu                           sync.Mutex
	stmts                        []recStmt
	begun, committed, rolledBack int
	failOn                       string // fails the first exec containing this
}

func (d *recDB) Connect(context.Context) (driver.Conn, error) { return &recConn{d: d}, nil }
func (d *recDB) Driver() driver.Driver                        { return recDriver{} }

type recDriver struct{}

func (recDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("fake driver: Connector only")
}

type recConn struct{ d *recDB }

func (c *recConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fake driver: no Prepare")
}
func (c *recConn) Close() error { return nil }
func (c *recConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *recConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.mu.Lock()
	c.d.begun++
	c.d.mu.Unlock()
	return &recTx{d: c.d}, nil
}

func (c *recConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	c.d.stmts = append(c.d.stmts, recStmt{sql: q, args: vals})
	if c.d.failOn != "" && strings.Contains(q, c.d.failOn) {
		c.d.failOn = ""
		return nil, errors.New("fake driver: built to fail")
	}
	return driver.RowsAffected(1), nil
}

type recTx struct{ d *recDB }

func (t *recTx) Commit() error   { t.d.mu.Lock(); t.d.committed++; t.d.mu.Unlock(); return nil }
func (t *recTx) Rollback() error { t.d.mu.Lock(); t.d.rolledBack++; t.d.mu.Unlock(); return nil }

// audits returns the audit INSERTs recorded, decoded: actor, action, subject, delta.
type recAudit struct {
	actor, action, subject string
	delta                  map[string]any
}

func (d *recDB) audits(t *testing.T) []recAudit {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []recAudit
	for _, s := range d.stmts {
		if !strings.Contains(s.sql, "INSERT INTO audit_log") {
			continue
		}
		var delta map[string]any
		if b, ok := s.args[7].([]byte); ok && len(b) > 0 {
			if err := json.Unmarshal(b, &delta); err != nil {
				t.Fatal(err)
			}
		}
		out = append(out, recAudit{actor: s.args[1].(string), action: s.args[4].(string), subject: s.args[5].(string), delta: delta})
	}
	return out
}

func openRec(t *testing.T, d *recDB) *store.DB {
	t.Helper()
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return store.New(db)
}

// --- in-memory repo ------------------------------------------------------------------------------

type memAsset struct {
	a       domain.MapAsset
	deleted bool
}

type fakeAssetRepo struct {
	types   map[string]bool // code -> live and in use
	fields  map[string][]domain.MapFieldSchema
	rows    map[string]*memAsset
	commune tenant.ID // the commune the last transaction was bound to

	inserts, updates, verifies, deletes int
}

func newFakeAssetRepo() *fakeAssetRepo {
	return &fakeAssetRepo{
		types: map[string]bool{"doanh-nghiep": true, "cho": true, "ho-kinh-doanh": false},
		fields: map[string][]domain.MapFieldSchema{
			"doanh-nghiep": {
				{FieldCode: "legal_form", ValueType: domain.ValueTypeChoice, IsActive: true, IsRequired: true,
					Options: []domain.FieldOption{{Value: "tnhh", Label: "TNHH"}}},
				{FieldCode: "revenue_estimate", ValueType: domain.ValueTypeDecimal, IsActive: true},
			},
		},
		rows: map[string]*memAsset{},
	}
}

func (r *fakeAssetRepo) note(tx *store.ScopedTx) { r.commune = tx.TenantID() }

func (r *fakeAssetRepo) AssetTypeAvailable(_ context.Context, tx *store.ScopedTx, code string) (bool, error) {
	r.note(tx)
	return r.types[code], nil
}

func (r *fakeAssetRepo) FieldsOfType(_ context.Context, tx *store.ScopedTx, code string) ([]domain.MapFieldSchema, error) {
	r.note(tx)
	return r.fields[code], nil
}

func (r *fakeAssetRepo) TaxCodeTaken(_ context.Context, tx *store.ScopedTx, taxCode, exceptID string) (bool, error) {
	r.note(tx)
	for id, m := range r.rows {
		if !m.deleted && id != exceptID && m.a.TaxCode == taxCode {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeAssetRepo) Insert(_ context.Context, tx *store.ScopedTx, a domain.MapAsset, _ string) (domain.MapAsset, error) {
	r.note(tx)
	r.inserts++
	a.CreatedAt = time.Date(2026, 10, 4, 3, 0, 0, 0, time.UTC)
	a.UpdatedAt = a.CreatedAt
	r.rows[a.ID] = &memAsset{a: a}
	return a, nil
}

func (r *fakeAssetRepo) ByIDForUpdate(_ context.Context, tx *store.ScopedTx, id string) (domain.MapAsset, error) {
	r.note(tx)
	m, ok := r.rows[id]
	if !ok || m.deleted {
		return domain.MapAsset{}, commsstore.ErrMapAssetNotFound
	}
	return m.a, nil
}

func (r *fakeAssetRepo) Update(_ context.Context, tx *store.ScopedTx, a domain.MapAsset, _ string) (domain.MapAsset, error) {
	r.note(tx)
	r.updates++
	r.rows[a.ID].a = a
	return a, nil
}

func (r *fakeAssetRepo) SetConfirmation(_ context.Context, tx *store.ScopedTx, a domain.MapAsset, verified bool, by string) (domain.MapAsset, error) {
	r.note(tx)
	r.verifies++
	a.Verified, a.VerifiedAt, a.VerifiedBy = verified, nil, ""
	if verified {
		at := time.Date(2026, 10, 4, 4, 0, 0, 0, time.UTC)
		a.VerifiedAt, a.VerifiedBy = &at, by
	}
	r.rows[a.ID].a = a
	return a, nil
}

func (r *fakeAssetRepo) SoftDelete(_ context.Context, tx *store.ScopedTx, id, _, _ string) error {
	r.note(tx)
	r.deletes++
	r.rows[id].deleted = true
	return nil
}

// --- harness -------------------------------------------------------------------------------------

const assetActorCode = "CB-00123"

func assetActor() audit.Actor { return audit.Actor{ID: assetActorCode, Kind: "staff", IP: "10.0.0.7"} }

func newAssetUseCase(t *testing.T) (*MapAssets, *fakeAssetRepo, *recDB, context.Context) {
	t.Helper()
	d := &recDB{}
	repo := newFakeAssetRepo()
	uc := NewMapAssets(openRec(t, d), repo)
	n := 0
	uc.newID = func() (string, error) { n++; return "01JMAPASSET00000000000000" + string(rune('0'+n)), nil }
	return uc, repo, d, tenant.Into(context.Background(), xaA)
}

func f64(v float64) *float64 { return &v }

func validInput() MapAssetInput {
	return MapAssetInput{
		AssetTypeCode: "doanh-nghiep", Name: "Công ty TNHH May Thăng Bình", Lat: f64(15.7305071), Lng: f64(108.378110),
		Address: "Cụm công nghiệp Hà Lam", Representative: "Nguyễn Văn Hùng", Phone: "0900000000", TaxCode: "0101234567",
		CustomValues: map[string]json.RawMessage{"legal_form": json.RawMessage(`"tnhh"`)},
	}
}

// --- create --------------------------------------------------------------------------------------

func TestMapAssetCreateWritesRowAndMaskedEntryInOneTransaction(t *testing.T) {
	uc, repo, d, ctx := newAssetUseCase(t)
	a, err := uc.Create(ctx, validInput(), assetActor())
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != domain.MapAssetStatusActive || a.Lat != 15.730507 || a.Verified {
		t.Errorf("row = %+v", a)
	}
	if d.begun != 1 || d.committed != 1 || d.rolledBack != 0 || repo.inserts != 1 || repo.commune != xaA {
		t.Fatalf("tx begun=%d committed=%d rolledBack=%d inserts=%d commune=%q", d.begun, d.committed, d.rolledBack, repo.inserts, repo.commune)
	}
	entries := d.audits(t)
	if len(entries) != 1 {
		t.Fatalf("%d audit entries, want 1", len(entries))
	}
	e := entries[0]
	if e.actor != assetActorCode || e.action != ActionCreateMapAsset {
		t.Errorf("entry = %+v", e)
	}
	// The subject never carries the name (a household is named after a person) nor the id.
	if e.subject != "doi-tuong-ban-do/doanh-nghiep/2026-10-04" {
		t.Errorf("subject = %q", e.subject)
	}
	after := e.delta["sau"].(map[string]any)
	raw, _ := json.Marshal(after)
	for _, leaked := range []string{"0900000000", "0101234567", "Hùng", "Hà Lam", "15.73", "108.37"} {
		if strings.Contains(string(raw), leaked) {
			t.Errorf("audit delta carries %q unmasked: %s", leaked, raw)
		}
	}
	if after["phone"] != "09****0000" || after["has_address"] != true {
		t.Errorf("delta = %s", raw)
	}
}

func TestMapAssetCreateRefusalsCommitNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		edit func(*MapAssetInput, *fakeAssetRepo)
		want error
	}{
		"type not in catalogue": {func(in *MapAssetInput, _ *fakeAssetRepo) { in.AssetTypeCode = "khong-co" }, commsstore.ErrMapAssetTypeUnavailable},
		"type disabled":         {func(in *MapAssetInput, _ *fakeAssetRepo) { in.AssetTypeCode = "ho-kinh-doanh" }, commsstore.ErrMapAssetTypeUnavailable},
		"tax code taken": {func(_ *MapAssetInput, r *fakeAssetRepo) {
			r.rows["other"] = &memAsset{a: domain.MapAsset{ID: "other", TaxCode: "0101234567"}}
		}, commsstore.ErrMapAssetTaxCodeTaken},
		"unknown custom key": {func(in *MapAssetInput, _ *fakeAssetRepo) {
			in.CustomValues["bogus"] = json.RawMessage(`1`)
		}, domain.ErrCustomValueUnknownField},
		"required custom missing": {func(in *MapAssetInput, _ *fakeAssetRepo) { in.CustomValues = nil }, domain.ErrCustomValueRequired},
		"wrong custom type": {func(in *MapAssetInput, _ *fakeAssetRepo) {
			in.CustomValues["revenue_estimate"] = json.RawMessage(`"nhiều"`)
		}, domain.ErrCustomValueWrongType},
	} {
		t.Run(name, func(t *testing.T) {
			uc, repo, d, ctx := newAssetUseCase(t)
			in := validInput()
			tc.edit(&in, repo)
			if _, err := uc.Create(ctx, in, assetActor()); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if repo.inserts != 0 || len(d.audits(t)) != 0 || d.committed != 0 {
				t.Errorf("a refusal wrote: inserts=%d audits=%d committed=%d", repo.inserts, len(d.audits(t)), d.committed)
			}
		})
	}
}

func TestMapAssetCreateShapeRefusedBeforeTransaction(t *testing.T) {
	uc, _, d, ctx := newAssetUseCase(t)
	in := validInput()
	in.Lng = nil
	if _, err := uc.Create(ctx, in, assetActor()); !errors.Is(err, domain.ErrMapAssetLocationMissing) {
		t.Fatalf("err = %v", err)
	}
	if _, err := uc.Create(ctx, validInput(), audit.Actor{}); !errors.Is(err, ErrMissingActor) {
		t.Fatalf("no actor: %v", err)
	}
	if d.begun != 0 {
		t.Error("a shape refusal opened a transaction")
	}
}

func TestMapAssetAuditFailureRollsBackTheWrite(t *testing.T) {
	uc, _, d, ctx := newAssetUseCase(t)
	d.failOn = "INSERT INTO audit_log"
	if _, err := uc.Create(ctx, validInput(), assetActor()); err == nil {
		t.Fatal("the audit entry failed and the create reported success")
	}
	if d.committed != 0 || d.rolledBack != 1 {
		t.Errorf("committed=%d rolledBack=%d — the row must go down with its entry", d.committed, d.rolledBack)
	}
}

// --- update --------------------------------------------------------------------------------------

func createOne(t *testing.T, uc *MapAssets, ctx context.Context) domain.MapAsset {
	t.Helper()
	a, err := uc.Create(ctx, validInput(), assetActor())
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func strp(s string) *string { return &s }

func TestMapAssetUpdateNoOpWritesNothing(t *testing.T) {
	uc, repo, d, ctx := newAssetUseCase(t)
	a := createOne(t, uc, ctx)
	before := len(d.audits(t))
	_, err := uc.Update(ctx, a.ID, MapAssetPatch{Name: strp(a.Name), Phone: strp(a.Phone),
		CustomValues: map[string]json.RawMessage{"legal_form": json.RawMessage(`"tnhh"`)}}, assetActor())
	if err != nil {
		t.Fatal(err)
	}
	if repo.updates != 0 || len(d.audits(t)) != before {
		t.Errorf("a no-op wrote: updates=%d audits=%d", repo.updates, len(d.audits(t))-before)
	}
}

func TestMapAssetUpdateDiffIsMasked(t *testing.T) {
	uc, repo, d, ctx := newAssetUseCase(t)
	a := createOne(t, uc, ctx)
	got, err := uc.Update(ctx, a.ID, MapAssetPatch{Phone: strp("0900000001"), Address: strp(""),
		Lat: f64(15.8), Lng: f64(108.4)}, assetActor())
	if err != nil {
		t.Fatal(err)
	}
	if repo.updates != 1 || got.Address != "" || got.Lat != 15.8 {
		t.Fatalf("updates=%d row=%+v", repo.updates, got)
	}
	entries := d.audits(t)
	e := entries[len(entries)-1]
	raw, _ := json.Marshal(e.delta)
	if e.action != ActionUpdateMapAsset || strings.Contains(string(raw), "0900000001") || strings.Contains(string(raw), "15.8") {
		t.Errorf("entry = %s", raw)
	}
	sau := e.delta["sau"].(map[string]any)
	if sau["location_changed"] != true || sau["address_changed"] != true || sau["phone"] != "09****0001" {
		t.Errorf("sau = %v", sau)
	}
	if _, ok := sau["name"]; ok {
		t.Error("an unchanged field is in the diff")
	}
}

func TestMapAssetUpdateRefusals(t *testing.T) {
	uc, repo, d, ctx := newAssetUseCase(t)
	a := createOne(t, uc, ctx)
	repo.rows["other"] = &memAsset{a: domain.MapAsset{ID: "other", TaxCode: "0309999999"}}
	auditsBefore := len(d.audits(t))

	for name, tc := range map[string]struct {
		p    MapAssetPatch
		want error
	}{
		"tax taken":          {MapAssetPatch{TaxCode: strp("0309999999")}, commsstore.ErrMapAssetTaxCodeTaken},
		"type disabled":      {MapAssetPatch{AssetTypeCode: strp("ho-kinh-doanh")}, commsstore.ErrMapAssetTypeUnavailable},
		"one coordinate":     {MapAssetPatch{Lat: f64(10)}, domain.ErrMapAssetLocationMissing},
		"bad status":         {MapAssetPatch{Status: strp("dong")}, domain.ErrMapAssetStatusUnknown},
		"custom key unknown": {MapAssetPatch{CustomValues: map[string]json.RawMessage{"x_y": json.RawMessage(`1`)}}, domain.ErrCustomValueUnknownField},
		"required removed":   {MapAssetPatch{CustomValues: map[string]json.RawMessage{"legal_form": json.RawMessage(`null`)}}, domain.ErrCustomValueRequired},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := uc.Update(ctx, a.ID, tc.p, assetActor()); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
	if repo.updates != 0 || len(d.audits(t)) != auditsBefore {
		t.Errorf("refusals wrote: updates=%d audits=%d", repo.updates, len(d.audits(t))-auditsBefore)
	}
	// Moving to ANOTHER live group: its own (empty) schema applies; the old values stay in the row.
	got, err := uc.Update(ctx, a.ID, MapAssetPatch{AssetTypeCode: strp("cho")}, assetActor())
	if err != nil {
		t.Fatal(err)
	}
	if string(got.CustomValues["legal_form"]) != `"tnhh"` {
		t.Errorf("custom values after type change = %v — spec §12.6 keeps them", got.CustomValues)
	}
}

func TestMapAssetEditKeepsADisabledTypeItAlreadyHas(t *testing.T) {
	uc, repo, _, ctx := newAssetUseCase(t)
	a := createOne(t, uc, ctx)
	repo.types["doanh-nghiep"] = false // the group was taken out of use after the asset was filed
	if _, err := uc.Update(ctx, a.ID, MapAssetPatch{Name: strp("Tên mới")}, assetActor()); err != nil {
		t.Fatalf("editing an asset of a disabled group failed: %v", err)
	}
}

// --- confirmation (xác minh) --------------------------------------------------------------------------------

func TestMapAssetConfirmationSetsClearsAndIsIdempotent(t *testing.T) {
	uc, repo, d, ctx := newAssetUseCase(t)
	a := createOne(t, uc, ctx)

	got, err := uc.SetConfirmation(ctx, a.ID, true, assetActor())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Verified || got.VerifiedBy != assetActorCode || got.VerifiedAt == nil {
		t.Fatalf("verified = %+v", got)
	}
	if _, err := uc.SetConfirmation(ctx, a.ID, true, assetActor()); err != nil {
		t.Fatal(err)
	}
	if repo.verifies != 1 {
		t.Errorf("verifying a verified asset wrote again (%d)", repo.verifies)
	}
	got, err = uc.SetConfirmation(ctx, a.ID, false, assetActor())
	if err != nil {
		t.Fatal(err)
	}
	if got.Verified || got.VerifiedAt != nil || got.VerifiedBy != "" {
		t.Errorf("cleared = %+v", got)
	}
	var actions []string
	for _, e := range d.audits(t) {
		actions = append(actions, e.action)
	}
	want := []string{ActionCreateMapAsset, ActionConfirmMapAsset, ActionUnconfirmMapAsset}
	if strings.Join(actions, ",") != strings.Join(want, ",") {
		t.Errorf("actions = %v, want %v", actions, want)
	}
	// The un-verify entry keeps who HAD verified — the row no longer does.
	last := d.audits(t)[2].delta["truoc"].(map[string]any)
	if last["verified_by"] != assetActorCode {
		t.Errorf("un-verify entry lost the verifier: %v", last)
	}
}

// --- delete --------------------------------------------------------------------------------------

func TestMapAssetDeleteNeedsReasonAndHides(t *testing.T) {
	uc, repo, d, ctx := newAssetUseCase(t)
	a := createOne(t, uc, ctx)
	if err := uc.Delete(ctx, a.ID, "  ", assetActor()); !errors.Is(err, domain.ErrThieuLyDoXoa) {
		t.Fatalf("no reason: %v", err)
	}
	if err := uc.Delete(ctx, a.ID, "trùng hồ sơ", assetActor()); err != nil {
		t.Fatal(err)
	}
	if repo.deletes != 1 {
		t.Fatalf("deletes = %d", repo.deletes)
	}
	entries := d.audits(t)
	e := entries[len(entries)-1]
	if e.action != ActionDeleteMapAsset || e.delta["ly_do"] != "trùng hồ sơ" {
		t.Errorf("entry = %+v", e)
	}
	// Second delete: 404, nothing rewritten.
	if err := uc.Delete(ctx, a.ID, "lần hai", assetActor()); !errors.Is(err, commsstore.ErrMapAssetNotFound) {
		t.Errorf("second delete: %v", err)
	}
	// The tax code of a deleted asset is free again (0015's live_tax_code).
	if _, err := uc.Create(ctx, validInput(), assetActor()); err != nil {
		t.Errorf("re-entering a deleted enterprise refused: %v", err)
	}
}

// --- full view -----------------------------------------------------------------------------------

func TestMapAssetRecordFullViewNamesFieldsNotValues(t *testing.T) {
	uc, _, d, ctx := newAssetUseCase(t)
	a := createOne(t, uc, ctx)
	if err := uc.RecordFullView(ctx, a, assetActor()); err != nil {
		t.Fatal(err)
	}
	entries := d.audits(t)
	e := entries[len(entries)-1]
	raw, _ := json.Marshal(e.delta)
	if e.action != ActionReadMapAssetFull || e.actor != assetActorCode {
		t.Errorf("entry = %+v", e)
	}
	if !strings.Contains(string(raw), `"truong":["representative","phone","tax_code"]`) || strings.Contains(string(raw), "0900000000") {
		t.Errorf("delta = %s", raw)
	}
	n := len(entries)
	if err := uc.RecordFullView(ctx, domain.MapAsset{ID: "x", AssetTypeCode: "cho"}, assetActor()); err != nil {
		t.Fatal(err)
	}
	if len(d.audits(t)) != n {
		t.Error("a full view of an asset with no personal field was audited as a disclosure")
	}
}
