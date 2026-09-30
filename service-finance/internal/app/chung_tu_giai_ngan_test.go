package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// WHAT THIS FILE IS FOR: the six write use cases of the disbursement voucher register, asserted
// through the REAL store over a fake driver (driver_gia_chung_tu_test.go), so that every property
// below is a property of the SQL and the transaction boundaries rather than of a mock's call log.
//
// SIX THINGS, each of which fails SILENTLY if it stops holding:
//
//  1. the business write and its audit entry share ONE transaction (rule 6, invariant 3);
//  2. every refusal leaves NOTHING committed — a rolled-back transaction, no row, no entry;
//  3. the trail's actor is the STAFF BUSINESS CODE, never an internal id (rule 6, invariant 8);
//  4. the unlock rules of migration 0005 — a mandatory reason, and never the person who locked it;
//  5. `so_lan_mo_khoa` is incremented IN SQL, so no retry and no lost lock can drop a count;
//  6. `CHECK (so_tien > 0)` is not worked around anywhere, in either direction (open question #30).

// The two acting people. SEPARATE CONSTANTS, and the whole self-unlock rule is the difference
// between them: a test that used one code for both could not tell "somebody else unlocked it" from
// "the rule is not being checked".
const (
	maKeToan  = "CB-00123"
	maLanhDao = "CB-00999"
	maDuAnMau = "DA-2026-be-tong-hoa-duong-ngo-xo-2"
	idChungTu = "01JCHUNGTUDANGCO000000000"
	duAnIDMau = "01JDUANCUAXAA000000000000"
)

func canBoCT(ma string) audit.Actor {
	return audit.Actor{ID: ma, Kind: "staff", IP: "10.0.0.7"}
}

var ngayChiMau = time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)

func themChungTuMau() YeuCauThemChungTu {
	return YeuCauThemChungTu{
		DuAnID:    duAnIDMau,
		NgayChi:   ngayChiMau,
		SoTien:    30_000_000,
		NoiDung:   "Thanh toán đợt 3",
		DoiTac:    "Công ty ABC",
		SoChungTu: "CT-2026-119",
	}
}

// khoaDaKhoa returns a voucher already frozen by `boi`.
func hangDaKhoa(boi string) *hangCT {
	khoa := lucCoDinh.Add(-24 * time.Hour)
	return &hangCT{
		id: idChungTu, duAnID: duAnIDMau, ngayChi: ngayChiMau, soTien: 30_000_000,
		noiDung: "Thanh toán đợt 3", doiTac: "Công ty ABC", soChungTu: "CT-2026-119",
		trangThai: string(domain.ChungTuDaKhoa),
		nguoiNhap: maKeToan, nguoiXacNhan: boi, nguoiKhoa: boi,
		thoiDiemKhoa: &khoa,
	}
}

func hangOTrangThai(tt domain.TrangThaiChungTu) *hangCT {
	return &hangCT{
		id: idChungTu, duAnID: duAnIDMau, ngayChi: ngayChiMau, soTien: 30_000_000,
		noiDung: "Thanh toán đợt 3", doiTac: "Công ty ABC", soChungTu: "CT-2026-119",
		trangThai: string(tt), nguoiNhap: maKeToan,
	}
}

// --- (1) the write and the trail share one transaction -------------------------------------------

func TestThemChungTu_GhiVaVetTrongCungMotGiaoDich(t *testing.T) {
	// THE INVARIANT THIS WHOLE LAYER EXISTS FOR (rule 6, invariant 3). The previous system had no
	// transactions anywhere, so "every write leaves a trail" could not actually hold: there was
	// always a window where the money had moved and the trail had not.
	k := &khoCTGia{maDuAn: maDuAnMau}
	uc, ctx := dungUseCaseChungTu(t, k)

	moi, err := uc.Them(ctx, themChungTuMau(), canBoCT(maKeToan))
	if err != nil {
		t.Fatalf("Them lỗi: %v", err)
	}
	if moi.ID != idChungTuMoi {
		t.Errorf("id = %q, muốn %q", moi.ID, idChungTuMoi)
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("giao dịch: mở %d, commit %d, rollback %d — muốn 1/1/0",
			k.batDau, k.daCommit, k.daRollback)
	}
	if !k.coCau("INSERT INTO chung_tu_giai_ngan") {
		t.Error("không có câu chèn chứng từ")
	}
	if !k.coCau("INSERT INTO audit_log") {
		t.Error("không có dòng vết kiểm toán — luật 6 bất biến 1")
	}
}

func TestThemChungTu_VetHongThiChungTuCungKhongCon(t *testing.T) {
	// THE ONE ORDERING THAT MATTERS. The audit entry is the LAST statement of the transaction, so a
	// failure there has to take the voucher down with it. If it did not, the register would hold a
	// payment nobody can attribute — a state the records rules do not permit (rule 2, invariant 6).
	k := &khoCTGia{maDuAn: maDuAnMau, loiSau: "INSERT INTO audit_log"}
	uc, ctx := dungUseCaseChungTu(t, k)

	if _, err := uc.Them(ctx, themChungTuMau(), canBoCT(maKeToan)); err == nil {
		t.Fatal("vết hỏng mà Them vẫn báo thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit %d, rollback %d — muốn 0/1", k.daCommit, k.daRollback)
	}
}

func TestVetMangMaCanBoChuKhongPhaiIDNoiBo(t *testing.T) {
	// RULE 6, INVARIANT 8. `audit_log.actor_id` is read years later by somebody handling an
	// inspection: `CB-00123` names a person with no lookup still alive, a ULID names nobody — and
	// the two are indistinguishable on sight, so a column holding both is a column nobody can query.
	// Six write paths in this repository put the internal id there on 2026-09-22 and no test turned
	// red.
	k := &khoCTGia{maDuAn: maDuAnMau}
	uc, ctx := dungUseCaseChungTu(t, k)

	if _, err := uc.Them(ctx, themChungTuMau(), canBoCT(maKeToan)); err != nil {
		t.Fatalf("Them lỗi: %v", err)
	}
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d dòng vết, muốn 1", len(vet))
	}
	// audit.Write binds (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta).
	if got := vet[0].args[1]; got != maKeToan {
		t.Errorf("actor_id = %v, muốn MÃ CÁN BỘ %q", got, maKeToan)
	}
	if got := vet[0].args[4]; got != HanhViThemChungTu {
		t.Errorf("action = %v, muốn %q", got, HanhViThemChungTu)
	}
	// THE SUBJECT IS A BUSINESS CODE. A voucher has no `ma` column of its own, so it is the
	// project's code — the identifier an inspection can actually look up — and the voucher is named
	// inside the delta.
	if got := vet[0].args[5]; got != maDuAnMau {
		t.Errorf("subject = %v, muốn mã dự án %q", got, maDuAnMau)
	}
	if delta, ok := vet[0].args[7].([]byte); !ok || !strings.Contains(string(delta), idChungTuMoi) {
		t.Errorf("delta không nêu chứng từ nào: %s", vet[0].args[7])
	}
}

