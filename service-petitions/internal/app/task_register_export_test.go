package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for app.TaskRegisterExport (P10).
//
//	PROVED HERE   the count refuses an oversized export BEFORE any row is read, and the page walk
//	              refuses it again when rows exceed the cap · the walk follows next_cursor through the
//	              list's own read · document blocks load per page-sized chunk · names are looked up
//	              de-duplicated and chunked to the contract ceiling · a names failure produces NO file
//	              and NO entry · a render failure writes NO entry · the audit entry is written in a
//	              committed transaction AFTER rendering, and an audit failure returns NO bytes · the
//	              entry records the row count, the sort, the filter codes, that staff names were
//	              included, and the search as present only — never its text.
//
//	NOT PROVED    the SQL of the reads (store tests) and the workbook (http tests).

type registerListFake struct {
	count      int
	pages      []page.Result[domain.NhiemVu]
	pageCalls  int
	cursors    []string
	docsCalls  int
	docsIDs    [][]string
	omitDocFor string
	tenant     tenant.ID
}

// vi-name-ok: implements TaskRegisterReader, which mirrors *petstore.NhiemVuStore's existing method
func (f *registerListFake) DanhSach(ctx context.Context, _ petstore.LocNhiemVu, yc page.Request) (
	page.Result[domain.NhiemVu], error) {
	f.tenant = tenant.MustFrom(ctx)
	a, ok := yc.After()
	c := ""
	if ok {
		c = a.ID
	}
	f.cursors = append(f.cursors, c)
	res := f.pages[f.pageCalls]
	f.pageCalls++
	return res, nil
}

func (f *registerListFake) CountByStatus(context.Context, petstore.LocNhiemVu) (map[domain.TrangThaiNhiemVu]int, error) {
	return map[domain.TrangThaiNhiemVu]int{domain.DangThucHien: f.count}, nil
}

func (f *registerListFake) DocumentsForTasks(_ context.Context, ids []string) (map[string][]domain.NhiemVuVanBan, error) {
	f.docsCalls++
	f.docsIDs = append(f.docsIDs, append([]string(nil), ids...))
	out := map[string][]domain.NhiemVuVanBan{}
	for _, id := range ids {
		if id != f.omitDocFor {
			out[id] = []domain.NhiemVuVanBan{}
		}
	}
	return out, nil
}

type registerNamesFake struct {
	staffCalls [][]string
	err        error
}

// vi-name-ok: implements RegisterNameResolver, which mirrors core/identityclient.Client's existing method
func (f *registerNamesFake) TenCanBoTheoMa(_ context.Context, ma []string) (map[string]identityclient.TenCanBo, error) {
	f.staffCalls = append(f.staffCalls, ma)
	if f.err != nil {
		return nil, f.err
	}
	return map[string]identityclient.TenCanBo{"CB-00311": {HoTen: "Lê Thị Thực Hiện"}}, nil
}

func (f *registerNamesFake) OrgUnitNames(context.Context, []string) (map[string]identityclient.OrgUnitName, error) {
	return map[string]identityclient.OrgUnitName{"bp-1": {Name: "Văn phòng"}}, nil
}

func (f *registerNamesFake) TaskBlocLabels(context.Context, []string) (map[string]identityclient.TaskBlocLabel, error) {
	return map[string]identityclient.TaskBlocLabel{}, nil
}

func registerTasks(n int, prefix string) []domain.NhiemVu {
	out := make([]domain.NhiemVu, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, domain.NhiemVu{
			ID: fmt.Sprintf("%s-%04d", prefix, i), Ma: fmt.Sprintf("NV%04d", i),
			NguoiThucHienMa: "CB-00311", BoPhanID: "bp-1",
			TaoLuc: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		})
	}
	return out
}

func nextCursorFor(id string) string {
	col := petstore.SapXepNhiemVu.Columns()[0] // created_at, the default sort
	return page.Encode(col, page.Desc, page.Anchor{Key: page.TimeKey(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)), ID: id})
}

func newRegisterExport(k *khoGia, list *registerListFake, names *registerNamesFake) *TaskRegisterExport {
	return NewTaskRegisterExport(pkgstore.New(sql.OpenDB(k)), list, names)
}

func okRender(calls *int) func(TaskRegisterData) ([]byte, error) {
	return func(TaskRegisterData) ([]byte, error) {
		*calls++
		return []byte("xlsx"), nil
	}
}

