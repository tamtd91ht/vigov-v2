package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for GET /api/v1/task-extensions (§5.8, the approval queue).
//
//	PROVED HERE   rule 5 invariant 7 — 401 no session · 403 without `task.read` (holding `task.extend`
//	              and `task.update` does not help) · 401 right key WRONG COMMUNE (this package's
//	              convention: authz compares the commune before the key) · 200 — with no store touched
//	              in the first three · the commune reaching the store is the request's · the wire shape,
//	              exactly nine fields · the cursor passed through both ways · `approver=me` hands the
//	              store the SESSION's staff code, and a query value naming anybody else is 400 with no
//	              store touched · `approver=me` on a principal with no staff code is refused (500) before
//	              any read · 400 on a bad page request · 500 without the reason text in a log line.
//
//	NOT PROVED    the SQL — internal/store/de_nghi_lui_han_cho_duyet_test.go (fake driver) and
//	              de_nghi_lui_han_cho_duyet_pg_test.go (PostgreSQL; SKIPS without VIGOV_TEST_DSN).

// deNghiChoDuyetGia is the queue read, KEYED BY COMMUNE from the context as *store.Scoped does, and
// applying the leader filter the way the SQL does — so the `approver=me` case shows a narrower page.
type deNghiChoDuyetGia struct {
	theo map[tenant.ID][]domain.DeNghiLuiHanChoDuyet
	tiep string
	loi  error
	goi  int
	xa   tenant.ID
	loc  petstore.LocDeNghiChoDuyet
	yc   page.Request
}

func (g *deNghiChoDuyetGia) ChoDuyet(ctx context.Context, loc petstore.LocDeNghiChoDuyet, yc page.Request) (
	page.Result[domain.DeNghiLuiHanChoDuyet], error) {

	g.goi++
	g.xa, g.loc, g.yc = tenant.MustFrom(ctx), loc, yc
	ra := page.NewResult[domain.DeNghiLuiHanChoDuyet]()
	if g.loi != nil {
		return ra, g.loi
	}
	for _, d := range g.theo[g.xa] {
		if loc.LanhDaoGiaoViecMa != "" && d.LanhDaoGiaoViecMa != loc.LanhDaoGiaoViecMa {
			continue
		}
		ra.Items = append(ra.Items, d)
	}
	ra.NextCursor, ra.HasMore = g.tiep, g.tiep != ""
	return ra, nil
}

var (
	mocDNCho1   = time.Date(2026, 9, 24, 2, 0, 0, 0, time.UTC)
	mocDNCho2   = time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC)
	mocHanCu    = time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	mocHanMoiDN = time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
)

// deNghiChoDuyetMau gives commune A two pending requests OLDEST FIRST as the store returns them — one
// on a task whose leader is the caller (maCanBo), one on a task that names NOBODY — and commune B one
// request whose leader code is ALSO maCanBo, so a handler that forgot the commune would show it.
func deNghiChoDuyetMau() *deNghiChoDuyetGia {
	return &deNghiChoDuyetGia{theo: map[tenant.ID][]domain.DeNghiLuiHanChoDuyet{
		xaA: {
			{
				DeNghi: domain.DeNghiLuiHan{ID: "dn-001", NhiemVuID: "nv-001", NguoiDeNghiMa: "CB-00311",
					HanMoi: mocHanMoiDN, LyDo: "Chờ số liệu của thôn.", TrangThai: domain.ChoDuyetLuiHan,
					ThoiDiem: mocDNCho1},
				NhiemVuMa: "NV19", NhiemVuTieuDe: "Rà soát tuyến đường liên thôn",
				HanXuLyHienTai: mocHanCu, LanhDaoGiaoViecMa: maCanBo,
			},
			{
				DeNghi: domain.DeNghiLuiHan{ID: "dn-002", NhiemVuID: "nv-002", NguoiDeNghiMa: "CB-00412",
					HanMoi: mocHanMoiDN, LyDo: "Thiếu nhân lực.", TrangThai: domain.ChoDuyetLuiHan,
					ThoiDiem: mocDNCho2},
				NhiemVuMa: "NV20", NhiemVuTieuDe: "Tổng hợp báo cáo quý",
			},
		},
		xaB: {
			{
				DeNghi: domain.DeNghiLuiHan{ID: "dn-b01", NhiemVuID: "nv-001", NguoiDeNghiMa: "CB-B0001",
					HanMoi: mocHanMoiDN, LyDo: "Của xã B.", TrangThai: domain.ChoDuyetLuiHan,
					ThoiDiem: mocDNCho1},
				NhiemVuMa: "NV01", NhiemVuTieuDe: "Việc của xã B", LanhDaoGiaoViecMa: maCanBo,
			},
		},
	}}
}

const duongHangChoLuiHan = "/api/v1/task-extensions"

// --- rule 5, invariant 7 ------------------------------------------------------------------------

