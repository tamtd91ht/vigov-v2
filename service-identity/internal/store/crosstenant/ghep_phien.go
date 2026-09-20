package crosstenant

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// GhepPhienStore finds a pairing code BEFORE the commune is known — ghep_phien, migration 0004.
//
// # WHY THE LOOKUP CANNOT BE SCOPED, AND WHY "IT WOULD STILL FAIL CLOSED" IS NOT THE ANSWER
//
// Scoping this by the citizen's own commune would be simpler and would still refuse a code
// from another commune: it would come back "not found". But ADR 0019, invariant 8 asks for
// more than failing closed — a commune mismatch must produce 401 AND AN ALERT, and an alert
// requires knowing that the code exists SOMEWHERE ELSE rather than nowhere. A quiet "not
// found" is indistinguishable from a typo, and the case worth alerting on is the one where a
// citizen's phone was pointed at the QR of a screen belonging to a different commune.
//
// So the lookup finds the row first and the commune is COMPARED afterwards, by the use case
// (ADR 0019 step 5). The migration says the same thing above the index that serves it
// (ghep_phien_theo_ma), and doc.go of this package named this store as the second shape that
// may be added here.
//
// # WHAT KEEPS THE EXEMPTION NARROW
//
//   - The key is the SHA-256 of a 256-bit secret the server minted. Nothing a caller can
//     enumerate reaches this query.
//   - It returns AT MOST ONE row and never a list, so there is no query here that spans
//     communes in the sense open question #4 is about.
//   - It is a READ of four timestamps and four characters. It cannot spend the code: that is
//     store.GhepPhienStore.Dung, which is scoped, runs in the caller's transaction, and lets
//     the database enforce single use.
//   - It returns no citizen identity and no session id, although the row carries both after a
//     redeem. Whoever presents a code holds the code; they must not thereby learn WHO spent it.
type GhepPhienStore struct {
	// The raw handle, for the reason ADR 0021 expects to see one in this package: there is no
	// commune yet, so core/store.For(ctx) has nothing to scope by. It does not leave (doc.go).
	raw *sql.DB
}

func NewGhepPhienStore(db *sql.DB) *GhepPhienStore {
	if db == nil {
		panic("crosstenant: NewGhepPhienStore cần một kết nối")
	}
	return &GhepPhienStore{raw: db}
}

// ErrMaGhepTrung means two rows answered to one code hash, and the answer to that is no.
//
// IT IS POSSIBLE AT ALL because the UNIQUE constraint on bam_ma is composite with tenant_id —
// a unique index on a hash-partitioned table must contain the partition key — so uniqueness is
// PER COMMUNE, not global. With 256 bits of randomness a collision is not a practical concern;
// it is handled because "not practical" is not "impossible", and the one thing that must never
// happen is telling a screen in commune A about commune B's pairing. Two rows means the store
// cannot say whose code this is, so it says nothing.
var ErrMaGhepTrung = errors.New("crosstenant: hai mã ghép cùng mã băm")

// MaGhep is a pairing code as this layer can see it.
//
// NO cong_dan_id AND NO phien_id, even though the row carries them once it has been redeemed.
// The state (DaGhep) is what a caller needs in order to refuse; WHO redeemed it is a fact
// about a citizen, and this lookup is reachable by anyone holding the code.
type MaGhep struct {
	// XaID is the commune OF THE SCREEN that asked for the code. ADR 0019, invariant 8 requires
	// the caller to compare it with the commune of the citizen's session — 401 plus an alert on
	// a mismatch, never a quiet "not found".
	XaID tenant.ID

	// ID identifies the row for store.GhepPhienStore.Dung, which is scoped and does the
	// spending.
	ID string

	// KyTuDoiChieu is the four characters the Mini App shows so the citizen can compare them by
	// eye with the screen in front of them (ADR 0019, invariant 4). Not a secret.
	KyTuDoiChieu string

	HetHanLuc time.Time
	DungLuc   *time.Time
	HuyLuc    *time.Time
}

// TrangThai derives the state of the code at `bayGio`. There is no trang_thai column and there
// must never be one — see domain.TrangThaiGhep for the rule and for why "expired" in
// particular is a comparison and not a stored value.
func (m MaGhep) TrangThai(bayGio time.Time) domain.TrangThaiMaGhep {
	return domain.TrangThaiGhep(m.HetHanLuc, m.DungLuc, m.HuyLuc, bayGio)
}

