package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The CITIZEN intake use case — the act rule 10 exists for.
//
//	PROVED HERE   the acknowledge deadline is asked for ONCE, before anything is produced, for
//	              `phan-anh`, for the DEFAULT row, for the acknowledge clock ALONE, counted from the
//	              instant stored as `goc_dem_han` · a refusal from identity ends the intake with NO
//	              row, NO code and NO commitment · the row and its audit entry share ONE COMMITTED
//	              transaction · the entry carries Kind="citizen" and the citizen's own id · the
//	              delta holds not one character of what the citizen typed · `han_xu_ly_xong` stays
//	              NULL and `linh_vuc` stays empty · the owner is the ACTOR and there is no argument
//	              that could make it anything else.
//
//	NOT PROVED    ANYTHING PostgreSQL DOES. The fake driver accepts any SQL, so the four CHECK
//	              constraints of migration 0004, the hash partition routing and
//	              `UNIQUE (tenant_id, ma_tra_cuu)` are invisible here. internal/store's pg suite is
//	              where those live, and it SKIPS without VIGOV_TEST_DSN.

// --- fakes -----------------------------------------------------------------------------------

// hanGia is identity's answer, and it RECORDS WHAT IT WAS ASKED. The arguments are the whole of
// ADR 0028 decision E expressed as a call, so asserting on them is asserting on the decision.
type hanGia struct {
	tra map[identityv1.DeadlineKind]time.Time
	loi error

	goi        int
	thayLoai   []identityv1.WorkKind
	thayLinh   []string
	thayTuLuc  []time.Time
	thayCanMoc [][]identityv1.DeadlineKind

	// The CLASSIFICATION CEILING half — ADR 0035 §C. It is a DIFFERENT RPC because the ceiling is not
	// a row of the `sla` table, and this fake records what it was asked for the same reason as above:
	// the arguments are the decision.
	traTran     map[uint32]time.Time
	loiTran     error
	goiTran     int
	thayTranTu  []time.Time
	thayTranGio [][]uint32
}

func (h *hanGia) TienGioLamViec(_ context.Context, tuLuc time.Time, gio []uint32) (
	map[uint32]time.Time, error) {

	h.goiTran++
	h.thayTranTu = append(h.thayTranTu, tuLuc)
	h.thayTranGio = append(h.thayTranGio, gio)
	if h.loiTran != nil {
		return nil, h.loiTran
	}
	return h.traTran, nil
}

func (h *hanGia) HanXuLy(_ context.Context, loai identityv1.WorkKind, linhVuc string,
	tuLuc time.Time, can []identityv1.DeadlineKind) (map[identityv1.DeadlineKind]time.Time, error) {

	h.goi++
	h.thayLoai = append(h.thayLoai, loai)
	h.thayLinh = append(h.thayLinh, linhVuc)
	h.thayTuLuc = append(h.thayTuLuc, tuLuc)
	h.thayCanMoc = append(h.thayCanMoc, can)
	if h.loi != nil {
		return nil, h.loi
	}
	return h.tra, nil
}

// khoPhieuGia records the row it was handed, INSIDE the transaction it was handed.
//
// It also runs a real statement through the fake driver, so the assertion "the business write and
// the audit entry are in the SAME transaction" is made against two statements the driver saw rather
// than against a boolean this fake set.
type khoPhieuGia struct {
	thay []domain.PhieuPhanAnh
	loi  error
}

func (k *khoPhieuGia) Tao(ctx context.Context, tx *pkgstore.ScopedTx, p domain.PhieuPhanAnh) error {
	k.thay = append(k.thay, p)
	if _, err := tx.Exec(ctx,
		`INSERT INTO phieu_phan_anh (tenant_id, id, ma_tra_cuu) VALUES ($1,$2,$3)`,
		string(tx.TenantID()), p.ID, p.MaTraCuu); err != nil {
		return err
	}
	return k.loi
}

// --- fixtures --------------------------------------------------------------------------------

const (
	idCongDan = "cd-01JCONGDANMODINHDANH"
	maCoDinh  = "PA-4K7M-92XR-BTVD"
	idCoDinh  = "01JPHIEUCODINHTRONGTEST"
)

