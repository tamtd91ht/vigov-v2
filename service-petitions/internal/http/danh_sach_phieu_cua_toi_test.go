package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/page"
)

// The CITIZEN list: GET /api/v1/my-citizen-reports — "Phản ánh của tôi".
//
// SAME HARNESS AS phieu_cua_toi_test.go — the real citizen chain, identity and commune from a bearer
// token, never an injected principal (except in the one test whose point is a STAFF principal).
//
//	PROVED HERE   401 with no session · a staff principal is refused 401 by authz.CitizenOnly and
//	              the store is not reached · citizen A never sees citizen B's petition in the same
//	              commune · the same citizen in commune B's session gets `items: []` and the store was
//	              asked with commune B · identity in the query string is ignored · the cursor
//	              round-trips across two pages · a bad status / order / sort is 400 before the store ·
//	              no staff-internal or reporter key on an item · the excerpt is cut in runes.
//
//	NOT PROVED    the SQL predicates, soft-delete and the staff-booked exclusion — those live in the
//	              statement and are asserted in internal/store/phieu_cua_toi_danh_sach_test.go (and
//	              against PostgreSQL in the _pg_test beside it). The fake here applies them in Go.

const duongDanhSachCuaToi = "/api/v1/my-citizen-reports"

func docTrangCuaToi(t *testing.T, than []byte) page.Result[phieuCuaToiTomTatRa] {
	t.Helper()
	var ra page.Result[phieuCuaToiTomTatRa]
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân phản hồi không phải JSON: %q", string(than))
	}
	return ra
}

func maTrongTrangCuaToi(kq page.Result[phieuCuaToiTomTatRa]) []string {
	ra := make([]string, 0, len(kq.Items))
	for _, p := range kq.Items {
		ra = append(ra, p.Code)
	}
	return ra
}

// --- 401 ---------------------------------------------------------------------------------------

func TestDanhSachCuaToiKhongPhienLa401VaKhoKhongBiChamToi(t *testing.T) {
	for ten, token := range map[string]string{
		"không có header Authorization": "",
		"token sổ phiên không nhận":     "token-khong-ai-cap-BAO-GIO",
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChuCongDan(t)
			doiMa(t, m.goi(t, duongDanhSachCuaToi, token), http.StatusUnauthorized)
			if m.phieu.goi != 0 {
				t.Errorf("kho bị đọc %d lần dù không có phiên dùng được", m.phieu.goi)
			}
		})
	}
}

func TestDanhSachCuaToiPhienChuaChonXaLa401(t *testing.T) {
	m := dungMayChuCongDan(t)
	m.h = chuoiCongDanVoi(t, m, phienKhongXa{})
	doiMa(t, m.goi(t, duongDanhSachCuaToi, "bat-ky-token-nao"), http.StatusUnauthorized)
	if m.phieu.goi != 0 {
		t.Errorf("kho bị đọc %d lần dù phiên chưa gắn xã nào", m.phieu.goi)
	}
}

// TestDanhSachCuaToiPrincipalCanBoBiTuChoi — a STAFF principal on the citizen route. Citizens hold no
// permissions (rule 5, invariant 6), so this route's "wrong kind of caller" is authz.CitizenOnly's
// 401, not a 403. The store is not reached: a staff id used as a citizen identifier would match
// nothing and look like a working, empty list.
func TestDanhSachCuaToiPrincipalCanBoBiTuChoi(t *testing.T) {
	m := dungMayChuCongDan(t)
	mux := http.NewServeMux()
	RegisterCongDan(mux, DepsCongDan{
		Phieu: m.phieu, GuiPhieu: soPhieuMoi(), NhanLinhVuc: nhanLinhVucMau(),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	canBo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := authz.Into(r.Context(), authz.Principal{
			ID: "nd-01JCANBO", Ma: "CB-00123", Kind: "staff", TenantID: xaA,
		})
		mux.ServeHTTP(w, r.WithContext(ctx))
	})

	r := httptest.NewRequest(http.MethodGet, "https://"+hostMiniApp+duongDanhSachCuaToi, nil)
	w := httptest.NewRecorder()
	canBo.ServeHTTP(w, r)

	doiMa(t, w, http.StatusUnauthorized)
	if m.phieu.goi != 0 {
		t.Errorf("kho bị đọc %d lần cho một principal CÁN BỘ trên tuyến công dân", m.phieu.goi)
	}
}