// --- (2) the state is not the client's, and the INSERT proves it ----------------------------------

func TestThemChungTu_TrangThaiLaHangSoTrongCauChen(t *testing.T) {
	// `'ke-toan-nhap'` IS A LITERAL IN THE INSERT, not a bound parameter, and that is the property
	// rather than a style: with no $n for the state there is no value any layer above could pass. A
	// voucher created already `Đã khoá` would be a figure nobody confirmed, frozen against editing,
	// counting toward the commune's disbursement total.
	k := &khoCTGia{maDuAn: maDuAnMau}
	uc, ctx := dungUseCaseChungTu(t, k)

	if _, err := uc.Them(ctx, themChungTuMau(), canBoCT(maKeToan)); err != nil {
		t.Fatalf("Them lỗi: %v", err)
	}
	chen := k.cau("INSERT INTO chung_tu_giai_ngan")[0]
	if !strings.Contains(chen.sql, "'ke-toan-nhap'") {
		t.Errorf("câu chèn không viết cứng trạng thái:\n%s", chen.sql)
	}
	for _, a := range chen.args {
		if s, ok := a.(string); ok && (s == "da-khoa" || s == "da-xac-nhan") {
			t.Fatalf("trạng thái đi vào câu chèn như THAM SỐ (%q) — client đặt được vòng đời", s)
		}
	}
}

// --- (3) so_tien > 0 is not worked around, in either direction ------------------------------------

func TestThemChungTu_SoTienKhongDuongThiTuChoiTruocKhiMoGiaoDich(t *testing.T) {
	// BOTH DIRECTIONS. Zero records nothing and is only ever an import artefact; NEGATIVE is a
	// REFUND — a different business event with a different name (open question #30, ADR 0035 §B) —
	// and letting it in here would silently reduce a disbursement total a decision has already
	// quoted.
	//
	// "TRƯỚC KHI MỞ GIAO DỊCH" is the second half: a request that fails its shape must not hold a
	// row lock while doing so.
	for ten, so := range map[string]domain.Dong{"không đồng": 0, "âm — khoản hoàn": -30_000_000} {
		t.Run(ten, func(t *testing.T) {
			k := &khoCTGia{maDuAn: maDuAnMau}
			uc, ctx := dungUseCaseChungTu(t, k)

			yc := themChungTuMau()
			yc.SoTien = so
			_, err := uc.Them(ctx, yc, canBoCT(maKeToan))
			if !errors.Is(err, domain.ErrSoTienKhongDuong) {
				t.Fatalf("lỗi = %v, muốn ErrSoTienKhongDuong", err)
			}
			if k.batDau != 0 {
				t.Errorf("mở %d giao dịch cho một yêu cầu sai hình dạng, muốn 0", k.batDau)
			}
		})
	}
}

func TestSuaChungTu_SoTienKhongDuongThiTuChoi(t *testing.T) {
	// The same rule on the edit path, because the detour open question #30 warns about is not a
	// negative INSERT — it is editing an old voucher down. Down to a SMALLER positive figure is
	// allowed and audited; down to zero or below is refused here as it is on create.
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangOTrangThai(domain.ChungTuKeToanNhap)}
	uc, ctx := dungUseCaseChungTu(t, k)

	so := domain.Dong(0)
	_, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{SoTien: &so}, canBoCT(maKeToan))
	if !errors.Is(err, domain.ErrSoTienKhongDuong) {
		t.Fatalf("lỗi = %v, muốn ErrSoTienKhongDuong", err)
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch, muốn 0", k.batDau)
	}
}

func TestSuaChungTu_GiamSoTienDeLaiVetCoCaTruocVaSau(t *testing.T) {
	// THE DETOUR THIS DELTA EXISTS TO MAKE VISIBLE. Editing an old voucher down to a smaller figure
	// is how a refund never appears as an event. The edit is legitimate as a correction and
	// indistinguishable from the detour on the row itself; the only thing that tells them apart
	// afterwards is this pair of numbers in a ledger that cannot be edited.
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangOTrangThai(domain.ChungTuKeToanNhap)}
	uc, ctx := dungUseCaseChungTu(t, k)

	so := domain.Dong(12_000_000)
	if _, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{SoTien: &so}, canBoCT(maKeToan)); err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d dòng vết, muốn 1", len(vet))
	}
	delta, _ := vet[0].args[7].([]byte)
	for _, muon := range []string{"30000000", "12000000"} {
		if !strings.Contains(string(delta), muon) {
			t.Errorf("delta thiếu %s — không đối chiếu được số trước và sau:\n%s", muon, delta)
		}
	}
}

// --- (4) a locked voucher is frozen, and the sentence is the point --------------------------------

func TestSuaChungTu_DangKhoaThiTuChoiVaKhongGhiGi(t *testing.T) {
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangDaKhoa(maLanhDao)}
	uc, ctx := dungUseCaseChungTu(t, k)

	noi := "Sửa nội dung"
	_, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{NoiDung: &noi}, canBoCT(maKeToan))
	if !errors.Is(err, domain.ErrChungTuDaKhoa) {
		t.Fatalf("lỗi = %v, muốn ErrChungTuDaKhoa", err)
	}
	// NOTHING COMMITTED. The trigger would refuse the UPDATE underneath too — but it would do so
	// with an English exception naming a constraint, after the statement had been sent.
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit %d, rollback %d — muốn 0/1", k.daCommit, k.daRollback)
	}
	if k.coCau("UPDATE chung_tu_giai_ngan") {
		t.Error("chứng từ đang khoá mà vẫn gửi câu UPDATE")
	}
	if k.coCau("INSERT INTO audit_log") {
		t.Error("từ chối mà vẫn ghi vết")
	}
}