var (
	// mocGui is the instant the citizen pressed send, and mocHan the acknowledge deadline identity
	// computed from it. THE TWO ARE DELIBERATELY NOT round numbers of hours apart: a deadline in
	// working hours lands wherever the commune's calendar puts it, and a fixture 8 hours later would
	// let a local `.Add(8 * time.Hour)` pass every assertion here.
	mocGuiThu = time.Date(2026, 9, 22, 7, 14, 3, 0, time.UTC)
	mocHanThu = time.Date(2026, 9, 23, 2, 30, 0, 0, time.UTC)

	// mocTranThu is the classification ceiling identity computed — one working day, wherever this
	// commune's calendar puts it. DELIBERATELY NOT 24 HOURS AFTER mocGuiThu and deliberately NOT
	// equal to mocHanThu: a fixture at a round offset would let a local `.Add(24 * time.Hour)` pass,
	// and a fixture equal to the acknowledge deadline would let the two columns be swapped with
	// nothing turning red.
	mocTranThu = time.Date(2026, 9, 23, 9, 45, 0, 0, time.UTC)
)

func hanThu() *hanGia {
	return &hanGia{
		tra: map[identityv1.DeadlineKind]time.Time{
			identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN: mocHanThu,
		},
		traTran: map[uint32]time.Time{domain.GioTranPhanLoai: mocTranThu},
	}
}

func congDanThu() audit.Actor {
	return audit.Actor{ID: idCongDan, Kind: "citizen", IP: "10.0.0.9"}
}

func ycThu() YeuCauGuiPhanAnh {
	return YeuCauGuiPhanAnh{
		NoiDung:   "Đống rác ở đầu ngõ đã ba ngày chưa ai dọn.",
		DiaChi:    "Đầu ngõ thôn Hà Lam",
		HoTen:     "Nguyễn Văn An",
		DienThoai: "0900000000", // the agreed fake number (rule 3, invariant 5)
	}
}

// dungGui builds the use case with BOTH generators and the clock pinned, so every assertion below
// is about the decision and not about randomness.
func dungGui(k *khoGia, kho *khoPhieuGia, han HanTiepNhanDoc) *GuiPhanAnh {
	uc := NewGuiPhanAnh(pkgstore.New(sql.OpenDB(k)), kho, han)
	uc.sinhID = func() (string, error) { return idCoDinh, nil }
	uc.sinhMa = func() (string, error) { return maCoDinh, nil }
	uc.luc = func() time.Time { return mocGuiThu }
	return uc
}

// --- the commitment is asked for, once, for the right thing ------------------------------------

// TestGuiHoiHanDungMotLanVaDungCauHoi asserts the four arguments ADR 0028 decision E decides.
//
// ĐỘT BIẾN: đổi `""` thành một mã lĩnh vực bất kỳ, hoặc thêm DEADLINE_KIND_XU_LY_XONG vào danh sách
// đồng hồ, và ca này ĐỎ — cái thứ hai dựng lại đúng trần 56 giờ mà ADR 0028 đã gỡ.
func TestGuiHoiHanDungMotLanVaDungCauHoi(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

	if _, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), congDanThu()); err != nil {
		t.Fatalf("Gui: %v", err)
	}

	if han.goi != 1 {
		t.Fatalf("hỏi hạn %d lần, muốn 1 — hạn được ấn định MỘT LẦN tại hành vi tạo dòng "+
			"(luật 10 bất biến 2)", han.goi)
	}
	if han.thayLoai[0] != identityv1.WorkKind_WORK_KIND_PHAN_ANH {
		t.Errorf("loại việc = %v, muốn WORK_KIND_PHAN_ANH — một loại khác đọc số giờ của nghiệp vụ khác",
			han.thayLoai[0])
	}
	if han.thayLinh[0] != "" {
		t.Errorf("linh_vuc = %q, muốn \"\" (DÒNG MẶC ĐỊNH) — lúc này chưa ai biết lĩnh vực, "+
			"và để công dân chọn là câu mở #23 đã đóng theo chiều ngược lại", han.thayLinh[0])
	}
	if !han.thayTuLuc[0].Equal(mocGuiThu) {
		t.Errorf("gốc đếm = %v, muốn %v (lúc công dân bấm gửi)", han.thayTuLuc[0], mocGuiThu)
	}
	moc := han.thayCanMoc[0]
	if len(moc) != 1 || moc[0] != identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN {
		t.Errorf("đồng hồ đã hỏi = %v, muốn CHỈ DEADLINE_KIND_TIEP_NHAN — hỏi thêm hạn xử lý xong "+
			"là ấn định nó từ dòng mặc định ngay lúc tiếp nhận, đúng cái trần ADR 0028 đã gỡ", moc)
	}
}

