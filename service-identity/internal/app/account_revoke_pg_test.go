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
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// REVOKE AGAINST A REAL POSTGRESQL (`admin.user.revoke`, user decision 2026-10-03). Skipped unless
// VIGOV_TEST_DSN is set (TestMain in danh_ba_can_bo_pg_test.go).
//
// What only a real server decides: 0009 §3's CHECK accepts `co_tai_khoan = false` with an empty
// hash; the partitioned UPDATE matches; the session rows are really marked revoked; the trail is
// read back from `audit_log`; and the two follow-on acts the decision names — re-issue through
// POST .../account and the soft delete of the now account-less row — really work on the row this
// statement leaves.

// openSessionPg opens one live session for a staff member, the state a revocation must end.
func openSessionPg(t *testing.T, db *sql.DB, ctx context.Context, staffID string) {
	t.Helper()
	kho := pkgstore.New(db)
	err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		_, _, err := idstore.NewPhienStore(kho).Tao(ctx, tx, staffID, "10.0.0.7", "test-agent")
		return err
	})
	if err != nil {
		t.Fatalf("mở phiên: %v", err)
	}
}

func liveSessionsPg(t *testing.T, db *sql.DB, xa, staffID string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(
		`SELECT count(*) FROM phien WHERE tenant_id = $1 AND nguoi_dung_id = $2 AND thu_hoi_luc IS NULL`,
		xa, staffID).Scan(&n); err != nil {
		t.Fatalf("đếm phiên: %v", err)
	}
	return n
}

// THE DECISION, END TO END: credential gone, sessions ended, row kept as directory-only with its
// role, one audit entry naming the staff code, and the account can be issued again.
func TestPgRevokeClearsCredentialEndsSessionsAuditsAndAllowsReissue(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	const id, ma = "nd-thu-hoi-01", "CB-2026-THUHO1"

	// Two administrators, neither the target: #13 permits the act.
	dungXaPg(t, db, xa, "vt-quan-tri", []string{"admin.user"}, "nd-quan-tri", "nd-quan-tri-2")
	dungXaPg(t, db, xa, "vt-thuong", []string{"document.read"})
	themDongDanhBaPg(t, db, xa, id, ma)
	if _, err := db.Exec(`UPDATE nguoi_dung SET vai_tro_id = 'vt-thuong' WHERE tenant_id = $1 AND id = $2`, xa, id); err != nil {
		t.Fatalf("gán vai trò: %v", err)
	}

	uc := ucTaiKhoanThat(db)
	issued, err := uc.Cap(ctx, id, nguoiPg("nd-quan-tri"))
	if err != nil {
		t.Fatalf("Cap lỗi: %v", err)
	}
	openSessionPg(t, db, ctx, id)
	openSessionPg(t, db, ctx, id)
	hashBefore, _, _ := dongCanBoPg(t, db, xa, id)

	actor := nguoiPg("nd-quan-tri")
	result, err := uc.Revoke(ctx, id, "Dòng nhập trùng", actor)
	if err != nil {
		t.Fatalf("Revoke lỗi: %v", err)
	}
	if result.CoTaiKhoan {
		t.Error("bản ghi trả về vẫn có tài khoản")
	}

	hash, hasAccount, mustChange := dongCanBoPg(t, db, xa, id)
	if hasAccount || hash != "" || !mustChange {
		t.Errorf("dòng sau thu hồi: co_tai_khoan=%v hash_rong=%v phai_doi=%v, muốn false/true/true",
			hasAccount, hash == "", mustChange)
	}
	var role string
	var deleted bool
	if err := db.QueryRow(`SELECT coalesce(vai_tro_id,''), deleted_at IS NOT NULL FROM nguoi_dung WHERE tenant_id=$1 AND id=$2`,
		xa, id).Scan(&role, &deleted); err != nil {
		t.Fatal(err)
	}
	if role != "vt-thuong" || deleted {
		t.Errorf("thu hồi đã chạm vai trò hoặc xoá dòng: vai_tro_id=%q deleted=%v", role, deleted)
	}
	if n := liveSessionsPg(t, db, xa, id); n != 0 {
		t.Errorf("còn %d phiên sống sau khi thu hồi tài khoản", n)
	}
	// The old temporary password no longer opens anything — there is no hash to check it against.
	if err := password.KiemTra(issued.MatKhauTam, hash); err == nil {
		t.Error("mật khẩu cũ vẫn khớp sau khi thu hồi")
	}

	entries := vetCuaXaPg(t, db, xa)
	var revokeEntry string
	for _, e := range entries {
		if strings.Contains(e, ActionAccountRevoked) {
			if revokeEntry != "" {
				t.Fatal("hơn một vết thu hồi")
			}
			revokeEntry = e
		}
	}
	if !strings.HasPrefix(revokeEntry, actor.Vet.ID+"|"+ActionAccountRevoked+"|"+ma+"|") {
		t.Errorf("vết thu hồi sai actor/hành vi/chủ thể: %s", revokeEntry)
	}
	if strings.Contains(revokeEntry, hashBefore) || strings.Contains(revokeEntry, issued.MatKhauTam) {
		t.Error("vết thu hồi chứa chuỗi băm hoặc mật khẩu")
	}
	if !strings.Contains(revokeEntry, "Dòng nhập trùng") {
		t.Errorf("vết thu hồi không mang lý do: %s", revokeEntry)
	}

	// RE-ISSUE through the existing route: works, with the same role still attached.
	again, err := uc.Cap(ctx, id, nguoiPg("nd-quan-tri"))
	if err != nil {
		t.Fatalf("cấp lại sau thu hồi lỗi: %v", err)
	}
	hash, hasAccount, mustChange = dongCanBoPg(t, db, xa, id)
	if !hasAccount || !mustChange {
		t.Errorf("sau cấp lại: co_tai_khoan=%v phai_doi=%v", hasAccount, mustChange)
	}
	if err := password.KiemTra(again.MatKhauTam, hash); err != nil {
		t.Errorf("mật khẩu cấp lại không khớp: %v", err)
	}
}

