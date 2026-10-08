package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// GET /api/v1/staff-directory — the picker every account of the commune reads.
//
// The four cases of rule 5, invariant 7, as they read on an AnyAuthenticated route (the same
// reading as bo_phan_test.go): 401 no session · 200 for an account holding NO permission · 401
// `tenant_mismatch` for commune A's token at commune B · 200. Plus what makes AnyAuthenticated
// safe here: the response carries only four keys, `code` is the business code, and one commune
// never sees another's people.
//
// WHICH ROWS ARE PICKABLE (locked, soft-deleted, no-account excluded) is proven against the real
// predicate in store/can_bo_chon_nguoi_test.go. This fake returns what it holds; the properties
// here are the edge, the commune key and the wire shape.

const duongChonNguoi = "/api/v1/staff-directory"

// chonNguoiGia is KEYED BY COMMUNE, read from the context the way *store.Scoped does — keyed any
// other way the isolation case would pass while proving nothing.
type chonNguoiGia struct {
	theo       map[tenant.ID][]domain.CanBoChonNguoi
	loi        error
	goi        int
	boPhanCuoi string
	quyenCuoi  string

	// giuQuyen is who holds which key, PER COMMUNE — the fake's stand-in for vai_tro_quyen. Read only
	// when a permission filter arrives; which rows the real join admits is proven in
	// store/can_bo_chon_nguoi_test.go.
	giuQuyen map[tenant.ID]map[string][]string
}

func (c *chonNguoiGia) ChonNguoi(ctx context.Context, loc domain.LocChonNguoi) ([]domain.CanBoChonNguoi, error) {
	c.goi++
	c.boPhanCuoi = loc.BoPhanID
	c.quyenCuoi = loc.QuyenMa
	if c.loi != nil {
		return nil, c.loi
	}
	xa := tenant.MustFrom(ctx)
	if loc.QuyenMa == "" {
		return c.theo[xa], nil
	}
	giu := map[string]bool{}
	for _, ma := range c.giuQuyen[xa][loc.QuyenMa] {
		giu[ma] = true
	}
	var ra []domain.CanBoChonNguoi
	for _, cb := range c.theo[xa] {
		if giu[cb.Ma] {
			ra = append(ra, cb)
		}
	}
	return ra, nil
}

// chonNguoiMau gives the two communes DIFFERENT people under the same code CB-001, so a leak shows
// up as a name, not merely as a count.
func chonNguoiMau() *chonNguoiGia {
	return &chonNguoiGia{theo: map[tenant.ID][]domain.CanBoChonNguoi{
		xaA: {
			{Ma: maCanBo, HoTen: "Nguyễn Văn A", ChucVu: "Công chức Văn phòng", BoPhanID: "bp-001"},
			{Ma: "CB-005", HoTen: "Đỗ Văn E", ChucVu: "Công chức Địa chính"},
		},
		xaB: {
			{Ma: "CB-001", HoTen: "Phạm Thị D", ChucVu: "Chủ tịch UBND xã", BoPhanID: "bp-b-001"},
		},
	}, giuQuyen: map[tenant.ID]map[string][]string{
		// CB-005 holds task.extend in commune A. In commune B the code CB-001 holds it — the same
		// code as commune A's other person, so a filter that ignored the commune would offer A's
		// CB-001 as well.
		xaA: {"task.extend": {"CB-005"}},
		xaB: {"task.extend": {"CB-001"}},
	}}
}

func docChonNguoi(t *testing.T, than []byte) danhBaChonNguoiRa {
	t.Helper()
	var ra danhBaChonNguoiRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

// --- the four cases ---------------------------------------------------------------------------

func TestChonNguoi_401KhongToken(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, duongChonNguoi, "", ""), http.StatusUnauthorized)
	if m.chonNguoi.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc danh bạ chọn người")
	}
}

