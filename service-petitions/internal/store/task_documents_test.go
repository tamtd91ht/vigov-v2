package store

import (
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/store"
)

// Tests for DocumentsForTasks — the page-wide read behind `GET /api/v1/tasks?include=documents`.
//
//	PROVED HERE   ONE statement for a whole page · the commune is $1 from the context · soft-deleted
//	              lines are excluded in the statement · every id is a bound placeholder, deduplicated ·
//	              each asked id comes back with a NON-nil slice, lines grouped to their own task in the
//	              order read · an empty id list runs no statement · a driver failure is wrapped.
//
//	NOT PROVED    what PostgreSQL does with the IN list and the ORDER BY — TestPgDocumentsForTasks in
//	              task_documents_pg_test.go, which SKIPS without VIGOV_TEST_DSN.

func documentRow(id, taskID, group string, position int) map[string]driver.Value {
	return map[string]driver.Value{
		"id": id, "nhiem_vu_id": taskID, "nhom": group,
		"so_ky_hieu": nil, "ngay_van_ban": nil, "trich_yeu": "Trích yếu " + id,
		"thu_tu": int64(position),
	}
}

func TestDocumentsForTasksOneStatementPerPage(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{
		documentRow("vb-1", "nv-1", "cap-tren-giao", 1),
		documentRow("vb-2", "nv-1", "san-pham-dau-ra", 3),
		documentRow("vb-3", "nv-3", "chi-dao-dang-uy", 1),
	}}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	got, err := s.DocumentsForTasks(ctxXa(xaThu), []string{"nv-1", "nv-2", "nv-3", "nv-1"})
	if err != nil {
		t.Fatalf("DocumentsForTasks: %v", err)
	}

	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn đúng 1 cho cả trang", len(k.lenh))
	}
	l := k.lenh[0]
	if len(l.args) != 4 || l.args[0] != string(xaThu) {
		t.Fatalf("tham số = %v, muốn xã ở $1 rồi ba mã đã bỏ trùng", l.args)
	}
	if l.args[1] != "nv-1" || l.args[2] != "nv-2" || l.args[3] != "nv-3" {
		t.Errorf("mã nhiệm vụ không được bind đúng thứ tự: %v", l.args[1:])
	}
	for _, want := range []string{"tenant_id = $1", "deleted_at IS NULL", "nhiem_vu_id IN ($2, $3, $4)",
		"ORDER BY nhiem_vu_id, nhom, thu_tu"} {
		if !strings.Contains(l.sql, want) {
			t.Errorf("câu lệnh thiếu %q: %s", want, l.sql)
		}
	}
	if strings.Contains(l.sql, "nv-1") {
		t.Errorf("mã nhiệm vụ bị ghép thẳng vào câu lệnh: %s", l.sql)
	}

	if len(got) != 3 {
		t.Fatalf("kết quả có %d khoá, muốn 3 (mỗi mã đã hỏi một khoá)", len(got))
	}
	if b := got["nv-1"]; len(b) != 2 || b[0].ID != "vb-1" || b[1].ThuTu != 3 {
		t.Errorf("nv-1 = %+v, muốn vb-1 rồi vb-2 (vị trí 3)", b)
	}
	if b, ok := got["nv-2"]; !ok || b == nil || len(b) != 0 {
		t.Errorf("nv-2 = %#v, muốn slice rỗng KHÁC nil — đã đọc, không có dòng", b)
	}
	if b := got["nv-3"]; len(b) != 1 || b[0].NhiemVuID != "nv-3" {
		t.Errorf("nv-3 = %+v, muốn đúng một dòng của chính nó", b)
	}
}

func TestDocumentsForTasksEmptyPageRunsNoStatement(t *testing.T) {
	k := &khoGia{}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	got, err := s.DocumentsForTasks(ctxXa(xaThu), nil)
	if err != nil {
		t.Fatalf("DocumentsForTasks: %v", err)
	}
	if len(k.lenh) != 0 || len(got) != 0 {
		t.Errorf("trang rỗng chạy %d câu lệnh, trả %d khoá — muốn 0 và 0", len(k.lenh), len(got))
	}
}

// TestDocumentsForTasksRefusesForeignLine — a row for a task nobody asked about means the statement
// was mis-bound; attaching it anywhere would be worse than failing.
func TestDocumentsForTasksRefusesForeignLine(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{documentRow("vb-9", "nv-khac", "cap-tren-giao", 1)}}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	if _, err := s.DocumentsForTasks(ctxXa(xaThu), []string{"nv-1"}); err == nil {
		t.Fatal("nhận một dòng của nhiệm vụ không được hỏi mà không báo lỗi")
	}
}

func TestDocumentsForTasksWrapsDriverFailure(t *testing.T) {
	cause := errors.New("mất kết nối")
	k := &khoGia{loi: cause}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	if _, err := s.DocumentsForTasks(ctxXa(xaThu), []string{"nv-1"}); !errors.Is(err, cause) {
		t.Fatalf("err = %v, muốn bọc lỗi driver bằng %%w", err)
	}
}
