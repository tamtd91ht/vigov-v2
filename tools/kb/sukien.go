package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// WHY THIS FILE EXISTS: `kb/30-indexes/event-flows.json` carried the label GENERATED and was
// never generated. `main.go` only ever wrote a placeholder for it, and `ensurePlaceholder`
// returns immediately once the file has real content — so even a hand-written answer would sit
// there untouched by `make kb` until the day it went stale.
//
// THAT MATTERS BECAUSE `kb/INDEX.yaml` ROUTES ONE QUESTION STRAIGHT AT IT: "đổi cái này thì ai
// vỡ". Three agent files and one skill tell a reader to check it before changing an event. The
// answer they got was an empty file — which reads as "nobody is listening", the most dangerous
// wrong answer this index can give, because it is also the green one.
//
// This is the same failure `data-ownership.json` had, recorded in main.go:75. It stayed
// invisible for the same reason: a placeholder does not look broken, it looks like an early
// repository.
//
// WHAT IS DERIVED FROM WHAT:
//
//	tên sự kiện, lược đồ, phiên bản   `.proto` — nguồn chuẩn của hợp đồng giữa service (luật 2 #7)
//	bên phát, bên nhận                mã Go — nơi tên sự kiện xuất hiện dưới dạng HẰNG CHUỖI
//
// Không vế nào được viết tay, và vế nào chưa suy được thì NÓI RA LÀ CHƯA CÓ. "Chưa có bên phát"
// và "không biết có bên phát hay không" là hai câu khác nhau; một tệp rỗng chỉ nói được câu thứ
// hai trong khi trông như câu thứ nhất.

// tenSuKien matches an event name as rule 2, invariant 4 requires it: `<miền>.<việc>.v<n>`,
// past tense, version in the name.
var tenSuKien = regexp.MustCompile(`^[a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)+\.v[0-9]+$`)

var (
	dauPackageProto = regexp.MustCompile(`^\s*package\s+([a-z0-9_.]+)\s*;`)
	dauMessage      = regexp.MustCompile(`^message\s+([A-Za-z0-9_]+)\s*\{`)
)

// mocMa is one place in the Go source where an event name appears.
type mocMa struct {
	Service string `json:"service"`
	At      string `json:"at"` // đường dẫn tương đối gốc kho + số dòng
}

type luongSuKien struct {
	Ten     string `json:"name"`
	Owner   string `json:"owner_service"`
	Version string `json:"version"`
	// Schema is `proto/…/events.proto#Message` — file and message, so a reader lands on the
	// shape rather than on a directory.
	Schema    string   `json:"schema"`
	BenPhat   []mocMa  `json:"publishers"`
	BenNhan   []mocMa  `json:"consumers"`
	ChuaRo    []mocMa  `json:"unclassified_mentions"`
	TinhTrang []string `json:"state"`
}

// quetSuKien builds the whole index: the contracts from proto/, the two sides from the Go code.
func quetSuKien(root string) ([]luongSuKien, []string, error) {
	sk, canhBao, err := suKienTuProto(root)
	if err != nil {
		return nil, nil, err
	}
	if len(sk) == 0 {
		return nil, canhBao, nil
	}

	ten := map[string]bool{}
	for _, s := range sk {
		ten[s.Ten] = true
	}
	phat, nhan, chuaRo, err := benTuMaGo(root, ten)
	if err != nil {
		return nil, nil, err
	}

	for i := range sk {
		// `[]` CHỨ KHÔNG PHẢI `null`. Một máy đọc phải xử lý hai hình dạng thì nó xử lý sai một
		// trong hai, và "null" là đúng cái nghĩa mơ hồ tệp này sinh ra để xoá.
		sk[i].BenPhat = rongNeuNil(phat[sk[i].Ten])
		sk[i].BenNhan = rongNeuNil(nhan[sk[i].Ten])
		sk[i].ChuaRo = rongNeuNil(chuaRo[sk[i].Ten])
		sk[i].TinhTrang = []string{}
		// KHÔNG ĐỂ TRỐNG NHƯ THỂ KHÔNG BIẾT. Một danh sách rỗng đọc được theo hai nghĩa — "chưa
		// ai" và "chưa ai đi tìm" — và người đọc tệp này đang hỏi "đổi cái này thì ai vỡ".
		if len(sk[i].BenPhat) == 0 {
			sk[i].TinhTrang = append(sk[i].TinhTrang,
				"CHƯA CÓ BÊN PHÁT trong mã Go: không tệp .go nào (ngoài test và mã sinh) nhắc tên "+
					"sự kiện này ở dạng hằng chuỗi. Hợp đồng đã tồn tại — "+sk[i].Schema+" — nhưng "+
					"chưa ai phát.")
		}
		if len(sk[i].BenNhan) == 0 {
			sk[i].TinhTrang = append(sk[i].TinhTrang,
				"CHƯA CÓ BÊN NHẬN trong mã Go: chưa dịch vụ nào đăng ký nghe. Đổi lược đồ hôm nay "+
					"chưa làm vỡ ai; ngày có bên nhận đầu tiên thì đổi tại chỗ là hết đường và "+
					"phải phát `.v2` song song (luật 2 bất biến 4).")
		}
		if len(sk[i].ChuaRo) > 0 {
			canhBao = append(canhBao, fmt.Sprintf(
				"%s: tên sự kiện xuất hiện ở %d chỗ trong mã Go mà không nhận ra vai trò — xem "+
					"`unclassified_mentions`. Bộ sinh KHÔNG đoán: hoặc chỗ ấy là phát/nhận viết "+
					"theo hình dạng bộ sinh chưa biết (sửa tools/kb/sukien.go), hoặc nó chỉ nhắc "+
					"tên và không phải cả hai.", sk[i].Ten, len(sk[i].ChuaRo)))
		}
	}
	return sk, canhBao, nil
}

