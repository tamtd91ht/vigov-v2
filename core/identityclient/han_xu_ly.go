package identityclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
)

// HanXuLy asks identity for the deadlines of ONE administrative record, in ONE commune.
//
// IT IS THE ONLY CORRECT WAY TO PRODUCE A DEADLINE FROM A COMMUNE'S SLA TABLE, and it exists
// because the two halves of that calculation both live in identity: `sla` says HOW LONG in working
// hours (migration 0008, ADR 0029), the three calendar tables say WHEN the commune is open
// (migration 0006, ADR 0007). Neither is reachable from another service — rule 2, forbidden #2
// forbids the database, and rule 2, forbidden #1 forbids the package.
//
// # WHY NOT TienGioLamViec, WHICH IS RIGHT THERE
//
// That method takes the number of hours FROM THE CALLER. Using it means `petitions` and
// `documents` each fetching their own hours and each deciding what to do when a commune has
// configured none — two implementations of "what is this commune's commitment", which is the shape
// ADR 0029 §123 asked about and the user closed on 2026-09-22 in favour of this one. TienGioLamViec
// stays for a caller that genuinely holds its own number of hours and is not reading `sla`.
//
// # THE ONE OBLIGATION, AND IT IS THE EASY ONE TO BREAK
//
// CALL IT ONCE, AT THE ACT THAT FIXES THE CLOCK, AND STORE WHAT IT RETURNS (rule 10, invariant 2;
// ADR 0028 decision E). `han_tiep_nhan` is fixed when the row is created; `han_xu_ly_xong` is fixed
// when a member of staff settles the field. Calling this again later to "re-derive" a deadline that
// is already stored silently moves a commitment already made to a citizen — the configuration may
// have changed since, and ADR 0007 decision 6 fixes that such a change is NOT retroactive.
//
// # AN ERROR MEANS FAIL THE INTAKE. IT NEVER MEANS "no deadline"
//
// The ordinary error today is FAILED_PRECONDITION, because `sla` is empty for every commune —
// migration 0008 seeds nothing and the onboarding step does not exist. THAT IS THE CONTRACT
// WORKING, not a bug to route around:
//
//	DO NOT fall back to a number. Not 24 hours, not the sixteen rows of the specification's own
//	table. A commitment invented by software is still told to a citizen as though the authority
//	made it (rule 10, forbidden #3).
//
//	DO NOT store the record without a deadline. A row with no deadline is a commitment nobody
//	counts, while every on-time/overdue report counts it anyway —
//	kb/30-indexes/transaction-boundaries.json, `tinh_han_xu_ly_luc_tiep_nhan`.
//
//	DO NOT add hours locally. `hooks/citizen_commitment_guard.py` blocks the shape outside
//	service-identity, and rule 10, forbidden #2 is why: adding a duration walks straight through
//	nights, weekends, `ngay_nghi_le` and `ngay_lam_bu` while looking like it respected the unit.
//
// # KEYED BY THE CLOCK, NEVER BY INDEX
//
// The contract collapses duplicates, so a caller asking for the same clock twice gets one item. A
// caller reading `items[0]` and `items[1]` would silently take one commitment for the other — and
// both are instants, so nothing downstream could tell.
//
// THERE IS NO PARTIAL SUCCESS. A clock asked for and not answered is a contract fault, not a
// missing optional: it would otherwise reach the caller as a zero `time.Time`, i.e. a deadline in
// year 1, i.e. a record overdue the moment it is created.
func (c *Client) HanXuLy(ctx context.Context, loaiViec identityv1.WorkKind, linhVuc string,
	tuLuc time.Time, can []identityv1.DeadlineKind) (map[identityv1.DeadlineKind]time.Time, error) {

	// REFUSED LOCALLY, AND EACH MESSAGE NAMES THE CAUSE RATHER THAN THE SYMPTOM. Sent, every one of
	// these comes back as an INVALID_ARGUMENT whose text describes a wire field — and an operator
	// reading that goes looking at the contract for a fault that is in this service's own wiring.
	if loaiViec == identityv1.WorkKind_WORK_KIND_UNSPECIFIED {
		return nil, fmt.Errorf(
			"identityclient: HanXuLy không có loại việc — bên gọi phải nói rõ van-ban-den, phan-anh hay nhiem-vu")
	}
	if tuLuc.IsZero() {
		// A zero instant sent would come back as a horizon failure about the year 1, and an operator
		// reading that would inspect the commune's calendar for a fault that is not there.
		return nil, fmt.Errorf(
			"identityclient: HanXuLy với gốc đếm rỗng — bên gọi chưa đặt thời điểm bắt đầu đếm")
	}
	if len(can) == 0 {
		// There is deliberately no "give me both" default here either. ADR 0028 decision E fixes the
		// two clocks at two different acts, and a caller that has not said which act it is performing
		// has not decided what it is storing.
		return nil, fmt.Errorf(
			"identityclient: HanXuLy không nói rõ đang ấn định đồng hồ nào (ADR 0028 quyết định E)")
	}
	for _, k := range can {
		if k == identityv1.DeadlineKind_DEADLINE_KIND_UNSPECIFIED {
			return nil, fmt.Errorf(
				"identityclient: HanXuLy có một đồng hồ chưa xác định — giá trị 0 của enum không phải một lựa chọn")
		}
	}

	// THE SAME DEADLINE AS ResolveStaff AND TienGioLamViec, on purpose: two numbers would make the
	// observed timeout depend on which dependency was slow. This is a read of configuration — one
	// small table plus at most two years of calendar rows, all indexed — and it runs ONCE per
	// record, at the act that fixes a clock, never per request and never per row.
	ctx, huy := context.WithTimeout(ctx, HanGoi)
	defer huy()

	ra, err := c.cl.ResolveDeadlines(ctx, &identityv1.ResolveDeadlinesRequest{
		WorkKind: loaiViec,
		// "" IS A REAL REQUEST and the ordinary one on the citizen channel: at the instant a
		// petition is created nobody knows the field yet, so the default row is what applies
		// (ADR 0028, decision E). It is passed straight through — never substituted, never defaulted
		// to some "general" code.
		LinhVuc:   linhVuc,
		CountFrom: timestamppb.New(tuLuc),
		Deadlines: can,
	})
	if err != nil {
		// Warn, not Error: this service is healthy and answering. Logged with the gRPC code because
		// that is how an operator tells the three apart — FAILED_PRECONDITION means "nobody has
		// filled in this commune's SLA or working hours", UNAVAILABLE means identity is down,
		// InvalidArgument/Unauthenticated mean the contract or the caller key is wrong. All of them
		// fail the intake; only one of them is fixed on a configuration screen.
		//
		// NOTHING PERSONAL IS LOGGED AND NOTHING PERSONAL IS IN THE REQUEST. `linh_vuc` is a
		// platform code and `count_from` is an instant; there is no citizen and no record here.
		c.log.WarnContext(ctx, "CẢNH BÁO: không lấy được hạn xử lý của xã",
			"ma_loi", status.Code(err).String(), "loai_viec", loaiViec.String(), "err", err)
		return nil, fmt.Errorf("identityclient: ResolveDeadlines: %w", err)
	}

	han := make(map[identityv1.DeadlineKind]time.Time, len(can))
	for _, it := range ra.GetItems() {
		k := it.GetKind()
		if k == identityv1.DeadlineKind_DEADLINE_KIND_UNSPECIFIED {
			// An item that does not say WHICH clock it is cannot be stored in any column. It is a
			// contract fault and worth as much noise as an outage.
			return nil, fmt.Errorf("identityclient: ResolveDeadlines trả về một hạn không nói rõ đồng hồ nào")
		}
		t := it.GetDueAt()
		if t == nil {
			// NEVER mapped to the zero time.Time. That value is a deadline in year 1, which every
			// overdue report reads as "already late" — and a nil here would reach the database as a
			// real column value.
			return nil, fmt.Errorf(
				"identityclient: ResolveDeadlines trả về hạn rỗng cho đồng hồ %s", k.String())
		}
		han[k] = t.AsTime()
	}

	for _, k := range can {
		if _, co := han[k]; !co {
			// The two ends disagree about what a complete answer is, and every deadline produced from
			// here on would be missing whichever clock went absent. There is no partial success.
			c.log.WarnContext(ctx, "CẢNH BÁO HỢP ĐỒNG: ResolveDeadlines thiếu một hạn đã hỏi",
				"moc", k.String())
			return nil, fmt.Errorf(
				"identityclient: ResolveDeadlines không trả hạn cho đồng hồ %s — không có thành công một phần",
				k.String())
		}
	}

	return han, nil
}