func TestGoChungTu_DangKhoaThiTuChoi(t *testing.T) {
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangDaKhoa(maLanhDao)}
	uc, ctx := dungUseCaseChungTu(t, k)

	err := uc.Go(ctx, idChungTu, "nhập trùng", canBoCT(maLanhDao))
	if !errors.Is(err, domain.ErrChungTuDaKhoa) {
		t.Fatalf("lỗi = %v, muốn ErrChungTuDaKhoa", err)
	}
	if k.coCau("deleted_at = now()") {
		t.Error("chứng từ đang khoá mà vẫn gửi câu xoá mềm")
	}
}

func TestGoChungTu_ThieuLyDoThiTuChoi(t *testing.T) {
	// Rule 7, invariant 1 names THREE columns — `deleted_at`, `deleted_by`, `delete_reason`. A
	// voucher that vanished from a project's total with no reason attached is money nobody can
	// account for, and the row is still there, so the question WILL be asked.
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangOTrangThai(domain.ChungTuKeToanNhap)}
	uc, ctx := dungUseCaseChungTu(t, k)

	if err := uc.Go(ctx, idChungTu, "   ", canBoCT(maKeToan)); !errors.Is(err, domain.ErrThieuLyDoGo) {
		t.Fatalf("lỗi = %v, muốn ErrThieuLyDoGo", err)
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch, muốn 0", k.batDau)
	}
}

func TestGoChungTu_XoaMemGhiDuBaCot(t *testing.T) {
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangOTrangThai(domain.ChungTuKeToanNhap)}
	uc, ctx := dungUseCaseChungTu(t, k)

	if err := uc.Go(ctx, idChungTu, "nhập trùng hai lần", canBoCT(maKeToan)); err != nil {
		t.Fatalf("Go lỗi: %v", err)
	}
	xoa := k.cau("deleted_at = now()")
	if len(xoa) != 1 {
		t.Fatalf("có %d câu xoá mềm, muốn 1", len(xoa))
	}
	for _, cot := range []string{"deleted_at", "deleted_by", "delete_reason", "deleted_at IS NULL"} {
		if !strings.Contains(xoa[0].sql, cot) {
			t.Errorf("câu xoá mềm thiếu %q:\n%s", cot, xoa[0].sql)
		}
	}
	// NO HARD DELETE ANYWHERE. `ho_so_luu_tru_cam_xoa_cung` refuses one underneath, and nothing in
	// this package sends one.
	if k.coCau("DELETE FROM") {
		t.Fatal("có câu xoá cứng — luật 7 cấm #1")
	}
	// `deleted_by` HOLDS THE STAFF BUSINESS CODE, the same value the entry's actor holds.
	if got := xoa[0].args[2]; got != maKeToan {
		t.Errorf("deleted_by = %v, muốn mã cán bộ %q", got, maKeToan)
	}
}

// --- (5) the lifecycle is a chain -----------------------------------------------------------------

func TestKhoa_ChuaXacNhanThiChuaKhoaDuoc(t *testing.T) {
	// §8.2's screen draws `Xác nhận` and `Khoá` on the same `Kế toán nhập` row, so this WILL look
	// like a bug from the outside. It is not: unlocking has to restore the state before the lock,
	// and the row does not store what that was. Requiring the chain makes "before the lock" always
	// `Đã xác nhận`, so an unlock invents nothing.
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangOTrangThai(domain.ChungTuKeToanNhap)}
	uc, ctx := dungUseCaseChungTu(t, k)

	_, err := uc.Khoa(ctx, idChungTu, canBoCT(maLanhDao))
	if !errors.Is(err, domain.ErrChuaXacNhanThiChuaKhoaDuoc) {
		t.Fatalf("lỗi = %v, muốn ErrChuaXacNhanThiChuaKhoaDuoc", err)
	}
	if k.coCau("trang_thai = 'da-khoa'") {
		t.Error("chưa xác nhận mà vẫn gửi câu khoá")
	}
}

func TestKhoa_GhiCaNguoiKhoaVaThoiDiemTrongMotCau(t *testing.T) {
	// BOTH COLUMNS IN ONE STATEMENT because the database requires both: `Đã khoá` with no timestamp
	// fails `chung_tu_giai_ngan_khoa_co_thoi_diem` and `Đã khoá` with nobody attached fails
	// `chung_tu_giai_ngan_khoa_co_nguoi`. Written as two statements the first would simply be
	// refused — and decision (2) of migration 0005 would have nobody to compare an unlocker against.
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangOTrangThai(domain.ChungTuDaXacNhan)}
	uc, ctx := dungUseCaseChungTu(t, k)

	sau, err := uc.Khoa(ctx, idChungTu, canBoCT(maLanhDao))
	if err != nil {
		t.Fatalf("Khoa lỗi: %v", err)
	}
	if sau.TrangThai != domain.ChungTuDaKhoa || sau.NguoiKhoaID != maLanhDao {
		t.Errorf("sau khi khoá = %q bởi %q", sau.TrangThai, sau.NguoiKhoaID)
	}
	khoa := k.cau("trang_thai = 'da-khoa'")
	if len(khoa) != 1 {
		t.Fatalf("có %d câu khoá, muốn 1", len(khoa))
	}
	if !strings.Contains(khoa[0].sql, "nguoi_khoa_id") || !strings.Contains(khoa[0].sql, "thoi_diem_khoa") {
		t.Errorf("câu khoá không ghi đủ người và thời điểm:\n%s", khoa[0].sql)
	}
	if got := khoa[0].args[3]; got != lucCoDinh {
		t.Errorf("thoi_diem_khoa = %v, muốn %v", got, lucCoDinh)
	}
}

func TestXacNhan_LanThuHaiThiTuChoi(t *testing.T) {
	// The second confirmation would overwrite who confirmed it and when — editing a historical fact
	// (rule 7, forbidden #5).
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangOTrangThai(domain.ChungTuDaXacNhan)}
	uc, ctx := dungUseCaseChungTu(t, k)

	if _, err := uc.XacNhan(ctx, idChungTu, canBoCT(maLanhDao)); !errors.Is(err, domain.ErrChungTuDaXacNhan) {
		t.Fatalf("lỗi = %v, muốn ErrChungTuDaXacNhan", err)
	}
	if k.coCau("trang_thai = 'da-xac-nhan'") {
		t.Error("xác nhận lần hai mà vẫn gửi câu UPDATE")
	}
}

