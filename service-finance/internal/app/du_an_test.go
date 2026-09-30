package app

// The invariants of the investment project WRITE path (docs/ui-ux/06-giai-ngan.md §9, §8, §13).
//
// EVERY CASE HERE RUNS THE REAL USE CASE OVER THE REAL STORE over the fake driver of
// driver_gia_du_an_test.go. What is being proved is the SQL and the transaction boundaries — a
// hand-written fake store would erase exactly those.
//
// WHAT IS NOT PROVED HERE, so nobody reads more into a green run than is there: nothing PostgreSQL
// does. `UNIQUE (tenant_id, ma)`, `ho_so_luu_tru_cam_xoa_cung`, `CHECK (ke_hoach_von_nam >= 0)` and
// the partition routing are the FLOOR under all of this, and they need a real server.

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// canBo is the acting principal. `CB-…` AND NEVER A ULID — rule 6, invariant 8: `audit_log.actor_id`
// is read years later by somebody handling an inspection, and a business code names a person with no
// lookup still alive while a ULID names nobody.
var canBo = audit.Actor{ID: "CB-2026-7K3M9Q", Kind: "staff", IP: "10.0.0.7"}

func themDuAnHopLe() YeuCauThemDuAn {
	return YeuCauThemDuAn{
		Ma:            "DA-2026-be-tong-hoa-duong-ngo-xom",
		Nam:           2026,
		HangMucID:     "hm-chuyen-tiep",
		Ten:           "Bê tông hoá đường ngõ xóm tổ 6",
		KeHoachVonNam: 100_000_000,
	}
}

func khoDuAnSan() *khoDAGia {
	return &khoDAGia{hangMucCo: map[string]bool{"hm-chuyen-tiep": true}}
}

// --- creating ----------------------------------------------------------------------------------

// The invariant rule 6, invariant 3 exists for: the project, its allocation lines and the audit
// entry are ONE transaction. Not "the write, then the entry if it works".
func TestThemDuAnGhiDuAnPhanBoVaVetTrongCungMotGiaoDich(t *testing.T) {
	k := khoDuAnSan()
	k.nguonVonCo = map[string]bool{"nv-xa|2026": true, "nv-thanh-pho|2026": true}
	uc, ctx := dungUseCaseDuAn(t, k)

	yc := themDuAnHopLe()
	yc.PhanBo = []domain.DongPhanBoMoi{
		{NguonVonID: "nv-xa", SoTien: 40_000_000},
		{NguonVonID: "nv-thanh-pho", SoTien: 60_000_000},
	}

	kq, err := uc.Them(ctx, yc, canBo)
	if err != nil {
		t.Fatalf("Them: %v", err)
	}

	if k.batDau != 1 {
		t.Errorf("mở %d giao dịch, muốn đúng 1 — dự án, phân bổ và vết phải cùng một giao dịch", k.batDau)
	}
	if k.daCommit != 1 || k.daRollback != 0 {
		t.Errorf("commit=%d rollback=%d, muốn commit=1 rollback=0", k.daCommit, k.daRollback)
	}
	if !k.coCau("INSERT INTO du_an") {
		t.Error("không thấy câu chèn dự án")
	}
	if n := len(k.cau("INSERT INTO phan_bo_nguon_von")); n != 2 {
		t.Errorf("chèn %d dòng phân bổ, muốn 2", n)
	}
	if !k.coCau("INSERT INTO audit_log") {
		t.Error("không thấy vết kiểm toán")
	}
	if kq.DuAn.ID != idDuAnMoi {
		t.Errorf("id dự án = %q, muốn %q", kq.DuAn.ID, idDuAnMoi)
	}
	// EVERY ROW GETS ITS OWN ID. A single id shared between `du_an` and `phan_bo_nguon_von` is
	// something the separate PRIMARY KEYs permit and nothing would notice until somebody joined them.
	rieng := map[string]bool{kq.DuAn.ID: true}
	for _, pb := range kq.PhanBo {
		if rieng[pb.ID] {
			t.Errorf("id %q bị dùng lại cho dòng phân bổ", pb.ID)
		}
		rieng[pb.ID] = true
		if pb.DuAnID != kq.DuAn.ID {
			t.Errorf("dòng phân bổ trỏ vào dự án %q, muốn %q", pb.DuAnID, kq.DuAn.ID)
		}
	}
}

