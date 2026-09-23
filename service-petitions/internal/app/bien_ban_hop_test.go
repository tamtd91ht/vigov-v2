package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for the three STAFF acts on the meeting-minutes register, over the REAL store on a fake
// driver.
//
//	PROVED HERE   the minutes, EVERY conclusion of the form and the audit entry share ONE
//	              transaction (rule 6, invariant 3) · every refusal commits NOTHING · the ordinal of
//	              an appended conclusion continues the HIGH-WATER MARK, read under the parent's lock
//	              (§7.2) · the audit subject is a readable business reference and the actor is the
//	              STAFF BUSINESS CODE (rule 6, invariant 8) · the ledger carries no minutes text ·
//	              the split DELEGATES to the task use case with the blurred pair filled in, and the
//	              body reaches it untouched · a conclusion of another meeting cannot be split.
//
//	NOT PROVED    anything PostgreSQL does with these statements — see the header of
//	              driver_gia_bien_ban_test.go. The unique key, the foreign key and the CHECK
//	              constraints of migration 0007 are the FLOOR under everything here.

// --- helpers --------------------------------------------------------------------------------------

func chiGhiTrongGiaoDichBB(t *testing.T, k *khoBienBanGia) {
	t.Helper()
	for _, l := range k.lenh {
		if !strings.HasPrefix(l.sql, "INSERT") && !strings.HasPrefix(l.sql, "UPDATE") {
			continue
		}
		if !l.trongGiaoDich {
			t.Errorf("câu ghi chạy NGOÀI giao dịch: %q", l.sql)
		}
	}
	if k.daCommit != 1 {
		t.Errorf("commit %d lần, muốn 1", k.daCommit)
	}
}

func khongGhiGiBB(t *testing.T, k *khoBienBanGia) {
	t.Helper()
	for _, l := range k.lenh {
		if strings.HasPrefix(l.sql, "INSERT") || strings.HasPrefix(l.sql, "UPDATE") {
			t.Errorf("đã ghi dù bị từ chối: %q", l.sql)
		}
	}
	if k.daCommit != 0 {
		t.Errorf("commit %d lần dù bị từ chối, muốn 0", k.daCommit)
	}
}

func vetKiemToanBB(t *testing.T, k *khoBienBanGia) lenhPhieu {
	t.Helper()
	l := k.cau("INSERT INTO audit_log")
	if len(l) != 1 {
		t.Fatalf("ghi %d vết kiểm toán, muốn 1", len(l))
	}
	return l[0]
}

func taoBienBanMau() YeuCauTaoBienBan {
	return YeuCauTaoBienBan{
		TenCuocHop: tenCuocHopBB,
		NgayHop:    mocNgayHopBB,
		SoHieu:     soHieuBBGoc,
		DiaDiem:    "Phòng họp UBND xã",
		ChuTriMa:   maChuTriBB,
		NoiDung:    "Toàn văn biên bản cuộc họp giao ban.",
		ThanhPhan:  []string{maChuTriBB, "Đại diện Mặt trận Tổ quốc xã"},
		KetLuan: []string{
			"Giao bộ phận Địa chính rà soát tiến độ tuyến đường, báo cáo trước ngày 20/8.",
			"Giao Văn hoá – Xã hội hoàn tất hồ sơ hỗ trợ sinh kế đợt 3.",
		},
	}
}

// --- 1. nhập biên bản (§4) --------------------------------------------------------------------------

