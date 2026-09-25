package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Tests for the processing logbook routes (migration 0013) and for the optional note on the six act
// bodies.
//
//	PROVED HERE   rule 5 invariant 7 on BOTH new routes — 401 no session · 403 wrong key · 401 right
//	              key WRONG COMMUNE (this package's convention: authz compares the commune before the
//	              key) · 200/201 — with the data untouched in the first three · the read goes through
//	              the petition read first (404 for unknown / other commune / `can-bo` without
//	              `feedback.restricted`) · the wire shape · the note-writer FACT is read from exactly
//	              resolve/assign/classify · the refusals map to 403 / 404 / 400 · the note reaches the
//	              use case from all six act bodies · no note text in a log line · the citizen mux mounts
//	              no logbook route.
//
//	NOT PROVED    who may write a note — app.duocGhiChu decides it on the locked row, and the matrix is
//	              proved over the real store in internal/app/nhat_ky_phan_anh_test.go.

// nhatKyPhieuGia is the logbook read, KEYED BY COMMUNE AND BY PETITION ID, reading the commune from
// the context exactly as *store.Scoped does.
type nhatKyPhieuGia struct {
	theo    map[tenant.ID]map[string][]domain.NhatKyPhanAnh
	loi     error
	goi     int
	phieuID string
}

func (n *nhatKyPhieuGia) NhatKyCuaPhieu(ctx context.Context, phieuID string, _ page.Request) (
	page.Result[domain.NhatKyPhanAnh], error) {

	n.goi++
	n.phieuID = phieuID
	if n.loi != nil {
		return page.NewResult[domain.NhatKyPhanAnh](), n.loi
	}
	ra := page.NewResult[domain.NhatKyPhanAnh]()
	ra.Items = append(ra.Items, n.theo[tenant.MustFrom(ctx)][phieuID]...)
	return ra, nil
}

var (
	mocNK1 = time.Date(2026, 9, 26, 2, 10, 0, 0, time.UTC)
	mocNK2 = time.Date(2026, 9, 26, 3, 20, 0, 0, time.UTC)
)

// nhatKyMau gives commune A's ordinary petition (`pa-001`) two rows, NEWEST FIRST as the store
// returns them, and the restricted one (`pa-002`) one row. Commune B has a row under the SAME
// internal id `pa-001` — so a handler that forgot the commune would show it.
func nhatKyMau() *nhatKyPhieuGia {
	return &nhatKyPhieuGia{theo: map[tenant.ID]map[string][]domain.NhatKyPhanAnh{
		xaA: {
			"pa-001": {
				{ID: "nk-2", PhieuPhanAnhID: "pa-001", ThoiDiem: mocNK2, NguoiMa: maCanBo,
					HanhVi: domain.NhatKyPhanCong, TrangThai: domain.DaChuyenXuLy,
					BoPhanID: "bp-001", CanBoXuLyMa: "CB-00777"},
				{ID: "nk-1", PhieuPhanAnhID: "pa-001", ThoiDiem: mocNK1, NguoiMa: maCanBo,
					HanhVi: domain.NhatKyPhanLoai, TrangThai: domain.DangPhanLoai,
					NoiDung: "Đã gọi xác minh với tổ trưởng."},
			},
			"pa-002": {
				{ID: "nk-hc", PhieuPhanAnhID: "pa-002", ThoiDiem: mocNK1, NguoiMa: maCanBo,
					HanhVi: domain.NhatKyGhiChu, TrangThai: domain.DangPhanLoai, NoiDung: "Ghi chú hạn chế."},
			},
		},
		xaB: {
			"pa-001": {{ID: "nk-b", PhieuPhanAnhID: "pa-001", ThoiDiem: mocNK1, NguoiMa: "CB-B0001",
				HanhVi: domain.NhatKyGhiChu, TrangThai: domain.DaTiepNhan, NoiDung: "Của xã B."}},
		},
	}}
}

func duongNhatKy(ma string) string { return duong(ma) + "/log-entries" }

// --- rule 5, invariant 7: four cases per route ----------------------------------------------------

func caTuyenNhatKy() []caTuyen {
	return []caTuyen{
		{"đọc nhật ký", http.MethodGet, duongNhatKy(maPhieuThuong), nil,
			authz.Perm("feedback.read"), authz.Perm("feedback.resolve")},
		// The gate is `feedback.read`: an account holding ONLY a commune-wide working key and not the
		// read key is refused at the gate, like `…/status`.
		{"ghi chú", http.MethodPost, duongNhatKy(maPhieuThuong), ghiChuPhieuVao{Note: "Đã gọi lại."},
			authz.Perm("feedback.read"), authz.Perm("feedback.classify")},
	}
}

