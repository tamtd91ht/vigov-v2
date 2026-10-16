package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/textfold"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// What is testable on every machine: the refusals made BEFORE the database, the SHAPE of the
// statements (placeholders bound, never concatenated; the Zalo predicate selects nothing but a
// boolean), and the two pure helpers. Everything that lives in the SQL — the keyset walk, the fold,
// the membership rows, the commune A / commune B invariant — is in citizen_directory_pg_test.go
// against a real PostgreSQL, where a fake cannot agree with whatever the query says.

func TestCitizenDirectoryWritesRefuseBeforeTheDatabase(t *testing.T) {
	s := &CitizenDirectoryStore{}
	g := &CitizenGroupStore{}
	ctx := context.Background()
	now := time.Now()

	for name, err := range map[string]error{
		"insert no tx":            s.Insert(ctx, nil, domain.CitizenContact{ID: "x", CreatedAt: now, Source: domain.CitizenContactSourceManual}),
		"update no tx":            s.Update(ctx, nil, domain.CitizenContact{ID: "x", UpdatedAt: now}),
		"soft delete no tx":       s.SoftDelete(ctx, nil, "x", "CB-1", "lý do", now),
		"set groups no tx":        s.SetGroups(ctx, nil, "x", nil, "CB-1", now),
		"group insert no tx":      g.Insert(ctx, nil, domain.CitizenGroup{ID: "x", CreatedAt: now}),
		"group update no tx":      g.Update(ctx, nil, domain.CitizenGroup{ID: "x", UpdatedAt: now}),
		"group soft delete no tx": g.SoftDelete(ctx, nil, "x", "CB-1", "", now),
	} {
		if !errors.Is(err, ErrThieuGiaoDich) {
			t.Errorf("%s: err = %v, want ErrThieuGiaoDich — a write outside the audit transaction must not reach the database", name, err)
		}
	}
	if _, _, err := s.ReadPhone(ctx, nil, "x"); !errors.Is(err, ErrThieuGiaoDich) {
		t.Errorf("ReadPhone without tx: %v — the full number is read only beside its audit entry", err)
	}
	if _, err := s.PhoneTaken(ctx, nil, "0900000000", ""); !errors.Is(err, ErrThieuGiaoDich) {
		t.Errorf("PhoneTaken without tx: %v", err)
	}
	if _, err := g.NameTaken(ctx, nil, "Hộ nghèo", ""); !errors.Is(err, ErrThieuGiaoDich) {
		t.Errorf("NameTaken without tx: %v", err)
	}
	if _, err := s.OwnAppSessionByPhone(ctx, nil, []string{"84900000000"}); !errors.Is(err, ErrThieuGiaoDich) {
		t.Errorf("OwnAppSessionByPhone without tx: %v", err)
	}
}

func TestCitizenContactFilterClauseBindsEverythingAndMatchesPhoneOnlyWithDigits(t *testing.T) {
	cases := []struct {
		name      string
		f         CitizenContactFilter
		wantArgs  int
		wantPhone bool
		wantParts []string
		noParts   []string
	}{
		{
			name: "empty filter is live rows only", f: CitizenContactFilter{}, wantArgs: 0,
			wantParts: []string{"deleted_at IS NULL"},
			noParts:   []string{"LIKE", "is_active", "EXISTS"},
		},
		{
			name: "keyword without digits folds name and address, never the phone",
			f:    CitizenContactFilter{Keyword: "Nguyễn"}, wantArgs: 1,
			wantParts: []string{"full_name_folded LIKE " + textfold.SQL("$2"), textfold.SQL("coalesce(address, '')") + " LIKE " + textfold.SQL("$2")},
			noParts:   []string{"phone LIKE"},
		},
		{
			name: "keyword with a digit also matches any part of the phone (ADR 0097 D11)",
			f:    CitizenContactFilter{Keyword: "văn 0900"}, wantArgs: 2, wantPhone: true,
			wantParts: []string{"OR phone LIKE $3"},
		},
		{
			name: "group filter binds the membership table to the commune itself",
			f:    CitizenContactFilter{GroupID: "g1"}, wantArgs: 1,
			wantParts: []string{"m.tenant_id = $1", "m.group_id = $2", "m.deleted_at IS NULL"},
		},
		{
			name: "unit, active and Zalo = true",
			f:    CitizenContactFilter{ResidentialUnitID: "u1", ActiveOnly: true, HasOwnAppSession: boolPtr(true)}, wantArgs: 1,
			wantParts: []string{"AND is_active", "residential_unit_id = $2", " AND EXISTS (SELECT 1 FROM phien_cong_dan p", "p.tenant_id = $1", "'84' || substr(citizen_contact.phone, 2)"},
			noParts:   []string{"AND NOT EXISTS"},
		},
		{
			name: "Zalo = false is the same predicate negated",
			f:    CitizenContactFilter{HasOwnAppSession: boolPtr(false)}, wantArgs: 0,
			wantParts: []string{" AND NOT EXISTS (SELECT 1 FROM phien_cong_dan p"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clause, args := citizenContactFilterClause(c.f)
			if len(args) != c.wantArgs {
				t.Errorf("args = %d, want %d (%q)", len(args), c.wantArgs, clause)
			}
			for _, p := range c.wantParts {
				if !strings.Contains(clause, p) {
					t.Errorf("clause lacks %q:\n%s", p, clause)
				}
			}
			for _, p := range c.noParts {
				if strings.Contains(clause, p) {
					t.Errorf("clause must not contain %q:\n%s", p, clause)
				}
			}
			if c.wantPhone {
				if got := args[len(args)-1]; got != "%0900%" {
					t.Errorf("phone pattern = %v, want %%0900%% — digits only, nothing for LIKE to misread", got)
				}
			}
			// Every placeholder in the clause is bound: $2..$(len(args)+1), none higher.
			for n := 2; n <= len(args)+1; n++ {
				if !strings.Contains(clause, fmt.Sprintf("$%d", n)) {
					t.Errorf("placeholder $%d bound but absent from the clause", n)
				}
			}
			if strings.Contains(clause, fmt.Sprintf("$%d", len(args)+2)) {
				t.Errorf("clause names $%d, which nothing binds", len(args)+2)
			}
			// A keyword is NEVER concatenated into the statement — only its placeholder number is.
			if c.f.Keyword != "" && strings.Contains(clause, strings.TrimSpace(c.f.Keyword)) {
				t.Errorf("the keyword itself is in the statement text")
			}
			// QueryPage refuses a filter that takes the ordering off it; keep the clause eligible.
			for _, cam := range []string{"order by", "limit", "offset", ";"} {
				if strings.Contains(strings.ToLower(clause), cam) {
					t.Errorf("clause contains %q — QueryPage would refuse it", cam)
				}
			}
		})
	}
}

