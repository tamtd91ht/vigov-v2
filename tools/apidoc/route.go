package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// tuyen is one HTTP route exactly as the source declares it. Nothing here is inferred: the
// method, path, permission and duplicate-request mode are read out of the statement, and the
// summary, screen and type names are read out of the comment block above it.
type tuyen struct {
	Service string
	Method  string
	Path    string
	Summary string
	Screen  string
	Request string // Go type name, "" when the route takes no body
	// Page names the page.Allowlist variable this route pages with — `@page store.SapXepCanBo`.
	//
	// NÓ LÀ MỘT CON TRỎ, KHÔNG PHẢI MỘT BẢN SAO. Danh sách cột được phép sắp xếp vẫn nằm đúng
	// một chỗ: biến Go ấy. Chú thích chỉ nói TÌM Ở ĐÂU, nên nó không thể lệch với sự thật —
	// lệch thì apidoc không giải được biến và dừng, chứ không công bố một danh sách cũ.
	Page string
	// Handler names the handler functions this route registers — `h.DanhSachDuAn`.
	//
	// ĐỌC RA CHỨ KHÔNG CHÚ THÍCH, cùng lý do với `authz.*`: nó đã nằm trong câu lệnh route. Nó
	// là lối vào để `thamSoTruyVanCua` đọc các THAM SỐ TRUY VẤN mà handler thật sự nhận —
	// xem truyvan.go.
	Handler []string
	Replies []traLoi // sorted by status
	Quyen   quyenDecl
	Idem    idemDecl
	Xa      xaDecl
	File    string // repo-relative, no line number: see the determinism note in main.go

	pkgDir string // where the annotated type names are resolved
}

type traLoi struct {
	Status int
	Kieu   string // Go type name, "" for an empty body
}

// quyenDecl mirrors the four declarations rule 5 allows, and there is no fifth. Kind is empty
// only when a route declares nothing — which this generator refuses, because an undeclared
// route is callable by every staff role and nothing reports it.
type quyenDecl struct {
	Kind   string // permission | public | any-authenticated | citizen-only
	Key    string // the flat permission key, for Kind == "permission"
	LyDo   string // the mandatory reason for public / any-authenticated
	NguonF string // file it was read from, for error messages
}

type idemDecl struct {
	Kind string // required | khong-can
	Mode string // MoKhiHong | DongKhiHong, for Kind == "required"
	LyDo string // the mandatory reason for khong-can
}

// xaDecl is the commune class of a CITIZEN route — ADR 0022. There are two, and there is no
// third: either the commune comes from the citizen session, or the route belongs to no commune
// at all and may only serve the commune-resolution path.
//
// Kind is empty when a route declares nothing, which kiemTuyen refuses on a citizen route: an
// undeclared class is deny, not allow (rule 5, invariant 2). The staff path has no class at
// all — its commune comes from Host, at httpx.TenantMiddleware.
type xaDecl struct {
	Kind string // tu-phien | khong-thuoc-xa
	LyDo string // the mandatory, specific reason for khong-thuoc-xa
}

var phuongThuc = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true,
	"DELETE": true, "HEAD": true, "OPTIONS": true,
}

// quetTuyen scans every service's internal/ tree.
//
// cmd/ is deliberately left out. The only routes registered there are /healthz and the mount
// of the service handler — infrastructure that sits outside /api/v1 and outside the tenant
// edge, which no admin screen calls. Including them would put a route in the contract that
// carries no commune and no permission, and every consumer would have to learn to skip it.
// TienToDichVu is the directory prefix that marks a backend service (`service-identity/`).
//
// tenNghiepVu strips it. The prefix tells a reader which top-level directories are backend and
// which are web; it is NOT part of the service's business name. Letting it into the generated
// contract would mean every consumer of `openapi.json` has to learn to strip it — and the one
// that forgets produces `service-identity.canBoTomTat` in a screen label.
const TienToDichVu = "service-"

func tenNghiepVu(thuMuc string) string {
	return strings.TrimPrefix(thuMuc, TienToDichVu)
}

