package app

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for §5.4's document block on the two write acts, over the REAL stores on the fake driver.
//
//	PROVED HERE   the task row, its document lines and the audit entry share ONE transaction · a
//	              refusal commits NOTHING · a new line's position comes from the store's high-water
//	              mark (which counts REMOVED lines) and not from the array index · an existing line's
//	              position is never rewritten · a removed line is UPDATEd and never deleted, naming
//	              the STAFF BUSINESS CODE · the audit delta carries counts and ids, never the text.
//
//	NOT PROVED    anything PostgreSQL does with these statements — see the header of
//	              driver_gia_nhiem_vu_test.go. Migration 0009's three CHECKs, its unique key counting
//	              soft-deleted rows, and its hard-delete trigger are the FLOOR under everything here.

// --- fixtures ---------------------------------------------------------------------------------------

const (
	idVBMot = "01JVANBANMOTTRONGTEST0000"
	idVBHai = "01JVANBANHAITRONGTEST0000"
)

// khoNVCoVanBan is the root task with TWO stored lines in one group, numbered 1 and 4 — A GAP,
// because lines ② and ③ were removed at some point and their numbers stay taken.
//
// THE GAP AND THE HIGH-WATER MARK ARE SET SEPARATELY ON PURPOSE. `thuTuVanBanLonNhat` is 4 while the
// live rows are 1 and 4: that is exactly what the real statement answers, because it does not filter
// `deleted_at`. A fixture that derived one from the other would make a renumbering write path look
// correct.
func khoNVCoVanBan() *khoNhiemVuGia {
	k := khoNVMau()
	k.vanBan[idNVGoc] = []map[string]driver.Value{
		dongVanBanGia(idVBMot, idNVGoc, domain.VanBanCapTrenGiao, 1, map[string]driver.Value{
			"so_ky_hieu":   "1742-CV/BTCTU",
			"ngay_van_ban": mocTaoNV,
			"trich_yeu":    "Công văn của Ban Tổ chức Thành uỷ",
		}),
		dongVanBanGia(idVBHai, idNVGoc, domain.VanBanCapTrenGiao, 4, nil),
	}
	k.thuTuVanBanLonNhat[string(domain.VanBanCapTrenGiao)] = 4
	return k
}

// vanBanMau is one line as §7.2's textarea produces it: prose in `TrichYeu`, nothing else.
func vanBanMau(nhom domain.NhomVanBanNhiemVu, chu string) domain.VanBanNhiemVuVao {
	return domain.VanBanNhiemVuVao{Nhom: nhom, TrichYeu: chu}
}

// cauThemVanBan returns every INSERT into the document table.
func cauThemVanBan(k *khoNhiemVuGia) []lenhPhieu {
	return k.cau("INSERT INTO nhiem_vu_van_ban")
}

// --- rule 1: every statement of the block is bound to the commune ---------------------------------

