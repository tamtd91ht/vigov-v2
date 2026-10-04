package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/platformclient"
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
	at := time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)
	switch {
	case strings.Contains(q, "INSERT INTO map_frame") && v[0] == string(xaA):
		// The SQL must turn the frame ON in both halves; the fake only honours what the SQL says.
		enabled := strings.Contains(q, "VALUES ($1, $2, $3, $4, true,") && strings.Contains(q, "is_enabled = true")
		r := domain.MapFrame{CenterLat: v[1].(float64), CenterLng: v[2].(float64), RadiusKm: v[3].(float64),
			Enabled: enabled, CreatedAt: at, CreatedBy: v[4].(string), UpdatedAt: at, UpdatedBy: v[4].(string)}
		if c.d.row != nil {
			r.CreatedAt, r.CreatedBy = c.d.row.CreatedAt, c.d.row.CreatedBy
		}
		c.d.row = &r
	case strings.Contains(q, "UPDATE map_frame SET is_enabled = false"):
		if v[0] != string(xaA) || c.d.row == nil || !c.d.row.Enabled {
			return driver.RowsAffected(0), nil
		}
		c.d.row.Enabled, c.d.row.UpdatedBy, c.d.row.UpdatedAt = false, v[1].(string), at
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
	cols := []string{"center_lat", "center_lng", "radius_km", "is_enabled", "created_at", "created_by", "updated_at", "updated_by"}
	if c.d.row == nil || v[0] != string(xaA) {
		return &mailRows{cols: cols}, nil
	}
	r := c.d.row
	return &mailRows{cols: cols, rows: [][]driver.Value{{
		r.CenterLat, r.CenterLng, r.RadiusKm, r.Enabled, r.CreatedAt, r.CreatedBy, r.UpdatedAt, r.UpdatedBy,
	}}}, nil
}

type frameTx struct{ d *frameDB }

func (t *frameTx) Commit() error   { t.d.mu.Lock(); t.d.committed++; t.d.mu.Unlock(); return nil }
func (t *frameTx) Rollback() error { t.d.mu.Lock(); t.d.rolledBack++; t.d.mu.Unlock(); return nil }

// fakeDefaults stands in for the platform: one answer, and a record of whether it was asked.
type fakeDefaults struct {
	frame   platformclient.MapFrameDefault
	ok      bool
	err     error
	calls   int
	commune tenant.ID
}

func (f *fakeDefaults) MapFrameDefault(ctx context.Context) (platformclient.MapFrameDefault, bool, error) {
	f.calls++
	f.commune = tenant.MustFrom(ctx)
	return f.frame, f.ok, f.err
}

func newMapFrameUseCaseWith(t *testing.T, d *frameDB, p *fakeDefaults) (*MapFrames, context.Context) {
	t.Helper()
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	h := store.New(db)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewMapFrames(h, commsstore.NewMapFrameStore(h), p, log), tenant.Into(context.Background(), xaA)
}

func newMapFrameUseCase(t *testing.T, d *frameDB) (*MapFrames, context.Context) {
	return newMapFrameUseCaseWith(t, d, &fakeDefaults{})
}

// mapFrameInput is a valid save acknowledging the current notice.
func mapFrameInput(lat, lng, r float64) MapFrameInput {
	return MapFrameInput{CenterLat: lat, CenterLng: lng, RadiusKm: r, NoticeVersion: domain.MapFrameNoticeVersion}
}

func enabledRow(lat, lng, r float64) *domain.MapFrame {
	return &domain.MapFrame{CenterLat: lat, CenterLng: lng, RadiusKm: r, Enabled: true,
		CreatedBy: "CB-00001", UpdatedBy: "CB-00001"}
}

func frameAuditDelta(t *testing.T, s frameStmt) map[string]any {
	t.Helper()
	var delta map[string]any
	if err := json.Unmarshal(s.args[7].([]byte), &delta); err != nil {
		t.Fatal(err)
	}
	return delta
}

func frameSide(delta map[string]any, k string) map[string]any {
	m, _ := delta[k].(map[string]any)
	return m
}

// --- the effective read (K3) ----------------------------------------------------------------------

