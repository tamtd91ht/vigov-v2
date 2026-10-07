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
)

// Tests for GET /api/v1/tasks/{ma}/extensions (§5.8, the per-task extension history).
//
//	PROVED HERE   rule 5 invariant 7 — 401 no session · 403 without `task.read` (holding `task.extend`
//	              and `task.update` does not help) · 401 right key WRONG COMMUNE (this package's
//	              convention: authz compares the commune before the key) · 200 — with no store touched
//	              in the first three · 404, ONE body (the body GET /api/v1/tasks/{ma} answers), for an
//	              unknown number, another commune's number and a soft-deleted task, with the history
//	              never read · the history is read by the task's INTERNAL id in the request's commune ·
//	              the wire shape incl. `decision_note` null vs text · the order and cursor passed through
//	              · 400 on a bad page request before any store · 500 without reason/note text in a log.
//
//	NOT PROVED    the SQL — internal/store/task_extension_history_test.go (fake driver) and
//	              task_extension_history_pg_test.go (PostgreSQL; SKIPS without VIGOV_TEST_DSN).

func (g *deNghiChoDuyetGia) TaskHistory(ctx context.Context, taskID string, req page.Request) (
	page.Result[domain.DeNghiLuiHan], error) {

	g.historyCalls++
	g.historyTask, g.historyReq = taskID, req
	g.historyComm = tenant.MustFrom(ctx)
	out := page.NewResult[domain.DeNghiLuiHan]()
	if g.historyErr != nil {
		return out, g.historyErr
	}
	out.Items = append(out.Items, g.history[g.historyComm][taskID]...)
	out.NextCursor, out.HasMore = g.historyNext, g.historyNext != ""
	return out, nil
}

var (
	historyFiled1   = time.Date(2026, 9, 20, 2, 0, 0, 0, time.UTC)
	historyDecided1 = time.Date(2026, 9, 21, 4, 0, 0, 0, time.UTC)
	historyFiled2   = time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	historyNewDue   = time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
)

const historyNote = "Đồng ý, báo cáo tiến độ hằng tuần."

// seedHistory gives commune A's `nv-001` (NV19) two requests NEWEST FIRST as the store returns them —
// one pending, one rejected with a note. Commune B has a request under the SAME internal id, so a
// handler that forgot the commune would show it.
func seedHistory(m *mayChu) {
	m.deNghiCho.history = map[tenant.ID]map[string][]domain.DeNghiLuiHan{
		xaA: {
			"nv-001": {
				{ID: "dn-002", NhiemVuID: "nv-001", NguoiDeNghiMa: "CB-00311", HanMoi: historyNewDue,
					LyDo: "Chờ số liệu của thôn.", TrangThai: domain.ChoDuyetLuiHan, ThoiDiem: historyFiled2},
				{ID: "dn-001", NhiemVuID: "nv-001", NguoiDeNghiMa: "CB-00311", NguoiDuyetMa: maCanBo,
					HanMoi: historyNewDue, LyDo: "Thiếu nhân lực.", TrangThai: domain.TuChoiLuiHan,
					ThoiDiem: historyFiled1, DuyetLuc: historyDecided1, DecisionNote: historyNote},
			},
		},
		xaB: {
			"nv-001": {{ID: "dn-b01", NhiemVuID: "nv-001", NguoiDeNghiMa: "CB-B0001", HanMoi: historyNewDue,
				LyDo: "Của xã B.", TrangThai: domain.ChoDuyetLuiHan, ThoiDiem: historyFiled1}},
		},
	}
}

func newHistoryServer(t *testing.T) *mayChu {
	t.Helper()
	m := dungMayChu(t)
	seedHistory(m)
	return m
}

func historyPath(taskCode string) string { return duongNhiemVu(taskCode) + "/extensions" }

// storeReads is every store read the route could make.
func (m *mayChu) historyStoreReads() int { return m.nhiemVu.goi + m.deNghiCho.historyCalls }

// --- rule 5, invariant 7 ------------------------------------------------------------------------

func TestTaskExtensionHistoryNoSession401(t *testing.T) {
	m := newHistoryServer(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, historyPath(maNhiemVuA), nil), http.StatusUnauthorized)
	if m.historyStoreReads() != 0 {
		t.Error("đã chạm kho dù chưa có phiên")
	}
}

