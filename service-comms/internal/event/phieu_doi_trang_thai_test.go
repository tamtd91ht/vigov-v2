package event

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/vihat/vigov/core/events"
	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// WHAT THIS FILE PROVES, and each item is a failure that is SILENT in production:
//
//  1. An envelope with no commune is REFUSED — and refused because Nhan goes through
//     core/events.Dispatch, not because a check was copied in here. Deleting the Dispatch call
//     turns this file red.
//  2. `occurrence` travels into `Lan` untouched, and 0 is refused rather than defaulted. Without
//     it the SECOND closing of a reopened petition (ADR 0008) is swallowed as a duplicate, and
//     from inside the system that is indistinguishable from correct deduplication.
//  3. An absent `citizen_message` writes NOTHING and says nothing — no ledger row, no log line.
//     It is a normal outcome on the six transitions that owe the citizen no message.
//  4. Nothing on any path logs the recipient id or anything else that reads as a person (rule 3).
//
// NOT PROVED HERE: anything the ledger does. The row, its audit entry and the transaction they
// share belong to internal/app and are proved there; this file asserts only what is handed over.

const (
	maPhieuThu   = "PA-2026-7F3K9Q"
	maCongDanThu = "01JCONGDANMAUTHUNGHIEM0001"
	maMauThu     = "ZNS-PHAN-ANH-DOI-TRANG-THAI"
	mocThu       = "da-chuyen-xu-ly"
)

var xaThu = tenant.ID("01JA" + strings.Repeat("A", 22))

// --- fakes --------------------------------------------------------------------------------------

type soGia struct {
	nhan []app.YeuCauGhiNo
	xa   []tenant.ID
	moi  bool
	loi  error
}

func (s *soGia) GhiNo(ctx context.Context, yc app.YeuCauGhiNo) (app.KetQuaGhiNo, error) {
	// The commune is read from the CONTEXT, which is the only place it may come from (rule 1,
	// invariant 4). Recording it here is what makes the missing-commune test prove something.
	s.xa = append(s.xa, tenant.MustFrom(ctx))
	s.nhan = append(s.nhan, yc)
	if s.loi != nil {
		return app.KetQuaGhiNo{}, s.loi
	}
	return app.KetQuaGhiNo{ID: "01JTHONGBAOMAUTHUNGHIEM01", Moi: s.moi}, nil
}

type mauGia struct {
	ma  string
	loi error
	hoi []string
}

func (m *mauGia) MauTin(_ context.Context, moc string) (string, error) {
	m.hoi = append(m.hoi, moc)
	if m.loi != nil {
		return "", m.loi
	}
	return m.ma, nil
}

// --- helpers ------------------------------------------------------------------------------------

func dungConsumer(t *testing.T) (*PhieuDoiTrangThai, *soGia, *mauGia, *bytes.Buffer) {
	t.Helper()
	so := &soGia{moi: true}
	mau := &mauGia{ma: maMauThu}
	var nhatKy bytes.Buffer
	log := slog.New(slog.NewTextHandler(&nhatKy, nil))
	return NewPhieuDoiTrangThai(so, mau, log), so, mau, &nhatKy
}

// tinMau is a well-formed transition that DOES owe the citizen a message.
func tinMau() *petitionsv1.PetitionStatusChanged {
	return &petitionsv1.PetitionStatusChanged{
		LookupCode: maPhieuThu,
		Status:     mocThu,
		Occurrence: 1,
		CitizenId:  maCongDanThu,
		CitizenMessage: &petitionsv1.CitizenMessage{
			StatusLabel: "Đã chuyển xử lý",
			NextStep:    "Bộ phận chuyên môn sẽ liên hệ trong thời hạn đã hẹn",
		},
	}
}

// phongBi encodes the payload EXACTLY as the contract says it travels: protojson with proto field
// names preserved. A test that hand-wrote the JSON would be testing its own spelling.
func phongBi(t *testing.T, tin *petitionsv1.PetitionStatusChanged) events.Envelope {
	t.Helper()
	b, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(tin)
	if err != nil {
		t.Fatalf("mã hoá payload: %v", err)
	}
	return events.Envelope{
		ID:       "01JPHONGBIMAUTHUNGHIEM001",
		Name:     TenSuKienPhieuDoiTrangThai,
		TenantID: xaThu,
		At:       time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC),
		Payload:  b,
	}
}

// --- rule 1: no commune, no work ----------------------------------------------------------------

