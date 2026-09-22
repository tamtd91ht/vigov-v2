package grpc

// What these tests defend, in one sentence: THAT A COMMUNE WHICH HAS CONFIGURED NOTHING IS TOLD SO,
// AND THAT A COMMUNE WHICH HAS CONFIGURED SOMETHING GETS ITS OWN NUMBER.
//
// Both halves are cheap to break and neither breaks loudly. A fallback added "so the route works"
// turns an unconfigured commune into a commitment nobody chose; a lookup dropped in a refactor
// turns every commune into the same commitment. In both cases the response is a perfectly
// plausible instant and nothing errors — which is why the two-communes test below is written to go
// RED if the `sla` read stops deciding the answer.

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

type slaGia struct {
	ds       []domain.DongSLA
	err      error
	soLanGoi int
}

func (f *slaGia) DanhSach(context.Context) ([]domain.DongSLA, error) {
	f.soLanGoi++
	return f.ds, f.err
}

// dongSLA builds one row. THE FIVE HOUR COLUMNS ARE GIVEN FIVE DIFFERENT VALUES, deliberately: they
// are adjacent integers, and swapping two of them anywhere between the column list and this wire
// type produces no error at all — the rows still load, the response still renders, and the only
// difference is which commitment the authority made. Different values are what makes such a swap
// visible.
func dongSLA(id string, loai domain.LoaiViec, linhVuc string, tiepNhan, xuLyXong int) domain.DongSLA {
	return domain.DongSLA{
		ID: id, LoaiViec: loai, LinhVuc: linhVuc,
		GioTiepNhan: tiepNhan, GioXuLyXong: xuLyXong,
		GioSapDenHan: 3, GioBaoLanhDao: 5, GioBaoChuTich: 7,
	}
}

func yeuCauHan(t *testing.T, tuMoc string, linhVuc string,
	can ...identityv1.DeadlineKind) *identityv1.ResolveDeadlinesRequest {
	t.Helper()
	return &identityv1.ResolveDeadlinesRequest{
		WorkKind:  identityv1.WorkKind_WORK_KIND_PHAN_ANH,
		LinhVuc:   linhVuc,
		CountFrom: timestamppb.New(mocVN(t, tuMoc)),
		Deadlines: can,
	}
}

// mocRa reads one clock out of the response, failing loudly rather than returning a zero instant:
// a zero time.Time is a deadline in year 1, and a test that quietly compared one would be green for
// a value no commune could ever have meant.
func mocRa(t *testing.T, ra *identityv1.ResolveDeadlinesResponse, k identityv1.DeadlineKind) time.Time {
	t.Helper()
	for _, it := range ra.GetItems() {
		if it.GetKind() == k {
			if it.GetDueAt() == nil {
				t.Fatalf("hạn %s có mục nhưng không có due_at", k.String())
			}
			return it.GetDueAt().AsTime()
		}
	}
	t.Fatalf("không có hạn %s trong %d mục trả về", k.String(), len(ra.GetItems()))
	return time.Time{}
}

// ------------------------------------------------- XÃ CHƯA CẤU HÌNH: TỪ CHỐI, KHÔNG PHẢI MẶC ĐỊNH

// ĐÂY LÀ CA QUAN TRỌNG NHẤT CỦA TỆP NÀY, và hôm nay nó là trạng thái của MỌI xã: migration 0008 cố
// ý không gieo dòng nào và bước khởi tạo xã chưa tồn tại.
//
// Mã phải là FAILED_PRECONDITION — *"mở màn hình cấu hình của xã"* — chứ không phải:
//
//	OK kèm một con số   ⟶ phần mềm tự hứa hộ cơ quan (luật 10 cấm #3)
//	OK kèm mục rỗng     ⟶ bên gọi nhận một time.Time rỗng, tức hạn năm 1, tức quá hạn ngay khi sinh
//	INVALID_ARGUMENT    ⟶ đẩy người vận hành đi sửa service đang gọi, nơi không có gì sai
//	NOT_FOUND           ⟶ "thiếu một dòng trong một tập hợp", trong khi sự thật là xã CHƯA QUYẾT
func TestHanBangSLARongLaFailedPreconditionChuKhongPhaiMotConSo(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.SLA = &slaGia{} })

	ra, err := s.ResolveDeadlines(ctxXa(xaA),
		yeuCauHan(t, "2026-09-21 08:00", "", identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN))

	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mã = %v, muốn FailedPrecondition (lỗi: %v)", status.Code(err), err)
	}
	if ra != nil {
		t.Fatalf("có phản hồi %v kèm lỗi — xã chưa cấu hình phải KHÔNG có hạn nào, không phải hạn rỗng", ra)
	}
}

