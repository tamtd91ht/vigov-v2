package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/password"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The owner's three commune-scoped acts against a real PostgreSQL: the partitioned `nguoi_dung`
// unique keys, the `quyen` catalogue grant, the session registry and the append-only audit_log only
// exist there. Skipped unless VIGOV_TEST_DSN is set (TestMain in danh_ba_can_bo_pg_test.go).

const (
	ownerPgCode = "VH-00001"
	ownerPgID   = "01JOWNERPG0000000000000000"
	ownerPgTemp = "OWNER-PG-TEMP-MARKER-not-real"
)

var ownerPgActor = MiniAppOperator{Code: ownerPgCode, IP: "203.0.113.11"}

// communeLookupFake answers the platform's GetTenant for the commune in ctx.
type communeLookupFake struct{ found, active bool }

func (f communeLookupFake) Current(ctx context.Context) (tenant.Tenant, bool, error) {
	id, _ := tenant.From(ctx)
	return tenant.Tenant{ID: id, Active: f.active}, f.found, nil
}

func ownerCommunePg(db *sql.DB, lookup tenant.CommuneLookup) *CommuneAccountsByOwner {
	kho := pkgstore.New(db)
	return NewCommuneAccountsByOwner(kho, idstore.NewCanBoStore(kho), ucTaiKhoanThat(db), lookup, idstore.NewRoleTemplateStore(kho))
}

