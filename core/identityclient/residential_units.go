package identityclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
)

// The two residential-unit lookups (identity.proto, ADR 0088). Both are keyed by `thon_to_dan_pho.id`
// — the value service-petitions stores in `phieu_phan_anh.thon_id` — and share lookupKeys' request
// rules: MaxLookupKeysPerCall refused locally, blanks dropped, duplicates collapsed, an empty request
// answered without a round trip and never "every unit". Ids are not personal data; only counts are
// logged anyway.

// ResidentialUnitName is one resolved unit name, as a caller of this package sees it. The standing is
// kept as the contract's enum for the reason OrgUnitName gives: a zero value must be loud.
type ResidentialUnitName struct {
	// Name is `thon_to_dan_pho.ten` as it stands TODAY. Not for storing.
	Name     string
	Standing identityv1.RecordStanding
}

// ActiveResidentialUnits asks identity which of these unit ids are ACTIVE units — live AND in use —
// of the commune the context carries. Returns id → today's name.
//
// # THE CALLER DECIDES FROM THIS
//
// An id ABSENT from the map MUST be refused (400 naming the field): unknown, out of use, soft deleted
// and another commune's unit are deliberately one answer. Never drop the hamlet silently, never write
// the id unchecked. The name answers the request; the record stores the ID.
//
// # AN ERROR IS NEVER "not active"
//
// It means the check did not happen: refuse the write — 503, retryable when
// errors.Is(err, ErrIdentityUnavailable) — never fall back to writing or dropping the id.
func (c *Client) ActiveResidentialUnits(ctx context.Context, ids []string) (map[string]string, error) {
	asked, err := lookupKeys("ActiveResidentialUnits", ids)
	if err != nil {
		return nil, err
	}
	if len(asked) == 0 {
		return map[string]string{}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	resp, err := c.cl.ResolveActiveResidentialUnits(ctx,
		&identityv1.ResolveActiveResidentialUnitsRequest{Ids: keysOf(asked)})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không kiểm được thôn / tổ dân phố đang dùng",
			"ma_loi", status.Code(err).String(), "so_thon", len(asked), "err", err)
		return nil, wrapCallError("ResolveActiveResidentialUnits", err, nil)
	}

	out := make(map[string]string, len(asked))
	for _, it := range resp.GetItems() {
		id := it.GetId()
		if _, ok := asked[id]; !ok {
			// Includes "". The caller is about to WRITE from this map: refused, not ignored.
			return nil, fmt.Errorf(
				"identityclient: ResolveActiveResidentialUnits trả về một thôn KHÔNG ĐƯỢC HỎI — phản hồi phải là tập con của yêu cầu")
		}
		if it.GetName() == "" {
			// `ten` is NOT NULL: an empty name is a contract fault, never "this unit has none".
			return nil, fmt.Errorf("identityclient: ResolveActiveResidentialUnits trả về một mục không có tên thôn")
		}
		out[id] = it.GetName()
	}
	// NO COMPLETENESS CHECK: an unanswered id is the answer "not active".
	return out, nil
}

// ResidentialUnitNames asks identity for the names behind unit ids an ALREADY-STORED record carries —
// INCLUDING units since taken out of use (LIVE) or removed (RECORD_STANDING_REMOVED).
//
// IT RESOLVES A NAME. THE CALLER MUST DECIDE NOTHING FROM IT: an id may resolve precisely because the
// record is old. "May this unit be written" is ActiveResidentialUnits' question.
//
// Fewer names than ids is ordinary (unknown id, or another commune's). An error means the call did
// not happen: fail the view/export/report (503, retryable when errors.Is(err, ErrIdentityUnavailable))
// — never print blank areas, which a reader takes as fact.
//
// At most MaxLookupKeysPerCall ids; a report over more uses ResidentialUnitNamesInBatches.
func (c *Client) ResidentialUnitNames(ctx context.Context, ids []string) (map[string]ResidentialUnitName, error) {
	asked, err := lookupKeys("ResidentialUnitNames", ids)
	if err != nil {
		return nil, err
	}
	if len(asked) == 0 {
		return map[string]ResidentialUnitName{}, nil
	}
	return c.residentialUnitNames(ctx, asked)
}

// ResidentialUnitNamesInBatches is ResidentialUnitNames for any number of ids: it collapses
// duplicates and blanks, then asks in calls of at most MaxLookupKeysPerCall DISTINCT ids and merges
// the answers.
//
// ALL OR NOTHING: if any call fails, the whole lookup fails and no partial map is returned — a report
// with half its areas blank is the absence-written-onto-a-record the contract forbids.
func (c *Client) ResidentialUnitNamesInBatches(ctx context.Context, ids []string) (map[string]ResidentialUnitName, error) {
	distinct := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		distinct = append(distinct, id)
	}

	out := make(map[string]ResidentialUnitName, len(distinct))
	for start := 0; start < len(distinct); start += MaxLookupKeysPerCall {
		end := min(start+MaxLookupKeysPerCall, len(distinct))
		batch := make(map[string]struct{}, end-start)
		for _, id := range distinct[start:end] {
			batch[id] = struct{}{}
		}
		got, err := c.residentialUnitNames(ctx, batch)
		if err != nil {
			return nil, err
		}
		for id, n := range got {
			out[id] = n
		}
	}
	return out, nil
}

// residentialUnitNames is one call over an already-deduplicated, non-empty, within-ceiling key set,
// with the response checked against exactly that set.
func (c *Client) residentialUnitNames(ctx context.Context, asked map[string]struct{}) (map[string]ResidentialUnitName, error) {
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	resp, err := c.cl.ResolveResidentialUnitNames(ctx,
		&identityv1.ResolveResidentialUnitNamesRequest{Ids: keysOf(asked)})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không tra được tên thôn / tổ dân phố",
			"ma_loi", status.Code(err).String(), "so_thon", len(asked), "err", err)
		return nil, wrapCallError("ResolveResidentialUnitNames", err, nil)
	}

	out := make(map[string]ResidentialUnitName, len(asked))
	for _, it := range resp.GetItems() {
		id := it.GetId()
		if _, ok := asked[id]; !ok {
			// Includes "". Anti-enumeration: the answer is a subset of the question.
			return nil, fmt.Errorf(
				"identityclient: ResolveResidentialUnitNames trả về một thôn KHÔNG ĐƯỢC HỎI — phản hồi không được mang khoá chưa hỏi")
		}
		if it.GetName() == "" {
			return nil, fmt.Errorf("identityclient: ResolveResidentialUnitNames trả về một mục không có tên thôn")
		}
		if it.GetStanding() == identityv1.RecordStanding_RECORD_STANDING_UNSPECIFIED {
			return nil, fmt.Errorf(
				"identityclient: ResolveResidentialUnitNames trả về một mục không nói rõ thôn còn hay đã gỡ")
		}
		out[id] = ResidentialUnitName{Name: it.GetName(), Standing: it.GetStanding()}
	}
	// NO COMPLETENESS CHECK: an unanswered id is rendered unknown.
	return out, nil
}
