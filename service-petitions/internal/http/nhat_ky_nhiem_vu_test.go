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

// Tests for GET /api/v1/tasks/{ma}/log-entries (§5.9, read half).
//
//	PROVED HERE   rule 5 invariant 7 — 401 no session · 403 without `task.read` · 401 right key WRONG
//	              COMMUNE (this package's convention: authz compares the commune before the key) · 200 —
//	              with no store touched in the first three · 404, ONE body, for an unknown number,
//	              another commune's number and a soft-deleted task, with the log never read · the log is
//	              read by the task's INTERNAL id, in the request's commune · the wire shape, the order the
//	              store returns, the cursor passed through both ways · 400 on a bad page request before
//	              any store · 500 without the entry text in a log line.
//
//	NOT PROVED    the SQL — internal/store/nhat_ky_nhiem_vu_test.go (fake driver) and
//	              nhat_ky_nhiem_vu_pg_test.go (PostgreSQL; SKIPS without VIGOV_TEST_DSN).

// nhatKyNhiemVuGia is the progress-log read, KEYED BY COMMUNE AND BY TASK INTERNAL id.
type nhatKyNhiemVuGia struct {
	theo      map[tenant.ID]map[string][]domain.NhatKyNhiemVu
	tiep      string // the NextCursor the fake answers with; HasMore follows it
	loi       error
	goi       int
	nhiemVuID string
	xa        tenant.ID
	yc        page.Request
}

func (n *nhiemVuGia) NhatKyCuaNhiemVu(ctx context.Context, nhiemVuID string, yc page.Request) (
	page.Result[domain.NhatKyNhiemVu], error) {

	n.nhatKy.goi++
	n.nhatKy.nhiemVuID, n.nhatKy.yc = nhiemVuID, yc
	n.nhatKy.xa = tenant.MustFrom(ctx)
	ra := page.NewResult[domain.NhatKyNhiemVu]()
	if n.nhatKy.loi != nil {
		return ra, n.nhatKy.loi
	}
	ra.Items = append(ra.Items, n.nhatKy.theo[n.nhatKy.xa][nhiemVuID]...)
	ra.NextCursor, ra.HasMore = n.nhatKy.tiep, n.nhatKy.tiep != ""
	return ra, nil
}

var (
	mocNKNV1 = time.Date(2026, 9, 26, 2, 10, 0, 0, time.UTC)
	mocNKNV2 = time.Date(2026, 9, 26, 3, 20, 0, 0, time.UTC)
)

// ganNhatKyMau gives commune A's `nv-001` (NV19) two rows NEWEST FIRST as the store returns them.
// Commune B has a row under the SAME internal id — so a handler that forgot the commune would show it.
func ganNhatKyMau(n *nhiemVuGia) {
	n.nhatKy.theo = map[tenant.ID]map[string][]domain.NhatKyNhiemVu{
		xaA: {
			"nv-001": {
				{ID: "nknv-2", NhiemVuID: "nv-001", ThoiDiem: mocNKNV2, NguoiMa: maCanBo,
					TrangThaiTaiThoiDiem: domain.DangThucHien, BoPhanID: "bp-vpdu", NguoiPhuTrachMa: "CB-00311",
					NoiDung: "Giao lại cho văn phòng Đảng ủy."},
				{ID: "nknv-1", NhiemVuID: "nv-001", ThoiDiem: mocNKNV1, NguoiMa: "CB-00007",
					TrangThaiTaiThoiDiem: domain.MoiGiao, NoiDung: "Tạo nhiệm vụ."},
			},
		},
		xaB: {
			"nv-001": {{ID: "nknv-b", NhiemVuID: "nv-001", ThoiDiem: mocNKNV1, NguoiMa: "CB-B0001",
				TrangThaiTaiThoiDiem: domain.MoiGiao, NoiDung: "Của xã B."}},
		},
	}
}

func dungMayChuNhatKyNV(t *testing.T) *mayChu {
	t.Helper()
	m := dungMayChu(t)
	ganNhatKyMau(m.nhiemVu)
	return m
}

func duongNhatKyNV(ma string) string { return duongNhiemVu(ma) + "/log-entries" }

// chamKhoNV is every store read the route could make.
func (m *mayChu) chamKhoNV() int { return m.nhiemVu.goi + m.nhiemVu.nhatKy.goi }

// --- rule 5, invariant 7 ------------------------------------------------------------------------

func TestNhatKyNVKhongCoPhienThi401(t *testing.T) {
	m := dungMayChuNhatKyNV(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongNhatKyNV(maNhiemVuA), nil), http.StatusUnauthorized)
	if m.chamKhoNV() != 0 {
		t.Error("đã chạm kho dù chưa có phiên")
	}
}