func TestMapFrameGetUnsetIsConfiguredFalse(t *testing.T) {
	p := &fakeDefaults{}
	uc, ctx := newMapFrameUseCaseWith(t, &frameDB{}, p)
	v, err := uc.Get(ctx)
	if err != nil || v.Configured || v.Default != nil {
		t.Fatalf("Get = %+v, %v — want Configured=false and no error", v, err)
	}
	if p.calls != 1 || p.commune != xaA {
		t.Errorf("platform asked %d times for %q — want once, for the context commune", p.calls, p.commune)
	}
}

func TestMapFrameGetOwnFramePlatformErrorOmitsDefault(t *testing.T) {
	// A broken platform — any error, Unimplemented included — must not touch a commune's own frame:
	// the reply is the commune's frame, with no `default`, and never an error.
	for name, perr := range map[string]error{
		"unavailable":   status.Error(codes.Unavailable, "connection refused"),
		"unimplemented": status.Error(codes.Unimplemented, "unknown method"),
		"plain":         errors.New("platform down"),
	} {
		t.Run(name, func(t *testing.T) {
			d := &frameDB{row: enabledRow(16, 108, 10)}
			p := &fakeDefaults{err: perr}
			uc, ctx := newMapFrameUseCaseWith(t, d, p)
			v, err := uc.Get(ctx)
			if err != nil || !v.Configured || v.Source != domain.MapFrameSourceCommune || v.Default != nil ||
				v.Bounds.MinLat != 15.910168 || v.Bounds.MaxLat != 16.089832 {
				t.Fatalf("Get = %+v, %v — want the own frame, no default, no error", v, err)
			}
			if p.calls != 1 || p.commune != xaA {
				t.Errorf("platform asked %d times for %q — want once, best-effort, for the context commune", p.calls, p.commune)
			}
		})
	}
}

func TestMapFrameGetOwnFrameCarriesThePlatformDefault(t *testing.T) {
	d := &frameDB{row: enabledRow(16, 108, 10)}
	p := &fakeDefaults{ok: true, frame: platformclient.MapFrameDefault{CenterLat: 21.0285, CenterLng: 105.8542, RadiusKm: 12.34}}
	uc, ctx := newMapFrameUseCaseWith(t, d, p)
	v, err := uc.Get(ctx)
	if err != nil || v.Source != domain.MapFrameSourceCommune || v.Frame.CenterLat != 16 || v.Default == nil {
		t.Fatalf("Get = %+v, %v — want the own frame applied and the default attached", v, err)
	}
	want := domain.MapFrame{CenterLat: 21.0285, CenterLng: 105.8542, RadiusKm: 12.3}
	if !domain.SameMapFrame(v.Default.Frame, want) || v.Default.Bounds != want.Bounds() {
		t.Errorf("default = %+v — want %+v with bounds from domain.MapFrame.Bounds", v.Default, want)
	}
	// Commune B, same database: no own frame — the store binds the context commune as $1, so B gets
	// the default AS its frame, never A's row.
	vb, err := uc.Get(tenant.Into(context.Background(), xaB))
	if err != nil || vb.Source != domain.MapFrameSourceDefault || vb.Frame.CenterLat == 16 {
		t.Fatalf("commune B sees %+v, %v", vb, err)
	}
	if got := d.with("FROM map_frame"); len(got) != 2 || got[0].args[0] != string(xaA) || got[1].args[0] != string(xaB) {
		t.Errorf("reads bound %v", got)
	}
}

func TestMapFrameGetFallsBackToThePlatformDefault(t *testing.T) {
	def := platformclient.MapFrameDefault{CenterLat: 21.0285, CenterLng: 105.8542, RadiusKm: 12.34}
	for name, row := range map[string]*domain.MapFrame{
		"no row":       nil,
		"disabled row": {CenterLat: 16, CenterLng: 108, RadiusKm: 10, Enabled: false, UpdatedBy: "CB-00001"},
	} {
		t.Run(name, func(t *testing.T) {
			p := &fakeDefaults{frame: def, ok: true}
			uc, ctx := newMapFrameUseCaseWith(t, &frameDB{row: row}, p)
			v, err := uc.Get(ctx)
			if err != nil || !v.Configured || v.Source != domain.MapFrameSourceDefault || v.Default == nil {
				t.Fatalf("Get = %+v, %v", v, err)
			}
			// Rounded to the stored scales, bounds by the SAME function as a commune row.
			want := domain.MapFrame{CenterLat: 21.0285, CenterLng: 105.8542, RadiusKm: 12.3}
			if !domain.SameMapFrame(v.Frame, want) || v.Bounds != want.Bounds() || v.Default.Bounds != want.Bounds() {
				t.Errorf("default view = %+v, want %+v / %+v", v, want, want.Bounds())
			}
			if v.Frame.CenterLat == 16 {
				t.Error("a disabled row's frame was applied")
			}
		})
	}
}

