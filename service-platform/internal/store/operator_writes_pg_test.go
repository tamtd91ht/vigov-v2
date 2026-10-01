package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/vihat/vigov/core/page"
	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// Integration tests for the operator registry writes against a real PostgreSQL — skipped without
// VIGOV_TEST_DSN, like every *_pg_test.go here. What they defend is what only the database can
// show: the audit entry commits WITH the change or not at all (rule 6 invariant 3), the CHECK and
// unique constraints the writes rely on, and the lock that makes the duplicate-name check hold.

const (
	seededProvinceID = "66QW36RCJ7GVW79W8GJRYH3ZNR" // migration 0005, the catalogue row for Đà Nẵng
	ulidNew          = "01JD8ZQK9M3NPXR7TVWYB2C4EH"
)

var operatorFake = domain.OperatorActor{Code: "VH-00001", IP: "203.0.113.7"}

func inCommune(id string) context.Context {
	return tenant.Into(context.Background(), tenant.ID(id))
}

func pageFirst(limit string) (page.Request, error) {
	return page.New(CommuneOrder, "", "", limit, "")
}

type auditRow struct {
	actorID, actorKind, actorIP, action, subject string
	delta                                        map[string]any
}

func auditRows(t *testing.T, db *sql.DB, tenantID string) []auditRow {
	t.Helper()
	rows, err := db.Query(`SELECT actor_id, actor_kind, actor_ip, action, subject, delta
		FROM audit_log WHERE tenant_id = $1 ORDER BY id`, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var r auditRow
		var raw []byte
		if err := rows.Scan(&r.actorID, &r.actorKind, &r.actorIP, &r.action, &r.subject, &raw); err != nil {
			t.Fatal(err)
		}
		_ = json.Unmarshal(raw, &r.delta)
		out = append(out, r)
	}
	return out
}

