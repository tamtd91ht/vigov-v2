package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-platform/internal/domain"
)

// ADR 0073 wave 2, part 2, against a real PostgreSQL — skipped without VIGOV_TEST_DSN, like every
// *_pg_test.go here. What only the database can show: the shared app's new row, the old row switched
// off and the platform_audit_log entry commit together; the resolver (Directory.MiniApp) follows at
// once; a tier-1 write and its entry commit together, and 0011's trigger still refuses a rename.

func TestPgDeclareSharedMiniApp(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	s := NewSharedMiniAppStore(db)
	dir := NewDirectory(db, "")
	ctx := context.Background()
	trail := `SELECT count(*) FROM platform_audit_log WHERE action = 'shared_mini_app.changed'`

	if _, err := s.SharedMiniApp(ctx); !errors.Is(err, ErrNoSharedMiniApp) {
		t.Fatalf("fresh registry: %v, want ErrNoSharedMiniApp (migration 0006 seeds no row)", err)
	}

	// First declaration: no before.
	a, changed, err := s.DeclareSharedMiniApp(ctx, "1001", "Khai báo app dùng chung", operatorFake)
	if err != nil || !changed || a.AppID != "1001" || a.CreatedBy != "VH-00001" {
		t.Fatalf("declare: %+v changed=%v err=%v", a, changed, err)
	}
	if got, err := dir.MiniApp(ctx, "1001"); err != nil || got.CheDo != domain.CheDoChinh || got.Xa != nil {
		t.Fatalf("resolver after declaration: %+v %v", got, err)
	}
	var before, after, reason, ip string
	if err := db.QueryRow(`SELECT COALESCE(before::text, ''), after::text, reason, actor_ip FROM platform_audit_log
		WHERE action = 'shared_mini_app.changed' ORDER BY id DESC LIMIT 1`).Scan(&before, &after, &reason, &ip); err != nil {
		t.Fatal(err)
	}
	if before != "" || !strings.Contains(after, `"app_id": "1001"`) || reason != "Khai báo app dùng chung" || ip != operatorFake.IP {
		t.Errorf("first entry before=%q after=%q reason=%q ip=%q", before, after, reason, ip)
	}

	// The same App ID again: nothing written.
	if _, changed, err := s.DeclareSharedMiniApp(ctx, "1001", "Lặp", operatorFake); err != nil || changed {
		t.Fatalf("repeat: changed=%v err=%v", changed, err)
	}
	if count(t, db, trail) != 1 {
		t.Error("a no-op wrote an entry")
	}

	// Replacement: new on, old off (never deleted), one entry naming both.
	if _, changed, err := s.DeclareSharedMiniApp(ctx, "1002", "Đổi App ID", operatorFake); err != nil || !changed {
		t.Fatalf("replace: changed=%v err=%v", changed, err)
	}
	if _, err := dir.MiniApp(ctx, "1001"); !errors.Is(err, ErrKhongCoMiniApp) {
		t.Errorf("old shared app still resolves: %v", err)
	}
	if count(t, db, `SELECT count(*) FROM mini_app WHERE app_id = '1001' AND NOT dang_hoat_dong AND deleted_at IS NULL`) != 1 {
		t.Error("the old row must be switched off, not deleted")
	}
	if got, err := s.SharedMiniApp(ctx); err != nil || got.AppID != "1002" {
		t.Errorf("current: %+v %v", got, err)
	}
	if err := db.QueryRow(`SELECT before::text FROM platform_audit_log WHERE action = 'shared_mini_app.changed'
		ORDER BY id DESC LIMIT 1`).Scan(&before); err != nil || !strings.Contains(before, `"1001"`) {
		t.Errorf("replacement entry before=%q err=%v", before, err)
	}

	// Refusals write nothing: an earlier shared app, a commune's app, no actor.
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themMiniApp(t, db, "2001", "rieng", ulidA, true)
	n := count(t, db, trail)
	for _, id := range []string{"1001", "2001"} {
		if _, _, err := s.DeclareSharedMiniApp(ctx, id, "x", operatorFake); !errors.Is(err, ErrMiniAppTaken) {
			t.Errorf("App ID %s with a row: %v, want ErrMiniAppTaken", id, err)
		}
	}
	if _, _, err := s.DeclareSharedMiniApp(ctx, "1003", "x", domain.OperatorActor{}); !errors.Is(err, domain.ErrNoActor) {
		t.Errorf("no actor: %v", err)
	}
	if count(t, db, trail) != n {
		t.Error("a refusal wrote an entry")
	}

	// Two running shared rows (hand-written): refused, never guessed.
	themMiniApp(t, db, "1009", "chinh", nil, true)
	if _, err := s.SharedMiniApp(ctx); !errors.Is(err, ErrSharedMiniAppAmbiguous) {
		t.Errorf("two running: %v", err)
	}
	if _, _, err := s.DeclareSharedMiniApp(ctx, "1004", "x", operatorFake); !errors.Is(err, ErrSharedMiniAppAmbiguous) {
		t.Errorf("declare over two running: %v", err)
	}
}

