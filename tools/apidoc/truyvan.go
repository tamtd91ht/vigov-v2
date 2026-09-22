package main

import (
	"go/ast"
	"sort"
)

// Reading the QUERY PARAMETERS a handler really accepts, out of the handler itself.
//
// VẤN ĐỀ TỆP NÀY SINH RA ĐỂ GIẢI. `kb/20-contracts/openapi.json` chỉ khai `parameters` cho
// tuyến có `@page` và cho tham số nằm trong đường dẫn. Mọi tham số truy vấn khác biến mất khỏi
// hợp đồng: `/api/v1/investment-projects` đọc `year` (bắt buộc) và `category`,
// `/api/v1/public-holidays` và `/api/v1/swap-working-days` đọc `year` (bắt buộc) — không cái
// nào có mặt trong hợp đồng.
//
// Hệ quả đo được: `finance_get_investment_projects["truyVan"]` sinh ra là `{}` rỗng, nên `tsc`
// không canh được tên tham số ở web. Gõ `nam` thay `year` thì không gì đỏ — màn hình chỉ lặng
// lẽ nhận 400, và cán bộ chỉ thấy "không tải được danh sách".
//
// VÌ SAO SUY TỪ MÃ CHỨ KHÔNG THÊM MỘT CHÚ THÍCH `@query`. Quyền và chế độ chống lặp đã theo lối
// ấy: chúng được ĐỌC từ `authz.*` / `idem.*` chứ không được khai lại bằng tay, vì một bản chép
// viết tay là một nguồn thứ hai cho một sự thật — và bản trôi là bản người đọc tin (luật 9 cấm
// #2). Tham số truy vấn cũng vậy: `r.URL.Query().Get("year")` ĐÃ nằm trong mã, nên hợp đồng đọc
// chính chỗ ấy. Đổi tên tham số ở handler là hợp đồng đổi theo trong cùng một lượt `make kb`;
// không có cách nào để hai bên lệch nhau.
//
// NHỮNG GÌ NÓ CỐ Ý KHÔNG SUY:
//
//   - KIỂU. Mọi giá trị trên query string là chuỗi, và đó là sự thật duy nhất đọc được mà không
//     phải đoán. Suy ra `integer` vì thấy `strconv.Atoi` là suy ra một khoảng và một định dạng
//     mà handler chưa chắc đã cam kết — một hợp đồng sai mà trông đúng là thứ web dựng màn hình
//     lên trên.
//   - THAM SỐ DỰNG LÚC CHẠY. `q.Get(ten)` với `ten` là biến thì không đọc được tên, nên nó
//     không vào hợp đồng — giống hệt cách `mauRoute` từ chối một đường dẫn dựng lúc chạy.

// thamSoTruyVan is one query parameter read by a route's handler.
type thamSoTruyVan struct {
	Ten     string
	BatBuoc bool
}

// sauToiDa bounds how far the walk follows calls out of the handler.
//
// MỘT GIỚI HẠN CHỨ KHÔNG PHẢI MỘT CON SỐ MAY MẮN: handler thật gọi sâu nhất hôm nay là một nấc
// (`DanhSachNgayNghiLe` -> `docNamTruyVan`), và đệ quy không giới hạn trên đồ thị gọi của cả gói
// là cách rẻ nhất để bộ sinh treo vào một vòng gọi lẫn nhau.
const sauToiDa = 4

// tenHandler pulls the handler names out of one route registration statement.
//
// Hai hình dạng đang dùng trong kho, và không có hình thứ ba: `http.HandlerFunc(h.DanhSachDuAn)`
// bọc trong middleware, và `mux.HandleFunc("GET /x", h.Foo)`. Trả về TÊN, không phải kiểu: hàm
// được tra lại trong chính gói khai route, đúng cách `@page` tra biến allowlist.
func tenHandler(call *ast.CallExpr) []string {
	var ra []string
	them := func(e ast.Expr) {
		switch x := e.(type) {
		case *ast.Ident:
			ra = append(ra, x.Name)
		case *ast.SelectorExpr:
			// `h.DanhSachDuAn` — phương thức trên receiver của Handler.
			ra = append(ra, x.Sel.Name)
		}
	}
	ast.Inspect(call, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok || len(c.Args) != 1 {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "HandlerFunc" {
			return true
		}
		them(c.Args[0])
		return true
	})
	// mux.HandleFunc("METHOD /path", h.Foo) — handler ở đối số thứ hai, không bọc HandlerFunc.
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "HandleFunc" && len(call.Args) == 2 {
		them(call.Args[1])
	}
	return ra
}

