package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// GET /api/v1/commune-staff?host= — the public Mini App staff directory.
//
// WHICH ROWS are published (not locked, not soft-deleted, published WITH consent) is the store's
// predicate and is proven there: internal/store/danh_ba_cong_khai_test.go (the statement) and
// danh_ba_cong_khai_pg_test.go (the rows, against PostgreSQL). What is proven HERE is the handler's
// half: the commune the host resolves to is the one — and the only one — whose directory is read; the
// wire carries exactly the allowed keys; every negative answers one shape; nothing personal is logged.
//
// Rule 5 invariant 7 adapted to Public: there is no 401 and no 403 (no principal, no permission);
// "right permission, wrong commune" becomes "commune A's host never returns commune B's staff".

// Every number is the agreed fake (rule 3, invariant 5).
func danhBaHaiXa() *danhBaCongKhaiGia {
	return &danhBaCongKhaiGia{theoXa: map[tenant.ID][]domain.CanBoCongKhai{
		xaQR: {
			{HoTen: "Nguyễn Văn A", ChucVu: "Chủ tịch UBND xã", TenBoPhan: "LÃNH ĐẠO UBND XÃ",
				DienThoaiCoQuan: "0900000000", DiDongCaNhan: "0900000000", CoZalo: true},
			// Has Zalo ticked but published no mobile: has_zalo must NOT be sent true.
			{HoTen: "Trần Thị B", ChucVu: "Văn thư", TenBoPhan: "",
				DienThoaiCoQuan: "0900000000", DiDongCaNhan: "", CoZalo: true},
		},
		xaB: {
			{HoTen: "Người Của Xã B", ChucVu: "Công chức", TenBoPhan: "VĂN PHÒNG XÃ B",
				DienThoaiCoQuan: "0900000000", DiDongCaNhan: "0900000000"},
		},
	}}
}

func goiDanhBa(h http.Handler, query string) *httptest.ResponseRecorder {
	return goiCongKhai(h, MauDanhBaCongKhai, query)
}

type danhBaRaThu struct {
	Items []map[string]any `json:"items"`
}

func docDanhBa(t *testing.T, w *httptest.ResponseRecorder) danhBaRaThu {
	t.Helper()
	var ra danhBaRaThu
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	return ra
}

// --- 200 ---------------------------------------------------------------------------------------

func TestDanhBaCongKhaiTraDungXaCuaTenMien(t *testing.T) {
	db := danhBaHaiXa()
	h := dungChuoiCongKhai(t, &nenTangXaGia{}, db)

	w := goiDanhBa(h, qHost(hostQR))
	doiMa(t, w, http.StatusOK)
	ra := docDanhBa(t, w)
	if len(ra.Items) != 2 || ra.Items[0]["full_name"] != "Nguyễn Văn A" || ra.Items[1]["full_name"] != "Trần Thị B" {
		t.Fatalf("xã QR nhận %v, muốn đúng hai người của xã QR, giữ thứ tự của kho", ra.Items)
	}
	if strings.Contains(w.Body.String(), "Người Của Xã B") {
		t.Fatalf("RÒ RỈ GIỮA HAI XÃ: tên miền xã QR trả cán bộ xã B: %s", w.Body.String())
	}

	wb := goiDanhBa(h, qHost(hostXaB))
	doiMa(t, wb, http.StatusOK)
	rb := docDanhBa(t, wb)
	if len(rb.Items) != 1 || rb.Items[0]["full_name"] != "Người Của Xã B" {
		t.Fatalf("xã B nhận %v, muốn đúng một người của xã B", rb.Items)
	}
	if strings.Contains(wb.Body.String(), "Nguyễn Văn A") {
		t.Fatalf("RÒ RỈ GIỮA HAI XÃ: tên miền xã B trả cán bộ xã QR: %s", wb.Body.String())
	}
}

func TestDanhBaCongKhaiChiTraDungCacTruongDuocPhep(t *testing.T) {
	h := dungChuoiCongKhai(t, &nenTangXaGia{}, danhBaHaiXa())
	w := goiDanhBa(h, qHost(hostQR))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, string(xaQR)) {
		t.Fatalf("phản hồi chứa tenant_id: %s", than)
	}
	// Owner decision 2026-09-27: EXACTLY these. A new key is a new publication question, not a free
	// addition — email, code, ids, consent marks and account state are refused outright.
	muon := "department_name,full_name,has_zalo,mobile,phone,position"
	for i, it := range docDanhBa(t, w).Items {
		var khoa []string
		for k := range it {
			khoa = append(khoa, k)
		}
		sort.Strings(khoa)
		if got := strings.Join(khoa, ","); got != muon {
			t.Fatalf("dòng %d có các trường %s, muốn đúng %s", i, got, muon)
		}
	}
	for _, cam := range []string{`"id"`, `"code"`, `"email"`, `"published"`, `"consent`, `"active"`,
		`"has_account"`, `"department_id"`, `"display_order"`} {
		if strings.Contains(than, cam) {
			t.Errorf("phản hồi mang trường cấm %s: %s", cam, than)
		}
	}
}