func TestCitizenContactKeywordEscapesLikeBeforeTheFold(t *testing.T) {
	_, args := citizenContactFilterClause(CitizenContactFilter{Keyword: `50% a_b \`})
	if got := args[0]; got != `%50\% a\_b \\%` {
		t.Errorf("pattern = %q — LIKE's three special characters must be matched literally", got)
	}
}

// The Zalo predicate (ADR 0097 §3): bound to THIS commune on the session row, live sessions only,
// and it yields a boolean — no column of either table is projected.
func TestOwnAppSessionPredicateShape(t *testing.T) {
	pred := fmt.Sprintf(ownAppSessionPredicate, "u.phone")
	for _, want := range []string{
		"EXISTS (SELECT 1 FROM phien_cong_dan p",
		"JOIN dinh_danh_cong_dan d ON d.id = p.cong_dan_id",
		"p.tenant_id = $1",
		"d.so_dien_thoai = u.phone",
		"p.thu_hoi_luc IS NULL",
		"p.het_han_luc > now()",
	} {
		if !strings.Contains(pred, want) {
			t.Errorf("predicate lacks %q:\n%s", want, pred)
		}
	}
	for _, forbidden := range []string{"SELECT p.", "SELECT d.", "cong_dan_id,", "p.id", "tai_khoan_zalo"} {
		if strings.Contains(pred, forbidden) {
			t.Errorf("predicate projects %q — §3 invariant 3: a boolean and nothing else", forbidden)
		}
	}
	// The page's Zalo column is the same predicate on the row's own identity-form number, last in
	// the SELECT list (scanCitizenContactListItem reads it last).
	column := fmt.Sprintf(ownAppSessionPredicate, "'84' || substr(citizen_contact.phone, 2)")
	if !strings.HasSuffix(citizenContactPageCols, column) {
		t.Errorf("the page's Zalo column must be the predicate on the row's own identity-form number:\n%s", citizenContactPageCols)
	}
}

func TestCitizenDirectoryHelpers(t *testing.T) {
	if got := splitStoredTags(""); got == nil || len(got) != 0 {
		t.Errorf("splitStoredTags(\"\") = %#v, want empty non-nil", got)
	}
	if got := splitStoredTags("hộ nghèo,cựu chiến binh"); len(got) != 2 || got[1] != "cựu chiến binh" {
		t.Errorf("splitStoredTags = %#v", got)
	}
	if got := digitsOf("Nguyễn 09-00 abc"); got != "0900" {
		t.Errorf("digitsOf = %q, want 0900", got)
	}
	if got := digitsOf("không số"); got != "" {
		t.Errorf("digitsOf = %q, want empty", got)
	}
	if nullableRef(nil) != nil || nullableRef(strPtr("")) != nil || nullableRef(strPtr("u1")) != "u1" {
		t.Error("nullableRef: nil and \"\" must bind NULL, a value binds itself")
	}
	if got := nonNilTags(nil); got == nil {
		t.Error("nonNilTags(nil) must be an empty array, the column is NOT NULL")
	}
}

func TestTranslateCitizenDirectoryWriteError(t *testing.T) {
	for msg, want := range map[string]error{
		`duplicate key value violates unique constraint "citizen_contact_phone_live_unique"`:                                      ErrCitizenPhoneTaken,
		`duplicate key value violates unique constraint "citizen_contact_p03_tenant_id_phone_live_key_key"`:                       ErrCitizenPhoneTaken,
		`insert or update on table "citizen_contact_p01" violates foreign key constraint "citizen_contact_residential_unit_fkey"`: ErrCitizenContactUnitUnknown,
		`violates foreign key constraint "citizen_contact_group_group_fkey"`:                                                      ErrCitizenGroupNotFound,
		`violates foreign key constraint "citizen_contact_group_contact_fkey"`:                                                    ErrCitizenContactNotFound,
		`duplicate key value violates unique constraint "citizen_group_name_live_unique"`:                                         ErrCitizenGroupNameTaken,
	} {
		if got := translateCitizenDirectoryWriteError(errors.New(msg)); !errors.Is(got, want) {
			t.Errorf("%q → %v, want %v", msg, got, want)
		}
	}
	other := errors.New("connection reset")
	if got := translateCitizenDirectoryWriteError(other); got != other {
		t.Errorf("an unrecognised error must be returned unchanged, got %v", got)
	}
}

func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }
