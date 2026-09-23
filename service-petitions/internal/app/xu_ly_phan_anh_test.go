package app

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The four STAFF acts, over the REAL store on the fake driver.
//
//	PROVED HERE   classification asks identity ONCE, for `phan-anh`, for the FIELD JUST SETTLED, for
//	              the RESOLVE clock ALONE, counted from `goc_dem_han` — and asks it OUTSIDE the
//	              transaction · the deadline identity returned is what reaches the UPDATE, unchanged ·
//	              a refusal from identity settles NOTHING · the change, the audit entry and the
//	              notification obligation share ONE COMMITTED transaction · a failure on the LAST of
//	              the three rolls back the first two · the audit actor is the staff BUSINESS CODE ·
//	              closing without a readable result is refused before anything is written · a status
//	              change that owes the citizen a word writes the outbox row, and one that does not
//	              still publishes the FACT · no UPDATE on this path touches the three deadline
//	              columns, `goc_dem_han` or `ma_tra_cuu` · the commune reaches every statement as $1
//	              from the CONTEXT.
//
//	NOT PROVED    ANYTHING PostgreSQL DOES. The fake driver accepts any SQL, so the CHECK constraints
//	              of migrations 0004 and 0005, the `ho_so_luu_tru_bat_bien` trigger and the partition
//	              routing are invisible here. internal/store's pg suite is where those live, and it
//	              SKIPS without VIGOV_TEST_DSN.

// --- classification asks identity the right question, once, outside the transaction ---------------

// TestChotLinhVucHoiDungCauHoi asserts the three arguments ADR 0028 decision E decides.
//
// ĐỘT BIẾN: đổi `linhVuc` thành `""` ở hanXuLyXong và ca này ĐỎ — dòng mặc định là thứ ADR 0028 quyết
// định E vừa gỡ khỏi đường phân loại, và một hạn lấy từ dòng mặc định trông y hệt một hạn đúng.
func TestChotLinhVucHoiDungCauHoi(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	uc, ctx := dungXuLy(t, k, han)

	if _, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "an-ninh"}, canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("ChotLinhVuc: %v", err)
	}

	if han.goi != 1 {
		t.Fatalf("hỏi hạn %d lần, muốn 1 — hạn được ấn định MỘT LẦN tại hành vi ấn định nó "+
			"(luật 10 bất biến 2)", han.goi)
	}
	if han.thayLoai[0] != identityv1.WorkKind_WORK_KIND_PHAN_ANH {
		t.Errorf("loại việc = %v, muốn WORK_KIND_PHAN_ANH", han.thayLoai[0])
	}
	if han.thayLinh[0] != "an-ninh" {
		t.Errorf("linh_vuc đã hỏi = %q, muốn lĩnh vực VỪA CHỐT — hỏi dòng mặc định là dựng lại đúng "+
			"cái trần 56 giờ ADR 0028 quyết định E đã gỡ", han.thayLinh[0])
	}
	if !han.thayTuLuc[0].Equal(mocGocThu) {
		t.Errorf("gốc đếm = %v, muốn %v (lúc công dân bấm gửi) — ADR 0027 quyết định D không đổi: "+
			"thời gian nằm chờ phân loại BỊ TRỪ vào hạn xử lý", han.thayTuLuc[0], mocGocThu)
	}
	moc := han.thayCanMoc[0]
	if len(moc) != 1 || moc[0] != identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG {
		t.Errorf("đồng hồ đã hỏi = %v, muốn CHỈ DEADLINE_KIND_XU_LY_XONG — hạn tiếp nhận đã được ấn "+
			"định lúc sinh dòng và không bao giờ tính lại", moc)
	}
}

// TestChotLinhVucHoiHanTruocKhiMoGiaoDich — a gRPC round trip inside the transaction would hold this
// petition's row lock for the length of a network call, and an identity outage would become a
// register that HANGS rather than one that refuses.
//
// It is safe to compute outside the lock only because `goc_dem_han` is immutable; the status is
// re-checked under the lock and the UPDATE carries it too.
func TestChotLinhVucHoiHanTruocKhiMoGiaoDich(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	uc, ctx := dungXuLy(t, k, han)

	if _, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "rac-thai"}, canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("ChotLinhVuc: %v", err)
	}

	// The FIRST statement is the unlocked read; the transaction opens only after identity answered.
	forUpdate := k.cau("FOR UPDATE")
	if len(forUpdate) != 1 {
		t.Fatalf("có %d câu đọc khoá, muốn 1", len(forUpdate))
	}
	if !forUpdate[0].trongGiaoDich {
		t.Error("câu đọc khoá chạy NGOÀI giao dịch — FOR UPDATE ngoài giao dịch không giữ khoá gì cả")
	}
	if k.batDau != 1 {
		t.Errorf("mở %d giao dịch, muốn 1", k.batDau)
	}
}

// TestChotLinhVucLuuDungHanIdentityTra — the deadline identity computed must reach the column
// UNCHANGED. A use case that rounded it, re-based it or added to it would move a commitment the
// authority is about to make to a citizen.
//
// ĐỘT BIẾN BẮT BUỘC (đổi hạn sang giờ đồng hồ thường): thay `hanChot` bằng
// `truoc.GocDemHan.Add(24 * time.Hour)` trong ChotLinhVuc và ca này ĐỎ — đó chính là hình dạng luật
// 10 cấm #2, và nó đi thẳng qua đêm, cuối tuần, ngày nghỉ lễ và ngày làm bù.
func TestChotLinhVucLuuDungHanIdentityTra(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	uc, ctx := dungXuLy(t, k, han)

	sau, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "rac-thai"}, canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("ChotLinhVuc: %v", err)
	}
	if !sau.HanXuLyXong.Equal(mocXuLyXongThu) {
		t.Fatalf("hạn trả về = %v, muốn %v", sau.HanXuLyXong, mocXuLyXongThu)
	}

	up := k.cau("UPDATE phieu_phan_anh")
	if len(up) != 1 {
		t.Fatalf("có %d câu UPDATE, muốn 1", len(up))
	}
	coHan := false
	for _, a := range up[0].args {
		if t2, ok := a.(time.Time); ok && t2.Equal(mocXuLyXongThu) {
			coHan = true
		}
	}
	if !coHan {
		t.Errorf("hạn identity tính KHÔNG có trong tham số của câu UPDATE: %v", up[0].args)
	}
	// `phan_loai_luc` is the pinned clock, and it is the instant BOTH the acknowledge clock and the
	// classification ceiling are measured against.
	coLuc := false
	for _, a := range up[0].args {
		if t2, ok := a.(time.Time); ok && t2.Equal(mocThaoTac) {
			coLuc = true
		}
	}
	if !coLuc {
		t.Errorf("thời điểm phân loại KHÔNG có trong tham số: %v", up[0].args)
	}
}