func TestRegisterExportWalksPagesRendersThenAudits(t *testing.T) {
	k := &khoGia{}
	first, second := registerTasks(100, "a"), registerTasks(3, "b")
	list := &registerListFake{count: 103, pages: []page.Result[domain.NhiemVu]{
		{Items: first, HasMore: true, NextCursor: nextCursorFor(first[99].ID)},
		{Items: second},
	}}
	names := &registerNamesFake{}
	renders := 0
	var seen TaskRegisterData

	file, n, err := newRegisterExport(k, list, names).Export(ctxXa(xaThu),
		RegisterExportRequest{Filter: petstore.LocNhiemVu{TrangThai: "dang-thuc-hien", Tim: "tổ dân phố 3"}},
		nguoiThu(), func(d TaskRegisterData) ([]byte, error) {
			renders++
			seen = d
			if len(k.lenh) != 0 {
				t.Error("vết được ghi TRƯỚC khi dựng tệp")
			}
			return []byte("xlsx"), nil
		})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if string(file) != "xlsx" || n != 103 || renders != 1 {
		t.Fatalf("tệp %q, %d dòng, %d lần dựng", file, n, renders)
	}
	if list.pageCalls != 2 || list.cursors[1] != first[99].ID || list.tenant != xaThu {
		t.Errorf("duyệt trang: %d lần, mốc %v, xã %q", list.pageCalls, list.cursors, list.tenant)
	}
	if list.docsCalls != 2 || len(list.docsIDs[0]) != 100 || len(list.docsIDs[1]) != 3 {
		t.Errorf("đọc văn bản theo lô: %d lần, cỡ %d/%d", list.docsCalls, len(list.docsIDs[0]), len(list.docsIDs[1]))
	}
	if len(names.staffCalls) != 1 || len(names.staffCalls[0]) != 1 {
		t.Errorf("tra tên cán bộ phải bỏ trùng thành 1 mã: %v", names.staffCalls)
	}
	if seen.Staff["CB-00311"].HoTen == "" || seen.Units["bp-1"].Name == "" || seen.Tasks[0].VanBan == nil {
		t.Errorf("dữ liệu đưa cho bộ dựng thiếu tên hoặc khối văn bản")
	}

	if len(k.lenh) != 1 || !strings.Contains(k.lenh[0].sql, "INSERT INTO audit_log") ||
		!k.lenh[0].trongGiaoDich || k.commit != 1 {
		t.Fatalf("vết: %+v commit=%d", k.lenh, k.commit)
	}
	args := k.lenh[0].args
	if args[1] != maCanBoThu || args[4] != ActionRegisterExport || args[5] != "so-theo-doi-nhiem-vu" {
		t.Errorf("vết: người %v, hành vi %v, đối tượng %v", args[1], args[4], args[5])
	}
	delta := string(args[7].([]byte))
	for _, want := range []string{`"so_dong":103`, `"co_ho_ten_can_bo":true`, `"status":"dang-thuc-hien"`,
		`"q_present":true`, `"sap_xep"`} {
		if !strings.Contains(delta, want) {
			t.Errorf("delta thiếu %s: %s", want, delta)
		}
	}
	if strings.Contains(delta, "tổ dân phố") {
		t.Errorf("chuỗi tìm kiếm lọt vào audit_log: %s", delta)
	}
}

func TestRegisterExportCountAboveCapReadsNothing(t *testing.T) {
	k := &khoGia{}
	list := &registerListFake{count: RegisterExportRowCap + 1}
	renders := 0

	_, _, err := newRegisterExport(k, list, &registerNamesFake{}).Export(ctxXa(xaThu),
		RegisterExportRequest{}, nguoiThu(), okRender(&renders))
	if !errors.Is(err, ErrRegisterExportTooLarge) {
		t.Fatalf("lỗi = %v, muốn ErrRegisterExportTooLarge", err)
	}
	if list.pageCalls != 0 || renders != 0 || len(k.lenh) != 0 {
		t.Errorf("đọc %d trang, dựng %d lần, %d câu lệnh — muốn 0/0/0", list.pageCalls, renders, len(k.lenh))
	}
}

