package domain

// ACCOUNTLESS PETITIONS — ADR 0083, A TEMPORARY PATH.
//
// While the shared Mini App (App ViHAT) is not approved by Zalo, `getAccessToken` answers -1401 and no
// session can be opened, so ADR 0080's unverified path is closed too. ADR 0083 opens a petition with NO
// owner at all: no session, no citizen, no Zalo account. It is removed by one code change when the app
// passes review (ADR 0083 §Gỡ bỏ) — every piece of it is named `Accountless…` so one search finds it;
// the petitions already received stay (rule 7).
//
// NO MIGRATION AND NO FLAG COLUMN (ADR 0083 row 11): the shape is DERIVED, here, from columns every row
// already has. A stored flag beside them would be a second copy of one fact (rule 9).

// OwnerAccountless names the sender of an accountless petition: there is no owner, and its ID is
// always "". It is NOT a kind PetitionOwner.Valid accepts — an owner-filtered read (OwnedByCode) handed
// it refuses, because "no owner" there would mean "no filter". It exists only so the intake can be told
// explicitly that this send has nobody behind it (app.AccountlessSender).
const OwnerAccountless PetitionOwnerKind = "accountless"

// IsAccountless reports that this owner is the accountless sender: the kind, and no id.
func (o PetitionOwner) IsAccountless() bool { return o.Kind == OwnerAccountless && o.ID == "" }

// AccountlessChannel is the channel every accountless petition carries — the Mini App's. The store
// binds it into the SQL form of Accountless, so the two cannot name different channels.
const AccountlessChannel = KenhZaloMiniApp

// Accountless reports that the petition was sent through the accountless path (ADR 0083): the Mini App
// channel, with neither a citizen nor a Zalo account behind it.
//
// THE ONE DEFINITION (ADR 0083 row 11). The staff DTO's `accountless`, the daily ceiling's count and the
// public lookup's read all derive from it — the store's SQL is the same three conditions, bound from
// AccountlessChannel and checked against this function after the scan. A staff-booked petition (channel
// `can-bo-nhap-ho`, also ownerless) is NOT accountless: the channel is what tells them apart.
func (p PhieuPhanAnh) Accountless() bool {
	return p.Kenh == AccountlessChannel && p.CongDanID == "" && p.ZaloAccountID == ""
}

// AccountlessDailyCeiling is how many accountless petitions ONE COMMUNE accepts in one day (ADR 0083
// row 3), counted from 00:00 Asia/Ho_Chi_Minh (UnverifiedCountingDayStart) with soft-deleted petitions
// included (row 12, as ADR 0080). A SECURITY THRESHOLD CHOSEN BY THE OWNER, a constant on purpose —
// changing it is ADR 0083 stop condition #3 and a rule 13 stop condition, never a configuration edit.
const AccountlessDailyCeiling = 200