// 403: commune A's account holds keys, but not `task.read` — the neighbouring petition read key
// included, so a route guarded by the wrong key fails here.
func TestNhatKyNVThieuTaskReadThi403(t *testing.T) {
	m := dungMayChuNhatKyNV(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("feedback.read"): true, authz.Perm("task.update"): true}},
		}}
	})
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongNhatKyNV(maNhiemVuA), canBoCuaXa(xaA)),
		http.StatusForbidden)
	if m.chamKhoNV() != 0 {
		t.Error("đã chạm kho dù thiếu `task.read`")
	}
}

// Right key, WRONG COMMUNE: 401 (authz compares the commune before the key), nothing read.
func TestNhatKyNVDungQuyenSaiXaThi401(t *testing.T) {
	m := dungMayChuNhatKyNV(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("task.read"): true}},
			xaB: {idCanBo: {authz.Perm("task.read"): true}},
		}}
	})
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongNhatKyNV(maNhiemVuA), canBoCuaXa(xaB)),
		http.StatusUnauthorized)
	if m.chamKhoNV() != 0 {
		t.Error("đã chạm kho của xã A bằng phiên của xã B — rò rỉ giữa hai cơ quan nhà nước")
	}
}

// 200, with the shape, the order, the key the log is read by, and the cursor both ways.
func TestNhatKyNVDungQuyenDungXaThi200(t *testing.T) {
	m := dungMayChuNhatKyNV(t)
	m.nhiemVu.nhatKy.tiep = "con-tro-trang-sau"

	w := m.goi(t, http.MethodGet, hostA, duongNhatKyNV(maNhiemVuA)+"?limit=2", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	if m.nhiemVu.nhatKy.nhiemVuID != "nv-001" || m.nhiemVu.nhatKy.xa != xaA {
		t.Errorf("đọc nhật ký theo (%q, %q), muốn id NỘI BỘ nv-001 trong xã A",
			m.nhiemVu.nhatKy.xa, m.nhiemVu.nhatKy.nhiemVuID)
	}
	if m.nhiemVu.nhatKy.yc.Limit() != 2 {
		t.Errorf("limit xuống kho = %d, muốn 2", m.nhiemVu.nhatKy.yc.Limit())
	}

	var ra page.Result[nhatKyNhiemVuRa]
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %s", w.Body.String())
	}
	if !ra.HasMore || ra.NextCursor != "con-tro-trang-sau" {
		t.Errorf("has_more=%v next_cursor=%q — con trỏ phải đi nguyên từ kho ra", ra.HasMore, ra.NextCursor)
	}
	if len(ra.Items) != 2 || ra.Items[0].ID != "nknv-2" || ra.Items[1].ID != "nknv-1" {
		t.Fatalf("items = %+v, muốn [nknv-2 nknv-1] đúng thứ tự kho trả", ra.Items)
	}
	pc, tao := ra.Items[0], ra.Items[1]
	if pc.Status != "dang-thuc-hien" || pc.Unit != "bp-vpdu" || pc.Assignee != "CB-00311" ||
		pc.ActorCode != maCanBo || !pc.At.Equal(mocNKNV2) || pc.Note != "Giao lại cho văn phòng Đảng ủy." {
		t.Errorf("dòng giao lại = %+v", pc)
	}
	if tao.Status != "moi-giao" || tao.Unit != "" || tao.Assignee != "" || tao.ActorCode != "CB-00007" {
		t.Errorf("dòng tạo = %+v", tao)
	}

	// ON THE RAW BODY: exactly the fields the table holds, and none it does not.
	var tho struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &tho)
	for _, dong := range tho.Items {
		for _, k := range []string{"id", "at", "actor_code", "status", "unit", "assignee", "note"} {
			if _, co := dong[k]; !co {
				t.Errorf("thiếu trường %q", k)
			}
		}
		for _, cam := range []string{"action", "attachments", "actor_name", "task_id", "nhiem_vu_id"} {
			if _, co := dong[cam]; co {
				t.Errorf("có trường %q — bảng không giữ nó, hoặc là id nội bộ của nhiệm vụ", cam)
			}
		}
		if len(dong) != 7 {
			t.Errorf("dòng có %d trường, muốn đúng 7: %v", len(dong), dong)
		}
	}
	if strings.Contains(w.Body.String(), "Của xã B.") {
		t.Error("thân chứa dòng nhật ký của xã B")
	}
}

// --- 404: one body, three causes, and the log is never read ---------------------------------------

