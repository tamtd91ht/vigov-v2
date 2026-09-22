package app

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
)

// WHAT THIS FILE PROVES. These are the first business WRITE routes in the repository, so the
// invariants below are being asserted here for the first time and each of them fails SILENTLY:
//
//	rule 6, invariant 3   the business write and its audit entry share ONE transaction
//	rule 6, invariant 2   who · what · on which record · from which IP · in which commune
//	rule 7, invariant 1   a delete is a SOFT delete, with all three columns
//	rule 7, invariant 3   a code, once issued, is never reissued — the duplicate check counts
//	                      soft-deleted rows
//	ADR 0024              tier 2 and 3 cannot be deleted, tier 3 cannot be disabled
//	rule 1, invariant 4   the commune comes from the context, and every statement carries it
//
// The tier rules are ALSO enforced by a database trigger, which is the floor that holds against
// every writer. What is asserted here is that this service refuses FIRST, in a sentence somebody can
// act on, and — more importantly — that a refusal commits nothing.

func nguoiThu() audit.Actor {
	return audit.Actor{ID: "nd-01JINTERNALIDCUACANBO", Kind: "staff", IP: "10.0.0.7"}
}

func themMau() YeuCauThemLoaiTaiNguyen {
	return YeuCauThemLoaiTaiNguyen{Ma: "bao-cao", Nhan: "Báo cáo", ThuTu: 5}
}

// dongTang1 is a row the commune added itself — tier 1, the only tier that may be deleted.
func dongTang1() *hangLVB {
	return &hangLVB{id: "ltn-001", ma: "bao-cao", nhan: "Báo cáo", dangDung: true,
		thuTu: 5, nguon: domain.NguonDonVi}
}

// dongTang2 ships with the software: it may be disabled and relabelled, never deleted.
func dongTang2() *hangLVB {
	return &hangLVB{id: "ltn-002", ma: "cong-van", nhan: "Công văn", dangDung: true,
		thuTu: 1, nguon: domain.NguonHeThong}
}

// dongTang3 is a system row the SOURCE CODE BRANCHES ON. Relabelling is all that is left.
func dongTang3() *hangLVB {
	return &hangLVB{id: "ltn-003", ma: "quyet-dinh", nhan: "Quyết định", dangDung: true,
		thuTu: 2, nguon: domain.NguonHeThong, reNhanh: true}
}

// --- rule 6, invariant 3: ONE transaction --------------------------------------------------------

func TestThemGhiDongVaVetTrongCUNGMotGiaoDich(t *testing.T) {
	// THE INVARIANT THIS WHOLE ARCHITECTURE EXISTS FOR. The measured defect on the previous system
	// was zero transactions across the entire backend, so "every write leaves a trail" could not
	// hold: there was always a window where the record had changed and the trail had not.
	//
	// Asserted three ways, because each one alone can be satisfied by the wrong code:
	//   ONE BeginTx      — two would mean two transactions, which is the defect itself
	//   ONE Commit       — and no Rollback
	//   both statements  — the catalogue INSERT and the audit INSERT, inside that one transaction
	k := &khoGia{}
	uc, ctx := dungUseCase(t, k)

	if _, err := uc.Them(ctx, themMau(), nguoiThu()); err != nil {
		t.Fatalf("Them lỗi: %v", err)
	}

	if k.batDau != 1 {
		t.Errorf("mở %d giao dịch, muốn 1 — hai giao dịch là đúng khiếm khuyết luật 6 bất biến 3 cấm", k.batDau)
	}
	if k.daCommit != 1 || k.daRollback != 0 {
		t.Errorf("commit=%d rollback=%d, muốn 1/0", k.daCommit, k.daRollback)
	}
	if !k.coCau("INSERT INTO loai_tai_nguyen_ban_do") {
		t.Error("không có câu chèn dòng danh mục")
	}
	if !k.coCau("INSERT INTO audit_log") {
		t.Fatal("KHÔNG CÓ VẾT KIỂM TOÁN — một thay đổi không ai truy được là thứ luật 6 cấm")
	}
}