// --- (6) the unlock: the two rules of migration 0005 ----------------------------------------------

func TestMoKhoa_ThieuLyDoThiTuChoiTruocKhiMoGiaoDich(t *testing.T) {
	// DECISION (1) OF MIGRATION 0005, settled by this project on 2026-09-22 and not by the customer.
	// `chung_tu_giai_ngan_mo_khoa_du_vet` refuses the row underneath; this refuses the REQUEST, in
	// Vietnamese, before a lock is taken — so the accountant is told what to add rather than shown a
	// constraint name.
	for ten, lyDo := range map[string]string{"rỗng": "", "toàn khoảng trắng": "   \t "} {
		t.Run(ten, func(t *testing.T) {
			k := &khoCTGia{maDuAn: maDuAnMau, hang: hangDaKhoa(maLanhDao)}
			uc, ctx := dungUseCaseChungTu(t, k)

			_, err := uc.MoKhoa(ctx, idChungTu, lyDo, canBoCT(maKeToan))
			if !errors.Is(err, domain.ErrThieuLyDoMoKhoa) {
				t.Fatalf("lỗi = %v, muốn ErrThieuLyDoMoKhoa", err)
			}
			if k.batDau != 0 {
				t.Errorf("mở %d giao dịch cho một lần mở khoá không lý do, muốn 0", k.batDau)
			}
			if k.coCau("so_lan_mo_khoa + 1") {
				t.Error("không lý do mà vẫn gửi câu mở khoá")
			}
		})
	}
}

func TestMoKhoa_NguoiVuaKhoaKhongTuMoLaiDuoc(t *testing.T) {
	// DECISION (2) OF MIGRATION 0005. Nobody acts alone on the act that gives themselves room — the
	// same shape the customer already settled for the staff register in #13 and #14.
	//
	// THE CALLER HOLDS `budget.confirm` IN THIS CASE. What is refused is this PERSON against THIS
	// row, which is why it is not a 403: sending them to the Phân quyền screen would offer a
	// permission they already have.
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangDaKhoa(maLanhDao)}
	uc, ctx := dungUseCaseChungTu(t, k)

	_, err := uc.MoKhoa(ctx, idChungTu, "sai số tiền, phải nhập lại", canBoCT(maLanhDao))
	if !errors.Is(err, domain.ErrTuMoKhoaChungTuMinhVuaKhoa) {
		t.Fatalf("lỗi = %v, muốn ErrTuMoKhoaChungTuMinhVuaKhoa", err)
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit %d, rollback %d — muốn 0/1", k.daCommit, k.daRollback)
	}
	if k.coCau("so_lan_mo_khoa + 1") {
		t.Error("người vừa khoá tự mở mà vẫn gửi câu mở khoá")
	}
}

func TestMoKhoa_CanBoKhacMoDuoc_DemTangTrongSQL_VetCungGiaoDich(t *testing.T) {
	// DECISION (3): no ceiling, but every unlock is counted — and counted IN SQL rather than
	// read-modify-written, so the count survives the day somebody removes the row lock.
	//
	// ONE STATEMENT CARRIES THE STATE, THE REASON, THE PERSON, THE INSTANT AND THE COUNT. Two
	// statements would be two events and the second can fail: a voucher unlocked with no reason
	// recorded is the precise state decision (1) exists to prevent (0005:100-112).
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangDaKhoa(maLanhDao)}
	uc, ctx := dungUseCaseChungTu(t, k)

	const lyDo = "kho bạc trả lại chứng từ, phải nhập lại số tiền"
	sau, err := uc.MoKhoa(ctx, idChungTu, lyDo, canBoCT(maKeToan))
	if err != nil {
		t.Fatalf("MoKhoa lỗi: %v", err)
	}
	// BACK TO `Đã xác nhận`, EXACTLY — not chosen, derived: ChoKhoa only admits a lock from that
	// state, so it IS where the voucher was immediately before.
	if sau.TrangThai != domain.ChungTuDaXacNhan {
		t.Errorf("trạng thái sau khi mở = %q, muốn %q", sau.TrangThai, domain.ChungTuDaXacNhan)
	}
	if sau.SoLanMoKhoa != 1 {
		t.Errorf("so_lan_mo_khoa = %d, muốn 1", sau.SoLanMoKhoa)
	}

	mo := k.cau("so_lan_mo_khoa + 1")
	if len(mo) != 1 {
		t.Fatalf("có %d câu mở khoá, muốn 1", len(mo))
	}
	for _, cot := range []string{"trang_thai = $3", "nguoi_mo_khoa_id", "thoi_diem_mo_khoa", "ly_do_mo_khoa"} {
		if !strings.Contains(mo[0].sql, cot) {
			t.Errorf("câu mở khoá thiếu %q — vết mở khoá không được tách thành hai lần ghi:\n%s",
				cot, mo[0].sql)
		}
	}
	// The reason reaches the column, not only the ledger.
	coLyDo := false
	for _, a := range mo[0].args {
		if a == lyDo {
			coLyDo = true
		}
	}
	if !coLyDo {
		t.Errorf("lý do không đi vào câu mở khoá: %v", mo[0].args)
	}

	// SAME TRANSACTION, and the reason is in the entry as well as in the column: the column is
	// overwritten by the NEXT unlock, the ledger is append-only. Neither is derivable from the other.
	if k.batDau != 1 || k.daCommit != 1 {
		t.Fatalf("giao dịch: mở %d, commit %d — muốn 1/1", k.batDau, k.daCommit)
	}
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d dòng vết, muốn 1", len(vet))
	}
	if got := vet[0].args[4]; got != HanhViMoKhoaChungTu {
		t.Errorf("action = %v, muốn %q", got, HanhViMoKhoaChungTu)
	}
	delta, _ := vet[0].args[7].([]byte)
	if !strings.Contains(string(delta), lyDo) {
		t.Errorf("delta không mang lý do mở khoá:\n%s", delta)
	}
}

