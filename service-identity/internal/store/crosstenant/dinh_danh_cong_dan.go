package crosstenant

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// DinhDanhStore finds — and, the first time, creates — the platform-wide identity of one
// citizen, keyed by the phone number Zalo verified (ADR 0020).
//
// THE TABLE HAS NO tenant_id AT ALL, WHICH IS WHY THIS TYPE IS IN THIS PACKAGE. ADR 0002
// decided that one phone number is ONE RECORD FOR THE WHOLE PLATFORM: a citizen deals with
// several communes at once — registered residence in one, temporary residence in another, a
// report about a pothole seen while passing through a third. Keying the phone per commune
// would confine a citizen to exactly one commune forever.
//
// IT HOLDS NO DATABASE HANDLE, AND THAT IS DELIBERATE. ADR 0021 expects a cross-commune store
// to carry a raw *sql.DB, because core/store.For(ctx) panics without a commune and there is no
// other way in. This one does not need the handle: its only method is a WRITE that must share
// the caller's transaction with the audit entry (rule 6, invariant 3), so the transaction
// arrives as an argument. The visible sign ADR 0021 asks for is still visible — a raw *sql.Tx
// in the signature, in a package named for what it does — and there is no handle sitting here
// that a later method could run an unscoped query on without anyone noticing.
//
// NOTHING HERE READS BY ID, AND THAT IS NOT AN OVERSIGHT. The only question this table
// answers on the citizen path is "which identity belongs to this verified number". A lookup
// keyed by anything a caller could enumerate would turn the one table with no commune boundary
// into an oracle over every citizen on the platform.
type DinhDanhStore struct{}

func NewDinhDanhStore() *DinhDanhStore { return &DinhDanhStore{} }

// DinhDanh is one citizen's platform-wide identity.
//
// THE PHONE NUMBER IS NOT A FIELD HERE, ON PURPOSE. It is personal data (rule 3) and nothing
// above this layer needs it: the identifier is what travels into sessions, audit entries and
// business records. A struct that cannot carry the number cannot leak it into a log line, an
// error, or a `%+v` somebody added while debugging.
type DinhDanh struct {
	ID string
}

var (
	ErrSoRong        = errors.New("dinh danh: thiếu số điện thoại")
	ErrSoQuaDai      = errors.New("dinh danh: số điện thoại dài bất thường")
	ErrThieuGiaoDich = errors.New("dinh danh: cần giao dịch của lời gọi để ghi vết đi cùng")
)

// doDaiSoToiDa bounds an unbounded client-supplied string reaching a government database. The
// longest E.164 number is 15 digits; the margin covers a leading '+' and separators that the
// caller has not stripped.
const doDaiSoToiDa = 20