// 403: the account holds the two EXTENSION keys and a petition read key — not `task.read`.
func TestTaskExtensionHistoryWithoutTaskRead403(t *testing.T) {
	m := newHistoryServer(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("task.extend"): true, authz.Perm("task.update"): true,
				authz.Perm("feedback.read"): true}},
		}}
	})
	doiMa(t, m.goi(t, http.MethodGet, hostA, historyPath(maNhiemVuA), canBoCuaXa(xaA)),
		http.StatusForbidden)
	if m.historyStoreReads() != 0 {
		t.Error("đã chạm kho dù thiếu `task.read`")
	}
}

// Right key, WRONG COMMUNE: 401 (authz compares the commune before the key), nothing read.
func TestTaskExtensionHistoryRightKeyWrongCommune401(t *testing.T) {
	m := newHistoryServer(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("task.read"): true}},
			xaB: {idCanBo: {authz.Perm("task.read"): true}},
		}}
	})
	doiMa(t, m.goi(t, http.MethodGet, hostA, historyPath(maNhiemVuA), canBoCuaXa(xaB)),
		http.StatusUnauthorized)
	if m.historyStoreReads() != 0 {
		t.Error("đã chạm kho của xã A bằng phiên của xã B — rò rỉ giữa hai cơ quan nhà nước")
	}
}

// 200, with the key the history is read by, the order, the cursor and the wire shape.
func TestTaskExtensionHistoryRightKeyRightCommune200(t *testing.T) {
	m := newHistoryServer(t)
	m.deNghiCho.historyNext = "next-page-cursor"

	w := m.goi(t, http.MethodGet, hostA, historyPath(maNhiemVuA)+"?limit=2", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	if m.deNghiCho.historyTask != "nv-001" || m.deNghiCho.historyComm != xaA {
		t.Errorf("đọc lịch sử theo (%q, %q), muốn id NỘI BỘ nv-001 trong xã A",
			m.deNghiCho.historyComm, m.deNghiCho.historyTask)
	}
	if m.deNghiCho.historyReq.Limit() != 2 {
		t.Errorf("limit xuống kho = %d, muốn 2", m.deNghiCho.historyReq.Limit())
	}
	// The pending queue is a different read; this route must not touch it.
	if m.deNghiCho.goi != 0 {
		t.Error("route lịch sử gọi vào hàng chờ duyệt")
	}

	var out page.Result[deNghiLuiHanRa]
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON: %s", w.Body.String())
	}
	if !out.HasMore || out.NextCursor != "next-page-cursor" {
		t.Errorf("has_more=%v next_cursor=%q — con trỏ phải đi nguyên từ kho ra", out.HasMore, out.NextCursor)
	}
	if len(out.Items) != 2 || out.Items[0].ID != "dn-002" || out.Items[1].ID != "dn-001" {
		t.Fatalf("items = %+v, muốn [dn-002 dn-001] đúng thứ tự kho trả", out.Items)
	}
	pending, rejected := out.Items[0], out.Items[1]
	if pending.Status != "cho-duyet" || pending.DecidedBy != "" || pending.DecidedAt != nil ||
		pending.DecisionNote != nil || pending.RequestedBy != "CB-00311" ||
		!pending.RequestedAt.Equal(historyFiled2) || !pending.NewDueAt.Equal(historyNewDue) {
		t.Errorf("dòng chờ duyệt = %+v", pending)
	}
	if rejected.Status != "tu-choi" || rejected.DecidedBy != maCanBo || rejected.DecidedAt == nil ||
		!rejected.DecidedAt.Equal(historyDecided1) || rejected.DecisionNote == nil ||
		*rejected.DecisionNote != historyNote || rejected.Reason != "Thiếu nhân lực." {
		t.Errorf("dòng đã từ chối = %+v", rejected)
	}

	// ON THE RAW BODY: `decision_note` is PRESENT on every row — null when there is none, never "" or
	// absent — and no internal task id leaves.
	var raw struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &raw)
	for _, row := range raw.Items {
		for _, k := range []string{"id", "requested_by", "new_due_at", "reason", "status",
			"requested_at", "decided_at", "decision_note"} {
			if _, ok := row[k]; !ok {
				t.Errorf("thiếu trường %q", k)
			}
		}
		for _, banned := range []string{"nhiem_vu_id", "task_id"} {
			if _, ok := row[banned]; ok {
				t.Errorf("có trường %q — id nội bộ của nhiệm vụ", banned)
			}
		}
	}
	if raw.Items[0]["decision_note"] != nil {
		t.Errorf("decision_note của đề nghị chờ duyệt = %#v, muốn null", raw.Items[0]["decision_note"])
	}
	if strings.Contains(w.Body.String(), "Của xã B.") {
		t.Error("thân chứa đề nghị của xã B")
	}
}

