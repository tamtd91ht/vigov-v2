package main

import (
	"fmt"
	"go/ast"
	"strconv"
	"strings"
)

// Reading the pagination contract out of the Go code, so it is never typed twice.
//
// VẤN ĐỀ NÀY TỆP SINH RA ĐỂ GIẢI. `kb/20-contracts/openapi.json` không khai tham số truy vấn
// nào, nên `limit`, `cursor`, `sort`, `order` — và nhất là DANH SÁCH CỘT được phép sắp xếp —
// không có mặt trong hợp đồng. Hệ quả đo được: web quản trị phải gõ tay
// `KHOA_SAP_XEP = ["code", "created_at"]`, tức chỗ duy nhất trong cả ứng dụng chép một phần
// hợp đồng, với nguồn sự thật nằm cách đó hai module trong một tệp Go.
//
// Một bản chép như thế không làm đỏ bài test nào lúc nó trôi. Thêm một cột sắp xếp ở máy chủ
// thì web không biết; gỡ một cột đi thì web vẫn gửi và nhận 400 — trên một màn hình mà cán bộ
// chỉ thấy "không tải được danh sách".
//
// CÁCH GIẢI: chú thích `@page store.SapXepCanBo` là một CON TRỎ, không phải một bản sao. Danh
// sách vẫn nằm đúng một chỗ — biến Go ấy — và apidoc đọc chính biến ấy. Chú thích trỏ sai thì
// bộ sinh DỪNG, chứ không công bố một danh sách cũ.

// phanTrang is what one route's `@page` resolves to.
type phanTrang struct {
	Cot       []string // tên tham số của các cột được phép, cột đầu là mặc định
	ChieuMacD string   // "asc" | "desc"
	LimitMacD int
	LimitMax  int
}

// docPhanTrang resolves `@page <ident>` into the pagination contract that route really offers.
//
// Nó CHỈ đọc được dạng khai báo thẳng:
//
//	var SapXepCanBo = page.NewAllowlist(page.Asc,
//	    page.Col("code", "ma", page.KindText),
//	    page.Col("created_at", "tao_luc", page.KindTime))
//
// Dựng danh sách bằng vòng lặp hay bằng một hàm khác thì nó TỪ CHỐI thay vì đoán — cùng kỷ
// luật với phần còn lại của bộ sinh: một hình dạng không mô tả được thì không được mô tả gần
// đúng, vì một hợp đồng sai mà trông đúng là thứ web dựng màn hình lên trên.
func (g *giaiMa) docPhanTrang(pkgDir, ten string) (phanTrang, error) {
	vs, goiPage, err := g.timBien(pkgDir, ten)
	if err != nil {
		return phanTrang{}, err
	}
	if len(vs.Values) != 1 {
		return phanTrang{}, fmt.Errorf("@page %s: khai báo không có đúng một giá trị", ten)
	}
	goi, err := g.callNewAllowlist(vs.Values[0], ten)
	if err != nil {
		return phanTrang{}, err
	}

	chieu, err := tenHang(goi.Args[0])
	if err != nil {
		return phanTrang{}, fmt.Errorf("@page %s: chiều mặc định: %w", ten, err)
	}
	var chieuRa string
	switch chieu {
	case "Asc":
		chieuRa = "asc"
	case "Desc":
		chieuRa = "desc"
	default:
		return phanTrang{}, fmt.Errorf("@page %s: chiều mặc định %q không phải page.Asc/page.Desc", ten, chieu)
	}

	var cot []string
	for i, a := range goi.Args[1:] {
		c, ok := a.(*ast.CallExpr)
		if !ok || !laGoi(c.Fun, "Col") || len(c.Args) < 1 {
			return phanTrang{}, fmt.Errorf(
				"@page %s: đối số cột thứ %d không phải page.Col(...) — apidoc chỉ đọc được danh "+
					"sách khai thẳng bằng page.Col, không đoán", ten, i+1)
		}
		s, err := chuoiLiteral(c.Args[0])
		if err != nil {
			return phanTrang{}, fmt.Errorf("@page %s: cột thứ %d: %w", ten, i+1, err)
		}
		cot = append(cot, s)
	}
	if len(cot) == 0 {
		return phanTrang{}, fmt.Errorf("@page %s: không có cột nào", ten)
	}

	// Giới hạn trang đọc từ chính hằng số của gói `page`, KHÔNG gõ lại ở đây — nếu không thì
	// tệp này lại thành bản chép thứ hai, đúng thứ nó sinh ra để xoá.
	mac, err := g.hangSoInt(goiPage, "DefaultLimit")
	if err != nil {
		return phanTrang{}, fmt.Errorf("@page %s: %w", ten, err)
	}
	max, err := g.hangSoInt(goiPage, "MaxLimit")
	if err != nil {
		return phanTrang{}, fmt.Errorf("@page %s: %w", ten, err)
	}

	return phanTrang{Cot: cot, ChieuMacD: chieuRa, LimitMacD: mac, LimitMax: max}, nil
}

// callNewAllowlist checks the value really is a page.NewAllowlist call with a default sort.
func (g *giaiMa) callNewAllowlist(e ast.Expr, ten string) (*ast.CallExpr, error) {
	goi, ok := e.(*ast.CallExpr)
	if !ok || !laGoi(goi.Fun, "NewAllowlist") {
		return nil, fmt.Errorf(
			"@page %s: không phải một lời gọi page.NewAllowlist — apidoc đọc chính biến này chứ "+
				"không chép danh sách cột, nên nó phải khai thẳng", ten)
	}
	if len(goi.Args) < 2 {
		return nil, fmt.Errorf("@page %s: NewAllowlist thiếu chiều mặc định hoặc cột mặc định", ten)
	}
	return goi, nil
}

