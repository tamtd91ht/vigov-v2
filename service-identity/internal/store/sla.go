package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// SLAStore reads the commune's processing deadlines, in working hours (migration 0008).
//
// NOT A CATALOGUE STORE, and ADR 0029 says why in its own section: a catalogue row is a CHOICE
// that appears in a dropdown, an SLA row is A NUMBER THE SOFTWARE COMPUTES A COMMITMENT FROM.
// The table has no `ma`, no `nhan`, no `thu_tu` and none of the tier columns, and the permission
// that guards it is `admin.sla` rather than `admin.lookup` — a different key, deliberately.
//
// NO WRITE PATH, AND THE REASON IS NOT THE ONE THE CALENDAR STORES GIVE. Writing a row means
// accepting a `linh_vuc`, and a field code has to be CHECKED AT WRITE TIME against the closed
// tier-1 code set that lives in service `platform` (ADR 0026 §2; ADR 0024 §Cái giá của dòng
// Khối nhiệm vụ). Whether that check reads `platform` over gRPC or out of an event-fed replica
// HAS NO ADR, and writing that read path without one is ADR 0026 stop condition #2. A write path
// that skipped the check would ship the defect the specification already carries as evidence:
// 14-cau-hinh.md:308, an SLA row pointing at a code that no longer exists, rendered raw on the
// screen.
//
// NO gRPC SURFACE EITHER, AND THAT IS A SEPARATE STOP: ADR 0029 §Hệ quả ngay states that the
// shape of the RPC the two reading services need is undecided — one call returning HOURS with
// the caller adding them up, or one call returning a DEADLINE already computed. The two put the
// working-hours arithmetic in different places, and ADR 0007 forbids there being two
// implementations of it. This store is the half both shapes need identically: one commune's
// rows, read inside the service that owns them.
type SLAStore struct {
	db *store.DB
}

func NewSLAStore(db *store.DB) *SLAStore { return &SLAStore{db: db} }

// TranSLA is the hard upper bound on one commune's deadline table.
//
// Same argument as TranLichLamViec: one process serves 200+ communes, so every list route is a
// shared resource and an unbounded one is forbidden (skills/rest-api-design §5). This read
// returns the WHOLE table — see DanhSach for why it cannot be paginated — so the bound cannot
// come from a `limit` parameter and has to be a ceiling the commune's data is checked against.
//
// 200 IS ABOUT FIVE TIMES THE OUTER PLAUSIBLE SHAPE. Three kinds of work times the twelve
// platform field codes, plus a default row each, is 39; the specification's own table has 16
// rows. The number is not an estimate of how many rows a commune might configure; it is the
// point past which the rows have stopped being a deadline table — an import run twice, a loop
// that inserted rows, a fixture on a live database.
const TranSLA = 200

// ErrQuaNhieuDongSLA says the ceiling was reached. The caller answers 500 and refuses.
//
// REFUSE RATHER THAN TRUNCATE, and here truncation has a second victim beyond the missing rows:
// DongTheoLinhVuc falls back to the DEFAULT row, so a truncated read that happened to drop a
// field's own row does not fail — it QUIETLY ANSWERS WITH THE DEFAULT DEADLINE. The commune
// promised 16 working hours for an `an-ninh-trat-tu` report and the software promises 56,
// nothing errors, and the difference reaches the citizen. A refusal breaks one commune's screen
// loudly and names itself in the log.
var ErrQuaNhieuDongSLA = errors.New("sla: vượt trần số dòng thời hạn xử lý")

// ErrLoaiViecLa says a row carries a kind of work the software does not know.
//
// THE DATABASE SHOULD HAVE REFUSED IT (sla_loai_viec_hop_le, migration 0008), so reaching this
// means the CHECK is not there: a database restored from elsewhere, a constraint dropped by
// hand, or a fourth value added without the decision ADR 0029 stop condition #1 requires.
// REFUSING THE WHOLE READ IS THE POINT — skipping the unknown row instead would hand back a
// table that looks complete and is missing a kind of work, and the screen showing it would give
// a commune no way to see that.
var ErrLoaiViecLa = errors.New("sla: loại việc không nằm trong ba giá trị được phép")