func TestHangChoLuiHanKhongCoPhienThi401(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongHangChoLuiHan, nil), http.StatusUnauthorized)
	if m.deNghiCho.goi != 0 {
		t.Error("đã chạm kho dù chưa có phiên")
	}
}

// 403: the account holds the TWO EXTENSION KEYS and not `task.read` — so a route guarded by
// `task.extend` (the decision's key) or `task.update` (the request's key) would pass here and fail.
func TestHangChoLuiHanThieuTaskReadThi403(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("task.extend"): true, authz.Perm("task.update"): true}},
		}}
	})
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongHangChoLuiHan, canBoCuaXa(xaA)), http.StatusForbidden)
	if m.deNghiCho.goi != 0 {
		t.Error("đã chạm kho dù thiếu `task.read`")
	}
}

// Right key, WRONG COMMUNE: 401 (authz compares the commune before the key), nothing read.
func TestHangChoLuiHanDungQuyenSaiXaThi401(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("task.read"): true}},
			xaB: {idCanBo: {authz.Perm("task.read"): true}},
		}}
	})
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongHangChoLuiHan, canBoCuaXa(xaB)), http.StatusUnauthorized)
	if m.deNghiCho.goi != 0 {
		t.Error("đã chạm kho của xã A bằng phiên của xã B — rò rỉ giữa hai cơ quan nhà nước")
	}
}

// 200: the whole commune, the request's commune at the store, the shape, the order, the cursor.
func TestHangChoLuiHanDungQuyenDungXaThi200(t *testing.T) {
	m := dungMayChu(t)
	m.deNghiCho.tiep = "con-tro-trang-sau"

	w := m.goi(t, http.MethodGet, hostA, duongHangChoLuiHan+"?limit=2", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	if m.deNghiCho.xa != xaA {
		t.Errorf("kho đọc xã %q, muốn xã của Host (%q)", m.deNghiCho.xa, xaA)
	}
	if m.deNghiCho.loc.LanhDaoGiaoViecMa != "" {
		t.Errorf("không có `approver` mà vẫn lọc theo %q — mặc định phải là cả xã", m.deNghiCho.loc.LanhDaoGiaoViecMa)
	}
	if m.deNghiCho.yc.Limit() != 2 {
		t.Errorf("limit xuống kho = %d, muốn 2", m.deNghiCho.yc.Limit())
	}
	if m.deNghiCho.yc.Dir() != page.Asc || m.deNghiCho.yc.Column().Param != "requested_at" {
		t.Errorf("thứ tự mặc định = %s %s, muốn requested_at asc (cũ nhất trước)",
			m.deNghiCho.yc.Column().Param, m.deNghiCho.yc.Dir())
	}

	var ra page.Result[deNghiChoDuyetRa]
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %s", w.Body.String())
	}
	if !ra.HasMore || ra.NextCursor != "con-tro-trang-sau" {
		t.Errorf("has_more=%v next_cursor=%q — con trỏ phải đi nguyên từ kho ra", ra.HasMore, ra.NextCursor)
	}
	if len(ra.Items) != 2 || ra.Items[0].ID != "dn-001" || ra.Items[1].ID != "dn-002" {
		t.Fatalf("items = %+v, muốn [dn-001 dn-002] đúng thứ tự kho trả", ra.Items)
	}
	a, b := ra.Items[0], ra.Items[1]
	if a.TaskCode != "NV19" || a.TaskTitle != "Rà soát tuyến đường liên thôn" || a.TaskAssigner != maCanBo ||
		a.RequestedBy != "CB-00311" || a.Reason != "Chờ số liệu của thôn." ||
		!a.NewDueAt.Equal(mocHanMoiDN) || !a.RequestedAt.Equal(mocDNCho1) ||
		a.TaskDueAt == nil || !a.TaskDueAt.Equal(mocHanCu) {
		t.Errorf("dòng 1 = %+v", a)
	}
	// A task naming nobody: assigner "" (ADR 0038 — nobody can decide it), and no deadline → null.
	if b.TaskAssigner != "" || b.TaskDueAt != nil {
		t.Errorf("dòng 2 = %+v, muốn task_assigner rỗng và task_due_at null", b)
	}

	// ON THE RAW BODY: exactly the nine fields, no internal task id, no status.
	var tho struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &tho)
	for _, dong := range tho.Items {
		for _, k := range []string{"id", "task_code", "task_title", "task_due_at", "task_assigner",
			"new_due_at", "reason", "requested_by", "requested_at"} {
			if _, co := dong[k]; !co {
				t.Errorf("thiếu trường %q", k)
			}
		}
		for _, cam := range []string{"nhiem_vu_id", "task_id", "status", "decided_by", "decided_at"} {
			if _, co := dong[cam]; co {
				t.Errorf("có trường %q — id nội bộ của nhiệm vụ, hoặc trường của một đề nghị đã quyết", cam)
			}
		}
		if len(dong) != 9 {
			t.Errorf("dòng có %d trường, muốn đúng 9: %v", len(dong), dong)
		}
	}
	if strings.Contains(w.Body.String(), "Của xã B.") {
		t.Error("thân chứa đề nghị của xã B")
	}
}

