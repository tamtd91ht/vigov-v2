package domain

import (
	"fmt"
	"sort"
)

// GioTrongNgay is a wall-clock time of day, held as SECONDS SINCE MIDNIGHT.
//
// WHY AN INTEGER AND NOT time.Time, AND IT IS NOT A STYLE CHOICE — three reasons, each of which
// has bitten somebody:
//
//  1. `bat_dau` and `ket_thuc` are `TIME WITHOUT TIME ZONE` (migration 0006:126). A time.Time
//     ALWAYS carries a date and a location, so scanning one attaches 0000-01-01 UTC to a value the
//     schema deliberately left without either. The instant is produced once, in the deadline
//     function, by combining this with a DATE in Asia/Ho_Chi_Minh — and a stray location here is
//     what shifts that instant by seven hours without anything failing.
//  2. PostgreSQL's TIME accepts 24:00:00, and the migration's CHECK (ket_thuc > bat_dau) permits a
//     session ending there. pgx refuses to scan 24:00:00 into a time.Time at all
//     (pgtype/builtin_wrappers.go:471) — so a commune whose office closes "at midnight" would make
//     this read fail with a driver error naming nothing. As seconds it is simply 86400.
//  3. The overlap check below is arithmetic. On time.Time it would be arithmetic on two instants
//     that are only accidentally on the same day.
//
// THE UNIT IS SECONDS AND NOT MINUTES because the column can hold seconds. Rounding to minutes
// would move a configured boundary by up to 59 seconds with nothing on any screen to show it —
// small, invisible, and exactly the kind of silent drift this codebase refuses elsewhere.
type GioTrongNgay int

// GiayTrongNgay is midnight-to-midnight in seconds. A session may end here — PostgreSQL's TIME
// allows 24:00:00 — so this is a legal value, not an upper bound to reject.
const GiayTrongNgay GioTrongNgay = 24 * 60 * 60

// Chuoi renders the time as HH:MM:SS.
//
// FIXED WIDTH, SECONDS ALWAYS PRESENT, AND ONE SHAPE ON THE WIRE. Dropping ":SS" when it is zero
// would make the contract carry two shapes for one field, and a client would have to parse both;
// a client that handles only one handles the other wrong. It also sorts lexicographically, which
// is what a screen listing sessions wants.
//
// The value comes from a TIME column and therefore cannot be negative; nothing here defends
// against a negative because nothing can produce one.
func (g GioTrongNgay) Chuoi() string {
	return fmt.Sprintf("%02d:%02d:%02d", int(g)/3600, (int(g)/60)%60, int(g)%60)
}

// Khoang is one half-open interval [BatDau, KetThuc) inside one GROUP.
//
// THE GROUP IS WHAT MAKES ONE OVERLAP CHECK SERVE TWO TABLES. For `lich_lam_viec` the group is the
// ISO weekday; for `ngay_lam_bu` it is the date. Two sessions in different groups can never
// overlap, and a check that ignored the group would report every Monday morning as clashing with
// every Tuesday morning.
type Khoang struct {
	ID      string
	Nhom    string
	BatDau  GioTrongNgay
	KetThuc GioTrongNgay
}

// CapChongNhau is one pair of sessions that overlap, inside one group.
type CapChongNhau struct {
	Nhom string
	A    string // the session that starts first — ids, never times: the times are on the rows
	B    string
}

// TimChongNhau reports every pair of intervals that overlap inside the same group.
//
// WHY THIS EXISTS AT ALL, AND WHY IT IS NOT IN THE DATABASE. Two overlapping sessions on one
// weekday DOUBLE-COUNT those hours, so a deadline computed across them is short — the authority
// looks on time when it was late. The database-level answer is an EXCLUDE constraint, which needs
// the `btree_gist` extension, and a migration that fails for a missing extension does not degrade:
// it stops the service (ADR 0013). Migration 0006:109 states that trade-off and hands the
// obligation here: UNIQUE (tenant_id, thu, bat_dau) keeps two sessions from STARTING at the same
// minute, and nothing in the schema keeps 07:30–11:30 from overlapping 09:00–12:00.
//
// IT SURFACES, IT DOES NOT SILENTLY REPAIR. Merging the two intervals, or dropping one, would make
// the read answer a question the commune never configured — and the configuration screen would go
// on showing two rows while the deadline used something else.
//
// HALF-OPEN ON PURPOSE: a morning ending at 11:30 and an afternoon starting at 11:30 do NOT
// overlap. Touching boundaries are the ordinary shape of a working day with no lunch break, and
// reporting them would bury the real clashes under noise.
//
// O(n²) WITHIN A GROUP, deliberately: a group holds a handful of sessions (migration 0006:31 puts
// the whole week at about ten), and the pairwise loop is the version whose correctness is readable.
// A sweep comparing only neighbours misses the case where one long session swallows two short ones.
func TimChongNhau(ks []Khoang) []CapChongNhau {
	theoNhom := map[string][]Khoang{}
	for _, k := range ks {
		theoNhom[k.Nhom] = append(theoNhom[k.Nhom], k)
	}

	// The groups are walked in sorted order so the answer is STABLE between two calls. An unstable
	// list of problems makes a screen reorder itself on every reload and makes any test of this
	// compare sets by accident.
	nhoms := make([]string, 0, len(theoNhom))
	for n := range theoNhom {
		nhoms = append(nhoms, n)
	}
	sort.Strings(nhoms)

	var ra []CapChongNhau
	for _, n := range nhoms {
		trong := append([]Khoang(nil), theoNhom[n]...)
		sort.SliceStable(trong, func(i, j int) bool {
			if trong[i].BatDau != trong[j].BatDau {
				return trong[i].BatDau < trong[j].BatDau
			}
			return trong[i].ID < trong[j].ID
		})
		for i := 0; i < len(trong); i++ {
			for j := i + 1; j < len(trong); j++ {
				if trong[j].BatDau >= trong[i].KetThuc {
					// Sorted by start, so nothing after j can start earlier either.
					break
				}
				ra = append(ra, CapChongNhau{Nhom: n, A: trong[i].ID, B: trong[j].ID})
			}
		}
	}
	return ra
}