// TestKhoiVanBan_MoiCauLenhDeuRangBuocXa asserts RULE 1, INVARIANTS 4 AND 5 on the STATEMENTS
// THEMSELVES: the commune comes from the context and is always the FIRST bound argument.
//
// # WHY THIS IS ASSERTED DIRECTLY RATHER THAN LEFT TO THE OTHER TESTS
//
// It was measured on 24/09/2026, by removing `tenant_id = $1` from ThuTuVanBanLonNhat. The suite DID
// go red — but as a 600-SECOND TIMEOUT, because the fake driver reads its arguments by position, the
// shifted list panicked inside a driver call, and the pool (SetMaxOpenConns(1)) never got its
// connection back. A red that takes ten minutes and names a deadlock is a red nobody traces back to
// a missing commune predicate; the next person deletes the hanging test instead.
//
// ⚠ AND IT IS WHAT NO OTHER TEST HERE COVERS. Every assertion in this file runs in ONE commune, so a
// statement that reached every commune's rows would return exactly the same data and every one of
// them would stay green. There is no second-commune case available on a fake driver, and the
// integration test that could build one (nhiem_vu_van_ban_pg_test.go) SKIPS with no VIGOV_TEST_DSN.
func TestKhoiVanBan_MoiCauLenhDeuRangBuocXa(t *testing.T) {
	k := khoNVCoVanBan()
	uc, ctx := dungGhiNhiemVu(t, k)

	// One edit that exercises EVERY statement shape at once: an added line (MAX + INSERT), an edited
	// line (UPDATE), and a removed line (UPDATE ... deleted_at), plus both reads.
	moi := khoiVanBanDaLuu()[:1]
	moi[0].TrichYeu = "Công văn của Ban Tổ chức Thành uỷ (đã đính chính)"
	moi = append(moi, vanBanMau(domain.VanBanCapTrenGiao, "Công văn 416-CV/ĐU"))

	if _, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{VanBan: &moi}, canBoThu()); err != nil {
		t.Fatalf("sửa nhiệm vụ: %v", err)
	}

	cau := k.cau("nhiem_vu_van_ban")
	// FIVE SHAPES MUST HAVE RUN, or this test would be asserting over a shorter list than it thinks:
	// the block read, the high-water mark, the INSERT, the UPDATE and the soft removal.
	if len(cau) < 5 {
		t.Fatalf("chỉ thấy %d câu lệnh chạm bảng văn bản, muốn ít nhất 5 — phép kiểm này đang kiểm "+
			"ít hơn nó tưởng", len(cau))
	}
	for _, l := range cau {
		if !strings.Contains(l.sql, "tenant_id") {
			t.Errorf("câu lệnh không nhắc tới `tenant_id`: %q", l.sql)
			continue
		}
		if len(l.args) == 0 || l.args[0] != string(xaThu) {
			t.Errorf("tham số thứ nhất = %v, muốn mã xã %q — xã phải là $1 của MỌI câu lệnh "+
				"(luật 1 bất biến 5), và một câu đọc được mọi xã trả đúng dữ liệu ấy trong một "+
				"bộ kiểm chỉ có một xã: %q", argDau(l), xaThu, l.sql)
		}
	}
}

// argDau names the first bound argument in a failure message without indexing an empty slice.
func argDau(l lenhPhieu) any {
	if len(l.args) == 0 {
		return "(không có tham số nào)"
	}
	return l.args[0]
}

// --- 1. giao việc mới, kèm khối văn bản ---------------------------------------------------------------

// TestTaoNhiemVu_KhoiVanBanGhiTrongCUNG giao dịch VOI nhiem vu va vet — rule 6, invariant 3. A task
// committed without the lines the clerk typed is a record missing part of itself, and there is no
// second act that would put them back.
func TestTaoNhiemVu_KhoiVanBanGhiCungGiaoDichVoiNhiemVuVaVet(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)

	yc := taoMau()
	yc.VanBan = []domain.VanBanNhiemVuVao{
		vanBanMau(domain.VanBanCapTrenGiao, "Thông báo số 90-TB/TU ngày 30/01/2026 về ý kiến chỉ đạo"),
		vanBanMau(domain.VanBanSanPhamRa, "Báo cáo số 335-BC/ĐU ngày 29/6/2026"),
	}

	if _, err := uc.Tao(ctx, yc, canBoThu()); err != nil {
		t.Fatalf("giao việc mới: %v", err)
	}

	for _, tu := range []string{"INSERT INTO nhiem_vu ", "INSERT INTO nhiem_vu_van_ban",
		"INSERT INTO nhat_ky_nhiem_vu", "INSERT INTO audit_log"} {
		if !k.coCau(tu) {
			t.Errorf("thiếu câu %q", tu)
		}
	}
	chiGhiTrongGiaoDich(t, k)

	if n := len(cauThemVanBan(k)); n != 2 {
		t.Fatalf("ghi %d dòng văn bản, muốn 2", n)
	}
}

