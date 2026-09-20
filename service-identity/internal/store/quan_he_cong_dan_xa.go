package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// QuanHeCongDanXaStore reads the relationship between a citizen and ONE commune —
// quan_he_cong_dan_xa (@entity CitizenCommune, migration 0004).
//
// SCOPED LIKE EVERY OTHER COMMUNE-OWNED TABLE: it is built from *store.DB and every read goes
// through For(ctx), which owns `WHERE tenant_id = $1` and takes the commune from the context
// (rule 1, invariants 4 and 5). There is no path here to a raw handle. The cross-commune
// question this table also answers — "which communes is this citizen known to" — is NOT here;
// it is in store/crosstenant/, where it is countable (ADR 0021, rule 1, forbidden #6).
//
// # NO WRITE PATH, AND THAT IS A DECISION WITH A DATE ON IT
//
// Declaring, verifying and rejecting are all business writes that need an audit entry in the
// same transaction (rule 6, invariant 3) and a route with an explicit permission (rule 5,
// invariant 1). The permission key for "officer verifies a residence declaration" DOES NOT
// EXIST — open question #20 is OPEN, ADR 0011 forbids inventing the name, and rule 5,
// invariant 3b makes a wrong name a migration over live authorisation data rather than a
// rename. Open question #19 — whether a citizen may EDIT a declaration an officer has already
// verified — is open too, and its reversal cost is recorded as high and one-directional:
// answering it wrong now loses version history that cannot be reconstructed later.
//
// So a half-written write path here would look like both questions had been answered. They
// have not been.
//
// # BOTH READS RETURN BOTH FACTS, AND NO METHOD WILL EVER RETURN ONE WITHOUT THE OTHER
//
// domain.QuanHeCongDanXa carries the declaration AND the verification state together, and
// every read in this file selects both. A convenience method returning just `khai_cu_tru`
// would be the merge the table exists to prevent, arrived at by omission: the caller would
// count claims and report registrations. The migration spends thirty lines on why that figure
// is wrong by construction and why nothing turns red when it is.
type QuanHeCongDanXaStore struct {
	db *store.DB
}

func NewQuanHeCongDanXaStore(db *store.DB) *QuanHeCongDanXaStore {
	return &QuanHeCongDanXaStore{db: db}
}

// TranHangChoXacThuc bounds one read of the verification queue.
//
// 100 IS core/page.MaxLimit's NUMBER, borrowed rather than re-argued: it is the cap every list
// route in the system already serves. It is repeated as a constant here instead of importing
// page, because this store has no page.Request to parse — see HangChoXacThuc for why this
// queue is not cursor-paginated.
const TranHangChoXacThuc = 100

var (
	// ErrThieuPhienCongDan means the context carries no usable citizen session. FAIL CLOSED:
	// there is no fallback that could name the citizen, and every fallback anyone would reach
	// for takes the identity from something the client sent (rule 4, forbidden #1).
	ErrThieuPhienCongDan = errors.New("quan_he_cong_dan_xa: không có phiên công dân trong context")

	// ErrPhienLechXa means the commune of the session is not the commune the request is scoped
	// to. See TheoPhienCongDan.
	ErrPhienLechXa = errors.New("quan_he_cong_dan_xa: xã của phiên lệch xã của yêu cầu")

	// ErrKhongCoQuanHe means this citizen has no relationship row with this commune. It is a
	// legitimate answer, not a failure: the row is created by the citizen's first act toward
	// the commune (ADR 0005).
	ErrKhongCoQuanHe = errors.New("quan_he_cong_dan_xa: công dân chưa có quan hệ với xã này")

	// ErrGioiHanKhongHopLe rejects a non-positive page size instead of substituting one. A
	// default here would answer, on behalf of a route that does not exist yet, how much of a
	// work queue an officer sees.
	ErrGioiHanKhongHopLe = errors.New("quan_he_cong_dan_xa: giới hạn phải lớn hơn 0")

	// ErrGioiHanVuotTran rejects a request above the ceiling RATHER THAN CLAMPING IT.
	//
	// core/page clamps, and is right to: there, the client gets a cursor and can fetch the
	// remainder, so the cap costs nothing. Here there is no cursor (see HangChoXacThuc), so a
	// silent clamp is rows that vanished — citizens waiting in a queue that looks shorter than
	// it is. Refusing tells the caller the bound exists; clamping does not.
	ErrGioiHanVuotTran = errors.New("quan_he_cong_dan_xa: giới hạn vượt trần một lần đọc")
)