func TestDanhBaCongKhaiCoZaloChiDiCungSoDiDong(t *testing.T) {
	h := dungChuoiCongKhai(t, &nenTangXaGia{}, danhBaHaiXa())
	ra := docDanhBa(t, goiDanhBa(h, qHost(hostQR)))
	if ra.Items[0]["has_zalo"] != true || ra.Items[0]["mobile"] != "0900000000" {
		t.Fatalf("người có di động + Zalo: %v", ra.Items[0])
	}
	if ra.Items[1]["has_zalo"] != false || ra.Items[1]["mobile"] != "" {
		t.Fatalf("\"Có Zalo\" gửi đi dưới một số di động không công khai: %v", ra.Items[1])
	}
}

// --- one answer for every negative -------------------------------------------------------------

func TestDanhBaCongKhaiTenMienKhongCoDanhRiengNgungHoatDongTraRongGiongXaChuaCongKhaiAi(t *testing.T) {
	// The byte-identical reference: an ACTIVE commune that has published nobody. If the three
	// negatives differ from it, the route answers "is this domain a commune".
	db := &danhBaCongKhaiGia{theoXa: map[tenant.ID][]domain.CanBoCongKhai{}}
	h := dungChuoiCongKhai(t, &nenTangXaGia{}, db)

	mau := goiDanhBa(h, qHost(hostQR))
	doiMa(t, mau, http.StatusOK)
	if strings.TrimSpace(mau.Body.String()) != `{"items":[]}` {
		t.Fatalf("xã chưa công khai ai trả %s, muốn {\"items\":[]}", mau.Body.String())
	}
	goiTruoc := db.goi
	for _, host := range []string{hostKhongCo, hostDanhRieng, hostNgung} {
		w := goiDanhBa(h, qHost(host))
		if w.Code != mau.Code || w.Body.String() != mau.Body.String() {
			t.Fatalf("%s trả %d %s, khác %d %s", host, w.Code, w.Body.String(), mau.Code, mau.Body.String())
		}
	}
	if db.goi != goiTruoc {
		// hostNgung resolves to a real (inactive) commune id: reading its directory would publish a
		// merged commune's staff under a domain that no longer serves it.
		t.Fatalf("kho danh bạ bị đọc %d lần cho tên miền không thuộc xã đang hoạt động nào", db.goi-goiTruoc)
	}
}

// --- 400 / 503 before any read -----------------------------------------------------------------

func TestDanhBaCongKhaiHostSaiHinhDangLa400KhongHoiNenTangKhongDocKho(t *testing.T) {
	nt, db := &nenTangXaGia{}, danhBaHaiXa()
	h := dungChuoiCongKhai(t, nt, db)
	for _, q := range hostSaiHinhDang {
		w := goiDanhBa(h, q)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%q: mã = %d, muốn 400 — thân: %s", q, w.Code, w.Body.String())
		}
	}
	if nt.goi != 0 || db.goi != 0 {
		t.Fatalf("tên miền sai hình dạng: nền tảng bị hỏi %d lần, kho bị đọc %d lần", nt.goi, db.goi)
	}
}

func TestDanhBaCongKhaiNenTangChetLa503KhongDocKho(t *testing.T) {
	db := danhBaHaiXa()
	h := dungChuoiCongKhai(t, &nenTangXaGia{chet: true}, db)
	w := goiDanhBa(h, qHost(hostQR))
	doiMa(t, w, http.StatusServiceUnavailable)
	if strings.Contains(w.Body.String(), "items") {
		t.Fatalf("503 mang hình dạng danh sách: %s", w.Body.String())
	}
	if db.goi != 0 {
		t.Fatalf("nền tảng chết mà kho vẫn bị đọc %d lần — xã lấy từ đâu?", db.goi)
	}
}

// --- 500, and nothing personal in the log or the body -------------------------------------------

func TestDanhBaCongKhaiLoiKhoLa500KhongLoDuLieuCaNhan(t *testing.T) {
	for ten, loi := range map[string]error{
		"vượt trần":    fmt.Errorf("bọc: %w", idstore.ErrQuaNhieuCanBoCongKhai),
		"lỗi hệ thống": errors.New("pq: connection reset"),
	} {
		var nhatKy bytes.Buffer
		mux := http.NewServeMux()
		RegisterCongKhai(mux, DepsCongKhai{
			Xa:     &nenTangXaGia{},
			DanhBa: &danhBaCongKhaiGia{loi: loi},
			Log:    slog.New(slog.NewJSONHandler(&nhatKy, nil)),
		})
		w := goiDanhBa(mux, qHost(hostQR))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("%s: mã = %d, muốn 500", ten, w.Code)
		}
		if strings.Contains(w.Body.String(), "items") || strings.Contains(w.Body.String(), "pq:") {
			t.Fatalf("%s: thân 500 lộ chi tiết hoặc mang danh sách: %s", ten, w.Body.String())
		}
		if nhatKy.Len() == 0 {
			t.Fatalf("%s: lỗi không để lại dòng nhật ký nào", ten)
		}
		for _, cam := range []string{"0900000000", "Nguyễn", "Trần"} {
			if strings.Contains(nhatKy.String(), cam) {
				t.Fatalf("%s: nhật ký chứa dữ liệu cá nhân %q: %s", ten, cam, nhatKy.String())
			}
		}
	}
}