func TestChonNguoi_200KhongCanQuyenNao(t *testing.T) {
	// THE "403 WRONG PERMISSION" CASE ON AN AnyAuthenticated ROUTE: an account holding NOTHING in its
	// commune must still get the list — that is the user's decision of 2026-09-24. Tightening the
	// route to RequirePermission turns this red, which is the point.
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{} // no grants at all, in any commune
	})

	w := m.goi(t, "GET", hostA, duongChonNguoi, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if len(docChonNguoi(t, w.Body.Bytes()).Items) != 2 {
		t.Errorf("tài khoản không quyền nào phải nhận đủ danh bạ: %s", w.Body.String())
	}
}

func TestChonNguoi_401TokenXaKhac(t *testing.T) {
	// "Right permission, wrong commune" as it reads here: commune A's token at commune B's domain,
	// refused at the token layer before any store is touched.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongChonNguoi, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusUnauthorized)
	if got := loiTra(t, w).Code; got != "tenant_mismatch" {
		t.Errorf("code = %q, muốn tenant_mismatch", got)
	}
	if m.chonNguoi.goi != 0 {
		t.Error("token của xã khác mà vẫn đọc danh bạ chọn người của xã này")
	}
}

func TestChonNguoi_200(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongChonNguoi, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docChonNguoi(t, w.Body.Bytes())
	if len(ra.Items) != 2 {
		t.Fatalf("nhận %d người, muốn 2", len(ra.Items))
	}
	// `code` IS THE BUSINESS CODE — the value assignment routes compare with Principal.Ma — and the
	// store's order is kept.
	if ra.Items[0].Code != maCanBo || ra.Items[0].FullName != "Nguyễn Văn A" ||
		ra.Items[0].Position != "Công chức Văn phòng" || ra.Items[0].DepartmentID != "bp-001" {
		t.Errorf("người đầu sai: %+v", ra.Items[0])
	}
	if strings.Contains(w.Body.String(), idNoiBo) {
		t.Errorf("mã nội bộ (ULID) lọt ra danh bạ chọn người: %s", w.Body.String())
	}
	if ra.Items[1].DepartmentID != "" {
		t.Errorf("người không thuộc bộ phận phải có department_id rỗng: %+v", ra.Items[1])
	}
}

// --- what AnyAuthenticated is allowed to carry ------------------------------------------------

func TestChonNguoiChiCoBonKhoaKhongSoDienThoaiKhongEmail(t *testing.T) {
	// The KEYS, asserted exactly. Adding `phone`, `mobile`, a raw `email`, `id` or an account flag to
	// this response turns this red — each of them would go to every account of the commune.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongChonNguoi, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var tho struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %s", w.Body.String())
	}
	var ngoai map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &ngoai); err != nil {
		t.Fatalf("thân không phải đối tượng JSON: %s", w.Body.String())
	}
	if len(ngoai) != 1 {
		t.Errorf("thân phải chỉ có `items`, có: %s", w.Body.String())
	}
	for _, muc := range tho.Items {
		var khoa []string
		for k := range muc {
			khoa = append(khoa, k)
		}
		sort.Strings(khoa)
		// `email_masked` joined on 08/10/2026 (ADR 0082 §3) — MASKED, never the raw address.
		if got := strings.Join(khoa, ","); got != "code,department_id,email_masked,full_name,position" {
			t.Errorf("khoá của một người = %s, muốn đúng code,department_id,email_masked,full_name,position", got)
		}
	}
	for _, cam := range []string{`"phone"`, `"mobile"`, `"email"`, `"id"`, `"has_account"`, `"active"`} {
		if strings.Contains(w.Body.String(), cam) {
			t.Errorf("danh bạ chọn người chứa %s: %s", cam, w.Body.String())
		}
	}
}

// --- one commune --------------------------------------------------------------------------------