// Lỗ hổng VÔ HÌNH TRÊN MÀN HÌNH: xã đã khai `an-ninh-trat-tu` nên bảng trông như đã cấu hình, nhưng
// không có dòng mặc định. Mọi phiếu thuộc lĩnh vực khác — và MỌI phiếu công dân gửi, vì lúc sinh
// phiếu chưa ai biết lĩnh vực (ADR 0028 quyết định E) — không có gì để rơi về.
//
// Từ chối, và thông điệp phải nói ra "thiếu dòng mặc định": một lời từ chối không nói sửa gì là một
// lời từ chối rồi sẽ bị bỏ qua.
func TestHanThieuDongMacDinhLaFailedPreconditionChuKhongMuonSangLinhVucKhac(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.SLA = &slaGia{ds: []domain.DongSLA{
			dongSLA("sla-1", domain.LoaiViecPhanAnh, "an-ninh-trat-tu", 2, 16),
		}}
	})

	// Một lĩnh vực KHÁC, không có dòng riêng và cũng không có dòng mặc định để rơi về.
	_, err := s.ResolveDeadlines(ctxXa(xaA),
		yeuCauHan(t, "2026-09-21 08:00", "rac-thai", identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG))
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mã = %v, muốn FailedPrecondition (lỗi: %v)", status.Code(err), err)
	}

	// VÀ TUYỆT ĐỐI KHÔNG ĐƯỢC MƯỢN SỐ GIỜ CỦA LĨNH VỰC ĐÃ KHAI. Nếu một ngày nào đó nó trả về hạn
	// tính từ 16 giờ của `an-ninh-trat-tu`, phép kiểm trên đã đỏ — ca này ghi lại VÌ SAO đỏ.
	if st, _ := status.FromError(err); st != nil && st.Code() == codes.OK {
		t.Fatalf("trả OK cho một lĩnh vực chưa cấu hình")
	}
}

// Kênh công dân hỏi dòng mặc định bằng `linh_vuc` RỖNG, và rỗng ở đây là một câu hỏi thật chứ không
// phải một trường bị quên: lúc dân bấm gửi chưa ai biết lĩnh vực (ADR 0028 quyết định E). Xã có
// dòng cho một lĩnh vực nhưng KHÔNG có dòng mặc định thì ca này cũng phải từ chối.
func TestHanLinhVucRongKhongCoDongMacDinhLaFailedPrecondition(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.SLA = &slaGia{ds: []domain.DongSLA{
			dongSLA("sla-1", domain.LoaiViecPhanAnh, "an-ninh-trat-tu", 2, 16),
		}}
	})

	_, err := s.ResolveDeadlines(ctxXa(xaA),
		yeuCauHan(t, "2026-09-21 08:00", "", identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN))
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mã = %v, muốn FailedPrecondition (lỗi: %v)", status.Code(err), err)
	}
}

// Loại việc KHÁC chưa cấu hình: xã khai đủ cho `phan-anh` không có nghĩa là đã khai cho `van-ban-den`.
// Mượn số giờ của loại việc kia là hứa số của nghiệp vụ khác.
func TestHanLoaiViecChuaCauHinhKhongMuonSoCuaLoaiViecKhac(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.SLA = &slaGia{ds: []domain.DongSLA{
			dongSLA("sla-1", domain.LoaiViecPhanAnh, "", 8, 40),
		}}
	})

	req := yeuCauHan(t, "2026-09-21 08:00", "", identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN)
	req.WorkKind = identityv1.WorkKind_WORK_KIND_VAN_BAN_DEN

	_, err := s.ResolveDeadlines(ctxXa(xaA), req)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mã = %v, muốn FailedPrecondition (lỗi: %v)", status.Code(err), err)
	}
}