func TestMoKhoa_ChungTuChuaKhoaThiKhongCoGiDeMo(t *testing.T) {
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangOTrangThai(domain.ChungTuDaXacNhan)}
	uc, ctx := dungUseCaseChungTu(t, k)

	_, err := uc.MoKhoa(ctx, idChungTu, "gõ nhầm", canBoCT(maKeToan))
	if !errors.Is(err, domain.ErrChungTuChuaKhoa) {
		t.Fatalf("lỗi = %v, muốn ErrChungTuChuaKhoa", err)
	}
}

// --- (7) a voucher must belong to a live project of THIS commune ----------------------------------

func TestThemChungTu_DuAnKhongCoTrongXaThiTuChoi(t *testing.T) {
	// Money filed against a project id that matches nothing counts toward NO project's total while
	// sitting in the register looking healthy — the commune's own figures then disagree with the sum
	// of its own vouchers and no row looks wrong. There is no foreign key underneath (0004:182-190
	// says why), so this check is the only one there is.
	k := &khoCTGia{maDuAn: ""}
	uc, ctx := dungUseCaseChungTu(t, k)

	_, err := uc.Them(ctx, themChungTuMau(), canBoCT(maKeToan))
	if !errors.Is(err, fistore.ErrKhongThayDuAnCuaChungTu) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayDuAnCuaChungTu", err)
	}
	if k.coCau("INSERT INTO chung_tu_giai_ngan") {
		t.Error("dự án không có trong xã mà vẫn chèn chứng từ")
	}
	if k.daCommit != 0 {
		t.Errorf("commit %d, muốn 0", k.daCommit)
	}
}

// --- (8) a no-op writes nothing, which is what idem.KhongCan claims -------------------------------

func TestSuaChungTu_KhongCoTruongNaoDoiThiKhongGhiVaKhongVet(t *testing.T) {
	// Sending a voucher the figures it already has is not an event. Recording it would fill a public
	// authority's ledger with entries saying nothing changed, and those are the entries that bury
	// the ones carrying legal weight. It is also what makes PATCH's `idem.KhongCan` declaration true
	// rather than hopeful.
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangOTrangThai(domain.ChungTuKeToanNhap)}
	uc, ctx := dungUseCaseChungTu(t, k)

	noi := "Thanh toán đợt 3" // exactly what the row already holds
	if _, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{NoiDung: &noi}, canBoCT(maKeToan)); err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if k.coCau("UPDATE chung_tu_giai_ngan") {
		t.Error("không có gì đổi mà vẫn gửi câu UPDATE")
	}
	if k.coCau("INSERT INTO audit_log") {
		t.Error("không có gì đổi mà vẫn ghi vết")
	}
	if k.daCommit != 1 {
		t.Errorf("commit %d, muốn 1 — không ghi gì vẫn là một giao dịch kết thúc sạch", k.daCommit)
	}
}

// --- (9) no write without an actor ----------------------------------------------------------------

func TestMoiLoiGhiDeuDoiNguoiThucHien(t *testing.T) {
	// core/audit refuses an entry with no actor; refusing HERE keeps `nguoi_nhap_id` / `deleted_by` /
	// `nguoi_khoa_id` and the entry telling the same story, and avoids a rollback whose cause is a
	// missing principal rather than anything about the voucher.
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangDaKhoa(maLanhDao)}
	uc, ctx := dungUseCaseChungTu(t, k)

	if _, err := uc.Them(ctx, themChungTuMau(), audit.Actor{}); err == nil {
		t.Error("Them chạy mà không có người thực hiện")
	}
	if err := uc.Go(ctx, idChungTu, "lý do", audit.Actor{}); err == nil {
		t.Error("Go chạy mà không có người thực hiện")
	}
	if _, err := uc.Khoa(ctx, idChungTu, audit.Actor{}); err == nil {
		t.Error("Khoa chạy mà không có người thực hiện")
	}
	if _, err := uc.MoKhoa(ctx, idChungTu, "lý do", audit.Actor{}); err == nil {
		t.Error("MoKhoa chạy mà không có người thực hiện")
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch mà không có chủ thể, muốn 0", k.batDau)
	}
}

// --- sửa một chứng từ ĐÃ XÁC NHẬN thì nó về nháp ---------------------------------------------------
//
// The customer's rule, measured in `../vigov-require` commit `c3f4d6a` and taken up on 23/09/2026:
// *"lãnh đạo xác nhận những con số kia, không phải những con số này"*. It fills a gap open questions
// #29 and #30 left — they settled UNLOCKING and REFUNDS and say nothing about a confirmation whose
// figures moved — and contradicts neither.

func TestSuaChungTuDaXacNhanThiVeNhapVaXoaNguoiXacNhan(t *testing.T) {
	truoc := hangOTrangThai(domain.ChungTuDaXacNhan)
	truoc.nguoiXacNhan = maLanhDao
	k := &khoCTGia{maDuAn: maDuAnMau, hang: truoc}
	uc, ctx := dungUseCaseChungTu(t, k)

	so := domain.Dong(31_000_000)
	sau, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{SoTien: &so}, canBoCT(maKeToan))
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if sau.TrangThai != domain.ChungTuKeToanNhap {
		t.Fatalf("trạng thái sau khi sửa = %q, muốn %q", sau.TrangThai, domain.ChungTuKeToanNhap)
	}
	if sau.NguoiXacNhanID != "" {
		t.Fatalf("người xác nhận còn %q — dòng vẫn khai một lãnh đạo đã duyệt những con số họ "+
			"chưa từng thấy", sau.NguoiXacNhanID)
	}

	// TWO STATEMENTS, ONE TRANSACTION. The figures and the state move separately so that a reader of
	// the row can tell WHICH of the two happened — the same reason 0004:132-135 gives for the
	// trigger's column allowlist.
	ve := k.cau("trang_thai = 'ke-toan-nhap'")
	if len(ve) != 1 {
		t.Fatalf("có %d câu đưa về nháp, muốn 1", len(ve))
	}
	if !strings.Contains(ve[0].sql, "nguoi_xac_nhan_id = NULL") {
		t.Fatalf("câu về nháp không xoá người xác nhận: %s", ve[0].sql)
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("giao dịch: mở %d commit %d rollback %d, muốn 1/1/0",
			k.batDau, k.daCommit, k.daRollback)
	}

	// WHO HAD CONFIRMED IT IS IN THE ENTRY, and only there: the column has just been nulled, so the
	// append-only ledger is the one place that still answers "whose confirmation did this edit undo".
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d dòng vết, muốn 1", len(vet))
	}
	than := thanVet(t, vet[0])
	if !strings.Contains(than, "mat_xac_nhan") {
		t.Fatalf("vết không ghi việc mất xác nhận: %s", than)
	}
	if !strings.Contains(than, maLanhDao) {
		t.Fatalf("vết không ghi NGƯỜI đã xác nhận trước đó: %s", than)
	}
}

