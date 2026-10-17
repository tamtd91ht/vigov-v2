package app

// The CONSUMER side of the directory search (ADR 0101 #11): identity reads its own
// `identity.citizen_contact_changed.v1` events (group <ENV>.identity.citizen-search) and makes the
// Elasticsearch document equal to the PostgreSQL row — and the per-commune / every-commune reindex.
//
// IDEMPOTENT BY CONSTRUCTION (rule 2, invariant 5): the handler never trusts the event beyond the contact
// id. It re-reads the live row of the commune in context: found → the document is written from it; not
// found (removed, or never this commune's) → the document is taken out of the index. Replaying,
// duplicating or reordering events converges on the row as it is now. The ROW is never touched here.
//
// NOTHING IS LOGGED but ids and counts (rule 3): the row read here holds a name and a number.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/vihat/vigov/core/events"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/search"
)

// reindexBatch is how many contacts one bulk request carries.
const reindexBatch = 500

// IndexerContacts is what the indexer reads from the directory store. *idstore.CitizenDirectoryStore.
type IndexerContacts interface {
	Get(ctx context.Context, id string) (domain.CitizenContact, bool, error)
	GroupIDsOf(ctx context.Context, contactIDs []string) (map[string][]string, error)
	LiveIDsAfter(ctx context.Context, after string, limit int) ([]string, error)
}

// IndexWriter is the write half of the index. *search.CitizenIndex. Every method takes the commune from
// the context (tenant_id: routing, filter and document id prefix).
type IndexWriter interface {
	Ready() error
	BuildDoc(t tenant.ID, c domain.CitizenContact, groupIDs []string, defs []domain.CitizenField) search.Doc
	Upsert(ctx context.Context, d search.Doc) error
	BulkUpsert(ctx context.Context, docs []search.Doc) error
	Unindex(ctx context.Context, contactID string) error
	DeleteCommune(ctx context.Context) error
	DeleteEverything(ctx context.Context) error
	AdoptKey(ctx context.Context) error
}

// CommunesWithContacts lists the communes to rebuild. *crosstenant.EventOutboxCommunes.
type CommunesWithContacts interface {
	CommunesWithContacts(ctx context.Context) ([]tenant.ID, error)
}

// CitizenSearchIndexer keeps the index equal to PostgreSQL.
type CitizenSearchIndexer struct {
	contacts IndexerContacts
	fields   CitizenFieldReader
	index    IndexWriter
	log      *slog.Logger
}