// Rule 6, invariant 8: the trail's subject is the BUSINESS CODE, never the internal id. A ULID names
// nobody to somebody handling an inspection years later, and the row it points at may by then be
// gone. `tools/check_audit_actor.py` scans the whole repository for the same defect.
func TestThemDuAnGhiVetTheoMaDuAnVaMaCanBo(t *testing.T) {
	k := khoDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)

	yc := themDuAnHopLe()
	if _, err := uc.Them(ctx, yc, canBo); err != nil {
		t.Fatalf("Them: %v", err)
	}

	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(vet))
	}
	args := vet[0].args
	if got := args[1]; got != canBo.ID {
		t.Errorf("actor_id = %v, muốn mã cán bộ %q — không bao giờ là id nội bộ", got, canBo.ID)
	}
	if got := args[4]; got != HanhViThemDuAn {
		t.Errorf("action = %v, muốn %q", got, HanhViThemDuAn)
	}
	if got := args[5]; got != yc.Ma {
		t.Errorf("subject = %v, muốn mã dự án %q — không bao giờ là ULID", got, yc.Ma)
	}
}

// §11's "mặc định 31/12", applied in ONE place. A commune that leaves the deadline blank must not
// end up with the zero time — which stores 01/01/0001 and makes every project on the screen overdue.
func TestThemDuAnDeTrongHanGiaiNganThiLay3112CuaNamNganSach(t *testing.T) {
	k := khoDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)

	kq, err := uc.Them(ctx, themDuAnHopLe(), canBo)
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	muon := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)
	if !kq.DuAn.ThoiHanGiaiNgan.Equal(muon) {
		t.Errorf("thoi_han_giai_ngan = %v, muốn %v", kq.DuAn.ThoiHanGiaiNgan, muon)
	}
}

// §9: "Để trống thì lấy bằng số tiền bố trí năm nay." The column goes to NULL and the RULE is read
// back out by domain.TongMucHieuLuc — a COPIED value would silently stop following the plan the day
// the plan is revised (0004:199-203).
func TestThemDuAnTongMucDeTrongThiGhiNULLChuKhongChepKeHoach(t *testing.T) {
	k := khoDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)

	if _, err := uc.Them(ctx, themDuAnHopLe(), canBo); err != nil {
		t.Fatalf("Them: %v", err)
	}
	chen := k.cau("INSERT INTO du_an")
	if len(chen) != 1 {
		t.Fatalf("có %d câu chèn, muốn 1", len(chen))
	}
	// $9 is tong_muc_duoc_duyet — index 8 in the arg slice.
	if got := chen[0].args[8]; got != nil {
		t.Errorf("tong_muc_duoc_duyet = %v, muốn NULL khi xã để trống", got)
	}
}

// §9 makes the allocation list OPTIONAL and §11 names the state it produces (`Chưa gắn nguồn`). Every
// rule on this path has to survive a project with no allocation at all — §13 rule 6 says the same
// thing about vouchers.
func TestThemDuAnKhongKhaiNguonVonVanTao(t *testing.T) {
	k := khoDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)

	kq, err := uc.Them(ctx, themDuAnHopLe(), canBo)
	if err != nil {
		t.Fatalf("Them không khai nguồn vốn phải được: %v", err)
	}
	if len(kq.PhanBo) != 0 {
		t.Errorf("có %d dòng phân bổ, muốn 0", len(kq.PhanBo))
	}
	if k.coCau("INSERT INTO phan_bo_nguon_von") {
		t.Error("không khai nguồn nào mà vẫn chèn dòng phân bổ")
	}
	if k.daCommit != 1 {
		t.Errorf("commit=%d, muốn 1", k.daCommit)
	}
}