// cotQuanHe is the SELECT list, kept next to the one function that scans it.
//
// READ BY POSITION, so this list and quetQuanHe move together. Two of the pairs here are the
// silent kind:
//
//	khai_cu_tru / nguon_khai          adjacent TEXT — but they scan into domain.KhaiCuTru and
//	                                  domain.NguonKhai, two named types, so swapping the Scan
//	                                  targets does not compile. That is why they are types.
//	xet_duyet_boi / ly_do_tu_choi     BOTH plain string, and a swap produces no error at all:
//	                                  an officer's account id would be shown to the citizen as
//	                                  the reason their declaration was refused, and the reason
//	                                  would be filed as the name of the reviewer. Nothing can
//	                                  catch that but a test that pins the order by column NAME.
//
// coalesce ON THE TWO NULLABLE TEXT COLUMNS so the domain type can hold plain strings: "not
// reviewed" and "no reason" are both the empty string, and neither is a value the caller has
// to branch on. xet_duyet_luc keeps its NULL — a zero time.Time would read as 01/01/0001 on a
// screen, which is a date somebody would report.
const cotQuanHe = `cong_dan_id, khai_cu_tru, nguon_khai, khai_luc, trang_thai_xac_thuc,
                   coalesce(xet_duyet_boi,''), xet_duyet_luc, coalesce(ly_do_tu_choi,'')`

// locQuanHe is the one predicate both reads share.
//
// deleted_at IS NULL — rule 7, invariant 2: every read path excludes soft-deleted rows,
// everywhere. A relationship row is the basis of a citizen's access to their own files, so it
// is kept rather than deleted; kept and invisible is the whole point of the pair.
const locQuanHe = `AND deleted_at IS NULL`

// TheoPhienCongDan reads THIS citizen's relationship with THIS commune.
//
// # NEITHER IDENTITY IS A PARAMETER, AND THAT IS THE WHOLE SHAPE OF THIS METHOD
//
//	the commune  comes from the context and is bound to $1 by the scoped repository
//	             (rule 1, invariants 4 and 5)
//	the citizen  comes from the session the SERVER issued, read out of the context by
//	             httpx.CitizenSessionFrom (rule 4, invariant 2)
//
// There is no signature here that accepts a citizen id, so there is no route by which a
// request parameter could become one. A handler shaped like `?cong_dan_id=` cannot be written
// against this store: changing the value would read somebody else's data, which is rule 4,
// forbidden #1.
//
// A STAFF-FACING READ OF ONE CITIZEN'S DECLARATION IS NOT THIS METHOD, and must not be made by
// loosening it into taking an id. It is a different question with a different permission, and
// that permission has no name yet (open question #20).
//
// IT COMPARES THE TWO COMMUNES AND REFUSES ON A MISMATCH. On the citizen edge they cannot
// differ — httpx.XaTuPhien is what puts the session's commune into the context (ADR 0022) — so
// this check only ever fires when some other code path put a different commune there. That is
// precisely the case where failing closed matters: it is the shape of a citizen reading
// another commune's row, and it would otherwise be silent.
func (s *QuanHeCongDanXaStore) TheoPhienCongDan(ctx context.Context) (domain.QuanHeCongDanXa, error) {
	// tenant.MustFrom PANICS when the context carries no commune, exactly as the scoped
	// repository does one line below (core/store/scoped.go). That is the intended behaviour and
	// not something to guard: a read that ran without a commune would reach every commune's
	// rows or none, and both are silent. httpx.Recover turns the panic into a traceable 500.
	congDanID, err := congDanTuPhien(ctx, tenant.MustFrom(ctx))
	if err != nil {
		return domain.QuanHeCongDanXa{}, err
	}

	rows, err := s.db.For(ctx).Query(ctx, cotQuanHe, "quan_he_cong_dan_xa",
		locQuanHe+` AND cong_dan_id = $2`, congDanID)
	if err != nil {
		// No citizen id in the message: an error travels into logs, and the id is the handle on
		// a person (rule 3).
		return domain.QuanHeCongDanXa{}, fmt.Errorf("quan_he_cong_dan_xa: đọc: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.QuanHeCongDanXa{}, fmt.Errorf("quan_he_cong_dan_xa: đọc: %w", err)
		}
		return domain.QuanHeCongDanXa{}, ErrKhongCoQuanHe
	}
	q, err := quetQuanHe(rows)
	if err != nil {
		return domain.QuanHeCongDanXa{}, err
	}
	if err := rows.Err(); err != nil {
		return domain.QuanHeCongDanXa{}, fmt.Errorf("quan_he_cong_dan_xa: duyệt kết quả: %w", err)
	}
	return q, nil
}

// HangCho is one read of the verification queue.
type HangCho struct {
	// Muc is oldest declaration first. Never nil — an empty queue is a valid answer and must
	// not be a case the caller branches on.
	Muc []domain.QuanHeCongDanXa

	// ConNua reports that at least one more row was waiting beyond this read.
	//
	// IT IS NOT A COUNT, and this store deliberately offers none. A count over this table is
	// exactly the figure the migration warns about: it is only safe when both columns are read
	// together, and counting rows drops one of them by construction.
	ConNua bool
}

