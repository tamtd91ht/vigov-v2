package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// duongHopDong is the REST contract. It is itself generated, from the route declarations
	// in */internal/ (ADR 0014), so this generator is one link further down the same chain:
	// route in Go -> openapi.json -> ingress.yaml. Nobody hand-maintains any link.
	duongHopDong = "kb/20-contracts/openapi.json"

	// duongTepSinh is the generated routing table. It lives in base/ and not in an overlay
	// because the table is identical in every environment; what differs per environment
	// (host, TLS secret) is patched by the overlay. That is the base/overlay boundary this
	// repository already uses — base carries no namespace, no image tag, no environment.
	duongTepSinh = "deploy/base/mang/ingress.yaml"

	// tienToAPI — every REST path shares it. A path outside it is not something this
	// generator knows how to route, and it stops rather than guessing.
	tienToAPI = "/api/v1/"

	// congREST / congWeb are the PORT NAMES declared by the Services in deploy/base/*/.
	// Referring to the name rather than the number means a port renumbering in service.yaml
	// cannot leave the Ingress pointing at a port nobody listens on.
	congREST = "rest"
	congWeb  = "http"

	// dichVuWeb serves everything that is not /api/v1 — the Next.js admin web.
	dichVuWeb = "web-admin"
)

// phuongThuc — the HTTP verbs an OpenAPI path item may carry. Everything else under a path
// ("parameters", "summary", "servers", ...) is not an operation and carries no ownership.
// Listing the verbs rather than excluding the known non-verbs is the fail-closed direction:
// a key this generator does not recognise is ignored for ownership, never read as one.
var phuongThuc = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true,
}

// tuyenHopDong is one path of the REST contract together with the service that owns it.
type tuyenHopDong struct {
	Duong  string // "/api/v1/sessions/current"
	DichVu string // "identity" — from the OpenAPI tag, cross-checked against operationId
}

// luatIngress is one entry of `spec.rules[].http.paths` in the generated file.
type luatIngress struct {
	Duong  string   // "/api/v1/sessions"
	DichVu string   // name of the k8s Service
	Cong   string   // NAME of the port on that Service
	Phu    []string // the contract paths this rule covers — rendered as a comment
	BatHet bool     // the "/" catch-all: it deliberately covers everything, so the overlap
	// check skips it and the ordering always puts it last.
}

// docHopDong parses openapi.json and resolves the owning service of every path.
//
// Three independent signals must agree before a path is accepted, because the cost of being
// wrong is a request answered by a service that does not own the data:
//
//  1. the operation declares exactly ONE tag;
//  2. its operationId begins with that tag (apidoc builds it as "<tag>_<method>_<path>", so
//     a tag edited by hand without the id would slip through a single-signal check);
//  3. every operation of one path agrees on the tag.
//
// Any disagreement is an error. There is no default owner.
func docHopDong(raw []byte) ([]tuyenHopDong, error) {
	var hd struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &hd); err != nil {
		return nil, fmt.Errorf("đọc %s: %w", duongHopDong, err)
	}
	if len(hd.Paths) == 0 {
		// An empty contract would generate an Ingress with no API rule at all — every REST
		// route 404. Far more likely the file moved or the shape changed.
		return nil, fmt.Errorf("%s không có tuyến nào — hợp đồng rỗng thì không sinh bảng định tuyến", duongHopDong)
	}

	duongs := make([]string, 0, len(hd.Paths))
	for d := range hd.Paths {
		duongs = append(duongs, d)
	}
	sort.Strings(duongs)

	out := make([]tuyenHopDong, 0, len(duongs))
	for _, duong := range duongs {
		if !strings.HasPrefix(duong, tienToAPI) {
			return nil, fmt.Errorf("tuyến %q không bắt đầu bằng %q — không biết định tuyến đi đâu", duong, tienToAPI)
		}
		chu := ""
		soThaoTac := 0
		for pt, raw := range hd.Paths[duong] {
			if !phuongThuc[strings.ToLower(pt)] {
				continue
			}
			soThaoTac++
			var op struct {
				Tags        []string `json:"tags"`
				OperationID string   `json:"operationId"`
			}
			if err := json.Unmarshal(raw, &op); err != nil {
				return nil, fmt.Errorf("tuyến %s %s: %w", strings.ToUpper(pt), duong, err)
			}
			if len(op.Tags) != 1 {
				return nil, fmt.Errorf(
					"tuyến %s %s khai %d tag — cần ĐÚNG MỘT để biết dịch vụ chủ; không mặc định về identity (luật 1)",
					strings.ToUpper(pt), duong, len(op.Tags))
			}
			tag := op.Tags[0]
			if !strings.HasPrefix(op.OperationID, tag+"_") {
				return nil, fmt.Errorf(
					"tuyến %s %s: tag %q không khớp operationId %q — hai tín hiệu chủ sở hữu mâu thuẫn, không chọn hộ",
					strings.ToUpper(pt), duong, tag, op.OperationID)
			}
			if chu == "" {
				chu = tag
			} else if chu != tag {
				return nil, fmt.Errorf(
					"tuyến %s có hai dịch vụ chủ khác nhau giữa các phương thức (%q và %q) — Ingress định tuyến theo ĐƯỜNG DẪN, không theo phương thức",
					duong, chu, tag)
			}
		}
		if soThaoTac == 0 {
			return nil, fmt.Errorf("tuyến %s không có thao tác HTTP nào — không xác định được dịch vụ chủ", duong)
		}
		out = append(out, tuyenHopDong{Duong: duong, DichVu: chu})
	}
	return out, nil
}