// --- isolation ---------------------------------------------------------------------------------

// TestDanhSachCuaToiChiPhieuCuaChinhMinh — citizen A, commune A: their two petitions, never the
// other citizen's, newest first with the id breaking the tie (both fixtures share one instant).
func TestDanhSachCuaToiChiPhieuCuaChinhMinh(t *testing.T) {
	m := dungMayChuCongDan(t)

	w := m.goi(t, duongDanhSachCuaToi, tokenCuaToi)
	doiMa(t, w, http.StatusOK)
	kq := docTrangCuaToi(t, w.Body.Bytes())

	if got := maTrongTrangCuaToi(kq); len(got) != 2 || got[0] != maAnDanh || got[1] != maCuaToi {
		t.Fatalf("mã trong trang = %v, muốn [%s %s] — chỉ phiếu của mình, mới nhất trước, phá hoà theo id",
			got, maAnDanh, maCuaToi)
	}
	if strings.Contains(w.Body.String(), maCuaNguoiKhac) || strings.Contains(w.Body.String(), "công dân khác") {
		t.Errorf("phiếu của công dân khác lọt vào danh sách: %s", w.Body.String())
	}
	if kq.HasMore || kq.NextCursor != "" {
		t.Errorf("has_more=%v cursor=%q trên trang cuối", kq.HasMore, kq.NextCursor)
	}
	if len(m.phieu.thayCongDan) != 1 || m.phieu.thayCongDan[0] != idToi {
		t.Errorf("định danh xuống kho = %v, muốn [%s]", m.phieu.thayCongDan, idToi)
	}
}

// TestDanhSachCuaToiDinhDanhVaXaTuPHIENChuKhongPhaiTuThamSo — the attacker names the victim and the
// victim's commune in the query string; the store is still asked with the SESSION's pair.
func TestDanhSachCuaToiDinhDanhVaXaTuPHIENChuKhongPhaiTuThamSo(t *testing.T) {
	m := dungMayChuCongDan(t)

	q := url.Values{"cong_dan_id": {idToi}, "citizen_id": {idToi}, "tenant": {string(xaB)}}
	w := m.goi(t, duongDanhSachCuaToi+"?"+q.Encode(), tokenNguoiKhac)
	doiMa(t, w, http.StatusOK)

	if len(m.phieu.thayCongDan) != 1 || m.phieu.thayCongDan[0] != idNguoiKhac {
		t.Errorf("định danh xuống kho = %v, muốn [%s] (CỦA PHIÊN)", m.phieu.thayCongDan, idNguoiKhac)
	}
	if len(m.phieu.thayXa) != 1 || m.phieu.thayXa[0] != xaA {
		t.Errorf("xã xuống kho = %v, muốn [%s] (CỦA PHIÊN)", m.phieu.thayXa, xaA)
	}
	if got := maTrongTrangCuaToi(docTrangCuaToi(t, w.Body.Bytes())); len(got) != 1 || got[0] != maCuaNguoiKhac {
		t.Errorf("người gọi thấy %v — muốn đúng phiếu của chính họ", got)
	}
}

