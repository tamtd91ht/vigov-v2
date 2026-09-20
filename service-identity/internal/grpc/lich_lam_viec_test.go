package grpc

// What these tests defend: THE CODE EACH FAULT GETS. The arithmetic itself is defended in
// internal/domain, where it can be read without a wire type in the way; what is checkable only
// here is that a caller fault, a commune's configuration fault and a store outage come back as
// three DIFFERENT codes — because confusing two of them sends an operator to the wrong screen,
// and confusing the third with an answer stores an administrative record with no commitment
// attached to it.

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

type lichGia struct {
	cas      []domain.CaLamViec
	err      error
	soLanGoi int
}

func (f *lichGia) DanhSach(context.Context) ([]domain.CaLamViec, error) {
	f.soLanGoi++
	return f.cas, f.err
}

type nghiLeGia struct {
	ds  []domain.NgayNghiLe
	err error
}

func (f *nghiLeGia) TheoNam(context.Context, int) ([]domain.NgayNghiLe, error) {
	return f.ds, f.err
}

type lamBuGia struct {
	ds  []domain.CaLamBu
	err error
}

func (f *lamBuGia) TheoNam(context.Context, int) ([]domain.CaLamBu, error) { return f.ds, f.err }

// tuanGia is the ordinary week `may` wires by default: Monday–Friday, 07:30–11:30 and 13:30–17:00.
func tuanGia() []domain.CaLamViec {
	var ra []domain.CaLamViec
	for thu := 1; thu <= 5; thu++ {
		ra = append(ra,
			domain.CaLamViec{ID: "sang-" + string(rune('0'+thu)), Thu: thu,
				BatDau: 7*3600 + 30*60, KetThuc: 11*3600 + 30*60},
			domain.CaLamViec{ID: "chieu-" + string(rune('0'+thu)), Thu: thu,
				BatDau: 13*3600 + 30*60, KetThuc: 17 * 3600},
		)
	}
	return ra
}

// muiDoiChungVN is a FIXED +07 built here, and deliberately NOT domain.MuiGio(): an expectation
// computed by the code under test agrees with that code by construction, and the TZ property would
// be green for the wrong reason. Vietnam has been UTC+7 with no daylight saving since 1975, so a
// fixed offset is an independent oracle for every date below.
var muiDoiChungVN = time.FixedZone("ICT-doi-chung", 7*3600)

// mocVN reads "2006-01-02 15:04" as a wall-clock time in the commune's zone — never the process's,
// so the expectation does not move with TZ.
func mocVN(t *testing.T, s string) time.Time {
	t.Helper()
	ra, err := time.ParseInLocation("2006-01-02 15:04", s, muiDoiChungVN)
	if err != nil {
		t.Fatalf("mốc %q không đọc được: %v", s, err)
	}
	return ra
}

func yeuCau(t *testing.T, tuMoc string, gio ...uint32) *identityv1.AdvanceWorkingHoursRequest {
	t.Helper()
	return &identityv1.AdvanceWorkingHoursRequest{
		CountFrom:    timestamppb.New(mocVN(t, tuMoc)),
		WorkingHours: gio,
	}
}

// ---------------------------------------------------------------- lỗi của BÊN GỌI