// ⚠ §9 SAYS THE MISMATCH IS A WARNING, NOT A REFUSAL: the system "đối chiếu tổng các nguồn với số ấy
// và CẢNH BÁO khi thiếu hoặc vượt". This is the case that stops a future reader from turning that
// sentence into a constraint — both directions, short AND over, must be accepted and written.
func TestThemDuAnTongNguonLechKeHoachVanTaoVichiCanhBao(t *testing.T) {
	for _, tc := range []struct {
		ten  string
		tien domain.Dong
	}{
		{"thiếu so với kế hoạch", 10_000_000},
		{"vượt kế hoạch", 500_000_000},
	} {
		t.Run(tc.ten, func(t *testing.T) {
			k := khoDuAnSan()
			k.nguonVonCo = map[string]bool{"nv-xa|2026": true}
			uc, ctx := dungUseCaseDuAn(t, k)

			yc := themDuAnHopLe() // kế hoạch 100.000.000
			yc.PhanBo = []domain.DongPhanBoMoi{{NguonVonID: "nv-xa", SoTien: tc.tien}}

			kq, err := uc.Them(ctx, yc, canBo)
			if err != nil {
				t.Fatalf("lệch tổng nguồn KHÔNG được từ chối (§9 là cảnh báo): %v", err)
			}
			if len(kq.PhanBo) != 1 {
				t.Errorf("có %d dòng phân bổ, muốn 1", len(kq.PhanBo))
			}
			if k.daCommit != 1 {
				t.Errorf("commit=%d, muốn 1", k.daCommit)
			}
		})
	}
}

// §13 rule 8: each budget year is its own set. A source of the right commune but the WRONG year must
// be refused, or a 2026 project's allocation lands on a card §6 draws for 2027.
func TestThemDuAnTuChoiNguonVonKhacNamNganSach(t *testing.T) {
	k := khoDuAnSan()
	k.nguonVonCo = map[string]bool{"nv-xa|2027": true} // live, but for 2027
	uc, ctx := dungUseCaseDuAn(t, k)

	yc := themDuAnHopLe() // nam 2026
	yc.PhanBo = []domain.DongPhanBoMoi{{NguonVonID: "nv-xa", SoTien: 100_000_000}}

	_, err := uc.Them(ctx, yc, canBo)
	if !errors.Is(err, fistore.ErrKhongThayNguonVonPhanBo) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayNguonVonPhanBo", err)
	}
	if k.daCommit != 0 {
		t.Errorf("commit=%d, muốn 0 — từ chối thì không ghi gì", k.daCommit)
	}
}

// §9: "Mã tự nhập phải chưa từng được dùng, KỂ CẢ BỞI DỰ ÁN ĐÃ RÚT KHỎI DANH SÁCH." The check must
// carry NO `deleted_at` predicate — rule 7, invariant 3: an issued code is never reissued.
func TestThemDuAnKiemMaTrungTinhCaDuAnDaXoaMem(t *testing.T) {
	k := khoDuAnSan()
	k.maDaDung = 1 // the only row carrying this code is soft-deleted
	uc, ctx := dungUseCaseDuAn(t, k)

	_, err := uc.Them(ctx, themDuAnHopLe(), canBo)
	if !errors.Is(err, fistore.ErrMaDuAnDaTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrMaDuAnDaTonTai", err)
	}

	kiem := k.cau("count(*) FROM du_an")
	if len(kiem) != 1 {
		t.Fatalf("có %d câu kiểm mã, muốn 1", len(kiem))
	}
	// THE ASSERTION THAT MATTERS. With `AND deleted_at IS NULL` in that statement, a withdrawn
	// project's code becomes available again — and the vouchers of the first project then read as
	// belonging to the second.
	if strings.Contains(kiem[0].sql, "deleted_at") {
		t.Errorf("câu kiểm mã trùng có lọc deleted_at — mã đã cấp thì không cấp lại: %q", kiem[0].sql)
	}
	if k.daCommit != 0 {
		t.Errorf("commit=%d, muốn 0", k.daCommit)
	}
}

// There is no foreign key under `du_an.hang_muc_id` (0004:182-190), so this check IS the constraint.
// A project classified under nothing is money counted in §3's KPI card and missing from every row of
// §5's table — two totals on one screen that disagree.
func TestThemDuAnTuChoiHangMucKhongCoTrongXa(t *testing.T) {
	k := khoDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)

	yc := themDuAnHopLe()
	yc.HangMucID = "hm-cua-xa-khac"

	_, err := uc.Them(ctx, yc, canBo)
	if !errors.Is(err, fistore.ErrKhongThayHangMuc) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayHangMuc", err)
	}
	if k.coCau("INSERT INTO du_an") {
		t.Error("từ chối hạng mục mà vẫn chèn dự án")
	}
	if k.daCommit != 0 {
		t.Errorf("commit=%d, muốn 0", k.daCommit)
	}
}

