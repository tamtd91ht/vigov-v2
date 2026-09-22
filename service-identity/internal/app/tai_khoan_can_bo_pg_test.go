package app

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/password"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// THE CASES ONLY A REAL POSTGRESQL CAN DECIDE, for the credential flow of open questions #9 and
// #17. The fake-driver suite next door proves the DECISIONS; these prove things a fake cannot have
// an opinion about:
//
//	THE TRAIL IS READ BACK FROM THE TABLE, not from a recorded statement. "The temporary password
//	is not in the audit trail" is a claim about what is IN `audit_log` after the commit, and the
//	only way to check it is to SELECT it. A fake agrees with whatever the code passed it.
//	THE CHECK CONSTRAINT of migration 0009 §3 — `NOT co_tai_khoan OR mat_khau_hash <> ''` — is
//	enforced by the server and by nothing in Go.
//	THE PARTITIONED UPDATE really matches (`nguoi_dung` is PARTITION BY HASH, 32 ways), and the
//	`AND NOT co_tai_khoan` / `AND co_tai_khoan` predicates really refuse.
//	THE ARGON2 HASH really verifies the value the administrator was handed, which is what "the
//	person can sign in with what was read out to them" actually means.
//
// These use the REAL password package, not the fast stand-in: a temporary password that does not
// verify against its own stored hash is the defect that would reach a commune, and it is exactly
// what a pinned hash function hides.
//
// Skipped unless VIGOV_TEST_DSN is set. TestMain in danh_ba_can_bo_pg_test.go builds the schema.

// ucTaiKhoanThat builds the credential use case over a real pool — no functions pinned.
func ucTaiKhoanThat(db *sql.DB) *TaiKhoanCanBo {
	kho := pkgstore.New(db)
	return NewTaiKhoanCanBo(kho, idstore.NewCanBoStore(kho), idstore.NewPhienStore(kho))
}