func TestChonNguoiKhongVuotSangXaKhac(t *testing.T) {
	// The same account signed in properly at commune B must see commune B's people only.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongChonNguoi, "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusOK)

	if strings.Contains(w.Body.String(), "Nguyễn Văn A") {
		t.Fatalf("RÒ RỈ: ở xã B nhận người của xã A: %s", w.Body.String())
	}
	ra := docChonNguoi(t, w.Body.Bytes())
	if len(ra.Items) != 1 || ra.Items[0].FullName != "Phạm Thị D" {
		t.Fatalf("xã B phải nhận đúng người của mình: %+v", ra.Items)
	}
}

// --- the filter -------------------------------------------------------------------------------

func TestChonNguoiLocUnitDuocChuyenNguyenVan(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	doiMa(t, m.goi(t, "GET", hostA, duongChonNguoi+"?unit=bp-001", "", tok), http.StatusOK)
	if m.chonNguoi.boPhanCuoi != "bp-001" {
		t.Errorf("kho nhận bộ phận %q, muốn bp-001", m.chonNguoi.boPhanCuoi)
	}
	doiMa(t, m.goi(t, "GET", hostA, duongChonNguoi, "", tok), http.StatusOK)
	if m.chonNguoi.boPhanCuoi != "" {
		t.Errorf("không lọc mà kho nhận bộ phận %q", m.chonNguoi.boPhanCuoi)
	}
}

func TestChonNguoiUnitQuaDaiTra400KhongGoiKho(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongChonNguoi+"?unit="+strings.Repeat("x", 500), "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusBadRequest)
	if m.chonNguoi.goi != 0 {
		t.Error("tham số lọc hỏng mà vẫn chạy câu đọc")
	}
}

// --- refusal and failure ----------------------------------------------------------------------

func TestChonNguoiVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	m := dungMayChu(t)
	m.chonNguoi.loi = idstore.ErrQuaNhieuCanBoChonNguoi

	w := m.goi(t, "GET", hostA, duongChonNguoi, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), `"items"`) {
		t.Errorf("phản hồi từ chối vẫn kèm danh sách: %s", w.Body.String())
	}
}

func TestChonNguoiLoiKhoKhongLoNoiDung(t *testing.T) {
	m := dungMayChu(t)
	m.chonNguoi.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongChonNguoi, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)
	if e := loiTra(t, w); strings.Contains(e.Message, "cơ sở dữ liệu") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

func TestChonNguoiXaChuaCoAiTraMangRong(t *testing.T) {
	m := dungMayChu(t)
	m.chonNguoi.theo[xaA] = nil

	w := m.goi(t, "GET", hostA, duongChonNguoi, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("items phải là [], nhận: %s", w.Body.String())
	}
}

// --- the `permission` filter (owner decision 27/09/2026) ---------------------------------------

func TestChonNguoiKhongCoPermissionThiNhuCu(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongChonNguoi, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if m.chonNguoi.quyenCuoi != "" {
		t.Fatalf("không có ?permission mà kho nhận khoá %q", m.chonNguoi.quyenCuoi)
	}
	if len(docChonNguoi(t, w.Body.Bytes()).Items) != 2 {
		t.Fatalf("không lọc quyền phải nhận đủ danh bạ: %s", w.Body.String())
	}
}

func TestChonNguoiPermissionChiNguoiGiuQuyen(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		// AnyAuthenticated stays AnyAuthenticated: the CALLER needs no permission to filter by one.
		d.Checker = checkerGia{}
	})

	w := m.goi(t, "GET", hostA, duongChonNguoi+"?permission=task.extend", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if m.chonNguoi.quyenCuoi != "task.extend" {
		t.Fatalf("kho nhận khoá %q, muốn task.extend nguyên văn", m.chonNguoi.quyenCuoi)
	}
	ra := docChonNguoi(t, w.Body.Bytes())
	if len(ra.Items) != 1 || ra.Items[0].Code != "CB-005" {
		t.Fatalf("lọc task.extend ở xã A = %+v, muốn đúng CB-005", ra.Items)
	}
}

