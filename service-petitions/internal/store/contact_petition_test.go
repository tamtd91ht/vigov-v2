package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The petitions side of the ADR 0102 link, on the recording driver of stored_file_test.go.
//
//	PROVED HERE   the intake INSERT binds $24 = core's national form of the typed number, NULL (never '')
//	              when empty or unnormalisable, anonymous rows included · the list and the count carry
//	              0046's partial-index predicate TEXT-IDENTICALLY (anonymous and deleted excluded), the
//	              commune as $1, the key as $2, `can-bo` excluded unless asked, the newest-first order with
//	              its tie-break, LIMIT/OFFSET bound · the backfill read selects only live, un-keyed rows with a
//	              number, after the cursor, in id order · the backfill write is single-column, guarded by
//	              `IS NULL` and the raw number, and reports 0 rows as "not written" · '' is refused.
//	NOT PROVED    PostgreSQL itself (the planner choosing the partial index, the triggers) — needs
//	              VIGOV_TEST_DSN, which this environment does not have.

func storeOn(d *sfDB) (*PhieuPhanAnhStore, *pkgstore.DB) {
	h := pkgstore.New(sql.OpenDB(sfConnector{d: d}))
	return NewPhieuPhanAnhStore(h), h
}

func TestCreateBindsSenderPhoneMatchKey(t *testing.T) {
	for name, c := range map[string]struct {
		raw       string
		anonymous bool
		want      any
	}{
		"national":              {"0900000000", false, "0900000000"},
		"international typed":   {"+84 900 000 000", false, "0900000000"},
		"anonymous still keyed": {"0900000000", true, "0900000000"},
		"empty is NULL":         {"", false, nil},
		"unnormalisable NULL":   {"090-abc", false, nil},
		"not national NULL":     {"900000000", false, nil},
	} {
		t.Run(name, func(t *testing.T) {
			row := createRow(nil, nil)
			row.NguoiGuiDienThoai, row.AnDanh = c.raw, c.anonymous
			st := runCreate(t, row)
			if !strings.Contains(st.sql, "location_citizen_adjusted, sender_phone_normalized)") ||
				!strings.Contains(st.sql, "$23,$24)") {
				t.Fatalf("column list moved; $24 no longer means sender_phone_normalized: %s", st.sql)
			}
			if len(st.args) != 24 {
				t.Fatalf("%d args, want 24", len(st.args))
			}
			if st.args[23] != c.want {
				t.Errorf("$24 = %#v, want %#v — NULL, never '', when there is no usable number", st.args[23], c.want)
			}
		})
	}
}

// THE PARTIAL INDEX'S PREDICATE, CHARACTER FOR CHARACTER (migration 0046 header).
const wantContactPredicate = "WHERE tenant_id = $1 AND sender_phone_normalized = $2 AND an_danh = false AND deleted_at IS NULL"

func TestContactPetitionsStatement(t *testing.T) {
	for name, restricted := range map[string]bool{"without restricted": false, "with restricted": true} {
		t.Run(name, func(t *testing.T) {
			d := &sfDB{rows: []map[string]driver.Value{{
				"ma_tra_cuu": "PA-4K7M-92XR-BTVD", "noi_dung": "x", "trang_thai": "da-tiep-nhan", "vao_so_luc": sfAt,
			}}}
			s, _ := storeOn(d)
			got, err := s.ContactPetitions(ctxXa(xaThu), "0900000000", restricted, 10, 20)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].LookupCode != "PA-4K7M-92XR-BTVD" || got[0].Status != domain.DaTiepNhan ||
				!got[0].ReceivedAt.Equal(sfAt) {
				t.Errorf("rows = %+v", got)
			}
			st := d.stmts[0]
			if !strings.Contains(st.sql, wantContactPredicate) {
				t.Errorf("predicate is not 0046's text — the partial index is silently unused:\n%s", st.sql)
			}
			if !strings.Contains(st.sql, "ORDER BY vao_so_luc DESC, id DESC LIMIT $3 OFFSET $4") {
				t.Errorf("order/paging = %s", st.sql)
			}
			hasExclusion := strings.Contains(st.sql, "linh_vuc <> '"+domain.LinhVucHanChe+"'")
			if hasExclusion == restricted {
				t.Errorf("can-bo exclusion present=%v for restricted=%v: %s", hasExclusion, restricted, st.sql)
			}
			if st.args[0] != string(xaThu) || st.args[1] != "0900000000" || st.args[2] != 10 || st.args[3] != 20 {
				t.Errorf("args = %#v", st.args)
			}
		})
	}
}

func TestCountContactPetitionsSamePredicate(t *testing.T) {
	d := &sfDB{rows: []map[string]driver.Value{{"count(*) AS total": int64(3)}}}
	s, _ := storeOn(d)
	n, err := s.CountContactPetitions(ctxXa(xaThu), "0900000000", false)
	if err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	st := d.stmts[0]
	if !strings.Contains(st.sql, wantContactPredicate) || !strings.Contains(st.sql, "linh_vuc <> '"+domain.LinhVucHanChe+"'") {
		t.Errorf("count predicate differs from the list's: %s", st.sql)
	}
}