func TestMapFrameGetNoDefaultConfigured(t *testing.T) {
	uc, ctx := newMapFrameUseCaseWith(t, &frameDB{}, &fakeDefaults{ok: false})
	if v, err := uc.Get(ctx); err != nil || v.Configured || v.Source != "" {
		t.Fatalf("Get = %+v, %v", v, err)
	}
}

func TestMapFrameGetUnimplementedPlatformIsNoDefault(t *testing.T) {
	// Rollout safety: a platform without the RPC yet is "no default", never a 503.
	p := &fakeDefaults{err: fmt.Errorf("platformclient: GetMapFrameDefault: %w",
		status.Error(codes.Unimplemented, "unknown method GetMapFrameDefault"))}
	uc, ctx := newMapFrameUseCaseWith(t, &frameDB{}, p)
	v, err := uc.Get(ctx)
	if err != nil || v.Configured {
		t.Fatalf("Get = %+v, %v — want configured=false, no error", v, err)
	}
}

func TestMapFrameGetOtherPlatformErrorIsUnavailable(t *testing.T) {
	for name, perr := range map[string]error{
		"unavailable": status.Error(codes.Unavailable, "connection refused"),
		"deadline":    status.Error(codes.DeadlineExceeded, "slow"),
		"no commune":  tenant.ErrNoTenant,
	} {
		t.Run(name, func(t *testing.T) {
			uc, ctx := newMapFrameUseCaseWith(t, &frameDB{}, &fakeDefaults{err: perr})
			v, err := uc.Get(ctx)
			if !errors.Is(err, ErrMapFrameDefaultUnavailable) || v.Configured {
				t.Fatalf("Get = %+v, %v — want ErrMapFrameDefaultUnavailable, never a guessed frame", v, err)
			}
		})
	}
}

func TestMapFrameGetOutOfRuleDefaultIsNoDefault(t *testing.T) {
	// 0.04 km rounds to 0.0 — the commune's own rule refuses it; never clamped into a frame.
	p := &fakeDefaults{ok: true, frame: platformclient.MapFrameDefault{CenterLat: 16, CenterLng: 108, RadiusKm: 0.04}}
	uc, ctx := newMapFrameUseCaseWith(t, &frameDB{}, p)
	if v, err := uc.Get(ctx); err != nil || v.Configured {
		t.Fatalf("Get = %+v, %v", v, err)
	}
}

// --- the save --------------------------------------------------------------------------------------

func TestMapFrameFirstSaveWritesAndAuditsInOneTransaction(t *testing.T) {
	d := &frameDB{}
	p := &fakeDefaults{}
	uc, ctx := newMapFrameUseCaseWith(t, d, p)
	v, err := uc.Save(ctx, mapFrameInput(15.7305074, 108.37811, 12.34), staffActor)
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
	delta := frameAuditDelta(t, au[0])
	if _, ok := delta["truoc"]; ok || frameSide(delta, "sau")["radius_km"] != 12.3 || frameSide(delta, "sau")["is_enabled"] != true {
		t.Errorf("first-save delta = %v", delta)
	}
	// K4: the entry records the acknowledgement and the notice version.
	if delta["notice_version"] != domain.MapFrameNoticeVersion || delta["notice_acknowledged"] != true {
		t.Errorf("delta carries no acknowledgement: %v", delta)
	}
	if !v.Configured || v.Source != domain.MapFrameSourceCommune || v.Frame.UpdatedBy != "CB-00123" ||
		v.Frame.UpdatedAt.IsZero() || !v.Frame.Enabled {
		t.Errorf("view = %+v", v)
	}
	// The reply has GET's shape: the default is attached best-effort, after the commit.
	if p.calls != 1 || v.Default != nil {
		t.Errorf("platform asked %d times, default %+v — want once, none configured", p.calls, v.Default)
	}
}