// TestDanhSachCuaToiPhienXaBKhongThayPhieuXaA — the SAME citizen, commune B's session: an empty page,
// byte-identical to what a citizen with nothing filed gets, and the store was asked with commune B.
func TestDanhSachCuaToiPhienXaBKhongThayPhieuXaA(t *testing.T) {
	m := dungMayChuCongDan(t)

	w := m.goi(t, duongDanhSachCuaToi, tokenXaB)
	doiMa(t, w, http.StatusOK)
	if len(m.phieu.thayXa) != 1 || m.phieu.thayXa[0] != xaB {
		t.Errorf("xã đến kho = %v, muốn %q (xã CỦA PHIÊN)", m.phieu.thayXa, xaB)
	}

	var tho map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatal(err)
	}
	ds, ok := tho["items"].([]any)
	if !ok || len(ds) != 0 {
		t.Fatalf("items = %v, muốn [] (không phải null) — thân: %s", tho["items"], w.Body.String())
	}
	if strings.Contains(w.Body.String(), maCuaToi) {
		t.Errorf("phiếu của xã A lọt sang phiên xã B: %s", w.Body.String())
	}

	// Compared with a citizen of commune A who filed nothing under the asked status: same bytes.
	rong := m.goi(t, duongDanhSachCuaToi+"?status=da-dong", tokenCuaToi)
	doiMa(t, rong, http.StatusOK)
	if !bytes.Equal(w.Body.Bytes(), rong.Body.Bytes()) {
		t.Errorf("hai trang rỗng khác nhau:\n  xã B: %s\n  rỗng: %s", w.Body.String(), rong.Body.String())
	}
}

// --- pagination ---------------------------------------------------------------------------------

func TestDanhSachCuaToiConTroQuaHaiTrang(t *testing.T) {
	m := dungMayChuCongDan(t)

	w1 := m.goi(t, duongDanhSachCuaToi+"?limit=1", tokenCuaToi)
	doiMa(t, w1, http.StatusOK)
	t1 := docTrangCuaToi(t, w1.Body.Bytes())
	if len(t1.Items) != 1 || !t1.HasMore || t1.NextCursor == "" {
		t.Fatalf("trang 1: %d mục, has_more=%v, cursor=%q", len(t1.Items), t1.HasMore, t1.NextCursor)
	}

	w2 := m.goi(t, duongDanhSachCuaToi+"?limit=1&cursor="+url.QueryEscape(t1.NextCursor), tokenCuaToi)
	doiMa(t, w2, http.StatusOK)
	t2 := docTrangCuaToi(t, w2.Body.Bytes())
	if len(t2.Items) != 1 || t2.HasMore {
		t.Fatalf("trang 2: %d mục, has_more=%v", len(t2.Items), t2.HasMore)
	}

	got := append(maTrongTrangCuaToi(t1), maTrongTrangCuaToi(t2)...)
	if got[0] == got[1] {
		t.Fatalf("hai trang lặp lại một phiếu: %v", got)
	}
	sort.Strings(got)
	muon := []string{maAnDanh, maCuaToi}
	sort.Strings(muon)
	if got[0] != muon[0] || got[1] != muon[1] {
		t.Errorf("hai trang gộp lại = %v, muốn %v", got, muon)
	}
}

func TestDanhSachCuaToiThamSoHongLa400TruocKhiChamKho(t *testing.T) {
	for ten, q := range map[string]string{
		"trạng thái ngoài chín mã": "?status=overdue",
		"chiều tăng dần":           "?order=asc",
		"cột sắp xếp của cán bộ":   "?sort=booked_at",
		"con trỏ hỏng":             "?cursor=khong-phai-con-tro",
		"limit âm":                 "?limit=-1",
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChuCongDan(t)
			doiMa(t, m.goi(t, duongDanhSachCuaToi+q, tokenCuaToi), http.StatusBadRequest)
			if m.phieu.goi != 0 {
				t.Errorf("kho bị đọc %d lần cho một yêu cầu bị từ chối", m.phieu.goi)
			}
		})
	}
}