// §9 offers `☑ Tự sinh mã` and this service does not generate one — the specification gives two
// incompatible formats and no scope for the sequence, and a project code is an ISSUED code. The
// refusal must be a 400-shaped domain error, not a 500 and not a silent blank code.
func TestThemDuAnThieuMaThiTuChoiChuKhongTuSinh(t *testing.T) {
	k := khoDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)

	yc := themDuAnHopLe()
	yc.Ma = "   "

	_, err := uc.Them(ctx, yc, canBo)
	if !errors.Is(err, domain.ErrThieuMaDuAn) {
		t.Fatalf("lỗi = %v, muốn ErrThieuMaDuAn", err)
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch, muốn 0 — hình dạng sai thì không được giữ khoá dòng", k.batDau)
	}
}

// Rule 6: a business write whose trail cannot name its author is refused BEFORE the transaction
// opens. core/audit refuses an entry with no actor too; refusing here keeps the row and the entry
// telling the same story and avoids a rollback whose cause is a missing principal.
func TestThemDuAnKhongCoChuTheThiTuChoiTruocKhiMoGiaoDich(t *testing.T) {
	k := khoDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)

	if _, err := uc.Them(ctx, themDuAnHopLe(), audit.Actor{Kind: "staff"}); err == nil {
		t.Fatal("thiếu chủ thể mà vẫn ghi được")
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch, muốn 0", k.batDau)
	}
}

// The entry is the LAST statement of the transaction, so a failure there must take the project and
// its allocation lines down with it. This is the shape rule 6, invariant 3 exists for, and it is why
// `loiSau` fails one statement rather than everything.
func TestThemDuAnVetHongThiKhongCoDuAnNao(t *testing.T) {
	k := khoDuAnSan()
	k.loiSau = "INSERT INTO audit_log"
	uc, ctx := dungUseCaseDuAn(t, k)

	if _, err := uc.Them(ctx, themDuAnHopLe(), canBo); err == nil {
		t.Fatal("vết hỏng mà Them vẫn thành công")
	}
	if k.daCommit != 0 {
		t.Errorf("commit=%d, muốn 0 — dự án không được commit khi vết hỏng", k.daCommit)
	}
	if k.daRollback != 1 {
		t.Errorf("rollback=%d, muốn 1", k.daRollback)
	}
}

// --- editing -----------------------------------------------------------------------------------

func hangDuAnSan() *hangDA {
	return &hangDA{
		id:          "01JDUANCU0000000000000000",
		ma:          "DA-2026-cu",
		nam:         2026,
		hangMucID:   "hm-chuyen-tiep",
		ten:         "Dự án cũ",
		keHoach:     100_000_000,
		hanGiaiNgan: time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC),
	}
}

// ⚠ RULE 7, FORBIDDEN #4 AND §13 RULE 8, PROVED AT THE STATEMENT. `ma` and `nam` must appear in NO
// update statement: a code that has been issued is never renumbered, and moving a project between
// budget years takes its whole plan and every voucher filed against it out of one year's totals.
// The domain refuses a body naming either; this proves the column is not even reachable.
func TestSuaDuAnKhongBaoGioGhiMaHoacNam(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)

	ten := "Dự án đã đổi tên"
	if _, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{Ten: &ten}, canBo); err != nil {
		t.Fatalf("Sua: %v", err)
	}

	capNhat := k.cau("UPDATE du_an")
	if len(capNhat) != 1 {
		t.Fatalf("có %d câu cập nhật, muốn 1", len(capNhat))
	}
	// A WORD BOUNDARY AND NOT strings.Contains, AND THE FIRST VERSION OF THIS ASSERTION WAS WRONG
	// BECAUSE OF IT: `ke_hoach_von_nam = $6` contains the substring "nam =", so a plain Contains
	// reported a correct statement as a violation. `\b` does not match between `_` and `n`, so
	// `\bnam\s*=` sees the column `nam` and never a column merely ending in it.
	for _, cot := range []string{`\bma\s*=`, `\bnam\s*=`} {
		if regexp.MustCompile(cot).MatchString(capNhat[0].sql) {
			t.Errorf("câu cập nhật có cột %s — cột này không được sửa: %q", cot, capNhat[0].sql)
		}
	}
}