func TestContactPetitionsRefuseEmptyKeyAndBadPage(t *testing.T) {
	d := &sfDB{}
	s, _ := storeOn(d)
	if _, err := s.ContactPetitions(ctxXa(xaThu), "", false, 10, 0); err == nil {
		t.Error("an empty key was queried — '' would match every blank sender")
	}
	if _, err := s.ContactPetitions(ctxXa(xaThu), "0900000000", false, 0, 0); err == nil {
		t.Error("limit 0 accepted")
	}
	if _, err := s.CountContactPetitions(ctxXa(xaThu), "", false); err == nil {
		t.Error("an empty key was counted")
	}
	if len(d.stmts) != 0 {
		t.Errorf("%d statements ran for refused calls", len(d.stmts))
	}
}

func TestSenderMatchKeyNullIsEmpty(t *testing.T) {
	d := &sfDB{rows: []map[string]driver.Value{{"sender_phone_normalized": nil}}}
	s, _ := storeOn(d)
	key, err := s.SenderMatchKey(ctxXa(xaThu), "pa-001")
	if err != nil || key != "" {
		t.Fatalf("key=%q err=%v", key, err)
	}
	if st := d.stmts[0]; !strings.Contains(st.sql, "AND id = $2 AND deleted_at IS NULL") || st.args[0] != string(xaThu) {
		t.Errorf("statement = %s %#v", st.sql, st.args)
	}
}

func TestSenderMatchKeyMissingRowIsNotFound(t *testing.T) {
	s, _ := storeOn(&sfDB{})
	if _, err := s.SenderMatchKey(ctxXa(xaThu), "pa-x"); err != ErrPhieuKhongTonTai {
		t.Fatalf("err = %v, want ErrPhieuKhongTonTai", err)
	}
}

func TestSenderPhoneBackfillBatchStatement(t *testing.T) {
	d := &sfDB{rows: []map[string]driver.Value{{"id": "pa-001", "ma_tra_cuu": "PA-4K7M-92XR-BTVD", "nguoi_gui_dien_thoai": "0900000000"}}}
	s, _ := storeOn(d)
	rows, err := s.SenderPhoneBackfillBatch(ctxXa(xaThu), "pa-000", 200)
	if err != nil || len(rows) != 1 || rows[0].ID != "pa-001" || rows[0].LookupCode != "PA-4K7M-92XR-BTVD" {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	st := d.stmts[0]
	for _, want := range []string{
		"WHERE tenant_id = $1", "sender_phone_normalized IS NULL", "nguoi_gui_dien_thoai <> ''",
		"deleted_at IS NULL", "id > $2 ORDER BY id LIMIT $3",
	} {
		if !strings.Contains(st.sql, want) {
			t.Errorf("backfill read lacks %q: %s", want, st.sql)
		}
	}
	if st.args[0] != string(xaThu) || st.args[1] != "pa-000" || st.args[2] != 200 {
		t.Errorf("args = %#v", st.args)
	}
}

func TestSetSenderPhoneMatchKeyIsGuardedSingleColumn(t *testing.T) {
	for name, c := range map[string]struct {
		affected int64
		want     bool
	}{"written": {1, true}, "keyed meanwhile": {0, false}} {
		t.Run(name, func(t *testing.T) {
			d := &sfDB{affected: c.affected}
			s, h := storeOn(d)
			var got bool
			if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
				var err error
				got, err = s.SetSenderPhoneMatchKey(ctx, tx, "pa-001", "+84900000000", "0900000000")
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("written = %v, want %v", got, c.want)
			}
			st := d.stmts[0]
			if !strings.Contains(st.sql, "SET sender_phone_normalized = $3\n") && !strings.Contains(st.sql, "SET sender_phone_normalized = $3 ") {
				t.Errorf("not a single-column UPDATE: %s", st.sql)
			}
			for _, want := range []string{"tenant_id = $1", "id = $2", "sender_phone_normalized IS NULL",
				"nguoi_gui_dien_thoai = $4", "deleted_at IS NULL"} {
				if !strings.Contains(st.sql, want) {
					t.Errorf("UPDATE lacks %q: %s", want, st.sql)
				}
			}
			if strings.Contains(st.sql, "cap_nhat_luc") {
				t.Error("a derived key must not move cap_nhat_luc")
			}
			if st.args[0] != string(xaThu) || st.args[2] != "0900000000" || st.args[3] != "+84900000000" {
				t.Errorf("args = %#v", st.args)
			}
		})
	}
}

func TestSetSenderPhoneMatchKeyRefusesEmpty(t *testing.T) {
	d := &sfDB{affected: 1}
	s, h := storeOn(d)
	err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		_, err := s.SetSenderPhoneMatchKey(ctx, tx, "pa-001", "", "")
		return err
	})
	if err == nil || len(d.stmts) != 0 {
		t.Fatalf("err=%v statements=%d — '' must never be written", err, len(d.stmts))
	}
}
