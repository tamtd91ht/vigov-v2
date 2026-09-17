package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// --- resolving Go type names -------------------------------------------------------------

// kieuGo is one named type, together with everything needed to resolve the names inside it.
type kieuGo struct {
	spec    *ast.TypeSpec
	pkgDir  string
	pkgName string
}

// goiGo is one parsed package directory.
//
// Imports are unioned across the package's files, not kept per file, because the annotation
// naming a reply type sits in routes.go while the type itself is declared in handler.go and
// its `httpx.Error` sibling is imported by neither. Resolving at package scope is what a
// reader does; resolving per file would refuse a name that is obviously valid.
type goiGo struct {
	ten     string
	kieu    map[string]kieuGo
	imports map[string]string // alias -> import path
	xungDot map[string]bool   // aliases two files bind differently — refuse rather than pick
}

type giaiMa struct {
	root   string
	module string
	goi    map[string]*goiGo
}

func moGiaiMa(root string) (*giaiMa, error) {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("apidoc: đọc go.mod: %w", err)
	}
	mod := ""
	for _, ln := range strings.Split(string(b), "\n") {
		if ln = strings.TrimSpace(ln); strings.HasPrefix(ln, "module ") {
			mod = strings.TrimSpace(strings.TrimPrefix(ln, "module "))
			break
		}
	}
	if mod == "" {
		return nil, fmt.Errorf("apidoc: go.mod không khai module")
	}
	return &giaiMa{root: root, module: mod, goi: map[string]*goiGo{}}, nil
}

func (g *giaiMa) nap(pkgDir string) (*goiGo, error) {
	if p, ok := g.goi[pkgDir]; ok {
		return p, nil
	}
	ents, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, fmt.Errorf("apidoc: đọc gói %s: %w", pkgDir, err)
	}
	p := &goiGo{kieu: map[string]kieuGo{}, imports: map[string]string{}, xungDot: map[string]bool{}}
	fset := token.NewFileSet()

	ten := make([]string, 0, len(ents))
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
			ten = append(ten, e.Name())
		}
	}
	sort.Strings(ten)

	for _, n := range ten {
		f, err := parser.ParseFile(fset, filepath.Join(pkgDir, n), nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("apidoc: đọc %s: %w", filepath.Join(pkgDir, n), err)
		}
		p.ten = f.Name.Name
		for _, im := range f.Imports {
			path, err := strconv.Unquote(im.Path.Value)
			if err != nil {
				continue
			}
			alias := path[strings.LastIndex(path, "/")+1:]
			if im.Name != nil {
				alias = im.Name.Name
			}
			if cu, co := p.imports[alias]; co && cu != path {
				p.xungDot[alias] = true
			}
			p.imports[alias] = path
		}
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, s := range gd.Specs {
				ts, ok := s.(*ast.TypeSpec)
				if !ok {
					continue
				}
				p.kieu[ts.Name.Name] = kieuGo{spec: ts, pkgDir: pkgDir}
			}
		}
	}
	for k, v := range p.kieu {
		v.pkgName = p.ten
		p.kieu[k] = v
	}
	g.goi[pkgDir] = p
	return p, nil
}

// thoiGian marks the one standard-library type with a JSON representation worth knowing.
var thoiGian = kieuGo{}

// timKieu resolves a type name — bare or qualified — against a package.
func (g *giaiMa) timKieu(pkgDir, ten string) (kieuGo, error) {
	if q, n, co := strings.Cut(ten, "."); co {
		p, err := g.nap(pkgDir)
		if err != nil {
			return kieuGo{}, err
		}
		if p.xungDot[q] {
			return kieuGo{}, fmt.Errorf("bí danh import %q trỏ vào hai gói khác nhau trong cùng package — không phân giải được %s", q, ten)
		}
		duong, co := p.imports[q]
		if !co {
			return kieuGo{}, fmt.Errorf("không tìm thấy import %q để phân giải %s", q, ten)
		}
		if duong == "time" && n == "Time" {
			return thoiGian, nil
		}
		if !strings.HasPrefix(duong, g.module+"/") {
			return kieuGo{}, fmt.Errorf("%s nằm ngoài module (%s) — apidoc không đoán hình dạng kiểu ngoài", ten, duong)
		}
		dir := filepath.Join(g.root, filepath.FromSlash(strings.TrimPrefix(duong, g.module+"/")))
		return g.timTrongGoi(dir, n, ten)
	}
	return g.timTrongGoi(pkgDir, ten, ten)
}

func (g *giaiMa) timTrongGoi(pkgDir, ten, nguyen string) (kieuGo, error) {
	p, err := g.nap(pkgDir)
	if err != nil {
		return kieuGo{}, err
	}
	k, ok := p.kieu[ten]
	if !ok {
		return kieuGo{}, fmt.Errorf("không tìm thấy kiểu %s trong %s", nguyen, pkgDir)
	}
	return k, nil
}

