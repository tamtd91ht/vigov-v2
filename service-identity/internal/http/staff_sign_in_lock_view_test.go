package http

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// `sign_in_locked_until` on the staff register — the automatic sign-in lock of #39 (migration 0020)
// made visible to the administrator. Three properties, each of which fails silently:
//
//  1. it is DERIVED against now: a lock that has run out must not be shown, although the stored
//     instant is still on the row (nobody clears it);
//  2. it is ABSENT, not null and not a zero time, when there is no lock in force — omitempty is what
//     keeps the contract change additive;
//  3. it leaves ONLY through `admin.user` routes: PUT .../publication (`content.update`) returns the
//     same shape and must never carry it.
//
// The four cases of rule 5, invariant 7 for every route touched here are unchanged and live in
// can_bo_test.go, can_bo_ghi_test.go and can_bo_cong_khai_test.go — no route was added.

// withSignInLock returns cb with a stored lock end at `until`.
func withSignInLock(cb domain.CanBoTomTat, until time.Time) domain.CanBoTomTat {
	u := until.UTC().Truncate(time.Second)
	cb.SignInLockedUntil = &u
	return cb
}

// decodeObject reads one JSON object so key PRESENCE can be asserted — a typed decode cannot tell an
// absent key from a null one.
func decodeObject(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("body is not a JSON object: %v — %s", err, raw)
	}
	return m
}

func assertLockedUntil(t *testing.T, where string, obj map[string]json.RawMessage, want time.Time) {
	t.Helper()
	raw, ok := obj["sign_in_locked_until"]
	if !ok {
		t.Errorf("%s: sign_in_locked_until absent, want %s", where, want.Format(time.RFC3339))
		return
	}
	var got time.Time
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("%s: sign_in_locked_until = %s, not an RFC3339 time: %v", where, raw, err)
	}
	if !got.Equal(want) {
		t.Errorf("%s: sign_in_locked_until = %s, want %s", where, got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func assertNoLockedUntil(t *testing.T, where string, obj map[string]json.RawMessage) {
	t.Helper()
	if raw, ok := obj["sign_in_locked_until"]; ok {
		t.Errorf("%s: sign_in_locked_until = %s, want the key ABSENT (no lock in force)", where, raw)
	}
	if _, ok := obj["failed_sign_in_count"]; ok {
		t.Errorf("%s: failed_sign_in_count is on the wire — the count is deliberately not exposed", where)
	}
}

// The list and the detail: a lock in force is shown with its end; an expired one and a clean row are
// not shown at all.
func TestStaffReadsShowSignInLockOnlyWhileInForce(t *testing.T) {
	m := dungMayChu(t)
	now := time.Now().UTC()
	live := now.Add(3 * time.Hour).Truncate(time.Second)
	rows := m.danhBa.theo[xaA]
	rows[0] = withSignInLock(rows[0], live)                // idNoiBo — lock in force
	rows[1] = withSignInLock(rows[1], now.Add(-time.Hour)) // nd-02 — lock ran out an hour ago
	// rows[2] (nd-03) — never auto-locked

	tok := m.tokenCho(t, xaA, sidA)

	w := m.goi(t, "GET", hostA, "/api/v1/staff", "", tok)
	doiMa(t, w, http.StatusOK)
	var list struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 3 {
		t.Fatalf("list has %d items, want 3", len(list.Items))
	}
	for _, raw := range list.Items {
		obj := decodeObject(t, raw)
		var id string
		_ = json.Unmarshal(obj["id"], &id)
		switch id {
		case idNoiBo:
			assertLockedUntil(t, "list "+id, obj, live)
		default:
			assertNoLockedUntil(t, "list "+id, obj)
		}
	}

	w = m.goi(t, "GET", hostA, "/api/v1/staff/"+idNoiBo, "", tok)
	doiMa(t, w, http.StatusOK)
	assertLockedUntil(t, "detail locked", decodeObject(t, w.Body.Bytes()), live)

	w = m.goi(t, "GET", hostA, "/api/v1/staff/nd-02", "", tok)
	doiMa(t, w, http.StatusOK)
	assertNoLockedUntil(t, "detail expired", decodeObject(t, w.Body.Bytes()))
}

// Every `admin.user` WRITE route that answers with the row carries it too. Without this the screen
// redraws a row after, say, a role change and the lock indicator vanishes although the lock is still
// in force — a row that says one thing on the list and another after an edit.
func TestStaffAdminWriteRepliesCarrySignInLock(t *testing.T) {
	m := dungMayChu(t)
	live := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	m.ghiDanhBa.kq = withSignInLock(m.ghiDanhBa.kq, live)
	tok := m.tokenCho(t, xaA, sidA)

	for _, tg := range moiTuyenGhi() {
		w := m.goiGhi(t, tg, hostA, tok)
		doiMa(t, w, tg.ok)
		assertLockedUntil(t, tg.ten, decodeObject(t, w.Body.Bytes()), live)
	}
}

// The expired case on a write reply: the use case returns the row with a stored instant in the past;
// the reply must not show it.
func TestStaffWriteReplyHidesExpiredSignInLock(t *testing.T) {
	m := dungMayChu(t)
	m.ghiDanhBa.kq = withSignInLock(m.ghiDanhBa.kq, time.Now().UTC().Add(-time.Minute))

	w := m.goiGhi(t, moiTuyenGhi()[3], hostA, m.tokenCho(t, xaA, sidA)) // DELETE .../lockout
	doiMa(t, w, http.StatusOK)
	assertNoLockedUntil(t, "unlock reply", decodeObject(t, w.Body.Bytes()))
}

// NOT EXPOSED TO A NON-ADMIN READER. PUT .../publication is the one route answering with this shape
// under `content.update`; the row carries a lock in force, and the reply must not.
//
// MUTATION THAT MUST TURN THIS RED: answer DatCongKhaiCanBo with adminStaffView.
func TestPublicationReplyNeverCarriesSignInLock(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)
	m.ghiDanhBa.kq = withSignInLock(m.ghiDanhBa.kq, time.Now().UTC().Add(5*time.Hour))

	w := m.goiGhi(t, tuyenCongKhai(thanCongKhaiDung), hostA, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	assertNoLockedUntil(t, "publication reply (content.update)", decodeObject(t, w.Body.Bytes()))
}

// The boundary itself, without a server: the instant of expiry is already unlocked (LockedAt is
// `now.Before(until)`), and the value on the wire is UTC.
func TestAdminStaffViewDerivesTheLockAgainstNow(t *testing.T) {
	until := time.Date(2026, 9, 30, 21, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	cb := domain.CanBoTomTat{ID: idNoiBo, SignInLockedUntil: &until}

	if v := adminStaffView(cb, until.Add(-time.Second)); v.SignInLockedUntil == nil ||
		!v.SignInLockedUntil.Equal(until) || v.SignInLockedUntil.Location() != time.UTC {
		t.Errorf("one second before the end: %v, want %v in UTC", v.SignInLockedUntil, until.UTC())
	}
	if v := adminStaffView(cb, until); v.SignInLockedUntil != nil {
		t.Errorf("at the end: %v, want nil — the lock has run out", v.SignInLockedUntil)
	}
	if v := raNgoai(cb); v.SignInLockedUntil != nil {
		t.Error("raNgoai set the lock — it must stay the shape for non-admin routes")
	}
}
