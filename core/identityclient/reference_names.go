package identityclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
)

// MaxLookupKeysPerCall is the ceiling on ONE ResolveOrgUnitNames, ResolveTaskBlocLabels or
// ResolveLiveOrgUnitCodes call.
//
// IT MIRRORS THE CONTRACT (identity.proto, each request message), the source of truth (rule 2,
// invariant 7), and it is TranMaMotLo's number on purpose: a caller printing the task register
// batches its unit ids, bloc codes and staff codes identically.
//
// REFUSED HERE RATHER THAN SENT, AND NEVER TRUNCATED: a clamped batch prints real rows blank on an
// archival register, or rejects real import rows as "unknown unit", with nothing reporting it. The
// caller chunks.
const MaxLookupKeysPerCall = 200

// OrgUnitName is one resolved unit name, as a caller of this package sees it.
//
// THE STANDING IS KEPT AS THE CONTRACT'S ENUM, not reduced to a bool — the reason TenCanBo gives: a
// zero value must be loud (UNSPECIFIED), never silently one of the two real answers.
type OrgUnitName struct {
	// Name is `bo_phan.ten` as it stands TODAY — not necessarily the name the unit had when the
	// record was filed (the contract says so). Not personal data.
	Name     string
	Standing identityv1.RecordStanding
}

// TaskBlocLabel is one resolved task-bloc label (`Khối Uỷ ban`), as a caller of this package sees it.
type TaskBlocLabel struct {
	Label    string
	Standing identityv1.RecordStanding
}

// OrgUnitNames asks identity for the names behind org-unit ids (`bo_phan.id`) an ALREADY-STORED
// record carries — INCLUDING units since removed from the org chart, flagged
// RECORD_STANDING_REMOVED. Built for the task register printout (docs/ui-ux/02-nhiem-vu.md §4.3).
//
// # IT RESOLVES A NAME. THE CALLER MUST DECIDE NOTHING FROM IT
//
// A present item may be present PRECISELY BECAUSE the unit was removed. "May this unit hold work"
// is LiveOrgUnits' question.
//
// # AN ABSENT ID IS ORDINARY. AN ERROR IS NOT AN ABSENT ID
//
// Fewer names than ids is a normal answer (unknown id, or another commune's — indistinguishable).
// An error means the call did not happen: fail the export (503, retryable when
// errors.Is(err, ErrIdentityUnavailable)) — never print blanks, which a reader takes as fact.
func (c *Client) OrgUnitNames(ctx context.Context, ids []string) (map[string]OrgUnitName, error) {
	asked, err := lookupKeys("OrgUnitNames", ids)
	if err != nil {
		return nil, err
	}
	if len(asked) == 0 {
		return map[string]OrgUnitName{}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	resp, err := c.cl.ResolveOrgUnitNames(ctx, &identityv1.ResolveOrgUnitNamesRequest{Ids: keysOf(asked)})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không tra được tên bộ phận",
			"ma_loi", status.Code(err).String(), "so_bo_phan", len(asked), "err", err)
		return nil, wrapCallError("ResolveOrgUnitNames", err, nil)
	}

	out := make(map[string]OrgUnitName, len(asked))
	for _, it := range resp.GetItems() {
		id := it.GetId()
		if _, ok := asked[id]; !ok {
			// Includes "". Anti-enumeration property 3 checked from the near end.
			return nil, fmt.Errorf(
				"identityclient: ResolveOrgUnitNames trả về một bộ phận KHÔNG ĐƯỢC HỎI — phản hồi không được mang khoá chưa hỏi")
		}
		if it.GetName() == "" {
			// `bo_phan.ten` is NOT NULL: an empty name is a contract fault, never "this unit has none".
			return nil, fmt.Errorf("identityclient: ResolveOrgUnitNames trả về một mục không có tên bộ phận")
		}
		if it.GetStanding() == identityv1.RecordStanding_RECORD_STANDING_UNSPECIFIED {
			return nil, fmt.Errorf(
				"identityclient: ResolveOrgUnitNames trả về một mục không nói rõ bộ phận còn hay đã gỡ")
		}
		out[id] = OrgUnitName{Name: it.GetName(), Standing: it.GetStanding()}
	}
	// NO COMPLETENESS CHECK, and its absence is the decision: an unanswered id is rendered unknown.
	return out, nil
}