// A no-op writes NOTHING and audits NOTHING. That is what makes `idem.KhongCan` on the PATCH route a
// property rather than a hope, and it is what keeps a public authority's ledger free of entries
// saying nothing changed — the entries that bury the ones carrying legal weight.
func TestSuaDuAnKhongDoiGiThiKhongGhiVaKhongCoVet(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)

	ten := k.hang.ten // exactly what the row already holds
	if _, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{Ten: &ten}, canBo); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if k.coCau("UPDATE du_an") {
		t.Error("không có gì đổi mà vẫn ghi dòng")
	}
	if k.coCau("INSERT INTO audit_log") {
		t.Error("không có gì đổi mà vẫn ghi vết")
	}
}

// Rule 6, invariant 5: before AND after, and only the fields that moved. `ke_hoach_von_nam` is the
// field this delta exists for — it is the denominator of §3's delay score and §7.2's ratio, so
// revising it moves a project from "chậm" to "bám sát tiến độ" without one đồng having moved.
func TestSuaDuAnVetMangTruocVaSauCuaKeHoachVon(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)

	moi := domain.Dong(250_000_000)
	if _, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{KeHoachVonNam: &moi}, canBo); err != nil {
		t.Fatalf("Sua: %v", err)
	}

	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(vet))
	}
	if got := vet[0].args[5]; got != k.hang.ma {
		t.Errorf("subject = %v, muốn mã dự án %q", got, k.hang.ma)
	}

	var than struct {
		Truoc map[string]any `json:"truoc"`
		Sau   map[string]any `json:"sau"`
	}
	tho, ok := vet[0].args[7].([]byte)
	if !ok {
		t.Fatalf("delta không phải []byte: %T", vet[0].args[7])
	}
	if err := json.Unmarshal(tho, &than); err != nil {
		t.Fatalf("đọc delta: %v", err)
	}
	if than.Truoc["ke_hoach_von_nam"] != float64(k.hang.keHoach) {
		t.Errorf("truoc.ke_hoach_von_nam = %v, muốn %d", than.Truoc["ke_hoach_von_nam"], k.hang.keHoach)
	}
	if than.Sau["ke_hoach_von_nam"] != float64(moi) {
		t.Errorf("sau.ke_hoach_von_nam = %v, muốn %d", than.Sau["ke_hoach_von_nam"], moi)
	}
	// ONLY THE FIELDS THAT MOVED. A delta carrying every column on every edit makes the one field
	// somebody actually changed impossible to find in a ledger that is never deleted.
	if _, co := than.Truoc["ten"]; co {
		t.Error("delta mang `ten` mà tên không đổi")
	}
}

// ⚠ ADR 0036 DECIDED FOR THE VOUCHER, NOT FOR THE PROJECT. A project has no `trang_thai` column and
// no confirmation to lose, so editing one whose vouchers are confirmed or locked is an ordinary
// edit. This case exists so that a future reader does not carry that ADR across and invent a
// lifecycle the specification never gave this record.
func TestSuaDuAnCoChungTuDaXacNhanVanSuaBinhThuong(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	k.soChungTu = 3 // vouchers exist, some confirmed — irrelevant to an edit
	uc, ctx := dungUseCaseDuAn(t, k)

	moi := domain.Dong(250_000_000)
	sau, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{KeHoachVonNam: &moi}, canBo)
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if sau.KeHoachVonNam != moi {
		t.Errorf("ke_hoach_von_nam = %d, muốn %d", sau.KeHoachVonNam, moi)
	}
	if k.daCommit != 1 {
		t.Errorf("commit=%d, muốn 1", k.daCommit)
	}
	// AND NOTHING COUNTED THE VOUCHERS. A count here would mean the edit path had grown a rule the
	// specification never stated.
	if k.coCau("count(*) FROM chung_tu_giai_ngan") {
		t.Error("đường SỬA đếm chứng từ — đó là luật của đường XOÁ, không phải của đường sửa")
	}
}