// TestTaoBienBan_BienBanKetLuanVaVetTrongMotGiaoDich is the property rule 6, invariant 3 is about,
// plus the one §4 adds: a meeting whose minutes landed and whose conclusions did not is a card
// showing `0 kết luận` for a meeting that reached two of them.
func TestTaoBienBan_BienBanKetLuanVaVetTrongMotGiaoDich(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	bb, err := uc.TaoBienBan(ctx, taoBienBanMau(), canBoThu())
	if err != nil {
		t.Fatalf("nhập biên bản: %v", err)
	}

	for _, tu := range []string{"INSERT INTO bien_ban_hop", "INSERT INTO ket_luan_hop",
		"INSERT INTO audit_log"} {
		if !k.coCau(tu) {
			t.Errorf("thiếu câu %q", tu)
		}
	}
	if n := len(k.cau("INSERT INTO ket_luan_hop")); n != 2 {
		t.Errorf("ghi %d kết luận, muốn 2", n)
	}
	chiGhiTrongGiaoDichBB(t, k)

	// THE ORDINALS ARE ① ② IN THE ORDER TYPED. They are minted here and not read back: these are the
	// first conclusions of a meeting that did not exist a statement ago.
	if len(bb.KetLuan) != 2 || bb.KetLuan[0].ThuTu != 1 || bb.KetLuan[1].ThuTu != 2 {
		t.Fatalf("số thứ tự kết luận sai: %+v", bb.KetLuan)
	}
	for _, kl := range bb.KetLuan {
		if kl.BienBanID != bb.ID {
			t.Errorf("kết luận trỏ về biên bản %q, muốn %q", kl.BienBanID, bb.ID)
		}
	}
}

// TestTaoBienBan_KhongCoKetLuanVanLuuDuoc — §7.3: "Biên bản không có kết luận nào vẫn lưu được (nhập
// nháp trước, bổ sung sau)". The prototype's own draft is exactly this shape.
func TestTaoBienBan_KhongCoKetLuanVanLuuDuoc(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	yc := taoBienBanMau()
	yc.KetLuan = nil
	yc.SoHieu, yc.DiaDiem, yc.ChuTriMa, yc.NoiDung, yc.ThanhPhan = "", "", "", "", nil

	bb, err := uc.TaoBienBan(ctx, yc, canBoThu())
	if err != nil {
		t.Fatalf("biên bản nháp bị từ chối: %v", err)
	}
	if len(bb.KetLuan) != 0 {
		t.Errorf("biên bản nháp có %d kết luận", len(bb.KetLuan))
	}
	if k.coCau("INSERT INTO ket_luan_hop") {
		t.Error("ghi kết luận cho một biên bản không có kết luận nào")
	}
	chiGhiTrongGiaoDichBB(t, k)
}

// TestTaoBienBan_ThieuTenThiTuChoiVaKhongMoGiaoDich — a rejected form must hold no row lock on a
// government register, so the refusal happens BEFORE the transaction opens.
func TestTaoBienBan_ThieuTenThiTuChoiVaKhongMoGiaoDich(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	yc := taoBienBanMau()
	yc.TenCuocHop = "   "

	if _, err := uc.TaoBienBan(ctx, yc, canBoThu()); !errors.Is(err, domain.ErrThieuTenCuocHop) {
		t.Fatalf("lỗi = %v, muốn ErrThieuTenCuocHop", err)
	}
	khongGhiGiBB(t, k)
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch cho một biểu mẫu bị từ chối, muốn 0", k.batDau)
	}
}

// TestTaoBienBan_MotKetLuanRongThiKhongGhiKetLuanNao — every conclusion is checked before any is
// written. Validating inside the insert loop would file the first and refuse the second.
func TestTaoBienBan_MotKetLuanRongThiKhongGhiKetLuanNao(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	yc := taoBienBanMau()
	yc.KetLuan = []string{"Kết luận thứ nhất, hợp lệ.", "   "}

	if _, err := uc.TaoBienBan(ctx, yc, canBoThu()); !errors.Is(err, domain.ErrThieuNoiDungKetLuan) {
		t.Fatalf("lỗi = %v, muốn ErrThieuNoiDungKetLuan", err)
	}
	khongGhiGiBB(t, k)
}

// TestTaoBienBan_ThieuChuTheThiTuChoi — rule 6 does not permit a business write whose trail cannot
// name who made it, and refusing BEFORE the transaction opens keeps the cause readable.
func TestTaoBienBan_ThieuChuTheThiTuChoi(t *testing.T) {
	for _, ca := range []struct {
		ten   string
		nguoi audit.Actor
	}{
		{"không có mã cán bộ", audit.Actor{Kind: "staff", IP: "10.0.0.7"}},
		{"chủ thể là công dân", audit.Actor{ID: "cd-001", Kind: "citizen", IP: "10.0.0.7"}},
	} {
		t.Run(ca.ten, func(t *testing.T) {
			k := khoBBMau()
			uc, _, ctx := dungGhiBienBan(t, k)

			if _, err := uc.TaoBienBan(ctx, taoBienBanMau(), ca.nguoi); err == nil {
				t.Fatal("ghi được biên bản mà vết không gọi tên ai")
			}
			khongGhiGiBB(t, k)
		})
	}
}