// HangChoXacThuc reads the commune's queue of declarations waiting for an officer.
//
// OLDEST FIRST, AND THE ORDER IS TOTAL. `khai_luc` is the queue's own order — the citizen who
// has waited longest is the one at risk — and `cong_dan_id` breaks ties. The pair is unique
// within a commune (PRIMARY KEY (tenant_id, cong_dan_id)), so two reads cannot return the same
// rows in a different sequence, and the partial index quan_he_cong_dan_xa_hang_cho serves the
// predicate.
//
// # WHY IT IS BOUNDED BUT NOT CURSOR-PAGINATED
//
// core/store.QueryPage is the system's keyset pagination and it cannot serve this table: it
// hard-codes `id` as the tie-break column, and quan_he_cong_dan_xa has no `id` — its key is
// (tenant_id, cong_dan_id). Teaching that helper a second tie-break column is a change to
// shared code in core/, and choosing the cursor shape a client sees is a route contract
// decision. The route is blocked anyway (open question #20), so the honest thing is a bounded
// read of the front of the queue plus ConNua, and a note saying what the next person has to
// build.
//
// WHAT THAT COSTS, stated rather than discovered later: an officer cannot walk past the first
// TranHangChoXacThuc rows. For a work queue — where the front is what gets processed and
// processing removes rows from it — that is the useful half. It is NOT enough for a screen
// that wants to browse the whole set, and building that screen means building the cursor.
func (s *QuanHeCongDanXaStore) HangChoXacThuc(ctx context.Context, gioiHan int) (HangCho, error) {
	switch {
	case gioiHan <= 0:
		return HangCho{}, ErrGioiHanKhongHopLe
	case gioiHan > TranHangChoXacThuc:
		return HangCho{}, ErrGioiHanVuotTran
	}

	// LIMIT IS gioiHan + 1: the extra row is how ConNua is known without counting rows over a
	// 32-way partitioned table. It is never returned.
	rows, err := s.db.For(ctx).Query(ctx, cotQuanHe, "quan_he_cong_dan_xa",
		locQuanHe+` AND trang_thai_xac_thuc = $2 ORDER BY khai_luc, cong_dan_id LIMIT $3`,
		string(domain.ChoXacThuc), gioiHan+1)
	if err != nil {
		return HangCho{}, fmt.Errorf("quan_he_cong_dan_xa: đọc hàng chờ: %w", err)
	}
	defer rows.Close()

	ra := HangCho{Muc: make([]domain.QuanHeCongDanXa, 0, gioiHan)}
	for rows.Next() {
		if len(ra.Muc) == gioiHan {
			ra.ConNua = true
			break
		}
		q, err := quetQuanHe(rows)
		if err != nil {
			return HangCho{}, err
		}
		ra.Muc = append(ra.Muc, q)
	}
	if err := rows.Err(); err != nil {
		return HangCho{}, fmt.Errorf("quan_he_cong_dan_xa: duyệt hàng chờ: %w", err)
	}
	return ra, nil
}

// congDanTuPhien reads the citizen identity out of the session the server issued, and refuses
// every other source.
//
// xaYeuCau IS THE COMMUNE THE REQUEST IS SCOPED TO, passed in rather than read from the
// context a second time, so the comparison is against the value the query will actually be
// bound to and not against a second lookup that could differ.
func congDanTuPhien(ctx context.Context, xaYeuCau tenant.ID) (string, error) {
	p, ok := httpx.CitizenSessionFrom(ctx)
	if !ok || p.CitizenID == "" {
		return "", ErrThieuPhienCongDan
	}
	if p.TenantID != xaYeuCau {
		return "", ErrPhienLechXa
	}
	return p.CitizenID, nil
}

// quetQuanHe scans the row the cursor is ALREADY on, in the order cotQuanHe names.
func quetQuanHe(rows quetDuoc) (domain.QuanHeCongDanXa, error) {
	var q domain.QuanHeCongDanXa
	// POSITIONAL — in lockstep with cotQuanHe. &q.XetDuyetLuc is a **time.Time: that is how
	// database/sql is told SQL NULL is a legitimate answer for a column that is NULL for every
	// row still waiting in the queue. A plain time.Time target would make every waiting row an
	// error, which is to say the queue would fail to read exactly the rows it exists to list.
	err := rows.Scan(&q.CongDanID, &q.Khai, &q.NguonKhai, &q.KhaiLuc, &q.TrangThai,
		&q.XetDuyetBoi, &q.XetDuyetLuc, &q.LyDoTuChoi)
	if err != nil {
		return domain.QuanHeCongDanXa{}, fmt.Errorf("quan_he_cong_dan_xa: đọc dòng: %w", err)
	}
	return q, nil
}