func TestPhongBiThieuXaBiTuChoi(t *testing.T) {
	c, so, mau, nhatKy := dungConsumer(t)
	pb := phongBi(t, tinMau())
	pb.TenantID = ""

	err := c.Nhan(context.Background(), pb)
	if !errors.Is(err, events.ErrNoTenant) {
		t.Fatalf("muốn ErrNoTenant, nhận %v", err)
	}
	if len(so.nhan) != 0 {
		t.Fatalf("đã ghi sổ %d dòng cho một phong bì không có xã", len(so.nhan))
	}
	if len(mau.hoi) != 0 {
		t.Fatalf("đã hỏi mẫu tin cho một phong bì không có xã")
	}
	if nhatKy.Len() != 0 {
		t.Fatalf("có dòng nhật ký: %q", nhatKy.String())
	}
}

func TestXaTuPhongBiDiVaoContext(t *testing.T) {
	c, so, _, _ := dungConsumer(t)
	if err := c.Nhan(context.Background(), phongBi(t, tinMau())); err != nil {
		t.Fatalf("Nhan: %v", err)
	}
	if len(so.xa) != 1 || so.xa[0] != xaThu {
		t.Fatalf("xã trong context: %v, muốn %v", so.xa, xaThu)
	}
}

// --- the contract's own refusals ----------------------------------------------------------------

func TestTenSuKienKhacBiTuChoi(t *testing.T) {
	c, so, _, _ := dungConsumer(t)
	pb := phongBi(t, tinMau())
	pb.Name = "petitions.status_changed.v2"

	err := c.Nhan(context.Background(), pb)
	if !errors.Is(err, ErrSaiTenSuKien) || !errors.Is(err, ErrKhongThuLai) {
		t.Fatalf("muốn ErrSaiTenSuKien + ErrKhongThuLai, nhận %v", err)
	}
	if len(so.nhan) != 0 {
		t.Fatalf("đã ghi sổ cho một sự kiện không thuộc consumer này")
	}
}

func TestPayloadHongBiTuChoiVaKhongBocLoiGiaiMa(t *testing.T) {
	c, so, _, _ := dungConsumer(t)
	pb := phongBi(t, tinMau())
	// A body that fails to decode is, by definition, one that did not honour the contract — so it
	// is exactly the body that may carry what the contract forbids. `DAUVETRIENGBIET` stands in for
	// that content: the returned error must not repeat any of it (rule 3, forbidden #3).
	pb.Payload = []byte(`{"occurrence": "DAUVETRIENGBIET"}`)

	err := c.Nhan(context.Background(), pb)
	if !errors.Is(err, ErrGiaiMaPayload) || !errors.Is(err, ErrKhongThuLai) {
		t.Fatalf("muốn ErrGiaiMaPayload + ErrKhongThuLai, nhận %v", err)
	}
	if strings.Contains(err.Error(), "DAUVETRIENGBIET") {
		t.Fatalf("lỗi trả về nhắc lại nội dung payload: %q", err.Error())
	}
	if len(so.nhan) != 0 {
		t.Fatalf("đã ghi sổ cho một payload hỏng")
	}
}

func TestTruongMoiTrongPayloadVanDiQua(t *testing.T) {
	// Rule 2, invariant 4: the publisher may add an OPTIONAL field to a live event. A consumer that
	// refused unknown fields would turn that permitted change into every message failing.
	c, so, _, _ := dungConsumer(t)
	pb := phongBi(t, tinMau())
	pb.Payload = []byte(`{"lookup_code":"` + maPhieuThu + `","status":"` + mocThu + `",` +
		`"occurrence":1,"citizen_id":"` + maCongDanThu + `",` +
		`"citizen_message":{"status_label":"Đã chuyển xử lý","next_step":"Bộ phận chuyên môn sẽ liên hệ"},` +
		`"previous_status":"dang-phan-loai"}`)

	if err := c.Nhan(context.Background(), pb); err != nil {
		t.Fatalf("Nhan: %v", err)
	}
	if len(so.nhan) != 1 {
		t.Fatalf("ghi sổ %d dòng, muốn 1", len(so.nhan))
	}
}