// TestChotLinhVucKhongCauHinhSLAThiKhongGhiGi — the ordinary answer in every commune today. A
// fallback deadline is refused outright (rule 10, forbidden #3), so the act must produce NOTHING: no
// field settled, no status moved, no audit entry, no notification owed.
func TestChotLinhVucKhongCauHinhSLAThiKhongGhiGi(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	han.loi = errors.New("FailedPrecondition: xã chưa cấu hình bảng thời hạn xử lý")
	uc, ctx := dungXuLy(t, k, han)

	_, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "rac-thai"}, canBoThu(), khongQuyenHanChe)
	if !errors.Is(err, ErrChuaAnDinhDuocHanXuLy) {
		t.Fatalf("lỗi = %v, muốn ErrChuaAnDinhDuocHanXuLy", err)
	}
	if k.coCau("UPDATE phieu_phan_anh") || k.coCau("INSERT") {
		t.Error("đã ghi gì đó dù không ấn định được hạn — một phiếu đã phân loại mà không có cam kết " +
			"là một phiếu không ai đếm trong khi mọi báo cáo đúng hạn vẫn đếm nó")
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch dù sẽ từ chối", k.batDau)
	}
}

// TestChotLinhVucLinhVucRongThiTuChoiTruocMoiThu — the act EXISTS to settle the field, so an empty one
// is refused before identity is asked and before anything is read.
func TestChotLinhVucLinhVucRongThiTuChoiTruocMoiThu(t *testing.T) {
	for ten, vao := range map[string]string{
		"rỗng":        "",
		"toàn dấu":    "   ",
		"có hoa":      "Rac-Thai",
		"có dấu gạch": "-rac-thai",
	} {
		t.Run(ten, func(t *testing.T) {
			k, han := khoPhieuMau(), hanXuLyThu()
			uc, ctx := dungXuLy(t, k, han)

			if _, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: vao}, canBoThu(), khongQuyenHanChe); err == nil {
				t.Fatal("chấp nhận một mã lĩnh vực không hợp lệ")
			}
			if han.goi != 0 || len(k.lenh) != 0 {
				t.Errorf("đã hỏi identity (%d) hoặc chạm dữ liệu (%d câu) dù đầu vào sai",
					han.goi, len(k.lenh))
			}
		})
	}
}

// --- the three writes share one transaction ---------------------------------------------------------

// TestChotLinhVucGhiBaThuTrongCUNGMotGiaoDich is rule 6, invariant 3 and rule 10, invariant 5 in one
// assertion.
//
// THE THIRD ROW IS THE ONE THAT IS EASY TO LOSE. Publishing the notification after the commit with
// nothing recorded loses it whenever the process dies in the window — and nothing anywhere then says
// a citizen was never told.
func TestChotLinhVucGhiBaThuTrongCUNGMotGiaoDich(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	uc, ctx := dungXuLy(t, k, han)

	if _, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "rac-thai"}, canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("ChotLinhVuc: %v", err)
	}

	for ten, tu := range map[string]string{
		"đổi phiếu":  "UPDATE phieu_phan_anh",
		"vết kiểm":   "INSERT INTO audit_log",
		"sự kiện đi": "INSERT INTO su_kien_di",
	} {
		cau := k.cau(tu)
		if len(cau) != 1 {
			t.Fatalf("%s: có %d câu, muốn 1", ten, len(cau))
		}
		if !cau[0].trongGiaoDich {
			t.Errorf("%s chạy NGOÀI giao dịch", ten)
		}
	}
	if k.daCommit != 1 || k.daRollback != 0 {
		t.Errorf("commit=%d rollback=%d, muốn 1/0", k.daCommit, k.daRollback)
	}
}

// TestChotLinhVucSuKienHongThiKhongCoGiDuoc — the outbox row is the LAST of the three, so a failure
// there is what proves the other two go down with it. Without this case the transaction boundary
// would be a claim rather than a property.
func TestChotLinhVucSuKienHongThiKhongCoGiDuoc(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	k.loiSau = "INSERT INTO su_kien_di"
	uc, ctx := dungXuLy(t, k, han)

	if _, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "rac-thai"}, canBoThu(), khongQuyenHanChe); err == nil {
		t.Fatal("ChotLinhVuc thành công dù dòng sự kiện hỏng")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit=%d rollback=%d, muốn 0/1 — phiếu đã phân loại mà không ai báo cho dân là "+
			"trạng thái luật 10 bất biến 5 không cho phép", k.daCommit, k.daRollback)
	}
}

// TestChotLinhVucVetMangMaCanBo pins rule 6, invariant 8 where it was broken across six write paths on
// 2026-09-22 with nothing turning red.
func TestChotLinhVucVetMangMaCanBo(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	uc, ctx := dungXuLy(t, k, han)

	if _, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "rac-thai"}, canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("ChotLinhVuc: %v", err)
	}
	vet := k.cau("INSERT INTO audit_log")[0]
	if !coThamSo(vet.args, maCanBoThu) {
		t.Errorf("vết không mang mã cán bộ %q: %v", maCanBoThu, vet.args)
	}
	if !coThamSo(vet.args, HanhViPhanLoaiPhanAnh) {
		t.Errorf("vết không mang hành vi %q: %v", HanhViPhanLoaiPhanAnh, vet.args)
	}
	if !coThamSo(vet.args, maPhieuThu) {
		t.Errorf("vết không mang MÃ TRA CỨU làm chủ thể: %v", vet.args)
	}
}

// TestChotLinhVucThieuMaCanBoThiTuChoi — a business write whose trail cannot name its author is what
// rule 6 does not permit, and refusing BEFORE the transaction opens keeps the refusal readable.
func TestChotLinhVucThieuMaCanBoThiTuChoi(t *testing.T) {
	for ten, nguoi := range map[string]audit.Actor{
		// AN EMPTY `Ma` NEVER FALLS BACK TO THE INTERNAL id. That fallback would put two kinds of
		// identifier into `audit_log.actor_id` one deployment window at a time, with every test green.
		"không có mã": {ID: "", Kind: "staff", IP: "10.0.0.7"},
		// A CITIZEN PRINCIPAL ON A STAFF ACT means the route was mounted on the wrong mux, and rule 4,
		// invariant 5 keeps the two surfaces apart precisely so that cannot happen quietly.
		"chủ thể công dân": {ID: idCongDanThu, Kind: "citizen", IP: "10.0.0.9"},
	} {
		t.Run(ten, func(t *testing.T) {
			k, han := khoPhieuMau(), hanXuLyThu()
			uc, ctx := dungXuLy(t, k, han)

			if _, err := uc.ChotLinhVuc(ctx, maPhieuThu,
				YeuCauChotLinhVuc{LinhVuc: "rac-thai"}, nguoi, khongQuyenHanChe); err == nil {
				t.Fatal("ghi được dù vết không gọi tên được người làm")
			}
			if len(k.lenh) != 0 {
				t.Errorf("đã chạm dữ liệu dù thiếu chủ thể (%d câu)", len(k.lenh))
			}
		})
	}
}