// TestTaoNhiemVu_KhongGuiKhoiVanBanThiKhongGhiDongNao — the ordinary request, and §7.3's `co-ban`
// task, which removes the whole block from the form.
func TestTaoNhiemVu_KhongGuiKhoiVanBanThiKhongGhiDongNao(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)

	if _, err := uc.Tao(ctx, taoMau(), canBoThu()); err != nil {
		t.Fatalf("giao việc mới: %v", err)
	}
	if n := len(cauThemVanBan(k)); n != 0 {
		t.Errorf("ghi %d dòng văn bản dù biểu mẫu không gửi khối nào", n)
	}
	// AND IT DOES NOT ASK FOR A POSITION EITHER: a group nobody adds to is never queried.
	if k.coCau("MAX(thu_tu)") {
		t.Error("đã đọc số thứ tự lớn nhất dù không thêm dòng nào")
	}
}

// TestTaoNhiemVu_DongVanBanSaiNhomThiTuChoiVaKhongMoGiaoDich: a rejected form must hold no row lock
// on a government register, so the refusal happens BEFORE the transaction opens.
func TestTaoNhiemVu_DongVanBanSaiNhomThiTuChoiVaKhongMoGiaoDich(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	yc := taoMau()
	yc.VanBan = []domain.VanBanNhiemVuVao{vanBanMau("van-ban-den", "x")}

	_, err := uc.Tao(ctx, yc, canBoThu())
	if !errors.Is(err, domain.ErrNhomVanBanKhongHopLe) {
		t.Fatalf("lỗi = %v, muốn ErrNhomVanBanKhongHopLe", err)
	}
	khongGhiGi(t, k)
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch cho một biểu mẫu bị từ chối, muốn 0", k.batDau)
	}
}

// TestTaoNhiemVu_DongVanBanMangIDThiTuChoi — a task that does not exist yet has no lines, so a
// client naming one is choosing internal ids.
func TestTaoNhiemVu_DongVanBanMangIDThiTuChoi(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)

	yc := taoMau()
	yc.VanBan = []domain.VanBanNhiemVuVao{{
		ID: "vb-toi-tu-chon", Nhom: domain.VanBanCapTrenGiao, TrichYeu: "x",
	}}

	_, err := uc.Tao(ctx, yc, canBoThu())
	if !errors.Is(err, domain.ErrVanBanKhongThuocNhiemVu) {
		t.Fatalf("lỗi = %v, muốn ErrVanBanKhongThuocNhiemVu", err)
	}
	khongGhiGi(t, k)
}

// TestTaoNhiemVu_VetGhiSO DONG chu KHONG ghi NOI DUNG van ban — rule 3. `trich_yeu` is the subject
// line of an administrative document and routinely names a citizen's case; `audit_log` is
// append-only and never deleted, so a copy there is a second permanent store of that text.
func TestTaoNhiemVu_VetGhiSoDongChuKhongGhiNoiDungVanBan(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)

	const biMat = "Công văn về việc giải quyết đơn của hộ ông Nguyễn Văn Ba"
	yc := taoMau()
	yc.VanBan = []domain.VanBanNhiemVuVao{vanBanMau(domain.VanBanCapTrenGiao, biMat)}

	if _, err := uc.Tao(ctx, yc, canBoThu()); err != nil {
		t.Fatalf("giao việc mới: %v", err)
	}

	vet := vetKiemToan(t, k)
	than := chuoiDelta(t, vet)
	if strings.Contains(than, biMat) {
		t.Fatalf("nội dung dòng văn bản lọt vào vết kiểm toán — sổ vết là append-only và không bao "+
			"giờ xoá, nên đó là bản lưu VĨNH VIỄN thứ hai của chữ ấy (luật 3): %s", than)
	}
	if !strings.Contains(than, `"so_dong_van_ban":1`) {
		t.Errorf("vết không ghi số dòng văn bản đã lập: %s", than)
	}
}

// --- 2. sửa khối văn bản ------------------------------------------------------------------------------