func TestThieuTruongBatBuocBiTuChoi(t *testing.T) {
	cases := []struct {
		ten  string
		sua  func(*petitionsv1.PetitionStatusChanged)
		muon error
	}{
		{"thiếu mã tra cứu", func(t *petitionsv1.PetitionStatusChanged) { t.LookupCode = "" }, ErrThieuMaTraCuu},
		{"thiếu trạng thái", func(t *petitionsv1.PetitionStatusChanged) { t.Status = "" }, ErrThieuMoc},
		{"thiếu người nhận", func(t *petitionsv1.PetitionStatusChanged) { t.CitizenId = "" }, ErrThieuNguoiNhan},
		{"lần bằng 0", func(t *petitionsv1.PetitionStatusChanged) { t.Occurrence = 0 }, ErrLanKhongHopLe},
	}
	for _, tc := range cases {
		t.Run(tc.ten, func(t *testing.T) {
			c, so, _, _ := dungConsumer(t)
			tin := tinMau()
			tc.sua(tin)

			err := c.Nhan(context.Background(), phongBi(t, tin))
			if !errors.Is(err, tc.muon) {
				t.Fatalf("muốn %v, nhận %v", tc.muon, err)
			}
			if !errors.Is(err, ErrKhongThuLai) {
				t.Fatalf("một payload sai không tự đúng lên khi giao lại: %v", err)
			}
			if len(so.nhan) != 0 {
				t.Fatalf("đã ghi sổ cho một payload thiếu trường bắt buộc")
			}
		})
	}
}

// --- what gets handed to the ledger -------------------------------------------------------------

func TestGhiNoDuTungTruong(t *testing.T) {
	c, so, mau, _ := dungConsumer(t)
	if err := c.Nhan(context.Background(), phongBi(t, tinMau())); err != nil {
		t.Fatalf("Nhan: %v", err)
	}
	if len(so.nhan) != 1 {
		t.Fatalf("ghi sổ %d dòng, muốn 1", len(so.nhan))
	}
	muon := app.YeuCauGhiNo{
		DoiTuongLoai: domain.DoiTuongPhieuPhanAnh,
		DoiTuongMa:   maPhieuThu,
		Moc:          mocThu,
		Lan:          1,
		Kenh:         domain.KenhZaloZNS,
		NguoiNhanMa:  maCongDanThu,
		MauMa:        maMauThu,
		ThamSo: domain.ThamSoThongBao{
			MaTraCuu:     maPhieuThu,
			MocNhan:      "Đã chuyển xử lý",
			ViecTiepTheo: "Bộ phận chuyên môn sẽ liên hệ trong thời hạn đã hẹn",
		},
	}
	if so.nhan[0] != muon {
		t.Fatalf("yêu cầu ghi nợ:\n nhận %+v\n muốn %+v", so.nhan[0], muon)
	}
	// NguoiGay left zero: app.GhiNo turns it into the system principal (rule 6, invariant 6). A
	// consumer that filled in a staff id would be attributing the write to somebody who did not do
	// it.
	if so.nhan[0].NguoiGay.ID != "" {
		t.Fatalf("người gây phải để trống cho app điền chủ thể hệ thống, nhận %q", so.nhan[0].NguoiGay.ID)
	}
	if len(mau.hoi) != 1 || mau.hoi[0] != mocThu {
		t.Fatalf("hỏi mẫu tin: %v", mau.hoi)
	}
}

func TestLanThuHaiDiQuaNguyenVen(t *testing.T) {
	// ADR 0008: a citizen may reopen a closed petition, so `da-dong` happens more than once and the
	// second closing owes a SECOND message. `occurrence` is what tells the two apart inside
	// domain.KhoaLanGui — passing 1 for both would swallow the second, and nothing would turn red
	// anywhere else.
	c, so, _, _ := dungConsumer(t)
	tin := tinMau()
	tin.Status = "da-dong"
	tin.Occurrence = 2

	if err := c.Nhan(context.Background(), phongBi(t, tin)); err != nil {
		t.Fatalf("Nhan: %v", err)
	}
	if so.nhan[0].Lan != 2 {
		t.Fatalf("Lan = %d, muốn 2", so.nhan[0].Lan)
	}
}

func TestGiaoLaiHaiLanChoCungMotYeuCau(t *testing.T) {
	// Queues deliver at least once, and a producer that crashed after publishing republishes under
	// a FRESH envelope id. Neither may change what is handed to the ledger, because the
	// deduplication key is built from these fields — it is a key of the FACT, not of the message.
	c, so, _, _ := dungConsumer(t)
	pb := phongBi(t, tinMau())
	if err := c.Nhan(context.Background(), pb); err != nil {
		t.Fatalf("lần 1: %v", err)
	}
	pb2 := phongBi(t, tinMau())
	pb2.ID = "01JPHONGBIKHACMAUTHUNGHIE"
	so.moi = false // the ledger reports "already recorded"
	if err := c.Nhan(context.Background(), pb2); err != nil {
		t.Fatalf("lần 2: %v", err)
	}
	if len(so.nhan) != 2 {
		t.Fatalf("gọi ghi sổ %d lần, muốn 2 — chống trùng là việc của sổ, không phải của consumer", len(so.nhan))
	}
	if so.nhan[0] != so.nhan[1] {
		t.Fatalf("hai lần giao cùng một sự thật cho hai yêu cầu khác nhau:\n %+v\n %+v", so.nhan[0], so.nhan[1])
	}
}