// --- the classification ceiling, and what it means for an overdue petition --------------------------

// TestTranPhanLoaiTinhTheoGioLamViecChuKhongCong24Gio is the expensive case the task brief names.
//
// It proves TWO halves at once, and both are the ADR 0035 §C decision rather than a style:
//
//	the ceiling on the fixture is NOT 24 wall-clock hours after the origin — it is what identity's
//	AdvanceWorkingHours returned, wherever that commune's calendar put it;
//	and asking for it is a call to identity, not arithmetic here.
//
// The fixture instants are deliberately at odd offsets so a local `.Add(24 * time.Hour)` could not
// produce them.
func TestTranPhanLoaiTinhTheoGioLamViecChuKhongCong24Gio(t *testing.T) {
	// The intake use case is what fixes the ceiling; this asserts the value it stores comes from the
	// RPC and not from arithmetic.
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()
	if _, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), congDanThu()); err != nil {
		t.Fatalf("Gui: %v", err)
	}

	if han.goiTran != 1 {
		t.Fatalf("hỏi trần phân loại %d lần, muốn 1", han.goiTran)
	}
	if len(han.thayTranGio[0]) != 1 || han.thayTranGio[0][0] != domain.GioTranPhanLoai {
		t.Errorf("số giờ đã hỏi = %v, muốn [%d] — ADR 0035 §C đếm bằng GIỜ LÀM VIỆC qua identity, "+
			"không phải ngày lịch", han.thayTranGio[0], domain.GioTranPhanLoai)
	}
	if !han.thayTranTu[0].Equal(mocGuiThu) {
		t.Errorf("gốc đếm trần = %v, muốn %v (lúc tiếp nhận)", han.thayTranTu[0], mocGuiThu)
	}

	p := kho.thay[0]
	if !p.HanPhanLoai.Equal(mocTranThu) {
		t.Fatalf("trần đã lưu = %v, muốn %v", p.HanPhanLoai, mocTranThu)
	}
	// THE ASSERTION THAT KILLS THE WALL-CLOCK VERSION. `24 * time.Hour` after the origin is a
	// DIFFERENT instant from what identity returned, and a use case that computed it locally would
	// land on the first value rather than the second.
	congThang := mocGuiThu.Add(24 * time.Hour) // @sla-ok: đây là GIÁ TRỊ SAI mà ca này chứng minh KHÔNG được dùng
	if p.HanPhanLoai.Equal(congThang) {
		t.Error("trần phân loại đúng bằng gốc + 24 giờ đồng hồ — đó là hình dạng luật 10 cấm #2, " +
			"nó đi thẳng qua đêm, cuối tuần, ngày nghỉ lễ và ngày làm bù")
	}
}

// TestPhieuVuotTranChuaPhanLoaiDocRaLaTre — the point of the ceiling. A petition sitting unclassified
// past it counts as LATE, so a commune that classifies slowly cannot have the better figure.
//
// DERIVED, NEVER STORED (rule 10, invariant 3): the answer changes with `now` and with nothing else.
func TestPhieuVuotTranChuaPhanLoaiDocRaLaTre(t *testing.T) {
	p := domain.PhieuPhanAnh{
		GocDemHan:   mocGocThu,
		HanPhanLoai: mocTranPhanLoaiThu,
		TrangThai:   domain.DaTiepNhan,
	}
	if p.QuaHanPhanLoai(mocTranPhanLoaiThu.Add(-time.Minute)) {
		t.Error("trước trần mà đã tính là trễ")
	}
	if !p.QuaHanPhanLoai(mocTranPhanLoaiThu.Add(time.Minute)) {
		t.Error("quá trần mà chưa phân loại KHÔNG tính là trễ — xã phân loại chậm lại sẽ có chỉ số " +
			"ĐẸP HƠN, đúng cái ADR 0035 §C sinh ra để chặn")
	}

	// Classified in time stops the clock, and classified late STAYS late — otherwise last quarter's
	// figure would change every time somebody opened the screen.
	dung := p
	dung.PhanLoaiLuc = mocTranPhanLoaiThu.Add(-time.Hour)
	if dung.QuaHanPhanLoai(mocTranPhanLoaiThu.Add(72 * time.Hour)) {
		t.Error("phân loại đúng hạn mà vẫn đọc ra trễ khi thời gian trôi")
	}
	tre := p
	tre.PhanLoaiLuc = mocTranPhanLoaiThu.Add(time.Hour)
	if !tre.QuaHanPhanLoai(mocTranPhanLoaiThu.Add(72 * time.Hour)) {
		t.Error("phân loại TRỄ mà sau đó đọc ra đúng hạn — việc chậm của xã tự biến mất khỏi số liệu")
	}

	// A staff-booked petition has NO ceiling at all, and "không áp dụng" is never "đúng hạn" by
	// accident: the column is NULL and the predicate says so.
	nhapHo := domain.PhieuPhanAnh{GocDemHan: mocGocThu, TrangThai: domain.DaTiepNhan}
	if !nhapHo.TranPhanLoaiKhongApDung() || nhapHo.QuaHanPhanLoai(mocTranPhanLoaiThu.Add(72*time.Hour)) {
		t.Error("phiếu nhập hộ có trần phân loại — kênh ấy vào sổ đã mang lĩnh vực, không có khoảng nào để chặn")
	}
}

// --- the notification obligation ----------------------------------------------------------------------