func TestSuaChungTuChuaXacNhanThiTrangThaiKhongDoi(t *testing.T) {
	// The other two states are untouched. A voucher already in `Kế toán nhập` must not pick up a
	// second, pointless state write — it would appear in the trail as a change that did not happen.
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangOTrangThai(domain.ChungTuKeToanNhap)}
	uc, ctx := dungUseCaseChungTu(t, k)

	so := domain.Dong(31_000_000)
	sau, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{SoTien: &so}, canBoCT(maKeToan))
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if sau.TrangThai != domain.ChungTuKeToanNhap {
		t.Fatalf("trạng thái = %q, muốn %q", sau.TrangThai, domain.ChungTuKeToanNhap)
	}
	if k.coCau("trang_thai = 'ke-toan-nhap'") {
		t.Fatal("chứng từ chưa xác nhận mà vẫn ghi lại trạng thái")
	}
}

func TestSuaChungTuDaKhoaVanTuChoiChuKhongVeNhap(t *testing.T) {
	// THE RULE ADDED THIS TURN MUST NOT BECOME A WAY AROUND THE LOCK. A locked voucher is refused
	// before any UPDATE is attempted — ChoSua first, and the `chung_tu_da_khoa` trigger underneath —
	// so the only path to a frozen figure is still the unlock route, with a reason, by somebody else.
	k := &khoCTGia{maDuAn: maDuAnMau, hang: hangDaKhoa(maLanhDao)}
	uc, ctx := dungUseCaseChungTu(t, k)

	so := domain.Dong(31_000_000)
	_, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{SoTien: &so}, canBoCT(maKeToan))
	if !errors.Is(err, domain.ErrChungTuDaKhoa) {
		t.Fatalf("lỗi = %v, muốn ErrChungTuDaKhoa", err)
	}
	if k.coCau("trang_thai = 'ke-toan-nhap'") {
		t.Fatal("chứng từ đã khoá mà vẫn bị đưa về nháp")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit %d rollback %d, muốn 0/1", k.daCommit, k.daRollback)
	}
}

func TestSuaChungTuDaXacNhanMaKhongDoiGiThiGiuNguyenXacNhan(t *testing.T) {
	// THE NO-OP BRANCH PROTECTS A LEADER'S ACT. `idem.KhongCan` on the PATCH route claims a repeat is
	// harmless; without this, a second identical request would strip a confirmation off a voucher
	// nobody edited.
	truoc := hangOTrangThai(domain.ChungTuDaXacNhan)
	truoc.nguoiXacNhan = maLanhDao
	k := &khoCTGia{maDuAn: maDuAnMau, hang: truoc}
	uc, ctx := dungUseCaseChungTu(t, k)

	noi := "Thanh toán đợt 3" // exactly what the row already holds
	sau, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{NoiDung: &noi}, canBoCT(maKeToan))
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if sau.TrangThai != domain.ChungTuDaXacNhan || sau.NguoiXacNhanID != maLanhDao {
		t.Fatalf("không đổi gì mà xác nhận bị gỡ: trạng thái %q, người %q",
			sau.TrangThai, sau.NguoiXacNhanID)
	}
	if k.coCau("trang_thai = 'ke-toan-nhap'") || k.coCau("INSERT INTO audit_log") {
		t.Fatal("không đổi gì mà vẫn ghi")
	}
}

// --- (10) the funding source of a payment (§8.2's `NGUỒN VỐN` column, migration 0007) --------------
//
// FOUR PROPERTIES, and the first is the one that fails in COMPLETE SILENCE:
//
//  1. an edit that moves ONLY the funding source is a real edit — it writes, it audits, and it sends
//     a confirmed voucher back to `Kế toán nhập` (ADR 0036);
//  2. "no source" reaches the column as NULL and never as '' — the two spellings are the difference
//     between §6's warning counting a voucher and losing it;
//  3. the source must be a LIVE source OF THIS COMMUNE, checked inside the transaction because there
//     is no foreign key underneath (0007:102-113);
//  4. the audit delta carries the move, because *"ai chuyển chứng từ này sang nguồn khác"* is a
//     question an inspection asks and the row can only ever answer where the money sits NOW.

const (
	idNguonVonCu  = "01JNGUONVONNGANSACHXA0000"
	idNguonVonMoi = "01JNGUONVONXAHOIHOA000000"
)

// khoCoNguonVon builds a store holding both sample sources of this commune.
func khoCoNguonVon(hang *hangCT) *khoCTGia {
	return &khoCTGia{
		maDuAn:     maDuAnMau,
		hang:       hang,
		nguonVonCo: map[string]bool{idNguonVonCu: true, idNguonVonMoi: true},
	}
}

