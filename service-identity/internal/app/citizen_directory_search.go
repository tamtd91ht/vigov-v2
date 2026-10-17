package app

// The directory SEARCH with dynamic-attribute filters (ADR 0101 #7, #10; task card E-1). Where it runs:
//
//	index configured and on       Elasticsearch → ids + order → PostgreSQL scoped store → masked page
//	index NOT configured (dev)    keyword / structural filters on PostgreSQL as before; attribute and
//	                              created-date filters → ErrCitizenSearchUnavailable (503)
//	index configured but off      (SEARCH_HASH_KEY mismatch, cluster down) → 503 for every search, never
//	                              a PostgreSQL answer that quietly differs (owner, 17/10/2026)
//	"Chỉ người có Zalo" set       PostgreSQL only: it is a join to live citizen sessions the index does not
//	                              hold. Combined with an index-only filter → ErrCitizenSearchCombination
//
// THE COMMUNE IS NEVER IN THE QUERY: the index takes it from the context (search.CitizenIndex), the store
// from store.DB.For(ctx) — and ListByIDs re-reads only this commune's live rows, so an id the index returns
// for any other reason is simply not shown (rule 1).

import (
	"context"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/search"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// MaxSearchFilterValues bounds the values of one option filter (the option list itself is ≤ 100).
const MaxSearchFilterValues = domain.MaxCitizenFieldOptions

// AttributeOptionFilter keeps contacts holding ANY of Values of one combobox / singleBox field.
type AttributeOptionFilter struct {
	Code   string
	Values []string
}

// AttributeRangeFilter keeps contacts whose number / dateTime value lies in [From, To] (inclusive; one
// bound is enough — ADR 0101 #10).
type AttributeRangeFilter struct {
	Code     string
	From, To *float64
}

// CitizenContactSearch is one search: the existing structural filter and keyword, plus what only the index
// answers.
type CitizenContactSearch struct {
	Filter      idstore.CitizenContactFilter
	CreatedFrom *int64 // epoch millis, inclusive
	CreatedTo   *int64 // epoch millis, inclusive
	Options     []AttributeOptionFilter
	Ranges      []AttributeRangeFilter
}

func (q CitizenContactSearch) needsIndex() bool {
	return q.CreatedFrom != nil || q.CreatedTo != nil || len(q.Options) > 0 || len(q.Ranges) > 0
}

// plan decides where the search runs and, for the index, builds its query (filters checked against the
// commune's LIVE definitions).
func (uc *CitizenDirectory) plan(ctx context.Context, q CitizenContactSearch) (bool, search.Query, error) {
	needs := q.needsIndex()
	sq := search.Query{
		Keyword: q.Filter.Keyword, GroupID: q.Filter.GroupID, ResidentialUnitID: q.Filter.ResidentialUnitID,
		ActiveOnly: q.Filter.ActiveOnly, CreatedFrom: q.CreatedFrom, CreatedTo: q.CreatedTo,
	}
	if needs {
		if q.CreatedFrom != nil && q.CreatedTo != nil && *q.CreatedFrom > *q.CreatedTo {
			return false, search.Query{}, fmt.Errorf("%w: ngày tạo — mốc đầu sau mốc cuối", ErrCitizenSearchFilterInvalid)
		}
		// The store scopes this read by tenant_id through store.DB.For(ctx).
		defs, err := uc.fields.List(ctx, false)
		if err != nil {
			return false, search.Query{}, err
		}
		byCode := make(map[string]domain.CitizenField, len(defs))
		for _, d := range defs {
			byCode[d.Code] = d
		}
		for _, o := range q.Options {
			d, ok := byCode[o.Code]
			if !ok || !d.Type.HasOptions() || len(o.Values) == 0 || len(o.Values) > MaxSearchFilterValues {
				return false, search.Query{}, fmt.Errorf("%w: %q", ErrCitizenSearchFilterInvalid, o.Code)
			}
			known := make(map[string]bool, len(d.Options))
			for _, opt := range d.Options {
				known[opt.Value] = true
			}
			for _, v := range o.Values {
				if !known[v] {
					return false, search.Query{}, fmt.Errorf("%w: %s — lựa chọn không có", ErrCitizenSearchFilterInvalid, d.Label)
				}
			}
			sq.Options = append(sq.Options, search.OptionFilter{Code: o.Code, Values: o.Values})
		}
		for _, r := range q.Ranges {
			d, ok := byCode[r.Code]
			if !ok || (d.Type != domain.FieldNumber && d.Type != domain.FieldDateTime) || (r.From == nil && r.To == nil) ||
				(r.From != nil && r.To != nil && *r.From > *r.To) {
				return false, search.Query{}, fmt.Errorf("%w: %q", ErrCitizenSearchFilterInvalid, r.Code)
			}
			sq.Ranges = append(sq.Ranges, search.RangeFilter{Code: r.Code, From: r.From, To: r.To})
		}
	}
	if q.Filter.HasOwnAppSession != nil {
		if needs {
			return false, search.Query{}, ErrCitizenSearchCombination
		}
		return false, sq, nil
	}
	if uc.index == nil {
		if needs {
			return false, search.Query{}, ErrCitizenSearchUnavailable
		}
		return false, sq, nil
	}
	if err := uc.index.Ready(); err != nil {
		return false, search.Query{}, fmt.Errorf("%w: %w", ErrCitizenSearchUnavailable, err)
	}
	return true, sq, nil
}

// Search reads one page of the directory under q, masked.
func (uc *CitizenDirectory) Search(ctx context.Context, q CitizenContactSearch, req page.Request) (page.Result[CitizenContactListItem], error) {
	useIndex, sq, err := uc.plan(ctx, q)
	if err != nil {
		return page.NewResult[CitizenContactListItem](), err
	}
	if !useIndex {
		// PostgreSQL: the store scopes this read by tenant_id through store.DB.For(ctx).
		return uc.List(ctx, q.Filter, req)
	}
	var after *search.After
	if a, ok := req.After(); ok {
		after = &search.After{CreatedAt: a.Key.Time().UnixMilli(), ContactID: a.ID}
	}
	// The index filters by the context's tenant_id on every query (search.CitizenIndex).
	p, err := uc.index.Search(ctx, sq, req.Limit(), after)
	if err != nil {
		return page.NewResult[CitizenContactListItem](), fmt.Errorf("%w: %w", ErrCitizenSearchUnavailable, err)
	}
	// The store scopes this read by tenant_id through store.DB.For(ctx); rows removed since indexing drop out.
	rows, err := uc.contacts.ListByIDs(ctx, p.IDs)
	if err != nil {
		return page.NewResult[CitizenContactListItem](), err
	}
	items, err := uc.maskListItems(ctx, rows)
	if err != nil {
		return page.NewResult[CitizenContactListItem](), err
	}
	out := page.Result[CitizenContactListItem]{Items: items, HasMore: p.HasMore}
	if p.Next != nil {
		// The cursor anchors on the LAST HIT of the index (even if its row dropped out of the page), so the
		// next page continues where the index stopped. Millisecond precision: the index's created_at.
		out.NextCursor = page.Encode(req.Column(), req.Dir(), page.Anchor{
			Key: page.TimeKey(time.UnixMilli(p.Next.CreatedAt).UTC()), ID: p.Next.ContactID,
		})
	}
	return out, nil
}

// CountSearch counts what Search would walk — the same plan, so the total and the list it heads agree.
func (uc *CitizenDirectory) CountSearch(ctx context.Context, q CitizenContactSearch) (int, error) {
	useIndex, sq, err := uc.plan(ctx, q)
	if err != nil {
		return 0, err
	}
	if !useIndex {
		// PostgreSQL: the store scopes this count by tenant_id through store.DB.For(ctx).
		return uc.Count(ctx, q.Filter)
	}
	// The index filters by the context's tenant_id on every query (search.CitizenIndex).
	n, err := uc.index.Count(ctx, sq)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrCitizenSearchUnavailable, err)
	}
	return n, nil
}