// TestTaoBienBan_VetLaMaNghiepVuVaKhongMangNoiDung pins two things at once, and both were paid for
// elsewhere in this repository:
//
//	actor_id  the STAFF BUSINESS CODE, never the internal id (rule 6, invariant 8 — measured broken
//	          on six write paths on 2026-09-22)
//	delta     the audit ledger is append-only and never deleted, so the minutes' text would be
//	          permanent there. Only the LENGTH goes in (rule 3, forbidden #4).
func TestTaoBienBan_VetLaMaNghiepVuVaKhongMangNoiDung(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	yc := taoBienBanMau()
	if _, err := uc.TaoBienBan(ctx, yc, canBoThu()); err != nil {
		t.Fatalf("nhập biên bản: %v", err)
	}

	vet := vetKiemToanBB(t, k)
	if vet.args[1] != maCanBoThu {
		t.Errorf("chủ thể vết = %v, muốn mã cán bộ %q — một ULID ở cột này không gọi tên ai",
			vet.args[1], maCanBoThu)
	}
	if vet.args[4] != HanhViTaoBienBanHop {
		t.Errorf("hành vi = %v, muốn %q", vet.args[4], HanhViTaoBienBanHop)
	}
	// THE SUBJECT IS A READABLE BUSINESS REFERENCE: the meeting day and the number printed on the
	// document. A ULID here names nothing to the person reading the ledger years later.
	chuDe, ok := vet.args[5].(string)
	if !ok || chuDe != "bien-ban-hop/2026-08-05/31/BB-UBND" {
		t.Errorf("đối tượng vết = %v, muốn `bien-ban-hop/2026-08-05/31/BB-UBND`", vet.args[5])
	}

	delta := string(vet.args[7].([]byte))
	for _, cam := range []string{yc.NoiDung, yc.KetLuan[0], yc.KetLuan[1]} {
		if strings.Contains(delta, cam) {
			t.Errorf("delta mang nguyên văn nội dung — sổ vết không xoá được, và biên bản xã có "+
				"trích dẫn vụ việc: %s", delta)
		}
	}
	if !strings.Contains(delta, `"so_ket_luan":2`) {
		t.Errorf("delta không ghi số kết luận đã lập: %s", delta)
	}
}

// TestTaoBienBan_KhongCoSoHieuThiChuDeVanDocDuoc — §4 marks the reference number optional, so the
// subject must stay a complete reference rather than ending in an empty segment.
func TestTaoBienBan_KhongCoSoHieuThiChuDeVanDocDuoc(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	yc := taoBienBanMau()
	yc.SoHieu = ""

	if _, err := uc.TaoBienBan(ctx, yc, canBoThu()); err != nil {
		t.Fatalf("nhập biên bản: %v", err)
	}
	if chuDe := vetKiemToanBB(t, k).args[5]; chuDe != "bien-ban-hop/2026-08-05/khong-so" {
		t.Errorf("đối tượng vết = %v, muốn `bien-ban-hop/2026-08-05/khong-so`", chuDe)
	}
}

// TestTaoBienBan_NgayHopLaNgayLich — the column is DATE and the card renders `5/8/2026`. A time of
// day nobody recorded would decide which DAY a reader one time zone away sees.
func TestTaoBienBan_NgayHopLaNgayLich(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	yc := taoBienBanMau()
	yc.NgayHop = time.Date(2026, 8, 5, 14, 27, 3, 0, time.UTC)

	bb, err := uc.TaoBienBan(ctx, yc, canBoThu())
	if err != nil {
		t.Fatalf("nhập biên bản: %v", err)
	}
	if !bb.NgayHop.Equal(time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ngày họp = %v, muốn nửa đêm UTC ngày 05/08/2026", bb.NgayHop)
	}

	chen := k.cau("INSERT INTO bien_ban_hop")
	if len(chen) != 1 {
		t.Fatalf("ghi %d dòng biên bản, muốn 1", len(chen))
	}
	// $4 is `ngay_hop`. The value reaching the column is the one the domain normalised.
	if ngay, ok := chen[0].args[3].(time.Time); !ok || ngay.Hour() != 0 {
		t.Errorf("cột ngay_hop nhận %v — còn giữ giờ", chen[0].args[3])
	}
	// $8 is `thanh_phan`, cast `::jsonb` in the statement: a JSON array, never `null`.
	if tp, ok := chen[0].args[7].(string); !ok || !strings.HasPrefix(tp, "[") {
		t.Errorf("cột thanh_phan nhận %#v, muốn một mảng JSON", chen[0].args[7])
	}
}