func TestDanhSachCuaToiLocTrangThaiDuocChuyenXuongKho(t *testing.T) {
	m := dungMayChuCongDan(t)
	w := m.goi(t, duongDanhSachCuaToi+"?status=dang-phan-loai&order=desc", tokenCuaToi)
	doiMa(t, w, http.StatusOK)
	if len(m.phieu.thayTrangThai) != 1 || m.phieu.thayTrangThai[0] != "dang-phan-loai" {
		t.Errorf("trạng thái xuống kho = %v", m.phieu.thayTrangThai)
	}
	if got := maTrongTrangCuaToi(docTrangCuaToi(t, w.Body.Bytes())); len(got) != 1 || got[0] != maCuaToi {
		t.Errorf("lọc trạng thái trả %v", got)
	}
}

// --- shape --------------------------------------------------------------------------------------

// TestDanhSachCuaToiMucChiMangTruongCuaTheTomTat reads the RAW keys of every item and requires the
// exact set, so a field added later — staff-internal or personal — turns this red whatever its name.
func TestDanhSachCuaToiMucChiMangTruongCuaTheTomTat(t *testing.T) {
	m := dungMayChuCongDan(t)
	w := m.goi(t, duongDanhSachCuaToi, tokenCuaToi)
	doiMa(t, w, http.StatusOK)

	var tho struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatal(err)
	}
	if len(tho.Items) == 0 {
		t.Fatal("không có mục nào để kiểm")
	}
	muon := map[string]bool{
		"code": true, "status": true, "field": true, "field_label": true, "content_excerpt": true,
		"clock_from": true, "acknowledge_due": true, "resolve_due": true,
	}
	for _, muc := range tho.Items {
		for k := range muc {
			if !muon[k] {
				t.Errorf("trường %q không thuộc thẻ tóm tắt của công dân: %v", k, muc)
			}
		}
		for k := range muon {
			if _, co := muc[k]; !co {
				t.Errorf("thiếu trường %q: %v", k, muc)
			}
		}
	}

	than := w.Body.String()
	// Staff-internal values (populated on purpose in the fixture) and the reporter's details.
	for _, gt := range []string{"bp-001", "nd-001", "pa-001", "pa-003", "0900000000", "Nguyễn", "09****0000"} {
		if strings.Contains(than, gt) {
			t.Errorf("giá trị %q lọt ra danh sách của công dân: %s", gt, than)
		}
	}
	// The commune's own wording for the field, as on the detail route.
	if !strings.Contains(than, "Rác thải – Vệ sinh môi trường") {
		t.Errorf("thiếu nhãn lĩnh vực của xã: %s", than)
	}
}

func TestTrichNoiDungCatTheoRune(t *testing.T) {
	ngan := "Đống rác ở đầu ngõ."
	if trichNoiDung(ngan) != ngan {
		t.Errorf("nội dung ngắn bị cắt: %q", trichNoiDung(ngan))
	}
	dai := strings.Repeat("ệ", 200)
	ra := trichNoiDung(dai)
	if n := utf8.RuneCountInString(ra); n != trichNoiDungToiDa {
		t.Errorf("độ dài trích = %d rune, muốn %d", n, trichNoiDungToiDa)
	}
	if !utf8.ValidString(ra) || !strings.HasSuffix(ra, "…") {
		t.Errorf("trích không hợp lệ hoặc thiếu dấu lược: %q", ra)
	}
	vuaDu := strings.Repeat("a", trichNoiDungToiDa)
	if trichNoiDung(vuaDu) != vuaDu {
		t.Error("nội dung đúng bằng trần bị cắt")
	}
}

// --- failure -------------------------------------------------------------------------------------

func TestDanhSachCuaToiLoiKhoLa500VaKhongLoDuLieu(t *testing.T) {
	m := dungMayChuCongDan(t)
	m.phieu.loi = errors.New("pg: connection refused cho công dân " + idToi)

	w := m.goi(t, duongDanhSachCuaToi, tokenCuaToi)
	doiMa(t, w, http.StatusInternalServerError)
	if than := w.Body.String(); strings.Contains(than, idToi) || strings.Contains(than, "connection refused") {
		t.Errorf("chi tiết nội bộ lọt ra thân lỗi: %s", than)
	}
}
