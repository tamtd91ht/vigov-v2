package store

import (
	"context"
	"database/sql"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
)

// The incoming register's `q`: which columns it reads and that the text is a bound, literal value.
// The SQL shape is pinned through a fake driver; what it MATCHES is proved against PostgreSQL in
// TestPgIncomingSearchMatchesIssuingBody, which skips without VIGOV_TEST_DSN.

const searchTenantID = "01JTESTSEARCHXAXAXAXAXAXAX"

func TestIncomingSearchReadsIssuingBodyAsABoundLiteral(t *testing.T) {
	f := &countFake{} // no row: the list is empty, only the statement matters
	req, err := page.Parse(url.Values{}, SapXepVanBanDen)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewVanBanDenStore(pkgstore.New(sql.OpenDB(f))).
		DanhSach(ctxXa(searchTenantID), LocVanBanDen{Nam: 2026, Tim: "UBND_huyện 10%"}, req); err != nil {
		t.Fatal(err)
	}
	if len(f.stmts) != 1 {
		t.Fatalf("%d statements, want 1", len(f.stmts))
	}
	l := f.stmts[0]
	// $2 is the year; the search text is ONE parameter, $3, read by all three columns.
	want := " AND nam = $2 AND (trich_yeu ILIKE $3 OR COALESCE(so_ky_hieu,'') ILIKE $3" +
		" OR co_quan_ban_hanh ILIKE $3)"
	if !strings.Contains(l.sql, want) {
		t.Fatalf("predicate missing\n  sql  %q\n  want %q", l.sql, want)
	}
	if strings.Contains(l.sql, "UBND") {
		t.Fatal("search text spliced into the SQL instead of bound")
	}
	if len(l.args) < 3 || l.args[0] != searchTenantID || l.args[1] != int64(2026) {
		t.Fatalf("args = %v", l.args)
	}
	// `_` and `%` typed by the clerk are escaped — characters, not wildcards.
	if got, wantArg := l.args[2], `%UBND\_huyện 10\%%`; got != wantArg {
		t.Errorf("search arg = %q, want %q", got, wantArg)
	}
}

func TestPgIncomingSearchMatchesIssuingBody(t *testing.T) {
	db := moKetNoi(t)
	a, b := xaRieng(t)
	s := NewVanBanDenStore(pkgstore.New(db))

	insert := func(tenantID, id string, no int, issuer, summary string) {
		t.Helper()
		if _, err := db.ExecContext(context.Background(),
			`INSERT INTO van_ban_den (tenant_id, id, so_vao_so, nam, ngay_den, co_quan_ban_hanh,
			   loai_van_ban, trich_yeu, han_xu_ly_xong, trang_thai, nguoi_tao_ma)
			 VALUES ($1, $2, $3, 2026, '2026-09-01'::date, $4, 'cong-van', $5, $6, 'moi-vao-so', 'CB-00123')`,
			tenantID, id, no, issuer, summary, time.Now().UTC().Add(time.Hour)); err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}
	insert(a, "vb-s-1", 1, "Sở Tài chính tỉnh", "Về việc quyết toán ngân sách")
	insert(a, "vb-s-2", 2, "UBND huyện", "Về việc phòng chống lụt bão")
	// `_` in the issuer: a literal `_` in q must match it, and ONLY it.
	insert(a, "vb-s-3", 3, "Phòng TN_MT", "Về việc cấp phép")
	insert(a, "vb-s-4", 4, "Phòng TNXMT", "Về việc kiểm tra")
	// The other commune, same issuer: found only if isolation is broken.
	insert(b, "vb-s-b", 1, "Sở Tài chính tỉnh", "Về việc quyết toán")

	req, err := page.Parse(url.Values{"limit": {"100"}}, SapXepVanBanDen)
	if err != nil {
		t.Fatal(err)
	}
	ctx := ctxXa(a)
	for q, want := range map[string][]string{
		"tài chính": {"vb-s-1"}, // in the issuing body only
		"ubnd":      {"vb-s-2"}, // issuing body, ASCII case folded (non-ASCII folding depends on the DB ctype)
		"TN_MT":     {"vb-s-3"}, // `_` literal: vb-s-4 is not a match
		"lụt bão":   {"vb-s-2"}, // summary still searched
	} {
		res, err := s.DanhSach(ctx, LocVanBanDen{Tim: q}, req)
		if err != nil {
			t.Fatalf("q=%q: %v", q, err)
		}
		var got []string
		for _, v := range res.Items {
			got = append(got, v.ID)
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("q=%q: got %v, want %v", q, got, want)
		}
	}
}
