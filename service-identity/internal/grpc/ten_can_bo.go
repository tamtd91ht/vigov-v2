package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// CanBoTen is the read behind ResolveStaffNames. SEPARATE FROM CanBoLo, and the separation is the
// point — see service-identity/internal/store/can_bo_ten.go for why the two predicates must not be
// merged. One interface carrying both would put "returns removed records" one careless edit away
// from the path an assignee dropdown calls.
type CanBoTen interface {
	TenTheoNhieuMa(ctx context.Context, ma []string) ([]domain.TenCanBo, error)
}

// ResolveStaffNames turns the staff codes an ALREADY-STORED record carries into names a person can
// read, INCLUDING codes whose staff record has since been removed from the commune's directory.
//
// WHY THIS EXISTS: `audit_log.actor_id` has no foreign key, so the ENTRY survives a soft delete —
// and the ability to READ it did not, because BatchGetStaff answers nothing for a removed record.
// A 2026 handling sheet opened in 2029 then shows a bare code where a name belongs, at the exact
// moment an inspection asks who handled the file. ADR 0034 carries the full argument.
//
// IT RESOLVES A NAME. IT DECIDES NOTHING. What comes back is for DISPLAY beside a record the caller
// was already entitled to open. A caller must not read "this code resolved" as "this person is
// staff" — it may have resolved PRECISELY BECAUSE the record was removed. The membership question
// is BatchGetStaff's, with the predicate that belongs to it.
//
// # THE FOUR PROPERTIES THAT KEEP THIS A LOOKUP AND NOT A ROSTER
//
// ADR 0034 §Bổ sung 22/09 restates them after the key changed from the internal id to `ma`, and the
// restatement matters here: `ma` is SHORT AND READABLE (`CB-2026-7K3M9Q`, 30 bits of randomness),
// so it is EASIER to guess than a ULID. Nothing below may lean on the key being unguessable.
//
//  1. ONE REQUEST FIELD, and it is a list of codes. Nothing in this handler reads a filter, a
//     cursor, a date range or a department, because the message declares none.
//  2. AN EMPTY LIST ANSWERS EMPTY. locID drops blanks and the store refuses an empty slice, so
//     there is no spelling of this request — including `ma: [""]` — that means "everybody".
//  3. THE RESPONSE CARRIES ONLY CODES THAT WERE ASKED FOR. Every item comes from a row the `ANY($2)`
//     predicate matched, and nothing adds a count or a cursor from which a caller could learn that
//     anything else exists.
//  4. THE ANSWER IS CUT BY "x-tenant-id". s.xa is called BEFORE the read for that reason, even
//     though StaffName carries no tenant_id — see below.
//
// # WHY s.xa IS CALLED WHEN ITS VALUE IS NOT USED
//
// It is the fail-closed check, not a field lookup. Without it a call arriving with no commune in
// context reaches store.Scoped, which calls tenant.MustFrom and PANICS — and a panic in a gRPC
// handler is not recovered, so one misrouted call takes down a process serving 200+ communes.
// Refusing here turns that into Internal, naming this deployment's wiring as the fault. The commune
// itself is bound to the query from the context (rule 1, invariant 4), never passed down.
//
// # NEVER LOG THE RESPONSE, OR ANY ITEM OF IT
//
// The generated String() prints full_name in full and `debug_redact` is a no-op in protobuf-go
// v1.36.12. A single `%+v` on this path puts a commune's staff names into the log pipeline, from
// where they cannot be recalled (rule 3, forbidden #1). Log the code; never the message.
//
// # READING THIS IS NOT AUDITED, and that is a decision rather than an omission
//
// This service cannot name a "who": the request carries codes and a commune and no principal, and
// ADR 0025's shared caller key proves only that the call came from inside the deployment. The
// service that OPENED THE RECORD holds the principal and the observed address, and that read is
// where the entry belongs, in its own transaction. ADR 0034 records both halves of the reasoning
// and what would change the answer.
//
// STATUS CODES: OK with zero or more items (fewer than asked is ORDINARY); INVALID_ARGUMENT above
// the ceiling, never a silent truncation; NOT_FOUND is never produced — an unresolved code is an
// ABSENT ITEM.
func (s *Server) ResolveStaffNames(ctx context.Context, req *identityv1.ResolveStaffNamesRequest) (
	*identityv1.ResolveStaffNamesResponse, error) {

	// NEVER LOG req OR THE RESPONSE BELOW. Both carry staff codes, and the response carries names.

	ma := req.GetMa()

	// THE CEILING IS CHECKED ON WHAT THE CALLER SENT, before duplicates are collapsed — same as
	// BatchGetStaff, and the same number on purpose (ResolveStaffNamesRequest says so), so a caller
	// that batches correctly for one batches correctly for both. Refusing is the contract; a
	// truncation would make real rows render blank with nothing reporting it.
	if len(ma) > TranIDMotLo {
		return nil, status.Errorf(codes.InvalidArgument,
			"ma vượt trần %d cho một lời gọi — bên gọi phải tự chia lô", TranIDMotLo)
	}

	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}

	can := locID(ma)
	if len(can) == 0 {
		// AN EMPTY REQUEST ANSWERS EMPTY, AND THE READ IS NEVER REACHED. It is not an error — a list
		// page with no rows legitimately produces no codes — and it is not delegated to the store
		// either, although that store refuses an empty slice too. This is anti-enumeration property
		// 2 (ADR 0034), the single line a later edit could turn a lookup into a roster with, so it
		// is held HERE, at the boundary the contract names, and does not depend on which
		// implementation is wired into Deps.Ten.
		return &identityv1.ResolveStaffNamesResponse{}, nil
	}

	hang, err := s.d.Ten.TenTheoNhieuMa(ctx, can)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveStaffNames")
	}

	// FEWER ITEMS THAN CODES IS A VALID RESPONSE and the order is not the order of the request. A
	// code is absent when it does not exist or belongs to another commune — the two are deliberately
	// indistinguishable. Being soft deleted is NO LONGER a reason for absence, which is the whole
	// point of this RPC.
	ra := make([]*identityv1.StaffName, 0, len(hang))
	for _, t := range hang {
		ra = append(ra, sangProtoTenCanBo(t))
	}
	return &identityv1.ResolveStaffNamesResponse{Items: ra}, nil
}