// taiNguyen returns the resource segment of a path: the first segment after /api/v1/.
// It is the grouping key, and it is the SMALLEST grouping that is still safe. Grouping by a
// dash-prefix would look tighter and be wrong today: /api/v1/task-blocs belongs to identity
// while /api/v1/task-priorities and /api/v1/task-types belong to petitions.
func taiNguyen(duong string) (string, error) {
	con := strings.TrimPrefix(duong, tienToAPI)
	doan := strings.SplitN(con, "/", 2)[0]
	if doan == "" {
		return "", fmt.Errorf("tuyến %q không có tên tài nguyên sau %s", duong, tienToAPI)
	}
	if strings.HasPrefix(doan, "{") {
		// /api/v1/{gi-do} would make the first segment a variable, and a prefix rule built
		// from it would capture paths of every other service.
		return "", fmt.Errorf("tuyến %q có tham số ngay ở đoạn tài nguyên — không gom thành tiền tố được", duong)
	}
	return doan, nil
}

// gomTheoTaiNguyen groups the contract paths into one ingress rule per resource.
// A resource claimed by two services is a STOP: the two would need two backends behind one
// prefix, which an Ingress cannot express, and picking either one 404s the other's routes.
func gomTheoTaiNguyen(tuyens []tuyenHopDong, cong func(string) (string, error)) ([]luatIngress, error) {
	type nhom struct {
		dichVu string
		phu    []string
	}
	theoTN := map[string]*nhom{}
	var thuTu []string
	for _, t := range tuyens {
		tn, err := taiNguyen(t.Duong)
		if err != nil {
			return nil, err
		}
		n, co := theoTN[tn]
		if !co {
			theoTN[tn] = &nhom{dichVu: t.DichVu, phu: []string{t.Duong}}
			thuTu = append(thuTu, tn)
			continue
		}
		if n.dichVu != t.DichVu {
			return nil, fmt.Errorf(
				"tài nguyên %q bị hai dịch vụ nhận chủ (%s và %s, ví dụ %s) — một tiền tố Ingress chỉ trỏ được một backend; tách tên tài nguyên hoặc hỏi chủ dự án",
				tn, n.dichVu, t.DichVu, t.Duong)
		}
		n.phu = append(n.phu, t.Duong)
	}

	sort.Strings(thuTu)
	luats := make([]luatIngress, 0, len(thuTu)+1)
	for _, tn := range thuTu {
		n := theoTN[tn]
		c, err := cong(n.dichVu)
		if err != nil {
			return nil, err
		}
		sort.Strings(n.phu)
		luats = append(luats, luatIngress{
			Duong:  tienToAPI + tn,
			DichVu: n.dichVu,
			Cong:   c,
			Phu:    n.phu,
		})
	}
	return luats, nil
}

// kiemChongLan refuses any two API rules where one is a path-ELEMENT prefix of the other.
//
// pathType: Prefix matches element by element, so /api/v1/roles does NOT capture
// /api/v1/role-permissions and the two may safely point at different services. But
// /api/v1/a WOULD capture /api/v1/a/b, and if those two belonged to different services the
// generated table would be silently wrong. Nothing in the contract forbids that shape, so it
// is checked here rather than assumed away.
func kiemChongLan(luats []luatIngress) error {
	for i := range luats {
		if luats[i].BatHet {
			continue
		}
		for j := range luats {
			if i == j || luats[j].BatHet {
				continue
			}
			if laTienToDoan(luats[i].Duong, luats[j].Duong) && luats[i].DichVu != luats[j].DichVu {
				return fmt.Errorf(
					"luật %s (%s) nuốt luật %s (%s) — khớp tiền tố theo đoạn làm tuyến sau không bao giờ tới nơi",
					luats[i].Duong, luats[i].DichVu, luats[j].Duong, luats[j].DichVu)
			}
		}
	}
	return nil
}

// laTienToDoan reports whether `tienTo` matches `duong` the way `pathType: Prefix` does:
// element by element, so "/api/v1/roles" matches "/api/v1/roles/7" but not
// "/api/v1/role-permissions".
func laTienToDoan(tienTo, duong string) bool {
	if tienTo == "/" {
		return true
	}
	tienTo = strings.TrimSuffix(tienTo, "/")
	return duong == tienTo || strings.HasPrefix(duong, tienTo+"/")
}

