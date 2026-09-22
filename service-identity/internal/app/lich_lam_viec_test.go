package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: the write surface of the commune's working calendar, proved against a REAL
// transaction (driver_gia_lich_test.go) rather than against a fake store.
//
// THE FIVE PROPERTIES THAT ARE EXPENSIVE TO LOSE, and every one of them fails SILENTLY:
//
//  1. seeding twice writes no duplicate AND overwrites no hour the commune edited — asserted on the
//     STATEMENTS (zero UPDATE, zero ON CONFLICT), because an UPSERT satisfies the first half and
//     destroys the second while every count still looks right;
//  2. a session that overlaps another is REFUSED — the obligation migration 0006:109 hands the write
//     path in writing, because the database cannot hold it;
//  3. a swap day on a weekday that already works is REFUSED (ADR 0007 decision 9), and a date that
//     is both a closure and a working day is refused from BOTH sides;
//  4. the audit entry shares the transaction with the change, and its actor is the STAFF CODE;
//  5. nothing on any path touches a deadline that has already been issued (rule 10, invariant 2).

// --- (1) the seed: first run ----------------------------------------------------------------------

func TestGieoTuanLanDauChenDuMuoiCa(t *testing.T) {
	k := &khoLichGia{}
	uc, ctx := dungUseCaseLich(t, k)

	kq, err := uc.GieoTuanMacDinh(ctx, nguoiLich())
	if err != nil {
		t.Fatalf("GieoTuanMacDinh lỗi: %v", err)
	}
	if kq.DaGieo != 10 || kq.DaCo != 0 || kq.BoQua != 0 {
		t.Fatalf("kết quả = %+v, muốn {DaGieo:10 DaCo:0 BoQua:0}", kq)
	}
	if n := k.soCau("INSERT INTO lich_lam_viec"); n != 10 {
		t.Fatalf("số câu INSERT = %d, muốn 10", n)
	}
	// MON–FRI ONLY, AND NOTHING FOR SATURDAY OR SUNDAY. A commune that does not work a day has NO
	// ROW for it (migration 0006:88) — not a flag and not a zero-length session — so a seed that
	// wrote seven weekdays would be writing two days the commune does not work.
	thay := map[int]int{}
	for _, l := range k.cau("INSERT INTO lich_lam_viec") {
		thu, ok := l.args[2].(int64)
		if !ok {
			t.Fatalf("tham số thứ không phải số: %#v", l.args[2])
		}
		thay[int(thu)]++
	}
	for thu := 1; thu <= 5; thu++ {
		if thay[thu] != 2 {
			t.Errorf("%s có %d ca, muốn 2 (sáng và chiều)", domain.TenThuISO(thu), thay[thu])
		}
	}
	if thay[6] != 0 || thay[7] != 0 {
		t.Errorf("gieo cả thứ Bảy/Chủ nhật (%d/%d ca) — xã không làm việc hai ngày đó thì KHÔNG có dòng nào",
			thay[6], thay[7])
	}
}

// THE LUNCH BREAK IS A GAP BETWEEN TWO ROWS, NOT A FIELD. This is the assertion that the seed's ten
// rows really express it: the morning ends at 11:30 and the afternoon starts at 13:30, so two hours
// of the day are covered by no session at all and the deadline function skips them without being
// told to. Over a 40-hour deadline a mishandled lunch break is a full working day of drift.
//
// MUTATION THAT MUST TURN THIS RED: make GioDongSang equal GioMoChieu.
func TestGieoTuanCoNghiTruaLaKhoangHoGiuaHaiCa(t *testing.T) {
	sang, chieu := 0, 0
	for _, g := range domain.BoGieoCaLamViec() {
		if g.Thu != 1 {
			continue
		}
		if g.BatDau == domain.GioMoSang {
			sang = int(g.KetThuc)
		}
		if g.BatDau == domain.GioMoChieu {
			chieu = int(g.BatDau)
		}
	}
	if sang == 0 || chieu == 0 {
		t.Fatal("bộ gieo thiếu ca sáng hoặc ca chiều của thứ Hai")
	}
	if chieu <= sang {
		t.Fatalf("ca chiều bắt đầu lúc %d giây, ca sáng kết thúc lúc %d — không có khoảng nghỉ trưa",
			chieu, sang)
	}
}

// --- (1) the seed: second run. THE LOAD-BEARING CASE ------------------------------------------------