// truyVanMaGhepTheoBam looks the code up by its hash.
//
// NO PREDICATE ON THE STATE, and that is deliberate: an expired or already-spent code must
// still be FOUND, because "expired" and "somebody else already used this" are two different
// things to tell a citizen standing at a counter, and both are different from "this code does
// not exist". Filtering here would collapse all three into silence — the same failure ADR 0019
// invariant 8 rejects for the commune mismatch.
//
// deleted_at IS ABSENT because ghep_phien has no soft-delete columns: a code ends by being
// spent, cancelled or expiring, all three of which this SELECT reads. Rule 7 applies to the
// tables that carry those columns, and this is not one of them — the rows are kept forever as
// evidence that a citizen authorised a specific screen at a specific time.
const truyVanMaGhepTheoBam = `
SELECT tenant_id, id, ky_tu_doi_chieu, het_han_luc, dung_luc, huy_luc
FROM ghep_phien
WHERE bam_ma = $1`

// TheoMa resolves a raw pairing code to the row the server wrote for it.
//
// ok=false WITH A NIL ERROR MEANS NO SUCH CODE. That is a legitimate answer — a mistyped or
// already-purged code — and it is separate from an error so that a database failure is never
// read as a statement about somebody's pairing.
//
// THE RAW CODE NEVER LEAVES THIS FUNCTION. Only its hash is bound as a parameter, so a driver
// error that echoes the statement cannot carry the code into a log; for the two minutes it
// lives, the code substitutes for a citizen's session (ADR 0019).
func (s *GhepPhienStore) TheoMa(ctx context.Context, ma string) (MaGhep, bool, error) {
	if ma == "" {
		return MaGhep{}, false, nil
	}

	// @cross-tenant: ADR 0019 bất biến 8 đòi lệch xã phải ra 401 KÈM BÁO ĐỘNG, mà báo động thì
	// cần biết mã CÓ TỒN TẠI ở một xã khác chứ không phải không tồn tại ở đâu cả — nên phải tìm
	// thấy dòng trước, đối chiếu xã sau (migration 0004 nói đúng câu này trên chỉ mục
	// ghep_phien_theo_ma). Tìm theo mã băm của một bí mật 256 bit do máy chủ phát, trả về nhiều
	// nhất MỘT dòng, không bao giờ là danh sách.
	rows, err := s.raw.QueryContext(ctx, truyVanMaGhepTheoBam, bamMaGhep(ma))
	if err != nil {
		return MaGhep{}, false, fmt.Errorf("crosstenant: tra mã ghép: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return MaGhep{}, false, fmt.Errorf("crosstenant: tra mã ghép: %w", err)
		}
		return MaGhep{}, false, nil
	}

	var m MaGhep
	var xa string
	// POSITIONAL — in lockstep with truyVanMaGhepTheoBam. tenant_id, id and ky_tu_doi_chieu are
	// three adjacent TEXT columns and dung_luc / huy_luc are two adjacent nullable timestamps:
	// every swap among either group scans cleanly and produces a wrong answer with no error at
	// all. Swapping the two timestamps turns "somebody cancelled this" into "somebody used
	// this" and back. A test pins the order by column NAME, which is the only thing that can.
	if err := rows.Scan(&xa, &m.ID, &m.KyTuDoiChieu, &m.HetHanLuc, &m.DungLuc, &m.HuyLuc); err != nil {
		return MaGhep{}, false, fmt.Errorf("crosstenant: đọc mã ghép: %w", err)
	}
	if rows.Next() {
		return MaGhep{}, false, ErrMaGhepTrung
	}
	if err := rows.Err(); err != nil {
		return MaGhep{}, false, fmt.Errorf("crosstenant: tra mã ghép: %w", err)
	}
	m.XaID = tenant.ID(xa)
	return m, true, nil
}

// bamMaGhep is SHA-256, hex — the same shape store.bamRefresh uses for session tokens, and the
// same reason it needs no salt and no work factor: the input is 256 bits of randomness, not a
// password.
//
// WHY IT IS THREE LINES DUPLICATED RATHER THAN AN IMPORT: store.bamRefresh is unexported, and
// this package deliberately does not depend on the scoped store (the dependency runs the other
// way — a use case wires both). Exporting it, or moving it to core/, is the right call on the
// day a third caller appears; today it would move a one-line function into shared code to save
// one line. What matters is that both compute the same thing, which the pg suite checks
// against the real column rather than against this comment.
func bamMaGhep(ma string) string {
	tong := sha256.Sum256([]byte(ma))
	return hex.EncodeToString(tong[:])
}