// TestTienTrangThaiGhiSuKienVaLoiNhan — rule 10, invariant 5. Entering processing is one of the three
// transitions the citizen is told about, and the obligation is recorded in the SAME transaction as
// the change.
//
// ĐỘT BIẾN: xoá dòng `DangXuLy` khỏi domain.vietTiepTheo và ca này ĐỎ — một người dân không được báo
// thì không phân biệt được "đang xử lý" với "bị bỏ quên".
func TestTienTrangThaiGhiSuKienVaLoiNhan(t *testing.T) {
	k := khoPhieuMau()
	// `da-chuyen-xu-ly` -> `dang-xu-ly`: the transition the citizen IS told about.
	k.hang = dongPhieuMau(map[string]any{
		"trang_thai":     string(domain.DaChuyenXuLy),
		"linh_vuc":       "rac-thai",
		"han_xu_ly_xong": mocXuLyXongThu,
		"phan_loai_luc":  mocThaoTac,
		"bo_phan_id":     "bp-001",
	})
	uc, ctx := dungXuLy(t, k, hanXuLyThu())

	sau, err := uc.TienTrangThai(ctx, maPhieuThu, canBoThu(), coQuyenCaXa, khongQuyenHanChe)
	if err != nil {
		t.Fatalf("TienTrangThai: %v", err)
	}
	if sau.TrangThai != domain.DangXuLy {
		t.Fatalf("trạng thái = %q, muốn dang-xu-ly", sau.TrangThai)
	}

	su := k.cau("INSERT INTO su_kien_di")
	if len(su) != 1 {
		t.Fatalf("có %d dòng sự kiện, muốn 1 — đổi trạng thái mà không báo cho dân là luật 10 bất biến 5",
			len(su))
	}
	than := thanSuKien(t, su[0].args)
	if than["status"] != string(domain.DangXuLy) {
		t.Errorf("sự kiện mang trạng thái %v, muốn dang-xu-ly", than["status"])
	}
	if than["lookup_code"] != maPhieuThu {
		t.Errorf("sự kiện mang mã %v, muốn mã tra cứu", than["lookup_code"])
	}
	if than["citizen_id"] != idCongDanThu {
		t.Errorf("sự kiện mang người nhận %v", than["citizen_id"])
	}
	// `occurrence` STARTS AT 1 AND IS NEVER 0. A consumer that received 0 treats the message as
	// malformed on purpose: defaulting it would make the second closing of a reopened petition look
	// like a duplicate of the first, and the citizen would never be told about it.
	if than["occurrence"] != float64(1) {
		t.Errorf("occurrence = %v, muốn 1", than["occurrence"])
	}
	tin, co := than["citizen_message"].(map[string]any)
	if !co {
		t.Fatalf("bước này KHÔNG mang lời nhắn cho dân: %v", than)
	}
	if tin["status_label"] != domain.NhanTrangThai(domain.DangXuLy) {
		t.Errorf("nhãn = %v", tin["status_label"])
	}
	if tin["next_step"] == "" || tin["next_step"] == tin["status_label"] {
		t.Errorf("việc tiếp theo rỗng hoặc chỉ lặp lại nhãn (%v) — bên nhận từ chối cả hai", tin["next_step"])
	}

	// NOT ONE CHARACTER OF THE PETITION, THE REPORTER OR THE RESULT (rule 3; the event contract closes
	// the list at four scalars plus two composed sentences).
	raw := string(su[0].args[4].([]byte))
	for _, cam := range []string{"Đống rác", "Nguyễn Văn An", "0900000000", "Đầu ngõ"} {
		if strings.Contains(raw, cam) {
			t.Errorf("thân sự kiện mang dữ liệu cá nhân %q — hàng đợi được sao lưu, nhân bản và đọc "+
				"lúc gỡ lỗi, và xoá trường về sau không thu hồi được bản sao", cam)
		}
	}
}

// TestPhanLoaiKhongGuiLoiNhanChoDan — classification is an INTERNAL step. The FACT is still published
// (a consumer counting time-in-status must not find three of nine transitions missing), but it carries
// no citizen message, so `comms` writes no ledger row and sends nothing.
func TestPhanLoaiKhongGuiLoiNhanChoDan(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	uc, ctx := dungXuLy(t, k, han)

	if _, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "rac-thai"}, canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("ChotLinhVuc: %v", err)
	}
	than := thanSuKien(t, k.cau("INSERT INTO su_kien_di")[0].args)
	if _, co := than["citizen_message"]; co {
		t.Error("bước phân loại gửi lời nhắn cho dân — đó là bước nội bộ, và luật 4 cấm #5 giữ lịch " +
			"sử chuyển và ghi chú nội bộ khỏi mắt người dân")
	}
	if than["status"] != string(domain.DangPhanLoai) {
		t.Errorf("sự kiện mang trạng thái %v, muốn dang-phan-loai — SỰ THẬT vẫn phải phát đi", than["status"])
	}
}

// --- closing ------------------------------------------------------------------------------------------

// TestDongPhieuKhongCoKetQuaThiTuChoiTruocMoiThu — rule 10, invariant 6. "Đã xử lý" alone is not a
// result: a citizen told only that their report was closed cannot tell being helped from being
// dismissed.
func TestDongPhieuKhongCoKetQuaThiTuChoiTruocMoiThu(t *testing.T) {
	for ten, ketQua := range map[string]string{
		"rỗng":      "",
		"toàn dấu":  "   \n\t ",
		"quá ngắn":  "xong",
		"chỉ trạng": "đã xử lý",
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = dongPhieuMau(map[string]any{"trang_thai": string(domain.ChoDanXacNhan)})
			uc, ctx := dungXuLy(t, k, hanXuLyThu())

			if _, err := uc.Dong(ctx, maPhieuThu, ketQua, canBoThu(), khongQuyenHanChe); err == nil {
				t.Fatal("đóng được phiếu mà người dân không đọc được gì")
			}
			if len(k.lenh) != 0 {
				t.Errorf("đã chạm dữ liệu dù sẽ từ chối (%d câu)", len(k.lenh))
			}
		})
	}
}

// TestDongPhieuGhiKetQuaVaBaoChoDan — the happy path, and the two things it must produce together.
func TestDongPhieuGhiKetQuaVaBaoChoDan(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(map[string]any{
		"trang_thai":     string(domain.ChoDanXacNhan),
		"linh_vuc":       "rac-thai",
		"han_xu_ly_xong": mocXuLyXongThu,
		"xu_ly_xong_luc": mocThaoTac,
	})
	uc, ctx := dungXuLy(t, k, hanXuLyThu())

	sau, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("Dong: %v", err)
	}
	if sau.TrangThai != domain.DaDong || sau.KetQuaXuLy != ketQuaThat {
		t.Fatalf("trạng thái=%q kết quả=%q", sau.TrangThai, sau.KetQuaXuLy)
	}
	if !coThamSo(k.cau("UPDATE phieu_phan_anh")[0].args, ketQuaThat) {
		t.Error("kết quả KHÔNG xuống câu UPDATE — đóng phiếu mà không lưu kết quả là đóng lặng lẽ")
	}

	than := thanSuKien(t, k.cau("INSERT INTO su_kien_di")[0].args)
	if than["status"] != string(domain.DaDong) {
		t.Errorf("sự kiện mang trạng thái %v", than["status"])
	}
	if _, co := than["citizen_message"]; !co {
		t.Error("đóng phiếu mà KHÔNG báo cho dân — luật 10 bất biến 5 và bất biến 6 cùng đòi bước này")
	}

	// THE RESULT ITSELF NEVER TRAVELS. It is free text about ONE case and a queue is persisted,
	// replicated and backed up; the citizen reaches it with their lookup code behind an authenticated
	// read.
	if strings.Contains(string(k.cau("INSERT INTO su_kien_di")[0].args[4].([]byte)), "thu gom") {
		t.Error("nội dung kết quả đi theo sự kiện ra hàng đợi")
	}
	// AND IT IS NOT IN THE AUDIT DELTA EITHER (rule 6, forbidden #4): `audit_log` is append-only and
	// never deleted, so a copy there is a second permanent store of what a member of staff wrote about
	// one citizen's case.
	for _, a := range k.cau("INSERT INTO audit_log")[0].args {
		if b, ok := a.([]byte); ok && strings.Contains(string(b), "thu gom") {
			t.Error("nội dung kết quả nằm trong delta của vết kiểm toán")
		}
		if s, ok := a.(string); ok && strings.Contains(s, "thu gom") {
			t.Error("nội dung kết quả nằm trong vết kiểm toán")
		}
	}
}