// TestSuaNhiemVu_ThemDongLayThuTuTuMocDaCapChuKhongPhaiViTriMang IS THE TEST THIS FILE EXISTS FOR.
//
// The task's group holds live lines at 1 and 4; the high-water mark is 4 because the numbers 2 and 3
// belong to lines that were REMOVED and keep them. A new line therefore gets 5.
//
// A write path that numbered from the array index would give it 3 — a number a removed row still
// holds — and `UNIQUE (tenant_id, nhiem_vu_id, nhom, thu_tu)` would refuse the INSERT inside the
// business transaction, rolling the whole edit back with a constraint error nobody can read.
func TestSuaNhiemVu_ThemDongLayThuTuTuMocDaCapChuKhongPhaiViTriMang(t *testing.T) {
	k := khoNVCoVanBan()
	uc, ctx := dungGhiNhiemVu(t, k)

	daLuu := khoiVanBanDaLuu()
	moi := append(daLuu, vanBanMau(domain.VanBanCapTrenGiao, "Công văn 416-CV/ĐU"))

	if _, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{VanBan: &moi}, canBoThu()); err != nil {
		t.Fatalf("sửa nhiệm vụ: %v", err)
	}

	chen := cauThemVanBan(k)
	if len(chen) != 1 {
		t.Fatalf("ghi %d dòng văn bản mới, muốn 1", len(chen))
	}
	// $8 is `thu_tu` in the INSERT's column list. IT COMES BACK AS int64 — database/sql widens every
	// Go integer before the driver sees it, so comparing against an untyped 5 would never match and
	// the assertion would fail whatever the write path did.
	if got := chen[0].args[7]; got != int64(5) {
		t.Fatalf("thu_tu của dòng mới = %v, muốn 5 — mốc đã cấp là 4 (số 2 và 3 do các dòng ĐÃ GỠ "+
			"giữ). Đánh số theo vị trí mảng sẽ ra 3 và đụng khoá duy nhất.", got)
	}
	chiGhiTrongGiaoDich(t, k)
}

// khoiVanBanDaLuu is the fixture block as a client re-sends it: both lines, with their ids.
func khoiVanBanDaLuu() []domain.VanBanNhiemVuVao {
	return []domain.VanBanNhiemVuVao{
		{ID: idVBMot, Nhom: domain.VanBanCapTrenGiao, SoKyHieu: "1742-CV/BTCTU",
			NgayVanBan: mocTaoNV, TrichYeu: "Công văn của Ban Tổ chức Thành uỷ"},
		{ID: idVBHai, Nhom: domain.VanBanCapTrenGiao,
			TrichYeu: "Thông báo số 90-TB/TU ngày 30/01/2026 về ý kiến chỉ đạo"},
	}
}

// TestSuaNhiemVu_GoDongLaXOA MEM, khong phai xoa cung, va NEU TEN NGUOI BANG MA CAN BO.
//
// Rule 7, invariant 1 and rule 6, invariant 8 in one act: the row stays (so the number stays taken),
// and `deleted_by` holds `CB-…` — the value somebody handling a complaint can still resolve years
// later, where a ULID would name nobody.
func TestSuaNhiemVu_GoDongLaXoaMemVaGhiMaCanBo(t *testing.T) {
	k := khoNVCoVanBan()
	uc, ctx := dungGhiNhiemVu(t, k)

	// Only the first line is sent back. The second is removed — that is the whole of `✕`.
	giuLai := khoiVanBanDaLuu()[:1]

	if _, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{VanBan: &giuLai}, canBoThu()); err != nil {
		t.Fatalf("sửa nhiệm vụ: %v", err)
	}

	// NOT A HARD REMOVAL ANYWHERE ON THIS PATH.
	for _, l := range k.lenh {
		if strings.HasPrefix(l.sql, "DELETE") {
			t.Fatalf("có câu xoá cứng trên dữ liệu nghiệp vụ: %q", l.sql)
		}
	}

	go_ := k.cau("SET deleted_at")
	if len(go_) != 1 {
		t.Fatalf("gỡ %d dòng, muốn 1", len(go_))
	}
	// $2 id · $3 nhiem_vu_id · $4 deleted_at · $5 deleted_by.
	if go_[0].args[1] != idVBHai {
		t.Errorf("gỡ nhầm dòng %v, muốn %s", go_[0].args[1], idVBHai)
	}
	if go_[0].args[2] != idNVGoc {
		t.Errorf("câu gỡ không khớp nhiệm vụ: %v — một dòng của nhiệm vụ khác sẽ gỡ được qua URL này",
			go_[0].args[2])
	}
	if go_[0].args[4] != maCanBoThu {
		t.Fatalf("deleted_by = %v, muốn MÃ NGHIỆP VỤ cán bộ %q (luật 6 bất biến 8) — một ULID ở cột "+
			"này không gọi tên ai sau nhiều năm", go_[0].args[4], maCanBoThu)
	}
	chiGhiTrongGiaoDich(t, k)
}