// Không có `count_from` thì KHÔNG có mặc định "đếm từ bây giờ": một mặc định ở đây làm hạn phụ
// thuộc vào lúc lời gọi tình cờ được thực hiện, chứ không phải lúc cơ quan nhận hồ sơ.
func TestAdvanceThieuCountFromLaInvalidArgument(t *testing.T) {
	s, _ := may(t, nil)

	_, err := s.AdvanceWorkingHours(ctxXa(xaA),
		&identityv1.AdvanceWorkingHoursRequest{WorkingHours: []uint32{2}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
	}
}

// Mỗi lỗi dưới đây là lỗi của BÊN GỌI, nên nó phải là INVALID_ARGUMENT chứ không phải
// FAILED_PRECONDITION: khác nhau giữa "sửa dịch vụ đang gọi" và "mở màn hình cấu hình của xã".
//
// VÀ KHÔNG ĐƯỢC CHẠM VÀO CƠ SỞ DỮ LIỆU: một lời gọi sai hình dạng không được biến thành tải đọc
// trên tiến trình đang phục vụ 200+ xã.
func TestAdvanceThamSoSaiLaInvalidArgumentVaKhongDocKho(t *testing.T) {
	ca := []struct {
		ten string
		req *identityv1.AdvanceWorkingHoursRequest
	}{
		{"không có mốc nào", yeuCau(t, "2026-09-21 08:00")},
		{"mốc 0 giờ", yeuCau(t, "2026-09-21 08:00", 2, 0)},
		{"mốc vượt trần 2000 giờ", yeuCau(t, "2026-09-21 08:00", 2001)},
		{"quá 20 mốc", yeuCau(t, "2026-09-21 08:00",
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21)},
		{"quá 20 mốc dù trùng hết", yeuCau(t, "2026-09-21 08:00",
			2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2)},
	}

	for _, c := range ca {
		t.Run(c.ten, func(t *testing.T) {
			lich := &lichGia{cas: tuanGia()}
			s, _ := may(t, func(d *Deps) { d.Lich = lich })

			_, err := s.AdvanceWorkingHours(ctxXa(xaA), c.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
			}
			if lich.soLanGoi != 0 {
				t.Errorf("đã đọc lịch %d lần cho một lời gọi sai hình dạng — phải từ chối trước",
					lich.soLanGoi)
			}
		})
	}
}

// Tới được handler mà không có xã nghĩa là grpcx.UnaryServerInterceptor không nằm trong chuỗi:
// lỗi cấu hình của bản triển khai này, nên là Internal — và KHÔNG ĐƯỢC PANIC, vì panic trong một
// handler gRPC không được phục hồi và hạ cả tiến trình đang phục vụ 200+ xã.
func TestAdvanceKhongCoXaTrongContextLaInternalChuKhongPanic(t *testing.T) {
	s, nhatKy := may(t, nil)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handler panic khi thiếu xã: %v", r)
		}
	}()

	_, err := s.AdvanceWorkingHours(context.Background(), yeuCau(t, "2026-09-21 08:00", 2))
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal (lỗi: %v)", status.Code(err), err)
	}
	if nhatKy.Len() == 0 {
		t.Error("không có dòng log nào chỉ ra nguyên nhân thật")
	}
}

// ---------------------------------------------------------------- lỗi CẤU HÌNH của xã

// LỊCH TRỐNG PHẢI HỎNG TO Ở ĐÂY, khác hẳn "OK + không có principal" của ResolveStaffPrincipal.
// Một bên gọi đọc sự vắng mặt thành "không có hạn" sẽ lưu một hồ sơ hành chính không mang cam
// kết nào — và hôm nay lịch trống là trạng thái của MỌI xã, vì migration 0006 không gieo dòng nào.
func TestAdvanceLichTrongLaFailedPreconditionChuKhongPhaiOK(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.Lich = &lichGia{} })

	ra, err := s.AdvanceWorkingHours(ctxXa(xaA), yeuCau(t, "2026-09-21 08:00", 2))
	if err == nil {
		t.Fatalf("lịch trống mà vẫn trả lời OK: %+v", ra.GetItems())
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mã = %v, muốn FailedPrecondition (lỗi: %v)", status.Code(err), err)
	}
	// Lời từ chối phải nói được cho người sửa là sửa cái gì — ngày giờ làm việc không phải dữ
	// liệu cá nhân (luật 3), và một lời từ chối không nêu chỗ cần sửa là lời từ chối bị bỏ qua.
	if status.Convert(err).Message() == "" {
		t.Error("thông điệp từ chối rỗng")
	}
}

// Ngày vừa nghỉ lễ vừa làm bù là LỖI CẤU HÌNH, không phải sự cố hệ thống: FAILED_PRECONDITION.
// Biến nó thành Internal là gửi operator đi tìm một sự cố không tồn tại trong khi màn hình cấu
// hình đang có hai dòng mâu thuẫn nhau.
func TestAdvanceXungDotLichLaFailedPreconditionChuKhongPhaiInternal(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.NghiLe = &nghiLeGia{err: &domain.LoiNgayVuaNghiVuaLamBu{Ngay: []string{"2026-09-19"}}}
	})

	_, err := s.AdvanceWorkingHours(ctxXa(xaA), yeuCau(t, "2026-09-18 22:00", 2))
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mã = %v, muốn FailedPrecondition (lỗi: %v)", status.Code(err), err)
	}
}