// TestDongPhieuSaiBuocThiTuChoi — `cho-dan-xac-nhan` is the only status the lifecycle closes from
// (ADR 0027). Closing from anywhere else would close a petition before the citizen was ever asked.
func TestDongPhieuSaiBuocThiTuChoi(t *testing.T) {
	for _, tt := range []domain.TrangThai{
		domain.DaTiepNhan, domain.DangPhanLoai, domain.DaChuyenXuLy,
		domain.DangXuLy, domain.DaXuLy, domain.DaDong,
	} {
		t.Run(string(tt), func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = dongPhieuMau(map[string]any{"trang_thai": string(tt)})
			uc, ctx := dungXuLy(t, k, hanXuLyThu())

			if _, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, canBoThu(), khongQuyenHanChe); !errors.Is(err, domain.ErrDongSaiLuc) {
				t.Fatalf("lỗi = %v, muốn ErrDongSaiLuc", err)
			}
			if k.coCau("UPDATE phieu_phan_anh") {
				t.Error("đã ghi dù bước không cho đóng")
			}
			if k.daCommit != 0 {
				t.Errorf("commit %d lần dù từ chối", k.daCommit)
			}
		})
	}
}

// --- assignment -------------------------------------------------------------------------------------

// TestPhanCongLanDauChuyenSangDaChuyenXuLy — the first routing IS the transition into
// `da-chuyen-xu-ly`, which is the only edge into that status in the whole lifecycle map.
func TestPhanCongLanDauChuyenSangDaChuyenXuLy(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(map[string]any{
		"trang_thai": string(domain.DangPhanLoai), "linh_vuc": "rac-thai",
		"han_xu_ly_xong": mocXuLyXongThu, "phan_loai_luc": mocThaoTac,
	})
	uc, ctx := dungXuLy(t, k, hanXuLyThu())

	sau, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-001", CanBo: "CB-00999"}, canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("PhanCong: %v", err)
	}
	if sau.TrangThai != domain.DaChuyenXuLy {
		t.Errorf("trạng thái = %q, muốn da-chuyen-xu-ly", sau.TrangThai)
	}
	if len(k.cau("INSERT INTO su_kien_di")) != 1 {
		t.Error("lần phân công đầu đổi trạng thái mà không phát sự kiện")
	}
}

// TestPhanCongLaiKhongDoiTrangThaiVaKhongPhatSuKien — docs/ui-ux/09 §8.5: "Chuyển xử lý, KHÔNG đổi
// trạng thái". Moving a petition between departments mid-processing must not drag it backwards, and
// a `status_changed` event where no status changed would be a lie on the wire.
func TestPhanCongLaiKhongDoiTrangThaiVaKhongPhatSuKien(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(map[string]any{
		"trang_thai": string(domain.DangXuLy), "linh_vuc": "rac-thai",
		"han_xu_ly_xong": mocXuLyXongThu, "phan_loai_luc": mocThaoTac, "bo_phan_id": "bp-001",
	})
	uc, ctx := dungXuLy(t, k, hanXuLyThu())

	sau, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-002"}, canBoThu(), khongQuyenHanChe)
	if err != nil {
		t.Fatalf("PhanCong: %v", err)
	}
	if sau.TrangThai != domain.DangXuLy {
		t.Errorf("trạng thái = %q — phân công lại kéo phiếu lùi lại một bước", sau.TrangThai)
	}
	if k.coCau("INSERT INTO su_kien_di") {
		t.Error("phát sự kiện status_changed dù không có trạng thái nào đổi")
	}
	// THE AUDIT ENTRY IS STILL WRITTEN. A re-assignment is a real administrative act — who was made
	// responsible, and when — and it is the first question asked of a disputed petition.
	if len(k.cau("INSERT INTO audit_log")) != 1 {
		t.Error("phân công lại KHÔNG để lại vết")
	}
}

// TestPhanCongChuaPhanLoaiThiTuChoi — assigning an unclassified petition would put a department in
// charge of work whose resolve deadline has not been fixed yet (ADR 0028 decision E).
func TestPhanCongChuaPhanLoaiThiTuChoi(t *testing.T) {
	k := khoPhieuMau() // the fixture is `da-tiep-nhan`
	uc, ctx := dungXuLy(t, k, hanXuLyThu())

	if _, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-001"}, canBoThu(), khongQuyenHanChe); !errors.Is(err, domain.ErrPhanCongSaiLuc) {
		t.Fatalf("lỗi = %v, muốn ErrPhanCongSaiLuc", err)
	}
	if k.coCau("UPDATE phieu_phan_anh") {
		t.Error("đã ghi dù chưa phân loại")
	}
}

// --- the race between two officers ---------------------------------------------------------------------

// TestPhieuDaChuyenTrangThiTuChoiChuKhongGhiDe — the UPDATE carries the expected status, so the loser
// of a race matches no row. A silent overwrite would move a deadline the winner already promised.
func TestPhieuDaChuyenTrangThiTuChoiChuKhongGhiDe(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	k.doiDong = 0 // the database saw the row move between the locking read and the write
	uc, ctx := dungXuLy(t, k, han)

	_, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "rac-thai"}, canBoThu(), khongQuyenHanChe)
	if !errors.Is(err, petstore.ErrPhieuDaChuyenTrang) {
		t.Fatalf("lỗi = %v, muốn ErrPhieuDaChuyenTrang", err)
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit=%d rollback=%d, muốn 0/1", k.daCommit, k.daRollback)
	}
}

// --- the holding rule: who may advance a petition ---------------------------------------------------
//
// Decided by the owner on 2026-09-23 ("theo require"). THE THIRD CELL IS THE REASON THE CHANGE EXISTS
// and the fourth is what keeps it from being "everybody with feedback.read"; the negative half — that
// the CLOSING path was not widened with it — is pinned at the route, in
// internal/http/xu_ly_phan_anh_test.go, because that is the layer that decides it.

// The two values of app.QuyenXuLyCaXa, named so a case reads as the fact it is setting rather than as
// a bare literal at a call site.
const (
	coQuyenCaXa    QuyenXuLyCaXa = true
	khongQuyenCaXa QuyenXuLyCaXa = false
)

// idCanBoNoiBoThu is the INTERNAL id of the same officer whose business code is maCanBoThu.
//
// IT EXISTS ONLY TO BE REFUSED. `authz.Principal` carries both identifiers and they are
// indistinguishable on sight; this fixture is what makes "the comparison is between BUSINESS CODES"
// an assertion instead of a comment.
const idCanBoNoiBoThu = "nd-01JCANBONOIBOCUAXA"

