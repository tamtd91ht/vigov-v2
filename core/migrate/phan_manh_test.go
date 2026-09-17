package migrate

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every table declared PARTITION BY must get its partitions in the SAME file.
//
// WHY THIS EXISTS AS A TEST AND NOT ONLY AS A HOOK. Six of the eight services shipped
// `audit_log ... PARTITION BY HASH (tenant_id)` with no partitions at all. A partitioned table
// with no partitions rejects EVERY insert, and because the audit entry shares the business
// transaction (rule 6, invariant 3), the first real business write would have rolled back
// entirely — not "the trail is missing", the operation itself cannot happen. In the petitions
// service that first write is a clerk taking a citizen's report.
//
// `tenant_scope_guard` now blocks this at the moment of typing, but a hook only sees edits that
// go through the agent. A person editing a migration in an editor, or a bad merge, reaches the
// repository without ever meeting it. This test is the half that covers them, and it is the
// half that runs in CI.
//
// It reads the files from disk rather than through go:embed on purpose: a .sql file that some
// service forgot to embed is still a file that will be read one day, and this catches it while
// an embed-based check would not see it at all.
var (
	khaiPhanManh = regexp.MustCompile(
		`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)\b[^;]*?\bPARTITION\s+BY\b`)
	taoPhanManh = regexp.MustCompile(`(?i)\bPARTITION\s+OF\s+([a-z_][a-z0-9_]*)`)
	binhLuanSQL = regexp.MustCompile(`--[^\n]*`)
)

func TestMoiBangPhanManhDeuCoManh(t *testing.T) {
	// Bố cục phẳng: mỗi dịch vụ là một thư mục cấp một, nên migration nằm ở
	// `<gốc kho>/<dịch vụ>/migrations/*.sql` chứ không còn dưới `services/`.
	tep, err := filepath.Glob(filepath.Join("..", "..", "*", "migrations", "*.sql"))
	if err != nil {
		t.Fatalf("không duyệt được thư mục migration: %v", err)
	}
	// A glob that silently matches nothing would make this test pass while checking nothing —
	// the exact shape of failure this whole area keeps producing.
	if len(tep) == 0 {
		t.Fatal("không tìm thấy tệp migration nào — đường dẫn sai, và một test không kiểm gì " +
			"thì tệ hơn không có test")
	}
	sort.Strings(tep)

	for _, f := range tep {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		// Comments are stripped first: the skeleton template teaches the shape by SHOWING the
		// declaration in a comment, and failing on the very comment that teaches it is how a
		// check gets deleted instead of obeyed.
		sql := binhLuanSQL.ReplaceAllString(string(b), "")

		khai := map[string]bool{}
		for _, m := range khaiPhanManh.FindAllStringSubmatch(sql, -1) {
			khai[strings.ToLower(m[1])] = true
		}
		for _, m := range taoPhanManh.FindAllStringSubmatch(sql, -1) {
			delete(khai, strings.ToLower(m[1]))
		}
		for bang := range khai {
			t.Errorf("%s: bảng %q khai PARTITION BY mà không có PARTITION OF nào.\n"+
				"  Bảng phân mảnh không có mảnh TỪ CHỐI MỌI INSERT. Vết đi cùng giao dịch với\n"+
				"  dữ liệu nghiệp vụ (luật 6 bất biến 3), nên bản ghi nghiệp vụ đầu tiên rollback\n"+
				"  toàn bộ — thao tác không thực hiện được, không phải chỉ thiếu vết.\n"+
				"  Tạo mảnh trong CÙNG tệp, MODULUS 32 theo ADR 0010.",
				filepath.ToSlash(f), bang)
		}
	}
}