// Số giờ trong dòng SLA không dương là lỗi CẤU HÌNH CỦA XÃ, không phải lỗi bên gọi — nên
// FAILED_PRECONDITION chứ không phải INVALID_ARGUMENT như AdvanceWorkingHours trả cho cùng hình
// dạng. Chỗ khác nhau là NGUỒN của con số.
//
// Và nó phải TỪ CHỐI chứ không đưa 0 vào phép tính: 0 giờ trả về đúng `count_from`, tức một hồ sơ
// đến hạn ngay lúc tiếp nhận, mà không có gì báo lỗi ở đâu cả.
func TestHanSoGioKhongDuongLaFailedPreconditionChuKhongPhaiInvalidArgument(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.SLA = &slaGia{ds: []domain.DongSLA{dongSLA("sla-1", domain.LoaiViecPhanAnh, "", 0, 40)}}
	})

	_, err := s.ResolveDeadlines(ctxXa(xaA),
		yeuCauHan(t, "2026-09-21 08:00", "", identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN))
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mã = %v, muốn FailedPrecondition (lỗi: %v)", status.Code(err), err)
	}
}

// ------------------------------------------------------------------------ lỗi của BÊN GỌI

// Mỗi lỗi dưới đây là lỗi của BÊN GỌI, nên INVALID_ARGUMENT — và KHÔNG ĐƯỢC CHẠM VÀO KHO: một lời
// gọi sai hình dạng không được biến thành tải đọc trên tiến trình đang phục vụ 200+ xã.
func TestHanThamSoSaiLaInvalidArgumentVaKhongDocKho(t *testing.T) {
	tiepNhan := identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN
	ca := []struct {
		ten string
		req *identityv1.ResolveDeadlinesRequest
	}{
		{"không có work_kind", &identityv1.ResolveDeadlinesRequest{
			CountFrom: timestamppb.New(mocVN(t, "2026-09-21 08:00")),
			Deadlines: []identityv1.DeadlineKind{tiepNhan},
		}},
		{"không có count_from", &identityv1.ResolveDeadlinesRequest{
			WorkKind:  identityv1.WorkKind_WORK_KIND_PHAN_ANH,
			Deadlines: []identityv1.DeadlineKind{tiepNhan},
		}},
		{"không có deadlines", yeuCauHan(t, "2026-09-21 08:00", "")},
		{"deadlines có mục chưa xác định", yeuCauHan(t, "2026-09-21 08:00", "",
			identityv1.DeadlineKind_DEADLINE_KIND_UNSPECIFIED)},
		{"deadlines vượt trần 2 mục", yeuCauHan(t, "2026-09-21 08:00", "",
			tiepNhan, tiepNhan, tiepNhan)},
	}

	for _, c := range ca {
		t.Run(c.ten, func(t *testing.T) {
			sla := &slaGia{ds: []domain.DongSLA{dongSLA("sla-1", domain.LoaiViecPhanAnh, "", 8, 40)}}
			lich := &lichGia{cas: tuanGia()}
			s, _ := may(t, func(d *Deps) { d.SLA = sla; d.Lich = lich })

			_, err := s.ResolveDeadlines(ctxXa(xaA), c.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
			}
			if sla.soLanGoi != 0 || lich.soLanGoi != 0 {
				t.Errorf("đã đọc kho (sla %d lần, lịch %d lần) cho một lời gọi sai hình dạng — phải từ chối trước",
					sla.soLanGoi, lich.soLanGoi)
			}
		})
	}
}

// Tới được handler mà không có xã trong context nghĩa là grpcx.UnaryServerInterceptor không nằm
// trong chuỗi: lỗi cấu hình của bản triển khai, nên Internal — và KHÔNG ĐƯỢC PANIC, vì panic trong
// một handler gRPC không được phục hồi và hạ cả tiến trình đang phục vụ 200+ xã.
func TestHanKhongCoXaTrongContextLaInternalChuKhongPanic(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.SLA = &slaGia{ds: []domain.DongSLA{dongSLA("sla-1", domain.LoaiViecPhanAnh, "", 8, 40)}}
	})

	_, err := s.ResolveDeadlines(context.Background(),
		yeuCauHan(t, "2026-09-21 08:00", "", identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN))
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal (lỗi: %v)", status.Code(err), err)
	}
}