// phieuDaGiaoCho is the fixture every case below starts from: a petition sitting at
// `da-chuyen-xu-ly`, ready to advance, ASSIGNED to whoever the argument names. An empty string is the
// unassigned petition — a real state, not an edge case: "— Để bộ phận phân công —" is a choice on the
// assignment screen.
func phieuDaGiaoCho(canBo string) map[string]driver.Value {
	var giao any
	if canBo != "" {
		giao = canBo
	}
	return dongPhieuMau(map[string]any{
		"trang_thai":      string(domain.DaChuyenXuLy),
		"linh_vuc":        "rac-thai",
		"han_xu_ly_xong":  mocXuLyXongThu,
		"phan_loai_luc":   mocThaoTac,
		"bo_phan_id":      "bp-001",
		"can_bo_xu_ly_id": giao,
	})
}

// TestTienTrangThaiBonOCuaLuatNamGiu is the four-cell table the decision was recorded with.
//
//	feedback.resolve | là người được giao | mong đợi
//	có               | có                 | tiến được
//	có               | không              | tiến được   — quyền của cả xã, đúng như trước
//	KHÔNG            | CÓ                 | TIẾN ĐƯỢC   ← ô mới, cả thay đổi này tồn tại vì nó
//	không            | không              | TỪ CHỐI
//
// ĐỘT BIẾN: đổi phép so trong duocTienTrangThai sang một định danh khác loại (id nội bộ) và ô thứ ba
// ĐỎ — trưởng thôn cầm phiếu của chính thôn mình vẫn bị báo là không được giao, im lặng, không gì khác
// đỏ theo.
func TestTienTrangThaiBonOCuaLuatNamGiu(t *testing.T) {
	for ten, ca := range map[string]struct {
		quyen    QuyenXuLyCaXa
		giaoCho  string
		tienDuoc bool
	}{
		"có quyền cả xã + là người được giao":            {coQuyenCaXa, maCanBoThu, true},
		"có quyền cả xã + KHÔNG phải người được giao":    {coQuyenCaXa, "CB-99999", true},
		"KHÔNG quyền cả xã + LÀ người được giao":         {khongQuyenCaXa, maCanBoThu, true},
		"không quyền cả xã + không phải người được giao": {khongQuyenCaXa, "CB-99999", false},
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = phieuDaGiaoCho(ca.giaoCho)
			uc, ctx := dungXuLy(t, k, hanXuLyThu())

			sau, err := uc.TienTrangThai(ctx, maPhieuThu, canBoThu(), ca.quyen, khongQuyenHanChe)

			if ca.tienDuoc {
				if err != nil {
					t.Fatalf("TienTrangThai: %v", err)
				}
				if sau.TrangThai != domain.DangXuLy {
					t.Fatalf("trạng thái = %q, muốn dang-xu-ly", sau.TrangThai)
				}
				return
			}
			if !errors.Is(err, ErrKhongPhaiNguoiDuocGiao) {
				t.Fatalf("lỗi = %v, muốn ErrKhongPhaiNguoiDuocGiao", err)
			}
			// NOTHING COMMITTED, and the count is the assertion: a refusal that had already written the
			// UPDATE and rolled back would be correct today and would stop being correct the first time
			// somebody moved the check after a write.
			if k.coCau("UPDATE phieu_phan_anh") || k.coCau("INSERT INTO audit_log") ||
				k.coCau("INSERT INTO su_kien_di") {
				t.Error("đã ghi dù từ chối — một lần ghi nghiệp vụ của người không được phép")
			}
			if k.daCommit != 0 {
				t.Errorf("commit=%d, muốn 0", k.daCommit)
			}
		})
	}
}

// TestTienTrangThaiSoBangMaCanBoChuKhongPhaiIdNoiBo pins rule 6, invariant 8 AT THE COMPARISON.
//
// The petition is assigned to `CB-00123` and the actor arrives carrying `nd-01J…` — the INTERNAL id of
// the same person. It must be refused: matching those two would mean the column and the actor hold
// interchangeable values, and the day a caller hands the internal id down, every officer would fall
// into the "not the assignee" branch with nothing red anywhere.
func TestTienTrangThaiSoBangMaCanBoChuKhongPhaiIdNoiBo(t *testing.T) {
	k := khoPhieuMau()
	k.hang = phieuDaGiaoCho(maCanBoThu)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())

	_, err := uc.TienTrangThai(ctx, maPhieuThu,
		audit.Actor{ID: idCanBoNoiBoThu, Kind: "staff", IP: "10.0.0.7"}, khongQuyenCaXa,
		khongQuyenHanChe)

	if !errors.Is(err, ErrKhongPhaiNguoiDuocGiao) {
		t.Fatalf("lỗi = %v, muốn ErrKhongPhaiNguoiDuocGiao — id nội bộ KHÔNG phải mã cán bộ, và hai "+
			"loại định danh khớp nhau là dấu hiệu cột người xử lý đang giữ cả hai", err)
	}
}

// TestTienTrangThaiPhieuChuaGiaoThiChuoiRongKhongKhopVoiAi is the `"" == ""` case.
//
// `can_bo_xu_ly_id` IS NULL WHENEVER A DEPARTMENT WAS TOLD TO ASSIGN INTERNALLY, which is an ordinary
// state of an ordinary petition. Without the explicit guard, an actor with an empty code — which a
// service-identity older than the `ma` field can produce — would match it, and EVERY account holding
// `feedback.read` could advance EVERY unassigned petition in the commune.
//
// ĐỘT BIẾN: bỏ vế `p.CanBoXuLyID == ""` khỏi duocTienTrangThai và ca này ĐỎ.
func TestTienTrangThaiPhieuChuaGiaoThiChuoiRongKhongKhopVoiAi(t *testing.T) {
	k := khoPhieuMau()
	k.hang = phieuDaGiaoCho("")
	uc, ctx := dungXuLy(t, k, hanXuLyThu())

	// The actor carries a REAL business code — so what is being proved is that an UNASSIGNED petition
	// matches nobody, not that an empty actor is refused (coCanBoThucHien already does that, and it is
	// asserted separately).
	_, err := uc.TienTrangThai(ctx, maPhieuThu, canBoThu(), khongQuyenCaXa, khongQuyenHanChe)
	if !errors.Is(err, ErrKhongPhaiNguoiDuocGiao) {
		t.Fatalf("lỗi = %v, muốn ErrKhongPhaiNguoiDuocGiao — phiếu chưa giao cho ai thì không khớp "+
			"với ai", err)
	}
	if k.coCau("UPDATE phieu_phan_anh") {
		t.Error("đã ghi dù phiếu chưa giao cho ai")
	}
}