// TaskBlocLabels asks identity for the labels behind task-bloc codes (`khoi_nhiem_vu.ma`) an
// ALREADY-STORED record carries — INCLUDING codes since removed from the catalogue, flagged. Same
// discipline as OrgUnitNames: it decides nothing, absence is ordinary, an error is never "no label".
func (c *Client) TaskBlocLabels(ctx context.Context, codes []string) (map[string]TaskBlocLabel, error) {
	asked, err := lookupKeys("TaskBlocLabels", codes)
	if err != nil {
		return nil, err
	}
	if len(asked) == 0 {
		return map[string]TaskBlocLabel{}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	resp, err := c.cl.ResolveTaskBlocLabels(ctx, &identityv1.ResolveTaskBlocLabelsRequest{Ma: keysOf(asked)})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không tra được nhãn khối nhiệm vụ",
			"ma_loi", status.Code(err).String(), "so_ma", len(asked), "err", err)
		return nil, wrapCallError("ResolveTaskBlocLabels", err, nil)
	}

	out := make(map[string]TaskBlocLabel, len(asked))
	for _, it := range resp.GetItems() {
		ma := it.GetMa()
		if _, ok := asked[ma]; !ok {
			return nil, fmt.Errorf(
				"identityclient: ResolveTaskBlocLabels trả về một mã KHÔNG ĐƯỢC HỎI — phản hồi không được mang khoá chưa hỏi")
		}
		if it.GetLabel() == "" {
			return nil, fmt.Errorf("identityclient: ResolveTaskBlocLabels trả về một mục không có nhãn")
		}
		if it.GetStanding() == identityv1.RecordStanding_RECORD_STANDING_UNSPECIFIED {
			return nil, fmt.Errorf(
				"identityclient: ResolveTaskBlocLabels trả về một mục không nói rõ khối còn hay đã gỡ")
		}
		out[ma] = TaskBlocLabel{Label: it.GetLabel(), Standing: it.GetStanding()}
	}
	return out, nil
}

// LiveOrgUnitIDsByCode translates org-unit business codes (`bo_phan.ma`, the slug a clerk types into
// an import spreadsheet) into the ids of the LIVE units carrying them, in the commune the context
// carries. Returns code → `bo_phan.id`.
//
// # THE CALLER DECIDES FROM THIS
//
// The predicate is LiveOrgUnits' (live = not soft deleted), so a returned id may be written as a
// task's unit. A code ABSENT from the map MUST be refused — the import row is rejected, never written
// with a blank unit and never matched by name instead. Unknown, removed and another commune's code
// are deliberately one answer.
//
// # AN ERROR IS NEVER "unknown unit"
//
// It means the check did not happen: fail the import (retryable when
// errors.Is(err, ErrIdentityUnavailable)); never reject rows as unknown, never write them unchecked.
func (c *Client) LiveOrgUnitIDsByCode(ctx context.Context, codes []string) (map[string]string, error) {
	asked, err := lookupKeys("LiveOrgUnitIDsByCode", codes)
	if err != nil {
		return nil, err
	}
	if len(asked) == 0 {
		return map[string]string{}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	resp, err := c.cl.ResolveLiveOrgUnitCodes(ctx, &identityv1.ResolveLiveOrgUnitCodesRequest{Ma: keysOf(asked)})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không tra được bộ phận theo mã",
			"ma_loi", status.Code(err).String(), "so_ma", len(asked), "err", err)
		return nil, wrapCallError("ResolveLiveOrgUnitCodes", err, nil)
	}

	out := make(map[string]string, len(asked))
	for _, it := range resp.GetItems() {
		ma, id := it.GetMa(), it.GetId()
		if _, ok := asked[ma]; !ok {
			// The caller is about to WRITE from this map: a key it did not ask for is refused, not ignored.
			return nil, fmt.Errorf(
				"identityclient: ResolveLiveOrgUnitCodes trả về một mã KHÔNG ĐƯỢC HỎI — phản hồi phải là tập con của yêu cầu")
		}
		if id == "" {
			// Written, an empty id is a task held by no unit.
			return nil, fmt.Errorf("identityclient: ResolveLiveOrgUnitCodes trả về một mục không có mã định danh bộ phận")
		}
		if prev, dup := out[ma]; dup && prev != id {
			// `(tenant_id, ma)` is unique, so one code has one unit. Two answers means the far end is
			// wrong, and picking either writes a guess onto a record.
			return nil, fmt.Errorf("identityclient: ResolveLiveOrgUnitCodes trả hai bộ phận khác nhau cho cùng một mã")
		}
		out[ma] = id
	}
	// NO COMPLETENESS CHECK: an unanswered code is the answer "no live unit carries it".
	return out, nil
}

// lookupKeys applies the shared request rules of the three lookups above: refuse over the ceiling
// (counted on what the caller passed, before dedup — the server counts the same way), drop blanks,
// collapse duplicates. An empty result means "answer empty without a round trip" — never "all".
func lookupKeys(method string, keys []string) (map[string]struct{}, error) {
	if len(keys) > MaxLookupKeysPerCall {
		return nil, fmt.Errorf(
			"identityclient: %s nhận %d khoá, vượt trần %d — bên gọi phải tự chia lô, không được cắt bớt",
			method, len(keys), MaxLookupKeysPerCall)
	}
	asked := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		if k == "" {
			continue
		}
		asked[k] = struct{}{}
	}
	return asked, nil
}

func keysOf(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}
