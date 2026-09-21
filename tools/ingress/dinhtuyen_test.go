package main

import (
	"strings"
	"testing"
)

// congGia stands in for the deploy/base/*/service.yaml lookup so these tests exercise the
// routing decision and nothing else.
func congGia(string) (string, error) { return congREST, nil }

func hopDongGia(than string) []byte {
	return []byte(`{"paths":{` + than + `}}`)
}

// TestDungKhiKhongXacDinhDuocDichVuChu — the STOP conditions. Every case here is one where a
// generator could plausibly "just pick identity" and produce a file that looks right. None of
// them may produce output: a route pointed at a service that does not own the data is rule 1
// broken at the outermost edge.
func TestDungKhiKhongXacDinhDuocDichVuChu(t *testing.T) {
	cases := []struct {
		ten  string
		than string
		moi  string // a fragment the error must name, so the message stays useful
	}{
		{
			ten:  "không có tag nào",
			than: `"/api/v1/x":{"get":{"operationId":"identity_get_x"}}`,
			moi:  "0 tag",
		},
		{
			ten:  "hai tag",
			than: `"/api/v1/x":{"get":{"tags":["identity","finance"],"operationId":"identity_get_x"}}`,
			moi:  "2 tag",
		},
		{
			ten:  "tag không khớp operationId",
			than: `"/api/v1/x":{"get":{"tags":["finance"],"operationId":"identity_get_x"}}`,
			moi:  "không khớp operationId",
		},
		{
			ten: "hai phương thức, hai chủ",
			than: `"/api/v1/x":{"get":{"tags":["identity"],"operationId":"identity_get_x"},` +
				`"post":{"tags":["finance"],"operationId":"finance_post_x"}}`,
			moi: "hai dịch vụ chủ khác nhau",
		},
		{
			ten:  "không có thao tác HTTP nào",
			than: `"/api/v1/x":{"parameters":[]}`,
			moi:  "không có thao tác HTTP",
		},
		{
			ten:  "đường dẫn ngoài /api/v1/",
			than: `"/healthz":{"get":{"tags":["identity"],"operationId":"identity_get_healthz"}}`,
			moi:  "không bắt đầu bằng",
		},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			_, err := docHopDong(hopDongGia(c.than))
			if err == nil {
				t.Fatal("bộ sinh CHẤP NHẬN một tuyến không xác định được chủ — phải DỪNG, không đoán")
			}
			if !strings.Contains(err.Error(), c.moi) {
				t.Errorf("thông báo lỗi không nói rõ chuyện gì: %v", err)
			}
		})
	}
}

// TestHopDongRongLaDung — an empty contract would render an Ingress whose only rule is the
// catch-all. Every REST route would 404 and the file would still look plausible.
func TestHopDongRongLaDung(t *testing.T) {
	if _, err := docHopDong([]byte(`{"paths":{}}`)); err == nil {
		t.Fatal("hợp đồng rỗng phải DỪNG bộ sinh")
	}
}

// TestHaiDichVuNhanMotTaiNguyenLaDung — one prefix cannot point at two backends. Choosing
// either one silently 404s the other service's routes.
func TestHaiDichVuNhanMotTaiNguyenLaDung(t *testing.T) {
	tuyens := []tuyenHopDong{
		{Duong: "/api/v1/task-types", DichVu: "petitions"},
		{Duong: "/api/v1/task-types/{id}", DichVu: "identity"},
	}
	_, err := gomTheoTaiNguyen(tuyens, congGia)
	if err == nil {
		t.Fatal("hai chủ trên một tài nguyên phải DỪNG")
	}
	if !strings.Contains(err.Error(), "hai dịch vụ nhận chủ") {
		t.Errorf("thông báo lỗi không nói rõ: %v", err)
	}
}