// Kho hỏng là SỰ CỐ, không phải một câu trả lời nghiệp vụ: Internal, và tuyệt đối không được biến
// thành "xã chưa cấu hình" — hai thứ ấy gửi người vận hành tới hai nơi khác nhau.
func TestHanKhoSLAHongLaInternalChuKhongPhaiFailedPrecondition(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.SLA = &slaGia{err: errors.New("kho sập")}
	})

	_, err := s.ResolveDeadlines(ctxXa(xaA),
		yeuCauHan(t, "2026-09-21 08:00", "", identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN))
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal (lỗi: %v)", status.Code(err), err)
	}
}

// ---------------------------------------------------------------------- câu trả lời đúng

// Hai đồng hồ, MỘT lần đọc lịch, và hai con số KHÁC NHAU lấy từ hai cột khác nhau của cùng một
// dòng. Đọc lịch một lần là thứ giữ cho hai hạn của một hồ sơ được tính trên CÙNG một bản lịch.
//
// Tuần giả: T2–T6, 07:30–11:30 và 13:30–17:00 (7,5 giờ/ngày). Đếm từ 08:00 thứ Hai 21/09/2026:
//
//	8 giờ  → 3,5 (sáng T2) + 3,5 (chiều T2) = 7,0 lúc 17:00 T2; còn 1 giờ → 08:30 thứ Ba 22/09
//	16 giờ → 7,5 (T2 từ 08:00 là 7,0) … 7,0 + 7,5 (cả T3) = 14,5 lúc 17:00 T3; còn 1,5 → 09:00 T4
func TestHanHaiDongHoMotLanDocLichVaHaiCotKhacNhau(t *testing.T) {
	lich := &lichGia{cas: tuanGia()}
	s, _ := may(t, func(d *Deps) {
		d.Lich = lich
		d.SLA = &slaGia{ds: []domain.DongSLA{dongSLA("sla-1", domain.LoaiViecPhanAnh, "", 8, 16)}}
	})

	ra, err := s.ResolveDeadlines(ctxXa(xaA), yeuCauHan(t, "2026-09-21 08:00", "",
		identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN,
		identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG))
	if err != nil {
		t.Fatalf("ResolveDeadlines: %v", err)
	}
	if len(ra.GetItems()) != 2 {
		t.Fatalf("có %d mục, muốn 2", len(ra.GetItems()))
	}

	muonTiepNhan := mocVN(t, "2026-09-22 08:30")
	muonXuLyXong := mocVN(t, "2026-09-23 09:00")
	if got := mocRa(t, ra, identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN); !got.Equal(muonTiepNhan) {
		t.Errorf("hạn tiếp nhận = %s, muốn %s", got, muonTiepNhan)
	}
	if got := mocRa(t, ra, identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG); !got.Equal(muonXuLyXong) {
		t.Errorf("hạn xử lý xong = %s, muốn %s", got, muonXuLyXong)
	}

	// MỘT lần đọc lịch cho cả hai đồng hồ. Hai lần đọc là hai bản lịch, và bản thứ hai có thể khác
	// bản thứ nhất — hai hạn của một hồ sơ tính trên hai cấu hình khác nhau.
	if lich.soLanGoi != 1 {
		t.Errorf("đọc lịch %d lần cho hai đồng hồ, muốn đúng 1", lich.soLanGoi)
	}
}

// Dòng riêng của lĩnh vực THẮNG dòng mặc định. Sai chiều này thì xã khai 16 giờ cho an ninh trật tự
// mà phần mềm hứa 40 — và không có gì báo lỗi.
func TestHanDongCuaLinhVucThangDongMacDinh(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.SLA = &slaGia{ds: []domain.DongSLA{
			dongSLA("sla-mac-dinh", domain.LoaiViecPhanAnh, "", 8, 40),
			dongSLA("sla-an-ninh", domain.LoaiViecPhanAnh, "an-ninh-trat-tu", 8, 16),
		}}
	})

	ra, err := s.ResolveDeadlines(ctxXa(xaA), yeuCauHan(t, "2026-09-21 08:00", "an-ninh-trat-tu",
		identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG))
	if err != nil {
		t.Fatalf("ResolveDeadlines: %v", err)
	}
	// 16 giờ → 09:00 thứ Tư 23/09. Dòng mặc định (40 giờ) sẽ cho một mốc khác hẳn.
	muon := mocVN(t, "2026-09-23 09:00")
	if got := mocRa(t, ra, identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG); !got.Equal(muon) {
		t.Errorf("hạn = %s, muốn %s (dòng riêng của lĩnh vực phải thắng dòng mặc định)", got, muon)
	}
}