// PRESSING THE BUTTON TWICE MUST WRITE NOTHING AND OVERWRITE NOTHING, and the assertion is on the
// STATEMENTS rather than on the counts. The obvious implementation — an UPSERT — would satisfy
// "no duplicate row" and silently destroy "no overwritten hour": a commune that shortened Monday
// afternoon to 16:00 would get 17:00 back, with nothing reporting that it moved and the number on
// the screen one nobody chose.
//
// MUTATION THAT MUST TURN THIS RED: give the store an `ON CONFLICT … DO UPDATE`, or call
// tuan.CapNhat from GieoTuanMacDinh.
func TestGieoTuanLanHaiKhongTaoBanTrungVaKhongGhiDeGioXaDaSua(t *testing.T) {
	k := &khoLichGia{caTuan: tuanDayDu()}
	uc, ctx := dungUseCaseLich(t, k)

	kq, err := uc.GieoTuanMacDinh(ctx, nguoiLich())
	if err != nil {
		t.Fatalf("GieoTuanMacDinh lỗi: %v", err)
	}
	if kq.DaGieo != 0 || kq.DaCo != 10 {
		t.Fatalf("kết quả = %+v, muốn {DaGieo:0 DaCo:10}", kq)
	}
	if n := k.soCau("INSERT INTO lich_lam_viec"); n != 0 {
		t.Errorf("lần gieo thứ hai chèn %d dòng — phải là 0", n)
	}
	// THE TWO ASSERTIONS THE TASK CALLS LOAD-BEARING, made on the statements themselves.
	if n := k.soCau("UPDATE lich_lam_viec"); n != 0 {
		t.Errorf("lần gieo thứ hai phát %d câu UPDATE — bộ gieo KHÔNG được ghi đè con số xã đã sửa", n)
	}
	if k.coCau("ON CONFLICT") {
		t.Error("có ON CONFLICT trong câu lệnh — UPSERT làm 'đã có sẵn' và 'vừa ghi' không phân biệt được, " +
			"và ghi đè đúng thứ bộ gieo phải giữ nguyên")
	}
	// NOTHING IS AUDITED EITHER: an entry recording that a button did nothing buries the entries
	// that carry legal weight.
	if k.coCau("INSERT INTO audit_log") {
		t.Error("lần gieo thứ hai ghi vết dù không đổi gì")
	}
}