// thamSoTruyVanCua resolves the query parameters of one route, starting from its handlers.
func (g *giaiMa) thamSoTruyVanCua(pkgDir string, handlers []string) ([]thamSoTruyVan, error) {
	if len(handlers) == 0 {
		return nil, nil
	}
	p, err := g.nap(pkgDir)
	if err != nil {
		return nil, err
	}

	co := map[string]bool{}      // tham số đọc được
	batBuoc := map[string]bool{} // và nó có bắt buộc không
	daXem := map[string]bool{}

	var di func(ten string, sau int)
	di = func(ten string, sau int) {
		if sau > sauToiDa || daXem[ten] {
			return
		}
		daXem[ten] = true
		for _, fd := range p.ham[ten] {
			if fd.Body == nil {
				continue
			}
			for t, bb := range docThanHam(fd) {
				co[t] = true
				batBuoc[t] = batBuoc[t] || bb
			}
			for _, goi := range goiCungGoi(fd.Body, p.imports) {
				di(goi, sau+1)
			}
		}
	}
	for _, h := range handlers {
		di(h, 0)
	}

	ten := make([]string, 0, len(co))
	for t := range co {
		ten = append(ten, t)
	}
	sort.Strings(ten) // thứ tự cố định: một tệp sinh đổi mỗi lượt chạy là một diff không ai đọc
	ra := make([]thamSoTruyVan, 0, len(ten))
	for _, t := range ten {
		ra = append(ra, thamSoTruyVan{Ten: t, BatBuoc: batBuoc[t]})
	}
	return ra, nil
}