// TestGuiLuuHanTiepNhanDaTinh is rule 10, invariant 2: the deadline identity returned is STORED, as
// given, and nothing else is written into that column.
//
// ĐỘT BIẾN BẮT BUỘC (a): bỏ dòng `HanTiepNhan: hanTiepNhan` khỏi bản ghi và ca này ĐỎ.
func TestGuiLuuHanTiepNhanDaTinh(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

	p, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), congDanThu())
	if err != nil {
		t.Fatalf("Gui: %v", err)
	}
	if len(kho.thay) != 1 {
		t.Fatalf("ghi %d dòng, muốn 1", len(kho.thay))
	}
	dong := kho.thay[0]

	if !dong.HanTiepNhan.Equal(mocHanThu) {
		t.Errorf("han_tiep_nhan đã ghi = %v, muốn %v — mốc do identity tính, lưu NGUYÊN VĂN",
			dong.HanTiepNhan, mocHanThu)
	}
	if dong.HanTiepNhanKhongApDung() {
		t.Error("han_tiep_nhan rỗng -> SQL NULL -> \"KHÔNG ÁP DỤNG\": phiếu của công dân bị ghi " +
			"thành phiếu không ai nợ một lời tiếp nhận")
	}
	// THE OTHER NULL KEEPS THE OPPOSITE MEANING. `han_xu_ly_xong` is fixed by the act that settles
	// the field (ADR 0028 decision E), never here.
	if !dong.ChuaChotHanXuLy() {
		t.Errorf("han_xu_ly_xong = %v — tuyến công dân KHÔNG ấn định hạn xử lý xong", dong.HanXuLyXong)
	}
	if dong.LinhVuc != "" {
		t.Errorf("linh_vuc = %q — công dân không chọn lĩnh vực", dong.LinhVuc)
	}
	if !dong.GocDemHan.Equal(mocGuiThu) || !dong.VaoSoLuc.Equal(mocGuiThu) {
		t.Errorf("goc_dem_han = %v, vao_so_luc = %v, muốn cả hai = %v",
			dong.GocDemHan, dong.VaoSoLuc, mocGuiThu)
	}
	if dong.Kenh != domain.KenhZaloMiniApp {
		t.Errorf("kenh_tiep_nhan = %q, muốn %q", dong.Kenh, domain.KenhZaloMiniApp)
	}
	if dong.TrangThai != domain.DaTiepNhan {
		t.Errorf("trang_thai = %q, muốn %q", dong.TrangThai, domain.DaTiepNhan)
	}
	if dong.HienCongKhai {
		t.Error("hien_cong_khai = true — phiếu chưa ai đọc không được lên trang công khai")
	}
	if p.MaTraCuu != maCoDinh {
		t.Errorf("mã tra cứu trả về = %q, muốn %q", p.MaTraCuu, maCoDinh)
	}
}

// --- xã chưa cấu hình SLA: HỎNG LƯỢT TIẾP NHẬN, không dòng, không mã --------------------------