// timBien finds a package-level var/const, and reports the directory of the `page` package that
// its file imports — needed to read DefaultLimit/MaxLimit from the same source.
func (g *giaiMa) timBien(pkgDir, ten string) (*ast.ValueSpec, string, error) {
	dir := pkgDir
	nho := ten
	if q, n, co := strings.Cut(ten, "."); co {
		p, err := g.nap(pkgDir)
		if err != nil {
			return nil, "", err
		}
		if p.xungDot[q] {
			return nil, "", fmt.Errorf("bí danh import %q trỏ vào hai gói khác nhau — không phân giải được %s", q, ten)
		}
		duong, co := p.imports[q]
		if !co {
			return nil, "", fmt.Errorf("không tìm thấy import %q để phân giải @page %s", q, ten)
		}
		dir = g.thuMucCuaImport(duong)
		if dir == "" {
			return nil, "", fmt.Errorf("@page %s nằm ngoài mọi module của kho (%s)", ten, duong)
		}
		nho = n
	}

	p, err := g.nap(dir)
	if err != nil {
		return nil, "", err
	}
	vs, ok := p.bien[nho]
	if !ok {
		return nil, "", fmt.Errorf("@page %s: không tìm thấy khai báo %q trong %s", ten, nho, dir)
	}

	duongPage, ok := p.imports["page"]
	if !ok {
		return nil, "", fmt.Errorf("@page %s: gói khai báo nó không import `page`", ten)
	}
	dirPage := g.thuMucCuaImport(duongPage)
	if dirPage == "" {
		return nil, "", fmt.Errorf("@page %s: gói `page` (%s) nằm ngoài mọi module của kho", ten, duongPage)
	}
	return vs, dirPage, nil
}

// hangSoInt reads one integer constant out of a package.
func (g *giaiMa) hangSoInt(pkgDir, ten string) (int, error) {
	p, err := g.nap(pkgDir)
	if err != nil {
		return 0, err
	}
	vs, ok := p.bien[ten]
	if !ok {
		return 0, fmt.Errorf("không tìm thấy hằng số %s trong %s", ten, pkgDir)
	}
	// `const ( DefaultLimit = 20; MaxLimit = 100 )` — một ValueSpec cho mỗi tên, nên Values[0]
	// là giá trị của chính nó.
	if len(vs.Values) != 1 {
		return 0, fmt.Errorf("hằng số %s không có đúng một giá trị", ten)
	}
	lit, ok := vs.Values[0].(*ast.BasicLit)
	if !ok {
		return 0, fmt.Errorf("hằng số %s không phải số nguyên viết thẳng", ten)
	}
	n, err := strconv.Atoi(lit.Value)
	if err != nil {
		return 0, fmt.Errorf("hằng số %s: %w", ten, err)
	}
	return n, nil
}

// laGoi reports whether `e` is `page.<ten>` — the selector form, qualified by the package.
func laGoi(e ast.Expr, ten string) bool {
	se, ok := e.(*ast.SelectorExpr)
	if !ok || se.Sel.Name != ten {
		return false
	}
	id, ok := se.X.(*ast.Ident)
	return ok && id.Name == "page"
}

func tenHang(e ast.Expr) (string, error) {
	se, ok := e.(*ast.SelectorExpr)
	if !ok {
		return "", fmt.Errorf("không phải một hằng số có tên gói")
	}
	return se.Sel.Name, nil
}

func chuoiLiteral(e ast.Expr) (string, error) {
	lit, ok := e.(*ast.BasicLit)
	if !ok {
		return "", fmt.Errorf("không phải chuỗi viết thẳng")
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", fmt.Errorf("không đọc được chuỗi: %w", err)
	}
	return s, nil
}

// thamSoPhanTrang turns the resolved contract into OpenAPI query parameters.
//
// `sort` mang enum, và enum ấy LÀ danh sách trắng của máy chủ — đó là toàn bộ điểm của tệp
// này. Máy khách sinh kiểu từ hợp đồng nhận được đúng hai giá trị hợp lệ, không phải một
// `string` rồi tự đoán.
func thamSoPhanTrang(pt phanTrang) []any {
	enum := make([]any, 0, len(pt.Cot))
	for _, c := range pt.Cot {
		enum = append(enum, c)
	}
	return []any{
		newOM().set("name", "limit").set("in", "query").set("required", false).
			set("description",
				"Số bản ghi mỗi trang. Vượt trần thì máy chủ TRẢ VỀ trần kèm con trỏ để đọc tiếp, "+
					"không báo lỗi.").
			set("schema", newOM().set("type", "integer").
				set("minimum", 1).set("maximum", pt.LimitMax).set("default", pt.LimitMacD)),

		newOM().set("name", "cursor").set("in", "query").set("required", false).
			set("description",
				"Con trỏ MỜ ĐỤC do máy chủ phát ra ở `next_cursor`, chuyền lại nguyên văn. Không "+
					"phải số thứ tự trang; không có `offset`, và không có tổng số bản ghi.").
			set("schema", newOM().set("type", "string")),

		newOM().set("name", "sort").set("in", "query").set("required", false).
			set("description", "Cột sắp xếp. Danh sách đóng — giá trị ngoài danh sách bị từ chối.").
			set("schema", newOM().set("type", "string").set("enum", enum).set("default", pt.Cot[0])),

		newOM().set("name", "order").set("in", "query").set("required", false).
			set("schema", newOM().set("type", "string").
				set("enum", []any{"asc", "desc"}).set("default", pt.ChieuMacD)),
	}
}