// --- 2. thêm một kết luận ----------------------------------------------------------------------------

// TestThemKetLuan_NoiTiepSoLonNhatDaCap is §7.2, and the fixture is the case the rule exists for: ③
// is the highest number ever issued in this meeting, so the next one is ④ — WHATEVER the number of
// live conclusions is.
func TestThemKetLuan_NoiTiepSoLonNhatDaCap(t *testing.T) {
	k := khoBBMau()
	// Two conclusions were removed after ③ was issued. The live count is 1; the high-water mark is 5.
	k.thuTuLonNhat = 5
	uc, _, ctx := dungGhiBienBan(t, k)

	kl, err := uc.ThemKetLuan(ctx, idBBGoc, YeuCauThemKetLuan{
		NoiDung: "Giao Tài chính – Kế toán đối chiếu số liệu giải ngân sáu tháng đầu năm.",
	}, canBoThu())
	if err != nil {
		t.Fatalf("thêm kết luận: %v", err)
	}
	if kl.ThuTu != 6 {
		t.Errorf("số thứ tự = %d, muốn 6 — §7.2 nối tiếp số ĐÃ CẤP, không đếm dòng còn sống", kl.ThuTu)
	}
	if kl.BienBanID != idBBGoc {
		t.Errorf("kết luận trỏ về biên bản %q", kl.BienBanID)
	}
	chiGhiTrongGiaoDichBB(t, k)
}

// TestThemKetLuan_KhoaDongBienBanTruocKhiCapSo is what makes the numbering true under two clerks:
// the parent row is read `FOR UPDATE` before the high-water mark is read, so the two acts serialise
// on the one row they have in common.
func TestThemKetLuan_KhoaDongBienBanTruocKhiCapSo(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	if _, err := uc.ThemKetLuan(ctx, idBBGoc, YeuCauThemKetLuan{NoiDung: "Một kết luận mới."},
		canBoThu()); err != nil {
		t.Fatalf("thêm kết luận: %v", err)
	}

	khoa := k.cau("FOR UPDATE")
	if len(khoa) != 1 || !strings.Contains(khoa[0].sql, "bien_ban_hop") {
		t.Fatalf("không khoá dòng biên bản trước khi cấp số: %+v", khoa)
	}
	// ORDER MATTERS: the lock must be taken BEFORE the high-water mark is read, or two clerks both
	// read the old maximum and both mint the same ordinal.
	var iKhoa, iMax int = -1, -1
	for i, l := range k.lenh {
		if strings.Contains(l.sql, "FOR UPDATE") {
			iKhoa = i
		}
		if strings.Contains(l.sql, "MAX(") && iMax < 0 {
			iMax = i
		}
	}
	if iKhoa < 0 || iMax < 0 || iKhoa > iMax {
		t.Errorf("thứ tự câu lệnh sai: khoá ở %d, đọc số lớn nhất ở %d", iKhoa, iMax)
	}
}

// TestThemKetLuan_BienBanKhongCoThiTuChoiVaKhongGhi — a conclusion hung off minutes that do not
// exist in THIS commune is refused, and the refusal writes nothing.
func TestThemKetLuan_BienBanKhongCoThiTuChoiVaKhongGhi(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	_, err := uc.ThemKetLuan(ctx, "khong-co-bien-ban-nay", YeuCauThemKetLuan{NoiDung: "Kết luận."},
		canBoThu())
	if !errors.Is(err, petstore.ErrBienBanKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrBienBanKhongTonTai", err)
	}
	khongGhiGiBB(t, k)
}

