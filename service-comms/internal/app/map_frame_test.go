package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// A fake database/sql driver under the REAL MapFrameStore — mail_settings_test.go's approach: the
// properties worth proving (one transaction for the write and its entry, the commune as $1, nothing
// written when nothing moved, a refusal runs no statement) live in the SQL and the transaction
// boundaries. WHAT IT DOES NOT PROVE: anything PostgreSQL does — the CHECKs, the trigger, the partition
// routing: internal/store/map_frame_pg_test.go.

type frameStmt struct {
	sql  string
	args []driver.Value
}

type frameDB struct {
	mu    sync.Mutex
	stmts []frameStmt

	begun, committed, rolledBack int
	// row is the stored frame of commune A; nil = none. Any other commune has none.
	row           *domain.MapFrame
	failOnContain string
}

func (d *frameDB) with(sub string) []frameStmt {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []frameStmt
	for _, s := range d.stmts {
		if strings.Contains(s.sql, sub) {
			out = append(out, s)
		}
	}
	return out
}

func (d *frameDB) Connect(context.Context) (driver.Conn, error) { return &frameConn{d: d}, nil }
func (d *frameDB) Driver() driver.Driver                        { return mailDriverOpen{} }

type frameConn struct{ d *frameDB }

func (c *frameConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no Prepare") }
func (c *frameConn) Close() error                        { return nil }
func (c *frameConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *frameConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.mu.Lock()
	c.d.begun++
	c.d.mu.Unlock()
	return &frameTx{d: c.d}, nil
}

func frameArgs(args []driver.NamedValue) []driver.Value {
	out := make([]driver.Value, 0, len(args))
	for _, a := range args {
		out = append(out, a.Value)
	}
	return out
}

func (c *frameConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	v := frameArgs(args)
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	c.d.stmts = append(c.d.stmts, frameStmt{sql: q, args: v})
	if c.d.failOnContain != "" && strings.Contains(q, c.d.failOnContain) {
		return nil, errors.New("fake driver: this statement is built to fail")
	}
	if strings.Contains(q, "INSERT INTO map_frame") && v[0] == string(xaA) {
		at := time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)
		r := domain.MapFrame{CenterLat: v[1].(float64), CenterLng: v[2].(float64), RadiusKm: v[3].(float64),
			CreatedAt: at, CreatedBy: v[4].(string), UpdatedAt: at, UpdatedBy: v[4].(string)}
		if c.d.row != nil {
			r.CreatedAt, r.CreatedBy = c.d.row.CreatedAt, c.d.row.CreatedBy
		}
		c.d.row = &r
	}
	return driver.RowsAffected(1), nil
}

func (c *frameConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	v := frameArgs(args)
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	c.d.stmts = append(c.d.stmts, frameStmt{sql: q, args: v})
	if !strings.Contains(q, "FROM map_frame") {
		return nil, fmt.Errorf("fake driver: no answer for %q", q)
	}
	cols := []string{"center_lat", "center_lng", "radius_km", "created_at", "created_by", "updated_at", "updated_by"}
	if c.d.row == nil || v[0] != string(xaA) {
		return &mailRows{cols: cols}, nil
	}
	r := c.d.row
	return &mailRows{cols: cols, rows: [][]driver.Value{{
		r.CenterLat, r.CenterLng, r.RadiusKm, r.CreatedAt, r.CreatedBy, r.UpdatedAt, r.UpdatedBy,
	}}}, nil
}

type frameTx struct{ d *frameDB }

func (t *frameTx) Commit() error   { t.d.mu.Lock(); t.d.committed++; t.d.mu.Unlock(); return nil }
func (t *frameTx) Rollback() error { t.d.mu.Lock(); t.d.rolledBack++; t.d.mu.Unlock(); return nil }

func newMapFrameUseCase(t *testing.T, d *frameDB) (*MapFrames, context.Context) {
	t.Helper()
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	h := store.New(db)
	return NewMapFrames(h, commsstore.NewMapFrameStore(h)), tenant.Into(context.Background(), xaA)
}

func TestMapFrameGetUnsetIsConfiguredFalse(t *testing.T) {
	uc, ctx := newMapFrameUseCase(t, &frameDB{})
	v, err := uc.Get(ctx)
	if err != nil || v.Configured {
		t.Fatalf("Get = %+v, %v — want Configured=false and no error", v, err)
	}
}

func TestMapFrameGetSetCarriesBounds(t *testing.T) {
	d := &frameDB{row: &domain.MapFrame{CenterLat: 16, CenterLng: 108, RadiusKm: 10, UpdatedBy: "CB-00123"}}
	uc, ctx := newMapFrameUseCase(t, d)
	v, err := uc.Get(ctx)
	if err != nil || !v.Configured || v.Bounds.MinLat != 15.910168 || v.Bounds.MaxLat != 16.089832 {
		t.Fatalf("Get = %+v, %v", v, err)
	}
	// Commune B, same database: nothing — the store binds the context commune as $1.
	if v, err := uc.Get(tenant.Into(context.Background(), xaB)); err != nil || v.Configured {
		t.Fatalf("commune B sees %+v, %v", v, err)
	}
	if got := d.with("FROM map_frame"); len(got) != 2 || got[0].args[0] != string(xaA) || got[1].args[0] != string(xaB) {
		t.Errorf("reads bound %v", got)
	}
}