// A COMMUNE WITH SOME ROWS GETS ONLY THE MISSING ONES, and the ones it has are not read for
// comparison — they are skipped whole.
func TestGieoTuanChiBuNhungCaConThieu(t *testing.T) {
	// The commune configured Monday only.
	k := &khoLichGia{caTuan: []hangCaTuan{
		{id: "llv-t2-sang", thu: 1, batDau: int(domain.GioMoSang), ketThuc: int(domain.GioDongSang)},
		{id: "llv-t2-chieu", thu: 1, batDau: int(domain.GioMoChieu), ketThuc: gioXaDaSuaLich},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	kq, err := uc.GieoTuanMacDinh(ctx, nguoiLich())
	if err != nil {
		t.Fatalf("GieoTuanMacDinh lỗi: %v", err)
	}
	if kq.DaGieo != 8 || kq.DaCo != 2 || kq.BoQua != 0 {
		t.Fatalf("kết quả = %+v, muốn {DaGieo:8 DaCo:2 BoQua:0}", kq)
	}
	for _, l := range k.cau("INSERT INTO lich_lam_viec") {
		if thu, _ := l.args[2].(int64); thu == 1 {
			t.Error("gieo lại ca của thứ Hai — xã đã có hai ca ấy")
		}
	}
	if n := k.soCau("UPDATE lich_lam_viec"); n != 0 {
		t.Errorf("phát %d câu UPDATE — hai ca thứ Hai phải được giữ NGUYÊN, kể cả giờ đã sửa", n)
	}
}

// THE SEED MUST NEVER BE THE THING THAT BREAKS THE CALENDAR. A commune that has already entered
// Monday 08:00–12:00 would, under a naive "insert what is missing", also get Monday 07:30–11:30 —
// two overlapping sessions, which makes EVERY deadline uncomputable (LoiCaChongNhau) and leaves the
// commune worse off than before they pressed the button.
//
// MUTATION THAT MUST TURN THIS RED: drop the domain.KhongChongCaNao call from GieoTuanMacDinh.
func TestGieoTuanBoQuaCaSeChongVoiCaXaDaCo(t *testing.T) {
	k := &khoLichGia{caTuan: []hangCaTuan{
		{id: "llv-t2-rieng", thu: 1, batDau: 8 * 3600, ketThuc: 12 * 3600, ghiChu: "Giờ riêng của xã"},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	kq, err := uc.GieoTuanMacDinh(ctx, nguoiLich())
	if err != nil {
		t.Fatalf("GieoTuanMacDinh lỗi: %v", err)
	}
	if kq.BoQua != 1 {
		t.Fatalf("kết quả = %+v, muốn BoQua = 1 (ca sáng thứ Hai của bộ gieo chồng giờ 08:00–12:00)", kq)
	}
	if kq.DaGieo != 9 {
		t.Errorf("DaGieo = %d, muốn 9 — tám ca của thứ Ba→thứ Sáu cộng ca chiều thứ Hai", kq.DaGieo)
	}
	for _, l := range k.cau("INSERT INTO lich_lam_viec") {
		thu, _ := l.args[2].(int64)
		gio, _ := l.args[3].(int64)
		if thu == 1 && gio == 7 {
			t.Error("gieo ca sáng 07:30 thứ Hai đè lên giờ 08:00–12:00 xã đã khai — hai ca chồng nhau " +
				"làm mọi hạn xử lý không tính được")
		}
	}
}

// --- (1) the seed: one transaction, one trail --------------------------------------------------------

// THE READ THAT DECIDES AND THE WRITES IT DECIDES ON ARE IN ONE TRANSACTION. A read outside it is a
// check with a gap in the middle through which a concurrent seeding run inserts the same rows.
func TestGieoTuanDocLichTrongCungGiaoDichVoiCacCauChen(t *testing.T) {
	k := &khoLichGia{}
	uc, ctx := dungUseCaseLich(t, k)

	if _, err := uc.GieoTuanMacDinh(ctx, nguoiLich()); err != nil {
		t.Fatalf("GieoTuanMacDinh lỗi: %v", err)
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("giao dịch: mở %d, commit %d, rollback %d — muốn 1/1/0",
			k.batDau, k.daCommit, k.daRollback)
	}
	// The SELECT comes first, and it is the one inside the transaction.
	viTriDoc, viTriChen := -1, -1
	for i, l := range k.lenh {
		if viTriDoc < 0 && strings.Contains(l.sql, "SELECT") && strings.Contains(l.sql, "FROM lich_lam_viec") {
			viTriDoc = i
		}
		if viTriChen < 0 && strings.Contains(l.sql, "INSERT INTO lich_lam_viec") {
			viTriChen = i
		}
	}
	if viTriDoc < 0 || viTriChen < 0 || viTriDoc > viTriChen {
		t.Fatalf("thứ tự câu lệnh sai: đọc ở %d, chèn ở %d — quyết định gieo phải ĐỌC trước, trong cùng giao dịch",
			viTriDoc, viTriChen)
	}
}

// RULE 6, INVARIANT 8: the trail names the STAFF CODE, never the internal id. The two are
// indistinguishable on sight, which is why this is asserted rather than assumed.
//
// MUTATION THAT MUST TURN THIS RED: build the audit.Actor from nguoi.ID in the handler, or fall back
// to it in NguoiThucHien.hopLe.
func TestGieoTuanGhiVetCungGiaoDichVaChuTheLaMaCanBo(t *testing.T) {
	k := &khoLichGia{}
	uc, ctx := dungUseCaseLich(t, k)

	if _, err := uc.GieoTuanMacDinh(ctx, nguoiLich()); err != nil {
		t.Fatalf("GieoTuanMacDinh lỗi: %v", err)
	}
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("số vết = %d, muốn ĐÚNG MỘT cho cả lượt gieo", len(vet))
	}
	if got := vet[0].args[1]; got != maCanBoLich {
		t.Errorf("chủ thể vết = %v, muốn MÃ CÁN BỘ %q (luật 6 bất biến 8)", got, maCanBoLich)
	}
	if got := vet[0].args[4]; got != HanhViGieoLichTuan {
		t.Errorf("hành vi = %v, muốn %q", got, HanhViGieoLichTuan)
	}
	if got := vet[0].args[5]; got != chuDeGieoLichTuan {
		t.Errorf("chủ đề = %v, muốn %q", got, chuDeGieoLichTuan)
	}
	// The delta carries the counts and the rows, so an inspection can see WHICH hours this commune
	// started from without the source of the release deployed that day.
	var delta map[string]any
	if err := json.Unmarshal(vet[0].args[7].([]byte), &delta); err != nil {
		t.Fatalf("delta không phải JSON: %v", err)
	}
	if delta["da_gieo"] != float64(10) {
		t.Errorf("delta.da_gieo = %v, muốn 10", delta["da_gieo"])
	}
}

// A FAILURE ON THE LAST STATEMENT OF THE TRANSACTION TAKES EVERY INSERT DOWN WITH IT. This is the
// shape rule 6, invariant 3 exists for: the change must not be able to succeed while the trail
// fails.
func TestGieoTuanHongVetThiKhongCaNaoDuocGhi(t *testing.T) {
	k := &khoLichGia{loiSau: "INSERT INTO audit_log"}
	uc, ctx := dungUseCaseLich(t, k)

	if _, err := uc.GieoTuanMacDinh(ctx, nguoiLich()); err == nil {
		t.Fatal("vết hỏng mà GieoTuanMacDinh vẫn báo thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit %d, rollback %d — muốn 0/1: vết hỏng thì không dòng nào được commit",
			k.daCommit, k.daRollback)
	}
}

func TestGieoTuanTuChoiKhiThieuMaCanBo(t *testing.T) {
	k := &khoLichGia{}
	uc, ctx := dungUseCaseLich(t, k)

	nguoi := nguoiLich()
	nguoi.Vet.ID = "" // identity older than the `ma` field
	if _, err := uc.GieoTuanMacDinh(ctx, nguoi); err == nil {
		t.Fatal("thiếu mã cán bộ mà vẫn gieo — vết sẽ không nói được ai làm")
	}
	if k.batDau != 0 {
		t.Errorf("đã mở %d giao dịch dù người thực hiện không hợp lệ — phải từ chối TRƯỚC", k.batDau)
	}
}

// --- (2) the overlap refusal the database cannot make ------------------------------------------------

// MIGRATION 0006:109 HANDS THIS OBLIGATION TO THE WRITE PATH IN WRITING. Two sessions of 07:30–11:30
// and 09:00–12:00 pass every constraint in the schema and double-count two hours, which makes the
// deadline come out EARLIER than the commune's real hours — the direction that reports an authority
// late when it was not.
func TestThemCaTuChoiCaChongGioVaKhongGhiGi(t *testing.T) {
	k := &khoLichGia{caTuan: []hangCaTuan{
		{id: idCaThu, thu: 1, batDau: 7*3600 + 30*60, ketThuc: 11*3600 + 30*60, ghiChu: "Buổi sáng"},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	_, err := uc.ThemCa(ctx, YeuCauThemCa{Thu: 1, BatDau: 9 * 3600, KetThuc: 12 * 3600}, nguoiLich())
	var chong *domain.LoiCaChongCaKhac
	if !asLoiChongCa(err, &chong) {
		t.Fatalf("lỗi = %v, muốn *domain.LoiCaChongCaKhac", err)
	}
	if chong.CaID != idCaThu {
		t.Errorf("lỗi nêu ca %q, muốn ca ĐÃ CÓ %q — người sửa cấu hình phải biết nhìn vào dòng nào",
			chong.CaID, idCaThu)
	}
	if k.coCau("INSERT INTO lich_lam_viec") || k.coCau("INSERT INTO audit_log") {
		t.Error("bị từ chối mà vẫn ghi")
	}
	if k.daCommit != 0 {
		t.Errorf("commit %d lần dù bị từ chối", k.daCommit)
	}
}

// TOUCHING IS NOT OVERLAPPING. A morning ending at 11:30 and an afternoon starting at 11:30 are the
// ordinary shape of a working day with no lunch break; refusing them would make the commonest
// configuration in the country unenterable.
func TestThemCaChapNhanCaChamNhauODungMotDiem(t *testing.T) {
	k := &khoLichGia{caTuan: []hangCaTuan{
		{id: idCaThu, thu: 1, batDau: 7*3600 + 30*60, ketThuc: 11*3600 + 30*60},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	if _, err := uc.ThemCa(ctx, YeuCauThemCa{
		Thu: 1, BatDau: 11*3600 + 30*60, KetThuc: 17 * 3600,
	}, nguoiLich()); err != nil {
		t.Fatalf("ca chạm nhau ở đúng 11:30 bị từ chối: %v", err)
	}
	if n := k.soCau("INSERT INTO lich_lam_viec"); n != 1 {
		t.Errorf("số câu chèn = %d, muốn 1", n)
	}
}

// A SESSION ON ANOTHER WEEKDAY IS NOT AN OVERLAP. The group is the weekday, and a check that ignored
// it would report every Monday morning as clashing with every Tuesday morning.
func TestThemCaKhacThuThiKhongTinhLaChongNhau(t *testing.T) {
	k := &khoLichGia{caTuan: []hangCaTuan{
		{id: idCaThu, thu: 1, batDau: 7*3600 + 30*60, ketThuc: 11*3600 + 30*60},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	if _, err := uc.ThemCa(ctx, YeuCauThemCa{
		Thu: 2, BatDau: 7*3600 + 30*60, KetThuc: 11*3600 + 30*60,
	}, nguoiLich()); err != nil {
		t.Fatalf("ca cùng giờ nhưng KHÁC THỨ bị từ chối: %v", err)
	}
}

// --- (1)+(4) the edit and the soft delete ------------------------------------------------------------

func TestSuaCaKhongDoiGiThiKhongGhiVaKhongCoVet(t *testing.T) {
	k := &khoLichGia{caTuan: []hangCaTuan{
		{id: idCaThu, thu: 1, batDau: 7*3600 + 30*60, ketThuc: 11*3600 + 30*60, ghiChu: "Buổi sáng"},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	ghiChu := "Buổi sáng"
	if _, err := uc.SuaCa(ctx, idCaThu, YeuCauSuaCa{GhiChu: &ghiChu}, nguoiLich()); err != nil {
		t.Fatalf("SuaCa lỗi: %v", err)
	}
	if k.coCau("UPDATE lich_lam_viec") {
		t.Error("gửi đúng giá trị đang có mà vẫn phát UPDATE")
	}
	if k.coCau("INSERT INTO audit_log") {
		t.Error("không có gì đổi mà vẫn ghi vết — những dòng ấy chôn mất các vết có giá trị pháp lý")
	}
}

// THE EDIT READS THE ROW `FOR UPDATE`, INSIDE THE TRANSACTION. Without the lock two administrators
// editing one session both read the old hours and the second write silently discards the first.
func TestSuaCaDocDongBangFORUPDATETrongGiaoDich(t *testing.T) {
	k := &khoLichGia{caTuan: []hangCaTuan{
		{id: idCaThu, thu: 1, batDau: 7*3600 + 30*60, ketThuc: 11*3600 + 30*60},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	moi := domain.GioTrongNgay(12 * 3600)
	if _, err := uc.SuaCa(ctx, idCaThu, YeuCauSuaCa{KetThuc: &moi}, nguoiLich()); err != nil {
		t.Fatalf("SuaCa lỗi: %v", err)
	}
	if !k.coCau("FOR UPDATE") {
		t.Error("không có FOR UPDATE — hai người sửa cùng một ca thì lần ghi sau âm thầm xoá lần trước")
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Errorf("giao dịch: mở %d, commit %d — muốn 1/1", k.batDau, k.daCommit)
	}
}

// XOÁ LÀ MỘT UPDATE, KHÔNG BAO GIỜ LÀ DELETE (rule 7, forbidden #1), and `deleted_by` carries the
// STAFF CODE for the same reason `actor_id` does: it is read years later by somebody handling an
// inspection.
func TestXoaCaLaXoaMemKemLyDoVaMaCanBo(t *testing.T) {
	k := &khoLichGia{caTuan: []hangCaTuan{
		{id: idCaThu, thu: 1, batDau: 7*3600 + 30*60, ketThuc: 11*3600 + 30*60},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	const lyDo = "Xã chuyển sang khung giờ mới theo thông báo của UBND huyện"
	if err := uc.XoaCa(ctx, idCaThu, lyDo, nguoiLich()); err != nil {
		t.Fatalf("XoaCa lỗi: %v", err)
	}
	if k.coCau("DELETE FROM") {
		t.Fatal("có DELETE FROM — dữ liệu nghiệp vụ chỉ được xoá MỀM (luật 7 cấm #1)")
	}
	xoa := k.cau("deleted_at = now()")
	if len(xoa) != 1 {
		t.Fatalf("số câu xoá mềm = %d, muốn 1", len(xoa))
	}
	if got := xoa[0].args[2]; got != maCanBoLich {
		t.Errorf("deleted_by = %v, muốn MÃ CÁN BỘ %q, không phải id nội bộ", got, maCanBoLich)
	}
	if got := xoa[0].args[3]; got != lyDo {
		t.Errorf("delete_reason = %v, muốn lý do đã gửi", got)
	}
	if len(k.cau("INSERT INTO audit_log")) != 1 {
		t.Error("xoá mềm mà không ghi vết")
	}
}

func TestXoaCaTuChoiKhiThieuLyDo(t *testing.T) {
	k := &khoLichGia{caTuan: []hangCaTuan{{id: idCaThu, thu: 1, batDau: 7 * 3600, ketThuc: 11 * 3600}}}
	uc, ctx := dungUseCaseLich(t, k)

	if err := uc.XoaCa(ctx, idCaThu, "   ", nguoiLich()); err == nil {
		t.Fatal("xoá không có lý do mà vẫn được — luật 7 bất biến 1 đòi delete_reason")
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch dù thiếu lý do — phải từ chối TRƯỚC", k.batDau)
	}
}

// --- (3) the two cross-table refusals ----------------------------------------------------------------

// ADR 0007 DECISION 9. A swap day on a date whose weekday already has sessions reads two opposite
// ways — the swap day REPLACES those hours, or it ADDS to them — they differ by exactly the overlap,
// and nothing decides between them. The system refuses rather than picking a winner.
//
// 2026-02-21 IS A SATURDAY. The fixture gives the commune a Saturday duty session, so the date's
// weekday already works.
func TestThemLamBuTuChoiKhiThuDoVonDaCoCaLamViec(t *testing.T) {
	k := &khoLichGia{caTuan: []hangCaTuan{
		{id: "llv-t7", thu: 6, batDau: 7*3600 + 30*60, ketThuc: 11*3600 + 30*60, ghiChu: "Ca trực thứ Bảy"},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	_, err := uc.ThemLamBu(ctx, YeuCauThemLamBu{
		Ngay: "2026-02-21", BatDau: 7*3600 + 30*60, KetThuc: 11*3600 + 30*60,
		Ten: "Làm bù nghỉ Tết",
	}, nguoiLich())
	if err == nil {
		t.Fatal("thêm được ngày làm bù vào thứ vốn đã có ca — ADR 0007 quyết định 9 đòi TỪ CHỐI")
	}
	var khongTinh *domain.LoiKhongTinhDuocHan
	if !asLoiKhongTinhDuoc(err, &khongTinh) {
		t.Fatalf("lỗi = %v, muốn *domain.LoiKhongTinhDuocHan", err)
	}
	if khongTinh.Loai != domain.LoiLamBuTrungNgayDaLam {
		t.Errorf("loại lỗi = %q, muốn %q — CÙNG một từ vựng với đường tính hạn, không phải từ vựng thứ hai",
			khongTinh.Loai, domain.LoiLamBuTrungNgayDaLam)
	}
	if k.coCau("INSERT INTO ngay_lam_bu") {
		t.Error("bị từ chối mà vẫn chèn")
	}
}

// A DATE THAT IS BOTH A CLOSURE AND A WORKING DAY IS REFUSED FROM BOTH SIDES. Refusing on one side
// only would leave the other screen looking healthy and the commune would fix nothing.
func TestThemLamBuTuChoiKhiNgayDoDaLaNgayNghiLe(t *testing.T) {
	k := &khoLichGia{nghiLe: []hangNghiLe{
		{id: idNghiLeThu, ngay: "2026-02-21", ten: "Nghỉ Tết"},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	_, err := uc.ThemLamBu(ctx, YeuCauThemLamBu{
		Ngay: "2026-02-21", BatDau: 7 * 3600, KetThuc: 11 * 3600, Ten: "Làm bù nghỉ Tết",
	}, nguoiLich())
	var xungDot *domain.LoiNgayVuaNghiVuaLamBu
	if !asLoiXungDot(err, &xungDot) {
		t.Fatalf("lỗi = %v, muốn *domain.LoiNgayVuaNghiVuaLamBu", err)
	}
	if len(xungDot.Ngay) != 1 || xungDot.Ngay[0] != "2026-02-21" {
		t.Errorf("lỗi nêu %v, muốn đúng ngày đang ghi", xungDot.Ngay)
	}
}

func TestThemNgayNghiTuChoiKhiNgayDoDaLaNgayLamBu(t *testing.T) {
	k := &khoLichGia{lamBu: []hangLamBu{
		{id: idLamBuThu, ngay: "2026-02-21", batDau: 7 * 3600, ketThuc: 11 * 3600, ten: "Làm bù nghỉ Tết"},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	_, err := uc.ThemNgayNghi(ctx, YeuCauThemNgayNghi{Ngay: "2026-02-21", Ten: "Nghỉ Tết"}, nguoiLich())
	var xungDot *domain.LoiNgayVuaNghiVuaLamBu
	if !asLoiXungDot(err, &xungDot) {
		t.Fatalf("lỗi = %v, muốn *domain.LoiNgayVuaNghiVuaLamBu — hai tuyến phải từ chối như nhau", err)
	}
	if k.coCau("INSERT INTO ngay_nghi_le") {
		t.Error("bị từ chối mà vẫn chèn")
	}
}

// A SOFT-DELETED SWAP DAY IS NOT A SWAP DAY (rule 7, invariant 2). Without this the commune could
// never clear a conflict: removing the offending row would leave the refusal in place forever.
func TestThemNgayNghiChapNhanKhiNgayLamBuDaBiXoaMem(t *testing.T) {
	k := &khoLichGia{lamBu: []hangLamBu{
		{id: idLamBuThu, ngay: "2026-02-21", batDau: 7 * 3600, ketThuc: 11 * 3600,
			ten: "Làm bù nghỉ Tết", daXoa: true},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	if _, err := uc.ThemNgayNghi(ctx,
		YeuCauThemNgayNghi{Ngay: "2026-02-21", Ten: "Nghỉ Tết"}, nguoiLich()); err != nil {
		t.Fatalf("ngày làm bù đã xoá mềm vẫn chặn được ngày nghỉ lễ: %v", err)
	}
}

// --- (1) the holiday seed ------------------------------------------------------------------------------

// FOUR FIXED SOLAR DATES, AND THE LUNAR ONES DELIBERATELY ABSENT. This is the case that guards the
// most consequential absence in the whole turn: a hand-written lunar conversion that is one day out
// is a deadline counted through a day the office was shut.
//
// MUTATION THAT MUST TURN THIS RED: add a Tết row to domain.BoGieoNgayNghiLe.
func TestGieoNgayNghiLeChiGieoNgayCoDinhTheoDuongLich(t *testing.T) {
	k := &khoLichGia{}
	uc, ctx := dungUseCaseLich(t, k)

	kq, err := uc.GieoNgayNghiLeMacDinh(ctx, 2026, nguoiLich())
	if err != nil {
		t.Fatalf("GieoNgayNghiLeMacDinh lỗi: %v", err)
	}
	if kq.DaGieo != 4 {
		t.Fatalf("DaGieo = %d, muốn 4 — 01/01, 30/04, 01/05, 02/09", kq.DaGieo)
	}

	muon := map[string]bool{"2026-01-01": true, "2026-04-30": true, "2026-05-01": true, "2026-09-02": true}
	thay := map[string]bool{}
	for _, l := range k.cau("INSERT INTO ngay_nghi_le") {
		ngay, _ := l.args[2].(string)
		thay[ngay] = true
	}
	for ngay := range muon {
		if !thay[ngay] {
			t.Errorf("thiếu ngày lễ dương lịch %s", ngay)
		}
	}
	for ngay := range thay {
		if !muon[ngay] {
			t.Errorf("gieo ngày %s — bộ gieo CHỈ được có ngày lễ CỐ ĐỊNH theo dương lịch. "+
				"Tết Nguyên đán và Giỗ Tổ Hùng Vương theo ÂM LỊCH: một phép quy đổi tự viết sai một "+
				"ngày là sai một cam kết với dân, nên xã tự nhập", ngay)
		}
	}
	// 01/9 AND 03/9 ARE NOT SEEDED EITHER: Bộ luật Lao động 2019 điều 112 khoản 3 gives the choice
	// between them to the Prime Minister, each year.
	if thay["2026-09-01"] || thay["2026-09-03"] {
		t.Error("gieo ngày liền kề 02/9 — Thủ tướng chọn giữa 01/9 và 03/9 TỪNG NĂM, không đoán được")
	}
}

func TestGieoNgayNghiLeLanHaiKhongTaoBanTrungVaKhongGhiDe(t *testing.T) {
	k := &khoLichGia{nghiLe: []hangNghiLe{
		{id: "nnl-1", ngay: "2026-01-01", ten: "Tết Dương lịch (xã tự đặt tên)"},
		{id: "nnl-2", ngay: "2026-04-30", ten: "Ngày Chiến thắng"},
		{id: "nnl-3", ngay: "2026-05-01", ten: "Ngày Quốc tế lao động"},
		{id: "nnl-4", ngay: "2026-09-02", ten: "Quốc khánh"},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	kq, err := uc.GieoNgayNghiLeMacDinh(ctx, 2026, nguoiLich())
	if err != nil {
		t.Fatalf("GieoNgayNghiLeMacDinh lỗi: %v", err)
	}
	if kq.DaGieo != 0 || kq.DaCo != 4 {
		t.Fatalf("kết quả = %+v, muốn {DaGieo:0 DaCo:4}", kq)
	}
	if k.coCau("UPDATE ngay_nghi_le") {
		t.Error("lần gieo thứ hai phát UPDATE — tên xã tự đặt phải được giữ NGUYÊN")
	}
	if k.coCau("ON CONFLICT") {
		t.Error("có ON CONFLICT trong câu lệnh")
	}
}

// A YEAR THE COMMUNE HAS NOT CONFIGURED IS A DIFFERENT WINDOW. A seed for 2027 must not see 2026's
// rows as "already there" — that is what would leave a whole year silently unseeded.
func TestGieoNgayNghiLeDungDungCuaSoNamNguoiGoiChon(t *testing.T) {
	k := &khoLichGia{nghiLe: []hangNghiLe{
		{id: "nnl-1", ngay: "2026-01-01", ten: "Tết Dương lịch"},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	kq, err := uc.GieoNgayNghiLeMacDinh(ctx, 2027, nguoiLich())
	if err != nil {
		t.Fatalf("GieoNgayNghiLeMacDinh lỗi: %v", err)
	}
	if kq.DaGieo != 4 || kq.DaCo != 0 {
		t.Fatalf("kết quả = %+v, muốn {DaGieo:4 DaCo:0} — cửa sổ 2027 không được nhìn thấy dòng 2026", kq)
	}
	for _, l := range k.cau("INSERT INTO ngay_nghi_le") {
		ngay, _ := l.args[2].(string)
		if !strings.HasPrefix(ngay, "2027-") {
			t.Errorf("gieo ngày %s trong lượt gieo năm 2027", ngay)
		}
	}
}

// --- (5) nothing recomputes a deadline already issued -------------------------------------------------

// RULE 10, INVARIANT 2, ASSERTED ON THE STATEMENTS. Changing a commune's calendar must not touch a
// commitment already made to a named citizen: those columns live on records in `service-petitions`
// and `service-documents`, which this service cannot write to at all (rule 2, forbidden #2).
//
// IT IS ASSERTED RATHER THAN ARGUED because the argument is a layering accident away from being
// false: one helpful UPDATE joining these tables to a deadline column would satisfy every other
// test in this file.
func TestDoiLichKhongChamToiHanDaLuuTrenBatKyHoSoNao(t *testing.T) {
	k := &khoLichGia{caTuan: []hangCaTuan{
		{id: idCaThu, thu: 1, batDau: 7*3600 + 30*60, ketThuc: 11*3600 + 30*60},
	}}
	uc, ctx := dungUseCaseLich(t, k)

	moi := domain.GioTrongNgay(10 * 3600)
	if _, err := uc.SuaCa(ctx, idCaThu, YeuCauSuaCa{KetThuc: &moi}, nguoiLich()); err != nil {
		t.Fatalf("SuaCa lỗi: %v", err)
	}
	if _, err := uc.GieoTuanMacDinh(ctx, nguoiLich()); err != nil {
		t.Fatalf("GieoTuanMacDinh lỗi: %v", err)
	}
	cam := []string{"han_tiep_nhan", "han_xu_ly_xong", "sla_deadline", "phan_anh", "van_ban_den", "nhiem_vu"}
	for _, l := range k.lenh {
		for _, tu := range cam {
			if strings.Contains(l.sql, tu) {
				t.Errorf("câu lệnh chạm tới %q — đổi lịch KHÔNG được tính lại hạn đã phát ra "+
					"(luật 10 bất biến 2): %s", tu, l.sql)
			}
		}
	}
}

func TestSuaCaKhongTimThayTraVeKhongTonTai(t *testing.T) {
	k := &khoLichGia{}
	uc, ctx := dungUseCaseLich(t, k)

	thu := 2
	_, err := uc.SuaCa(ctx, "01JKHONGCOCANAY0000000000X", YeuCauSuaCa{Thu: &thu}, nguoiLich())
	if err == nil || !strings.Contains(err.Error(), idstore.ErrCaLamViecKhongTonTai.Error()) {
		t.Fatalf("lỗi = %v, muốn %v", err, idstore.ErrCaLamViecKhongTonTai)
	}
}

// --- small helpers so the errors.As calls read as one line at the call sites -------------------------

// errAs is errors.As with the type parameter inferred, so a call site reads as one line. It adds no
// behaviour: a test that wants the UNWRAPPED type still gets exactly errors.As's answer.
func errAs[T any](err error, dich *T) bool { return errors.As(err, dich) }

func asLoiChongCa(err error, dich **domain.LoiCaChongCaKhac) bool { return errAs(err, dich) }

func asLoiKhongTinhDuoc(err error, dich **domain.LoiKhongTinhDuocHan) bool { return errAs(err, dich) }

func asLoiXungDot(err error, dich **domain.LoiNgayVuaNghiVuaLamBu) bool { return errAs(err, dich) }