func TestThemVetMangDuNguoiViecVaMa(t *testing.T) {
	// Rule 6, invariant 2: who · what · on which record · when · from which IP · in which commune.
	// The subject is the BUSINESS code and never the internal ULID — a trail nobody can match to
	// the row in front of them answers nothing.
	k := &khoGia{}
	uc, ctx := dungUseCase(t, k)

	if _, err := uc.Them(ctx, themMau(), nguoiThu()); err != nil {
		t.Fatalf("Them lỗi: %v", err)
	}

	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("ghi %d vết, muốn 1", len(vet))
	}
	doi := chuoiTrongDoi(vet[0].args)
	for _, muon := range []string{string(xaA), "nd-01JINTERNALIDCUACANBO", "staff", "10.0.0.7",
		HanhViThemLoaiTaiNguyen, "bao-cao"} {
		if !doi[muon] {
			t.Errorf("vết thiếu %q — đối số: %v", muon, vet[0].args)
		}
	}
	// The internal id must NOT be the subject. It is allowed to appear nowhere in the entry.
	if doi["01JIDMOICUADONGVUATAO0000"] {
		t.Error("vết lấy mã nội bộ làm chủ thể thay vì mã nghiệp vụ")
	}
}

func TestThemKhongGhiGiKhiVetHong(t *testing.T) {
	// THE DIRECTION THAT MATTERS MOST. If the audit entry fails, the row must not exist either:
	// a catalogue row nobody can attribute is precisely the state the records rules do not permit.
	//
	// The failure is injected on the LAST statement, so everything before it has already run — which
	// is the only way to tell "rolled back" from "never started".
	k := &khoGia{loiSau: "INSERT INTO audit_log"}
	uc, ctx := dungUseCase(t, k)

	if _, err := uc.Them(ctx, themMau(), nguoiThu()); err == nil {
		t.Fatal("vết hỏng mà Them vẫn báo thành công")
	}
	if k.daCommit != 0 {
		t.Error("commit dù vết không ghi được")
	}
	if k.daRollback != 1 {
		t.Errorf("rollback=%d, muốn 1", k.daRollback)
	}
}

// --- ADR 0024: the three tiers -------------------------------------------------------------------

