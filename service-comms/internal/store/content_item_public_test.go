package store

import (
	"errors"
	"strings"
	"testing"
)

// The public reads (content_item_public.go), over the SAME fake driver as the staff register. It proves
// the statement: commune bound to $1 from the context, soft-deleted rows excluded, the state bound to
// `dang-hien`, no body on the page. That the database actually returns only published rows of one
// commune is the pg suite's (content_item_public_pg_test.go).

func TestListPublicOnlyVisibleAndBindsTenant(t *testing.T) {
	f := &fakeContentStore{rows: []fakeContentRow{sampleContentRow()}}
	s, ctx := newTestContentItemStore(t, f)

	res, err := s.ListPublic(ctx, "", contentFirstPage(t))
	if err != nil {
		t.Fatalf("đọc trang công khai lỗi: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].Body != "" {
		t.Fatalf("trang công khai = %+v — một dòng, không mang toàn văn", res.Items)
	}
	st := f.stmts[0]
	// MUTATIONS THAT MUST TURN THIS RED: drop the state clause (every draft is published), drop the
	// soft-delete clause, or bind the state to anything but `dang-hien`.
	for _, want := range []string{"tenant_id = $1", "deleted_at IS NULL", "trang_thai = $2",
		"ORDER BY tao_luc DESC, id DESC"} {
		if !strings.Contains(st.sql, want) {
			t.Errorf("câu đọc trang công khai thiếu %q: %s", want, st.sql)
		}
	}
	if st.args[0] != string(contentTenant) || st.args[1] != "dang-hien" {
		t.Fatalf("tham số = %v, muốn [xã của ngữ cảnh, dang-hien, …]", st.args)
	}
	if cols := st.sql[:strings.Index(st.sql, " FROM ")]; strings.Contains(cols, "noi_dung") {
		t.Errorf("trang công khai chọn cả `noi_dung`: %s", cols)
	}
	if strings.Contains(st.sql, "loai =") {
		t.Errorf("không lọc loại mà câu vẫn có `loai =`: %s", st.sql)
	}
}

func TestPublicContentListTypeFilterKeepsStateAndCommune(t *testing.T) {
	fake := &fakeContentStore{rows: []fakeContentRow{sampleContentRow()}}
	repo, ctx := newTestContentItemStore(t, fake)

	if _, err := repo.ListPublic(ctx, "su-kien", contentFirstPage(t)); err != nil {
		t.Fatalf("đọc trang công khai lọc loại lỗi: %v", err)
	}
	stmt := fake.stmts[0]
	// MUTATIONS THAT MUST TURN THIS RED: the type clause replaces the state clause, or binds the type
	// to the state's placeholder.
	for _, want := range []string{"tenant_id = $1", "deleted_at IS NULL", "trang_thai = $2", "loai = $3"} {
		if !strings.Contains(stmt.sql, want) {
			t.Errorf("câu đọc trang công khai lọc loại thiếu %q: %s", want, stmt.sql)
		}
	}
	if stmt.args[0] != string(contentTenant) || stmt.args[1] != "dang-hien" || stmt.args[2] != "su-kien" {
		t.Fatalf("tham số = %v, muốn [xã, dang-hien, su-kien, …]", stmt.args)
	}
}

func TestPublicByIDOnlyVisibleAndBindsTenant(t *testing.T) {
	f := &fakeContentStore{rows: []fakeContentRow{sampleContentRow()}}
	s, ctx := newTestContentItemStore(t, f)

	c, err := s.PublicByID(ctx, "nd-001")
	if err != nil {
		t.Fatalf("đọc chi tiết công khai lỗi: %v", err)
	}
	if c.Body != "<p>Toàn văn</p>" {
		t.Fatalf("toàn văn = %q", c.Body)
	}
	st := f.stmts[0]
	for _, want := range []string{"tenant_id = $1", "id = $2", "deleted_at IS NULL", "trang_thai = $3"} {
		if !strings.Contains(st.sql, want) {
			t.Errorf("câu đọc chi tiết công khai thiếu %q: %s", want, st.sql)
		}
	}
	if st.args[0] != string(contentTenant) || st.args[1] != "nd-001" || st.args[2] != "dang-hien" {
		t.Fatalf("tham số = %v, muốn [xã, nd-001, dang-hien]", st.args)
	}
}

func TestPublicByIDNoRowReportsNotFound(t *testing.T) {
	// A draft, another commune's id and no id at all are ONE answer: the predicate returns no row.
	s, ctx := newTestContentItemStore(t, &fakeContentStore{})
	if _, err := s.PublicByID(ctx, "nd-ban-nhap"); !errors.Is(err, ErrContentItemNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrContentItemNotFound", err)
	}
}