func TestMapFrameSaveReplyAttachesTheDefaultBestEffort(t *testing.T) {
	def := platformclient.MapFrameDefault{CenterLat: 21, CenterLng: 105.8, RadiusKm: 8}
	for name, tc := range map[string]struct {
		p           *fakeDefaults
		wantDefault bool
	}{
		"default exists": {&fakeDefaults{ok: true, frame: def}, true},
		"platform down":  {&fakeDefaults{err: status.Error(codes.Unavailable, "down")}, false},
	} {
		t.Run(name, func(t *testing.T) {
			d := &frameDB{}
			uc, ctx := newMapFrameUseCaseWith(t, d, tc.p)
			v, err := uc.Save(ctx, mapFrameInput(16, 108, 10), staffActor)
			if err != nil || v.Source != domain.MapFrameSourceCommune || (v.Default != nil) != tc.wantDefault {
				t.Fatalf("Save = %+v, %v — want the saved frame, default attached=%v", v, err, tc.wantDefault)
			}
			// A platform failure after the commit never undoes or fails the save.
			if d.committed != 1 || len(d.with("INSERT INTO audit_log")) != 1 {
				t.Errorf("committed %d, entries %d", d.committed, len(d.with("INSERT INTO audit_log")))
			}
		})
	}
}

func TestMapFrameChangeAuditsBeforeAndAfter(t *testing.T) {
	d := &frameDB{row: enabledRow(16, 108, 10)}
	uc, ctx := newMapFrameUseCase(t, d)
	if _, err := uc.Save(ctx, mapFrameInput(16, 108, 15), staffActor); err != nil {
		t.Fatal(err)
	}
	au := d.with("INSERT INTO audit_log")
	if len(au) != 1 {
		t.Fatalf("%d entries", len(au))
	}
	delta := frameAuditDelta(t, au[0])
	if frameSide(delta, "truoc")["radius_km"] != 10.0 || frameSide(delta, "sau")["radius_km"] != 15.0 ||
		frameSide(delta, "truoc")["center_lat"] != 16.0 || delta["notice_version"] != domain.MapFrameNoticeVersion {
		t.Errorf("delta = %v", delta)
	}
	if d.row.CreatedBy != "CB-00001" || d.row.UpdatedBy != "CB-00123" {
		t.Errorf("row = %+v — created_by is fixed, updated_by moves", d.row)
	}
	if len(d.with("FOR UPDATE")) != 1 {
		t.Error("the save did not lock the row before deciding")
	}
}

func TestMapFrameSaveReEnablesADisabledFrame(t *testing.T) {
	// The same values onto a row left by "Về mặc định" are a CHANGE: the commune applies its own frame
	// again. The upsert must say so in SQL (0017 "OWED BY GO"), and the entry must record it.
	row := enabledRow(16, 108, 10)
	row.Enabled = false
	d := &frameDB{row: row}
	uc, ctx := newMapFrameUseCase(t, d)
	v, err := uc.Save(ctx, mapFrameInput(16, 108, 10), staffActor)
	if err != nil {
		t.Fatal(err)
	}
	if !d.row.Enabled || v.Source != domain.MapFrameSourceCommune {
		t.Fatalf("row = %+v, view = %+v — the save left the frame off", d.row, v)
	}
	au := d.with("INSERT INTO audit_log")
	if len(au) != 1 {
		t.Fatalf("%d entries, want 1", len(au))
	}
	delta := frameAuditDelta(t, au[0])
	if frameSide(delta, "truoc")["is_enabled"] != false || frameSide(delta, "sau")["is_enabled"] != true {
		t.Errorf("delta = %v", delta)
	}
}