// sangProtoTenCanBo copies the read onto the wire type, FIELD BY FIELD and never by reflection.
// This is the one boundary a staff member's name crosses, and a generic mapper that copies
// "whatever is on the struct" is how a personal-data field eventually leaves this service without
// anybody deciding that it should.
func sangProtoTenCanBo(t domain.TenCanBo) *identityv1.StaffName {
	return &identityv1.StaffName{
		Ma:       t.Ma,
		FullName: t.HoTen,
		Standing: dungDanhBa(t.ConTrongDanhBa),
	}
}

// dungDanhBa maps `deleted_at IS NULL` onto the enum, and it is a named function so the mapping
// exists in exactly one place.
//
// IT NEVER PRODUCES UNSPECIFIED. The contract calls an unspecified standing a contract fault rather
// than "we could not find out" — the server answers it from one nullable column it has already
// read, so there is no third outcome to represent.
//
// A LOCKED ACCOUNT IS IN_DIRECTORY. Open question #10 (decided 2026-09-22) makes retirement a lock
// and keeps the person listed, so `dang_hoat_dong` is deliberately not consulted here. Reading it
// would put "không còn công tác" on a government screen about somebody who has not left.
func dungDanhBa(conTrongDanhBa bool) identityv1.StaffRecordStanding {
	if conTrongDanhBa {
		return identityv1.StaffRecordStanding_STAFF_RECORD_STANDING_IN_DIRECTORY
	}
	return identityv1.StaffRecordStanding_STAFF_RECORD_STANDING_REMOVED_FROM_DIRECTORY
}