func quetTuyen(root string) ([]tuyen, error) {
	// Bố cục phẳng: mỗi dịch vụ là một thư mục CẤP MỘT, ngang cấp với core/ và web-admin/.
	// Không còn thư mục `services/` để quét, nên dấu hiệu nhận biết là `<tên>/cmd/server`.
	// Đọc từ đĩa chứ không gõ danh sách: dịch vụ thứ chín được nhận ra ngay, và quan trọng
	// hơn, một danh sách gõ tay thiếu một dịch vụ thì hợp đồng REST lặng lẽ thiếu tuyến của
	// dịch vụ ấy — không có gì đỏ, chỉ là web không bao giờ biết tuyến đó tồn tại.
	base := root
	dichVu, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(dichVu, func(i, j int) bool { return dichVu[i].Name() < dichVu[j].Name() })

	fset := token.NewFileSet()
	var out []tuyen
	var loi []error

	for _, d := range dichVu {
		if !d.IsDir() {
			continue
		}
		ten := d.Name()
		if strings.HasPrefix(ten, ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(base, ten, "cmd", "server")); err != nil {
			continue // không phải một dịch vụ Go
		}
		trong := filepath.Join(base, ten, "internal")
		if _, err := os.Stat(trong); err != nil {
			continue
		}
		err := filepath.WalkDir(trong, func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			ts, errs := quetFile(fset, p, ten, root)
			out = append(out, ts...)
			loi = append(loi, errs...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	if len(loi) > 0 {
		return nil, errors.Join(loi...)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out, nil
}

// quetFile extracts every route registered in one file.
//
// WHY THE AST AND NOT A REGEX: rest_api_guard.py had to grow a Go-comment stripper because its
// regex fired on the commented-out example route in the header of every routes.go — eight
// false hits, and a guard that fires on a comment is a guard somebody turns off. Parsing
// removes the whole class of mistake: a route inside a comment is not a CallExpr, so it cannot
// be seen at all.
func quetFile(fset *token.FileSet, duongDan, service, root string) ([]tuyen, []error) {
	f, err := parser.ParseFile(fset, duongDan, nil, parser.ParseComments)
	if err != nil {
		return nil, []error{fmt.Errorf("apidoc: đọc %s: %w", duongDan, err)}
	}

	rel, _ := filepath.Rel(root, duongDan)
	rel = filepath.ToSlash(rel)
	pkgDir := filepath.Dir(duongDan)
	hangChuoi := hangChuoiTrongTep(f)

	var out []tuyen
	var loi []error

	ast.Inspect(f, func(n ast.Node) bool {
		st, ok := n.(*ast.ExprStmt)
		if !ok {
			return true
		}
		call, ok := st.X.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Handle" && sel.Sel.Name != "HandleFunc") || len(call.Args) == 0 {
			return true
		}
		mau, ok := mauRoute(call.Args[0], hangChuoi)
		if !ok {
			// A pattern built at runtime is a route this generator cannot describe, and a
			// route missing from the contract is exactly what it exists to prevent.
			loi = append(loi, fmt.Errorf(
				"apidoc: %s:%d: mẫu route không đọc được lúc dựng — phải là chuỗi viết thẳng, "+
					"hoặc một hằng chuỗi khai trong CÙNG tệp",
				rel, fset.Position(call.Args[0].Pos()).Line))
			return true
		}
		method, p, ok := tachMauRoute(mau)
		if !ok {
			// `mux.Handle("/", h)` mounts a handler; it is not a route.
			return true
		}

		dong := fset.Position(st.Pos()).Line
		t := tuyen{Service: tenNghiepVu(service), Method: method, Path: p, File: rel, pkgDir: pkgDir}

		if err := phanTichChuThich(chuThichTren(f, fset, dong), &t); err != nil {
			loi = append(loi, fmt.Errorf("apidoc: %s:%d: %s %s: %w", rel, dong, method, p, err))
			return true
		}
		q, id, xa, err := khaiBaoTrong(call)
		if err != nil {
			loi = append(loi, fmt.Errorf("apidoc: %s:%d: %s %s: %w", rel, dong, method, p, err))
			return true
		}
		t.Quyen, t.Idem, t.Xa = q, id, xa
		t.Handler = tenHandler(call)

		if err := kiemTuyen(&t); err != nil {
			loi = append(loi, fmt.Errorf("apidoc: %s:%d: %s %s: %w", rel, dong, method, p, err))
			return true
		}
		out = append(out, t)
		return true
	})
	return out, loi
}

// tachMauRoute splits the Go 1.22 ServeMux pattern "METHOD /path".
func tachMauRoute(mau string) (string, string, bool) {
	phan := strings.Fields(mau)
	if len(phan) != 2 {
		return "", "", false
	}
	m := strings.ToUpper(phan[0])
	if !phuongThuc[m] || !strings.HasPrefix(phan[1], "/") {
		return "", "", false
	}
	return m, phan[1], true
}

// chuThichTren returns the comment block sitting IMMEDIATELY above a line — the last comment
// line must be the line before the statement.
//
// Requiring adjacency rather than "nearest preceding comment" is deliberate. A route separated
// from its annotation by a blank line reads as annotated to a person but would let one route
// borrow the block written for another — the same trap rbac_guard hit when it anchored on line
// distance. Here the failure is loud: the route comes out unannotated and the run fails.
func chuThichTren(f *ast.File, fset *token.FileSet, dong int) string {
	for _, g := range f.Comments {
		if fset.Position(g.End()).Line == dong-1 {
			return g.Text()
		}
	}
	return ""
}

// phanTichChuThich reads the @-lines. Anything it does not recognise is an error: a typo in a
// tag would otherwise drop a reply shape from the contract without a word.
func phanTichChuThich(text string, t *tuyen) error {
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "@") {
			continue
		}
		truong := strings.Fields(ln)
		the := truong[0]
		phanCon := strings.TrimSpace(strings.TrimPrefix(ln, the))

		switch the {
		case "@summary":
			if t.Summary != "" {
				return fmt.Errorf("@summary khai hai lần")
			}
			t.Summary = phanCon
		case "@screen":
			if t.Screen != "" {
				return fmt.Errorf("@screen khai hai lần")
			}
			t.Screen = phanCon
		case "@page":
			if t.Page != "" {
				return fmt.Errorf("@page khai hai lần")
			}
			t.Page = phanCon
		case "@request":
			if t.Request != "" {
				return fmt.Errorf("@request khai hai lần")
			}
			t.Request = phanCon
		case "@reply":
			if len(truong) != 3 {
				return fmt.Errorf("@reply cần đúng dạng `@reply <mã> <Kiểu|->`, gặp %q", ln)
			}
			ma, err := strconv.Atoi(truong[1])
			if err != nil || ma < 100 || ma > 599 {
				return fmt.Errorf("@reply: %q không phải mã trạng thái HTTP", truong[1])
			}
			kieu := truong[2]
			if kieu == "-" {
				kieu = ""
			}
			for _, r := range t.Replies {
				if r.Status == ma {
					return fmt.Errorf("@reply %d khai hai lần", ma)
				}
			}
			t.Replies = append(t.Replies, traLoi{Status: ma, Kieu: kieu})
		default:
			return fmt.Errorf("thẻ chú thích không biết: %s — chỉ có @summary, @screen, @page, @request, @reply", the)
		}
	}
	sort.Slice(t.Replies, func(i, j int) bool { return t.Replies[i].Status < t.Replies[j].Status })
	return nil
}

// kiemTuyen refuses a route the contract cannot describe honestly.
func kiemTuyen(t *tuyen) error {
	if t.Summary == "" {
		return fmt.Errorf("chưa chú thích: thiếu @summary. " +
			"Khối chú thích phải nằm NGAY TRÊN câu lệnh đăng ký route, không cách dòng trống")
	}
	if len(t.Replies) == 0 {
		return fmt.Errorf("thiếu @reply — ghi đúng các mã mà handler thật sự trả, không đoán")
	}
	if t.Quyen.Kind == "" {
		return fmt.Errorf("không có khai báo quyền trong cùng câu lệnh — " +
			"cần authz.RequirePermission / Public / AnyAuthenticated / CitizenOnly (luật 5)")
	}
	// --- LỚP XÃ CỦA TUYẾN CÔNG DÂN — ADR 0022 ----------------------------------------------
	//
	// Hai trục vuông góc: `authz.*` trả lời AI, lớp xã trả lời XÃ NÀO. Cả hai đều là khai báo
	// nằm trong cùng câu lệnh route, và cả hai đều mặc định TỪ CHỐI khi thiếu.
	if t.Quyen.Kind == "citizen-only" && t.Xa.Kind == "" {
		return fmt.Errorf("tuyến công dân không khai lớp xã — cần httpx.XaTuPhien() cho tuyến " +
			"nghiệp vụ, hoặc httpx.KhongThuocXa(\"<lý do>\") cho đường phân giải xã (ADR 0022). " +
			"Thiếu khai là từ chối, không phải cho qua")
	}
	// Chiều ngược lại: lớp xã trên một tuyến KHÔNG phải công dân. `XaTuPhien` ở đó nghĩa là một
	// tuyến cán bộ lấy xã từ phiên công dân, và `KhongThuocXa` ở đó nghĩa là một tuyến cán bộ
	// mượn lối miễn trừ của kênh công dân — ADR 0022 loại thẳng cả hai. Xã của đường cán bộ đến
	// từ `Host`, ở httpx.TenantMiddleware, và không có khai báo nào đổi được điều đó.
	if t.Xa.Kind != "" && t.Quyen.Kind != "citizen-only" {
		return fmt.Errorf("lớp xã (%s) khai trên tuyến %s — lớp xã CHỈ thuộc tuyến công dân "+
			"authz.CitizenOnly(); xã của tuyến cán bộ đến từ Host (ADR 0022)",
			t.Xa.Kind, t.Quyen.Kind)
	}
	if t.Screen == "" {
		fmt.Fprintf(os.Stderr, "apidoc: LƯU Ý %s %s không có @screen — web không biết màn hình nào dùng nó\n",
			t.Method, t.Path)
	}
	return nil
}

// khaiBaoTrong reads the authz.* and idem.* declarations out of the route statement.
//
// They are read, never annotated: they already exist in the code, and a hand-written copy in a
// comment is a second source for one fact — the copy that drifts is the one a reader trusts.
func khaiBaoTrong(call *ast.CallExpr) (quyenDecl, idemDecl, xaDecl, error) {
	var q quyenDecl
	var id idemDecl
	var xa xaDecl
	var loi error

	ast.Inspect(call, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		goi, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		switch goi.Name + "." + sel.Sel.Name {
		case "authz.RequirePermission":
			if len(c.Args) < 2 {
				loi = errors.Join(loi, fmt.Errorf("authz.RequirePermission thiếu tham số"))
				return true
			}
			key, ok := chuoiLit(c.Args[1])
			if !ok {
				loi = errors.Join(loi, fmt.Errorf(
					"authz.RequirePermission: khóa quyền không phải hằng chuỗi — không ghi vào hợp đồng được"))
				return true
			}
			q = quyenDecl{Kind: "permission", Key: key}
		case "authz.Public":
			ly, _ := chuoiLit(argDau(c))
			q = quyenDecl{Kind: "public", LyDo: ly}
		case "authz.AnyAuthenticated":
			ly, _ := chuoiLit(argDau(c))
			q = quyenDecl{Kind: "any-authenticated", LyDo: ly}
		case "authz.CitizenOnly":
			q = quyenDecl{Kind: "citizen-only"}
		case "idem.Required":
			mode := ""
			if len(c.Args) == 1 {
				if s, ok := c.Args[0].(*ast.SelectorExpr); ok {
					mode = s.Sel.Name
				}
			}
			if mode != "MoKhiHong" && mode != "DongKhiHong" {
				loi = errors.Join(loi, fmt.Errorf(
					"idem.Required: không đọc được chế độ hỏng (MoKhiHong|DongKhiHong)"))
				return true
			}
			id = idemDecl{Kind: "required", Mode: mode}
		case "idem.KhongCan":
			ly, _ := chuoiLit(argDau(c))
			id = idemDecl{Kind: "khong-can", LyDo: ly}
		case "httpx.XaTuPhien":
			xa = xaDecl{Kind: "tu-phien"}
		case "httpx.KhongThuocXa":
			ly, _ := chuoiLit(argDau(c))
			// Lý do đọc được THÀNH HẰNG CHUỖI mới ghi được vào hợp đồng. Một lý do lấy từ biến
			// thì người rà soát mở `openapi.json` ra thấy một miễn trừ không kèm lý do — đúng
			// thứ luật 5 cấm #4 nói tới, chỉ là muộn hơn sáu tháng.
			if strings.TrimSpace(ly) == "" {
				loi = errors.Join(loi, fmt.Errorf(
					"httpx.KhongThuocXa: thiếu lý do cụ thể dạng hằng chuỗi — một miễn trừ "+
						"không có lý do là thứ không ai dám gỡ về sau (luật 5 cấm #4)"))
				return true
			}
			xa = xaDecl{Kind: "khong-thuoc-xa", LyDo: ly}
		}
		return true
	})
	return q, id, xa, loi
}