// --- the credential guard ----------------------------------------------------------------

// Rule 3 and rule 8. Two lists because two matching rules are needed: a long name can only be
// matched as a substring ("MatKhauHash" contains "matkhau"), while a short one must match a
// WHOLE word or it fires on innocent names — "khoa" would otherwise flag "KhoaHoc", and a
// guard that cries wolf is a guard somebody deletes.
var chuaBiMat = []string{"matkhau", "password", "passwd", "secret", "token", "apikey", "privatekey", "credential"}

var tuBiMat = map[string]bool{
	"khoa": true, "key": true, "hash": true, "otp": true, "pin": true, "pw": true,
}

// laBiMat judges a field name. It looks at BOTH the Go name and the JSON name: a Go field
// `MatKhau` tagged `json:"p"` is still a password on the wire.
func laBiMat(ten, tenJSON string) bool {
	for _, n := range []string{ten, tenJSON} {
		if n == "" {
			continue
		}
		chuan := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(n))
		for _, k := range chuaBiMat {
			if strings.Contains(chuan, k) {
				return true
			}
		}
		for _, t := range tachTu(n) {
			if tuBiMat[t] {
				return true
			}
		}
	}
	return false
}

// tachTu splits camelCase and snake_case into lowercase words.
func tachTu(s string) []string {
	var out []string
	cur := strings.Builder{}
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, strings.ToLower(cur.String()))
			cur.Reset()
		}
	}
	for _, r := range s {
		switch {
		case r == '_' || r == '-':
			flush()
		case unicode.IsUpper(r):
			flush()
			cur.WriteRune(r)
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// --- Go type -> JSON Schema --------------------------------------------------------------

var coBan = map[string]string{
	"string": "string", "bool": "boolean",
	"int": "integer", "int8": "integer", "int16": "integer", "int32": "integer", "int64": "integer",
	"uint": "integer", "uint8": "integer", "uint16": "integer", "uint32": "integer",
	"uint64": "integer", "uintptr": "integer", "byte": "integer", "rune": "integer",
	"float32": "number", "float64": "number",
}

// boSchema turns named Go types into OpenAPI components.
//
// nghiem ("strict") is the credential policy, and it is a property of HOW A TYPE IS REACHED,
// not of the type. Reply shapes are built first so that a type reachable from both a request
// and a reply is judged as a reply — fail closed.
type boSchema struct {
	gm      *giaiMa
	comps   map[string]*om
	nghiem  map[string]bool   // component name -> was reached from a reply
	tenComp map[string]string // pkgDir\x00TypeName -> component name
}

func moBoSchema(gm *giaiMa) *boSchema {
	return &boSchema{
		gm:      gm,
		comps:   map[string]*om{},
		nghiem:  map[string]bool{},
		tenComp: map[string]string{},
	}
}

// refCua emits the component for a named type and returns its $ref target name.
func (b *boSchema) refCua(k kieuGo, nghiem bool) (string, error) {
	khoa := k.pkgDir + "\x00" + k.spec.Name.Name
	if ten, co := b.tenComp[khoa]; co {
		if nghiem && !b.nghiem[ten] {
			// Reached from a reply after having been emitted as request-only. Replies are
			// built first, so this cannot happen; refuse loudly rather than silently
			// downgrading the credential policy if the order ever changes.
			return "", fmt.Errorf("kiểu %s đã sinh theo diện request rồi mới gặp ở reply — "+
				"sinh reply trước request (fail closed)", k.spec.Name.Name)
		}
		return ten, nil
	}

	ten := tenThanhPhan(k)
	if _, co := b.comps[ten]; co {
		// tenComp already handled "same type seen twice", so this is genuinely two different
		// types competing for one name.
		return "", fmt.Errorf("hai kiểu khác nhau cùng tên thành phần %q", ten)
	}
	b.tenComp[khoa] = ten
	b.nghiem[ten] = nghiem
	b.comps[ten] = nil // reserve the name so a self-referencing type terminates

	s, err := b.schemaCua(k.spec.Type, k, nghiem)
	if err != nil {
		return "", fmt.Errorf("%s: %w", k.spec.Name.Name, err)
	}
	b.comps[ten] = s
	return ten, nil
}

// tenThanhPhan names a component. A type owned by a service is prefixed with the SERVICE name
// rather than its Go package name: half the packages here are called `http`, and `http.Error`
// next to `httpx.Error` in one components map is a name nobody can read.
func tenThanhPhan(k kieuGo) string {
	// Bố cục phẳng: không còn đoạn `/services/` để neo. Tên dịch vụ là đoạn đứng NGAY TRƯỚC
	// `/internal/` — cùng quy tắc mà .claude/hooks/_common.dich_vu_cua dùng.
	dir := filepath.ToSlash(k.pkgDir)
	if i := strings.Index(dir, "/internal/"); i >= 0 {
		truoc := dir[:i]
		if j := strings.LastIndex(truoc, "/"); j >= 0 {
			if ten := truoc[j+1:]; ten != "" && ten != "core" {
				return ten + "." + k.spec.Name.Name
			}
		}
	}
	return k.pkgName + "." + k.spec.Name.Name
}

// schemaCua maps one type expression. Anything it does not understand is an error: a shape
// this tool cannot describe must not be described approximately.
func (b *boSchema) schemaCua(e ast.Expr, ngucanh kieuGo, nghiem bool) (*om, error) {
	switch t := e.(type) {
	case *ast.Ident:
		if j, ok := coBan[t.Name]; ok {
			s := newOM().set("type", j)
			if t.Name == "int64" || t.Name == "uint64" {
				s.set("format", "int64")
			}
			return s, nil
		}
		if t.Name == "any" {
			return newOM(), nil
		}
		k, err := b.gm.timKieu(ngucanh.pkgDir, t.Name)
		if err != nil {
			return nil, err
		}
		return b.refHoacThoiGian(k, nghiem)

	case *ast.SelectorExpr:
		x, ok := t.X.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("biểu thức kiểu không hiểu được")
		}
		k, err := b.gm.timKieu(ngucanh.pkgDir, x.Name+"."+t.Sel.Name)
		if err != nil {
			return nil, err
		}
		return b.refHoacThoiGian(k, nghiem)

	case *ast.StarExpr:
		// A pointer means "may be absent". OpenAPI 3.1 says that with a null in the type union.
		trong, err := b.schemaCua(t.X, ngucanh, nghiem)
		if err != nil {
			return nil, err
		}
		return choPhepNull(trong), nil

	case *ast.ArrayType:
		if id, ok := t.Elt.(*ast.Ident); ok && (id.Name == "byte" || id.Name == "uint8") && t.Len == nil {
			// encoding/json base64-encodes []byte.
			return newOM().set("type", "string").set("contentEncoding", "base64"), nil
		}
		phanTu, err := b.schemaCua(t.Elt, ngucanh, nghiem)
		if err != nil {
			return nil, err
		}
		return newOM().set("type", "array").set("items", phanTu), nil

	case *ast.MapType:
		id, ok := t.Key.(*ast.Ident)
		if !ok || coBan[id.Name] != "string" {
			return nil, fmt.Errorf("map có khóa không phải string — JSON không biểu diễn được")
		}
		gt, err := b.schemaCua(t.Value, ngucanh, nghiem)
		if err != nil {
			return nil, err
		}
		return newOM().set("type", "object").set("additionalProperties", gt), nil

	case *ast.StructType:
		return b.structSchema(t, ngucanh, nghiem)

	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return newOM(), nil
		}
		return nil, fmt.Errorf("interface có phương thức — không có hình dạng JSON để công bố")
	}
	return nil, fmt.Errorf("kiểu %T chưa hỗ trợ — bổ sung tools/apidoc thay vì đoán", e)
}