// Correcting a name must not fail because the category the project has always been classified under
// was removed from the catalogue afterwards. The row is the historical fact; refusing to edit
// anything else would strand it.
func TestSuaDuAnKhongDoiHangMucThiKhongKiemHangMuc(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	k.hangMucCo = map[string]bool{} // the category has since been removed
	uc, ctx := dungUseCaseDuAn(t, k)

	ten := "Tên mới"
	if _, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{Ten: &ten}, canBo); err != nil {
		t.Fatalf("sửa tên không được phụ thuộc vào hạng mục cũ: %v", err)
	}
	if k.coCau("FROM hang_muc_ke_hoach_von") {
		t.Error("không đổi hạng mục mà vẫn kiểm hạng mục")
	}
}

// Reclassifying INTO a category this commune does not have is what is refused — rule 1: a category
// of another commune is indistinguishable from one that does not exist.
func TestSuaDuAnDoiSangHangMucKhongCoThiTuChoi(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)

	moi := "hm-cua-xa-khac"
	_, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{HangMucID: &moi}, canBo)
	if !errors.Is(err, fistore.ErrKhongThayHangMuc) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayHangMuc", err)
	}
	if k.coCau("UPDATE du_an") {
		t.Error("từ chối hạng mục mà vẫn ghi dòng")
	}
}

// A project of another commune, or one that does not exist, is one answer: the query cannot reach
// another commune's row at all (rule 1). The handler answers 404 either way.
func TestSuaDuAnKhongCoThiBaoKhongThay(t *testing.T) {
	k := khoDuAnSan() // k.hang is nil
	uc, ctx := dungUseCaseDuAn(t, k)

	ten := "Tên mới"
	_, err := uc.Sua(ctx, "01JKHONGCO000000000000000", YeuCauSuaDuAn{Ten: &ten}, canBo)
	if !errors.Is(err, fistore.ErrKhongThayDuAn) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayDuAn", err)
	}
}

// --- removing ----------------------------------------------------------------------------------

// ⚠ THE STOP CONDITION, PROVED. The specification says nothing about removing a project that still
// carries vouchers; refusing is the only direction that writes nothing and can be loosened later
// with one branch. What must NOT happen is the project going while the vouchers stay live — their
// money would vanish from §3 and §5 while the rows sit in the table, and the commune's own totals
// would stop agreeing with the sum of its own vouchers.
func TestXoaDuAnConChungTuThiTuChoiVaKhongGhiGi(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	k.soChungTu = 1
	uc, ctx := dungUseCaseDuAn(t, k)

	err := uc.Xoa(ctx, k.hang.id, "nhập nhầm năm ngân sách", canBo)
	if !errors.Is(err, fistore.ErrDuAnConChungTu) {
		t.Fatalf("lỗi = %v, muốn ErrDuAnConChungTu", err)
	}
	if k.coCau("UPDATE du_an") {
		t.Error("từ chối mà vẫn xoá mềm dự án")
	}
	if k.coCau("UPDATE phan_bo_nguon_von") {
		t.Error("từ chối mà vẫn xoá mềm dòng phân bổ")
	}
	if k.coCau("INSERT INTO audit_log") {
		t.Error("từ chối mà vẫn ghi vết")
	}
	if k.daCommit != 0 {
		t.Errorf("commit=%d, muốn 0", k.daCommit)
	}
}

// A soft-deleted voucher is already out of every read path and out of `da_giai_ngan`, so nothing is
// stranded by removing the project it pointed at. The count must therefore exclude them.
func TestXoaDuAnChiConChungTuDaGoThiXoaDuoc(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	k.soChungTu = 0 // every voucher carries deleted_at
	uc, ctx := dungUseCaseDuAn(t, k)

	if err := uc.Xoa(ctx, k.hang.id, "dự án rút khỏi kế hoạch năm", canBo); err != nil {
		t.Fatalf("Xoa: %v", err)
	}
	dem := k.cau("count(*) FROM chung_tu_giai_ngan")
	if len(dem) != 1 {
		t.Fatalf("có %d câu đếm chứng từ, muốn 1", len(dem))
	}
	if !strings.Contains(dem[0].sql, "deleted_at IS NULL") {
		t.Errorf("câu đếm chứng từ không loại chứng từ đã gỡ: %q", dem[0].sql)
	}
}