func TestMapFrameSaveTakesTheCommuneLockBeforeReading(t *testing.T) {
	// FOR UPDATE cannot lock a row that is not there yet: on a FIRST save only the commune's advisory
	// lock makes a concurrent first save wait and then read the row, so its entry carries `truoc`.
	// The order is the property — a lock taken after the read serialises nothing that matters.
	d := &frameDB{}
	uc, ctx := newMapFrameUseCase(t, d)
	if _, err := uc.Save(ctx, mapFrameInput(16, 108, 10), staffActor); err != nil {
		t.Fatal(err)
	}
	lockAt, readAt := -1, -1
	for i, s := range d.stmts {
		switch {
		case lockAt < 0 && strings.Contains(s.sql, "pg_advisory_xact_lock"):
			lockAt = i
			if !strings.Contains(s.sql, "'t:' || $1 || ':map-frame'") || len(s.args) != 1 || s.args[0] != string(xaA) {
				t.Errorf("lock = %q %v — want key t:<commune>:map-frame, the commune as $1", s.sql, s.args)
			}
		case readAt < 0 && strings.Contains(s.sql, "FOR UPDATE"):
			readAt = i
		}
	}
	if lockAt < 0 || readAt < 0 || lockAt > readAt {
		t.Fatalf("lock at %d, FOR UPDATE read at %d — the lock must come first", lockAt, readAt)
	}
	if d.begun != 1 || d.committed != 1 {
		t.Errorf("tx begun/committed = %d/%d — the lock must sit in the save's own transaction", d.begun, d.committed)
	}
}

func TestMapFrameLockFailureWritesNothing(t *testing.T) {
	d := &frameDB{failOnContain: "pg_advisory_xact_lock"}
	uc, ctx := newMapFrameUseCase(t, d)
	if _, err := uc.Save(ctx, mapFrameInput(16, 108, 10), staffActor); err == nil {
		t.Fatal("saved without the commune's lock")
	}
	if len(d.with("FOR UPDATE")) != 0 || len(d.with("INSERT INTO")) != 0 || d.committed != 0 {
		t.Error("the save went on after the lock failed")
	}
}

func TestMapFrameUnchangedSaveWritesNothing(t *testing.T) {
	d := &frameDB{row: enabledRow(16, 108, 10)}
	uc, ctx := newMapFrameUseCase(t, d)
	// 10.04 rounds to 10.0: the same stored value, so the same state.
	v, err := uc.Save(ctx, mapFrameInput(16, 108, 10.04), staffActor)
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
		"outside":        {mapFrameInput(16, 109.51, 10), true, domain.ErrMapFrameCenterOutsideMainland},
		"radius 50.1":    {mapFrameInput(16, 108, 50.1), true, domain.ErrMapFrameRadiusOutOfRange},
		"radius 0":       {mapFrameInput(16, 108, 0), true, domain.ErrMapFrameRadiusOutOfRange},
		"no actor":       {mapFrameInput(16, 108, 10), false, ErrMissingActor},
		"notice missing": {MapFrameInput{CenterLat: 16, CenterLng: 108, RadiusKm: 10}, true, domain.ErrMapFrameNoticeNotAcknowledged},
		"notice stale":   {MapFrameInput{CenterLat: 16, CenterLng: 108, RadiusKm: 10, NoticeVersion: "2026-01-01.1"}, true, domain.ErrMapFrameNoticeNotAcknowledged},
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
	if _, err := uc.Save(ctx, mapFrameInput(16, 108, 10), staffActor); err == nil {
		t.Fatal("saved with no audit entry")
	}
	if d.committed != 0 || d.rolledBack != 1 {
		t.Errorf("committed/rolled back = %d/%d — the frame must not outlive its missing entry", d.committed, d.rolledBack)
	}
}

// --- "Về mặc định" -------------------------------------------------------------------------------

