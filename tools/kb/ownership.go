package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// WHY THIS FILE EXISTS: until 2026-09-20 `data-ownership.json` was an honest placeholder saying
// "no schemas defined yet", and that sentence had been false since ADR 0021 put `-- @entity:`
// marks into the migrations. Twenty-four marks across seven services, and the index said none.
//
// THAT IS THE EXPENSIVE KIND OF WRONG, because CLAUDE.md step 4 sends every session here first —
// "Who owns X -> data-ownership.json, do NOT read every schema". An index that answers "nothing
// is owned" does not look broken; it looks like an early-stage repo. So the instruction quietly
// stops working, every session falls back to reading schemas, and rule 2's one-owner rule is
// enforced by nobody. A generated index that is empty for a stale reason is worse than no index:
// the placeholder's own wording is what stops anyone asking.

// dauEntity matches `-- @entity: Name`. The mark, not the table: the ENTITY is the business
// concept rule 2 assigns an owner to, and a table name is an implementation of it.
var dauEntity = regexp.MustCompile(`^\s*--\s*@entity:\s*(\S+)`)

// dauScope matches `-- @scope: tenant | platform | cross-tenant`.
var dauScope = regexp.MustCompile(`^\s*--\s*@scope:\s*(\S+)`)

// dauBang picks the table the mark sits above, so a reviewer can go straight to it.
var dauBang = regexp.MustCompile(`(?i)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z0-9_]+)`)

type soHuu struct {
	Entity  string `json:"entity"`
	Service string `json:"service"`
	Scope   string `json:"scope"`
	Table   string `json:"table"`
	Source  string `json:"source"`
}

// quetSoHuu reads every service's migrations and returns one row per `@entity` mark.
//
// IT DOES NOT PARSE SQL, on purpose. The mark is the declaration — a hand-written statement of
// intent that a schema cannot carry — and reading the statement is the whole point. Inferring
// ownership from CREATE TABLE instead would produce a row for every join table and every
// partition, and would silently claim ownership nobody declared.
func quetSoHuu(root string, services []serviceEntry) ([]soHuu, []string, error) {
	var rows []soHuu
	var canhBao []string

	for _, s := range services {
		thuMuc := filepath.Join(root, s.Path, "migrations")
		tep, err := filepath.Glob(filepath.Join(thuMuc, "*.sql"))
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

			for i, d := range dong {
				m := dauEntity.FindStringSubmatch(d)
				if m == nil {
					continue
				}
				// The prose at the top of several migrations explains the convention using the
				// literal text `@entity`. dauEntity requires the colon form at line start, so
				// that prose does not match — but a mark with no table under it would still be a
				// declaration about nothing, and is reported rather than emitted.
				r := soHuu{Entity: m[1], Service: s.Name, Scope: "", Table: "",
					Source: fmt.Sprintf("%s:%d", rel, i+1)}

				// WALK UNTIL THE TABLE OR THE NEXT MARK — no line budget. The first version
				// stopped after 12 lines and lost 11 of 20 marks, because the house style puts a
				// long WHY comment between the mark and the table (0004_phieu_phan_anh.sql:214
				// declares `Petition`, the CREATE TABLE is 13 lines below). A number chosen by
				// guess gave an index that looked populated and was missing more than half —
				// worse than the empty placeholder it replaced, because it invites trust.
				//
				// A mark owns the FIRST CREATE TABLE after it; another @entity mark in between
				// means this one declares nothing, and that is reported rather than absorbed.
				for j := i + 1; j < len(dong); j++ {
					if dauEntity.MatchString(dong[j]) {
						break
					}
					if r.Scope == "" {
						if sm := dauScope.FindStringSubmatch(dong[j]); sm != nil {
							r.Scope = sm[1]
							continue
						}
					}
					if bm := dauBang.FindStringSubmatch(dong[j]); bm != nil {
						r.Table = bm[1]
						break
					}
				}

				if r.Table == "" {
					canhBao = append(canhBao, fmt.Sprintf(
						"%s: dấu @entity %q không có CREATE TABLE nào bên dưới", r.Source, r.Entity))
					continue
				}
				if r.Scope == "" {
					// SCOPE MISSING IS REPORTED, NEVER DEFAULTED. Guessing "tenant" here would put
					// a platform table behind a commune filter, or — far worse in the other
					// direction — declare a commune's table as platform-wide, which is rule 1's
					// whole subject matter. A blank field makes a reader look; a guessed one does
					// not.
					canhBao = append(canhBao, fmt.Sprintf(
						"%s: thực thể %q không khai @scope", r.Source, r.Entity))
				}
				rows = append(rows, r)
			}
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Entity != rows[j].Entity {
			return rows[i].Entity < rows[j].Entity
		}
		return rows[i].Source < rows[j].Source
	})
	return rows, canhBao, nil
}

// trung reports entities declared by more than one service — rule 2, invariant 1: exactly one
// owning service. Two owners is not a documentation problem, it is a distributed monolith
// forming, and the generated index is the only place it can be seen at a glance.
func trung(rows []soHuu) []string {
	theo := map[string][]string{}
	for _, r := range rows {
		theo[r.Entity] = append(theo[r.Entity], r.Service)
	}
	var ra []string
	for e, sv := range theo {
		khac := map[string]bool{}
		for _, s := range sv {
			khac[s] = true
		}
		if len(khac) > 1 {
			var ten []string
			for s := range khac {
				ten = append(ten, s)
			}
			sort.Strings(ten)
			ra = append(ra, fmt.Sprintf("thực thể %q được KHAI BỞI %d service: %s — luật 2 bất biến 1",
				e, len(ten), strings.Join(ten, ", ")))
		}
	}
	sort.Strings(ra)
	return ra
}