// --- approver=me --------------------------------------------------------------------------------

// `approver=me` resolves "me" from the SESSION: the store receives the principal's staff code, and
// only the request whose task names that code comes back.
func TestHangChoLuiHanApproverMeLayMaTuPhien(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, duongHangChoLuiHan+"?approver=me", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	if got := m.deNghiCho.loc.LanhDaoGiaoViecMa; got != maCanBo {
		t.Errorf("lọc theo lãnh đạo = %q, muốn mã của CHÍNH người đăng nhập (%q)", got, maCanBo)
	}
	var ra page.Result[deNghiChoDuyetRa]
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %s", w.Body.String())
	}
	if len(ra.Items) != 1 || ra.Items[0].ID != "dn-001" {
		t.Errorf("items = %+v, muốn đúng [dn-001]", ra.Items)
	}
}

// Naming ANYBODY by value is refused — a staff code, the caller's own code, a different case of
// `me` — and no store is touched. There is no spelling that makes the client the source of identity.
func TestHangChoLuiHanApproverKhacMeThi400KhongChamKho(t *testing.T) {
	for _, q := range []string{"?approver=CB-00999", "?approver=" + maCanBo, "?approver=ME",
		"?approver=CB-00999&approver=me", "?approver=all"} {
		t.Run(q, func(t *testing.T) {
			m := dungMayChu(t)
			w := m.goi(t, http.MethodGet, hostA, duongHangChoLuiHan+q, canBoCuaXa(xaA))
			doiMa(t, w, http.StatusBadRequest)
			if m.deNghiCho.goi != 0 {
				t.Error("đã chạm kho dù `approver` không hợp lệ")
			}
			if strings.Contains(w.Body.String(), "CB-00999") {
				t.Error("thân lỗi dội lại giá trị client gửi")
			}
		})
	}
}

// A principal with no staff code and `approver=me`: refused BEFORE any read, never widened to the
// whole commune.
func TestHangChoLuiHanApproverMeMaRongThiTuChoi(t *testing.T) {
	m := dungMayChu(t)
	p := &authz.Principal{ID: idCanBo, Kind: "staff", TenantID: xaA}
	w := m.goi(t, http.MethodGet, hostA, duongHangChoLuiHan+"?approver=me", p)
	doiMa(t, w, http.StatusInternalServerError)
	if m.deNghiCho.goi != 0 {
		t.Errorf("đã đọc kho %d lần — lẽ ra từ chối TRƯỚC khi truy vấn", m.deNghiCho.goi)
	}
}

// --- the rest of the read -------------------------------------------------------------------------

func TestHangChoLuiHanRongThiItemsLaMangRong(t *testing.T) {
	m := dungMayChu(t)
	m.deNghiCho.theo[xaA] = nil
	w := m.goi(t, http.MethodGet, hostA, duongHangChoLuiHan, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) || !strings.Contains(w.Body.String(), `"has_more":false`) {
		t.Errorf("hàng chờ rỗng phải là items: [] — thân: %s", w.Body.String())
	}
}

func TestHangChoLuiHanYeuCauTrangSaiThi400KhongChamKho(t *testing.T) {
	for _, q := range []string{"?sort=ly_do", "?cursor=khong-phai-con-tro", "?order=ngang"} {
		t.Run(q, func(t *testing.T) {
			m := dungMayChu(t)
			doiMa(t, m.goi(t, http.MethodGet, hostA, duongHangChoLuiHan+q, canBoCuaXa(xaA)),
				http.StatusBadRequest)
			if m.deNghiCho.goi != 0 {
				t.Error("đã chạm kho dù yêu cầu phân trang sai")
			}
		})
	}
}

func TestHangChoLuiHanKhoHongThi500KhongLoLyDo(t *testing.T) {
	m := dungMayChu(t)
	var nhatKy bytes.Buffer
	m.dungLai(t, func(d *Deps) { d.Log = slog.New(slog.NewTextHandler(&nhatKy, nil)) })
	m.deNghiCho.loi = errors.New("kho hỏng")
	w := m.goi(t, http.MethodGet, hostA, duongHangChoLuiHan, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if nhatKy.Len() == 0 {
		t.Fatal("không có dòng log nào — phép kiểm này sẽ xanh vì lý do sai")
	}
	if strings.Contains(w.Body.String(), "kho hỏng") || strings.Contains(nhatKy.String(), "Chờ số liệu") {
		t.Error("thân lỗi lộ lỗi nội bộ, hoặc log lộ lý do lùi hạn")
	}
}