func TestSuaChungTu_ChiDoiNguonVon_VanLaMotLanSuaThatSu(t *testing.T) {
	// ⚠ THE TRAP THIS TEST EXISTS FOR, AND IT IS INVISIBLE WITHOUT EXACTLY THIS CASE. `khongDoiChungTu`
	// compares the editable fields and Sua RETURNS EARLY when it reports "nothing moved". A field that
	// is editable but missing from that comparison is a field whose edit writes no row, leaves no audit
	// entry and does NOT send a confirmed voucher back to `Kế toán nhập` — while the request answers
	// 200 with the old values. Nothing is red anywhere.
	//
	// SO THE EDIT HERE TOUCHES THE FUNDING SOURCE AND NOTHING ELSE. Every other assertion about
	// `nguon_von_id` in this file changes a second field as well, and every one of them would stay
	// green with the comparison left unfixed.
	truoc := hangOTrangThai(domain.ChungTuDaXacNhan)
	truoc.nguonVonID = idNguonVonCu
	truoc.nguoiXacNhan = maLanhDao
	k := khoCoNguonVon(truoc)
	uc, ctx := dungUseCaseChungTu(t, k)

	moi := idNguonVonMoi
	sau, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{NguonVonID: &moi}, canBoCT(maKeToan))
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if sau.NguonVonID != idNguonVonMoi {
		t.Fatalf("nguồn vốn sau khi sửa = %q, muốn %q", sau.NguonVonID, idNguonVonMoi)
	}

	// (a) IT WRITES. A voucher that changed source and was never written is money still counted on the
	// old card of §6, with the screen showing the new one.
	capNhat := k.cau("UPDATE chung_tu_giai_ngan")
	if len(capNhat) == 0 {
		t.Fatal("đổi RIÊNG nguồn vốn mà không gửi câu UPDATE nào — `khongDoiChungTu` đang coi đây là " +
			"không đổi gì")
	}
	if got := capNhat[0].args[7]; got != idNguonVonMoi {
		t.Errorf("nguon_von_id vào câu cập nhật = %v, muốn %q", got, idNguonVonMoi)
	}

	// (b) IT COSTS THE CONFIRMATION (ADR 0036). Moving a payment to another funding source moves it
	// between two cards of §6 — the leader confirmed the figures on the old one.
	if sau.TrangThai != domain.ChungTuKeToanNhap || sau.NguoiXacNhanID != "" {
		t.Fatalf("trạng thái %q, người xác nhận %q — sửa một chứng từ đã xác nhận thì phải VỀ NHÁP",
			sau.TrangThai, sau.NguoiXacNhanID)
	}
	if len(k.cau("trang_thai = 'ke-toan-nhap'")) != 1 {
		t.Fatal("không có câu đưa về nháp")
	}

	// (c) IT LEAVES A TRAIL NAMING BOTH SOURCES, in the same transaction.
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("giao dịch: mở %d commit %d rollback %d, muốn 1/1/0",
			k.batDau, k.daCommit, k.daRollback)
	}
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d dòng vết, muốn 1", len(vet))
	}
	than := thanVet(t, vet[0])
	for _, muon := range []string{idNguonVonCu, idNguonVonMoi} {
		if !strings.Contains(than, muon) {
			t.Errorf("vết thiếu %q — không trả lời được *ai chuyển chứng từ này sang nguồn khác*:\n%s",
				muon, than)
		}
	}
}

func TestThemChungTu_ChuaGanNguonThiGhiNULLChuKhongPhaiChuoiRong(t *testing.T) {
	// §13 rule 6: a voucher with no funding source is a state the specification DEFINES and §6 REPORTS
	// ("Còn 3,4 tỷ đã chi nhưng chưa ghi rút từ nguồn nào"). The column carries
	// `CHECK (nguon_von_id IS NULL OR btrim(nguon_von_id) <> '')` (0007:274-277), so '' is refused
	// outright by the database — and if that CHECK were ever dropped, the voucher would fall out of the
	// warning while belonging to no source either: money missing from BOTH sides of one screen.
	for ten, gui := range map[string]string{"bỏ trống": "", "toàn khoảng trắng": "   "} {
		t.Run(ten, func(t *testing.T) {
			k := khoCoNguonVon(nil)
			uc, ctx := dungUseCaseChungTu(t, k)

			yc := themChungTuMau()
			yc.NguonVonID = gui
			if _, err := uc.Them(ctx, yc, canBoCT(maKeToan)); err != nil {
				t.Fatalf("Them lỗi: %v", err)
			}
			chen := k.cau("INSERT INTO chung_tu_giai_ngan")
			if len(chen) != 1 {
				t.Fatalf("có %d câu chèn, muốn 1", len(chen))
			}
			if got := chen[0].args[8]; got != nil {
				t.Fatalf("nguon_von_id vào câu chèn = %#v, muốn NULL", got)
			}
			// AND NOTHING WAS ASKED OF THE CATALOGUE. "No source" has nothing to verify; a lookup here
			// would turn the specification's own state into an error.
			if k.coCau("FROM nguon_von") {
				t.Error("không gắn nguồn mà vẫn đi hỏi bảng nguồn vốn")
			}
		})
	}
}

func TestSuaChungTu_GoKhoiNguon_GhiNULLVaKhongHoiDanhMuc(t *testing.T) {
	// `""` MEANS DETACH, and it is the middle of the three answers the pointer carries: nil leaves the
	// source alone, "" puts the voucher back into §6's "đã chi nhưng chưa ghi rút từ nguồn nào", an id
	// attaches it. A real correction — the accountant attributed a payment to the wrong source and the
	// right one is not yet known.
	truoc := hangOTrangThai(domain.ChungTuKeToanNhap)
	truoc.nguonVonID = idNguonVonCu
	k := khoCoNguonVon(truoc)
	uc, ctx := dungUseCaseChungTu(t, k)

	go_ := ""
	sau, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{NguonVonID: &go_}, canBoCT(maKeToan))
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if sau.NguonVonID != "" {
		t.Fatalf("nguồn vốn = %q, muốn rỗng", sau.NguonVonID)
	}
	capNhat := k.cau("UPDATE chung_tu_giai_ngan")
	if len(capNhat) != 1 {
		t.Fatalf("có %d câu cập nhật, muốn 1", len(capNhat))
	}
	if got := capNhat[0].args[7]; got != nil {
		t.Fatalf("nguon_von_id vào câu cập nhật = %#v, muốn NULL — '' bị CSDL từ chối, và nếu "+
			"CHECK bị gỡ thì chứng từ rơi khỏi cả cảnh báo §6 lẫn mọi thẻ nguồn", got)
	}
	// DETACHING VERIFIES NOTHING. There is no source to look up, and demanding one would make the
	// state §13 rule 6 defines unreachable from the edit path.
	if k.coCau("FROM nguon_von") {
		t.Error("gỡ khỏi nguồn mà vẫn đi hỏi bảng nguồn vốn")
	}
}

