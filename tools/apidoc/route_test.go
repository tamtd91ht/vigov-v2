package main

import (
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// vietNguon drops a Go file into a temporary directory and returns its path.
func vietNguon(t *testing.T, ten, nguon string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, ten)
	if err := os.WriteFile(p, []byte(nguon), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func trich(t *testing.T, nguon string) ([]tuyen, []error) {
	t.Helper()
	p := vietNguon(t, "routes.go", nguon)
	return quetFile(token.NewFileSet(), p, "identity", filepath.Dir(p))
}

const dauFile = `package http

import (
	"net/http"

	"github.com/vihat/vigov/pkg/authz"
	"github.com/vihat/vigov/pkg/idem"
)

func Register(mux *http.ServeMux) {
`

func TestTrichMethodVaPathTuServeMux(t *testing.T) {
	ts, errs := trich(t, dauFile+`
	// @summary  Đăng nhập
	// @reply    201 -
	mux.Handle("POST /api/v1/sessions",
		authz.Public("màn hình đăng nhập")(
			idem.KhongCan("mở phiên thứ hai là vô hại")(
				http.HandlerFunc(nil))))
}
`)
	if len(errs) != 0 {
		t.Fatalf("không mong đợi lỗi: %v", errs)
	}
	if len(ts) != 1 {
		t.Fatalf("mong 1 route, được %d", len(ts))
	}
	got := ts[0]
	if got.Method != "POST" || got.Path != "/api/v1/sessions" {
		t.Errorf("method/path sai: %q %q", got.Method, got.Path)
	}
	if got.Summary != "Đăng nhập" {
		t.Errorf("@summary sai: %q", got.Summary)
	}
	if got.Quyen.Kind != "public" || got.Quyen.LyDo != "màn hình đăng nhập" {
		t.Errorf("quyền đọc sai: %+v", got.Quyen)
	}
	if got.Idem.Kind != "khong-can" {
		t.Errorf("idem đọc sai: %+v", got.Idem)
	}
}

// A route inside a comment is not a route.
//
// rest_api_guard.py had to grow a whole Go-comment stripper because its regex fired on the
// commented-out example in the header of every routes.go — eight false hits across eight
// services. The same trap is here, and the AST is why it cannot be stepped in.
func TestRouteTrongCommentKhongTinh(t *testing.T) {
	ts, errs := trich(t, dauFile+`
	// Ví dụ, KHÔNG phải route thật:
	//
	//	mux.Handle("GET /api/v1/ma-ao", authz.RequirePermission(d.Checker, "ma.ao")(
	//		http.HandlerFunc(nil)))

	// @summary  Route thật
	// @reply    200 -
	mux.Handle("GET /api/v1/that",
		authz.Public("lý do")(http.HandlerFunc(nil)))
}
`)
	if len(errs) != 0 {
		t.Fatalf("không mong đợi lỗi: %v", errs)
	}
	if len(ts) != 1 {
		t.Fatalf("mong đúng 1 route, được %d: %+v", len(ts), ts)
	}
	if ts[0].Path != "/api/v1/that" {
		t.Errorf("bắt nhầm route trong comment: %q", ts[0].Path)
	}
}

func TestHandleFuncCungBatDuoc(t *testing.T) {
	ts, errs := trich(t, dauFile+`
	// @summary  Danh sách
	// @reply    200 -
	mux.HandleFunc("GET /api/v1/things",
		authz.RequirePermission(nil, "thing.read")(http.HandlerFunc(nil)).ServeHTTP)
}
`)
	if len(errs) != 0 {
		t.Fatalf("không mong đợi lỗi: %v", errs)
	}
	if len(ts) != 1 || ts[0].Method != "GET" || ts[0].Path != "/api/v1/things" {
		t.Fatalf("HandleFunc không bắt được: %+v", ts)
	}
	if ts[0].Quyen.Kind != "permission" || ts[0].Quyen.Key != "thing.read" {
		t.Errorf("khóa quyền đọc sai: %+v", ts[0].Quyen)
	}
}

func TestMountKhongPhaiRoute(t *testing.T) {
	ts, errs := trich(t, dauFile+`
	mux.Handle("/", http.HandlerFunc(nil))
}
`)
	if len(errs) != 0 || len(ts) != 0 {
		t.Fatalf("mux.Handle(\"/\", h) là mount, không phải route: %+v %v", ts, errs)
	}
}

func TestChuThichCachDongTrongThiCoiNhuChuaChuThich(t *testing.T) {
	// The block below looks annotated to a person. Requiring adjacency is what stops one route
	// borrowing the block written for another.
	_, errs := trich(t, dauFile+`
	// @summary  Trông như đã chú thích
	// @reply    200 -

	mux.Handle("GET /api/v1/things", authz.Public("lý do")(http.HandlerFunc(nil)))
}
`)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "@summary") {
		t.Fatalf("mong lỗi thiếu @summary, được: %v", errs)
	}
}

func TestThieuKhaiBaoQuyenLaLoi(t *testing.T) {
	_, errs := trich(t, dauFile+`
	// @summary  Không khai quyền
	// @reply    200 -
	mux.Handle("GET /api/v1/things", http.HandlerFunc(nil))
}
`)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "quyền") {
		t.Fatalf("mong lỗi thiếu khai báo quyền, được: %v", errs)
	}
}

func TestTheChuThichLaKhongBietThiBaoLoi(t *testing.T) {
	// A typo in a tag would otherwise drop a reply shape out of the contract without a word.
	_, errs := trich(t, dauFile+`
	// @summary  Có
	// @replies  200 -
	mux.Handle("GET /api/v1/things", authz.Public("lý do")(http.HandlerFunc(nil)))
}
`)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "@replies") {
		t.Fatalf("mong lỗi thẻ không biết, được: %v", errs)
	}
}

func TestKhoaQuyenKhongPhaiHangChuoiThiBaoLoi(t *testing.T) {
	// A permission read from a variable cannot be written into the contract, and guessing it
	// would publish a route as protected by something nobody checked.
	_, errs := trich(t, dauFile+`
	// @summary  Có
	// @reply    200 -
	mux.Handle("GET /api/v1/things", authz.RequirePermission(nil, quyenNaoDo)(http.HandlerFunc(nil)))
}
`)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "hằng chuỗi") {
		t.Fatalf("mong lỗi khóa quyền động, được: %v", errs)
	}
}

func TestReplySapXepTheoMaTrangThai(t *testing.T) {
	ts, errs := trich(t, dauFile+`
	// @summary  Có
	// @reply    500 -
	// @reply    200 -
	// @reply    404 -
	mux.Handle("GET /api/v1/things", authz.Public("lý do")(http.HandlerFunc(nil)))
}
`)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	var ma []int
	for _, r := range ts[0].Replies {
		ma = append(ma, r.Status)
	}
	if len(ma) != 3 || ma[0] != 200 || ma[1] != 404 || ma[2] != 500 {
		t.Fatalf("@reply chưa sắp xếp: %v", ma)
	}
}
