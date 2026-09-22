package identityclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
)

// TranMaMotLo is the ceiling on ONE ResolveStaffNames call.
//
// IT MIRRORS THE CONTRACT, which is the source of truth (rule 2, invariant 7): both ends read the
// same number from ResolveStaffNamesRequest, and neither imports the other — service-identity's
// copy lives in its own `internal/`, which rule 2, forbidden #1 puts out of reach. Deliberately the
// same number as the BatchGetStaff ceiling, so a caller that batches correctly for one batches
// correctly for both.
//
// REFUSED HERE RATHER THAN SENT: the server answers INVALID_ARGUMENT for exactly this, and a
// request that can only fail has no business on the network. NEVER TRUNCATED — a clamped batch
// makes real rows render blank on an archival record with nothing reporting it.
const TranMaMotLo = 200

// TenCanBo is one resolved name, as a caller of this package sees it.
//
// THE STANDING IS KEPT AS THE CONTRACT'S ENUM AND IS NOT REDUCED TO A bool. The zero value of this
// struct is then UNSPECIFIED — loud — whereas a bool's zero value would quietly claim one of the two
// real answers, and on this field one of them is a statement about a person printed on a government
// screen. ResolveStaffNames states the same reasoning for the wire type; it holds one layer up for
// the same reason.
type TenCanBo struct {
	// HoTen is PERSONAL DATA (rule 3). NEVER LOG IT, at any level, on any path that holds it. The
	// whole point of the wrapper below is that the gRPC response never escapes it, because the
	// generated String() prints full_name in full.
	HoTen string

	// TrangThai says whether the staff RECORD is still in the commune's directory. IT DOES NOT SAY
	// WHETHER THE PERSON STILL WORKS THERE — a locked account is IN_DIRECTORY (open question #10),
	// and a removed record may belong to somebody perfectly current under another row. The
	// Vietnamese sentence a screen prints is the CALLER's wording, not this contract's.
	TrangThai identityv1.StaffRecordStanding
}