// TimHoacTao returns the identity of a phone number, creating it the first time that number is
// seen anywhere on the platform. The second result reports whether this call created it.
//
// WHY vuaTao IS RETURNED: the two cases are two different business facts, and the use case has
// to record the right one — "a citizen registered" is not "a citizen signed in". Rule 6,
// invariant 2 asks the trail to say what a person DID, and only the caller can put a who, a
// when, an IP and a commune next to it.
//
// IT RUNS IN THE CALLER'S TRANSACTION AND OPENS NONE OF ITS OWN. If the caller's transaction
// rolls back, this row goes with it. The alternative — committing it separately — leaves a row
// holding a citizen's phone number with no session and no audit entry beside it, which is a
// personal-data record created without a trail (rule 3, rule 6).
//
// THE RACE, AND WHAT IT COSTS, stated rather than discovered later: two first sign-ins of the
// SAME number at the same instant. The loser's SELECT sees nothing, its INSERT blocks on the
// unique index, and ON CONFLICT then returns the winner's id — the right identity, so no
// citizen is ever split in two. What it gets wrong is vuaTao: both calls report true, so one
// audit entry says "registered" where "signed in" would have been exact. That is one mislabelled
// line in the trail in a race that needs two devices on one number in the same millisecond, and
// it is the price of not reading PostgreSQL's system columns to tell insert from update.
func (s *DinhDanhStore) TimHoacTao(ctx context.Context, tx *sql.Tx, soDienThoai string) (DinhDanh, bool, error) {
	// THE ARGUMENTS ARE CHECKED BEFORE THE TRANSACTION IS TOUCHED, and the order is not
	// arbitrary: it is what lets every one of these refusals be tested on a machine with no
	// database, which is the machine this repository is usually built on.
	so := strings.TrimSpace(soDienThoai)
	switch {
	case so == "":
		return DinhDanh{}, false, ErrSoRong
	case len(so) > doDaiSoToiDa:
		// The number itself NEVER goes into the error. An error travels to a log line, to a
		// monitoring system, and sometimes to a screen (rule 3, forbidden #3).
		return DinhDanh{}, false, ErrSoQuaDai
	}
	if tx == nil {
		return DinhDanh{}, false, ErrThieuGiaoDich
	}

	// @cross-tenant: dinh_danh_cong_dan có một dòng cho mỗi công dân trên TOÀN NỀN TẢNG và không
	// có cột tenant_id nào để phạm vi hoá (ADR 0002). Trả về nhiều nhất một dòng, tìm theo số đã
	// được Zalo xác thực — không phải theo thứ người gọi liệt kê được.
	var id string
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM dinh_danh_cong_dan WHERE so_dien_thoai = $1`, so).Scan(&id)
	switch {
	case err == nil:
		return DinhDanh{ID: id}, false, nil
	case !errors.Is(err, sql.ErrNoRows):
		// %w, and without the number: the caller decides what to do, the operator sees which
		// statement failed, and neither sees the citizen's phone number.
		return DinhDanh{}, false, fmt.Errorf("dinh danh: tìm: %w", err)
	}

	moi, err := maULID()
	if err != nil {
		return DinhDanh{}, false, err
	}

	// ON CONFLICT DO UPDATE, not DO NOTHING, and the difference is the race above: DO NOTHING
	// returns no row when another transaction has just inserted this number, leaving the caller
	// with neither an id nor an error. DO UPDATE waits for that transaction and then returns the
	// row that won. The update is a deliberate no-op — a sign-in changes nothing about an
	// identity, and touching cap_nhat_luc would record a change that did not happen.
	//
	// @cross-tenant: cùng lý do với truy vấn trên — bảng định danh công dân không thuộc xã nào
	// (ADR 0002). Ghi một dòng cho một số điện thoại, không đụng tới dữ liệu của xã nào.
	err = tx.QueryRowContext(ctx,
		`INSERT INTO dinh_danh_cong_dan (id, so_dien_thoai) VALUES ($1, $2)
		 ON CONFLICT (so_dien_thoai)
		 DO UPDATE SET cap_nhat_luc = dinh_danh_cong_dan.cap_nhat_luc
		 RETURNING id`, moi, so).Scan(&id)
	if err != nil {
		return DinhDanh{}, false, fmt.Errorf("dinh danh: tạo: %w", err)
	}
	return DinhDanh{ID: id}, true, nil
}

// chuCaiULID is Crockford's base32 alphabet — no I, L, O or U, so a transcribed identifier
// cannot turn into a different valid one.
const chuCaiULID = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// maULID returns a ULID: 48 bits of millisecond timestamp followed by 80 bits of randomness,
// encoded as 26 characters.
//
// WHY A ULID AND NOT A RANDOM STRING: the schema constrains this column to 26 characters
// (dinh_danh_cong_dan_id_la_ulid), and rule 1, invariant 2 requires an identifier that is
// opaque and immutable. The timestamp prefix keeps inserts roughly ordered in the index; the
// 80 random bits are what make the value unguessable.
//
// IT IS NEVER DERIVED FROM THE PHONE NUMBER. An identifier that can be reversed into personal
// data IS personal data, and this one appears in audit entries and in business records of every
// commune the citizen deals with (ADR 0020, invariant 5).
//
// WHY IT IS WRITTEN HERE AND NOT PULLED FROM A LIBRARY: it is 20 lines with no dependency, and
// this is the only place in the repository that mints one. Moving it to core/ is the right call
// on the day a second caller appears, not before.
func maULID() (string, error) {
	var b [16]byte

	ms := uint64(time.Now().UTC().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	if _, err := rand.Read(b[6:]); err != nil {
		return "", fmt.Errorf("dinh danh: sinh mã ngẫu nhiên: %w", err)
	}

	// 128 bits do not divide into 5-bit groups, so the encoding is of a 130-bit field whose two
	// leading bits are zero: 130 / 5 = 26 characters exactly. `du` starts at 2 for those bits.
	var out [26]byte
	var goi uint32
	du := uint(2)
	vt := 0
	for _, by := range b {
		goi = goi<<8 | uint32(by)
		du += 8
		for du >= 5 {
			du -= 5
			out[vt] = chuCaiULID[(goi>>du)&31]
			vt++
		}
	}
	return string(out[:]), nil
}