// chamDuLieu is every fake a logbook route could reach.
func (m *mayChu) chamDuLieu() int { return m.phieu.goi + m.nhatKy.goi + m.xuLy.goi }

func TestTuyenNhatKyKhongCoPhienThi401(t *testing.T) {
	for _, ca := range caTuyenNhatKy() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			w := m.goiGhiNV(t, ca.method, hostA, ca.duong, nil, ca.than)
			doiMa(t, w, http.StatusUnauthorized)
			if m.chamDuLieu() != 0 {
				t.Error("đã chạm dữ liệu dù chưa có phiên")
			}
		})
	}
}

func TestTuyenNhatKySaiQuyenThi403(t *testing.T) {
	for _, ca := range caTuyenNhatKy() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoaSai)
			w := m.goiGhiNV(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than)
			doiMa(t, w, http.StatusForbidden)
			if m.chamDuLieu() != 0 {
				t.Error("đã chạm dữ liệu dù sai quyền")
			}
		})
	}
}

// 401 AND NOT 403 — this package's convention (TestTuyenXuLyDungQuyenSaiXaThi401): the commune on the
// principal is compared with the one resolved from Host BEFORE the key is consulted.
func TestTuyenNhatKyDungQuyenSaiXaThi401(t *testing.T) {
	for _, ca := range caTuyenNhatKy() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)
			w := m.goiGhiNV(t, ca.method, hostB, ca.duong, canBoCuaXa(xaA), ca.than)
			doiMa(t, w, http.StatusUnauthorized)
			if m.chamDuLieu() != 0 {
				t.Error("đã chạm dữ liệu của xã B bằng phiên của xã A — rò rỉ giữa hai cơ quan nhà nước")
			}
		})
	}
}

func TestTuyenNhatKyDungQuyenDungXa(t *testing.T) {
	for ca, muon := range map[int]int{0: http.StatusOK, 1: http.StatusCreated} {
		c := caTuyenNhatKy()[ca]
		t.Run(c.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, c.khoa)
			doiMa(t, m.goiGhiNV(t, c.method, hostA, c.duong, canBoCuaXa(xaA), c.than), muon)
		})
	}
}

// --- the read -----------------------------------------------------------------------------------------

func docTrangNhatKy(t *testing.T, than []byte) (page.Result[nhatKyPhieuRa], []map[string]any) {
	t.Helper()
	var ra page.Result[nhatKyPhieuRa]
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	var tho struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(than, &tho)
	return ra, tho.Items
}

func TestDocNhatKyTraDungHinhDangVaThuTu(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, duongNhatKy(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	if m.nhatKy.phieuID != "pa-001" {
		t.Errorf("đọc nhật ký theo %q, muốn id NỘI BỘ của phiếu vừa đọc (pa-001)", m.nhatKy.phieuID)
	}
	ra, tho := docTrangNhatKy(t, w.Body.Bytes())
	if len(ra.Items) != 2 || ra.Items[0].ID != "nk-2" || ra.Items[1].ID != "nk-1" {
		t.Fatalf("items = %+v, muốn [nk-2 nk-1] đúng thứ tự kho trả", ra.Items)
	}
	pc, pl := ra.Items[0], ra.Items[1]
	if pc.Action != "phan-cong" || pc.Status != "da-chuyen-xu-ly" || pc.Unit != "bp-001" ||
		pc.Assignee != "CB-00777" || pc.ActorCode != maCanBo || !pc.At.Equal(mocNK2) || pc.Note != "" {
		t.Errorf("dòng phân công = %+v", pc)
	}
	if pl.Action != "phan-loai" || pl.Unit != "" || pl.Assignee != "" ||
		pl.Note != "Đã gọi xác minh với tổ trưởng." {
		t.Errorf("dòng phân loại = %+v", pl)
	}
	for _, dong := range tho {
		if v, co := dong["attachments"].([]any); !co || len(v) != 0 {
			t.Errorf("attachments = %#v, muốn [] (không phải null)", dong["attachments"])
		}
		// The precedent resolves no names — the field must not appear as an empty promise.
		if _, co := dong["actor_name"]; co {
			t.Error("có actor_name dù không phân giải tên qua identity")
		}
		for _, k := range []string{"id", "at", "actor_code", "action", "status", "unit", "assignee", "note"} {
			if _, co := dong[k]; !co {
				t.Errorf("thiếu trường %q", k)
			}
		}
	}
}

func TestDocNhatKyRongThiItemsLaMangRong(t *testing.T) {
	m := dungMayChu(t)
	m.nhatKy.theo[xaA]["pa-001"] = nil
	w := m.goi(t, http.MethodGet, hostA, duongNhatKy(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("nhật ký rỗng phải là items: [] — thân: %s", w.Body.String())
	}
}

// ONE 404 FOR FOUR CAUSES, and in none of them is the logbook read at all.
func TestDocNhatKy404(t *testing.T) {
	for ten, ma := range map[string]string{
		"mã không tồn tại":                      "PA-ZZZZ-ZZZZ-ZZZZ",
		"mã của xã khác":                        maPhieuXaB,
		"lĩnh vực hạn chế, không có restricted": maPhieuCanBo,
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t)
			w := m.goi(t, http.MethodGet, hostA, duongNhatKy(ma), canBoCuaXa(xaA))
			doiMa(t, w, http.StatusNotFound)
			if m.nhatKy.goi != 0 {
				t.Error("đã đọc nhật ký dù phiếu không được phép thấy")
			}
			if !strings.Contains(w.Body.String(), "Không tìm thấy phiếu phản ánh.") {
				t.Errorf("không phải câu 404 chung: %s", w.Body.String())
			}
		})
	}
}