func NewCitizenSearchIndexer(contacts IndexerContacts, fields CitizenFieldReader, index IndexWriter, log *slog.Logger) *CitizenSearchIndexer {
	if contacts == nil || fields == nil || index == nil {
		panic("app.NewCitizenSearchIndexer: contacts, fields and index are required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &CitizenSearchIndexer{contacts: contacts, fields: fields, index: index, log: log}
}

// Handle is the events.Handler of the consumer: ctx already carries the envelope's commune
// (events.Dispatch refused an envelope without one).
func (ix *CitizenSearchIndexer) Handle(ctx context.Context, e events.Envelope) error {
	if e.Name != EventCitizenContactChanged {
		return fmt.Errorf("%w: sự kiện %q không thuộc bộ dựng chỉ mục", events.ErrPermanent, e.Name)
	}
	var p identityv1.CitizenContactChanged
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(e.Payload, &p); err != nil || p.GetContactId() == "" {
		return fmt.Errorf("%w: nội dung sự kiện %s không đọc được", events.ErrPermanent, e.ID)
	}
	// A transient refusal: the index is off (key mismatch, cluster down) — the consumer retries, the lag
	// grows, the warn line repeats. Never committed past.
	if err := ix.index.Ready(); err != nil {
		return err
	}
	return ix.sync(ctx, p.GetContactId())
}

// sync makes one contact's document equal to its row in the commune of ctx.
func (ix *CitizenSearchIndexer) sync(ctx context.Context, id string) error {
	// The store scopes this read by tenant_id through store.DB.For(ctx).
	c, found, err := ix.contacts.Get(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		// The index unindexes under the context's tenant_id (document id prefix and routing).
		return ix.index.Unindex(ctx, id)
	}
	d, err := ix.docs(ctx, []domain.CitizenContact{c})
	if err != nil {
		return err
	}
	return ix.index.Upsert(ctx, d[0])
}

func (ix *CitizenSearchIndexer) docs(ctx context.Context, cs []domain.CitizenContact) ([]search.Doc, error) {
	// The store scopes these reads by tenant_id through store.DB.For(ctx).
	defs, err := ix.fields.List(ctx, true)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(cs))
	for i, c := range cs {
		ids[i] = c.ID
	}
	groups, err := ix.contacts.GroupIDsOf(ctx, ids)
	if err != nil {
		return nil, err
	}
	t := tenant.MustFrom(ctx)
	out := make([]search.Doc, 0, len(cs))
	for _, c := range cs {
		out = append(out, ix.index.BuildDoc(t, c, groups[c.ID], defs))
	}
	return out, nil
}

// ReindexCommune rebuilds the commune of ctx from PostgreSQL: its search documents cleared, then every live
// row written in batches. Searches of that commune see a partial index while it runs (seconds for
// thousands of rows) — an operator action. Returns how many were written.
//
// With a mismatching key the index is off and this refuses: adopting a key is ReindexAll's alone.
func (ix *CitizenSearchIndexer) ReindexCommune(ctx context.Context) (int, error) {
	if err := ix.index.Ready(); err != nil {
		return 0, err
	}
	// The index clears only documents of the context's tenant_id.
	if err := ix.index.DeleteCommune(ctx); err != nil {
		return 0, err
	}
	n, after := 0, ""
	for {
		// The store scopes this read by tenant_id through store.DB.For(ctx).
		ids, err := ix.contacts.LiveIDsAfter(ctx, after, reindexBatch)
		if err != nil {
			return n, err
		}
		if len(ids) == 0 {
			return n, nil
		}
		cs := make([]domain.CitizenContact, 0, len(ids))
		for _, id := range ids {
			// The store scopes this read by tenant_id through store.DB.For(ctx).
			c, found, err := ix.contacts.Get(ctx, id)
			if err != nil {
				return n, err
			}
			if found {
				cs = append(cs, c)
			}
		}
		docs, err := ix.docs(ctx, cs)
		if err != nil {
			return n, err
		}
		if err := ix.index.BulkUpsert(ctx, docs); err != nil {
			return n, err
		}
		n += len(docs)
		after = ids[len(ids)-1]
	}
}

// ReindexAll rebuilds EVERY commune and adopts the configured SEARCH_HASH_KEY: every search document
// cleared (none hashed with an old key survives), the key's fingerprint written, then each commune rebuilt
// in its own context. The only path that accepts a new key (owner, 17/10/2026). A commune that fails is
// reported and the others continue; the error names the communes, never a row.
func (ix *CitizenSearchIndexer) ReindexAll(ctx context.Context, communes CommunesWithContacts) (int, error) {
	if communes == nil {
		return 0, errors.New("dựng lại chỉ mục: thiếu danh sách xã")
	}
	list, err := communes.CommunesWithContacts(ctx)
	if err != nil {
		return 0, err
	}
	if err := ix.index.DeleteEverything(ctx); err != nil {
		return 0, err
	}
	if err := ix.index.AdoptKey(ctx); err != nil {
		return 0, err
	}
	total := 0
	var failed []string
	for _, t := range list {
		n, err := ix.ReindexCommune(tenant.Into(ctx, t))
		total += n
		if err != nil {
			ix.log.Error("dựng lại chỉ mục danh bạ: một xã lỗi", "xa", string(t), "da_ghi", n, "err", err)
			failed = append(failed, string(t))
			continue
		}
		ix.log.Info("dựng lại chỉ mục danh bạ: xong một xã", "xa", string(t), "so_nguoi", n)
	}
	if len(failed) > 0 {
		return total, fmt.Errorf("dựng lại chỉ mục danh bạ: %d xã lỗi: %v", len(failed), failed)
	}
	return total, nil
}