func TestXoaChiChoTang1(t *testing.T) {
	for ten, tc := range map[string]struct {
		hang *hangLVB
		loi  error
	}{
		"tầng 1 xoá được":       {dongTang1(), nil},
		"tầng 2 không xoá được": {dongTang2(), domain.ErrKhongXoaDuocMucHeThong},
		"tầng 3 không xoá được": {dongTang3(), domain.ErrKhongXoaDuocMucHeThong},
	} {
		t.Run(ten, func(t *testing.T) {
			k := &khoGia{hang: tc.hang}
			uc, ctx := dungUseCase(t, k)

			err := uc.Xoa(ctx, tc.hang.id, "gộp vào loại khác", nguoiThu())
			if tc.loi == nil {
				if err != nil {
					t.Fatalf("Xoá lỗi: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.loi) {
				t.Fatalf("lỗi = %v, muốn %v", err, tc.loi)
			}
			// A REFUSAL WRITES NOTHING. Not the row, and — just as important — not an audit entry
			// saying somebody deleted something they did not delete.
			if k.coCau("UPDATE loai_tai_nguyen_ban_do") {
				t.Error("từ chối rồi mà vẫn chạy câu cập nhật")
			}
			if k.coCau("INSERT INTO audit_log") {
				t.Error("từ chối rồi mà vẫn ghi vết — vết nói một việc chưa hề xảy ra")
			}
		})
	}
}

func TestTatChiBiTuChoiOTang3(t *testing.T) {
	// The operation the specification allows and the system cannot survive: disabling a code the
	// source code branches on leaves that branch with no reachable row, and the screen offering the
	// button reports nothing wrong (open question #21 describes the same shape for task statuses).
	sai := false
	for ten, tc := range map[string]struct {
		hang *hangLVB
		duoc bool
	}{
		"tầng 1 tắt được":       {dongTang1(), true},
		"tầng 2 tắt được":       {dongTang2(), true},
		"tầng 3 không tắt được": {dongTang3(), false},
	} {
		t.Run(ten, func(t *testing.T) {
			k := &khoGia{hang: tc.hang}
			uc, ctx := dungUseCase(t, k)

			_, err := uc.Sua(ctx, tc.hang.id, YeuCauSuaLoaiTaiNguyen{DangDung: &sai}, nguoiThu())
			if tc.duoc {
				if err != nil {
					t.Fatalf("tắt lỗi: %v", err)
				}
				if !k.coCau("UPDATE loai_tai_nguyen_ban_do") || !k.coCau("INSERT INTO audit_log") {
					t.Error("tắt thành công mà thiếu câu cập nhật hoặc vết")
				}
				return
			}
			if !errors.Is(err, domain.ErrKhongTatDuocMucReNhanh) {
				t.Fatalf("lỗi = %v, muốn ErrKhongTatDuocMucReNhanh", err)
			}
			if k.coCau("UPDATE loai_tai_nguyen_ban_do") || k.coCau("INSERT INTO audit_log") {
				t.Error("từ chối rồi mà vẫn ghi")
			}
		})
	}
}

func TestSuaNhanDuocOMoiTang(t *testing.T) {
	// RELABELLING IS THE ONE OPERATION ALL THREE TIERS ALLOW, and at tier 3 it is the only one left.
	// The wording on a screen belongs to the commune; the code does not. A guard that refused this
	// would leave a commune unable to correct a misspelt Vietnamese label on its own screens.
	nhan := "Quyết định (sửa)"
	for ten, hang := range map[string]*hangLVB{
		"tầng 1": dongTang1(), "tầng 2": dongTang2(), "tầng 3": dongTang3(),
	} {
		t.Run(ten, func(t *testing.T) {
			k := &khoGia{hang: hang}
			uc, ctx := dungUseCase(t, k)

			sau, err := uc.Sua(ctx, hang.id, YeuCauSuaLoaiTaiNguyen{Nhan: &nhan}, nguoiThu())
			if err != nil {
				t.Fatalf("đổi nhãn lỗi: %v", err)
			}
			if sau.Nhan != nhan {
				t.Errorf("nhãn sau = %q, muốn %q", sau.Nhan, nhan)
			}
			if !k.coCau("INSERT INTO audit_log") {
				t.Error("đổi nhãn mà không để lại vết")
			}
		})
	}
}

// --- what the commune may never supply -----------------------------------------------------------

func TestChenKhongCoThamSoNaoChoNguon(t *testing.T) {
	// THE SECURITY PROPERTY, ASSERTED ON THE STATEMENT ITSELF. `nguon` and `ma_nguon_re_nhanh`
	// decide which tier a row is in, and the migration says what a writable `nguon` would cost:
	// "every guard below could be stepped around by setting nguon = 'don-vi' first".
	//
	// So the INSERT must carry them as LITERALS and bind neither. A `$8` appearing here would mean
	// a value travelled from somewhere — and the only somewhere above this line is a request.
	k := &khoGia{}
	uc, ctx := dungUseCase(t, k)

	if _, err := uc.Them(ctx, themMau(), nguoiThu()); err != nil {
		t.Fatalf("Them lỗi: %v", err)
	}
	chen := k.cau("INSERT INTO loai_tai_nguyen_ban_do")
	if len(chen) != 1 {
		t.Fatalf("chạy %d câu chèn, muốn 1", len(chen))
	}
	if !strings.Contains(chen[0].sql, "'don-vi'") {
		t.Errorf("`nguon` không phải hằng trong câu lệnh: %q", chen[0].sql)
	}
	// Seven bound parameters: tenant_id, id, ma, nhan, thu_tu, la_mac_dinh, dang_dung. Nothing for
	// `nguon`, nothing for `ma_nguon_re_nhanh`.
	if len(chen[0].args) != 7 {
		t.Errorf("câu chèn nhận %d tham số, muốn 7 — thêm một tham số là thêm một đường cho client",
			len(chen[0].args))
	}
	for _, a := range chen[0].args {
		if s, ok := a.(string); ok && (s == domain.NguonHeThong || s == domain.NguonDonVi) {
			t.Errorf("`nguon` đi vào câu lệnh như một THAM SỐ: %v", chen[0].args)
		}
	}
}

func TestCapNhatKhongDungToiMaVaNguon(t *testing.T) {
	// `ma` because an issued code is never renumbered (rule 7, invariant 3) — document records hold
	// it as a value and nothing rewrites them. `nguon` and `ma_nguon_re_nhanh` because they decide
	// the tier. All three are refused by the trigger too; their ABSENCE here is what makes that
	// refusal unreachable from this service in the first place.
	nhan := "Công văn mới"
	k := &khoGia{hang: dongTang2()}
	uc, ctx := dungUseCase(t, k)

	if _, err := uc.Sua(ctx, "ltn-002", YeuCauSuaLoaiTaiNguyen{Nhan: &nhan}, nguoiThu()); err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	capNhat := k.cau("UPDATE loai_tai_nguyen_ban_do")
	if len(capNhat) != 1 {
		t.Fatalf("chạy %d câu cập nhật, muốn 1", len(capNhat))
	}
	for _, cot := range []string{"ma =", "nguon =", "ma_nguon_re_nhanh ="} {
		if strings.Contains(capNhat[0].sql, cot) {
			t.Errorf("câu cập nhật đụng tới `%s`: %q", strings.TrimSuffix(cot, " ="), capNhat[0].sql)
		}
	}
}

// --- rule 7: soft delete, and a code that stays taken ---------------------------------------------

func TestXoaLaXoaMemVaGhiDuBaCot(t *testing.T) {
	// Rule 7, invariant 1 names THREE columns, and the reason all three are asserted is that a row
	// which vanished from every screen with no reason attached is a row nobody can explain when
	// somebody asks why a document type disappeared — and the row is still there, so the question
	// WILL be asked.
	k := &khoGia{hang: dongTang1()}
	uc, ctx := dungUseCase(t, k)

	if err := uc.Xoa(ctx, "ltn-001", "gộp vào loại khác", nguoiThu()); err != nil {
		t.Fatalf("Xoá lỗi: %v", err)
	}
	// NOT A DELETE. `DELETE FROM` anywhere on this path is rule 7, forbidden #1, and the trigger
	// refuses it — but the statement must never be written in the first place.
	if k.coCau("DELETE FROM") {
		t.Fatal("XOÁ CỨNG trên dữ liệu nghiệp vụ — luật 7 cấm #1")
	}
	xoa := k.cau("deleted_at = now()")
	if len(xoa) != 1 {
		t.Fatalf("chạy %d câu xoá mềm, muốn 1", len(xoa))
	}
	for _, cot := range []string{"deleted_at", "deleted_by", "delete_reason"} {
		if !strings.Contains(xoa[0].sql, cot) {
			t.Errorf("câu xoá mềm thiếu cột `%s`: %q", cot, xoa[0].sql)
		}
	}
	doi := chuoiTrongDoi(xoa[0].args)
	if !doi["gộp vào loại khác"] || !doi["nd-01JINTERNALIDCUACANBO"] {
		t.Errorf("xoá mềm không ghi ai xoá và vì sao: %v", xoa[0].args)
	}
}

func TestMoiDuongDocGhiDeuLoaiDongDaXoaMem(t *testing.T) {
	// RULE 7, INVARIANT 2 — "EVERYWHERE, ALWAYS", and the write path is where "everywhere" is
	// easiest to forget, because a soft-deleted row is invisible on the screen so nobody tries.
	//
	// Two statements, two different consequences of dropping the predicate:
	//
	//	the FOR UPDATE read  — an edit would RESURRECT a deleted row: it would come back on every
	//	                       screen, under a code that is still recorded as withdrawn;
	//	the soft delete      — a second delete would OVERWRITE `deleted_by` and `delete_reason`,
	//	                       which is editing a historical record (rule 7, forbidden #5). The
	//	                       first deletion is the one that happened.
	//
	// Asserted on the SQL because the fake driver has no notion of a deleted row: what can be
	// checked without a PostgreSQL is that the statement ASKS for it, and that is the half that gets
	// deleted while tidying a query.
	k := &khoGia{hang: dongTang1()}
	uc, ctx := dungUseCase(t, k)

	if err := uc.Xoa(ctx, "ltn-001", "gộp vào loại khác", nguoiThu()); err != nil {
		t.Fatalf("Xoá lỗi: %v", err)
	}
	doc := k.cau("FOR UPDATE")
	if len(doc) != 1 {
		t.Fatalf("chạy %d câu đọc-để-sửa, muốn 1", len(doc))
	}
	if !strings.Contains(doc[0].sql, "deleted_at IS NULL") {
		t.Errorf("câu đọc-để-sửa KHÔNG loại dòng đã xoá — sửa một dòng đã xoá sẽ làm nó sống lại: %q",
			doc[0].sql)
	}
	xoa := k.cau("deleted_at = now()")
	if !strings.Contains(xoa[0].sql, "deleted_at IS NULL") {
		t.Errorf("câu xoá mềm KHÔNG loại dòng đã xoá — lần xoá thứ hai ghi đè người xoá và lý do: %q",
			xoa[0].sql)
	}
	// The same predicate on the UPDATE that edits a row, for the first of the two reasons above.
	nhan := "Báo cáo mới"
	k2 := &khoGia{hang: dongTang1()}
	uc2, ctx2 := dungUseCase(t, k2)
	if _, err := uc2.Sua(ctx2, "ltn-001", YeuCauSuaLoaiTaiNguyen{Nhan: &nhan}, nguoiThu()); err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	for _, l := range k2.cau("UPDATE loai_tai_nguyen_ban_do") {
		if !strings.Contains(l.sql, "deleted_at IS NULL") {
			t.Errorf("câu ghi không loại dòng đã xoá: %q", l.sql)
		}
	}
}

func TestSuaDongDaXoaLa404(t *testing.T) {
	// The other half of the same rule, from the caller's side: a row the FOR UPDATE read does not
	// find is "not there", and every path answers that the same way. `hang: nil` is how the fake
	// says the predicate excluded it.
	nhan := "Báo cáo mới"
	k := &khoGia{hang: nil}
	uc, ctx := dungUseCase(t, k)

	if _, err := uc.Sua(ctx, "ltn-001", YeuCauSuaLoaiTaiNguyen{Nhan: &nhan}, nguoiThu()); !errors.Is(err, docstore.ErrDanhMucKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrDanhMucKhongTonTai", err)
	}
	if k.coCau("UPDATE loai_tai_nguyen_ban_do") || k.coCau("INSERT INTO audit_log") {
		t.Error("không tìm thấy dòng mà vẫn ghi")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit=%d rollback=%d, muốn 0/1", k.daCommit, k.daRollback)
	}
}

func TestXoaDoiLyDo(t *testing.T) {
	// A soft delete with no reason is refused BEFORE the transaction opens. `delete_reason` is not
	// decoration: it is the only thing that will ever answer "why is this type gone".
	k := &khoGia{hang: dongTang1()}
	uc, ctx := dungUseCase(t, k)

	if err := uc.Xoa(ctx, "ltn-001", "   ", nguoiThu()); !errors.Is(err, domain.ErrThieuLyDoXoa) {
		t.Fatalf("lỗi = %v, muốn ErrThieuLyDoXoa", err)
	}
	if k.batDau != 0 {
		t.Error("mở giao dịch cho một yêu cầu đã sai hình dạng — giữ khoá dòng mà không cần")
	}
}

func TestThemTuChoiMaDaCoKeCaDongDaXoa(t *testing.T) {
	// THE DUPLICATE CHECK COUNTS SOFT-DELETED ROWS, and that single predicate decides whether old
	// documents can still be read: a commune that could soft-delete `cong-van` and create a new,
	// unrelated `cong-van` would silently change the type shown on every document already
	// registered under the old one — and numbering follows the type (ADR 0024).
	k := &khoGia{maTrung: 1}
	uc, ctx := dungUseCase(t, k)

	_, err := uc.Them(ctx, themMau(), nguoiThu())
	if !errors.Is(err, docstore.ErrMaDaTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrMaDaTonTai", err)
	}
	if k.coCau("INSERT INTO") {
		t.Error("mã trùng mà vẫn chèn")
	}
	// The statement that asked must NOT exclude deleted rows. Asserted on the SQL because this is
	// exactly the predicate somebody "fixes" while tidying up a query.
	hoi := k.cau("count(*)")
	var kiem string
	for _, l := range hoi {
		if strings.Contains(l.sql, "ma = $2") {
			kiem = l.sql
		}
	}
	if kiem == "" {
		t.Fatal("không có câu kiểm mã trùng")
	}
	if strings.Contains(kiem, "deleted_at") {
		t.Errorf("câu kiểm mã trùng LOẠI dòng đã xoá — mã đã cấp sẽ được cấp lại: %q", kiem)
	}
}

func TestThemTuChoiKhiDayTran(t *testing.T) {
	// A CREATE HAS TO CARE ABOUT THE READ ROUTE'S CEILING. DanhSach REFUSES rather than truncates
	// past TranDanhMucLoaiTaiNguyen, so the row that crosses the line does not merely add itself — it
	// turns the whole catalogue into a 500 for every screen in the commune.
	k := &khoGia{dem: docstore.TranDanhMucLoaiTaiNguyen}
	uc, ctx := dungUseCase(t, k)

	_, err := uc.Them(ctx, themMau(), nguoiThu())
	if !errors.Is(err, docstore.ErrDanhMucDayTran) {
		t.Fatalf("lỗi = %v, muốn ErrDanhMucDayTran", err)
	}
	if k.coCau("INSERT INTO") {
		t.Error("đầy trần mà vẫn chèn")
	}

	// One row below the ceiling is accepted. An off-by-one here refuses a commune whose data is
	// perfectly valid.
	k2 := &khoGia{dem: docstore.TranDanhMucLoaiTaiNguyen - 1}
	uc2, ctx2 := dungUseCase(t, k2)
	if _, err := uc2.Them(ctx2, themMau(), nguoiThu()); err != nil {
		t.Fatalf("dưới trần mà bị từ chối: %v", err)
	}
}

// --- idempotent edit -------------------------------------------------------------------------------

func TestSuaKhongDoiGiThiKhongGhiVaKhongVet(t *testing.T) {
	// THE PROPERTY THE ROUTE'S idem.KhongCan DECLARATION RESTS ON. Sending the label a row already
	// has is not an event; recording it would fill a public authority's ledger with entries saying
	// nothing changed, and those are the entries that bury the ones carrying legal weight.
	//
	// Delete the comparison in Sua and this goes red — which is the point, because the route would
	// then be claiming an idempotency it no longer has.
	nhan := "Báo cáo" // exactly what dongTang1() already holds
	k := &khoGia{hang: dongTang1()}
	uc, ctx := dungUseCase(t, k)

	sau, err := uc.Sua(ctx, "ltn-001", YeuCauSuaLoaiTaiNguyen{Nhan: &nhan}, nguoiThu())
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if sau.Nhan != "Báo cáo" {
		t.Errorf("trả về sai: %+v", sau)
	}
	if k.coCau("UPDATE loai_tai_nguyen_ban_do") {
		t.Error("không có gì đổi mà vẫn chạy câu cập nhật")
	}
	if k.coCau("INSERT INTO audit_log") {
		t.Error("không có gì đổi mà vẫn ghi vết — vết nói một việc chưa hề xảy ra")
	}
}

func TestSuaDatMacDinhBoMacDinhCuTruocTrongCungGiaoDich(t *testing.T) {
	// `UNIQUE (tenant_id, moc_mac_dinh)` admits exactly ONE live default, so setting a new one
	// without clearing the old one fails the constraint. Doing it in a SECOND transaction would
	// leave a window where the commune has no default at all and every form pre-selects nothing.
	dung := true
	k := &khoGia{hang: dongTang1()}
	uc, ctx := dungUseCase(t, k)

	if _, err := uc.Sua(ctx, "ltn-001", YeuCauSuaLoaiTaiNguyen{LaMacDinh: &dung}, nguoiThu()); err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if !k.coCau("la_mac_dinh = false") {
		t.Fatal("đặt mặc định mới mà không bỏ mặc định cũ — khoá duy nhất sẽ từ chối")
	}
	if k.batDau != 1 {
		t.Errorf("mở %d giao dịch, muốn 1", k.batDau)
	}
}

// --- rule 1: the commune is in every statement, and comes from the context -------------------------

func TestMoiCauLenhMangXaTuContext(t *testing.T) {
	// Rule 1, invariants 4 and 5. The commune is not a parameter of any method on this use case and
	// cannot be: it arrives in the context, Scoped binds it to $1, and a store built without one
	// does not exist. Asserted on EVERY statement rather than on one, because the leak is whichever
	// statement forgot.
	for _, xa := range []string{string(xaA), string(xaB)} {
		k := &khoGia{}
		uc, _ := dungUseCase(t, k)
		ctx := tenant.Into(context.Background(), tenant.ID(xa))
		if _, err := uc.Them(ctx, themMau(), nguoiThu()); err != nil {
			t.Fatalf("Them lỗi: %v", err)
		}
		for _, l := range k.lenh {
			if len(l.args) == 0 || l.args[0] != xa {
				t.Errorf("câu lệnh %q chạy với $1 = %v, muốn xã %q", l.sql, l.args, xa)
			}
		}
	}
}

func TestKhongCoXaTrongContextThiPanic(t *testing.T) {
	// FAIL CLOSED, LOUDLY. A write that ran without a commune would file a catalogue row — and its
	// audit entry — in nobody's archive, or in everybody's. tenant.MustFrom panics by design and
	// httpx.Recover turns that into a traceable 500 at the edge. What must never happen is a
	// default commune (rule 1, forbidden #1).
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("ghi danh mục khi context không có xã mà không panic")
		}
	}()
	k := &khoGia{}
	uc, _ := dungUseCase(t, k)
	_, _ = uc.Them(context.Background(), themMau(), nguoiThu())
}

// --- helpers ----------------------------------------------------------------------------------------

// chuoiTrongDoi indexes a statement's arguments by their string form, so an assertion names the
// VALUE it expects instead of a position that shifts whenever a column is added.
func chuoiTrongDoi(args []driver.Value) map[string]bool {
	ra := map[string]bool{}
	for _, a := range args {
		switch v := a.(type) {
		case string:
			ra[v] = true
		case []byte:
			// The audit delta arrives as JSON bytes. Indexed whole so a test can look for a field
			// name inside it without re-parsing.
			ra[string(v)] = true
		}
	}
	return ra
}