func TestChonNguoiPermissionKhongVuotXa(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongChonNguoi+"?permission=task.extend", "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), "Đỗ Văn E") || strings.Contains(w.Body.String(), "Nguyễn Văn A") {
		t.Fatalf("RÒ RỈ: xã B nhận người giữ quyền của xã A: %s", w.Body.String())
	}
	ra := docChonNguoi(t, w.Body.Bytes())
	if len(ra.Items) != 1 || ra.Items[0].FullName != "Phạm Thị D" {
		t.Fatalf("xã B phải nhận đúng người giữ quyền của mình: %+v", ra.Items)
	}
}

func TestChonNguoiPermissionNgoaiDanhSachTra400KhongGoiKho(t *testing.T) {
	// THE ALLOWLIST (quyenLanhDaoGiaoViec). A well-formed key off it is 400 — NOT 200 with [] — so an
	// AnyAuthenticated account cannot list who holds admin.* or any other right. Widening the switch in
	// kiemQuyenLoc turns this red, which is the review the constant's comment asks for.
	m := dungMayChu(t)
	// The fake says a holder EXISTS for each key, so a 200 here would leak a name, not just a count.
	m.chonNguoi.giuQuyen[xaA]["admin.user"] = []string{maCanBo}
	m.chonNguoi.giuQuyen[xaA]["task.approve"] = []string{maCanBo}
	tok := m.tokenCho(t, xaA, sidA)

	for _, k := range []string{"admin.user", "task.approve", "admin.role", "task.khong_ton_tai"} {
		w := m.goi(t, "GET", hostA, duongChonNguoi+"?permission="+k, "", tok)
		doiMa(t, w, http.StatusBadRequest)
		e := loiTra(t, w)
		if e.Code != "invalid_request" {
			t.Errorf("%s: code = %q, muốn invalid_request", k, e.Code)
		}
		if !strings.Contains(e.Message, "chỉ hỗ trợ") {
			t.Errorf("%s: thông điệp phải nói bộ lọc chỉ hỗ trợ khoá của ô chọn người, nhận %q", k, e.Message)
		}
		if strings.Contains(w.Body.String(), `"items"`) {
			t.Errorf("%s: bị từ chối mà vẫn kèm danh sách: %s", k, w.Body.String())
		}
	}
	if m.chonNguoi.goi != 0 {
		t.Errorf("khoá ngoài danh sách cho phép mà vẫn chạy câu đọc %d lần", m.chonNguoi.goi)
	}
}

func TestChonNguoiPermissionSaiDangTra400KhongGoiKho(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	// `?permission=` (present, empty) is refused rather than read as "no filter": a client that meant
	// to narrow must not receive everybody.
	for _, q := range []string{"", "task", "task.extend.x", "TASK.EXTEND", "*", "task.%2A", "task%2Cextend",
		strings.Repeat("a", 100) + ".b"} {
		w := m.goi(t, "GET", hostA, duongChonNguoi+"?permission="+q, "", tok)
		doiMa(t, w, http.StatusBadRequest)
		if got := loiTra(t, w).Code; got != "invalid_request" {
			t.Errorf("?permission=%s: code = %q, muốn invalid_request", q, got)
		}
	}
	// Two keys is not "either" and not "the first": one key is the whole contract.
	doiMa(t, m.goi(t, "GET", hostA, duongChonNguoi+"?permission=task.extend&permission=admin.user", "", tok),
		http.StatusBadRequest)
	if m.chonNguoi.goi != 0 {
		t.Errorf("khoá quyền sai dạng mà vẫn chạy câu đọc %d lần", m.chonNguoi.goi)
	}
}

func TestChonNguoiPermissionVaUnitCungChuyen(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, duongChonNguoi+"?unit=bp-001&permission=task.extend", "",
		m.tokenCho(t, xaA, sidA)), http.StatusOK)
	if m.chonNguoi.boPhanCuoi != "bp-001" || m.chonNguoi.quyenCuoi != "task.extend" {
		t.Fatalf("kho nhận (%q, %q), muốn (bp-001, task.extend)", m.chonNguoi.boPhanCuoi, m.chonNguoi.quyenCuoi)
	}
}
