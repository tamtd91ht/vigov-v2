package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for the LIFECYCLE routes of the meeting-minutes register (user decisions 25/09/2026).
//
//	PROVED HERE   rule 5, invariant 7 for all seven routes: 401 no session · 403 wrong key (for the
//	              SIGNATURE, `task.create` alone is refused) · 401 right key wrong commune · 2xx —
//	              and in the first three the use case is not reached · the actor is the staff code ·
//	              bodies reach the use case (pointers kept, dates as calendar days, empty body on the
//	              signature) · the state refusals map to 409 with the domain's own sentence and no
//	              commune id · PATCH and signature answer with the detail re-read after the commit ·
//	              `supplements_id` reaches the use case and its refusals map to 404 / 400 · the split
//	              answers 409 while the no-task mark is set.
//
//	NOT PROVED    the decisions themselves (draft vs signed, live tasks, no-op) — internal/app.

type caVongDoi struct {
	ten     string
	method  string
	duong   string
	than    any
	khoa    authz.Perm
	khoaSai authz.Perm
	ok      int
}

func duongBB(id string) string             { return duongMeetings + "/" + id }
func duongKL(id string, stt int) string    { return duongKetLuan(id) + "/" + itoaThu(stt) }
func duongDauKL(id string, stt int) string { return duongKL(id, stt) + "/no-task-marker" }

func caCacTuyenVongDoi() []caVongDoi {
	return []caVongDoi{
		{"sửa biên bản", http.MethodPatch, duongBB(idBBThu), suaBienBanVao{Title: ptr("Tên mới")},
			"task.create", "task.read", http.StatusOK},
		{"xoá biên bản", http.MethodDelete, duongBB(idBBThu), xoaBienBanVao{Reason: "Nhập trùng."},
			"task.create", "task.read", http.StatusNoContent},
		// THE KEY THAT MUST NOT OPEN THE SIGNATURE IS `task.create` — the key of every other write of
		// this register. Typing the minutes does not make an account the one who may sign them.
		{"ký biên bản", http.MethodPost, duongBB(idBBThu) + "/signature", nil,
			"task.approve", "task.create", http.StatusOK},
		{"sửa kết luận", http.MethodPatch, duongKL(idBBThu, 1), suaKetLuanVao{Content: "Câu mới."},
			"task.create", "task.read", http.StatusOK},
		{"xoá kết luận", http.MethodDelete, duongKL(idBBThu, 1), xoaBienBanVao{Reason: "Ghi nhầm."},
			"task.create", "task.read", http.StatusNoContent},
		{"đánh dấu không phát sinh", http.MethodPut, duongDauKL(idBBThu, 1), nil,
			"task.create", "task.read", http.StatusOK},
		{"bỏ dấu không phát sinh", http.MethodDelete, duongDauKL(idBBThu, 1), nil,
			"task.create", "task.update", http.StatusNoContent},
	}
}

func ptr[T any](v T) *T { return &v }

func TestTuyenVongDoiBienBanKhongCoPhienThi401(t *testing.T) {
	for _, ca := range caCacTuyenVongDoi() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			doiMa(t, m.goiGhiNV(t, ca.method, hostA, ca.duong, nil, ca.than), http.StatusUnauthorized)
			if m.ghiBienBan.goi != 0 {
				t.Errorf("đã chạm dữ liệu dù chưa có phiên")
			}
		})
	}
}

func TestTuyenVongDoiBienBanSaiQuyenThi403(t *testing.T) {
	for _, ca := range caCacTuyenVongDoi() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoaSai)
			doiMa(t, m.goiGhiNV(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than), http.StatusForbidden)
			if m.ghiBienBan.goi != 0 {
				t.Errorf("đã chạm dữ liệu dù sai quyền (%s)", ca.khoaSai)
			}
		})
	}
}

// Right key, wrong commune: 401 (authz compares the principal's commune with Host before the key),
// and NOTHING is touched — see TestTuyenGhiBienBanDungQuyenSaiXaThi401.
func TestTuyenVongDoiBienBanDungQuyenSaiXaThi401(t *testing.T) {
	for _, ca := range caCacTuyenVongDoi() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)
			doiMa(t, m.goiGhiNV(t, ca.method, hostB, ca.duong, canBoCuaXa(xaA), ca.than), http.StatusUnauthorized)
			if m.ghiBienBan.goi != 0 {
				t.Errorf("đã GHI vào sổ biên bản của xã B bằng phiên của xã A")
			}
		})
	}
}

func TestTuyenVongDoiBienBanDungQuyenDungXaThiQua(t *testing.T) {
	for _, ca := range caCacTuyenVongDoi() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)
			doiMa(t, m.goiGhiNV(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than), ca.ok)
			if m.ghiBienBan.goi != 1 || m.ghiBienBan.xa != xaA {
				t.Errorf("use case gọi %d lần, xã %q", m.ghiBienBan.goi, m.ghiBienBan.xa)
			}
			if m.ghiBienBan.nguoi.ID != maCanBo {
				t.Errorf("chủ thể = %q, muốn mã cán bộ %q", m.ghiBienBan.nguoi.ID, maCanBo)
			}
			if m.ghiBienBan.bienBanID != idBBThu {
				t.Errorf("mã biên bản tới use case = %q", m.ghiBienBan.bienBanID)
			}
		})
	}
}