// nhiemVuDaXoaGia stands for the store's TheoMa on a SOFT-DELETED task: `deleted_at IS NULL` in its
// SQL (proved at internal/store/nhiem_vu_test.go) makes it answer ErrNhiemVuKhongTonTai. The log rows
// of that task are still in the fixture — so a handler that read the log by number would show them.
type nhiemVuDaXoaGia struct {
	*nhiemVuGia
	daXoa map[string]bool
}

func (n nhiemVuDaXoaGia) TheoMa(ctx context.Context, ma string) (domain.NhiemVu, error) {
	if n.daXoa[ma] {
		n.goi++
		return domain.NhiemVu{}, petstore.ErrNhiemVuKhongTonTai
	}
	return n.nhiemVuGia.TheoMa(ctx, ma)
}

func TestNhatKyNV404MotCauChoBaNguyenNhan(t *testing.T) {
	var thanMau string
	for _, ca := range []struct {
		ten   string
		ma    string
		daXoa bool
	}{
		{"mã không tồn tại", "NV999", false},
		{"mã của xã khác (đúng quyền, đúng xã phiên)", maNhiemVuB, false},
		{"nhiệm vụ đã xoá mềm", maNhiemVuA, true},
	} {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChuNhatKyNV(t)
			if ca.daXoa {
				m.dungLai(t, func(d *Deps) {
					d.NhiemVu = nhiemVuDaXoaGia{nhiemVuGia: m.nhiemVu, daXoa: map[string]bool{ca.ma: true}}
				})
			}
			w := m.goi(t, http.MethodGet, hostA, duongNhatKyNV(ca.ma), canBoCuaXa(xaA))
			doiMa(t, w, http.StatusNotFound)
			if m.nhiemVu.nhatKy.goi != 0 {
				t.Error("đã đọc nhật ký dù nhiệm vụ không được thấy")
			}
			if e := loiTra(t, w); e.Code != "not_found" {
				t.Errorf("mã lỗi = %q", e.Code)
			}
			if thanMau == "" {
				thanMau = w.Body.String()
			} else if w.Body.String() != thanMau {
				t.Errorf("thân 404 khác nhau giữa các nguyên nhân:\n%s\n%s", thanMau, w.Body.String())
			}
		})
	}

	// AND IT IS THE SAME BODY GET /api/v1/tasks/{ma} ANSWERS — never distinguishable by route.
	m := dungMayChuNhatKyNV(t)
	w := m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuB), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusNotFound)
	if w.Body.String() != thanMau {
		t.Errorf("404 của nhật ký khác 404 của chi tiết nhiệm vụ:\n%s\n%s", thanMau, w.Body.String())
	}
}

// --- the rest of the read -------------------------------------------------------------------------

func TestNhatKyNVRongThiItemsLaMangRong(t *testing.T) {
	m := dungMayChuNhatKyNV(t)
	m.nhiemVu.nhatKy.theo[xaA]["nv-001"] = nil
	w := m.goi(t, http.MethodGet, hostA, duongNhatKyNV(maNhiemVuA), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) || !strings.Contains(w.Body.String(), `"has_more":false`) {
		t.Errorf("nhật ký rỗng phải là items: [] — thân: %s", w.Body.String())
	}
}

func TestNhatKyNVYeuCauTrangSaiThi400KhongChamKho(t *testing.T) {
	for _, q := range []string{"?sort=noi_dung", "?cursor=khong-phai-con-tro", "?order=ngang"} {
		t.Run(q, func(t *testing.T) {
			m := dungMayChuNhatKyNV(t)
			doiMa(t, m.goi(t, http.MethodGet, hostA, duongNhatKyNV(maNhiemVuA)+q, canBoCuaXa(xaA)),
				http.StatusBadRequest)
			if m.chamKhoNV() != 0 {
				t.Error("đã chạm kho dù yêu cầu phân trang sai")
			}
		})
	}
}

func TestNhatKyNVKhoHongThi500KhongLoNoiDung(t *testing.T) {
	m := dungMayChuNhatKyNV(t)
	var nhatKy bytes.Buffer
	m.dungLai(t, func(d *Deps) { d.Log = slog.New(slog.NewTextHandler(&nhatKy, nil)) })
	m.nhiemVu.nhatKy.loi = errors.New("kho hỏng")
	w := m.goi(t, http.MethodGet, hostA, duongNhatKyNV(maNhiemVuA), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if nhatKy.Len() == 0 {
		t.Fatal("không có dòng log nào — phép kiểm này sẽ xanh vì lý do sai")
	}
	if strings.Contains(w.Body.String(), "kho hỏng") || strings.Contains(nhatKy.String(), "Giao lại") {
		t.Error("thân lỗi lộ lỗi nội bộ, hoặc log lộ nội dung nhật ký")
	}
}
