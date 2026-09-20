package crosstenant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// QuanHeStore answers ONE question: which communes is this citizen known to.
//
// # WHY IT IS IN THIS PACKAGE AND NOT NEXT TO THE SCOPED STORE
//
// quan_he_cong_dan_xa HAS a tenant_id, and every other read of it goes through the scoped
// repository (store.QuanHeCongDanXaStore). This one cannot: ADR 0002 makes a citizen
// many-to-many with communes — registered residence in one, temporary residence in another, a
// report about a pothole seen while passing through a third — and the Mini App has to show
// them that list. Scoping it would mean naming a commune in order to find out which communes
// there are.
//
// The migration says the same thing above the index that serves it
// (quan_he_cong_dan_xa_theo_cong_dan, deliberately not prefixed with tenant_id): the query
// belongs here, and it still carries its own `// @cross-tenant:` mark (rule 1, forbidden #6).
//
// # WHAT MAKES IT A SANCTIONED READ RATHER THAN THE ONE THIS PACKAGE FORBIDS
//
// doc.go forbids, by name, any read of a commune's business data that returns rows of more
// than one commune — district or province rollups, while open question #4 is open. The
// difference is not the number of communes in the result; it is what the query is keyed on:
//
//	THIS         keyed on ONE citizen identity, taken from the session the server issued,
//	             returning that citizen's OWN rows. It is rule 4, invariant 1 — a citizen sees
//	             only their own data — expressed across the commune boundary that ADR 0002 put
//	             between a citizen and the communes they deal with.
//	A ROLLUP     keyed on a commune, or on nothing, returning rows belonging to OTHER people.
//	             That is the door open question #4 has not opened yet.
//
// There is no aggregate here and none may be added: no COUNT, no GROUP BY, no "how many
// citizens declared thường trú". The moment a method in this file stops being keyed on one
// citizen, it has become the thing doc.go forbids.
//
// # IT RETURNS A DIFFERENT TYPE FROM THE STAFF-FACING READ, ON PURPOSE
//
// QuanHeXa carries no xet_duyet_boi: the officer who reviewed a declaration is internal
// routing information and rule 4, forbidden #5 keeps it out of the citizen channel. The
// SELECT list does not name the column at all, so a handler cannot leak it by forgetting to
// drop a field — the value never leaves the database.
//
// ly_do_tu_choi IS RETURNED, and that is the deliberate other half: it is prose an officer
// wrote FOR THE CITIZEN TO READ (ADR 0023 §A2). A citizen who cannot tell "nobody has looked
// at this yet" from "this was refused, and here is why" is being closed out in silence.
type QuanHeStore struct {
	// THE RAW HANDLE, AND THE REASON ADR 0021 EXPECTS TO SEE ONE HERE: core/store.For(ctx)
	// panics without a commune, so an unscoped read has to step outside it — and that step is
	// visible, in a package named after what it does. The handle does not leave this package
	// (doc.go).
	raw *sql.DB
}

func NewQuanHeStore(db *sql.DB) *QuanHeStore {
	if db == nil {
		// Fail at wiring time, not on the first citizen request of the day.
		panic("crosstenant: NewQuanHeStore cần một kết nối")
	}
	return &QuanHeStore{raw: db}
}

// TranXaCuaCongDan bounds the list of communes one citizen is known to.
//
// IT IS A SIGNAL, NOT AN ESTIMATE OF HOW MANY COMMUNES A PERSON MIGHT DEAL WITH. A row is
// created by the citizen's first act toward a commune (ADR 0005), so fifty of them is a person
// who has filed something in fifty different commune-level units — past the point where this
// is one person using the channel and into the territory of an import gone wrong or an
// automated sender.
//
// REFUSE RATHER THAN TRUNCATE, the same decision as every other bounded read in this service:
// a silently short list is a commune missing from the citizen's own screen, where they have
// filed something and now cannot find it. A refusal breaks one screen loudly and names itself.
const TranXaCuaCongDan = 50

var (
	// ErrThieuPhienCongDan means the context carries no usable citizen session. There is no
	// fallback: every fallback anyone would reach for takes the identity from something the
	// client sent (rule 4, forbidden #1).
	ErrThieuPhienCongDan = errors.New("crosstenant: không có phiên công dân trong context")

	ErrQuaNhieuXa = errors.New("crosstenant: công dân có quá nhiều quan hệ xã")
)