// TestGomDungMucTaiNguyen is the constraint from the brief, checked against the real shape of
// the contract: group by resource prefix, but never so far that one prefix points at the
// wrong service. `task-blocs` is identity; `task-priorities` and `task-types` are petitions.
// A generator grouping on "task" would send two petitions routes to identity.
func TestGomDungMucTaiNguyen(t *testing.T) {
	tuyens := []tuyenHopDong{
		{Duong: "/api/v1/task-blocs", DichVu: "identity"},
		{Duong: "/api/v1/task-priorities", DichVu: "petitions"},
		{Duong: "/api/v1/task-types", DichVu: "petitions"},
		{Duong: "/api/v1/sessions", DichVu: "identity"},
		{Duong: "/api/v1/sessions/current", DichVu: "identity"},
		{Duong: "/api/v1/sessions/{sid}", DichVu: "identity"},
	}
	luats, err := gomTheoTaiNguyen(tuyens, congGia)
	if err != nil {
		t.Fatalf("gom thất bại: %v", err)
	}
	if len(luats) != 4 {
		t.Fatalf("mong 4 luật (task-blocs · task-priorities · task-types · sessions), có %d", len(luats))
	}
	theoDuong := map[string]luatIngress{}
	for _, l := range luats {
		theoDuong[l.Duong] = l
	}
	if l, co := theoDuong["/api/v1/task-blocs"]; !co || l.DichVu != "identity" {
		t.Errorf("task-blocs phải trỏ identity, có %+v", l)
	}
	if l, co := theoDuong["/api/v1/task-types"]; !co || l.DichVu != "petitions" {
		t.Errorf("task-types phải trỏ petitions, có %+v", l)
	}
	if l := theoDuong["/api/v1/sessions"]; len(l.Phu) != 3 {
		t.Errorf("ba tuyến sessions phải gom thành MỘT luật, luật ấy phủ %d tuyến", len(l.Phu))
	}
	if _, co := theoDuong["/api/v1/task"]; co {
		t.Error("có luật tiền tố `task` — nó sẽ nuốt cả identity lẫn petitions")
	}
}

// TestKhopTienToTheoDoan pins down the matching semantics the grouping relies on. Character
// prefixing would make /api/v1/roles capture /api/v1/role-permissions; pathType Prefix does
// not, and that is exactly why grouping by resource is safe here.
func TestKhopTienToTheoDoan(t *testing.T) {
	cases := []struct {
		tienTo, duong string
		mong          bool
	}{
		{"/api/v1/roles", "/api/v1/roles", true},
		{"/api/v1/roles", "/api/v1/roles/7", true},
		{"/api/v1/roles", "/api/v1/role-permissions", false},
		{"/api/v1/residential-units", "/api/v1/residential-unit-types", false},
		{"/api/v1/staff", "/api/v1/staff/{id}", true},
		{"/", "/api/v1/bat-ky", true},
	}
	for _, c := range cases {
		if got := laTienToDoan(c.tienTo, c.duong); got != c.mong {
			t.Errorf("laTienToDoan(%q, %q) = %v, mong %v", c.tienTo, c.duong, got, c.mong)
		}
	}
}

// TestLuatNuotNhauLaDung — the shape that character-level reasoning would miss: a resource
// whose sub-path belongs to another service. Nothing in the contract forbids it today, so the
// generator refuses rather than emitting a table where the second rule never receives a
// request.
func TestLuatNuotNhauLaDung(t *testing.T) {
	luats := []luatIngress{
		{Duong: "/api/v1/a", DichVu: "identity"},
		{Duong: "/api/v1/a/b", DichVu: "finance"},
	}
	if err := kiemChongLan(luats); err == nil {
		t.Fatal("luật nuốt luật phải DỪNG bộ sinh")
	}
}

// TestDichVuThieuManifestLaDung — a service that owns routes but has no Service object. The
// Ingress would be accepted and every request would answer 503, with nothing in deploy/ to
// explain it. `reporting` is the live candidate: zero routes today, so no manifest.
func TestDichVuThieuManifestLaDung(t *testing.T) {
	root := goc(t)
	_, err := gomTheoTaiNguyen(
		[]tuyenHopDong{{Duong: "/api/v1/bao-cao", DichVu: "reporting"}},
		congCuaDichVu(root),
	)
	if err == nil {
		t.Fatal("dịch vụ không có manifest Service phải DỪNG bộ sinh")
	}
	if !strings.Contains(err.Error(), "deploy/base/reporting/service.yaml") {
		t.Errorf("thông báo lỗi không chỉ ra tệp còn thiếu: %v", err)
	}
}

// TestSapXepDaiTruocBatHetCuoi — ordering, at the unit level.
func TestSapXepDaiTruocBatHetCuoi(t *testing.T) {
	ra := sapXep([]luatIngress{
		{Duong: "/", BatHet: true},
		{Duong: "/api/v1/ab"},
		{Duong: "/api/v1/abcdef"},
		{Duong: "/api/v1/aa"},
	})
	mong := []string{"/api/v1/abcdef", "/api/v1/aa", "/api/v1/ab", "/"}
	for i, m := range mong {
		if ra[i].Duong != m {
			t.Fatalf("thứ tự sai: %+v", ra)
		}
	}
}