// Rule 7, invariant 1: `deleted_at`, `deleted_by` AND `delete_reason`, all three, in ONE statement so
// none can be forgotten. `deleted_by` holds the STAFF BUSINESS CODE — two kinds of identifier in one
// column is a column nobody can query (rule 6, invariant 8).
func TestXoaDuAnGhiDuBaCotVaXoaLuonDongPhanBo(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	k.soPhanBoXoa = 3
	uc, ctx := dungUseCaseDuAn(t, k)

	const lyDo = "xã rút dự án khỏi kế hoạch vốn năm 2026"
	if err := uc.Xoa(ctx, k.hang.id, lyDo, canBo); err != nil {
		t.Fatalf("Xoa: %v", err)
	}

	xoa := k.cau("UPDATE du_an")
	if len(xoa) != 1 {
		t.Fatalf("có %d câu xoá mềm dự án, muốn 1", len(xoa))
	}
	for _, cot := range []string{"deleted_at", "deleted_by", "delete_reason"} {
		if !strings.Contains(xoa[0].sql, cot) {
			t.Errorf("câu xoá mềm thiếu %q: %q", cot, xoa[0].sql)
		}
	}
	// A SECOND REMOVAL MUST BE A 404, NOT A SILENT REWRITE of who removed it and why — the first
	// removal is the one that happened (rule 7, forbidden #5).
	if !strings.Contains(xoa[0].sql, "deleted_at IS NULL") {
		t.Errorf("câu xoá mềm thiếu `AND deleted_at IS NULL`: %q", xoa[0].sql)
	}
	if got := xoa[0].args[2]; got != canBo.ID {
		t.Errorf("deleted_by = %v, muốn mã cán bộ %q", got, canBo.ID)
	}

	// THE ALLOCATION LINES GO WITH THE PROJECT. Left live they would keep counting toward §6's
	// "đã phân bổ" and toward that source's "Số dự án" for a project no screen can show.
	pb := k.cau("UPDATE phan_bo_nguon_von")
	if len(pb) != 1 {
		t.Fatalf("có %d câu xoá mềm dòng phân bổ, muốn 1", len(pb))
	}
	if got := pb[0].args[2]; got != canBo.ID {
		t.Errorf("deleted_by của dòng phân bổ = %v, muốn %q", got, canBo.ID)
	}

	// THE COUNT REACHES THE TRAIL, because nothing else can rebuild it once the lines carry
	// `deleted_at` — and it is what explains why a §6 card's "đã phân bổ" dropped.
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(vet))
	}
	var than map[string]any
	tho, _ := vet[0].args[7].([]byte)
	if err := json.Unmarshal(tho, &than); err != nil {
		t.Fatalf("đọc delta: %v", err)
	}
	if than["so_dong_phan_bo"] != float64(k.soPhanBoXoa) {
		t.Errorf("so_dong_phan_bo = %v, muốn %d", than["so_dong_phan_bo"], k.soPhanBoXoa)
	}
	if than["ly_do"] != lyDo {
		t.Errorf("ly_do = %v, muốn %q", than["ly_do"], lyDo)
	}
}

// Rule 7, invariant 1 names `delete_reason` and it is not optional: a project that vanished from the
// commune's plan with no reason attached is a whole year's allocation nobody can account for — and
// the row is still there, so the question WILL be asked.
func TestXoaDuAnThieuLyDoThiTuChoiTruocKhiMoGiaoDich(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)

	err := uc.Xoa(ctx, k.hang.id, "   ", canBo)
	if !errors.Is(err, domain.ErrThieuLyDoXoaDuAn) {
		t.Fatalf("lỗi = %v, muốn ErrThieuLyDoXoaDuAn", err)
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch, muốn 0", k.batDau)
	}
}

// There is no hard delete anywhere on this path. `ho_so_luu_tru_cam_xoa_cung` refuses one underneath
// (0004:325-328); this proves the application never even writes the statement.
func TestXoaDuAnKhongBaoGioSinhDELETE(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)

	if err := uc.Xoa(ctx, k.hang.id, "rút khỏi kế hoạch", canBo); err != nil {
		t.Fatalf("Xoa: %v", err)
	}
	for _, l := range k.lenh {
		if strings.Contains(strings.ToUpper(l.sql), "DELETE FROM") {
			t.Errorf("sinh câu xoá cứng: %q", l.sql)
		}
	}
}