// docThanHam reads one function body: which query parameters it touches, and which of them it
// refuses the request over.
//
// BẮT BUỘC ĐƯỢC SUY, KHÔNG ĐƯỢC ĐOÁN. Một tham số là bắt buộc khi chính nó — hoặc một biến lấy
// giá trị từ nó — xuất hiện trong điều kiện của một nhánh `if` trả 400. Đó đúng là hình dạng của
// hai handler đang có:
//
//	nam, err := strconv.Atoi(thamSo.Get("year"))     // `nam` và `err` nhiễm từ "year"
//	if err != nil || nam < 2000 || nam > 2100 { ... http.StatusBadRequest ... }
//
//	gt := r.URL.Query()["year"]                      // `gt` nhiễm từ "year"
//	if len(gt) != 1 { ... http.StatusBadRequest ... }
//
// Còn `thamSo.Get("category")` chỉ chảy thẳng vào bộ lọc của store, không nhánh 400 nào nhắc
// tới nó, nên nó ra `required: false` — đúng như handler cư xử.
//
// KHI KHÔNG SUY ĐƯỢC THÌ RA `false`, và đó là phía an toàn: web nhận một tham số TÙY CHỌN mà
// đáng lẽ bắt buộc thì `tsc` vẫn canh đúng TÊN — thứ đang thiếu hoàn toàn hôm nay. Ngược lại,
// bịa ra `required: true` cho một tham số thật sự tùy chọn là ép mọi máy khách gửi một thứ máy
// chủ không đòi.
func docThanHam(fd *ast.FuncDecl) map[string]bool {
	than := fd.Body
	bienQuery := map[string]bool{} // biến giữ `r.URL.Query()`

	// THAM SỐ KIỂU `url.Values` CŨNG LÀ MỘT BIẾN QUERY, và thiếu dòng này là chỗ phép đi bộ MẤT
	// DẤU ở biên hàm.
	//
	// Đo được 23/09/2026: `GET /api/v1/incoming-documents` và `/outgoing-documents` đọc năm tham
	// số (`year · status · document_type · holding_unit · q`) trong một HÀM PHỤ cùng gói, còn
	// `GET /api/v1/investment-projects` đọc `year`/`category` NGAY TRONG handler. Phép đi bộ theo
	// được lời gọi sang hàm phụ (`goiCungGoi` nhận cả `*ast.Ident`), nhưng tới nơi thì `q` chỉ là
	// một tham số — chưa từng có phép gán nào từ `r.URL.Query()` — nên `bienQuery` rỗng và cả năm
	// tham số biến mất khỏi hợp đồng TRONG IM LẶNG. Hai tuyến ấy sinh ra `truyVan: {}`, và web
	// phải chép tên bộ lọc từ MÃ NGUỒN, tức `tsc` không còn canh được tên nào.
	//
	// SUY THEO KIỂU, KHÔNG THEO ĐỐI SỐ. Ghép đối số ở chỗ gọi với tham số ở chỗ khai là một phép
	// phân tích luồng dữ liệu thật, và nó hỏng ngay khi có hai chỗ gọi khác nhau. Một hàm nhận
	// `url.Values` thì theo định nghĩa đang nhận tham số truy vấn — không cần biết ai gọi nó.
	//
	// `map[string][]string` CŨNG NHẬN vì `url.Values` chính là kiểu ấy, và một hàm phụ khai kiểu
	// nền thay vì tên kiểu vẫn đang làm đúng một việc.
	for _, ts := range thamSoKieuQuery(fd) {
		bienQuery[ts] = true
	}

	nhiem := map[string]map[string]bool{} // tham số -> tên biến mang giá trị của nó
	co := map[string]bool{}               // tham số đọc được
	batBuoc := map[string]bool{}

	ghiNhiem := func(ts string, lhs []ast.Expr) {
		if nhiem[ts] == nil {
			nhiem[ts] = map[string]bool{}
		}
		for _, l := range lhs {
			if id, ok := l.(*ast.Ident); ok && id.Name != "_" {
				nhiem[ts][id.Name] = true
			}
		}
	}

	// MỘT LƯỢT DUY NHẤT, THEO THỨ TỰ NGUỒN. ast.Inspect đi đúng thứ tự viết, nên khi tới nhánh
	// `if` thì phép gán ở trên nó đã được xử lý — không cần điểm bất động, và một phép gán nằm
	// SAU nhánh kiểm tra thì đúng là không nhiễm vào nhánh ấy.
	ast.Inspect(than, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			if len(s.Rhs) == 1 && laGoiQuery(s.Rhs[0]) {
				for _, l := range s.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						bienQuery[id.Name] = true
					}
				}
				return true
			}
			for ts := range phuThuocVao(s.Rhs, bienQuery, nhiem) {
				co[ts] = true
				ghiNhiem(ts, s.Lhs)
			}
		case *ast.IfStmt:
			if !coBadRequest(s.Body) {
				return true
			}
			for ts := range phuThuocVao([]ast.Expr{s.Cond}, bienQuery, nhiem) {
				co[ts] = true
				batBuoc[ts] = true
			}
		}
		// Mọi lần đọc khác — tham số chảy thẳng vào một lời gọi, không qua biến nào.
		if ts, ok := docThamSo(n, bienQuery); ok {
			co[ts] = true
		}
		return true
	})

	ra := make(map[string]bool, len(co))
	for t := range co {
		ra[t] = batBuoc[t]
	}
	return ra
}

// phuThuocVao returns the query parameters an expression tree reads, directly or through a
// variable already tainted by one.
func phuThuocVao(es []ast.Expr, bienQuery map[string]bool, nhiem map[string]map[string]bool) map[string]bool {
	ra := map[string]bool{}
	for _, e := range es {
		if e == nil {
			continue
		}
		ast.Inspect(e, func(n ast.Node) bool {
			if ts, ok := docThamSo(n, bienQuery); ok {
				ra[ts] = true
				return true
			}
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			for ts, bien := range nhiem {
				if bien[id.Name] {
					ra[ts] = true
				}
			}
			return true
		})
	}
	return ra
}

// docThamSo recognises one read of a named query parameter: `q.Get("x")` or `q["x"]`.
func docThamSo(n ast.Node, bienQuery map[string]bool) (string, bool) {
	laNguon := func(e ast.Expr) bool {
		if laGoiQuery(e) {
			return true
		}
		id, ok := e.(*ast.Ident)
		return ok && bienQuery[id.Name]
	}
	switch x := n.(type) {
	case *ast.CallExpr:
		sel, ok := x.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Get" || len(x.Args) != 1 || !laNguon(sel.X) {
			return "", false
		}
		return chuoiLit(x.Args[0])
	case *ast.IndexExpr:
		if !laNguon(x.X) {
			return "", false
		}
		return chuoiLit(x.Index)
	}
	return "", false
}