// TestDongKhongNhanQuyenNguoiDuocGiao is the NEGATIVE HALF, asserted at the only place this layer can
// assert it: the SHAPE of the closing use case.
//
// `Dong` takes no QuyenXuLyCaXa, so there is no value any caller could pass that would let an assignee
// without `feedback.resolve` close a petition. This test fails to COMPILE if somebody gives it one —
// which is the point: open question #7 was settled on 2026-09-16 ("`feedback.resolve` quyết định ai
// đóng được") and the holding rule of 2026-09-23 widened the WORKING path only. The route-level half
// of this negative is in internal/http/xu_ly_phan_anh_test.go.
func TestDongKhongNhanQuyenNguoiDuocGiao(t *testing.T) {
	k := khoPhieuMau()
	k.hang = phieuDaGiaoCho(maCanBoThu)
	k.hang["trang_thai"] = string(domain.ChoDanXacNhan)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())

	// NO QuyenXuLyCaXa ANYWHERE IN THE SIGNATURE. The fifth argument is QuyenXemHanChe, which is the
	// OPPOSITE kind of fact: it can only ever narrow who may close a petition, and no value of it
	// lets an assignee without `feedback.resolve` close anything. Naming both types here is what makes
	// this assertion survive a later parameter being added — it fails to compile if the one that
	// WIDENS appears.
	//
	// The assignment on the row above is irrelevant to this act by construction.
	var dong func(context.Context, string, string, audit.Actor, QuyenXemHanChe) (
		domain.PhieuPhanAnh, error) = uc.Dong
	if _, err := dong(ctx, maPhieuThu, ketQuaThat, canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("Dong: %v", err)
	}
}

// --- the restricted field: a report ABOUT a member of staff ------------------------------------------
//
// THE LEAK THIS CLOSES WAS MEASURED, NOT SUSPECTED. Both READ paths refused a caller without
// `feedback.restricted` — the detail route answers 404 and the list excludes the rows inside the
// WHERE clause — while all four WRITE paths asked nothing, and all four return the whole petition on
// success. So one POST read out a report about a member of staff to a colleague of that person.
//
// WHAT IS PROVED BELOW is the harder half: the ACT is refused, not merely the body. A refusal writes
// no UPDATE, files no audit entry, records no notification, and commits nothing.

// The two values of QuyenXemHanChe, named so a case reads as the fact it is setting rather than as a
// bare literal — the same reason coQuyenCaXa and khongQuyenCaXa are named above, and it matters more
// here: the wrong literal at one call site opens exactly what this restriction protects.
const (
	coQuyenHanChe    QuyenXemHanChe = true
	khongQuyenHanChe QuyenXemHanChe = false
)

// phieuLinhVuc is one petition at a chosen point of the lifecycle, in a chosen field.
//
// EVERY CASE BELOW RUNS THE SAME PETITION TWICE — once in `rac-thai`, once in `can-bo` — so the only
// difference between "refused" and "allowed" is the field. A fixture that differed in anything else
// would let a bug in the lifecycle checks pass as the restriction doing its job.
func phieuLinhVuc(tt domain.TrangThai, linhVuc string) map[string]driver.Value {
	return dongPhieuMau(map[string]any{
		"trang_thai":      string(tt),
		"linh_vuc":        linhVuc,
		"han_xu_ly_xong":  mocXuLyXongThu,
		"phan_loai_luc":   mocThaoTac,
		"bo_phan_id":      "bp-001",
		"can_bo_xu_ly_id": maCanBoThu,
	})
}

// hanhViPhieu is one of the four staff acts, with the status its fixture must start from.
//
// `quyenCaXa` IS TRUE ON THE ADVANCE ACT ON PURPOSE. The caller of that act therefore holds
// `feedback.resolve`, the commune-wide right to work on ANY petition — the widest account this
// service has — and is STILL refused on a `can-bo` petition. Anything narrower would leave the
// question "does the commune-wide right also open the restricted field" unanswered, and that is the
// account most likely to be used on this route.
type hanhViPhieu struct {
	trangThai domain.TrangThai
	chay      func(uc *XuLyPhanAnh, ctx context.Context, hanChe QuyenXemHanChe) error
}

func bonHanhVi() map[string]hanhViPhieu {
	return map[string]hanhViPhieu{
		"phân loại": {domain.DaTiepNhan, func(uc *XuLyPhanAnh, ctx context.Context, q QuyenXemHanChe) error {
			// THE FIELD BEING SETTLED IS `rac-thai`, WHICH IS THE INTERESTING DIRECTION: re-filing a
			// report about a member of staff under an ordinary field would make it visible to the whole
			// register. The refusal reads the field the ROW holds, not the one the request names.
			_, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "rac-thai"}, canBoThu(), q)
			return err
		}},
		"phân công": {domain.DangPhanLoai, func(uc *XuLyPhanAnh, ctx context.Context, q QuyenXemHanChe) error {
			_, err := uc.PhanCong(ctx, maPhieuThu, YeuCauPhanCong{BoPhan: "bp-002"}, canBoThu(), q)
			return err
		}},
		"chuyển trạng thái": {domain.DaChuyenXuLy, func(uc *XuLyPhanAnh, ctx context.Context, q QuyenXemHanChe) error {
			_, err := uc.TienTrangThai(ctx, maPhieuThu, canBoThu(), coQuyenCaXa, q)
			return err
		}},
		"đóng phiếu": {domain.ChoDanXacNhan, func(uc *XuLyPhanAnh, ctx context.Context, q QuyenXemHanChe) error {
			_, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, canBoThu(), q)
			return err
		}},
	}
}

// TestBonHanhViVaLinhVucHanChe is the four-cell table, on all four acts — sixteen cases.
//
//	phiếu là `can-bo` | có feedback.restricted | mong đợi
//	không             | không                  | như cũ        ← the negative half: a patch that
//	không             | có                     | như cũ          locked the whole commune turns these red
//	CÓ                | KHÔNG                  | TỪ CHỐI, KHÔNG GHI GÌ  ← the leak this closes
//	có                | có                     | như cũ
//
// THE THIRD CELL ASSERTS THE COUNTS AND NOT ONLY THE ERROR. A refusal that had already written the
// UPDATE and then rolled back would be correct today and would stop being correct the first time
// somebody moved the check below a write — and no error-only assertion could tell the two apart.
//
// ĐỘT BIẾN: xoá lời gọi duocChamPhieuHanChe ở tầng app và ô thứ ba ĐỎ trên cả bốn hành vi.
func TestBonHanhViVaLinhVucHanChe(t *testing.T) {
	for tenHanhVi, hv := range bonHanhVi() {
		for tenO, ca := range map[string]struct {
			linhVuc string
			quyen   QuyenXemHanChe
			lamDuoc bool
		}{
			"lĩnh vực thường + không quyền hạn chế": {"rac-thai", khongQuyenHanChe, true},
			"lĩnh vực thường + có quyền hạn chế":    {"rac-thai", coQuyenHanChe, true},
			"LĨNH VỰC HẠN CHẾ + KHÔNG quyền":        {domain.LinhVucHanChe, khongQuyenHanChe, false},
			"lĩnh vực hạn chế + có quyền":           {domain.LinhVucHanChe, coQuyenHanChe, true},
		} {
			t.Run(tenHanhVi+"/"+tenO, func(t *testing.T) {
				k := khoPhieuMau()
				k.hang = phieuLinhVuc(hv.trangThai, ca.linhVuc)
				uc, ctx := dungXuLy(t, k, hanXuLyThu())

				err := hv.chay(uc, ctx, ca.quyen)

				if ca.lamDuoc {
					if err != nil {
						t.Fatalf("%s: %v — phép kiểm lĩnh vực hạn chế khoá cả những phiếu nó không "+
							"được phép khoá, và một xã không xử lý được phiếu nào là hỏng nặng hơn "+
							"cái nó vá", tenHanhVi, err)
					}
					return
				}
				if !errors.Is(err, ErrPhieuHanChe) {
					t.Fatalf("lỗi = %v, muốn ErrPhieuHanChe — đồng nghiệp của người bị nêu tên đang "+
						"xử lý chính đơn tố cáo về người ấy", err)
				}
				if k.coCau("UPDATE phieu_phan_anh") || k.coCau("INSERT INTO audit_log") ||
					k.coCau("INSERT INTO su_kien_di") {
					t.Error("đã ghi dù từ chối — từ chối phải KHÔNG ghi gì: không câu UPDATE, không " +
						"vết kiểm toán, không sự kiện")
				}
				if k.daCommit != 0 {
					t.Errorf("commit=%d, muốn 0", k.daCommit)
				}
			})
		}
	}
}

