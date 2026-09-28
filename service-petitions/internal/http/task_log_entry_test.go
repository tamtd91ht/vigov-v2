package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Tests for POST /api/v1/tasks/{ma}/log-entries at the HTTP boundary. The four rule-5 cases are rows of
// caCacTuyenGhiNhiemVu (nhiem_vu_ghi_test.go). What is proved HERE: the `task.update` fact handed down
// is read from THAT key and nothing else · the body reaches the use case · the reply is the timeline
// row · the use case's refusals map to 403 / 400 with the domain's sentence and no commune id.
// The holder rule itself is proved over the real store in internal/app/task_log_entry_test.go.

func taskLogEntriesPath(ma string) string { return duongNV(ma) + "/log-entries" }

func TestAddTaskLogEntry_HandsDownTaskUpdateFactFromThatKey(t *testing.T) {
	for _, c := range []struct {
		name  string
		perms []authz.Perm
		want  bool
	}{
		{"chỉ task.read", []authz.Perm{"task.read"}, false},
		// task.approve is a real key of this subsystem and must NOT count as the edit right.
		{"task.read + task.approve", []authz.Perm{"task.read", "task.approve"}, false},
		{"task.read + task.update", []authz.Perm{"task.read", "task.update"}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, c.perms...)

			w := m.goiGhiNV(t, http.MethodPost, hostA, taskLogEntriesPath(maNVThu), canBoCuaXa(xaA),
				taskLogEntryIn{Note: "Đã gửi công văn sang huyện."})

			doiMa(t, w, http.StatusCreated)
			if bool(m.ghiNhiemVu.taskUpdate) != c.want {
				t.Errorf("sự thật task.update truyền xuống = %v, muốn %v", m.ghiNhiemVu.taskUpdate, c.want)
			}
			if m.ghiNhiemVu.maDa != maNVThu || m.ghiNhiemVu.logText != "Đã gửi công văn sang huyện." {
				t.Errorf("use case nhận %q / %q", m.ghiNhiemVu.maDa, m.ghiNhiemVu.logText)
			}
			var row nhatKyNhiemVuRa
			if err := json.Unmarshal(w.Body.Bytes(), &row); err != nil {
				t.Fatalf("thân phản hồi không phải JSON: %s", w.Body.String())
			}
			if row.ID != "nk-001" || row.ActorCode != maCanBo || row.Note != "Đã gửi công văn sang huyện." {
				t.Errorf("phản hồi = %+v", row)
			}
		})
	}
}

func TestAddTaskLogEntry_RefusalsMapWithDomainSentence(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		want int
		code string
	}{
		{"không phải người liên quan", domain.ErrNotTaskParticipant, http.StatusForbidden, "forbidden"},
		{"thiếu nội dung", domain.ErrThieuNoiDungNhatKy, http.StatusBadRequest, "invalid_request"},
		{"nội dung quá dài", domain.ErrNoiDungNhatKyQuaDai, http.StatusBadRequest, "invalid_request"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("task.read"))
			m.ghiNhiemVu.loi = bocNhuApp(c.err)

			w := m.goiGhiNV(t, http.MethodPost, hostA, taskLogEntriesPath(maNVThu), canBoCuaXa(xaA),
				taskLogEntryIn{Note: "x"})

			doiMa(t, w, c.want)
			e := loiTra(t, w)
			if e.Code != c.code || e.Message != c.err.Error() {
				t.Errorf("lỗi = %q / %q, muốn %q / %q", e.Code, e.Message, c.code, c.err.Error())
			}
			if strings.Contains(w.Body.String(), string(xaBocThu)) {
				t.Errorf("thân lộ mã xã: %s", w.Body.String())
			}
		})
	}
}