// --- bodies ----------------------------------------------------------------------------------------------

func TestSuaBienBan_ThanDiXuongUseCaseVaTraChiTiet(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create")

	w := m.goiGhiNVTho(t, http.MethodPatch, hostA, duongBB(idBBThu), canBoCuaXa(xaA),
		`{"held_on":"2026-08-06","minutes_taker":"","attendees":[],"notice":{"reference_no":"12/TB-UBND","issued_on":"2026-08-12"}}`)
	doiMa(t, w, http.StatusOK)

	yc := m.ghiBienBan.ycSua
	if yc.TenCuocHop != nil || yc.NoiDung != nil {
		t.Errorf("trường không gửi vẫn tới use case: %+v", yc)
	}
	if yc.NgayHop == nil || !yc.NgayHop.Equal(time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ngày họp = %v", yc.NgayHop)
	}
	// "" CLEARS and [] EMPTIES — both must arrive as non-nil pointers.
	if yc.ThuKyMa == nil || *yc.ThuKyMa != "" || yc.ThanhPhan == nil || len(*yc.ThanhPhan) != 0 {
		t.Errorf("xoá thư ký / làm rỗng thành phần không tới use case: %+v", yc)
	}
	if yc.ThongBao == nil || yc.ThongBao.SoKyHieu != "12/TB-UBND" ||
		!yc.ThongBao.Ngay.Equal(time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("thông báo = %+v", yc.ThongBao)
	}
	// THE REPLY IS THE DETAIL, re-read after the commit: conclusions and counters included.
	b := docBienBanMot(t, w.Body.Bytes())
	if b.ID != idBBThu || len(b.Conclusions) != 3 || b.Content == nil {
		t.Errorf("phản hồi không phải bản chi tiết đọc lại: %+v", b)
	}
}

func TestSuaBienBan_NgayHongThi400KhongChamDuLieu(t *testing.T) {
	for _, than := range []string{
		`{"held_on":"05/08/2026"}`,
		`{"notice":{"reference_no":"12/TB-UBND","issued_on":"12/08/2026"}}`,
	} {
		m := dungMayChu(t)
		m.capQuyen(t, "task.create")
		doiMa(t, m.goiGhiNVTho(t, http.MethodPatch, hostA, duongBB(idBBThu), canBoCuaXa(xaA), than),
			http.StatusBadRequest)
		if m.ghiBienBan.goi != 0 {
			t.Errorf("%s: đã gọi use case", than)
		}
	}
}

func TestKyBienBan_ThanRongVaThanCoThongBao(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.approve")
	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, duongBB(idBBThu)+"/signature", canBoCuaXa(xaA), nil),
		http.StatusOK)
	if m.ghiBienBan.ycKy.ThongBao != nil {
		t.Errorf("thân rỗng mà có thông báo: %+v", m.ghiBienBan.ycKy)
	}

	m = dungMayChu(t)
	m.capQuyen(t, "task.approve")
	w := m.goiGhiNVTho(t, http.MethodPost, hostA, duongBB(idBBThu)+"/signature", canBoCuaXa(xaA),
		`{"notice":{"reference_no":"12/TB-UBND","issued_on":"2026-08-12"}}`)
	doiMa(t, w, http.StatusOK)
	if tb := m.ghiBienBan.ycKy.ThongBao; tb == nil || tb.SoKyHieu != "12/TB-UBND" {
		t.Errorf("thông báo khi ký = %+v", tb)
	}
	if b := docBienBanMot(t, w.Body.Bytes()); b.Status == "" {
		t.Errorf("phản hồi ký không mang trạng thái: %s", w.Body.String())
	}
}