// Ngày làm bù rơi vào ngày vốn đã có ca (ADR 0007 quyết định 9) cũng là lỗi cấu hình.
func TestAdvanceLamBuTrungNgayLamViecLaFailedPrecondition(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.LamBu = &lamBuGia{ds: []domain.CaLamBu{{
			ID: "bu-1", Ngay: "2026-09-21", BatDau: 18 * 3600, KetThuc: 20 * 3600,
			Ten: "Làm bù giả định",
		}}}
	})

	_, err := s.AdvanceWorkingHours(ctxXa(xaA), yeuCau(t, "2026-09-18 22:00", 2))
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mã = %v, muốn FailedPrecondition (lỗi: %v)", status.Code(err), err)
	}
}

// ---------------------------------------------------------------- sự cố HỆ THỐNG

// KHO HỎNG LÀ Internal, KHÔNG PHẢI FAILED_PRECONDITION VÀ KHÔNG BAO GIỜ LÀ MỘT CÂU TRẢ LỜI.
// Một bên gọi đọc sự cố thành "không có hạn" sẽ lưu hồ sơ không mang cam kết; một operator đọc
// sự cố thành lỗi cấu hình sẽ đi đọc một cuốn lịch đang hoàn toàn đúng.
func TestAdvanceKhoHongLaInternalChuKhongPhaiCauTraLoi(t *testing.T) {
	ca := []struct {
		ten string
		sua func(*Deps)
	}{
		{"kho lịch làm việc hỏng", func(d *Deps) { d.Lich = &lichGia{err: loiKho} }},
		{"kho ngày nghỉ lễ hỏng", func(d *Deps) { d.NghiLe = &nghiLeGia{err: loiKho} }},
		{"kho ngày làm bù hỏng", func(d *Deps) { d.LamBu = &lamBuGia{err: loiKho} }},
		{"lịch vượt trần", func(d *Deps) { d.Lich = &lichGia{err: idstore.ErrQuaNhieuCaLamViec} }},
	}

	for _, c := range ca {
		t.Run(c.ten, func(t *testing.T) {
			s, _ := may(t, c.sua)

			ra, err := s.AdvanceWorkingHours(ctxXa(xaA), yeuCau(t, "2026-09-21 08:00", 2))
			if err == nil {
				t.Fatalf("kho hỏng mà vẫn trả lời OK: %+v", ra.GetItems())
			}
			if status.Code(err) != codes.Internal {
				t.Fatalf("mã = %v, muốn Internal (lỗi: %v)", status.Code(err), err)
			}
			// Nguyên nhân đi vào log của DỊCH VỤ NÀY, nơi có operator — không đi qua ranh giới
			// vào log của tiến trình khác (luật 3, cấm #3).
			if msg := status.Convert(err).Message(); msg != "lỗi nội bộ, vui lòng thử lại" {
				t.Errorf("thông điệp = %q — không được mang nguyên nhân qua ranh giới", msg)
			}
		})
	}
}

// ---------------------------------------------------------------- câu trả lời

// Hai hạn của MỘT lần tiếp nhận — `Tiếp nhận` và `Xử lý xong` của bảng ADR 0007 — trong MỘT lời
// gọi, trên MỘT lần đọc lịch. Mốc trùng thì gộp, nên len(items) có thể nhỏ hơn len(working_hours)
// và vị trí không bao giờ là khoá: mỗi item mang chính con số được hỏi.
func TestAdvanceTraDuMocVaGopTrung(t *testing.T) {
	lich := &lichGia{cas: tuanGia()}
	s, _ := may(t, func(d *Deps) { d.Lich = lich })

	ra, err := s.AdvanceWorkingHours(ctxXa(xaA), yeuCau(t, "2026-09-21 07:30", 2, 16, 2))
	if err != nil {
		t.Fatalf("AdvanceWorkingHours: %v", err)
	}
	if len(ra.GetItems()) != 2 {
		t.Fatalf("số item = %d, muốn 2 (mốc trùng phải gộp)", len(ra.GetItems()))
	}
	if lich.soLanGoi != 1 {
		t.Errorf("đọc lịch %d lần cho một lời gọi — hai lần đọc là hai trạng thái cấu hình có thể khác nhau",
			lich.soLanGoi)
	}

	theoGio := map[uint32]time.Time{}
	for _, m := range ra.GetItems() {
		theoGio[m.GetWorkingHours()] = m.GetReachedAt().AsTime()
	}
	for _, c := range []struct {
		gio  uint32
		muon string
	}{
		{2, "2026-09-21 09:30"},
		{16, "2026-09-23 08:30"},
	} {
		duoc, co := theoGio[c.gio]
		if !co {
			t.Fatalf("thiếu mốc %d giờ — không có cách nào diễn đạt 'thành công một phần'", c.gio)
		}
		if !duoc.Equal(mocVN(t, c.muon)) {
			t.Errorf("mốc %d giờ đạt lúc %s, muốn %s", c.gio,
				duoc.Format(time.RFC3339), mocVN(t, c.muon).Format(time.RFC3339))
		}
	}
}