// TenCanBoTheoMa asks identity for the names behind staff codes an ALREADY-STORED record carries —
// including codes whose staff record has since been removed from the commune's directory.
//
// WHY IT EXISTS: `audit_log.actor_id` stores `ma` (rule 6, invariant 8) and nothing outside
// service-identity can turn that into a person — a second service reading `nguoi_dung` directly is
// rule 2, forbidden #2. Without this call a 2026 handling sheet opened in 2029 shows a bare code
// where a name belongs, at the exact moment an inspection asks who handled the file (ADR 0034).
//
// # IT RESOLVES A NAME. THE CALLER MUST DECIDE NOTHING FROM IT
//
// A code that resolves may have resolved PRECISELY BECAUSE the record was removed. Never read a
// present item as "this is staff", "this person can be assigned work", or "this account exists" —
// that question is BatchGetStaff's, and it has the predicate that answers it.
//
// # ONLY STAFF CODES BELONG HERE
//
// `audit_log.actor_kind` holds "staff", "citizen" or "system". Only the first is answerable: a
// citizen is platform-wide and has no `nguoi_dung` row (ADR 0002), and `system` is not a person at
// all. FILTER ON actor_kind BEFORE CALLING — sending either costs an absent item the caller then has
// to tell apart from a real absence.
//
// # AN ABSENT CODE IS ORDINARY. AN ERROR IS NOT AN ABSENT CODE
//
// Fewer names than codes asked for is a NORMAL answer — the code may belong to no row, or to
// another commune, and the two are deliberately indistinguishable. This is the one place this
// package does NOT demand a complete answer, and the difference from HanXuLy is deliberate: there a
// missing item would become a zero deadline written into a column, here it is a name the caller
// already has to be able to render as unknown.
//
// AN ERROR, HOWEVER, IS NEVER "no name". A caller that renders an outage as a blank writes an
// absence onto an administrative record that a person then reads as fact. Say the names could not be
// loaded, or fail the page.
//
// # NOTHING HERE LOGS A NAME
//
// Not the response, not an item, not at debug level. The generated String() prints full_name in full
// and `debug_redact` is a no-op in protobuf-go v1.36.12, so a single `"ra", ra` would put a
// commune's staff names into the log pipeline, from where they cannot be recalled (rule 3,
// forbidden #1). Every field logged below is chosen one at a time for that reason.
func (c *Client) TenCanBoTheoMa(ctx context.Context, ma []string) (map[string]TenCanBo, error) {
	// AN EMPTY REQUEST IS ANSWERED WITHOUT A ROUND TRIP, and it is NOT an error: a list page with no
	// rows legitimately produces no codes. It is also not "give me everybody" — there is no spelling
	// of this request that means that, and this early return is the near end of the property the
	// contract states (ADR 0034).
	if len(ma) == 0 {
		return map[string]TenCanBo{}, nil
	}
	if len(ma) > TranMaMotLo {
		return nil, fmt.Errorf(
			"identityclient: TenCanBoTheoMa nhận %d mã, vượt trần %d — bên gọi phải tự chia lô, không được cắt bớt",
			len(ma), TranMaMotLo)
	}

	// The set of codes actually asked for, so the response can be checked against it below. Built
	// before the call because that is the only moment this side knows what it asked.
	daHoi := make(map[string]struct{}, len(ma))
	for _, m := range ma {
		if m == "" {
			// A blank code matches no row and only widens the array the database scans. Dropped
			// rather than sent — and dropping it here means a request of nothing but blanks never
			// reaches the network at all.
			continue
		}
		daHoi[m] = struct{}{}
	}
	if len(daHoi) == 0 {
		return map[string]TenCanBo{}, nil
	}

	// THE SAME DEADLINE AS ResolveStaff AND HanXuLy, on purpose: two numbers would make the observed
	// timeout depend on which dependency was slow. This is a keyed read of at most 200 rows and it
	// runs once per screen that shows who did something.
	ctx, huy := context.WithTimeout(ctx, HanGoi)
	defer huy()

	ra, err := c.cl.ResolveStaffNames(ctx, &identityv1.ResolveStaffNamesRequest{Ma: ma})
	if err != nil {
		// Warn, not Error: this service is healthy and answering. The gRPC code is here because it is
		// how an operator tells an outage (UNAVAILABLE, DeadlineExceeded) from a misconfiguration
		// (Unauthenticated: the caller key; InvalidArgument: the contract or the ceiling). NO CODE AND
		// NO NAME IS LOGGED — a staff code is the key to a person, and the response holds the person.
		c.log.WarnContext(ctx, "CẢNH BÁO: không tra được tên cán bộ cho hồ sơ",
			"ma_loi", status.Code(err).String(), "so_ma", len(daHoi), "err", err)
		return nil, fmt.Errorf("identityclient: ResolveStaffNames: %w", err)
	}

	ten := make(map[string]TenCanBo, len(daHoi))
	for _, it := range ra.GetItems() {
		m := it.GetMa()
		if m == "" {
			// An item that does not say WHICH code it answers cannot be joined onto any line. A
			// contract fault, and worth as much noise as an outage: silently dropped, it would render
			// as a blank on an archival record.
			return nil, fmt.Errorf("identityclient: ResolveStaffNames trả về một mục không mang mã cán bộ")
		}
		if _, co := daHoi[m]; !co {
			// A CODE NOBODY ASKED FOR. This is anti-enumeration property 3 (ADR 0034) checked from the
			// near end: the response must never teach a caller that a staff code it did not hold
			// exists. Refused rather than ignored, because the day this fires the far end has grown a
			// behaviour the contract forbids, and a silent drop would hide it.
			return nil, fmt.Errorf(
				"identityclient: ResolveStaffNames trả về một mã KHÔNG ĐƯỢC HỎI — phản hồi không được mang khoá chưa hỏi")
		}
		if it.GetFullName() == "" {
			// `nguoi_dung.ho_ten` is NOT NULL, so an empty name is a contract fault and never "this
			// person has no name". Rendered, it is an unattributed line on an administrative record.
			return nil, fmt.Errorf("identityclient: ResolveStaffNames trả về một mục không có họ tên")
		}
		if it.GetStanding() == identityv1.StaffRecordStanding_STAFF_RECORD_STANDING_UNSPECIFIED {
			// The server answers this from one nullable column it has already read, so there is no
			// third outcome to represent. Passed through, it would reach a screen as neither "trong
			// danh bạ" nor "đã gỡ", and somebody would pick a default.
			return nil, fmt.Errorf(
				"identityclient: ResolveStaffNames trả về một mục không nói rõ bản ghi còn trong danh bạ hay không")
		}
		ten[m] = TenCanBo{HoTen: it.GetFullName(), TrangThai: it.GetStanding()}
	}

	// NO COMPLETENESS CHECK, AND ITS ABSENCE IS THE DECISION. A code asked for and not answered is
	// ORDINARY here — it belongs to no row, or to another commune — unlike every other method in this
	// package. The caller maps by code and renders what it did not get as unknown; it must never
	// render it as an empty name.
	return ten, nil
}