func TestThemKetLuan_NoiDungRongThiTuChoi(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	_, err := uc.ThemKetLuan(ctx, idBBGoc, YeuCauThemKetLuan{NoiDung: "  "}, canBoThu())
	if !errors.Is(err, domain.ErrThieuNoiDungKetLuan) {
		t.Fatalf("lỗi = %v, muốn ErrThieuNoiDungKetLuan", err)
	}
	khongGhiGiBB(t, k)
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch cho một kết luận rỗng, muốn 0", k.batDau)
	}
}

// TestThemKetLuan_VetGoiTenBienBanVaSoThuTu — the entry has to say WHICH conclusion of WHICH minutes
// was added, in terms somebody holding the paper can follow.
func TestThemKetLuan_VetGoiTenBienBanVaSoThuTu(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	const noiDung = "Giao Tài chính – Kế toán đối chiếu số liệu giải ngân."
	if _, err := uc.ThemKetLuan(ctx, idBBGoc, YeuCauThemKetLuan{NoiDung: noiDung},
		canBoThu()); err != nil {
		t.Fatalf("thêm kết luận: %v", err)
	}

	vet := vetKiemToanBB(t, k)
	if vet.args[4] != HanhViThemKetLuan {
		t.Errorf("hành vi = %v, muốn %q", vet.args[4], HanhViThemKetLuan)
	}
	if vet.args[5] != "bien-ban-hop/2026-08-05/31/BB-UBND/ket-luan/4" {
		t.Errorf("đối tượng vết = %v", vet.args[5])
	}
	if delta := string(vet.args[7].([]byte)); strings.Contains(delta, noiDung) {
		t.Errorf("delta mang nguyên văn kết luận: %s", delta)
	}
}

// --- 3. tách kết luận thành nhiệm vụ (§3) --------------------------------------------------------------

func tachMau() YeuCauTaoNhiemVu {
	return YeuCauTaoNhiemVu{
		TuSinhMa:          true,
		Loai:              "theo-van-ban",
		TieuDe:            "Rà soát tiến độ tuyến đường Hà Lam – Bình Trị",
		MucUuTien:         "cao",
		LanhDaoGiaoViecMa: maLanhDao,
		HanXuLy:           mocHanNV,
	}
}

// TestTachKetLuan_GoiLaiUseCaseTaoNhiemVuKemCapMo is the whole point of the route: it does not write
// a task, it asks the task register to — with `nguon_giao` / `nguon_id` filled in from the
// conclusion it just read (§5's back-link, and the pair migration 0007 refused to duplicate with a
// column of its own).
func TestTachKetLuan_GoiLaiUseCaseTaoNhiemVuKemCapMo(t *testing.T) {
	k := khoBBMau()
	uc, nv, ctx := dungGhiBienBan(t, k)

	n, err := uc.TachKetLuanThanhNhiemVu(ctx, idBBGoc, int(thuTuKLGoc), tachMau(), canBoThu())
	if err != nil {
		t.Fatalf("tách kết luận: %v", err)
	}

	if nv.goi != 1 {
		t.Fatalf("gọi use case tạo nhiệm vụ %d lần, muốn 1", nv.goi)
	}
	if nv.yc.NguonGiao != string(domain.NguonKetLuanHop) {
		t.Errorf("nguồn giao = %q, muốn %q", nv.yc.NguonGiao, domain.NguonKetLuanHop)
	}
	if nv.yc.NguonID != idKLGoc {
		t.Errorf("nguồn id = %q, muốn id kết luận %q — không có cặp này thì §1 mất đường truy vết",
			nv.yc.NguonID, idKLGoc)
	}
	// THIS FILE WRITES NOTHING ITSELF. The task, its timeline row and its audit entry are the task
	// use case's, in ITS transaction — a second create path here would be a second place every rule
	// about a task lives.
	for _, l := range k.lenh {
		if strings.HasPrefix(l.sql, "INSERT") || strings.HasPrefix(l.sql, "UPDATE") {
			t.Errorf("tuyến tách tự ghi dữ liệu: %q", l.sql)
		}
	}
	if n.NguonID != idKLGoc {
		t.Errorf("nhiệm vụ trả về mang nguồn id %q", n.NguonID)
	}
}