// QuanHeXa is one commune a citizen is known to, with both facts about the relationship.
//
// BOTH, ALWAYS — what the citizen declared and whether an officer has checked it. They are
// independent, and a type carrying one without the other invites exactly the count the
// migration spends thirty lines warning about. domain.QuanHeCongDanXa says the same from the
// staff side.
type QuanHeXa struct {
	// XaID is the commune. The app pairs it with the commune's name through its own catalogue
	// call; this query has no business reading another service's tenant register (rule 2,
	// forbidden #2).
	XaID tenant.ID

	Khai      domain.KhaiCuTru
	NguonKhai domain.NguonKhai
	KhaiLuc   time.Time

	// TrangThai is independent of Khai. domain.DaXacThuc is CONVENIENCE DATA, never legal
	// grounds (ADR 0023 §A1).
	TrangThai domain.TrangThaiXacThuc

	// LyDoTuChoi is what the officer wrote for the citizen to read. Empty unless TrangThai is
	// domain.TuChoi — CONSTRAINT quan_he_tu_choi_phai_co_ly_do guarantees the pairing.
	LyDoTuChoi string
}

// truyVanXaCuaCongDan reads every live relationship of ONE citizen.
//
// xet_duyet_boi IS ABSENT FROM THE SELECT LIST — see the type comment. What is not read cannot
// be leaked.
//
// deleted_at IS NULL: rule 7, invariant 2. The partial index quan_he_cong_dan_xa_theo_cong_dan
// carries the same predicate, so this is the shape it was built for.
//
// ORDER BY khai_luc DESC, tenant_id — most recently declared first, which is the order a
// citizen's own list reads best in, and tenant_id breaks ties. The pair is unique per citizen
// (PRIMARY KEY (tenant_id, cong_dan_id)), so the order is TOTAL: two reads cannot return the
// same communes in a different sequence, and a screen cannot reshuffle under somebody's finger.
const truyVanXaCuaCongDan = `
SELECT tenant_id, khai_cu_tru, nguon_khai, khai_luc, trang_thai_xac_thuc,
       coalesce(ly_do_tu_choi,'')
FROM quan_he_cong_dan_xa
WHERE cong_dan_id = $1
  AND deleted_at IS NULL
ORDER BY khai_luc DESC, tenant_id
LIMIT $2`

// XaCuaCongDanTrongPhien lists the communes THIS citizen is known to.
//
// THE CITIZEN IS NOT A PARAMETER. It is read from the session the server issued
// (httpx.CitizenSessionFrom), so there is no signature here that a request parameter could be
// passed to — `?cong_dan_id=` cannot be written against this store, and changing a value in a
// URL cannot list somebody else's communes (rule 4, invariant 2 and forbidden #1).
//
// THE SESSION'S OWN COMMUNE IS IRRELEVANT HERE AND IS NOT READ. This call belongs to the
// discovery layer, where a citizen has signed in but has not chosen a commune yet (ADR 0005),
// so CitizenSession.TenantID is legitimately empty. Comparing against it would break the one
// screen this query exists for.
func (s *QuanHeStore) XaCuaCongDanTrongPhien(ctx context.Context) ([]QuanHeXa, error) {
	p, ok := httpx.CitizenSessionFrom(ctx)
	if !ok || p.CitizenID == "" {
		return nil, ErrThieuPhienCongDan
	}

	// @cross-tenant: ADR 0002 cho một công dân quan hệ NHIỀU-NHIỀU với xã, và Mini App phải
	// hiện được danh sách ấy — phạm vi hoá theo xã nghĩa là phải biết xã trước khi hỏi có
	// những xã nào. Khoá truy vấn là MỘT định danh công dân lấy từ phiên do máy chủ phát, trả
	// về đúng các dòng của chính công dân đó; không có tổng hợp, không có dòng của người khác
	// (câu hỏi mở #4 vẫn đang mở — xem doc.go).
	rows, err := s.raw.QueryContext(ctx, truyVanXaCuaCongDan, p.CitizenID, TranXaCuaCongDan+1)
	if err != nil {
		// No citizen id in the message: an error travels into logs, and the id is the handle on
		// a person (rule 3).
		return nil, fmt.Errorf("crosstenant: đọc quan hệ xã: %w", err)
	}
	defer rows.Close()

	ra := make([]QuanHeXa, 0, 8)
	for rows.Next() {
		var q QuanHeXa
		var xa string
		// POSITIONAL — in lockstep with truyVanXaCuaCongDan. khai_cu_tru and nguon_khai are
		// adjacent TEXT columns and scan into two NAMED types, so swapping them does not
		// compile; tenant_id and ly_do_tu_choi are the two plain strings, and a test pins the
		// order by column NAME because nothing else can.
		if err := rows.Scan(&xa, &q.Khai, &q.NguonKhai, &q.KhaiLuc, &q.TrangThai,
			&q.LyDoTuChoi); err != nil {
			return nil, fmt.Errorf("crosstenant: đọc dòng quan hệ xã: %w", err)
		}
		q.XaID = tenant.ID(xa)
		ra = append(ra, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("crosstenant: duyệt quan hệ xã: %w", err)
	}
	if len(ra) > TranXaCuaCongDan {
		// The rows already read are DROPPED rather than trimmed and returned: handing back a
		// list the caller might render anyway is how a refusal turns back into a silent
		// truncation, one careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuXa
	}
	return ra, nil
}