// --- 404: one body, three causes, and the history is never read ----------------------------------

func TestTaskExtensionHistory404OneBodyThreeCauses(t *testing.T) {
	var firstBody string
	for _, c := range []struct {
		name     string
		taskCode string
		deleted  bool
	}{
		{"mã không tồn tại", "NV999", false},
		{"mã của xã khác (đúng quyền, đúng xã phiên)", maNhiemVuB, false},
		{"nhiệm vụ đã xoá mềm", maNhiemVuA, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := newHistoryServer(t)
			if c.deleted {
				m.dungLai(t, func(d *Deps) {
					d.NhiemVu = nhiemVuDaXoaGia{nhiemVuGia: m.nhiemVu, daXoa: map[string]bool{c.taskCode: true}}
				})
			}
			w := m.goi(t, http.MethodGet, hostA, historyPath(c.taskCode), canBoCuaXa(xaA))
			doiMa(t, w, http.StatusNotFound)
			if m.deNghiCho.historyCalls != 0 {
				t.Error("đã đọc lịch sử lùi hạn dù nhiệm vụ không được thấy")
			}
			if e := loiTra(t, w); e.Code != "not_found" {
				t.Errorf("mã lỗi = %q", e.Code)
			}
			if firstBody == "" {
				firstBody = w.Body.String()
			} else if w.Body.String() != firstBody {
				t.Errorf("thân 404 khác nhau giữa các nguyên nhân:\n%s\n%s", firstBody, w.Body.String())
			}
		})
	}

	m := newHistoryServer(t)
	w := m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuB), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusNotFound)
	if w.Body.String() != firstBody {
		t.Errorf("404 của lịch sử khác 404 của chi tiết nhiệm vụ:\n%s\n%s", firstBody, w.Body.String())
	}
}

// --- the rest of the read -------------------------------------------------------------------------

func TestTaskExtensionHistoryEmptyIsEmptyArray(t *testing.T) {
	m := newHistoryServer(t)
	m.deNghiCho.history[xaA]["nv-001"] = nil
	w := m.goi(t, http.MethodGet, hostA, historyPath(maNhiemVuA), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) || !strings.Contains(w.Body.String(), `"has_more":false`) {
		t.Errorf("lịch sử rỗng phải là items: [] — thân: %s", w.Body.String())
	}
}

func TestTaskExtensionHistoryBadPageRequest400NoStore(t *testing.T) {
	for _, q := range []string{"?sort=ly_do", "?cursor=not-a-cursor", "?order=sideways"} {
		t.Run(q, func(t *testing.T) {
			m := newHistoryServer(t)
			doiMa(t, m.goi(t, http.MethodGet, hostA, historyPath(maNhiemVuA)+q, canBoCuaXa(xaA)),
				http.StatusBadRequest)
			if m.historyStoreReads() != 0 {
				t.Error("đã chạm kho dù yêu cầu phân trang sai")
			}
		})
	}
}

func TestTaskExtensionHistoryStoreFailure500NoText(t *testing.T) {
	m := newHistoryServer(t)
	var logs bytes.Buffer
	m.dungLai(t, func(d *Deps) { d.Log = slog.New(slog.NewTextHandler(&logs, nil)) })
	m.deNghiCho.historyErr = errors.New("store broke")
	w := m.goi(t, http.MethodGet, hostA, historyPath(maNhiemVuA), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if logs.Len() == 0 {
		t.Fatal("không có dòng log nào — phép kiểm này sẽ xanh vì lý do sai")
	}
	if strings.Contains(w.Body.String(), "store broke") || strings.Contains(logs.String(), historyNote) ||
		strings.Contains(logs.String(), "Thiếu nhân lực") {
		t.Error("thân lỗi lộ lỗi nội bộ, hoặc log lộ lý do / ghi chú")
	}
}

// The decision reply carries the note just written; a decision without one carries null.
func TestDecisionReplyCarriesDecisionNote(t *testing.T) {
	withNote := deNghiRaNgoai(domain.DeNghiLuiHan{ID: "dn-1", DecisionNote: historyNote})
	if withNote.DecisionNote == nil || *withNote.DecisionNote != historyNote {
		t.Errorf("decision_note = %v, muốn ghi chú vừa ghi", withNote.DecisionNote)
	}
	body, _ := json.Marshal(deNghiRaNgoai(domain.DeNghiLuiHan{ID: "dn-2"}))
	if !strings.Contains(string(body), `"decision_note":null`) {
		t.Errorf("không có ghi chú phải ra null, không phải \"\" hay vắng: %s", body)
	}
}