func TestThemChungTu_NguonVonKhongCoTrongXaThiTuChoiVaKhongGhiGi(t *testing.T) {
	// RULE 1, AND THERE IS NO FOREIGN KEY UNDERNEATH TO CATCH IT (0007:102-113). An id naming another
	// commune's source is indistinguishable from one that never existed, because the statement binds
	// tenant_id = $1 — and either way the money would sit on no card of §6 while ALSO being absent from
	// the "chưa ghi rút từ nguồn nào" warning, which counts NULL.
	k := &khoCTGia{maDuAn: maDuAnMau, nguonVonCo: map[string]bool{}}
	uc, ctx := dungUseCaseChungTu(t, k)

	yc := themChungTuMau()
	yc.NguonVonID = idNguonVonMoi
	_, err := uc.Them(ctx, yc, canBoCT(maKeToan))
	if !errors.Is(err, fistore.ErrKhongThayNguonVonCuaChungTu) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayNguonVonCuaChungTu", err)
	}
	if k.coCau("INSERT INTO chung_tu_giai_ngan") {
		t.Error("nguồn vốn không có trong xã mà vẫn chèn chứng từ")
	}
	if k.coCau("INSERT INTO audit_log") {
		t.Error("từ chối mà vẫn ghi vết")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit %d, rollback %d — muốn 0/1", k.daCommit, k.daRollback)
	}
}

func TestSuaChungTu_GanSangNguonKhongCoTrongXaThiTuChoi(t *testing.T) {
	// The same refusal on the edit path, and it is NOT redundant with the create one: the two call
	// sites are different branches, and the edit path is the one where the check is conditional.
	truoc := hangOTrangThai(domain.ChungTuKeToanNhap)
	truoc.nguonVonID = idNguonVonCu
	k := &khoCTGia{maDuAn: maDuAnMau, hang: truoc, nguonVonCo: map[string]bool{idNguonVonCu: true}}
	uc, ctx := dungUseCaseChungTu(t, k)

	lac := "01JNGUONVONCUAXAKHAC00000"
	_, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{NguonVonID: &lac}, canBoCT(maKeToan))
	if !errors.Is(err, fistore.ErrKhongThayNguonVonCuaChungTu) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayNguonVonCuaChungTu", err)
	}
	if k.coCau("UPDATE chung_tu_giai_ngan") {
		t.Error("nguồn vốn của xã khác mà vẫn gửi câu cập nhật")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit %d, rollback %d — muốn 0/1", k.daCommit, k.daRollback)
	}
}

func TestSuaChungTu_KhongDoiNguonThiKhongDoiHoiNguonConTonTai(t *testing.T) {
	// THE CHECK IS ABOUT THE MOVE, NOT ABOUT THE ROW. A voucher recorded last year against a source the
	// commune has since removed from its catalogue is a historical fact; refusing to correct its
	// DESCRIPTION because of that would strand the record with no way to fix a typo. What is refused is
	// attaching money to a source that is not there — which this edit does not do.
	truoc := hangOTrangThai(domain.ChungTuKeToanNhap)
	truoc.nguonVonID = idNguonVonCu
	k := &khoCTGia{maDuAn: maDuAnMau, hang: truoc, nguonVonCo: map[string]bool{}} // nguồn đã bị gỡ
	uc, ctx := dungUseCaseChungTu(t, k)

	noi := "Thanh toán đợt 4"
	sau, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{NoiDung: &noi}, canBoCT(maKeToan))
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if sau.NguonVonID != idNguonVonCu {
		t.Errorf("nguồn vốn = %q, muốn giữ nguyên %q", sau.NguonVonID, idNguonVonCu)
	}
	if k.coCau("FROM nguon_von") {
		t.Error("không đụng tới nguồn vốn mà vẫn đi hỏi bảng nguồn vốn")
	}
	if k.daCommit != 1 {
		t.Errorf("commit %d, muốn 1", k.daCommit)
	}
}

func TestSuaChungTu_GuiLaiDungNguonDangCoThiKhongGhiGi(t *testing.T) {
	// THE NO-OP BRANCH, WITH THE NEW FIELD. A screen that resends the whole form must not strip a
	// leader's confirmation off a voucher nobody changed — that is what `idem.KhongCan` on the PATCH
	// route claims, and this is the claim with `nguon_von_id` in the body.
	truoc := hangOTrangThai(domain.ChungTuDaXacNhan)
	truoc.nguonVonID = idNguonVonCu
	truoc.nguoiXacNhan = maLanhDao
	k := khoCoNguonVon(truoc)
	uc, ctx := dungUseCaseChungTu(t, k)

	nhu := idNguonVonCu // exactly what the row already holds
	sau, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{NguonVonID: &nhu}, canBoCT(maKeToan))
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if sau.TrangThai != domain.ChungTuDaXacNhan || sau.NguoiXacNhanID != maLanhDao {
		t.Fatalf("không đổi gì mà xác nhận bị gỡ: %q / %q", sau.TrangThai, sau.NguoiXacNhanID)
	}
	if k.coCau("UPDATE chung_tu_giai_ngan") || k.coCau("INSERT INTO audit_log") {
		t.Error("gửi lại đúng nguồn đang có mà vẫn ghi")
	}
}

func TestSuaChungTu_ChungTuDaKhoaThiKhongDoiDuocNguonVon(t *testing.T) {
	// 0007 ADDED `nguon_von_id` TO THE `chung_tu_da_khoa` TRIGGER'S FROZEN LIST (0007:329), because
	// without it the funding source would have been the ONLY business fact of a locked voucher anybody
	// could rewrite — moving money between two cards of §6 after the figure was signed off, leaving the
	// voucher itself looking untouched. This asserts the half that arrives FIRST: ChoSua refuses before
	// any UPDATE is sent, in Vietnamese, naming the way out.
	k := khoCoNguonVon(hangDaKhoa(maLanhDao))
	uc, ctx := dungUseCaseChungTu(t, k)

	moi := idNguonVonMoi
	_, err := uc.Sua(ctx, idChungTu, YeuCauSuaChungTu{NguonVonID: &moi}, canBoCT(maKeToan))
	if !errors.Is(err, domain.ErrChungTuDaKhoa) {
		t.Fatalf("lỗi = %v, muốn ErrChungTuDaKhoa", err)
	}
	if k.coCau("UPDATE chung_tu_giai_ngan") {
		t.Error("chứng từ đã khoá mà vẫn gửi câu cập nhật nguồn vốn")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit %d, rollback %d — muốn 0/1", k.daCommit, k.daRollback)
	}
}

// thanVet returns the JSON delta of one audit statement as text.
func thanVet(t *testing.T, l lenhGhi) string {
	t.Helper()
	for _, a := range l.args {
		if b, ok := a.([]byte); ok {
			return string(b)
		}
	}
	t.Fatalf("câu vết không mang delta: %v", l.args)
	return ""
}