func TestDocNhatKyHanCheCoQuyenThi200(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"), QuyenHanChe)
	w := m.goi(t, http.MethodGet, hostA, duongNhatKy(maPhieuCanBo), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if ra, _ := docTrangNhatKy(t, w.Body.Bytes()); len(ra.Items) != 1 || ra.Items[0].ID != "nk-hc" {
		t.Errorf("items = %+v", ra.Items)
	}
}

func TestDocNhatKyConTroSaiThi400KhongChamKho(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, duongNhatKy(maPhieuThuong)+"?sort=noi_dung", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusBadRequest)
	if m.phieu.goi != 0 || m.nhatKy.goi != 0 {
		t.Error("đã chạm kho dù yêu cầu phân trang sai")
	}
}

func TestDocNhatKyKhoHongThi500KhongLoNoiDung(t *testing.T) {
	m := dungMayChu(t)
	var nhatKy bytes.Buffer
	m.dungLai(t, func(d *Deps) { d.Log = slog.New(slog.NewTextHandler(&nhatKy, nil)) })
	m.nhatKy.loi = errors.New("kho hỏng")
	w := m.goi(t, http.MethodGet, hostA, duongNhatKy(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(nhatKy.String(), maPhieuThuong) || strings.Contains(w.Body.String(), "kho hỏng") {
		t.Error("nhật ký hệ thống hoặc thân lỗi lộ mã tra cứu / lỗi nội bộ")
	}
}

// --- the write ------------------------------------------------------------------------------------------

// The FACT handed down is true for exactly the three commune-wide petition keys, and false for a
// neighbouring key — so a handler reading the wrong key fails here.
func TestGhiChuDocDungBaKhoaQuyenCaXa(t *testing.T) {
	for ten, ca := range map[string]struct {
		them authz.Perm
		muon app.QuyenGhiChuCaXa
	}{
		"chỉ feedback.read":        {"", false},
		"thêm feedback.resolve":    {QuyenXuLyCaXa, true},
		"thêm feedback.assign":     {QuyenPhanCongPhieu, true},
		"thêm feedback.classify":   {QuyenPhanLoaiPhieu, true},
		"thêm feedback.unmask":     {QuyenXemDayDu, false},
		"thêm feedback.restricted": {QuyenHanChe, false},
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t)
			khoa := []authz.Perm{"feedback.read"}
			if ca.them != "" {
				khoa = append(khoa, ca.them)
			}
			m.capQuyen(t, khoa...)
			w := m.goiGhiNV(t, http.MethodPost, hostA, duongNhatKy(maPhieuThuong), canBoCuaXa(xaA),
				ghiChuPhieuVao{Note: "Đã gọi lại."})
			doiMa(t, w, http.StatusCreated)
			if m.xuLy.quyenGhiChu != ca.muon {
				t.Errorf("quyền cả xã báo xuống = %v, muốn %v", m.xuLy.quyenGhiChu, ca.muon)
			}
			if bool(m.xuLy.hanChe) != (ca.them == QuyenHanChe) {
				t.Errorf("sự thật hạn chế = %v", m.xuLy.hanChe)
			}
		})
	}
}