// operatorTrailPg returns "actor_kind|actor_id|action|subject|delta" for every row of one action.
func operatorTrailPg(t *testing.T, db *sql.DB, xa, action string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT actor_kind || '|' || actor_id || '|' || action || '|' || subject || '|' ||
		coalesce(delta::text, '') || '|' || coalesce(actor_ip, '')
		FROM audit_log WHERE tenant_id = $1 AND action = $2 ORDER BY at`, xa, action)
	if err != nil {
		t.Fatalf("read audit_log: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func countPg(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestCommuneAdminRoleCodeMatchesStore(t *testing.T) {
	if domain.CommuneAdminRoleCode != idstore.MaVaiTroQuanTriMacDinh {
		t.Fatalf("domain %q != store %q", domain.CommuneAdminRoleCode, idstore.MaVaiTroQuanTriMacDinh)
	}
}

func TestPgOwnerCreatesCommuneAdminWithRoleGrantsAndTrail(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	uc := ownerCommunePg(db, communeLookupFake{found: true, active: true})

	a, err := uc.CreateAdmin(ctx, ownerPgActor, " Quan.Tri@Xa.Example.Test ", "  Quản trị   xã ", ownerPgTemp)
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}
	if !a.IsAdmin() || !a.MustChangePassword || a.Disabled || a.Email != "quan.tri@xa.example.test" ||
		!strings.HasPrefix(a.Code, "CB-") {
		t.Fatalf("account = %+v", a)
	}
	var hash string
	if err := db.QueryRow(`SELECT mat_khau_hash FROM nguoi_dung WHERE tenant_id = $1 AND id = $2`, xa, a.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if password.KiemTra(ownerPgTemp, hash) != nil {
		t.Fatal("the stored hash does not verify the owner's temporary password")
	}
	catalogue := countPg(t, db, `SELECT count(*) FROM quyen`)
	grants := countPg(t, db, `SELECT count(*) FROM vai_tro_quyen vq JOIN vai_tro vt
		ON vt.tenant_id = vq.tenant_id AND vt.id = vq.vai_tro_id WHERE vq.tenant_id = $1 AND vt.ma = 'quan-tri-he-thong'`, xa)
	if catalogue == 0 || grants != catalogue {
		t.Fatalf("a created role must hold every key: %d of %d", grants, catalogue)
	}
	trail := operatorTrailPg(t, db, xa, ActionCommuneAdminCreatedByOperator)
	if len(trail) != 1 || !strings.HasPrefix(trail[0], "operator|"+ownerPgCode+"|") || !strings.Contains(trail[0], a.Code) ||
		!strings.HasSuffix(trail[0], ownerPgActor.IP) {
		t.Fatalf("trail = %v", trail)
	}
	for _, leak := range []string{ownerPgTemp, hash, "quan.tri@", "Quản trị xã"} {
		if strings.Contains(trail[0], leak) {
			t.Fatalf("the trail carries %q: %s", leak, trail[0])
		}
	}

	// Owner decision 15/10/2026: a brand-new commune receives the eight template roles in the same
	// transaction, each with its own entry under the owner's VH- code, granted by that code.
	if n := countPg(t, db, `SELECT count(*) FROM vai_tro WHERE tenant_id = $1 AND deleted_at IS NULL`, xa); n != 1+len(domain.RoleTemplates()) {
		t.Fatalf("live roles = %d, want the administrator role + %d templates", n, len(domain.RoleTemplates()))
	}
	seeds := operatorTrailPg(t, db, xa, ActionSeedRoleTemplate)
	if len(seeds) != len(domain.RoleTemplates()) || !strings.HasPrefix(seeds[0], "operator|"+ownerPgCode+"|") {
		t.Fatalf("template entries = %v", seeds)
	}
	if n := countPg(t, db, `SELECT count(*) FROM vai_tro_quyen vq JOIN vai_tro vt ON vt.tenant_id = vq.tenant_id
		AND vt.id = vq.vai_tro_id WHERE vq.tenant_id = $1 AND vt.ma = 'ke-toan' AND vq.cap_boi = $2`, xa, ownerPgCode); n == 0 {
		t.Fatal("the Kế toán template holds no key granted by the owner's code")
	}
	if !strings.Contains(trail[0], `"so_vai_tro_mau_tao": 8`) {
		t.Fatalf("the act's entry does not count the templates: %s", trail[0])
	}

	// The same email again (another case): ALREADY_EXISTS, nothing written.
	if _, err := uc.CreateAdmin(ctx, ownerPgActor, "QUAN.TRI@xa.example.test", "Khác", ownerPgTemp); !errors.Is(err, ErrCommuneAdminEmailTaken) {
		t.Fatalf("duplicate email: %v", err)
	}
	if n := countPg(t, db, `SELECT count(*) FROM nguoi_dung WHERE tenant_id = $1`, xa); n != 1 {
		t.Fatalf("rows = %d after a refused duplicate", n)
	}
	if n := len(operatorTrailPg(t, db, xa, ActionCommuneAdminCreatedByOperator)); n != 1 {
		t.Fatalf("entries = %d after a refused duplicate", n)
	}
}

func TestPgOwnerCreateAdminRefusesInactiveCommuneAndKeepsExistingRole(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))

	for _, lookup := range []communeLookupFake{{found: false}, {found: true, active: false}} {
		if _, err := ownerCommunePg(db, lookup).CreateAdmin(ctx, ownerPgActor, "a@xa.example.test", "A", ownerPgTemp); !errors.Is(err, ErrCommuneNotLive) {
			t.Fatalf("%+v: %v", lookup, err)
		}
	}
	if n := countPg(t, db, `SELECT count(*) FROM nguoi_dung WHERE tenant_id = $1`, xa); n != 0 {
		t.Fatalf("an account was written for a commune that is not live: %d", n)
	}

	// An EXISTING administrator role with no keys: used as the commune left it, never re-filled.
	if _, err := db.Exec(`INSERT INTO vai_tro (tenant_id, id, ten, ma) VALUES ($1, 'vt-existing', 'Quản trị hệ thống', 'quan-tri-he-thong')`, xa); err != nil {
		t.Fatal(err)
	}
	a, err := ownerCommunePg(db, communeLookupFake{found: true, active: true}).CreateAdmin(ctx, ownerPgActor,
		"b@xa.example.test", "B", ownerPgTemp)
	if err != nil {
		t.Fatal(err)
	}
	if !a.IsAdmin() {
		t.Fatalf("account not on the existing role: %+v", a)
	}
	if n := countPg(t, db, `SELECT count(*) FROM vai_tro_quyen WHERE tenant_id = $1`, xa); n != 0 {
		t.Fatalf("the existing role was re-filled with %d keys", n)
	}
	// An existing administrator role means a commune set up before: no template is seeded into it.
	if n := countPg(t, db, `SELECT count(*) FROM vai_tro WHERE tenant_id = $1`, xa); n != 1 {
		t.Fatalf("roles = %d — templates were seeded into a commune that already had an administrator role", n)
	}
	if trail := operatorTrailPg(t, db, xa, ActionCommuneAdminCreatedByOperator); len(trail) != 1 ||
		!strings.Contains(trail[0], `"vai_tro_tao_moi": false`) {
		t.Fatalf("trail = %v", trail)
	}
}

func TestPgOwnerListCommuneStaffAccountsIsScopedAndAudited(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	xaB := xa[:25] + "Z"
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	uc := ownerCommunePg(db, communeLookupFake{found: true, active: true})

	if _, err := uc.CreateAdmin(ctx, ownerPgActor, "list@xa.example.test", "Liệt kê", ownerPgTemp); err != nil {
		t.Fatal(err)
	}
	themDongDanhBaPg(t, db, xa, "nd-dir-only-1", "CB-2026-DIRON1") // directory only: not an account
	if _, err := uc.CreateAdmin(tenant.Into(context.Background(), tenant.ID(xaB)), ownerPgActor,
		"other@xa.example.test", "Xã khác", ownerPgTemp); err != nil {
		t.Fatal(err)
	}

	rows, err := uc.List(ctx, ownerPgActor)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Email != "list@xa.example.test" {
		t.Fatalf("rows = %+v — only this commune's sign-in accounts", rows)
	}
	trail := operatorTrailPg(t, db, xa, ActionStaffAccountsReadByOperator)
	if len(trail) != 1 || !strings.HasPrefix(trail[0], "operator|"+ownerPgCode+"|") || strings.Contains(trail[0], "list@") {
		t.Fatalf("read trail = %v", trail)
	}
}

func TestPgOwnerResetStaffPassword(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	const id, ma = "nd-owner-reset-1", "CB-2026-ORESET"
	themDongDanhBaPg(t, db, xa, id, ma)
	if _, err := ucTaiKhoanThat(db).Cap(ctx, id, nguoiPg("nd-quan-tri-cap")); err != nil {
		t.Fatal(err)
	}
	kho := pkgstore.New(db)
	if err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		_, _, err := idstore.NewPhienStore(kho).Tao(ctx, tx, id, "10.0.0.8", "test-agent")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	uc := ownerCommunePg(db, communeLookupFake{found: true, active: true})

	a, err := uc.ResetPassword(ctx, ownerPgActor, ownerPgID, id, ownerPgTemp, "quên mật khẩu")
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	hash, _, mustChange := dongCanBoPg(t, db, xa, id)
	if !mustChange || !a.MustChangePassword || password.KiemTra(ownerPgTemp, hash) != nil {
		t.Fatal("not a temporary password the owner typed")
	}
	if n := countPg(t, db, `SELECT count(*) FROM phien WHERE tenant_id = $1 AND nguoi_dung_id = $2 AND thu_hoi_luc IS NULL`, xa, id); n != 0 {
		t.Fatalf("%d sessions still open after the reset", n)
	}
	trail := operatorTrailPg(t, db, xa, HanhViDatLaiMatKhau)
	if len(trail) != 1 || !strings.HasPrefix(trail[0], "operator|"+ownerPgCode+"|") || !strings.Contains(trail[0], "quên mật khẩu") ||
		strings.Contains(trail[0], ownerPgTemp) || strings.Contains(trail[0], ownerPgID) {
		t.Fatalf("trail = %v", trail)
	}

	// Refusals, each with nothing written.
	themDongDanhBaPg(t, db, xa, "nd-owner-dir-1", "CB-2026-ODIR01")
	if _, err := uc.ResetPassword(ctx, ownerPgActor, ownerPgID, "nd-owner-dir-1", ownerPgTemp, "x"); !errors.Is(err, ErrChuaCoTaiKhoan) {
		t.Fatalf("directory-only: %v", err)
	}
	other := tenant.Into(context.Background(), tenant.ID(xa[:25]+"Y"))
	if _, err := uc.ResetPassword(other, ownerPgActor, ownerPgID, id, ownerPgTemp, "x"); !errors.Is(err, idstore.ErrCanBoKhongTonTai) {
		t.Fatalf("another commune's id: %v", err)
	}
	if _, err := db.Exec(`UPDATE nguoi_dung SET dang_hoat_dong = false WHERE tenant_id = $1 AND id = $2`, xa, id); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.ResetPassword(ctx, ownerPgActor, ownerPgID, id, ownerPgTemp+"-2", "x"); !errors.Is(err, ErrTaiKhoanDangKhoa) {
		t.Fatalf("locked: %v", err)
	}
	if n := len(operatorTrailPg(t, db, xa, HanhViDatLaiMatKhau)); n != 1 {
		t.Fatalf("a refused reset wrote an entry: %d", n)
	}
}