// MỘT NGÀY NGHỈ LỄ NẰM GIỮA — hạn phải NHẢY QUA nó.
//
// Cùng một lời gọi, khác đúng một thứ: thứ Ba 22/09 là ngày nghỉ lễ. 8 giờ làm việc từ 08:00 thứ
// Hai rơi vào 08:30 thứ Ba khi cơ quan mở, và phải dời sang 08:30 thứ Tư khi cơ quan đóng cửa thứ
// Ba. Chênh lệch ấy chính là thứ một phép cộng `n * time.Hour` ở bên gọi KHÔNG BAO GIỜ thấy.
func TestHanNhayQuaNgayNghiLe(t *testing.T) {
	slaMot := func() *slaGia {
		return &slaGia{ds: []domain.DongSLA{dongSLA("sla-1", domain.LoaiViecPhanAnh, "", 8, 40)}}
	}
	req := func() *identityv1.ResolveDeadlinesRequest {
		return yeuCauHan(t, "2026-09-21 08:00", "", identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN)
	}

	sKhongLe, _ := may(t, func(d *Deps) { d.SLA = slaMot() })
	raKhongLe, err := sKhongLe.ResolveDeadlines(ctxXa(xaA), req())
	if err != nil {
		t.Fatalf("không có ngày lễ: %v", err)
	}

	sCoLe, _ := may(t, func(d *Deps) {
		d.SLA = slaMot()
		d.NghiLe = &nghiLeGia{ds: []domain.NgayNghiLe{{ID: "le-1", Ngay: "2026-09-22"}}}
	})
	raCoLe, err := sCoLe.ResolveDeadlines(ctxXa(xaA), req())
	if err != nil {
		t.Fatalf("có ngày lễ: %v", err)
	}

	k := identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN
	muonKhongLe := mocVN(t, "2026-09-22 08:30")
	muonCoLe := mocVN(t, "2026-09-23 08:30")

	if got := mocRa(t, raKhongLe, k); !got.Equal(muonKhongLe) {
		t.Errorf("không có ngày lễ: hạn = %s, muốn %s", got, muonKhongLe)
	}
	if got := mocRa(t, raCoLe, k); !got.Equal(muonCoLe) {
		t.Errorf("có ngày lễ 22/09: hạn = %s, muốn %s — hạn phải NHẢY QUA ngày cơ quan đóng cửa",
			got, muonCoLe)
	}
}