// TestSuaNhiemVu_SuaChuMotDongKhongDung toi thu_tu va nhom: the UPDATE names three content columns
// and nothing else, so there is no value any caller could pass that would move a line.
func TestSuaNhiemVu_SuaChuMotDongKhongDungToiThuTuVaNhom(t *testing.T) {
	k := khoNVCoVanBan()
	uc, ctx := dungGhiNhiemVu(t, k)

	moi := khoiVanBanDaLuu()
	moi[1].TrichYeu = "Thông báo số 90-TB/TU ngày 30/01/2026 (đã đính chính)"

	if _, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{VanBan: &moi}, canBoThu()); err != nil {
		t.Fatalf("sửa nhiệm vụ: %v", err)
	}

	sua := k.cau("UPDATE nhiem_vu_van_ban")
	if len(sua) != 1 {
		t.Fatalf("sửa %d dòng, muốn 1", len(sua))
	}
	if strings.Contains(sua[0].sql, "thu_tu") || strings.Contains(sua[0].sql, "nhom = ") {
		t.Fatalf("câu sửa dòng văn bản chạm tới `thu_tu` hoặc `nhom`: %q", sua[0].sql)
	}
	if len(cauThemVanBan(k)) != 0 || len(k.cau("SET deleted_at")) != 0 {
		t.Error("sửa chữ một dòng mà lại thêm hoặc gỡ dòng")
	}
}

// TestSuaNhiemVu_GuiLaiKhoiNguyenVenThiKhongGhiGi — no statement and no audit entry. That is what
// makes the route's `idem.KhongCan` declaration a property rather than a hope.
func TestSuaNhiemVu_GuiLaiKhoiNguyenVenThiKhongGhiGi(t *testing.T) {
	k := khoNVCoVanBan()
	uc, ctx := dungGhiNhiemVu(t, k)

	nguyenVen := khoiVanBanDaLuu()

	if _, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{VanBan: &nguyenVen}, canBoThu()); err != nil {
		t.Fatalf("sửa nhiệm vụ: %v", err)
	}
	// NOT khongGhiGi: this is a SUCCESS that wrote nothing, so the transaction opens and commits
	// empty — the same shape TestSuaNhiemVu_KhongDoiGiThiKhongGhiGi asserts for the scalar columns.
	// What must not exist is a statement and an entry.
	for _, l := range k.lenh {
		if strings.HasPrefix(l.sql, "UPDATE") || strings.HasPrefix(l.sql, "INSERT") {
			t.Errorf("ghi dù khối văn bản không đổi gì: %q", l.sql)
		}
	}
}

// TestSuaNhiemVu_KhongGuiKhoiVanBanThiKhoiGiuNguyen — `nil` is "not mentioned", and an empty slice
// would be "empty it". Collapsing the two is how a PATCH of the title deletes three documents.
func TestSuaNhiemVu_KhongGuiKhoiVanBanThiKhoiGiuNguyen(t *testing.T) {
	k := khoNVCoVanBan()
	uc, ctx := dungGhiNhiemVu(t, k)

	tieuDe := "Tiêu đề đã sửa"
	if _, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{TieuDe: &tieuDe}, canBoThu()); err != nil {
		t.Fatalf("sửa nhiệm vụ: %v", err)
	}
	if len(cauThemVanBan(k)) != 0 || len(k.cau("UPDATE nhiem_vu_van_ban")) != 0 {
		t.Fatal("PATCH không nhắc tới khối văn bản mà khối vẫn bị ghi")
	}
	// THE BLOCK IS STILL READ, because the response carries it: a PATCH of the title answering
	// `documents: null` would be indistinguishable from a task whose lines had just been deleted.
	if !k.coCau("FROM nhiem_vu_van_ban") {
		t.Error("không đọc khối văn bản — phản hồi PATCH sẽ thiếu `documents`")
	}
}