func rongNeuNil(m []mocMa) []mocMa {
	if m == nil {
		return []mocMa{}
	}
	return m
}

func rongNeuNilChuoi(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// suKienTuProto reads every .proto under the repository and returns one row per event contract.
//
// TÊN SỰ KIỆN ĐƯỢC ĐỌC, KHÔNG ĐƯỢC SUY RA TỪ TÊN MESSAGE. `PetitionStatusChanged` trong gói
// `vigov.petitions.v1` mang tên `petitions.status_changed.v1` — không phải
// `petitions.petition_status_changed.v1` — nên mọi phép đổi tên máy móc đều sai ở đúng sự kiện
// đầu tiên của kho. Cái đọc được là dòng tên đứng RIÊNG MỘT MÌNH trong khối chú thích ngay trên
// `message`, tức chỗ `.proto` thật sự khai nó.
//
// Bù lại, hai phần của tên VẪN được kiểm chéo với gói proto: `<miền>` phải là đoạn giữa của
// `package vigov.<miền>.<ver>` và `.v<n>` phải khớp `<ver>`. Đổi tên message mà quên sửa dòng ấy
// thì bộ sinh kêu, chứ không công bố một cái tên không ai phát.
func suKienTuProto(root string) ([]luongSuKien, []string, error) {
	var ra []luongSuKien
	var canhBao []string

	thuMuc := filepath.Join(root, "proto")
	if _, err := os.Stat(thuMuc); err != nil {
		return nil, nil, nil // kho chưa có hợp đồng nào — không phải lỗi
	}

	var tep []string
	err := filepath.WalkDir(thuMuc, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".proto") {
			tep = append(tep, p)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(tep)

	for _, t := range tep {
		b, err := os.ReadFile(t)
		if err != nil {
			return nil, nil, err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(t, root+string(filepath.Separator)))
		dong := strings.Split(string(b), "\n")

		pkg := ""
		var chuThich []string
		sau := 0               // độ sâu ngoặc nhọn: chỉ message ở mức 0 mới là message cấp một
		khongTen := []string{} // message cấp một không mang tên sự kiện
		coSuKien := false
		for _, d := range dong {
			cat := strings.TrimSpace(d)
			if strings.HasPrefix(cat, "//") {
				chuThich = append(chuThich, strings.TrimSpace(strings.TrimPrefix(cat, "//")))
				continue
			}

			if m := dauPackageProto.FindStringSubmatch(cat); m != nil && pkg == "" {
				pkg = m[1]
			}
			if m := dauMessage.FindStringSubmatch(cat); m != nil && sau == 0 {
				ten := ""
				for _, c := range chuThich {
					if tenSuKien.MatchString(c) {
						ten = c
					}
				}
				if ten == "" {
					khongTen = append(khongTen, m[1])
				} else {
					coSuKien = true
					owner, ver := chuXuatXu(pkg)
					if mien := strings.SplitN(ten, ".", 2)[0]; owner != "" && mien != owner {
						canhBao = append(canhBao, fmt.Sprintf(
							"%s: sự kiện %q không cùng miền với gói proto %q — tên sự kiện phải "+
								"bắt đầu bằng dịch vụ sở hữu nó", rel, ten, pkg))
					}
					if ver != "" && !strings.HasSuffix(ten, "."+ver) {
						canhBao = append(canhBao, fmt.Sprintf(
							"%s: sự kiện %q không cùng phiên bản với gói proto %q", rel, ten, pkg))
					}
					ra = append(ra, luongSuKien{
						Ten: ten, Owner: owner, Version: ver,
						Schema: rel + "#" + m[1],
					})
				}
			}
			sau += strings.Count(cat, "{") - strings.Count(cat, "}")
			// MỌI DÒNG KHÔNG PHẢI CHÚ THÍCH ĐỀU CẮT KHỐI — kể cả một dòng trống.
			//
			// Khối chú thích của một message là khối DÍNH LIỀN ngay trên nó, đúng kỷ luật `apidoc`
			// áp cho chú thích route. Không cắt ở đây thì một message MƯỢN được cái tên viết cho
			// thứ khác ở trên nó, và tệp sinh ra công bố một sự kiện không ai khai dưới cái tên ấy.
			chuThich = nil
		}

		// Một message cấp một KHÔNG mang tên sự kiện, trong một tệp CÓ sự kiện, và không được
		// dùng làm kiểu trường ở đâu cả: đó là một hợp đồng không ai phát được và không ai gọi
		// tên được. Báo, chứ không lặng lẽ bỏ qua — bỏ qua là đúng cách tệp này rỗng suốt.
		if coSuKien {
			for _, m := range khongTen {
				if !dungLamTruong(dong, m) {
					canhBao = append(canhBao, fmt.Sprintf(
						"%s: message %q không mang tên sự kiện trong chú thích và cũng không được "+
							"dùng làm kiểu trường — không vào được event-flows", rel, m))
				}
			}
		}
	}
	sort.Slice(ra, func(i, j int) bool { return ra[i].Ten < ra[j].Ten })
	return ra, canhBao, nil
}

// chuXuatXu splits `vigov.petitions.v1` into the owning service and the version.
func chuXuatXu(pkg string) (string, string) {
	phan := strings.Split(pkg, ".")
	if len(phan) < 3 {
		return "", ""
	}
	return phan[len(phan)-2], phan[len(phan)-1]
}

// dungLamTruong reports whether a message name is used as a field type somewhere in the file.
func dungLamTruong(dong []string, ten string) bool {
	re := regexp.MustCompile(`(^|\s)` + regexp.QuoteMeta(ten) + `\s+[a-z_]`)
	for _, d := range dong {
		cat := strings.TrimSpace(d)
		if strings.HasPrefix(cat, "//") || dauMessage.MatchString(cat) {
			continue
		}
		if re.MatchString(cat) {
			return true
		}
	}
	return false
}

// benPhat / benNhan are the call shapes that make a service visible as one side of an event.
//
// ĐỌC MÃ THẬT, KHÔNG THÊM MỘT DẤU KHAI BÁO MỚI. Một `// @publishes:` viết tay là một bản chép
// thứ hai của thứ câu lệnh publish đã nói, và bản trôi là bản người đọc tin (luật 9 cấm #2).
//
// CHƯA KHỚP HÌNH DẠNG NÀO THÌ VÀO `unclassified_mentions` VÀ `_warnings`, không bị bỏ đi. Đó là
// điểm khác nhau giữa bộ sinh này và tệp rỗng nó thay thế: một bên phát viết theo hình dạng lạ
// vẫn hiện ra, chỉ là chưa được xếp vai — chứ không biến mất trong im lặng.
var (
	benPhat = map[string]bool{"Publish": true, "PublishContext": true, "Phat": true, "PhatSuKien": true}
	benNhan = map[string]bool{"Subscribe": true, "Consume": true, "Nghe": true, "DangKyNghe": true}
)

// benTuMaGo finds every mention of an event name as a Go string constant, and classifies it.
//
// QUA CẢ MỘT HẰNG SỐ CÓ TÊN, không chỉ chuỗi viết thẳng. `service-comms` khai
// `const TenSuKienPhieuDoiTrangThai = "petitions.status_changed.v1"` rồi so tên phong bì với
// hằng ấy — đó là cách ĐÚNG để viết (một chỗ duy nhất giữ tên), nên một bộ sinh chỉ nhìn chuỗi
// viết thẳng sẽ đọc chính đoạn mã tốt nhất thành "không rõ vai".
func benTuMaGo(root string, ten map[string]bool) (phat, nhan, chuaRo map[string][]mocMa, err error) {
	phat, nhan, chuaRo = map[string][]mocMa{}, map[string][]mocMa{}, map[string][]mocMa{}

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			// `gen` là mã sinh từ .proto: nó chỉ mang tên sự kiện trong chú thích chép lại từ
			// `.proto`, nên nó không nói được ai phát ai nghe.
			case ".git", "node_modules", "vendor", "gen", "kb", ".claude":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil // một tệp không biên dịch được là việc của trình biên dịch
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		dichVu := strings.SplitN(rel, "/", 2)[0]

		// LƯỢT MỘT — hằng số cấp gói mang tên một sự kiện. Nó CHƯA phải một vai: khai một cái
		// tên không phát và không nghe gì cả. Vai đến từ chỗ DÙNG nó, ở lượt hai.
		biDanh := map[string]string{}           // tên hằng -> tên sự kiện
		biDanhLit := map[string]*ast.BasicLit{} // và chỗ khai nó, để báo khi không ai dùng
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
				continue
			}
			for _, sp := range gd.Specs {
				vs, ok := sp.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, nm := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					if lit, s, ok := chuoiSuKien(vs.Values[i], ten); ok {
						biDanh[nm.Name] = s
						biDanhLit[nm.Name] = lit
					}
				}
			}
		}

		daXep := map[ast.Node]bool{}
		daDung := map[string]bool{} // bí danh đã được xếp vai ở ít nhất một chỗ dùng
		ghi := func(vao map[string][]mocMa, n ast.Node, s string) {
			daXep[n] = true
			if id, ok := n.(*ast.Ident); ok {
				daDung[id.Name] = true
			}
			vao[s] = append(vao[s], mocMa{
				Service: strings.TrimPrefix(dichVu, "service-"),
				At:      fmt.Sprintf("%s:%d", rel, fset.Position(n.Pos()).Line),
			})
		}
		// tenCua đọc một biểu thức thành tên sự kiện — chuỗi viết thẳng hoặc hằng số ở trên.
		tenCua := func(e ast.Expr) (ast.Node, string, bool) {
			if lit, s, ok := chuoiSuKien(e, ten); ok {
				return lit, s, true
			}
			id, ok := e.(*ast.Ident)
			if !ok {
				return nil, "", false
			}
			s, ok := biDanh[id.Name]
			return id, s, ok
		}

		// LƯỢT HAI — xếp vai theo hình dạng câu lệnh.
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CompositeLit:
				// events.Envelope{Name: "petitions.status_changed.v1", …} — ĐẶT tên vào phong bì
				// là phát.
				sel, ok := x.Type.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Envelope" {
					return true
				}
				for _, e := range x.Elts {
					kv, ok := e.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					k, ok := kv.Key.(*ast.Ident)
					if !ok || k.Name != "Name" {
						continue
					}
					if nd, s, ok := tenCua(kv.Value); ok {
						ghi(phat, nd, s)
					}
				}
			case *ast.BinaryExpr:
				// `e.Name != TenSuKien…` — SO tên của một phong bì đang đến là nghe. Đối xứng với
				// nhánh trên: đặt tên vào là phát, so tên ra là nhận.
				if x.Op != token.EQL && x.Op != token.NEQ {
					return true
				}
				coTruongName := func(e ast.Expr) bool {
					sel, ok := e.(*ast.SelectorExpr)
					return ok && sel.Sel.Name == "Name"
				}
				if coTruongName(x.X) {
					if nd, s, ok := tenCua(x.Y); ok {
						ghi(nhan, nd, s)
					}
				}
				if coTruongName(x.Y) {
					if nd, s, ok := tenCua(x.X); ok {
						ghi(nhan, nd, s)
					}
				}
			case *ast.CallExpr:
				var ham string
				switch fn := x.Fun.(type) {
				case *ast.Ident:
					ham = fn.Name
				case *ast.SelectorExpr:
					ham = fn.Sel.Name
				}
				vao := map[string][]mocMa(nil)
				switch {
				case benPhat[ham]:
					vao = phat
				case benNhan[ham]:
					vao = nhan
				default:
					return true
				}
				for _, a := range x.Args {
					if nd, s, ok := tenCua(a); ok {
						ghi(vao, nd, s)
					}
				}
			case *ast.CaseClause:
				// `case "petitions.status_changed.v1":` — một bộ định tuyến thông điệp.
				for _, e := range x.List {
					if nd, s, ok := tenCua(e); ok {
						ghi(nhan, nd, s)
					}
				}
			}
			return true
		})

		// LƯỢT BA — mọi hằng chuỗi mang tên sự kiện mà chưa được xếp vai. Chỗ khai một hằng số
		// ĐÃ được dùng ở một vai thì không báo lại: nó là cùng một sự thật, kể hai lần.
		ast.Inspect(f, func(n ast.Node) bool {
			lit, s, ok := chuoiSuKien(n, ten)
			if !ok || daXep[lit] {
				return true
			}
			for nm, l := range biDanhLit {
				if l == lit && daDung[nm] {
					return true
				}
			}
			ghi(chuaRo, lit, s)
			return true
		})
		return nil
	})
	return phat, nhan, chuaRo, err
}

// chuoiSuKien reports whether a node is a string literal naming a known event.
func chuoiSuKien(n ast.Node, ten map[string]bool) (*ast.BasicLit, string, bool) {
	lit, ok := n.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return nil, "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil || !ten[s] {
		return nil, "", false
	}
	return lit, s, true
}