// TestGuiXaChuaCauHinhSLAThiKhongGhiGiVaKhongCapMa is the case that is TODAY'S ORDINARY ANSWER for
// every commune: migration 0008 seeds nothing, so `sla` is empty everywhere.
//
// THE THREE ASSERTIONS ARE THREE DIFFERENT PROMISES:
//
//	no statement reached the database   the record was not created (and neither was a trail)
//	no lookup code in the returned value  rule 7, invariant 3 — an issued code is never reissued,
//	                                      so a code handed out on a failed intake is burned forever
//	                                      and the citizen holds a slip for a petition that does not
//	                                      exist (rule 10, invariant 1 inverted)
//	the error is ErrChuaAnDinhDuocHan     the caller can answer "the channel is not open" rather
//	                                      than "something went wrong"
//
// ĐỘT BIẾN BẮT BUỘC (b): bỏ `return` sau lỗi của HanXuLy — tức cho tạo dòng khi không có hạn — và
// ca này ĐỎ ở cả ba phép so.
func TestGuiXaChuaCauHinhSLAThiKhongGhiGiVaKhongCapMa(t *testing.T) {
	k, kho := &khoGia{}, &khoPhieuGia{}
	han := hanThu()
	// The shape core/identityclient really returns for FAILED_PRECONDITION.
	han.loi = errors.New("identityclient: ResolveDeadlines: rpc error: code = FailedPrecondition " +
		"desc = xã chưa cấu hình bảng thời hạn xử lý")

	p, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), congDanThu())

	if err == nil {
		t.Fatal("NHẬN phiếu dù xã chưa có cam kết nào — phần mềm vừa tự hứa thay cơ quan nhà nước")
	}
	if !errors.Is(err, ErrChuaAnDinhDuocHan) {
		t.Errorf("lỗi = %v, muốn bọc ErrChuaAnDinhDuocHan", err)
	}
	if !errors.Is(err, han.loi) {
		t.Errorf("lỗi không bọc nguyên nhân gốc bằng %%w: %v", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("chạm cơ sở dữ liệu %d lần dù lượt tiếp nhận đã hỏng", len(k.lenh))
	}
	if len(kho.thay) != 0 {
		t.Errorf("ghi %d dòng dù lượt tiếp nhận đã hỏng", len(kho.thay))
	}
	if p.MaTraCuu != "" {
		t.Errorf("CẤP MÃ TRA CỨU %q cho một lượt tiếp nhận đã hỏng — mã đã phát ra "+
			"thì không bao giờ phát lại (luật 7 bất biến 3)", p.MaTraCuu)
	}
	if k.commit != 0 {
		t.Errorf("commit %d lần", k.commit)
	}
}

// TestGuiHopDongTraHanRongThiTuChoi — an item with no instant reaches the caller as the zero
// time.Time, which becomes SQL NULL, which on this column means "KHÔNG ÁP DỤNG". A petition of a
// citizen silently recorded as one nobody owes an acknowledgement for.
func TestGuiHopDongTraHanRongThiTuChoi(t *testing.T) {
	k, kho := &khoGia{}, &khoPhieuGia{}
	han := &hanGia{tra: map[identityv1.DeadlineKind]time.Time{}} // no error, no item

	_, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), congDanThu())
	if !errors.Is(err, ErrChuaAnDinhDuocHan) {
		t.Fatalf("lỗi = %v, muốn ErrChuaAnDinhDuocHan", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("ghi %d câu lệnh dù không có hạn", len(k.lenh))
	}
}

// TestGuiLoiHanKhongLoDuLieuCaNhan — the error travels into centralised logging across every
// commune at once (rule 3, invariants 1 and 2).
func TestGuiLoiHanKhongLoDuLieuCaNhan(t *testing.T) {
	k, kho := &khoGia{}, &khoPhieuGia{}
	han := hanThu()
	han.loi = errors.New("rpc error: code = Unavailable")

	_, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), congDanThu())
	if err == nil {
		t.Fatal("không từ chối")
	}
	for _, bi := range []string{"Nguyễn Văn An", "0900000000", "Đống rác", idCongDan, maCoDinh} {
		if strings.Contains(err.Error(), bi) {
			t.Errorf("thông điệp lỗi mang %q: %v", bi, err)
		}
	}
	if !strings.Contains(err.Error(), string(xaThu)) {
		t.Errorf("thông điệp lỗi không nói xã nào — thứ DUY NHẤT người vận hành hành động được: %v", err)
	}
}

// --- một giao dịch, hai câu lệnh ---------------------------------------------------------------