func TestMapFrameFirstSaveWritesAndAuditsInOneTransaction(t *testing.T) {
	d := &frameDB{}
	uc, ctx := newMapFrameUseCase(t, d)
	v, err := uc.Save(ctx, MapFrameInput{CenterLat: 15.7305074, CenterLng: 108.37811, RadiusKm: 12.34}, staffActor)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if d.begun != 1 || d.committed != 1 || d.rolledBack != 0 {
		t.Fatalf("tx begun/committed/rolled back = %d/%d/%d, want 1/1/0", d.begun, d.committed, d.rolledBack)
	}
	up := d.with("INSERT INTO map_frame")
	if len(up) != 1 || up[0].args[0] != string(xaA) {
		t.Fatalf("upserts = %+v — want one, commune A as $1", up)
	}
	// Rounded to the stored scales BEFORE the write.
	if up[0].args[1] != 15.730507 || up[0].args[3] != 12.3 || up[0].args[4] != "CB-00123" {
		t.Errorf("upsert args = %v", up[0].args)
	}
	au := d.with("INSERT INTO audit_log")
	if len(au) != 1 {
		t.Fatalf("%d audit entries, want 1", len(au))
	}
	// (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
	a := au[0].args
	if a[0] != string(xaA) || a[1] != "CB-00123" || a[3] != "10.0.0.7" || a[4] != ActionSaveMapFrame ||
		a[5] != domain.MapFrameSubject {
		t.Errorf("audit args = %v", a)
	}
	var delta map[string]map[string]float64
	if err := json.Unmarshal(a[7].([]byte), &delta); err != nil {
		t.Fatal(err)
	}
	if _, ok := delta["truoc"]; ok || delta["sau"]["radius_km"] != 12.3 {
		t.Errorf("first-save delta = %v", delta)
	}
	if !v.Configured || v.Frame.UpdatedBy != "CB-00123" || v.Frame.UpdatedAt.IsZero() {
		t.Errorf("view = %+v", v)
	}
}

func TestMapFrameChangeAuditsBeforeAndAfter(t *testing.T) {
	d := &frameDB{row: &domain.MapFrame{CenterLat: 16, CenterLng: 108, RadiusKm: 10, CreatedBy: "CB-00001", UpdatedBy: "CB-00001"}}
	uc, ctx := newMapFrameUseCase(t, d)
	if _, err := uc.Save(ctx, MapFrameInput{CenterLat: 16, CenterLng: 108, RadiusKm: 15}, staffActor); err != nil {
		t.Fatal(err)
	}
	au := d.with("INSERT INTO audit_log")
	if len(au) != 1 {
		t.Fatalf("%d entries", len(au))
	}
	var delta map[string]map[string]float64
	_ = json.Unmarshal(au[0].args[7].([]byte), &delta)
	if delta["truoc"]["radius_km"] != 10 || delta["sau"]["radius_km"] != 15 || delta["truoc"]["center_lat"] != 16 {
		t.Errorf("delta = %v", delta)
	}
	if d.row.CreatedBy != "CB-00001" || d.row.UpdatedBy != "CB-00123" {
		t.Errorf("row = %+v — created_by is fixed, updated_by moves", d.row)
	}
	if len(d.with("FOR UPDATE")) != 1 {
		t.Error("the save did not lock the row before deciding")
	}
}

func TestMapFrameUnchangedSaveWritesNothing(t *testing.T) {
	d := &frameDB{row: &domain.MapFrame{CenterLat: 16, CenterLng: 108, RadiusKm: 10, CreatedBy: "CB-00001", UpdatedBy: "CB-00001"}}
	uc, ctx := newMapFrameUseCase(t, d)
	// 10.04 rounds to 10.0: the same stored value, so the same state.
	v, err := uc.Save(ctx, MapFrameInput{CenterLat: 16, CenterLng: 108, RadiusKm: 10.04}, staffActor)
	if err != nil || !v.Configured || v.Frame.UpdatedBy != "CB-00001" {
		t.Fatalf("Save = %+v, %v", v, err)
	}
	if len(d.with("INSERT INTO map_frame")) != 0 || len(d.with("INSERT INTO audit_log")) != 0 {
		t.Error("an unchanged save wrote or audited")
	}
}

func TestMapFrameRefusalsRunNoStatement(t *testing.T) {
	for name, tc := range map[string]struct {
		in    MapFrameInput
		actor bool
		want  error
	}{
		"outside":  {MapFrameInput{CenterLat: 16, CenterLng: 109.51, RadiusKm: 10}, true, domain.ErrMapFrameCenterOutsideMainland},
		"radius":   {MapFrameInput{CenterLat: 16, CenterLng: 108, RadiusKm: 30.1}, true, domain.ErrMapFrameRadiusOutOfRange},
		"no actor": {MapFrameInput{CenterLat: 16, CenterLng: 108, RadiusKm: 10}, false, ErrMissingActor},
	} {
		t.Run(name, func(t *testing.T) {
			d := &frameDB{}
			uc, ctx := newMapFrameUseCase(t, d)
			actor := staffActor
			if !tc.actor {
				actor.ID = ""
			}
			if _, err := uc.Save(ctx, tc.in, actor); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if d.begun != 0 || len(d.stmts) != 0 {
				t.Errorf("a refusal reached the database: %d tx, %d statements", d.begun, len(d.stmts))
			}
		})
	}
}

func TestMapFrameAuditFailureRollsBackTheWrite(t *testing.T) {
	d := &frameDB{failOnContain: "INSERT INTO audit_log"}
	uc, ctx := newMapFrameUseCase(t, d)
	if _, err := uc.Save(ctx, MapFrameInput{CenterLat: 16, CenterLng: 108, RadiusKm: 10}, staffActor); err == nil {
		t.Fatal("saved with no audit entry")
	}
	if d.committed != 0 || d.rolledBack != 1 {
		t.Errorf("committed/rolled back = %d/%d — the frame must not outlive its missing entry", d.committed, d.rolledBack)
	}
}