func (b *boSchema) refHoacThoiGian(k kieuGo, nghiem bool) (*om, error) {
	if k.spec == nil { // time.Time
		return newOM().set("type", "string").set("format", "date-time"), nil
	}
	ten, err := b.refCua(k, nghiem)
	if err != nil {
		return nil, err
	}
	return newOM().set("$ref", "#/components/schemas/"+ten), nil
}

func choPhepNull(s *om) *om {
	if s.co("$ref") {
		// $ref beside a sibling keyword is ignored by most tooling; a union is the honest form.
		return newOM().set("anyOf", []any{s, newOM().set("type", "null")})
	}
	if t, ok := s.gt["type"].(string); ok {
		s.gt["type"] = []any{t, "null"}
		return s
	}
	return s
}

// structSchema is where the two refusals live.
func (b *boSchema) structSchema(st *ast.StructType, ngucanh kieuGo, nghiem bool) (*om, error) {
	props := newOM()
	var batBuoc []string

	for _, fld := range st.Fields.List {
		if len(fld.Names) == 0 {
			// encoding/json flattens an embedded struct into its parent. Working out which
			// fields that produces means re-implementing the promotion rules, and getting them
			// subtly wrong publishes a shape the server does not send.
			return nil, fmt.Errorf("có trường nhúng (embedded) — apidoc chưa hỗ trợ, "+
				"khai tường minh có tên và thẻ json trong %s", ngucanh.spec.Name.Name)
		}
		tenJSON, boQuaKhiRong, coThe := theJSON(fld.Tag)

		for _, nm := range fld.Names {
			if !nm.IsExported() {
				// encoding/json never emits an unexported field. There is no name to guess and
				// nothing to document.
				continue
			}
			if !coThe {
				return nil, fmt.Errorf("trường %s không có thẻ json — "+
					"apidoc KHÔNG đoán tên trường. Thêm `json:\"...\"`, hoặc `json:\"-\"` nếu nó không được ra ngoài",
					nm.Name)
			}
			ten := tenJSON
			if ten == "-" {
				continue // the sanctioned way to keep a field off the wire
			}
			if ten == "" {
				ten = nm.Name // documented encoding/json behaviour for `json:",omitempty"`
			}

			if laBiMat(nm.Name, ten) {
				if nghiem {
					return nil, fmt.Errorf(
						"trường %s (json:%q) mang tên gợi bí mật/dữ liệu cá nhân nhưng không có `json:\"-\"`, "+
							"và kiểu này nằm trong hình dạng PHẢN HỒI. "+
							"Một tài liệu công bố hình dạng chứa mật khẩu là tài liệu dạy người ta chờ mật khẩu ở đó "+
							"(luật 3, luật 8)", nm.Name, ten)
				}
				fmt.Fprintf(os.Stderr,
					"apidoc: LƯU Ý %s.%s (json:%q) là thông tin nhạy cảm đi VÀO — sinh writeOnly, không bao giờ trả ra\n",
					ngucanh.spec.Name.Name, nm.Name, ten)
			}

			s, err := b.schemaCua(fld.Type, ngucanh, nghiem)
			if err != nil {
				return nil, fmt.Errorf("trường %s: %w", nm.Name, err)
			}
			if laBiMat(nm.Name, ten) {
				s.set("writeOnly", true)
			}
			if c := moTaTruong(fld); c != "" {
				s.set("description", c)
			}
			props.set(ten, s)
			// CÓ MẶT và CÓ THỂ RỖNG là hai sự thật khác nhau, và chỉ `omitempty` quyết định
			// cái thứ nhất.
			//
			// `encoding/json` bỏ một trường đi khi và chỉ khi nó mang `omitempty`. Một con trỏ
			// KHÔNG có `omitempty` vẫn luôn được phát ra, chỉ là phát ra `null`. Nên
			// `LastLoginAt *time.Time `json:"last_login_at"`` là một trường BẮT BUỘC có mặt,
			// giá trị có thể là null — và tính null đã được ghi riêng ở `type: [T, "null"]`.
			//
			// Trước đây điều kiện này còn `&& !laConTro(...)`, tức mọi con trỏ đều bị khai là
			// không bắt buộc. Hậu quả nằm ở phía đọc hợp đồng: máy khách không còn phân biệt
			// được "máy chủ nói: chưa đăng nhập bao giờ" (null) với "máy chủ không nói gì"
			// (vắng mặt), và phải viết một nhánh không bao giờ chạy tới. Hợp đồng mô tả sai
			// một thứ máy chủ vẫn luôn làm đúng là hợp đồng dạy người ta phòng nhầm chỗ.
			if !boQuaKhiRong {
				batBuoc = append(batBuoc, ten)
			}
		}
	}

	out := newOM().set("type", "object").set("properties", props)
	if len(batBuoc) > 0 {
		sort.Strings(batBuoc)
		out.set("required", batBuoc)
	}
	out.set("additionalProperties", false)
	return out, nil
}

func laConTro(e ast.Expr) bool {
	_, ok := e.(*ast.StarExpr)
	return ok
}

// theJSON reads the `json` struct tag. coThe is false when there is no json tag at all — the
// one case this generator refuses rather than guesses.
func theJSON(tag *ast.BasicLit) (ten string, boQuaKhiRong bool, coThe bool) {
	if tag == nil {
		return "", false, false
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return "", false, false
	}
	v, ok := reflect.StructTag(raw).Lookup("json")
	if !ok {
		return "", false, false
	}
	phan := strings.Split(v, ",")
	for _, o := range phan[1:] {
		if o == "omitempty" {
			boQuaKhiRong = true
		}
	}
	return phan[0], boQuaKhiRong, true
}

// moTaTruong lifts the TRAILING one-line comment on a field into the schema, so the shape the
// web reads carries the same words the Go author wrote.
//
// Only the trailing comment, never the block above it. A field's doc comment in this codebase
// explains WHY the field exists — cookie handling, threat models, what is deliberately absent
// — and that is internal reasoning, not a description of the wire format. Publishing it would
// put security notes into a document handed to integrators.
func moTaTruong(fld *ast.Field) string {
	if fld.Comment == nil {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(fld.Comment.Text(), "\n", " "))
}