func TestGhiChuTraVe201VaNguoiLaMaCanBo(t *testing.T) {
	m := dungMayChu(t)
	w := m.goiGhiNV(t, http.MethodPost, hostA, duongNhatKy(maPhieuThuong), canBoCuaXa(xaA),
		ghiChuPhieuVao{Note: "Đã gọi lại cho người phản ánh."})
	doiMa(t, w, http.StatusCreated)
	if m.xuLy.nguoi.ID != maCanBo || m.xuLy.maDa != maPhieuThuong || m.xuLy.xa != xaA {
		t.Errorf("xuống use case: người %q, mã %q, xã %q", m.xuLy.nguoi.ID, m.xuLy.maDa, m.xuLy.xa)
	}
	if m.xuLy.ghiChu != "Đã gọi lại cho người phản ánh." {
		t.Errorf("ghi chú xuống use case = %q", m.xuLy.ghiChu)
	}
	var ra nhatKyPhieuRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân: %s", w.Body.String())
	}
	if ra.Action != "ghi-chu" || ra.ActorCode != maCanBo || ra.Note != m.xuLy.ghiChu ||
		ra.Attachments == nil {
		t.Errorf("thân 201 = %+v", ra)
	}
}

func TestGhiChuThieuIdempotencyKeyThi400(t *testing.T) {
	m := dungMayChu(t)
	w := m.goiThan(t, http.MethodPost, hostA, duongNhatKy(maPhieuThuong), canBoCuaXa(xaA),
		ghiChuPhieuVao{Note: "Đã gọi lại."})
	doiMa(t, w, http.StatusBadRequest)
	if m.xuLy.goi != 0 {
		t.Error("use case chạy dù thiếu Idempotency-Key — bấm hai lần sẽ để lại hai dòng không xoá được")
	}
}

func TestGhiChuTuChoiAnhXaDungMa(t *testing.T) {
	for ten, ca := range map[string]struct {
		loi    error
		status int
	}{
		"không phải người được giao": {app.ErrKhongPhaiNguoiDuocGiao, http.StatusForbidden},
		"lĩnh vực hạn chế":           {app.ErrPhieuHanChe, http.StatusNotFound},
		"ghi chú rỗng":               {domain.ErrThieuGhiChu, http.StatusBadRequest},
		"ghi chú quá dài":            {fmt.Errorf("%w (tối đa 2000 ký tự)", domain.ErrGhiChuQuaDai), http.StatusBadRequest},
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.xuLy.loi = fmt.Errorf("xu_ly_phan_anh: ghi chú cho xã %s: %w", xaA, ca.loi)
			w := m.goiGhiNV(t, http.MethodPost, hostA, duongNhatKy(maPhieuThuong), canBoCuaXa(xaA),
				ghiChuPhieuVao{Note: "x"})
			doiMa(t, w, ca.status)
			if strings.Contains(w.Body.String(), string(xaA)) {
				t.Errorf("thân lỗi lộ mã xã: %s", w.Body.String())
			}
		})
	}
}

// Rule 3: the note may hold personal data and NO log line carries it — not on a refusal, not on a
// failure. The agreed fake number (rule 3, invariant 5) is the marker.
func TestGhiChuKhongVaoNhatKyHeThong(t *testing.T) {
	const ghiChuCoSo = "Ông An, số 0900000000, xin gọi lại buổi chiều."
	for ten, loi := range map[string]error{
		"từ chối":      domain.ErrGhiChuQuaDai,
		"lỗi hệ thống": errors.New("kho hỏng"),
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t)
			var nhatKy bytes.Buffer
			m.dungLai(t, func(d *Deps) { d.Log = slog.New(slog.NewTextHandler(&nhatKy, nil)) })
			m.xuLy.loi = loi
			w := m.goiGhiNV(t, http.MethodPost, hostA, duongNhatKy(maPhieuThuong), canBoCuaXa(xaA),
				ghiChuPhieuVao{Note: ghiChuCoSo})
			if w.Code < 400 {
				t.Fatalf("mã = %d, muốn lỗi", w.Code)
			}
			if nhatKy.Len() == 0 {
				t.Fatal("không có dòng log nào — phép kiểm này sẽ xanh vì lý do sai")
			}
			for _, cam := range []string{"0900000000", "Ông An", maPhieuThuong} {
				if strings.Contains(nhatKy.String(), cam) || strings.Contains(w.Body.String(), cam) {
					t.Errorf("lộ %q ra log hoặc thân lỗi", cam)
				}
			}
		})
	}
}

// --- the optional note on the six act bodies -------------------------------------------------------------