// TestSuaNhiemVu_GuiKhoiRONG thi go het: three `✕` clicks then Save, which has to be expressible.
func TestSuaNhiemVu_GuiKhoiRongThiGoHet(t *testing.T) {
	k := khoNVCoVanBan()
	uc, ctx := dungGhiNhiemVu(t, k)

	rong := []domain.VanBanNhiemVuVao{}
	if _, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{VanBan: &rong}, canBoThu()); err != nil {
		t.Fatalf("sửa nhiệm vụ: %v", err)
	}
	if n := len(k.cau("SET deleted_at")); n != 2 {
		t.Fatalf("gỡ %d dòng, muốn 2", n)
	}
	chiGhiTrongGiaoDich(t, k)
}

// TestSuaNhiemVu_DongLaCuaNhiemVuKhacThiTuChoiVaKhongGhiGi — refused INSIDE the transaction, on the
// block read under the task's lock, and the transaction commits nothing.
func TestSuaNhiemVu_DongLaCuaNhiemVuKhacThiTuChoiVaKhongGhiGi(t *testing.T) {
	k := khoNVCoVanBan()
	uc, ctx := dungGhiNhiemVu(t, k)

	la := []domain.VanBanNhiemVuVao{{
		ID: "01JVANBANCUANHIEMVUKHAC00", Nhom: domain.VanBanCapTrenGiao, TrichYeu: "x",
	}}

	_, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{VanBan: &la}, canBoThu())
	if !errors.Is(err, domain.ErrVanBanKhongThuocNhiemVu) {
		t.Fatalf("lỗi = %v, muốn ErrVanBanKhongThuocNhiemVu", err)
	}
	khongGhiGi(t, k)
}

// TestSuaNhiemVu_VetGhiDemVaIDDongDaGoChuKhongGhiChu — rule 3 on the audit ledger, and the one thing
// an inspection needs about a removed line: the id, which is the way back to a row that is still in
// the table carrying `deleted_at` and `deleted_by`.
func TestSuaNhiemVu_VetGhiDemVaIDDongDaGoChuKhongGhiChu(t *testing.T) {
	k := khoNVCoVanBan()
	uc, ctx := dungGhiNhiemVu(t, k)

	const biMat = "Công văn về việc giải quyết đơn của hộ ông Nguyễn Văn Ba"
	moi := append(khoiVanBanDaLuu()[:1], vanBanMau(domain.VanBanCapTrenGiao, biMat))

	if _, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{VanBan: &moi}, canBoThu()); err != nil {
		t.Fatalf("sửa nhiệm vụ: %v", err)
	}

	than := chuoiDelta(t, vetKiemToan(t, k))
	if strings.Contains(than, biMat) {
		t.Fatalf("nội dung dòng văn bản lọt vào vết kiểm toán (luật 3): %s", than)
	}
	if !strings.Contains(than, idVBHai) {
		t.Errorf("vết không nêu id dòng đã gỡ — không còn đường quay lại hàng ấy: %s", than)
	}
	if !strings.Contains(than, `"them":1`) || !strings.Contains(than, `"xoa":1`) {
		t.Errorf("vết không đếm đúng thêm/gỡ: %s", than)
	}
}

// chuoiDelta returns the audit entry's delta as text, so an assertion can say what must NOT be in it.
//
// THE DELTA IS ARGUMENT $7 of core/audit's INSERT. Read by position rather than parsed, because what
// is being asserted is the BYTES that reach the ledger — a parse would normalise exactly the thing
// under test.
func chuoiDelta(t *testing.T, vet lenhPhieu) string {
	t.Helper()
	for _, a := range vet.args {
		switch v := a.(type) {
		case []byte:
			if json.Valid(v) && strings.HasPrefix(strings.TrimSpace(string(v)), "{") {
				return string(v)
			}
		case string:
			if json.Valid([]byte(v)) && strings.HasPrefix(strings.TrimSpace(v), "{") {
				return v
			}
		}
	}
	t.Fatalf("không tìm thấy delta JSON trong câu ghi vết: %v", vet.args)
	return ""
}
