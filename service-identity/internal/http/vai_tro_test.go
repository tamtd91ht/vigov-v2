package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: GET /api/v1/roles is the second AnyAuthenticated route returning data that
// belongs to the COMMUNE rather than to the caller, and it carries one thing org-units does not —
// `is_leader`. Five things have to hold, and each fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, read for an AnyAuthenticated route;
//  2. one commune's roles never reach another commune's caller;
//  3. the list is returned WHOLE and in the store's order, or refused — never trimmed;
//  4. an empty commune serialises as [] and not null;
//  5. `id` is present, because the staff list references roles BY id and a catalogue without it
//     cannot resolve a single name.

const duongVaiTro = "/api/v1/roles"

func docVaiTro(t *testing.T, than []byte) danhSachVaiTroRa {
	t.Helper()
	var ra danhSachVaiTroRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

// --- rule 5, invariant 7 ------------------------------------------------------------------------

func TestVaiTro_401KhongToken(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, duongVaiTro, "", ""), http.StatusUnauthorized)
	if m.vaiTroMuc.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc danh mục vai trò")
	}
}

func TestVaiTro_200KhongCanQuyenCauHinh(t *testing.T) {
	// THE "403 WRONG PERMISSION" CASE AS IT READS ON AN AnyAuthenticated ROUTE, asserted rather
	// than assumed because it IS the decision this route was declared with.
	//
	// The account holds NOTHING — not admin.role, not admin.user, nothing — and must still get 200.
	// Tightening this to RequirePermission turns this test red, which is the point: role names fill
	// the picker on the staff form, the column on the directory and the header of the Phân quyền
	// matrix, so a configuration permission here would need three grants to render one screen.
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{} // no grants at all, in any commune
	})

	w := m.goi(t, "GET", hostA, duongVaiTro, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if len(docVaiTro(t, w.Body.Bytes()).Items) == 0 {
		t.Error("tài khoản không có quyền nào nhận danh sách rỗng — tuyến này phải trả đủ")
	}
}

func TestVaiTro_401XaKhac(t *testing.T) {
	// A token minted for commune A, presented on commune B's host. Refused at the token layer
	// BEFORE any query runs — which is what makes the isolation independent of every store.
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostB, duongVaiTro, "", m.tokenCho(t, xaA, sidA)), http.StatusUnauthorized)
	if m.vaiTroMuc.goi != 0 {
		t.Error("token của xã khác vẫn chạm tới kho — phép so xã phải chặn TRƯỚC mọi truy vấn")
	}
}

func TestVaiTro_200(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongVaiTro, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docVaiTro(t, w.Body.Bytes())
	if len(ra.Items) != 2 {
		t.Fatalf("trả %d vai trò, muốn 2: %+v", len(ra.Items), ra.Items)
	}
	mot := ra.Items[0]
	if mot.ID == "" {
		t.Error("thiếu `id` — danh bạ tham chiếu vai trò BẰNG id, không có nó thì không tra được tên nào")
	}
	if mot.Code != "chu-tich-ubnd" || mot.Name != "Chủ tịch UBND" || !mot.IsLeader {
		t.Errorf("mục đầu sai: %+v", mot)
	}
	if ra.Items[1].IsLeader {
		t.Error("vai trò thứ hai không phải lãnh đạo mà cờ bật — hai cột kề nhau đã hoán đổi?")
	}
}

// --- cách ly giữa hai xã ------------------------------------------------------------------------

func TestVaiTroKhongVuotSangXaKhac(t *testing.T) {
	// THE CASE THE WHOLE ROUTE IS JUDGED ON. Commune B's role is named differently on purpose: two
	// communes whose roles shared a name could not show a leak at all.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongVaiTro, "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, "Chủ tịch UBND") || strings.Contains(than, "vt-001") {
		t.Fatalf("RÒ RỈ: vai trò của xã A xuất hiện trên tên miền xã B: %s", than)
	}
	ra := docVaiTro(t, w.Body.Bytes())
	if len(ra.Items) != 1 || ra.Items[0].Name != "Kế toán xã B" {
		t.Fatalf("xã B phải thấy đúng vai trò của mình: %+v", ra.Items)
	}
}

// --- trả đủ, đúng thứ tự, hoặc từ chối ---------------------------------------------------------

func TestVaiTroGiuNguyenThuTuCuaKho(t *testing.T) {
	// `thu_tu` là thứ tự xã tự sắp. Một handler sắp lại sẽ lặng lẽ ghi đè lên nó, và người dùng
	// thấy ô chọn vai trò đổi thứ tự sau mỗi lần tải mà không ai đổi gì.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongVaiTro, "", m.tokenCho(t, xaA, sidA))
	ra := docVaiTro(t, w.Body.Bytes())

	if ra.Items[0].Code != "chu-tich-ubnd" || ra.Items[1].Code != "can-bo-mot-cua" {
		t.Errorf("thứ tự bị đổi: %+v", ra.Items)
	}
}

func TestVaiTroVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// TỪ CHỐI, KHÔNG CẮT BỚT. Danh sách này đổ vào ô chọn vai trò trên form cán bộ: một danh sách
	// ngắn đi lặng lẽ nghĩa là người được tạo tiếp theo nhận sai vai trò — tức sai bộ quyền, mà
	// màn hình không có gì để lộ ra.
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.VaiTroMuc = &vaiTroMucGia{loi: idstore.ErrQuaNhieuVaiTro}
	})

	w := m.goi(t, "GET", hostA, duongVaiTro, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)

	than := w.Body.String()
	if strings.Contains(than, "items") {
		t.Fatalf("vẫn trả danh sách khi vượt trần — cắt bớt là điều tuyến này từ chối làm: %s", than)
	}
	if strings.Contains(strings.ToLower(than), "tran") || strings.Contains(than, "vai_tro") {
		t.Errorf("lộ chi tiết nội bộ ra client: %s", than)
	}
}

func TestVaiTroLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.VaiTroMuc = &vaiTroMucGia{loi: errors.New("pq: relation \"vai_tro\" does not exist")}
	})

	w := m.goi(t, "GET", hostA, duongVaiTro, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)

	if strings.Contains(w.Body.String(), "relation") {
		t.Errorf("thông báo lỗi của CSDL lọt ra client (luật 3, cấm #3): %s", w.Body.String())
	}
}

func TestVaiTroXaChuaCauHinhTraMangRong(t *testing.T) {
	// Xã vừa onboard chưa khai vai trò nào. `items` phải là [] chứ không phải null: một máy khách
	// phải xử cả hai hình dạng là một máy khách xử sai một trong hai.
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.VaiTroMuc = &vaiTroMucGia{}
	})

	w := m.goi(t, "GET", hostA, duongVaiTro, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("mảng rỗng phải là [] chứ không null: %s", w.Body.String())
	}
}