// THE SOFT DELETE OF #10 REFUSES A ROW WITH AN ACCOUNT, AND ACCEPTS IT ONCE REVOKED — the path the
// user decision names for a genuine duplicate.
func TestPgRevokeThenSoftDeleteAccepted(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	const id, ma = "nd-thu-hoi-02", "CB-2026-THUHO2"

	dungXaPg(t, db, xa, "vt-quan-tri", []string{"admin.user"}, "nd-quan-tri", "nd-quan-tri-2")
	themDongDanhBaPg(t, db, xa, id, ma)
	accounts := ucTaiKhoanThat(db)
	if _, err := accounts.Cap(ctx, id, nguoiPg("nd-quan-tri")); err != nil {
		t.Fatalf("Cap lỗi: %v", err)
	}

	register := ucThat(db)
	if err := register.Xoa(ctx, id, "Nhập trùng", nguoiPg("nd-quan-tri")); !errors.Is(err, ErrCanBoCoTaiKhoan) {
		t.Fatalf("xoá khi còn tài khoản: lỗi = %v, muốn ErrCanBoCoTaiKhoan", err)
	}
	if _, err := accounts.Revoke(ctx, id, "Nhập trùng", nguoiPg("nd-quan-tri")); err != nil {
		t.Fatalf("Revoke lỗi: %v", err)
	}
	if err := register.Xoa(ctx, id, "Nhập trùng", nguoiPg("nd-quan-tri")); err != nil {
		t.Fatalf("xoá sau thu hồi lỗi: %v", err)
	}
	var deleted bool
	if err := db.QueryRow(`SELECT deleted_at IS NOT NULL FROM nguoi_dung WHERE tenant_id=$1 AND id=$2`, xa, id).
		Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Error("dòng đã thu hồi tài khoản mà xoá mềm không ghi deleted_at")
	}
}

// #13 — the commune's only administrator cannot be revoked; nothing changes, nothing is audited.
func TestPgRevokeLastAdminRefused(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	const only = "nd-qt-duy-nhat"

	dungXaPg(t, db, xa, "vt-quan-tri", []string{"admin.user"}, only)

	if _, err := ucTaiKhoanThat(db).Revoke(ctx, only, "Thử", nguoiPg("nd-nguoi-khac")); !errors.Is(err, ErrQuanTriCuoiCung) {
		t.Fatalf("lỗi = %v, muốn ErrQuanTriCuoiCung", err)
	}
	if n := demQuanTriConSong(t, db, xa); n != 1 {
		t.Errorf("còn %d quản trị viên, muốn 1", n)
	}
	if n := len(vetCuaXaPg(t, db, xa)); n != 0 {
		t.Errorf("bị từ chối vẫn để lại %d vết", n)
	}
}

// A second revoke is 409-shaped (ErrChuaCoTaiKhoan) and files no second entry; another commune's
// id is "not found".
func TestPgRevokeTwiceAndOtherCommune(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	const id, ma = "nd-thu-hoi-03", "CB-2026-THUHO3"

	dungXaPg(t, db, xa, "vt-quan-tri", []string{"admin.user"}, "nd-quan-tri", "nd-quan-tri-2")
	themDongDanhBaPg(t, db, xa, id, ma)
	uc := ucTaiKhoanThat(db)
	if _, err := uc.Cap(ctx, id, nguoiPg("nd-quan-tri")); err != nil {
		t.Fatalf("Cap lỗi: %v", err)
	}
	if _, err := uc.Revoke(ctx, id, "Lần một", nguoiPg("nd-quan-tri")); err != nil {
		t.Fatalf("Revoke lần một lỗi: %v", err)
	}
	if _, err := uc.Revoke(ctx, id, "Lần hai", nguoiPg("nd-quan-tri")); !errors.Is(err, ErrChuaCoTaiKhoan) {
		t.Fatalf("lần hai: lỗi = %v, muốn ErrChuaCoTaiKhoan", err)
	}
	revokes := 0
	for _, e := range vetCuaXaPg(t, db, xa) {
		if strings.Contains(e, ActionAccountRevoked) {
			revokes++
		}
	}
	if revokes != 1 {
		t.Errorf("có %d vết thu hồi, muốn 1", revokes)
	}

	// Another commune's context cannot see the row at all.
	other := xaRiengPg(t)
	otherCtx := tenant.Into(context.Background(), tenant.ID(other))
	if _, err := uc.Revoke(otherCtx, id, "Sai xã", nguoiPg("nd-quan-tri")); !errors.Is(err, idstore.ErrCanBoKhongTonTai) {
		t.Fatalf("xã khác: lỗi = %v, muốn ErrCanBoKhongTonTai", err)
	}
}
