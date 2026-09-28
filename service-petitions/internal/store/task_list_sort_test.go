package store

import (
	"database/sql/driver"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
)

// Tests for `sort=priority` and `sort=title` on the task register (P9, 28/09/2026).
//
//	PROVED HERE   priority pages the catalogue-joined relation, with the join bound to the commune,
//	              orders by the key of the direction asked, scans that key into the anchor and binds
//	              it on page two (ascending rebuilt onto the ascending key) · a key paired with the
//	              wrong direction is refused · title pages the PLAIN table, its cursor carries the row
//	              id and NO title text, and page two looks the title up from the anchor row inside the
//	              statement — the title is never a bound value either.
//
//	NOT PROVED    PostgreSQL's join, COALESCE and scalar-subquery evaluation, nor collation order —
//	              TestPgPriorityAndTitleSortWalk (task_list_sort_pg_test.go), which SKIPS without
//	              VIGOV_TEST_DSN.

func sortRequest(t *testing.T, q url.Values) page.Request {
	t.Helper()
	yc, err := page.Parse(q, SapXepNhiemVu)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	return yc
}

func twoTaskRows(extra map[string]driver.Value) []map[string]driver.Value {
	a := map[string]driver.Value{"id": "nv-001", "ma": "NV01"}
	b := map[string]driver.Value{"id": "nv-002", "ma": "NV02"}
	for k, v := range extra {
		a[k], b[k] = v, v
	}
	return []map[string]driver.Value{dongNhiemVu(a), dongNhiemVu(b)}
}

func TestPrioritySortPagesTheCatalogueJoinAndCarriesTheRank(t *testing.T) {
	k := &khoGia{hangTheoCot: twoTaskRows(map[string]driver.Value{prioritySortDescColumn: int64(2)})}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	kq, err := s.DanhSach(ctxXa(xaThu), LocNhiemVu{}, sortRequest(t, url.Values{"sort": {"priority"}, "limit": {"1"}}))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	q := k.lenh[0].sql
	for _, want := range []string{
		"LEFT JOIN muc_uu_tien_nhiem_vu m ON m.tenant_id = $1 AND m.ma = nhiem_vu.muc_uu_tien",
		"ORDER BY " + prioritySortDescColumn + " DESC, id DESC",
		", " + prioritySortDescColumn + " FROM ",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("câu lệnh thiếu %q: %s", want, q)
		}
	}
	if !kq.HasMore || kq.Items[0].PriorityRank != 2 {
		t.Fatalf("trang = %+v, muốn còn trang sau và hạng 2 đã quét", kq)
	}

	// Page two, ASCENDING: rebuilt onto the ascending key, with the rank as the bound anchor.
	// The client's cursor is issued against the PUBLISHED list (the param, not the SQL key);
	// forDueDirection re-encodes it onto the ascending key.
	pub, _ := columnOf(SapXepNhiemVu, "priority")
	cursor := page.Encode(pub, page.Asc, page.Anchor{Key: page.IntKey(2), ID: "nv-001"})
	k2 := &khoGia{hangTheoCot: twoTaskRows(map[string]driver.Value{prioritySortAscColumn: int64(3)})}
	s2 := NewNhiemVuStore(store.New(moKhoGia(k2)))
	if _, err := s2.DanhSach(ctxXa(xaThu), LocNhiemVu{},
		sortRequest(t, url.Values{"sort": {"priority"}, "order": {"asc"}, "cursor": {cursor}})); err != nil {
		t.Fatalf("DanhSach trang 2: %v", err)
	}
	l := k2.lenh[0]
	if !strings.Contains(l.sql, "("+prioritySortAscColumn+", id) > ($2, $3)") ||
		!strings.Contains(l.sql, "ORDER BY "+prioritySortAscColumn+" ASC, id ASC") {
		t.Errorf("trang 2 không sắp theo khoá tăng: %s", l.sql)
	}
	if l.args[1] != int64(2) || l.args[2] != "nv-001" {
		t.Errorf("mốc trang 2 = %v, muốn (2, nv-001)", l.args[1:3])
	}
}

func TestPrioritySortKeyDirectionMismatchRefused(t *testing.T) {
	asc, _ := columnOf(taskSortAscAllowlist, "priority")
	yc, err := page.New(taskSortAscAllowlist, "priority", "desc", "", "")
	if err != nil || yc.Column().SQL != asc.SQL {
		t.Fatalf("dựng yêu cầu: %v", err)
	}
	if _, _, _, err := taskPageShape(yc); !errors.Is(err, ErrTaskSortDirection) {
		t.Errorf("err = %v, muốn ErrTaskSortDirection", err)
	}
}

func TestTitleSortCursorCarriesNoTitleAndLooksItUp(t *testing.T) {
	title := "Phản ánh tiếng ồn kéo dài tại tổ dân phố 3"
	k := &khoGia{hangTheoCot: twoTaskRows(map[string]driver.Value{"tieu_de": title})}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	kq, err := s.DanhSach(ctxXa(xaThu), LocNhiemVu{},
		sortRequest(t, url.Values{"sort": {"title"}, "order": {"asc"}, "limit": {"1"}}))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if !strings.Contains(k.lenh[0].sql, " FROM nhiem_vu WHERE tenant_id = $1") ||
		!strings.Contains(k.lenh[0].sql, "ORDER BY tieu_de ASC, id ASC") {
		t.Errorf("trang 1 không sắp theo tiêu đề trên bảng trần: %s", k.lenh[0].sql)
	}
	raw, err := base64.RawURLEncoding.DecodeString(kq.NextCursor)
	if err != nil || !kq.HasMore {
		t.Fatalf("con trỏ: %v, has_more=%v", err, kq.HasMore)
	}
	if strings.Contains(string(raw), "ồn") || strings.Contains(string(raw), "dân phố") || !strings.Contains(string(raw), `"i":"nv-001"`) {
		t.Fatalf("con trỏ mang tiêu đề hoặc thiếu mã dòng: %s", raw)
	}

	k2 := &khoGia{hangTheoCot: twoTaskRows(map[string]driver.Value{"tieu_de": title})}
	s2 := NewNhiemVuStore(store.New(moKhoGia(k2)))
	if _, err := s2.DanhSach(ctxXa(xaThu), LocNhiemVu{},
		sortRequest(t, url.Values{"sort": {"title"}, "order": {"asc"}, "cursor": {kq.NextCursor}})); err != nil {
		t.Fatalf("DanhSach trang 2: %v", err)
	}
	l := k2.lenh[0]
	if !strings.Contains(l.sql, "AND (tieu_de, id) > ((SELECT r.tieu_de FROM nhiem_vu r WHERE r.tenant_id = $1 AND r.id = $2), $2)") {
		t.Errorf("trang 2 không tra tiêu đề từ dòng mốc: %s", l.sql)
	}
	for _, a := range l.args {
		if s, ok := a.(string); ok && strings.Contains(s, "tổ dân phố") {
			t.Errorf("tiêu đề bị ràng buộc làm tham số: %v", l.args)
		}
	}
}