// TestTachKetLuan_ThanYeuCauDiNguyenVenXuongUseCase — §3 says the confirmation box carries the whole
// "Giao việc mới" form, and NOTHING here may derive a title or a deadline out of the conclusion's
// sentence.
func TestTachKetLuan_ThanYeuCauDiNguyenVenXuongUseCase(t *testing.T) {
	k := khoBBMau()
	uc, nv, ctx := dungGhiBienBan(t, k)

	yc := tachMau()
	if _, err := uc.TachKetLuanThanhNhiemVu(ctx, idBBGoc, int(thuTuKLGoc), yc, canBoThu()); err != nil {
		t.Fatalf("tách kết luận: %v", err)
	}

	if nv.yc.TieuDe != yc.TieuDe {
		t.Errorf("tiêu đề = %q, muốn %q — tuyến này KHÔNG suy tiêu đề từ nội dung kết luận",
			nv.yc.TieuDe, yc.TieuDe)
	}
	if !nv.yc.HanXuLy.Equal(yc.HanXuLy) {
		t.Errorf("hạn xử lý = %v, muốn %v — hạn là thứ người dùng xác nhận, không phải thứ máy đoán "+
			"ra từ câu `báo cáo trước ngày 20/8`", nv.yc.HanXuLy, yc.HanXuLy)
	}
	if nv.yc.Loai != yc.Loai || nv.yc.MucUuTien != yc.MucUuTien ||
		nv.yc.LanhDaoGiaoViecMa != yc.LanhDaoGiaoViecMa || !nv.yc.TuSinhMa {
		t.Errorf("thân yêu cầu tới use case = %+v", nv.yc)
	}
	if nv.nguoi.ID != maCanBoThu {
		t.Errorf("chủ thể = %q, muốn mã cán bộ %q", nv.nguoi.ID, maCanBoThu)
	}
}

// TestTachKetLuan_KetLuanKhongThuocBienBanNayThiTuChoi — the conclusion is looked up BY THE PAIR.
// Every meeting has a ①, so an ordinal alone would let a task be born from another meeting's
// conclusion while the back-link looked perfectly valid.
func TestTachKetLuan_KetLuanKhongThuocBienBanNayThiTuChoi(t *testing.T) {
	k := khoBBMau()
	uc, nv, ctx := dungGhiBienBan(t, k)

	_, err := uc.TachKetLuanThanhNhiemVu(ctx, "bien-ban-khac", int(thuTuKLGoc), tachMau(), canBoThu())
	if !errors.Is(err, petstore.ErrKetLuanKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrKetLuanKhongTonTai", err)
	}
	if nv.goi != 0 {
		t.Error("đã tạo nhiệm vụ trỏ vào một kết luận không có thật — bộ đếm §2 sẽ đếm THIẾU, im lặng")
	}
}

// TestTachKetLuan_SoThuTuKhongCoThiTuChoi — a soft-deleted conclusion takes this answer too: its
// number stays taken for ever, but nothing new may be hung off it.
func TestTachKetLuan_SoThuTuKhongCoThiTuChoi(t *testing.T) {
	k := khoBBMau()
	uc, nv, ctx := dungGhiBienBan(t, k)

	_, err := uc.TachKetLuanThanhNhiemVu(ctx, idBBGoc, 99, tachMau(), canBoThu())
	if !errors.Is(err, petstore.ErrKetLuanKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrKetLuanKhongTonTai", err)
	}
	if nv.goi != 0 {
		t.Error("đã tạo nhiệm vụ cho một số thứ tự không tồn tại")
	}
}

// TestTachKetLuan_LoiCuaUseCaseNhiemVuDiThangRaNgoai keeps ONE set of answers for one act: a refusal
// from the task register must reach the handler unchanged, so the split route maps it exactly as
// POST /api/v1/tasks does.
func TestTachKetLuan_LoiCuaUseCaseNhiemVuDiThangRaNgoai(t *testing.T) {
	k := khoBBMau()
	uc, nv, ctx := dungGhiBienBan(t, k)
	nv.loi = petstore.ErrMaNhiemVuDaTonTai

	_, err := uc.TachKetLuanThanhNhiemVu(ctx, idBBGoc, int(thuTuKLGoc), tachMau(), canBoThu())
	if !errors.Is(err, petstore.ErrMaNhiemVuDaTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrMaNhiemVuDaTonTai đi nguyên vẹn ra ngoài", err)
	}
}