// cotSLA IS READ BY POSITION in DanhSach.
//
// FIVE ADJACENT INTEGER COLUMNS — the worst shape this repository knows. Swapping any two of
// them, here or in the Scan, produces NO ERROR AT ALL: the rows still load, the screen still
// renders, and the only difference is that the authority has promised something else. There is
// no CHECK that could catch it either, because every one of the five is simply a positive
// number (migration 0008, sla_gio_phai_duong). The fixtures in sla_test.go therefore give all
// five DIFFERENT values, and the column list is asserted against the Scan order BY NAME.
//
// `linh_vuc` IS SELECTED RAW, NOT COALESCED, and that is what keeps `ORDER BY ... NULLS FIRST`
// unambiguous: PostgreSQL resolves a bare name in ORDER BY against the OUTPUT column list
// first, so an output column named `linh_vuc` that was really `coalesce(linh_vuc, ”)` would
// silently make the NULLS FIRST clause decorative. The NULL is folded into "" once, in Go,
// where it can be read.
const cotSLA = `id, loai_viec, linh_vuc, gio_tiep_nhan, gio_xu_ly_xong, ` +
	`gio_sap_den_han, gio_bao_lanh_dao, gio_bao_chu_tich`

// DanhSach reads the commune's whole deadline table, ordered.
//
// NOT PAGINATED, AND THE REASON IS NOT SIZE. Answering "what is the deadline for this field"
// needs two rows, not one: the field's own row IF IT HAS ONE, and the default row it falls back
// on otherwise (domain.DongTheoLinhVuc). A page cannot promise to hold both, and a caller
// handed a page has no way to tell "this field has no row" from "this page does not reach it" —
// the first means use the default, the second means the answer is wrong. The bound pagination
// would have provided is TranSLA, enforced in SQL.
//
// AN EMPTY RESULT IS NOT AN ERROR AND IT IS NOT A DEFAULT. It is returned as an empty list —
// a list, never a nil the caller has to branch on — because the configuration screen must be
// able to show "chưa cấu hình", and today that is EVERY commune: migration 0008 seeds nothing
// and the onboarding step does not exist. What must never happen is a caller treating the empty
// list as "no deadline applies"; domain.VanDeCuaSLA turns it into a named problem, and anything
// computing a deadline must refuse on it (rule 10).
//
// ORDER BY loai_viec, linh_vuc NULLS FIRST — the default row first, because it is the row
// everything else falls back to. It is TOTAL, so two calls cannot return the same rows in a
// different sequence: UNIQUE (tenant_id, loai_viec, linh_vuc_khoa) makes the pair unique per
// commune among live rows (migration 0008). It is also exactly the index, NULLS FIRST included
// — an index built with PostgreSQL's ASC default sorts NULLs LAST and could not serve it.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the
// context (rule 1, invariants 4 and 5), so this can only ever read the deadlines of the commune
// the request arrived in. Which matters more here than on most tables: an SLA row read across
// the boundary would make one commune's promise to its citizens be computed from another
// commune's policy.
func (s *SLAStore) DanhSach(ctx context.Context) ([]domain.DongSLA, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling returns a full list indistinguishable from a complete one of
	// that size — the truncation this refuses to perform, performed by the bound meant to
	// prevent it.
	rows, err := s.db.For(ctx).Query(ctx, cotSLA, "sla",
		`AND deleted_at IS NULL ORDER BY loai_viec, linh_vuc NULLS FIRST LIMIT $2`, TranSLA+1)
	if err != nil {
		return nil, fmt.Errorf("sla: đọc thời hạn xử lý: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.DongSLA, 0, 16)
	for rows.Next() {
		var d domain.DongSLA
		var loaiViec string
		// NULL IS THE DEFAULT ROW, and it is folded into "" exactly here — the one place that
		// conversion happens. The schema refuses the empty string outright
		// (sla_linh_vuc_khong_rong), so "" arriving in Go can only have come from a NULL.
		var linhVuc sql.NullString
		// POSITIONAL — in lockstep with cotSLA. See the note there on the five adjacent integer
		// columns; this Scan is the other half of that pair.
		if err := rows.Scan(&d.ID, &loaiViec, &linhVuc,
			&d.GioTiepNhan, &d.GioXuLyXong, &d.GioSapDenHan,
			&d.GioBaoLanhDao, &d.GioBaoChuTich); err != nil {
			return nil, fmt.Errorf("sla: đọc dòng: %w", err)
		}
		d.LoaiViec = domain.LoaiViec(loaiViec)
		if !d.LoaiViec.HopLe() {
			// The value is NOT put in the error: it came from the database and an error message
			// travels into logs and back to clients (rule 3, forbidden #3). The row's ULID is
			// what an operator needs to find it, and it is not personal data.
			return nil, fmt.Errorf("sla: dòng %s: %w", d.ID, ErrLoaiViecLa)
		}
		d.LinhVuc = linhVuc.String
		ra = append(ra, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sla: duyệt kết quả: %w", err)
	}
	if len(ra) > TranSLA {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a
		// list the caller might render anyway is how a refusal turns back into a silent
		// truncation one careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuDongSLA
	}
	return ra, nil
}