func TestSuaXoaKetLuan_DuongDanVaThan(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create")
	w := m.goiGhiNV(t, http.MethodPatch, hostA, duongKL(idBBThu, 2), canBoCuaXa(xaA),
		suaKetLuanVao{Content: "Câu mới."})
	doiMa(t, w, http.StatusOK)
	if m.ghiBienBan.thuTu != 2 || m.ghiBienBan.ycSuaKL.NoiDung != "Câu mới." {
		t.Errorf("tới use case: %d / %+v", m.ghiBienBan.thuTu, m.ghiBienBan.ycSuaKL)
	}
	// THE CONCLUSION IS RE-READ with its counters: ② of the fixture has one task.
	var kl ketLuanRa
	if err := json.Unmarshal(w.Body.Bytes(), &kl); err != nil || kl.Ordinal != 2 || kl.TaskCount != 1 {
		t.Errorf("phản hồi kết luận = %+v (%v)", kl, err)
	}

	m = dungMayChu(t)
	m.capQuyen(t, "task.create")
	doiMa(t, m.goiGhiNV(t, http.MethodDelete, hostA, duongKL(idBBThu, 3), canBoCuaXa(xaA),
		xoaBienBanVao{Reason: "Ghi nhầm."}), http.StatusNoContent)
	if m.ghiBienBan.thuTu != 3 || m.ghiBienBan.lyDo != "Ghi nhầm." {
		t.Errorf("xoá kết luận tới use case: %d / %q", m.ghiBienBan.thuTu, m.ghiBienBan.lyDo)
	}

	for _, stt := range []string{"0", "hai"} {
		m = dungMayChu(t)
		m.capQuyen(t, "task.create")
		doiMa(t, m.goiGhiNV(t, http.MethodPut, hostA, duongKetLuan(idBBThu)+"/"+stt+"/no-task-marker",
			canBoCuaXa(xaA), nil), http.StatusBadRequest)
		if m.ghiBienBan.goi != 0 {
			t.Errorf("stt=%q: đã gọi use case", stt)
		}
	}
}

func TestTaoBienBan_BoSungVaThuKyDiXuongUseCase(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create")
	than := thanTaoBienBan()
	than.SupplementsID = idBBThu
	than.MinutesTaker = "CB-00042"
	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, duongMeetings, canBoCuaXa(xaA), than), http.StatusCreated)
	if m.ghiBienBan.ycTao.BoSungChoID != idBBThu || m.ghiBienBan.ycTao.ThuKyMa != "CB-00042" {
		t.Errorf("yêu cầu tới use case = %+v", m.ghiBienBan.ycTao)
	}
}

// --- refusals ---------------------------------------------------------------------------------------------

func TestLoiVongDoiBienBanAnhXaDungMa(t *testing.T) {
	boc := func(e error) error {
		return errors.Join(errors.New("bien_ban_hop: sửa cho xã 01JXAAAAAAAAAAAAAAAAAAAAAA"), e)
	}
	for _, ca := range []struct {
		ten  string
		loi  error
		muon int
		ma   string
	}{
		{"đã ký", boc(domain.ErrBienBanDaKy), http.StatusConflict, "meeting_state"},
		{"thông báo đã ghi", boc(domain.ErrDaCoThongBao), http.StatusConflict, "meeting_state"},
		{"kết luận có nhiệm vụ", boc(domain.ErrKetLuanDaCoNhiemVu), http.StatusConflict, "conclusion_tasks"},
		{"biên bản còn nhiệm vụ", boc(domain.LoiBienBanConNhiemVu(2)), http.StatusConflict, "conclusion_tasks"},
		{"gốc không có", boc(app.ErrBienBanGocKhongTonTai), http.StatusNotFound, "not_found"},
		{"gốc còn nháp", boc(domain.ErrBoSungChoBanNhap), http.StatusBadRequest, "invalid_request"},
		{"thiếu lý do", domain.ErrThieuLyDoXoaBienBan, http.StatusBadRequest, "invalid_request"},
		{"không có biên bản", petstore.ErrBienBanKhongTonTai, http.StatusNotFound, "not_found"},
		{"lỗi hệ thống", errors.New("kết nối hỏng"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "task.create")
			m.ghiBienBan.loi = ca.loi
			w := m.goiGhiNV(t, http.MethodDelete, hostA, duongBB(idBBThu), canBoCuaXa(xaA),
				xoaBienBanVao{Reason: "Nhập trùng."})
			doiMa(t, w, ca.muon)
			if e := loiTra(t, w); e.Code != ca.ma {
				t.Errorf("mã lỗi = %q, muốn %q", e.Code, ca.ma)
			}
			// THE COMMUNE ID FROM THE WRAPPING NEVER REACHES THE WIRE.
			if strings.Contains(w.Body.String(), "01JXAAAA") {
				t.Errorf("thân lỗi lộ mã xã: %s", w.Body.String())
			}
		})
	}
	// The count is in the sentence.
	m := dungMayChu(t)
	m.capQuyen(t, "task.create")
	m.ghiBienBan.loi = domain.LoiBienBanConNhiemVu(2)
	w := m.goiGhiNV(t, http.MethodDelete, hostA, duongBB(idBBThu), canBoCuaXa(xaA), xoaBienBanVao{Reason: "x"})
	if !strings.Contains(w.Body.String(), "2 nhiệm vụ") {
		t.Errorf("409 không nói còn bao nhiêu nhiệm vụ: %s", w.Body.String())
	}
}

func TestTachKetLuan_DauKhongPhatSinhThi409(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create")
	m.ghiBienBan.loi = domain.ErrKetLuanKhongPhatSinh
	w := m.goiGhiNV(t, http.MethodPost, hostA, duongTachNV(idBBThu, 1), canBoCuaXa(xaA), thanTachNhiemVu())
	doiMa(t, w, http.StatusConflict)
	if e := loiTra(t, w); e.Code != "conclusion_state" {
		t.Errorf("mã lỗi = %q", e.Code)
	}
}