// TestGuiGhiDongVaVetTrongCUNGMotGiaoDich is rule 6, invariant 3 — the invariant this whole
// repository was shaped around, because the previous system had zero transactions and so "every
// write leaves a trail" could not hold.
//
// ĐỘT BIẾN BẮT BUỘC (d): bỏ lời gọi audit.Write trong khối giao dịch và ca này ĐỎ.
func TestGuiGhiDongVaVetTrongCUNGMotGiaoDich(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

	if _, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), congDanThu()); err != nil {
		t.Fatalf("Gui: %v", err)
	}

	if len(k.lenh) != 2 {
		t.Fatalf("chạy %d câu lệnh, muốn 2 (dòng nghiệp vụ + vết): %v", len(k.lenh), k.lenh)
	}
	if !strings.Contains(k.lenh[0].sql, "INSERT INTO phieu_phan_anh") {
		t.Errorf("câu lệnh đầu không phải ghi phiếu: %q", k.lenh[0].sql)
	}
	if !strings.Contains(k.lenh[1].sql, "INSERT INTO audit_log") {
		t.Errorf("câu lệnh sau không phải ghi vết: %q", k.lenh[1].sql)
	}
	for i, l := range k.lenh {
		if !l.trongGiaoDich {
			t.Errorf("câu lệnh %d chạy NGOÀI giao dịch — luật 6 bất biến 3 và cấm #2: %q", i, l.sql)
		}
	}
	if k.commit != 1 {
		t.Errorf("commit %d lần, muốn 1 — một vết chưa commit là một vết không tồn tại", k.commit)
	}
	if k.rollback != 0 {
		t.Errorf("rollback %d lần trên đường thành công", k.rollback)
	}
}

// TestGuiVetHongThiKhongCoPhieu — the other direction of the same invariant. If the trail cannot be
// written, the petition must not exist: a record with no trail is a state the records rules do not
// permit (rule 2, invariant 6).
func TestGuiVetHongThiKhongCoPhieu(t *testing.T) {
	k := &khoGia{loi: errors.New("pg: relation audit_log is read only")}
	kho, han := &khoPhieuGia{}, hanThu()

	_, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), congDanThu())
	if err == nil {
		t.Fatal("Gui nuốt lỗi — phiếu tồn tại mà không có vết")
	}
	if k.commit != 0 {
		t.Errorf("commit %d lần dù ghi hỏng", k.commit)
	}
	if k.rollback != 1 {
		t.Errorf("rollback %d lần, muốn 1", k.rollback)
	}
}

// --- người gây là CÔNG DÂN, và đó cũng là chủ của phiếu ----------------------------------------

// TestGuiVetMangChuTheCongDan checks the bound arguments of audit.Write by position:
//
//	$1 tenant_id · $2 actor_id · $3 actor_kind · $4 actor_ip · $5 action · $6 subject · $7 at · $8 delta
func TestGuiVetMangChuTheCongDan(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

	if _, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), congDanThu()); err != nil {
		t.Fatalf("Gui: %v", err)
	}
	args := k.lenh[1].args
	if len(args) != 8 {
		t.Fatalf("%d tham số cho vết, muốn 8: %v", len(args), args)
	}

	if args[0] != string(xaThu) {
		t.Errorf("tenant_id = %v, muốn %q (từ context, không từ tham số)", args[0], xaThu)
	}
	if args[1] != idCongDan {
		t.Errorf("actor_id = %v, muốn %q", args[1], idCongDan)
	}
	if args[2] != "citizen" {
		t.Errorf("actor_kind = %v, muốn %q — một hành vi của công dân ghi thành của cán bộ "+
			"làm cả sổ vết không trả lời được 'ai làm'", args[2], "citizen")
	}
	if args[3] != "10.0.0.9" {
		t.Errorf("actor_ip = %v, muốn %q (luật 6 bất biến 2)", args[3], "10.0.0.9")
	}
	if args[4] != HanhViGuiPhanAnh {
		t.Errorf("action = %v, muốn %q", args[4], HanhViGuiPhanAnh)
	}
	if args[5] != maCoDinh {
		t.Errorf("subject = %v, muốn %q (MÃ NGHIỆP VỤ, không phải ULID nội bộ)", args[5], maCoDinh)
	}
}