func TestMapFrameResetDisablesAuditsAndReturnsTheDefault(t *testing.T) {
	d := &frameDB{row: enabledRow(16, 108, 10)}
	p := &fakeDefaults{ok: true, frame: platformclient.MapFrameDefault{CenterLat: 21, CenterLng: 105.8, RadiusKm: 8}}
	uc, ctx := newMapFrameUseCaseWith(t, d, p)
	v, err := uc.Reset(ctx, domain.MapFrameNoticeVersion, staffActor)
	if err != nil {
		t.Fatal(err)
	}
	if d.begun != 1 || d.committed != 1 {
		t.Errorf("tx begun/committed = %d/%d", d.begun, d.committed)
	}
	// An UPDATE of the kept row; never a DELETE, never an upsert.
	upd := d.with("UPDATE map_frame SET is_enabled = false")
	if len(upd) != 1 || upd[0].args[0] != string(xaA) || upd[0].args[1] != "CB-00123" ||
		len(d.with("DELETE")) != 0 || len(d.with("INSERT INTO map_frame")) != 0 {
		t.Fatalf("statements = %+v", d.stmts)
	}
	if d.row.Enabled || d.row.RadiusKm != 10 || d.row.CenterLat != 16 {
		t.Errorf("row = %+v — off, with its last values kept", d.row)
	}
	au := d.with("INSERT INTO audit_log")
	if len(au) != 1 || au[0].args[4] != ActionResetMapFrame || au[0].args[1] != "CB-00123" {
		t.Fatalf("entries = %+v", au)
	}
	delta := frameAuditDelta(t, au[0])
	if frameSide(delta, "truoc")["is_enabled"] != true || frameSide(delta, "truoc")["radius_km"] != 10.0 ||
		frameSide(delta, "sau")["is_enabled"] != false || delta["notice_version"] != domain.MapFrameNoticeVersion ||
		delta["notice_acknowledged"] != true {
		t.Errorf("delta = %v", delta)
	}
	if !v.Configured || v.Source != domain.MapFrameSourceDefault || v.Frame.CenterLat != 21 {
		t.Errorf("reply = %+v — want the effective frame, the platform default", v)
	}
}

func TestMapFrameResetIsIdempotent(t *testing.T) {
	off := enabledRow(16, 108, 10)
	off.Enabled = false
	for name, row := range map[string]*domain.MapFrame{"no row": nil, "already off": off} {
		t.Run(name, func(t *testing.T) {
			d := &frameDB{row: row}
			uc, ctx := newMapFrameUseCaseWith(t, d, &fakeDefaults{})
			v, err := uc.Reset(ctx, domain.MapFrameNoticeVersion, staffActor)
			if err != nil || v.Configured {
				t.Fatalf("Reset = %+v, %v — want the effective frame (none here)", v, err)
			}
			if len(d.with("UPDATE map_frame")) != 0 || len(d.with("INSERT INTO audit_log")) != 0 {
				t.Error("a no-op reset wrote or audited")
			}
		})
	}
	// Twice in a row: one write, one entry.
	d := &frameDB{row: enabledRow(16, 108, 10)}
	uc, ctx := newMapFrameUseCase(t, d)
	for i := 0; i < 2; i++ {
		if _, err := uc.Reset(ctx, domain.MapFrameNoticeVersion, staffActor); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.with("UPDATE map_frame")) != 1 || len(d.with("INSERT INTO audit_log")) != 1 {
		t.Errorf("two resets: %d updates, %d entries — want 1/1", len(d.with("UPDATE map_frame")), len(d.with("INSERT INTO audit_log")))
	}
}

func TestMapFrameResetRefusalsRunNoStatement(t *testing.T) {
	for name, tc := range map[string]struct {
		notice string
		actor  bool
		want   error
	}{
		"notice missing": {"", true, domain.ErrMapFrameNoticeNotAcknowledged},
		"notice stale":   {"2026-01-01.1", true, domain.ErrMapFrameNoticeNotAcknowledged},
		"no actor":       {domain.MapFrameNoticeVersion, false, ErrMissingActor},
	} {
		t.Run(name, func(t *testing.T) {
			d := &frameDB{row: enabledRow(16, 108, 10)}
			p := &fakeDefaults{}
			uc, ctx := newMapFrameUseCaseWith(t, d, p)
			actor := staffActor
			if !tc.actor {
				actor.ID = ""
			}
			if _, err := uc.Reset(ctx, tc.notice, actor); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if d.begun != 0 || len(d.stmts) != 0 || p.calls != 0 || !d.row.Enabled {
				t.Errorf("a refusal reached the database or the platform")
			}
		})
	}
}

func TestMapFrameResetAuditFailureRollsBack(t *testing.T) {
	d := &frameDB{row: enabledRow(16, 108, 10), failOnContain: "INSERT INTO audit_log"}
	uc, ctx := newMapFrameUseCase(t, d)
	if _, err := uc.Reset(ctx, domain.MapFrameNoticeVersion, staffActor); err == nil {
		t.Fatal("reset with no audit entry")
	}
	if d.committed != 0 || d.rolledBack != 1 {
		t.Errorf("committed/rolled back = %d/%d", d.committed, d.rolledBack)
	}
}