func TestRegisterExportRowsAboveCapWhilePagingRefused(t *testing.T) {
	k := &khoGia{}
	list := &registerListFake{count: 10, pages: []page.Result[domain.NhiemVu]{
		{Items: registerTasks(RegisterExportRowCap+1, "x")},
	}}
	renders := 0
	_, _, err := newRegisterExport(k, list, &registerNamesFake{}).Export(ctxXa(xaThu),
		RegisterExportRequest{}, nguoiThu(), okRender(&renders))
	if !errors.Is(err, ErrRegisterExportTooLarge) || renders != 0 || len(k.lenh) != 0 {
		t.Fatalf("lỗi = %v, dựng %d, câu lệnh %d — không được cắt ở trần", err, renders, len(k.lenh))
	}
}

func TestRegisterExportNamesFailureMakesNoFileAndNoEntry(t *testing.T) {
	k := &khoGia{}
	list := &registerListFake{count: 1, pages: []page.Result[domain.NhiemVu]{{Items: registerTasks(1, "a")}}}
	names := &registerNamesFake{err: fmt.Errorf("x: %w", identityclient.ErrIdentityUnavailable)}
	renders := 0

	_, _, err := newRegisterExport(k, list, names).Export(ctxXa(xaThu), RegisterExportRequest{}, nguoiThu(), okRender(&renders))
	if !errors.Is(err, ErrRegisterNamesUnavailable) || !errors.Is(err, identityclient.ErrIdentityUnavailable) {
		t.Fatalf("lỗi = %v, muốn ErrRegisterNamesUnavailable bọc lỗi identity", err)
	}
	if renders != 0 || len(k.lenh) != 0 {
		t.Errorf("dựng %d lần, %d câu lệnh — không có tên thì không có tệp", renders, len(k.lenh))
	}
}

func TestRegisterExportRenderFailureWritesNoEntry(t *testing.T) {
	k := &khoGia{}
	list := &registerListFake{count: 1, pages: []page.Result[domain.NhiemVu]{{Items: registerTasks(1, "a")}}}

	_, _, err := newRegisterExport(k, list, &registerNamesFake{}).Export(ctxXa(xaThu), RegisterExportRequest{},
		nguoiThu(), func(TaskRegisterData) ([]byte, error) { return nil, errors.New("dựng hỏng") })
	if err == nil || len(k.lenh) != 0 {
		t.Fatalf("lỗi = %v, %d câu lệnh — tệp không dựng được thì không ghi vết xuất", err, len(k.lenh))
	}
}

func TestRegisterExportAuditFailureReturnsNoBytes(t *testing.T) {
	k := &khoGia{loi: errors.New("pg: audit_log read only")}
	list := &registerListFake{count: 1, pages: []page.Result[domain.NhiemVu]{{Items: registerTasks(1, "a")}}}
	renders := 0

	file, _, err := newRegisterExport(k, list, &registerNamesFake{}).Export(ctxXa(xaThu), RegisterExportRequest{},
		nguoiThu(), okRender(&renders))
	if err == nil || file != nil {
		t.Fatalf("lỗi = %v, tệp %q — không ghi được vết thì không có tệp", err, file)
	}
	if k.rollback != 1 || k.commit != 0 {
		t.Errorf("commit %d, rollback %d", k.commit, k.rollback)
	}
}

func TestRegisterExportMissingDocumentBlockRefused(t *testing.T) {
	k := &khoGia{}
	tasks := registerTasks(2, "a")
	list := &registerListFake{count: 2, pages: []page.Result[domain.NhiemVu]{{Items: tasks}}, omitDocFor: tasks[1].ID}
	renders := 0
	if _, _, err := newRegisterExport(k, list, &registerNamesFake{}).Export(ctxXa(xaThu), RegisterExportRequest{},
		nguoiThu(), okRender(&renders)); err == nil || renders != 0 {
		t.Fatalf("lỗi = %v, dựng %d — thiếu khối văn bản không được in thành ô trống", err, renders)
	}
}

func TestLookupChunkedRespectsTheCeiling(t *testing.T) {
	var keys []string
	for i := 0; i < identityclient.MaxLookupKeysPerCall+50; i++ {
		keys = append(keys, fmt.Sprintf("CB-%05d", i), "", fmt.Sprintf("CB-%05d", i))
	}
	var sizes []int
	got, err := lookupChunked(context.Background(), keys, func(_ context.Context, ks []string) (map[string]int, error) {
		sizes = append(sizes, len(ks))
		out := map[string]int{}
		for _, k := range ks {
			out[k] = 1
		}
		return out, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sizes) != 2 || sizes[0] != identityclient.MaxLookupKeysPerCall || sizes[1] != 50 || len(got) != 250 {
		t.Errorf("lô = %v, kết quả %d khoá", sizes, len(got))
	}
}