// TestGuiChuPhieuLaCHUTHE is rule 4, invariant 2 held at the TYPE level: YeuCauGuiPhanAnh has no
// CongDanID field, so there is no value a handler could pass that would make the owner anything
// other than the actor the session produced.
//
// ĐỘT BIẾN BẮT BUỘC (c) sống ở tầng HTTP — đây là nửa dưới: thêm một trường CongDanID vào
// YeuCauGuiPhanAnh và ca này KHÔNG biên dịch được.
func TestGuiChuPhieuLaCHUTHE(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

	if _, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), congDanThu()); err != nil {
		t.Fatalf("Gui: %v", err)
	}
	if kho.thay[0].CongDanID != idCongDan {
		t.Errorf("cong_dan_id đã ghi = %q, muốn %q (định danh CỦA PHIÊN)",
			kho.thay[0].CongDanID, idCongDan)
	}
}

func TestGuiChuTheKhongPhaiCongDanThiTuChoiTruocMoiThu(t *testing.T) {
	for ten, nguoi := range map[string]audit.Actor{
		"không có chủ thể": {},
		// @actor-ok: DỮ LIỆU THỬ CỐ Ý SAI, không phải một lượt ghi vết. Chính ca này khẳng định
		// tuyến công dân TỪ CHỐI một chủ thể cán bộ trước khi chạm vào bất cứ thứ gì, nên định
		// danh nội bộ ở đây là thứ đang BỊ từ chối. Đổi nó sang một mã cán bộ hợp lệ để rào chắn
		// im đi là làm chính ca kiểm yếu hẳn mà vẫn xanh.
		"chủ thể là cán bộ": {ID: "nd-01JCANBO", Kind: "staff", IP: "10.0.0.7"},
		"thiếu định danh":   {Kind: "citizen", IP: "10.0.0.9"},
	} {
		t.Run(ten, func(t *testing.T) {
			k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

			if _, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), nguoi); err == nil {
				t.Fatal("nhận phiếu mà không có công dân đứng tên")
			}
			if han.goi != 0 {
				t.Errorf("hỏi identity %d lần dù chủ thể đã sai", han.goi)
			}
			if len(k.lenh) != 0 {
				t.Errorf("chạm cơ sở dữ liệu %d lần dù chủ thể đã sai", len(k.lenh))
			}
		})
	}
}

// --- vết không mang một chữ nào công dân đã gõ -------------------------------------------------

// TestGuiDeltaKhongMangDuLieuCaNhan is rule 6, forbidden #4 taken further than the rule requires:
// the delta carries no personal value at all, not even a masked one. The reasoning — and why
// masked-but-present was rejected rather than overlooked — is on the delta itself.
func TestGuiDeltaKhongMangDuLieuCaNhan(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

	if _, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), congDanThu()); err != nil {
		t.Fatalf("Gui: %v", err)
	}
	tho, ok := k.lenh[1].args[7].([]byte)
	if !ok {
		t.Fatalf("delta = %T, muốn []byte", k.lenh[1].args[7])
	}

	for _, bi := range []string{"Nguyễn Văn An", "0900000000", "Đống rác", "Hà Lam", "09****"} {
		if strings.Contains(string(tho), bi) {
			t.Errorf("delta mang dữ liệu cá nhân (%q) — sổ vết là bảng append-only không bao giờ "+
				"xoá, tức nơi tệ nhất để dữ liệu cá nhân tích lại: %s", bi, tho)
		}
	}

	var d struct {
		Kenh         string    `json:"kenh_tiep_nhan"`
		TrangThai    string    `json:"trang_thai"`
		AnDanh       bool      `json:"an_danh"`
		HanTiepNhan  time.Time `json:"han_tiep_nhan"`
		TruongDaDien []string  `json:"truong_da_dien"`
		DoDai        int       `json:"do_dai_noi_dung"`
	}
	if err := json.Unmarshal(tho, &d); err != nil {
		t.Fatalf("delta không phải JSON: %q", tho)
	}
	if d.Kenh != string(domain.KenhZaloMiniApp) || d.TrangThai != string(domain.DaTiepNhan) {
		t.Errorf("delta không nói kênh và trạng thái: %s", tho)
	}
	// THE COMMITMENT IS IN THE ENTRY, and that is what an inspection asks: what did the authority
	// promise, and by when. It is an instant, not personal data.
	if !d.HanTiepNhan.Equal(mocHanThu) {
		t.Errorf("delta không mang hạn đã cam kết: %s", tho)
	}
	if len(d.TruongDaDien) != 3 {
		t.Errorf("truong_da_dien = %v, muốn ba trường đã điền (tên cột, không phải giá trị)", d.TruongDaDien)
	}
	if d.DoDai != len([]rune(ycThu().NoiDung)) {
		t.Errorf("do_dai_noi_dung = %d", d.DoDai)
	}
}

