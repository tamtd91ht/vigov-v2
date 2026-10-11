package store

import (
	"database/sql/driver"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// `location_citizen_adjusted` (migration 0044).
//
//	PROVED HERE   the Scan reads true, false and NULL as true, false and nil — NULL is never folded
//	              into false · Tao binds the row's value as $23 under that column name, a nil pointer
//	              for "not recorded" (SQL NULL), never false.
//	NOT PROVED    the table's CHECK (true only with lat/lng) — that needs a real PostgreSQL.

func TestScanReadsLocationCitizenAdjusted(t *testing.T) {
	for name, c := range map[string]struct {
		col  driver.Value
		want string
	}{
		"true":  {true, "true"},
		"false": {false, "false"},
		"NULL":  {nil, "nil"},
	} {
		t.Run(name, func(t *testing.T) {
			k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(map[string]driver.Value{
				"location_citizen_adjusted": c.col})}}
			p, err := NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(k))).TheoMaTraCuu(ctxXa(xaThu), maThu)
			if err != nil {
				t.Fatal(err)
			}
			got := "nil"
			if p.LocationCitizenAdjusted != nil {
				got = map[bool]string{true: "true", false: "false"}[*p.LocationCitizenAdjusted]
			}
			if got != c.want {
				t.Errorf("LocationCitizenAdjusted = %s, want %s", got, c.want)
			}
			// The neighbours still line up: the new column is appended at the tail of cotPhieu.
			if p.Lat == nil || *p.Lat != 15.7654 || p.AwaitingConfirmationAt.IsZero() != true {
				t.Errorf("neighbouring columns shifted: lat %v, awaiting %v", p.Lat, p.AwaitingConfirmationAt)
			}
		})
	}
}

func TestCreateBindsLocationCitizenAdjusted(t *testing.T) {
	lat, lng := 16.0544, 108.2022
	yes, no := true, false
	for name, c := range map[string]*bool{"true": &yes, "false": &no, "not recorded": nil} {
		t.Run(name, func(t *testing.T) {
			row := createRow(&lat, &lng)
			row.LocationCitizenAdjusted = c
			st := runCreate(t, row)

			if !strings.Contains(st.sql, "zalo_account_id, location_citizen_adjusted)") ||
				!strings.Contains(st.sql, "$22,$23)") {
				t.Fatalf("column list moved; $23 no longer means location_citizen_adjusted: %s", st.sql)
			}
			if len(st.args) != 23 {
				t.Fatalf("%d args, want 23", len(st.args))
			}
			got, ok := st.args[22].(*bool)
			if !ok {
				t.Fatalf("$23 = %#v, want a *bool", st.args[22])
			}
			switch {
			case c == nil && got != nil:
				t.Errorf("$23 = %v, want nil (SQL NULL) — not recorded must never bind false", *got)
			case c != nil && (got == nil || *got != *c):
				t.Errorf("$23 = %v, want %v", got, *c)
			}
		})
	}
}