// HAI XÃ, HAI SỐ GIỜ, HAI KẾT QUẢ KHÁC NHAU — và đây là ca ĐỘT BIẾN dùng để chứng minh phép kiểm
// còn sống.
//
// Mọi thứ khác giữ NGUYÊN Y HỆT giữa hai lời gọi: cùng lịch làm việc, cùng `count_from`, cùng loại
// việc, cùng lĩnh vực. Khác biệt duy nhất là dòng `sla` mỗi xã đọc được. Vì thế:
//
//	bỏ phần tra bảng `sla` và trả một hằng số ⟶ hai lời gọi cho ra CÙNG một mốc ⟶ ca này ĐỎ
//
// Không có ca này thì một bản cài đặt bỏ qua bảng `sla` vẫn xanh hết: nó vẫn trả về một mốc hợp lý,
// vẫn không lỗi, và con số sai chỉ lộ ra ở phía người dân.
func TestHanHaiXaKhacSoGioChoHaiKetQuaKhacNhau(t *testing.T) {
	req := func() *identityv1.ResolveDeadlinesRequest {
		return yeuCauHan(t, "2026-09-21 08:00", "", identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG)
	}
	k := identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG

	// Xã A hứa 8 giờ làm việc; xã B hứa 16. Cùng một tuần làm việc.
	sA, _ := may(t, func(d *Deps) {
		d.SLA = &slaGia{ds: []domain.DongSLA{dongSLA("sla-a", domain.LoaiViecPhanAnh, "", 2, 8)}}
	})
	sB, _ := may(t, func(d *Deps) {
		d.SLA = &slaGia{ds: []domain.DongSLA{dongSLA("sla-b", domain.LoaiViecPhanAnh, "", 2, 16)}}
	})

	raA, err := sA.ResolveDeadlines(ctxXa(xaA), req())
	if err != nil {
		t.Fatalf("xã A: %v", err)
	}
	raB, err := sB.ResolveDeadlines(ctxXa(xaB), req())
	if err != nil {
		t.Fatalf("xã B: %v", err)
	}

	hanA := mocRa(t, raA, k)
	hanB := mocRa(t, raB, k)

	if hanA.Equal(hanB) {
		t.Fatalf("hai xã khai hai số giờ khác nhau (8 và 16) nhưng nhận CÙNG một hạn %s — "+
			"tức con số không còn đến từ bảng `sla` của xã", hanA)
	}

	// Và không chỉ "khác nhau": mỗi mốc phải đúng số giờ CỦA CHÍNH XÃ ẤY. Chỉ khẳng định "khác
	// nhau" thì một bản cài đặt lấy nhầm cột vẫn qua được.
	if muon := mocVN(t, "2026-09-22 08:30"); !hanA.Equal(muon) {
		t.Errorf("xã A (8 giờ): hạn = %s, muốn %s", hanA, muon)
	}
	if muon := mocVN(t, "2026-09-23 09:00"); !hanB.Equal(muon) {
		t.Errorf("xã B (16 giờ): hạn = %s, muốn %s", hanB, muon)
	}
}

// Hai đồng hồ khai CÙNG một số giờ là một cấu hình hợp lệ, và phép tính gộp trùng — nên vẫn phải
// trả ĐỦ HAI mục. Đây là chỗ một bản cài đặt ánh xạ theo vị trí thay vì theo `kind` sẽ hỏng: nó trả
// một mục, và bên gọi lưu một cột với time.Time rỗng ở cột kia.
func TestHanHaiDongHoCungSoGioVanTraDuHaiMuc(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.SLA = &slaGia{ds: []domain.DongSLA{dongSLA("sla-1", domain.LoaiViecPhanAnh, "", 8, 8)}}
	})

	ra, err := s.ResolveDeadlines(ctxXa(xaA), yeuCauHan(t, "2026-09-21 08:00", "",
		identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN,
		identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG))
	if err != nil {
		t.Fatalf("ResolveDeadlines: %v", err)
	}
	if len(ra.GetItems()) != 2 {
		t.Fatalf("có %d mục, muốn 2 — gộp trùng là chuyện của phép tính, không phải của phản hồi",
			len(ra.GetItems()))
	}
	muon := mocVN(t, "2026-09-22 08:30")
	for _, kk := range []identityv1.DeadlineKind{
		identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN,
		identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG,
	} {
		if got := mocRa(t, ra, kk); !got.Equal(muon) {
			t.Errorf("%s = %s, muốn %s", kk.String(), got, muon)
		}
	}
}

// Lịch của xã trống là lỗi CẤU HÌNH, và nó phải giữ nguyên mã FAILED_PRECONDITION khi đi qua RPC
// này — cùng một đường mã mà AdvanceWorkingHours dùng, nên nếu hai bên trả hai mã khác nhau thì
// phần tách dùng chung đã hỏng.
func TestHanLichTrongVanLaFailedPrecondition(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.SLA = &slaGia{ds: []domain.DongSLA{dongSLA("sla-1", domain.LoaiViecPhanAnh, "", 8, 40)}}
		d.Lich = &lichGia{}
	})

	_, err := s.ResolveDeadlines(ctxXa(xaA),
		yeuCauHan(t, "2026-09-21 08:00", "", identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN))
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mã = %v, muốn FailedPrecondition (lỗi: %v)", status.Code(err), err)
	}
}