// Tiếp nhận NGOÀI MỌI CA thì đếm từ đầu ca kế tiếp (ADR 0007 quyết định 8) — và ngày nghỉ lễ,
// ngày làm bù của chính xã ấy quyết định "ca kế tiếp" là ca nào.
func TestAdvanceNgoaiGioDemTuDauCaKeTiepQuaCaRanhGioiNgayNghi(t *testing.T) {
	ca := []struct {
		ten  string
		sua  func(*Deps)
		muon string
	}{
		{"tuần bình thường", nil, "2026-09-21 09:30"},
		{"thứ Hai nghỉ lễ", func(d *Deps) {
			d.NghiLe = &nghiLeGia{ds: []domain.NgayNghiLe{{ID: "le-1", Ngay: "2026-09-21", Ten: "Lễ giả định"}}}
		}, "2026-09-22 09:30"},
		{"thứ Bảy có làm bù", func(d *Deps) {
			d.LamBu = &lamBuGia{ds: []domain.CaLamBu{{
				ID: "bu-1", Ngay: "2026-09-19", BatDau: 7*3600 + 30*60, KetThuc: 11*3600 + 30*60,
				Ten: "Làm bù giả định",
			}}}
		}, "2026-09-19 09:30"},
	}

	for _, c := range ca {
		t.Run(c.ten, func(t *testing.T) {
			s, _ := may(t, c.sua)

			ra, err := s.AdvanceWorkingHours(ctxXa(xaA), yeuCau(t, "2026-09-18 22:00", 2))
			if err != nil {
				t.Fatalf("AdvanceWorkingHours: %v", err)
			}
			if duoc := ra.GetItems()[0].GetReachedAt().AsTime(); !duoc.Equal(mocVN(t, c.muon)) {
				t.Errorf("đạt lúc %s, muốn %s", duoc.Format(time.RFC3339), mocVN(t, c.muon).Format(time.RFC3339))
			}
		})
	}
}

// MÚI GIỜ LÀ CỦA XÃ, KHÔNG PHẢI CỦA TIẾN TRÌNH — kiểm ngay trên ranh giới, vì đây là nơi giá trị
// rời khỏi dịch vụ. Đặt thẳng time.Local là đúng thứ TZ làm lúc tiến trình khởi động.
func TestAdvanceKhongPhuThuocTZCuaTienTrinh(t *testing.T) {
	goc := time.Local
	t.Cleanup(func() { time.Local = goc })

	var dapAn []time.Time
	for _, mui := range []*time.Location{time.UTC, time.FixedZone("GIA", 7*3600)} {
		time.Local = mui
		s, _ := may(t, nil)
		ra, err := s.AdvanceWorkingHours(ctxXa(xaA), yeuCau(t, "2026-09-21 08:00", 5))
		if err != nil {
			t.Fatalf("AdvanceWorkingHours (TZ %s): %v", mui, err)
		}
		dapAn = append(dapAn, ra.GetItems()[0].GetReachedAt().AsTime())
	}
	if !dapAn[0].Equal(dapAn[1]) {
		t.Fatalf("TZ của tiến trình đổi thì đáp án đổi: %s so với %s",
			dapAn[0].Format(time.RFC3339), dapAn[1].Format(time.RFC3339))
	}
	if !dapAn[0].Equal(mocVN(t, "2026-09-21 15:00")) {
		t.Errorf("đạt lúc %s, muốn 2026-09-21 15:00 giờ Việt Nam", dapAn[0].Format(time.RFC3339))
	}
}
