package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"
)

// OPEN QUESTION #18, DECIDED 2026-09-22: a staff session lives ONE WORKING DAY (~8–12 hours), and
// the commune may NOT configure it.
//
// WHY A FIGURE NEEDS A TEST AT ALL. It is one constant, and changing it is one character — the
// cheapest edit in this repository and the one with no symptom. A session stretched to 30 days
// works perfectly: everybody stays signed in, every screen is faster, nobody complains. What breaks
// is the AUDIT TRAIL, silently: the computer at the one-stop-shop counter is SHARED, so the person
// sitting down in the afternoon acts under the name of the person who sat there in the morning, and
// every entry written in between states — with the authority of a government record — that somebody
// did something they did not do (rule 6, invariant 2).
//
// SO THE ASSERTION IS A RANGE, NOT AN EQUALITY. The customer gave a range; pinning 12h exactly
// would make a move to 8h fail a test for no reason, and a test that fails for no reason is a test
// somebody deletes.
func TestThoiHanPhienLaMotNgayLamViec(t *testing.T) {
	const min, max = 8 * time.Hour, 12 * time.Hour
	if ThoiHanPhien < min || ThoiHanPhien > max {
		t.Fatalf("ThoiHanPhien = %v, ngoài khoảng một ngày làm việc %v–%v mà khách đã chốt "+
			"(câu hỏi mở #18, 22/09/2026). Đây là ĐÁNH ĐỔI AN TOÀN trên máy DÙNG CHUNG ở bộ phận "+
			"một cửa, không phải một tham số hiệu năng — đổi nó là mở lại câu hỏi với khách",
			ThoiHanPhien, min, max)
	}
}

// AND IT IS A COMPILE-TIME CONSTANT, NOT A VALUE READ FROM ANYWHERE.
//
// #18 says in so many words that the commune must NOT be able to set this ("KHÔNG để xã tự cấu
// hình"), which is the opposite of rule 1, invariant 10's usual direction — and that is precisely
// why somebody will one day "fix" it by moving the figure into per-commune configuration, in good
// faith, to make a commune happy. A `var` fed from a config struct or from the tenant row would
// still satisfy the range above on the day it was written.
//
// The parser is what makes this checkable: `const` cannot be assigned at runtime, so if this
// declaration is still a const with a literal duration, there is no path by which a commune's row
// reaches it.
func TestThoiHanPhienLaHangSoKhongPhaiCauHinhTheoXa(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "phien.go", nil, 0)
	if err != nil {
		t.Fatalf("đọc phien.go: %v", err)
	}

	var thay bool
	ast.Inspect(f, func(n ast.Node) bool {
		gd, ok := n.(*ast.GenDecl)
		if !ok {
			return true
		}
		for _, s := range gd.Specs {
			vs, ok := s.(*ast.ValueSpec)
			if !ok || len(vs.Names) == 0 || vs.Names[0].Name != "ThoiHanPhien" {
				continue
			}
			thay = true
			if gd.Tok != token.CONST {
				t.Fatalf("ThoiHanPhien khai bằng %v chứ không phải const — một `var` là một giá trị "+
					"có thể gán lúc chạy, tức là cấu hình theo xã trá hình (#18 cấm đích danh)", gd.Tok)
			}
			if len(vs.Values) != 1 {
				t.Fatalf("ThoiHanPhien có %d giá trị khởi tạo, muốn đúng 1", len(vs.Values))
			}
			// A binary expression of two literals (`12 * time.Hour`) is what it is today. Anything
			// that reaches for an identifier outside `time` would be reading a value from somewhere.
			var xau strings.Builder
			ast.Inspect(vs.Values[0], func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					xau.WriteString(id.Name + " ")
				}
				return true
			})
			for _, ten := range strings.Fields(xau.String()) {
				if ten != "time" && ten != "Hour" && ten != "Minute" {
					t.Fatalf("ThoiHanPhien được tính từ %q — chỉ được là một hằng thời lượng viết thẳng, "+
						"không đọc từ cấu hình, không từ dòng của xã (#18)", ten)
				}
			}
		}
		return true
	})
	if !thay {
		t.Fatal("không tìm thấy khai báo ThoiHanPhien trong phien.go — nếu nó đã chuyển đi nơi khác " +
			"thì phép kiểm này không còn canh được gì, và đó là điều cần biết")
	}
}