func argDau(c *ast.CallExpr) ast.Expr {
	if len(c.Args) == 0 {
		return nil
	}
	return c.Args[0]
}

// mauRoute reads the route pattern out of the first argument of mux.Handle.
//
// TWO SHAPES ARE ACCEPTED, AND THE SECOND ONE IS WHY THIS FUNCTION EXISTS. A literal is the
// common case. A NAMED CONSTANT declared in the same file is the case the generator used to
// refuse — and refusing it punished the shape this repository actually wants: a path that is
// registered, asserted in a test and printed in a runbook has to have ONE source, which is
// exactly what a constant is (rule 9). `service-platform/internal/http/webhook_zalo.go:14`
// declares `DuongDanWebhookZalo` for that reason, and the whole generator then failed on it —
// `make kb` could not regenerate the REST contract at all while the author had done the right
// thing. The alternative the old behaviour pushed people toward was writing the path twice,
// which is the drift rule 9 exists to prevent.
//
// SAME FILE ONLY, DELIBERATELY. Reaching across the package would mean parsing every file of
// the package to answer a question about one call, and a constant that lives in another file
// is a constant a reader of this call cannot see either. The error message names this limit
// instead of leaving the author to guess which half of the rule they broke.
//
// EVERYTHING ELSE IS STILL AN ERROR, and that half must not soften: a pattern built at runtime
// (a variable, a concatenation, fmt.Sprintf) is a route that cannot appear in the contract, and
// a route missing from the contract is the one thing this generator exists to make impossible.
func mauRoute(e ast.Expr, hangChuoi map[string]string) (string, bool) {
	if s, ok := chuoiLit(e); ok {
		return s, true
	}
	id, ok := e.(*ast.Ident)
	if !ok {
		return "", false
	}
	s, ok := hangChuoi[id.Name]
	return s, ok
}

// hangChuoiTrongTep collects the file's package-level `const X = "..."` declarations.
//
// Only untyped string constants with one name and one literal value: `const a, b = "x", "y"` and
// a constant whose value is itself an expression are left out rather than half-resolved. A
// generator that guesses at a value it cannot see is worse than one that says it cannot see it.
func hangChuoiTrongTep(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, d := range f.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, s := range gen.Specs {
			vs, ok := s.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			if v, ok := chuoiLit(vs.Values[0]); ok {
				out[vs.Names[0].Name] = v
			}
		}
	}
	return out
}

// chuoiLit unquotes a string literal, including a raw one.
func chuoiLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}
