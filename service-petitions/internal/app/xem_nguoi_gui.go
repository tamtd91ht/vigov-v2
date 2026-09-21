package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// XemNguoiGui records ONE disclosure of a reporter's full name and phone number to a member of
// staff holding `feedback.unmask`.
//
// # WHY A READ PATH OWNS A WRITE AT ALL
//
// Rule 6, invariant 7: reading FULL personal data is itself audited. `feedback.unmask` exists
// because the masked read route left an officer unable to ring a reporter back (ADR 0030), and
// the same ADR attaches the condition in the same breath — "mỗi lần đọc đầy đủ phải ghi vết" —
// and makes it stop condition #4: returning full personal data with no trail, on any route.
//
// So this is not bookkeeping around the feature. It IS half of the feature: a key that opens
// every reporter's contact details in a commune, with no record of who looked, is a silent back
// door into the personal data of everyone who has ever filed a petition. The trail is what makes
// the key answerable when an inspection asks who read what.
//
// # WHY IT IS A USE CASE AND NOT A LINE IN THE HANDLER
//
// audit.Write takes a *store.ScopedTx and there is deliberately no overload that writes outside
// one (rule 6, invariant 3). Opening that transaction is the use-case layer's job — the handler
// holds HTTP translation and nothing else — and putting it here is what keeps the disclosure and
// its record impossible to separate: there is no exported path in this package that hands back
// unmasked data without having written the entry first.
type XemNguoiGui struct {
	db *store.DB
}

func NewXemNguoiGui(db *store.DB) *XemNguoiGui { return &XemNguoiGui{db: db} }

// HanhViXemDayDu is the business verb in the trail. Vietnamese snake_case, like every other
// action already written by this system (`dang_nhap`, `ghi_so_thong_bao`) — an inspection reads
// these strings, and a function name would tell them nothing.
const HanhViXemDayDu = "xem_day_du_nguoi_gui"

// truongDaMo names the COLUMNS disclosed, never their values.
//
// Rule 6, forbidden #4: storing the before/after of a personal-data field turns the audit ledger
// itself into a store of personal data — one that is append-only and never deleted, which is the
// worst possible place for it to accumulate. Naming the fields answers "what was seen" without
// keeping a second copy of it.
var truongDaMo = []string{"nguoi_gui_ho_ten", "nguoi_gui_dien_thoai"}

// GhiVet appends the entry and returns only when it is COMMITTED.
//
// THE CALLER MUST TREAT AN ERROR AS "DO NOT DISCLOSE". Rule 6 leaves no second option: without
// the trail, the read is not allowed to happen. The ordering that makes that true — write first,
// answer second — belongs to the caller, and the reason it cannot be enforced from here is that
// only the caller is holding the response.
//
// `maTraCuu` IS THE SUBJECT and that is the sanctioned identifier: it is the code the citizen was
// handed, the one every screen names the petition by, and audit.Entry asks for a business code
// rather than an internal ULID that means nothing outside this service.
func (uc *XemNguoiGui) GhiVet(ctx context.Context, maTraCuu string, nguoi audit.Actor) error {
	if maTraCuu == "" {
		// An entry naming no record cannot answer "who read whose details", which is the only
		// question it exists to answer. Refusing here fails closed: the caller does not disclose.
		return fmt.Errorf("xem_nguoi_gui: thiếu mã tra cứu — vết không gắn được vào bản ghi nào")
	}

	delta, err := json.Marshal(map[string]any{
		"truong_da_mo": truongDaMo,
		"quyen":        "feedback.unmask",
	})
	if err != nil {
		return fmt.Errorf("xem_nguoi_gui: mã hoá delta: %w", err)
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// TenantID is left unset ON PURPOSE: audit.Write fills it from the transaction, which took
		// it from the context (rule 1, invariant 4). Passing it here would be a second source for
		// the one fact that decides which commune the entry belongs to.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViXemDayDu,
			Subject: maTraCuu,
			Delta:   delta,
		})
	})
	if err != nil {
		// NEITHER THE CODE NOR THE ACTOR IN THE MESSAGE. An error travels into centralised
		// logging, and `ma_tra_cuu` is the one string that opens a citizen's petition (rule 3).
		// The commune is not personal data and is what an operator needs to find the database.
		return fmt.Errorf("xem_nguoi_gui: ghi vết xem đầy đủ cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return nil
}