func TestLoiCuaSoDuocBoc(t *testing.T) {
	c, so, _, _ := dungConsumer(t)
	rieng := errors.New("sổ đang bận")
	so.loi = rieng

	err := c.Nhan(context.Background(), phongBi(t, tinMau()))
	if !errors.Is(err, rieng) {
		t.Fatalf("muốn bọc lỗi của sổ, nhận %v", err)
	}
	if errors.Is(err, ErrKhongThuLai) {
		t.Fatalf("một lỗi ghi sổ PHẢI được giao lại, không được đánh dấu không thử lại")
	}
}

// --- an absent citizen_message is a fact -------------------------------------------------------

func TestVangCitizenMessageThiKhongGhiVaKhongNoiGi(t *testing.T) {
	c, so, mau, nhatKy := dungConsumer(t)
	tin := tinMau()
	tin.CitizenMessage = nil

	if err := c.Nhan(context.Background(), phongBi(t, tin)); err != nil {
		t.Fatalf("một bước nội bộ không phải lỗi: %v", err)
	}
	if len(so.nhan) != 0 {
		t.Fatalf("đã ghi %d dòng cho một chuyển trạng thái không nợ tin nào", len(so.nhan))
	}
	if len(mau.hoi) != 0 {
		t.Fatalf("đã hỏi mẫu tin cho một chuyển trạng thái không gửi gì")
	}
	if nhatKy.Len() != 0 {
		// It is the frequent, correct outcome on six of the nine transitions. Printing it as an
		// anomaly teaches whoever reads the channel to skip past it.
		t.Fatalf("đã log một ca hoàn toàn bình thường: %q", nhatKy.String())
	}
}

// --- a commune with no OA degrades VISIBLY, and does not retry ---------------------------------

func TestXaChuaCauHinhKenhThiKhongThuLai(t *testing.T) {
	c, so, mau, nhatKy := dungConsumer(t)
	mau.loi = ErrXaChuaCauHinhKenh

	err := c.Nhan(context.Background(), phongBi(t, tinMau()))
	if !errors.Is(err, ErrXaChuaCauHinhKenh) || !errors.Is(err, ErrKhongThuLai) {
		t.Fatalf("muốn ErrXaChuaCauHinhKenh + ErrKhongThuLai, nhận %v", err)
	}
	if len(so.nhan) != 0 {
		t.Fatalf("đã ghi sổ dù chưa có mẫu tin nào được duyệt")
	}
	// ADR 0006 consequence 3: it must be VISIBLE, not silent.
	if !strings.Contains(nhatKy.String(), maPhieuThu) {
		t.Fatalf("suy giảm phải nhìn thấy được, nhật ký: %q", nhatKy.String())
	}
}

func TestLoiKhacCuaMauTinVanDuocGiaoLai(t *testing.T) {
	c, _, mau, _ := dungConsumer(t)
	mau.loi = errors.New("kho cấu hình không với tới được")

	err := c.Nhan(context.Background(), phongBi(t, tinMau()))
	if err == nil {
		t.Fatalf("muốn lỗi")
	}
	if errors.Is(err, ErrKhongThuLai) {
		t.Fatalf("một sự cố hạ tầng PHẢI được giao lại: %v", err)
	}
}

// --- rule 3: nothing on any path names a person -------------------------------------------------

func TestKhongDuongNaoLogDinhDanhCongDan(t *testing.T) {
	// The recipient id is opaque, which is why it may travel in the event at all — but a log line
	// pairing it with a record code is the first half of a map of who reported what (rule 6,
	// forbidden #4). Checked on BOTH paths that can log.
	for _, tc := range []struct {
		ten string
		loi error
	}{
		{"gửi được", nil},
		{"xã chưa cấu hình kênh", ErrXaChuaCauHinhKenh},
	} {
		t.Run(tc.ten, func(t *testing.T) {
			c, _, mau, nhatKy := dungConsumer(t)
			mau.loi = tc.loi
			_ = c.Nhan(context.Background(), phongBi(t, tinMau()))
			if strings.Contains(nhatKy.String(), maCongDanThu) {
				t.Fatalf("mã công dân lọt vào nhật ký: %q", nhatKy.String())
			}
		})
	}
}