// --- xã đến từ context, không bao giờ từ tham số -----------------------------------------------

// TestGuiTheoXaTrongContext: the SAME use case object, two communes, two rows and two entries, each
// bound to the commune of its own request. A use case that cached the commune from the first call
// would pass every test above and file one commune's petition into another.
func TestGuiTheoXaTrongContext(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()
	uc := dungGui(k, kho, han)

	for _, xa := range []tenant.ID{xaThu, xaKia} {
		if _, err := uc.Gui(ctxXa(xa), ycThu(), congDanThu()); err != nil {
			t.Fatalf("Gui cho %s: %v", xa, err)
		}
	}
	if len(k.lenh) != 4 {
		t.Fatalf("chạy %d câu lệnh, muốn 4", len(k.lenh))
	}
	// lenh[0..1] = commune A's row and entry, lenh[2..3] = commune B's.
	for i, muon := range map[int]tenant.ID{0: xaThu, 1: xaThu, 2: xaKia, 3: xaKia} {
		if k.lenh[i].args[0] != string(muon) {
			t.Errorf("tenant_id của câu lệnh %d = %v, muốn %q", i, k.lenh[i].args[0], muon)
		}
	}
}

func TestGuiKhongCoXaThiPanicChuKhongMacDinh(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("nhận được phiếu mà không có xã trong context — một mặc định trên đường " +
				"cách ly là luật 1 cấm #1")
		}
	}()
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()
	_, _ = dungGui(k, kho, han).Gui(context.Background(), ycThu(), congDanThu())
}

// --- đầu vào sai bị từ chối TRƯỚC khi chạm identity và cơ sở dữ liệu --------------------------

func TestGuiNoiDungRongThiTuChoiTruocMoiThu(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

	yc := ycThu()
	yc.NoiDung = "   "
	_, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), yc, congDanThu())

	if !errors.Is(err, domain.ErrNoiDungTrong) {
		t.Fatalf("lỗi = %v, muốn ErrNoiDungTrong", err)
	}
	if han.goi != 0 {
		t.Errorf("hỏi identity %d lần cho một yêu cầu sai hình dạng — tải lên một tiến trình "+
			"phục vụ 200+ xã", han.goi)
	}
	if len(k.lenh) != 0 {
		t.Errorf("chạm cơ sở dữ liệu %d lần dù đã từ chối", len(k.lenh))
	}
}

// TestGuiAnDanhVanLuuDanhTinh — ADR 0008: anonymity means hidden from staff screens and from the
// public page, NOT "identity not recorded". Without the identity there is no anti-spam, the citizen
// cannot find their own petition, and a defamatory report becomes untraceable.
func TestGuiAnDanhVanLuuDanhTinh(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

	yc := ycThu()
	yc.AnDanh = true
	if _, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), yc, congDanThu()); err != nil {
		t.Fatalf("Gui: %v", err)
	}

	dong := kho.thay[0]
	if !dong.AnDanh {
		t.Error("an_danh = false dù công dân đã chọn gửi ẩn danh")
	}
	if dong.CongDanID != idCongDan || dong.NguoiGuiHoTen == "" || dong.NguoiGuiDienThoai == "" {
		t.Errorf("phiếu ẩn danh bị XOÁ danh tính khỏi bản ghi: %+v", dong)
	}
}