func count(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPgCreateCommuneWritesRowsAndAuditTogether(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	w := NewRegistryWriter(corestore.New(db))

	c, err := w.CreateCommune(inCommune(ulidNew),
		NewCommune{Name: "Xã Mới", ProvinceID: seededProvinceID, Host: "xamoi.vigov.vn"}, operatorFake)
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != ulidNew || c.Province != "Đà Nẵng" {
		t.Errorf("created %+v", c)
	}
	if count(t, db, `SELECT count(*) FROM tenant_domain WHERE host = 'xamoi.vigov.vn' AND tenant_id = $1 AND la_chinh`, ulidNew) != 1 {
		t.Error("primary host not written")
	}
	a := auditRows(t, db, ulidNew)
	if len(a) != 1 || a[0].action != ActionCreateCommune || a[0].actorID != "VH-00001" ||
		a[0].actorKind != domain.AuditKindOperator || a[0].actorIP != operatorFake.IP || a[0].subject != "Xã Mới" ||
		a[0].delta["tinh_thanh"] != "Đà Nẵng" {
		t.Fatalf("audit = %+v", a)
	}
}

// The tenant INSERT succeeds, the domain INSERT fails on migration 0007's CHECK — and the tenant row
// and the audit entry must both be gone. This is the "never half-processed" property (rule 2).
func TestPgCreateCommuneRollsBackWhole(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	w := NewRegistryWriter(corestore.New(db))
	_, err := w.CreateCommune(inCommune(ulidNew),
		NewCommune{Name: "Xã Mới", ProvinceID: seededProvinceID, Host: "admin.vigov.vn"}, operatorFake)
	if err == nil {
		t.Fatal("a reserved host passed the database CHECK")
	}
	if count(t, db, `SELECT count(*) FROM tenant WHERE id = $1`, ulidNew) != 0 ||
		count(t, db, `SELECT count(*) FROM audit_log WHERE tenant_id = $1`, ulidNew) != 0 {
		t.Fatal("a failed create left a tenant row or an audit entry behind")
	}
}

func TestPgCreateCommuneRefusals(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true) // legacy province spelling 'Thành phố Đà Nẵng'
	themHost(t, db, "thangbinh.vigov.vn", ulidA, true)
	w := NewRegistryWriter(corestore.New(db))

	for name, c := range map[string]struct {
		in   NewCommune
		want error
	}{
		"unknown province":           {NewCommune{Name: "Xã X", ProvinceID: "01JD8ZQK9M3NPXR7TVWYB2C4EZ", Host: "x.vigov.vn"}, ErrProvinceNotFound},
		"host held by another":       {NewCommune{Name: "Xã X", ProvinceID: seededProvinceID, Host: "thangbinh.vigov.vn"}, ErrDomainTaken},
		"same name, legacy province": {NewCommune{Name: "XÃ THĂNG  BÌNH", ProvinceID: seededProvinceID, Host: "x.vigov.vn"}, ErrDuplicateName},
	} {
		if _, err := w.CreateCommune(inCommune(ulidNew), c.in, operatorFake); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
	if count(t, db, `SELECT count(*) FROM tenant WHERE id = $1`, ulidNew) != 0 {
		t.Error("a refused create wrote a tenant row")
	}
	if _, err := w.CreateCommune(inCommune(ulidNew), NewCommune{Name: "Xã X", ProvinceID: seededProvinceID,
		Host: "x.vigov.vn"}, domain.OperatorActor{}); !errors.Is(err, domain.ErrNoActor) {
		t.Errorf("empty actor: %v", err)
	}
}

func TestPgDomainsNameActivationMiniApp(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themHost(t, db, "thangbinh.vigov.vn", ulidA, true)
	themXa(t, db, ulidB, "Xã Tân Phú", true)
	w := NewRegistryWriter(corestore.New(db))
	ctx := inCommune(ulidA)

	if err := w.AddDomain(ctx, "thangbinh.example.gov.vn", operatorFake); err != nil {
		t.Fatal(err)
	}
	if err := w.AddDomain(inCommune(ulidB), "thangbinh.example.gov.vn", operatorFake); !errors.Is(err, ErrDomainTaken) {
		t.Errorf("host moved to a second commune: %v", err)
	}
	if changed, err := w.SetPrimaryDomain(ctx, "thangbinh.example.gov.vn", operatorFake); err != nil || !changed {
		t.Fatalf("set primary: %v %v", changed, err)
	}
	if count(t, db, `SELECT count(*) FROM tenant_domain WHERE tenant_id = $1 AND la_chinh`, ulidA) != 1 {
		t.Error("not exactly one primary host")
	}
	if changed, _ := w.SetPrimaryDomain(ctx, "thangbinh.example.gov.vn", operatorFake); changed {
		t.Error("setting the current primary again must be a no-op")
	}
	if _, err := w.SetPrimaryDomain(ctx, "other.vigov.vn", operatorFake); !errors.Is(err, ErrDomainNotInCommune) {
		t.Errorf("foreign host as primary: %v", err)
	}
	if _, err := w.CorrectName(inCommune(ulidB), "xã thăng bình", "lỗi gõ", operatorFake); !errors.Is(err, ErrDuplicateName) {
		t.Errorf("rename onto another commune's name: %v", err)
	}
	if changed, err := w.CorrectName(ctx, "Xã Thăng Bình (sửa)", "Sửa lỗi gõ", operatorFake); err != nil || !changed {
		t.Fatalf("correct name: %v %v", changed, err)
	}
	app, err := w.AttachMiniApp(ctx, "3291993990104489440", "", operatorFake)
	if err != nil || app.CreatedBy != "VH-00001" || app.Mode != domain.CheDoRieng {
		t.Fatalf("attach: %+v %v", app, err)
	}
	if _, err := w.AttachMiniApp(inCommune(ulidB), "3291993990104489440", "", operatorFake); !errors.Is(err, ErrMiniAppTaken) {
		t.Errorf("App ID re-bound: %v", err)
	}
	changed, hosts, err := w.SetActivation(ctx, false, "Sáp nhập theo nghị quyết", operatorFake)
	if err != nil || !changed || len(hosts) != 2 {
		t.Fatalf("deactivate: %v %v %v", changed, hosts, err)
	}
	if err := w.AddDomain(ctx, "late.vigov.vn", operatorFake); !errors.Is(err, ErrCommuneInactive) {
		t.Errorf("domain added to an inactive commune: %v", err)
	}
	if _, ok := NewDirectory(db, "").ByHost(context.Background(), "thangbinh.vigov.vn"); ok {
		t.Error("an inactive commune's host still resolves")
	}

	want := []string{ActionAddDomain, ActionSetPrimaryDomain, ActionCorrectName, ActionAttachMiniApp, ActionDeactivate}
	got := auditRows(t, db, ulidA)
	if len(got) != len(want) {
		t.Fatalf("audit rows = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, a := range got {
		if a.action != want[i] || a.actorID != "VH-00001" || a.actorKind != domain.AuditKindOperator {
			t.Errorf("entry %d = %+v, want action %s by VH-00001", i, a, want[i])
		}
	}
	if got[2].delta["ly_do"] != "Sửa lỗi gõ" {
		t.Errorf("name correction lost its reason: %+v", got[2].delta)
	}
	if n := count(t, db, `SELECT count(*) FROM audit_log WHERE tenant_id = $1`, ulidB); n != 0 {
		t.Errorf("refused writes on commune B left %d audit entries", n)
	}
}

func TestPgOperatorRegistryReads(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themHost(t, db, "b.vigov.vn", ulidA, false)
	themHost(t, db, "a.vigov.vn", ulidA, true)
	themXa(t, db, ulidB, "Xã Tân Phú", false)
	r := NewOperatorRegistry(db)

	req, err := pageFirst("1")
	if err != nil {
		t.Fatal(err)
	}
	p1, err := r.ListCommunes(context.Background(), req)
	if err != nil || len(p1.Items) != 1 || !p1.HasMore || p1.NextCursor == "" {
		t.Fatalf("page 1 = %+v, %v", p1, err)
	}
	c, _, err := r.Commune(context.Background(), ulidA)
	if err != nil || len(c.Domains) != 2 || c.Domains[0] != "a.vigov.vn" {
		t.Fatalf("commune = %+v %v (primary must come first)", c, err)
	}
	if _, _, err := r.Commune(context.Background(), "01JD8ZQK9M3NPXR7TVWYB2C4EZ"); !errors.Is(err, ErrCommuneNotFound) {
		t.Errorf("unknown: %v", err)
	}
	ps, err := r.Provinces(context.Background())
	if err != nil || len(ps) != 34 {
		t.Errorf("provinces = %d, %v", len(ps), err)
	}
}

// A commune named as a predecessor in tenant_succession is never reactivated (rule 7 invariant 6,
// rule 1 stop #3); deactivating, and reactivating a commune with no successor, are unaffected.
func TestPgReactivationRefusedForSucceededCommune(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Cũ", false)
	themXa(t, db, ulidB, "Xã Mới", true)
	if _, err := db.Exec(`INSERT INTO tenant_succession (tu_id, den_id, can_cu, hieu_luc_tu)
		VALUES ($1, $2, 'NQ-FAKE/2026', '2026-07-01')`, ulidA, ulidB); err != nil {
		t.Fatal(err)
	}
	w := NewRegistryWriter(corestore.New(db))

	if _, _, err := w.SetActivation(inCommune(ulidA), true, "Mở lại", operatorFake); !errors.Is(err, ErrCommuneSucceeded) {
		t.Fatalf("reactivating a merged commune: %v, want ErrCommuneSucceeded", err)
	}
	if count(t, db, `SELECT count(*) FROM tenant WHERE id = $1 AND dang_hoat_dong`, ulidA) != 0 ||
		count(t, db, `SELECT count(*) FROM audit_log WHERE tenant_id = $1`, ulidA) != 0 {
		t.Fatal("a refused reactivation changed the row or left an audit entry")
	}
	// The successor itself may be switched off and on again.
	if _, _, err := w.SetActivation(inCommune(ulidB), false, "Tạm dừng", operatorFake); err != nil {
		t.Fatalf("deactivate successor: %v", err)
	}
	if _, _, err := w.SetActivation(inCommune(ulidB), true, "Mở lại", operatorFake); err != nil {
		t.Fatalf("reactivate successor: %v", err)
	}
}

// A primary-domain switch that would touch a frozen admin*.vigov.vn row hits 0007's CHECK: that is
// reserved_domain, not a 500 — and the transaction leaves nothing behind.
func TestPgPrimarySwitchOffFrozenRowIsReserved(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	// 0007 is NOT VALID: the frozen row predates it. Re-create that state the way it arose.
	if _, err := db.Exec(`ALTER TABLE tenant_domain DROP CONSTRAINT tenant_domain_khong_danh_rieng`); err != nil {
		t.Fatal(err)
	}
	themHost(t, db, "admin.vigov.vn", ulidA, true)
	themHost(t, db, "thangbinh.vigov.vn", ulidA, false)
	if _, err := db.Exec(`ALTER TABLE tenant_domain ADD CONSTRAINT tenant_domain_khong_danh_rieng CHECK (
		host !~* '^((admin|admin-stg|api|api-stg|stg|www)\.)?(stg\.)?vigov\.vn\.*$'
		AND host !~* '\.(api|api-stg)\.vigov\.vn\.*$') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	w := NewRegistryWriter(corestore.New(db))
	if _, err := w.SetPrimaryDomain(inCommune(ulidA), "thangbinh.vigov.vn", operatorFake); !errors.Is(err, domain.ErrCommuneHostReserved) {
		t.Fatalf("err = %v, want ErrCommuneHostReserved", err)
	}
	if count(t, db, `SELECT count(*) FROM audit_log WHERE tenant_id = $1`, ulidA) != 0 {
		t.Fatal("a refused switch left an audit entry")
	}
}

// HostHeld sees active AND inactive communes' hosts, whatever their spelling.
func TestPgHostHeld(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Cũ", false)
	themHost(t, db, "console.example.org", ulidA, true)
	d := NewDirectory(db, "")
	for host, want := range map[string]bool{"console.example.org": true, "CONSOLE.example.org": true, "other.example.org": false} {
		if got, err := d.HostHeld(context.Background(), host); err != nil || got != want {
			t.Errorf("HostHeld(%q) = %v, %v; want %v", host, got, err, want)
		}
	}
}