// thamSoKieuQuery names the parameters of one function whose type is a query-values map.
//
// HAI CÁCH VIẾT, cả hai là cùng một kiểu: `url.Values` (tên kiểu, cách viết đúng) và
// `map[string][]string` (kiểu nền — `url.Values` được khai đúng là nó). Không nhận bất kỳ kiểu
// nào khác: một hàm nhận `map[string]string` đang làm việc khác, và đoán rộng ra là đưa vào hợp
// đồng những tên không phải tham số truy vấn.
func thamSoKieuQuery(fd *ast.FuncDecl) []string {
	if fd.Type == nil || fd.Type.Params == nil {
		return nil
	}
	var ra []string
	for _, f := range fd.Type.Params.List {
		if !laKieuQuery(f.Type) {
			continue
		}
		for _, n := range f.Names {
			if n.Name != "_" {
				ra = append(ra, n.Name)
			}
		}
	}
	return ra
}

func laKieuQuery(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.SelectorExpr: // url.Values
		id, ok := t.X.(*ast.Ident)
		return ok && id.Name == "url" && t.Sel.Name == "Values"
	case *ast.MapType: // map[string][]string
		k, ok := t.Key.(*ast.Ident)
		if !ok || k.Name != "string" {
			return false
		}
		arr, ok := t.Value.(*ast.ArrayType)
		if !ok || arr.Len != nil {
			return false
		}
		v, ok := arr.Elt.(*ast.Ident)
		return ok && v.Name == "string"
	}
	return false
}

// laGoiQuery reports whether an expression is `<req>.URL.Query()`.
func laGoiQuery(e ast.Expr) bool {
	c, ok := e.(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return false
	}
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Query" {
		return false
	}
	url, ok := sel.X.(*ast.SelectorExpr)
	return ok && url.Sel.Name == "URL"
}

// coBadRequest reports whether a branch answers 400.
//
// Cả hai cách viết đều tính: `http.StatusBadRequest` là cách kho đang viết, còn `400` viết
// thẳng là cách một tệp khác có thể viết — nhận cả hai để suy luận "bắt buộc" không lặng lẽ
// hỏng vì một cách viết khác.
func coBadRequest(than ast.Node) bool {
	thay := false
	ast.Inspect(than, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if x.Sel.Name == "StatusBadRequest" {
				thay = true
			}
		case *ast.BasicLit:
			if x.Value == "400" {
				thay = true
			}
		}
		return !thay
	})
	return thay
}

// goiCungGoi lists the functions of the SAME package a body calls.
//
// Chỉ cùng gói, có chủ ý: `docNamTruyVan` nằm cạnh handler dùng nó, còn `page.Parse` thì không —
// và một tham số đọc bên trong `core/page` đã được `@page` mô tả rồi. Bỏ qua tên là bí danh
// import để `strconv.Atoi` không bị hiểu thành một hàm trong gói này.
func goiCungGoi(than *ast.BlockStmt, imports map[string]string) []string {
	var ra []string
	ast.Inspect(than, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch f := c.Fun.(type) {
		case *ast.Ident:
			ra = append(ra, f.Name)
		case *ast.SelectorExpr:
			// `h.docGiDo(...)` — phương thức trên receiver. `strconv.Atoi(...)` bị loại vì
			// `strconv` là một import.
			if id, ok := f.X.(*ast.Ident); ok && imports[id.Name] == "" {
				ra = append(ra, f.Sel.Name)
			}
		}
		return true
	})
	return ra
}

// thamSoTruyVanJSON turns the resolved parameters into OpenAPI query parameters.
func thamSoTruyVanJSON(ts []thamSoTruyVan) []any {
	out := make([]any, 0, len(ts))
	for _, t := range ts {
		out = append(out, newOM().
			set("name", t.Ten).
			set("in", "query").
			set("required", t.BatBuoc).
			// KIỂU CHUỖI VÀ KHÔNG SUY GÌ THÊM — xem đầu tệp. Trên query string mọi giá trị là
			// chuỗi; bất kỳ kiểu hẹp hơn nào cũng là một cam kết handler chưa đưa ra.
			set("schema", newOM().set("type", "string")))
	}
	return out
}