// sapXep orders the rules longest-first, alphabetical within one length, catch-all last.
//
// ORDERING MATTERS and this is what the file guarantees. ingress-nginx sorts its locations by
// descending path length itself, so on that controller the order in this file does not decide
// the match. The file does not rely on that: written longest-first it is also correct under a
// controller that takes the first match, and it reads in the order a person would resolve it.
func sapXep(luats []luatIngress) []luatIngress {
	ra := make([]luatIngress, len(luats))
	copy(ra, luats)
	sort.SliceStable(ra, func(i, j int) bool {
		if ra[i].BatHet != ra[j].BatHet {
			return !ra[i].BatHet
		}
		if len(ra[i].Duong) != len(ra[j].Duong) {
			return len(ra[i].Duong) > len(ra[j].Duong)
		}
		return ra[i].Duong < ra[j].Duong
	})
	return ra
}

// congCuaDichVu returns the port NAME the Ingress must use for a service, after proving the
// Service object actually exists in deploy/base/<service>/service.yaml and declares it.
//
// This is the check that catches the next version of the bug this generator exists for: a new
// service acquires REST routes, the contract lists them, and there is no Service to send them
// to. The Ingress would be accepted by the API server, and every request would answer 503 with
// nothing in the manifests to explain it.
func congCuaDichVu(root string) func(string) (string, error) {
	nho := map[string]string{}
	return func(dichVu string) (string, error) {
		if c, co := nho[dichVu]; co {
			return c, nil
		}
		ten := congREST
		if dichVu == dichVuWeb {
			ten = congWeb
		}
		duong := filepath.Join(root, "deploy", "base", dichVu, "service.yaml")
		raw, err := os.ReadFile(duong)
		if err != nil {
			return "", fmt.Errorf(
				"dịch vụ %q sở hữu tuyến REST nhưng không có manifest Service ở deploy/base/%s/service.yaml — Ingress sẽ trỏ vào hư không (503), nên không sinh: %w",
				dichVu, dichVu, err)
		}
		dec := yaml.NewDecoder(strings.NewReader(string(raw)))
		for {
			var svc struct {
				Kind     string `yaml:"kind"`
				Metadata struct {
					Name string `yaml:"name"`
				} `yaml:"metadata"`
				Spec struct {
					Ports []struct {
						Name string `yaml:"name"`
					} `yaml:"ports"`
				} `yaml:"spec"`
			}
			if err := dec.Decode(&svc); err != nil {
				break
			}
			if svc.Kind != "Service" || svc.Metadata.Name != dichVu {
				continue
			}
			for _, p := range svc.Spec.Ports {
				if p.Name == ten {
					nho[dichVu] = ten
					return ten, nil
				}
			}
			return "", fmt.Errorf(
				"Service %q ở deploy/base/%s/service.yaml không khai cổng tên %q — Ingress tham chiếu cổng theo TÊN, thiếu tên là manifest không áp được",
				dichVu, dichVu, ten)
		}
		return "", fmt.Errorf("deploy/base/%s/service.yaml không chứa Service tên %q", dichVu, dichVu)
	}
}

// bangDinhTuyen turns a parsed contract into the ordered, checked list of ingress rules,
// including the "/" catch-all that sends everything else to the admin web.
func bangDinhTuyen(tuyens []tuyenHopDong, cong func(string) (string, error)) ([]luatIngress, error) {
	luats, err := gomTheoTaiNguyen(tuyens, cong)
	if err != nil {
		return nil, err
	}
	if err := kiemChongLan(luats); err != nil {
		return nil, err
	}
	congWebAdmin, err := cong(dichVuWeb)
	if err != nil {
		return nil, err
	}
	// The catch-all is NOT derived from the contract and is marked so it is never mistaken
	// for a rule that covers an API path. An /api/v1/<unknown> falls through to it and gets
	// the web app's 404 — the fail-closed direction: an unrouted API path reaches no service
	// at all rather than a service that does not own it.
	luats = append(luats, luatIngress{
		Duong:  "/",
		DichVu: dichVuWeb,
		Cong:   congWebAdmin,
		BatHet: true,
	})
	return sapXep(luats), nil
}

// sinhTuKho is the whole pipeline: read the contract from the repository, resolve, render.
// It returns the file CONTENT and writes nothing, so the test can compare it against the file
// on disk without a temporary directory.
func sinhTuKho(root string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(root, duongHopDong))
	if err != nil {
		return nil, fmt.Errorf("đọc %s: %w", duongHopDong, err)
	}
	tuyens, err := docHopDong(raw)
	if err != nil {
		return nil, err
	}
	luats, err := bangDinhTuyen(tuyens, congCuaDichVu(root))
	if err != nil {
		return nil, err
	}
	return sinhYAML(tuyens, luats), nil
}