func TestSauTuyenHanhViChuyenGhiChuXuongUseCase(t *testing.T) {
	const g = "Ghi chú nội bộ của bước này."
	for _, ca := range []struct {
		ten   string
		khoa  authz.Perm
		duong string
		than  any
		lay   func(x *xuLyPhieuGia) string
	}{
		{"phân loại", "feedback.classify", duongPhanLoai(maPhieuThuong),
			phanLoaiVao{Field: "rac-thai", Note: g}, func(x *xuLyPhieuGia) string { return x.ycLinhVuc.GhiChu }},
		{"phân công", "feedback.assign", duongPhanCong(maPhieuThuong),
			phanCongVao{Unit: "bp-001", Note: g}, func(x *xuLyPhieuGia) string { return x.ycPhanCong.GhiChu }},
		{"chuyển trạng thái", "feedback.read", duongTienTrang(maPhieuThuong),
			tienTrangThaiVao{Note: g}, func(x *xuLyPhieuGia) string { return x.ghiChu }},
		{"đóng phiếu", "feedback.resolve", duongDong(maPhieuThuong),
			dongPhieuVao{Result: ketQuaThat, Note: g}, func(x *xuLyPhieuGia) string { return x.ghiChu }},
		{"không tiếp nhận", "feedback.classify", duongKhongTiepNhan(maPhieuThuong),
			khongTiepNhanVao{Reason: lyDoThatHTTP, Note: g}, func(x *xuLyPhieuGia) string { return x.ghiChu }},
		{"chuyển cấp trên", "feedback.classify", duongChuyenCapTren(maPhieuThuong),
			chuyenCapTrenVao{Reason: lyDoThatHTTP, ReceivingBody: coQuanThatHTTP, Note: g},
			func(x *xuLyPhieuGia) string { return x.ycChuyen.GhiChu }},
	} {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)
			doiMa(t, m.goiThan(t, http.MethodPost, hostA, ca.duong, canBoCuaXa(xaA), ca.than), http.StatusOK)
			if got := ca.lay(m.xuLy); got != g {
				t.Errorf("ghi chú xuống use case = %q, muốn %q", got, g)
			}
		})
	}
}

// The status route's body is OPTIONAL: none is the ordinary call (existing clients), and a malformed
// one is still refused.
func TestTienTrangThaiThanTuyChon(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.read")
	doiMa(t, m.goiThan(t, http.MethodPost, hostA, duongTienTrang(maPhieuThuong), canBoCuaXa(xaA), nil),
		http.StatusOK)
	if m.xuLy.ghiChu != "" {
		t.Errorf("không gửi thân mà ghi chú = %q", m.xuLy.ghiChu)
	}

	r := httptest.NewRequest(http.MethodPost, "https://"+hostA+duongTienTrang(maPhieuThuong),
		strings.NewReader("{không phải json"))
	r.Host = hostA
	r = r.WithContext(authz.Into(r.Context(), *canBoCuaXa(xaA)))
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	doiMa(t, w, http.StatusBadRequest)
}

// The optional `note` stays OPTIONAL on every published request type, and the manual note's is
// mandatory: `omitempty` is what tools/apidoc reads (schema.go, boQuaKhiRong).
func TestTruongNoteTuyChonTrenSauThan(t *testing.T) {
	for _, than := range []any{phanLoaiVao{Field: "x"}, phanCongVao{Unit: "x"}, tienTrangThaiVao{},
		dongPhieuVao{Result: "x"}, khongTiepNhanVao{Reason: "x"},
		chuyenCapTrenVao{Reason: "x", ReceivingBody: "x"}} {
		b, _ := json.Marshal(than)
		if strings.Contains(string(b), `"note"`) {
			t.Errorf("%T phát ra `note` khi rỗng — trường mới phải tuỳ chọn: %s", than, b)
		}
	}
	b, _ := json.Marshal(ghiChuPhieuVao{})
	if !strings.Contains(string(b), `"note"`) {
		t.Error("ghiChuPhieuVao.note phải bắt buộc (không omitempty)")
	}
}

// --- rule 4, forbidden #5: never on the citizen mux ----------------------------------------------------

func TestTuyenCongDanKhongCoNhatKy(t *testing.T) {
	mux := http.NewServeMux()
	RegisterCongDan(mux, DepsCongDan{
		Phieu: phieuCuaToiMau(), GuiPhieu: soPhieuMoi(), NhanLinhVuc: nhanLinhVucMau(),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	for _, p := range []string{
		"/api/v1/my-citizen-reports/" + maPhieuThuong + "/log-entries",
		"/api/v1/citizen-reports/" + maPhieuThuong + "/log-entries",
	} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			r := httptest.NewRequest(method, "https://"+hostMiniApp+p, nil)
			if _, mau := mux.Handler(r); mau != "" {
				t.Errorf("mux công dân có tuyến %q cho %s %s — nhật ký xử lý là nội bộ cán bộ", mau, method, p)
			}
		}
	}
}
