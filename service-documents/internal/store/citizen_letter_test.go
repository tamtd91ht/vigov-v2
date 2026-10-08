package store

// CitizenLetterStore without a database: the predicate builder for ADR 0084's status groups, and the
// "fixed at booking" columns appearing in no UPDATE. What only PostgreSQL can prove is in
// citizen_letter_pg_test.go.

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-documents/internal/domain"
)

func TestCitizenLetterFilterSQLStatusGroup(t *testing.T) {
	cases := []struct {
		group   domain.LetterStatusGroup
		holding string
		args    []any
	}{
		{domain.LetterGroupNew, " AND holding_unit_id IS NULL", []any{"moi-vao-so"}},
		{domain.LetterGroupAssigned, " AND holding_unit_id IS NOT NULL", []any{"moi-vao-so"}},
		{domain.LetterGroupInProgress, "", []any{"dang-xu-ly-don", "thu-ly", "dang-giai-quyet"}},
		{domain.LetterGroupNotProcessed, "", []any{"khong-thu-ly", "huong-dan", "luu-don", "dinh-chi"}},
	}
	for _, tc := range cases {
		cond, args := citizenLetterFilterSQL(CitizenLetterFilter{StatusGroup: tc.group})
		ph := make([]string, len(tc.args))
		for i := range tc.args {
			ph[i] = "$" + string(rune('2'+i))
		}
		want := " AND status IN (" + strings.Join(ph, ", ") + ")" + tc.holding
		if cond != want {
			t.Fatalf("%s: điều kiện %q, muốn %q", tc.group, cond, want)
		}
		if len(args) != len(tc.args) {
			t.Fatalf("%s: tham số %v, muốn %v", tc.group, args, tc.args)
		}
		for i := range args {
			if args[i] != tc.args[i] {
				t.Fatalf("%s: tham số %v, muốn %v", tc.group, args, tc.args)
			}
		}
	}

	// Combined with `status`: an intersection, numbered in order.
	cond, args := citizenLetterFilterSQL(CitizenLetterFilter{Status: domain.LetterStatusNew, StatusGroup: domain.LetterGroupAssigned})
	if cond != " AND status = $2 AND status IN ($3) AND holding_unit_id IS NOT NULL" || len(args) != 2 {
		t.Fatalf("trạng thái + nhóm: %q %v", cond, args)
	}

	// A value that slipped past the edge narrows to nothing — never the whole register.
	if cond, args := citizenLetterFilterSQL(CitizenLetterFilter{StatusGroup: "cho-phan-cong"}); cond != " AND false" || len(args) != 0 {
		t.Fatalf("nhóm lạ: %q %v", cond, args)
	}
}

// `source` is a fact of the booking act (domain.LetterSource), and `number` / `year` are never reissued
// (rule 7, invariant 3): none of the three may appear in any UPDATE of this file. Read from the source
// text because the statements are function-local constants — a new UPDATE that names one fails here.
func TestCitizenLetterUpdatesNeverTouchBookingFacts(t *testing.T) {
	raw, err := os.ReadFile("citizen_letter.go")
	if err != nil {
		t.Fatal(err)
	}
	updates := regexp.MustCompile(`(?s)UPDATE citizen_letter\s+SET(.*?)WHERE`).FindAllStringSubmatch(string(raw), -1)
	// Holder, status, result, sender, deadline — five today. Fewer means the regexp went blind.
	if len(updates) < 5 {
		t.Fatalf("tìm được %d câu UPDATE, muốn ít nhất 5 — phép đọc mã nguồn đã hỏng", len(updates))
	}
	col := regexp.MustCompile(`\b(source|number|year)\s*=`)
	for _, u := range updates {
		if m := col.FindString(u[1]); m != "" {
			t.Fatalf("một câu UPDATE ghi %q — cột cố định từ lúc vào sổ: %s", m, strings.TrimSpace(u[1]))
		}
	}
	if !strings.Contains(insertCitizenLetter, "source") {
		t.Fatal("câu INSERT không ghi `source` tường minh — nguồn do đường vào sổ khai, không phải DEFAULT của cột")
	}
}