func TestPgCommuneLaunchHost(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	s := NewSharedMiniAppStore(db)
	ctx := context.Background()
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themHost(t, db, "a-phu.vigov.vn", ulidA, false)
	themHost(t, db, "thangbinh.vigov.vn", ulidA, true)
	if host, active, err := s.CommuneLaunchHost(ctx, ulidA); err != nil || !active || host != "thangbinh.vigov.vn" {
		t.Errorf("got %q %v %v, want the PRIMARY host", host, active, err)
	}
	themXa(t, db, ulidB, "Xã Cũ", false)
	themHost(t, db, "xacu.vigov.vn", ulidB, false)
	if _, active, err := s.CommuneLaunchHost(ctx, ulidB); !errors.Is(err, ErrCommuneNoPrimaryHost) || active {
		t.Errorf("no primary, inactive: active=%v err=%v", active, err)
	}
	if _, _, err := s.CommuneLaunchHost(ctx, "01JD8ZQK9M3NPXR7TVWYB2C4EZ"); !errors.Is(err, ErrCommuneNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

func TestPgPetitionFieldWrites(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	s := NewPetitionFieldStore(db)
	ctx := context.Background()
	trail := `SELECT count(*) FROM platform_audit_log WHERE subject = $1 AND action = $2`

	f := domain.PetitionField{Code: "cay-xanh", DefaultLabel: "Cây xanh", SortOrder: 13, Icon: "Trees", Tone: "green"}
	out, err := s.CreatePetitionField(ctx, f, "Xã đề nghị", operatorFake)
	if err != nil || !out.Active || out.Code != "cay-xanh" {
		t.Fatalf("create: %+v %v", out, err)
	}
	if count(t, db, trail, "cay-xanh", ActionPetitionFieldCreated) != 1 {
		t.Error("no entry for the create")
	}
	var by string
	if err := db.QueryRow(`SELECT created_by FROM petition_field WHERE code = 'cay-xanh'`).Scan(&by); err != nil || by != "VH-00001" {
		t.Errorf("created_by %q %v", by, err)
	}
	// An issued code — seeded or created, active or retired — is never issued again.
	for _, code := range []string{"cay-xanh", "khac"} {
		f.Code = code
		if _, err := s.CreatePetitionField(ctx, f, "x", operatorFake); !errors.Is(err, ErrPetitionFieldCodeTaken) {
			t.Errorf("re-create %s: %v", code, err)
		}
	}

	next := domain.PetitionField{DefaultLabel: "Điện – chiếu sáng", SortOrder: 4, Icon: "Zap", Tone: "orange"}
	got, changed, err := s.EditPetitionField(ctx, "dien", next, "Sửa nhãn", operatorFake)
	if err != nil || !changed || got.DefaultLabel != "Điện – chiếu sáng" || got.Code != "dien" || !got.Active {
		t.Fatalf("edit: %+v changed=%v %v", got, changed, err)
	}
	if _, changed, err := s.EditPetitionField(ctx, "dien", next, "Lặp", operatorFake); err != nil || changed {
		t.Errorf("repeat edit: changed=%v %v", changed, err)
	}
	if count(t, db, trail, "dien", ActionPetitionFieldChanged) != 1 {
		t.Error("edit entries: want exactly one")
	}
	if _, _, err := s.EditPetitionField(ctx, "khong-co", next, "x", operatorFake); !errors.Is(err, ErrPetitionFieldNotFound) {
		t.Errorf("unknown code: %v", err)
	}

	got, changed, err = s.SetPetitionFieldActive(ctx, "dien", false, "Gộp", operatorFake)
	if err != nil || !changed || got.Active {
		t.Fatalf("retire: %+v changed=%v %v", got, changed, err)
	}
	// Retired, still listed — old petitions keep their label (ADR 0060 §4).
	all, err := s.ListPetitionFields(ctx)
	if err != nil {
		t.Fatal(err)
	}
	listed := false
	for _, r := range all {
		listed = listed || r.Code == "dien"
	}
	if !listed {
		t.Error("a retired code vanished from the read path")
	}
	if _, changed, _ := s.SetPetitionFieldActive(ctx, "dien", false, "Lặp", operatorFake); changed {
		t.Error("retiring a retired code wrote again")
	}
	if count(t, db, trail, "dien", ActionPetitionFieldDeactivated) != 1 {
		t.Error("retire entries: want exactly one")
	}

	// The second copy of "a code never changes": 0011's trigger.
	if _, err := db.Exec(`UPDATE petition_field SET code = 'dien-moi' WHERE code = 'dien'`); err == nil {
		t.Error("the database accepted a code rename")
	}
}