// themDongDanhBaPg seeds a DIRECTORY-ONLY row: `co_tai_khoan = false`, empty hash. That is exactly
// what POST /api/v1/staff writes, and it is the state the account route starts from.
func themDongDanhBaPg(t *testing.T, db *sql.DB, xa, id, ma string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO nguoi_dung (tenant_id, id, ma, ho_ten, email, co_tai_khoan, mat_khau_hash)
		 VALUES ($1,$2,$3,$4,$5,false,'')`,
		xa, id, ma, "Trần Thị B", id+"@xa.danang.gov.vn")
	if err != nil {
		t.Fatalf("thêm dòng danh bạ: %v", err)
	}
}

// dongCanBoPg reads back the three columns this flow writes.
func dongCanBoPg(t *testing.T, db *sql.DB, xa, id string) (bam string, coTaiKhoan, phaiDoi bool) {
	t.Helper()
	err := db.QueryRow(
		`SELECT mat_khau_hash, co_tai_khoan, phai_doi_mat_khau
		   FROM nguoi_dung WHERE tenant_id = $1 AND id = $2`, xa, id).
		Scan(&bam, &coTaiKhoan, &phaiDoi)
	if err != nil {
		t.Fatalf("đọc lại dòng cán bộ: %v", err)
	}
	return
}

// vetCuaXaPg returns every audit row of one commune, flattened into one string per row.
//
// IT READS THE TABLE, NOT A LOG AND NOT A RECORDED STATEMENT. That is the whole point: the claim
// being checked is about what a government record CONTAINS after the commit, kept for years and
// never deletable (rule 6, invariant 4).
func vetCuaXaPg(t *testing.T, db *sql.DB, xa string) []string {
	t.Helper()
	rows, err := db.Query(
		`SELECT coalesce(actor_id,'') || '|' || coalesce(action,'') || '|' ||
		        coalesce(subject,'') || '|' || coalesce(delta::text,'')
		   FROM audit_log WHERE tenant_id = $1 ORDER BY at`, xa)
	if err != nil {
		t.Fatalf("đọc audit_log: %v", err)
	}
	defer rows.Close()

	var ra []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("đọc dòng audit_log: %v", err)
		}
		ra = append(ra, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("duyệt audit_log: %v", err)
	}
	return ra
}

// THE CASE THE WHOLE TURN RESTS ON, PROVED AGAINST THE TABLE ITSELF.
//
// After issuing an account: the temporary password is in NO audit row — and neither is the argon2
// hash, because an append-only table holding the encoding of every credential ever issued is an
// offline-cracking target that outlives every rotation (rule 6, forbidden #4). What IS there is the
// NAME of the column that changed.
//
// MUTATIONS THAT MUST TURN THIS RED (both were run; see the ledger):
//   - put the plaintext into the audit delta;
//   - put the hash into the audit delta.
func TestPgCapTaiKhoanKhongDeMatKhauTamVaoVetKiemToan(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	const id, ma = "nd-cap-tk-0001", "CB-2026-CAPTK1"

	themDongDanhBaPg(t, db, xa, id, ma)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))

	kq, err := ucTaiKhoanThat(db).Cap(ctx, id, nguoiPg("nd-quan-tri-cap"))
	if err != nil {
		t.Fatalf("Cap lỗi: %v", err)
	}
	if kq.MatKhauTam == "" {
		t.Fatal("không có mật khẩu tạm để đọc cho người ta")
	}

	bam, coTaiKhoan, phaiDoi := dongCanBoPg(t, db, xa, id)
	if !coTaiKhoan {
		t.Fatal("co_tai_khoan vẫn false sau khi cấp tài khoản")
	}
	if !phaiDoi {
		t.Fatal("phai_doi_mat_khau = false ngay sau khi cấp — #9 đòi BẮT ĐỔI ở lần đăng nhập đầu")
	}
	if bam == kq.MatKhauTam {
		t.Fatal("CSDL đang giữ MẬT KHẨU TRẦN chứ không phải chuỗi băm")
	}
	// The value handed to the administrator really does open the account. A hash that did not
	// verify would send somebody to the counter with a password that cannot work.
	if err := password.KiemTra(kq.MatKhauTam, bam); err != nil {
		t.Fatalf("mật khẩu tạm KHÔNG khớp chuỗi băm đã lưu: %v", err)
	}

	vet := vetCuaXaPg(t, db, xa)
	if len(vet) != 1 {
		t.Fatalf("có %d vết kiểm toán, muốn đúng 1", len(vet))
	}
	if strings.Contains(vet[0], kq.MatKhauTam) {
		t.Fatal("MẬT KHẨU TẠM NẰM TRONG audit_log — bảng chỉ ghi thêm, giữ nhiều năm, " +
			"không xoá được kể cả bởi quản trị viên (luật 6 bất biến 4)")
	}
	if strings.Contains(vet[0], bam) {
		t.Fatal("CHUỖI BĂM nằm trong audit_log — vết chỉ được ghi TÊN CỘT (luật 6 cấm #4)")
	}
	if !strings.Contains(vet[0], "mat_khau_hash") {
		t.Fatalf("vết không nêu tên cột mat_khau_hash: %s", vet[0])
	}
	if !strings.Contains(vet[0], HanhViCapTaiKhoan) || !strings.Contains(vet[0], ma) {
		t.Fatalf("vết không mang hành vi %q hoặc mã cán bộ %q: %s", HanhViCapTaiKhoan, ma, vet[0])
	}
}

// #17: the administrator resets, twice, and gets a DIFFERENT value each time. The old one stops
// working the moment the new one is written — which is what a reset is for.
func TestPgDatLaiSinhMatKhauKhacLanTruocVaHuyMatKhauCu(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	const id, ma = "nd-dat-lai-0001", "CB-2026-DATLAI"

	themDongDanhBaPg(t, db, xa, id, ma)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	uc := ucTaiKhoanThat(db)

	dau, err := uc.Cap(ctx, id, nguoiPg("nd-quan-tri-cap"))
	if err != nil {
		t.Fatalf("Cap lỗi: %v", err)
	}
	sau, err := uc.DatLai(ctx, id, nguoiPg("nd-quan-tri-cap"))
	if err != nil {
		t.Fatalf("DatLai lỗi: %v", err)
	}

	if sau.MatKhauTam == dau.MatKhauTam {
		t.Fatal("đặt lại cho ĐÚNG mật khẩu cũ — mỗi lần phải sinh một giá trị khác")
	}
	bam, _, phaiDoi := dongCanBoPg(t, db, xa, id)
	if !phaiDoi {
		t.Fatal("phai_doi_mat_khau = false sau khi quản trị viên đặt lại — #9: mật khẩu do người khác chọn")
	}
	if err := password.KiemTra(sau.MatKhauTam, bam); err != nil {
		t.Fatalf("mật khẩu tạm MỚI không khớp chuỗi băm: %v", err)
	}
	if err := password.KiemTra(dau.MatKhauTam, bam); err == nil {
		t.Fatal("MẬT KHẨU CŨ VẪN DÙNG ĐƯỢC sau khi đặt lại")
	}
	// Neither value may be in the ledger.
	for _, v := range vetCuaXaPg(t, db, xa) {
		if strings.Contains(v, dau.MatKhauTam) || strings.Contains(v, sau.MatKhauTam) {
			t.Fatal("một mật khẩu tạm nằm trong audit_log")
		}
	}
}

// THE FLAG GOES BACK TO false ONLY BY THE PERSON'S OWN CHANGE, and every session of theirs is
// revoked in the same transaction (skills/session-and-token, required #7).
//
// MUTATIONS THAT MUST TURN THIS RED: drop `phai_doi_mat_khau = false` from doiMatKhauChinhMinh;
// drop the ThuHoiCuaCanBo call.
func TestPgDoiMatKhauChinhMinhGoCoVaThuHoiPhien(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	const id, ma = "nd-tu-doi-0001", "CB-2026-TUDOI1"

	themDongDanhBaPg(t, db, xa, id, ma)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	uc := ucTaiKhoanThat(db)

	kq, err := uc.Cap(ctx, id, nguoiPg("nd-quan-tri-cap"))
	if err != nil {
		t.Fatalf("Cap lỗi: %v", err)
	}

	// The person signs in and works: one open session, the state a real change happens from.
	kho := pkgstore.New(db)
	err = kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		_, _, err := idstore.NewPhienStore(kho).Tao(ctx, tx, id, "10.0.0.7", "test-agent")
		return err
	})
	if err != nil {
		t.Fatalf("mở phiên: %v", err)
	}

	const moi = "mat-khau-moi-cua-chinh-toi"
	nguoi := NguoiThucHien{ID: id, Vet: nguoiPg(id).Vet}
	nguoi.Vet.ID = ma
	if err := uc.DoiCuaChinhMinh(ctx, YeuCauDoiMatKhau{HienTai: kq.MatKhauTam, Moi: moi}, nguoi); err != nil {
		t.Fatalf("DoiCuaChinhMinh lỗi: %v", err)
	}

	bam, _, phaiDoi := dongCanBoPg(t, db, xa, id)
	if phaiDoi {
		t.Fatal("phai_doi_mat_khau VẪN true sau khi chính người đó đổi — họ sẽ bị kẹt ở màn đổi mật khẩu mãi mãi")
	}
	if err := password.KiemTra(moi, bam); err != nil {
		t.Fatalf("mật khẩu mới không khớp chuỗi băm đã lưu: %v", err)
	}
	if err := password.KiemTra(kq.MatKhauTam, bam); err == nil {
		t.Fatal("MẬT KHẨU TẠM VẪN DÙNG ĐƯỢC sau khi người đó đổi — quản trị viên vẫn biết mật khẩu của họ (#9)")
	}

	var conMo int
	if err := db.QueryRow(
		`SELECT count(*) FROM phien WHERE tenant_id = $1 AND nguoi_dung_id = $2 AND thu_hoi_luc IS NULL`,
		xa, id).Scan(&conMo); err != nil {
		t.Fatalf("đếm phiên: %v", err)
	}
	if conMo != 0 {
		t.Fatalf("còn %d phiên chưa thu hồi sau khi đổi mật khẩu — đổi mật khẩu phải kết thúc MỌI phiên", conMo)
	}

	// And the new password is nowhere in the ledger either.
	for _, v := range vetCuaXaPg(t, db, xa) {
		if strings.Contains(v, moi) || strings.Contains(v, kq.MatKhauTam) {
			t.Fatal("một mật khẩu nằm trong audit_log")
		}
	}
}

// A REFUSED OPERATION WRITES NOTHING — no credential, no trail. Proved against the server, where a
// half-applied transaction would actually be visible.
func TestPgCapTaiKhoanLanHaiBiTuChoiVaKhongDoiGi(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	const id, ma = "nd-cap-hai-lan", "CB-2026-CAPHAI"

	themDongDanhBaPg(t, db, xa, id, ma)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	uc := ucTaiKhoanThat(db)

	if _, err := uc.Cap(ctx, id, nguoiPg("nd-quan-tri-cap")); err != nil {
		t.Fatalf("Cap lần 1 lỗi: %v", err)
	}
	bamTruoc, _, _ := dongCanBoPg(t, db, xa, id)

	if _, err := uc.Cap(ctx, id, nguoiPg("nd-quan-tri-cap")); err == nil {
		t.Fatal("cấp tài khoản lần hai vẫn thành công")
	}
	bamSau, _, _ := dongCanBoPg(t, db, xa, id)
	if bamSau != bamTruoc {
		t.Fatal("lần cấp thứ hai BỊ TỪ CHỐI nhưng vẫn ghi đè chuỗi băm — người đang dùng tài khoản bị đá ra")
	}
	if n := len(vetCuaXaPg(t, db, xa)); n != 1 {
		t.Fatalf("có %d vết kiểm toán sau một lần cấp thành công và một lần bị từ chối, muốn 1", n)
	}
}

// THE CONSTRAINT OF MIGRATION 0009 §3 IS ENFORCED BY THE SERVER, and this flow is the one that has
// to satisfy it: an account may never exist with an empty hash. Written as a positive check on the
// row this flow produces, plus a direct attempt that the server must refuse.
func TestPgTaiKhoanKhongBaoGioTonTaiVoiChuoiBamRong(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	const id, ma = "nd-rang-buoc-01", "CB-2026-RANGBU"

	themDongDanhBaPg(t, db, xa, id, ma)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	if _, err := ucTaiKhoanThat(db).Cap(ctx, id, nguoiPg("nd-quan-tri-cap")); err != nil {
		t.Fatalf("Cap lỗi: %v", err)
	}

	if _, err := db.Exec(
		`UPDATE nguoi_dung SET mat_khau_hash = '' WHERE tenant_id = $1 AND id = $2`, xa, id); err == nil {
		t.Fatal("máy chủ CHẤP NHẬN một tài khoản có chuỗi băm rỗng — " +
			"ràng buộc nguoi_dung_co_tai_khoan_co_mat_khau (migration 0009 §3) không còn hiệu lực")
	}
}