// TestPhanLoaiPhieuHanCheKhongHoiIdentity — the refusal happens BEFORE the gRPC call, and the ORDER
// is what this case pins rather than the refusal itself.
//
// Two things ride on it. The lesser: a refused act must not become load on another service. The one
// that matters: the check below the identity call is a 409 answering "xã chưa cấu hình thời hạn" —
// the ORDINARY answer in every commune today — and that sentence is a statement about a record
// existing under this code. Said to a colleague of the person being reported on, it is the
// disclosure the 404 exists to prevent.
func TestPhanLoaiPhieuHanCheKhongHoiIdentity(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	k.hang = phieuLinhVuc(domain.DaTiepNhan, domain.LinhVucHanChe)
	uc, ctx := dungXuLy(t, k, han)

	_, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "rac-thai"},
		canBoThu(), khongQuyenHanChe)

	if !errors.Is(err, ErrPhieuHanChe) {
		t.Fatalf("lỗi = %v, muốn ErrPhieuHanChe", err)
	}
	if han.goi != 0 {
		t.Errorf("hỏi identity %d lần dù sẽ từ chối", han.goi)
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch dù sẽ từ chối", k.batDau)
	}
}

// TestTienTrangThaiPhieuHanCheTraLoiHanCheChuKhongPhaiKhongDuocGiao pins WHICH refusal wins when two
// apply at once.
//
// The caller is neither the assignee nor a holder of the commune-wide right, AND the petition is
// restricted. Both rules refuse — but they answer differently at the edge: ErrKhongPhaiNguoiDuocGiao
// becomes a 403 that names the reason, ErrPhieuHanChe becomes a 404 identical to an unknown code.
// The weaker disclosure has to win, or the restriction leaks through the more talkative refusal.
func TestTienTrangThaiPhieuHanCheTraLoiHanCheChuKhongPhaiKhongDuocGiao(t *testing.T) {
	k := khoPhieuMau()
	k.hang = phieuLinhVuc(domain.DaChuyenXuLy, domain.LinhVucHanChe)
	k.hang["can_bo_xu_ly_id"] = "CB-99999" // giao cho người khác
	uc, ctx := dungXuLy(t, k, hanXuLyThu())

	_, err := uc.TienTrangThai(ctx, maPhieuThu, canBoThu(), khongQuyenCaXa, khongQuyenHanChe)

	if !errors.Is(err, ErrPhieuHanChe) {
		t.Fatalf("lỗi = %v, muốn ErrPhieuHanChe — 403 'phiếu không được giao cho bạn' xác nhận rằng "+
			"CÓ một phiếu dưới mã này, nói với chính đồng nghiệp của người bị nêu tên", err)
	}
}

// --- rule 1: the commune is $1, from the context ----------------------------------------------------

// TestXuLyTheoXaTrongContext — THE SAME USE CASE, TWO COMMUNES, TWO CONTEXTS. A use case that had
// captured a commune at construction, or taken one as an argument, would send the same $1 twice and
// no test of a single commune could tell.
func TestXuLyTheoXaTrongContext(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	uc, _ := dungXuLy(t, k, han)

	if _, err := uc.ChotLinhVuc(ctxXa(xaKia), maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "rac-thai"}, canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("ChotLinhVuc: %v", err)
	}
	for _, l := range k.lenh {
		if len(l.args) == 0 {
			continue
		}
		if s, ok := l.args[0].(string); ok && s != string(xaKia) {
			t.Errorf("câu lệnh %q mang xã %q, muốn %q", l.sql, s, xaKia)
		}
	}
}

// TestKhongUpdateNaoChamCotBatBien — the three deadline columns, the origin and the lookup code are
// fixed by the acts that set them and by the archival trigger. Their ABSENCE from these statements is
// what keeps that floor unreachable from this service.
func TestKhongUpdateNaoChamCotBatBien(t *testing.T) {
	k, han := khoPhieuMau(), hanXuLyThu()
	uc, ctx := dungXuLy(t, k, han)
	if _, err := uc.ChotLinhVuc(ctx, maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "rac-thai"}, canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("ChotLinhVuc: %v", err)
	}

	for _, l := range k.cau("UPDATE phieu_phan_anh") {
		for _, cot := range []string{"ma_tra_cuu =", "goc_dem_han =", "kenh_tiep_nhan =",
			"han_tiep_nhan =", "han_phan_loai ="} {
			if strings.Contains(l.sql, cot) {
				t.Errorf("câu UPDATE ghi vào cột bất biến %q: %q", cot, l.sql)
			}
		}
	}
}

// --- helpers ------------------------------------------------------------------------------------------

// coThamSo reports whether a statement was given this exact bound value.
//
// BY VALUE AND NOT BY POSITION, so an assertion survives a column being added to a statement — the
// thing it must not survive is the value going missing.
func coThamSo(args []driver.Value, muon string) bool {
	for _, a := range args {
		if s, ok := a.(string); ok && s == muon {
			return true
		}
	}
	return false
}

// thanSuKien decodes the protojson body the outbox row carries.
//
// `args[4]` IS THE BODY, and the index is the store's INSERT order (tenant, id, ten, doi_tuong,
// than, xay_ra_luc). It is asserted as []byte rather than assumed: a body that reached the column as
// a string would still store, and the field-name spelling the contract requires would be invisible.
func thanSuKien(t *testing.T, args []driver.Value) map[string]any {
	t.Helper()
	b, ok := args[4].([]byte)
	if !ok {
		t.Fatalf("tham số thân sự kiện không phải []byte: %T", args[4])
	}
	var ra map[string]any
	if err := json.Unmarshal(b, &ra); err != nil {
		t.Fatalf("thân sự kiện không phải JSON: %q", string(b))
	}
	return ra
}
