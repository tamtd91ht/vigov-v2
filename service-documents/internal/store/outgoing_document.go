package store

// The SỔ VĂN BẢN ĐI register — SQL, and nothing else.
//
// THE FIVE PROPERTIES STATED AT THE TOP OF incoming_document.go HOLD HERE TOO, word for word: the
// commune is $1 everywhere, nothing here opens a transaction, every read excludes soft-deleted rows,
// the issued number appears in no UPDATE, and there is no hard delete. They are not repeated; what
// follows is only what is DIFFERENT about the outgoing register.
//
// AND THE DIFFERENCE IS THE WEIGHT OF THE NUMBER. An incoming number is the commune's own
// bookkeeping. An OUTGOING number is printed on the document, sealed, and sent to a district office,
// a court or a citizen — so a duplicate is not a data problem inside this system, it is two
// documents nobody outside the commune can tell apart, one of which somebody has signed. The
// mechanisms are identical (document_number_series.go); the consequence of a hole in them is not.
//
// ⚠ NO SPECIFICATION EXISTS FOR THIS REGISTER. docs/ui-ux/05-van-ban-don-thu.md describes the
// incoming register and the citizen-letter register and says nothing at all about outgoing
// documents. The columns below are this session's reading, reported as such — see migration 0004.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// OutgoingDocumentStore reads and writes one commune's outgoing-document register.
type OutgoingDocumentStore struct {
	db *store.DB
}

func NewOutgoingDocumentStore(db *store.DB) *OutgoingDocumentStore {
	return &OutgoingDocumentStore{db: db}
}

var (
	// ErrOutgoingDocumentNotFound — no such live entry in this commune. Same single answer for "does
	// not exist" and "belongs to another commune", for the same reason as the incoming register.
	ErrOutgoingDocumentNotFound = errors.New("van_ban_di: không có văn bản này trong xã")

	// ErrOutgoingDocumentTypeNotInUse — the type code names no live, in-use row of this commune's
	// catalogue. A SEPARATE SENTINEL from the incoming register's, so the handler's message names
	// the register the clerk is actually looking at.
	ErrOutgoingDocumentTypeNotInUse = errors.New("van_ban_di: loại văn bản không có trong danh mục của xã")
)

// OutgoingDocumentSorts — `number` descending by default, the newest issued document first. Same
// closed set and same exclusions as the incoming register: `noi_nhan` and `trich_yeu` are free text
// about a government matter and a sort key travels in a URL and an access log.
var OutgoingDocumentSorts = page.NewAllowlist(page.Desc,
	page.Col("number", "so_di", page.KindInt),
	page.Col("created_at", "tao_luc", page.KindTime),
)

// OutgoingDocumentFilter is the set of filters the list route accepts, already validated by the
// handler. Every field becomes a BOUND PARAMETER — see outgoingFilterSQL.
type OutgoingDocumentFilter struct {
	Year         int
	DocumentType string
	Search       string
}

// outgoingDocumentColumns IS READ BY POSITION in the scans below.
//
// THE TRAP HERE IS `trich_yeu` / `noi_nhan` / `nguoi_ky` — three adjacent TEXT columns. Swapped, the
// register shows the recipient where the summary belongs and, worse, the signer's name in the
// recipient column: an outgoing document that reads as having been sent to the person who signed it.
// Nothing errors, and the row looks plausible.
const outgoingDocumentColumns = `id, so_di, nam, ngay_van_ban, loai_van_ban, trich_yeu, noi_nhan, ` +
	`COALESCE(nguoi_ky, ''), nguoi_tao_ma, tao_luc, cap_nhat_luc`

func scanOutgoingDocument(scan func(...any) error) (domain.OutgoingDocument, error) {
	var d domain.OutgoingDocument
	err := scan(&d.ID, &d.IssuedNo, &d.Year, &d.DocumentDate, &d.DocumentType, &d.Summary,
		&d.Recipient, &d.Signer, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return domain.OutgoingDocument{}, err
	}
	return d, nil
}

// List reads ONE PAGE of the commune's outgoing register.
func (s *OutgoingDocumentStore) List(ctx context.Context, filter OutgoingDocumentFilter,
	req page.Request) (page.Result[domain.OutgoingDocument], error) {

	where, args := outgoingFilterSQL(filter)

	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: outgoingDocumentColumns,
		Table:   "van_ban_di",
		Filter:  `AND deleted_at IS NULL` + where,
		Args:    args,
	}, req, outgoingDocumentCursor, func(rows *sql.Rows) (domain.OutgoingDocument, string, error) {
		d, err := scanOutgoingDocument(rows.Scan)
		if err != nil {
			return domain.OutgoingDocument{}, "", err
		}
		return d, d.ID, nil
	})
}

func outgoingFilterSQL(filter OutgoingDocumentFilter) (string, []any) {
	var (
		where string
		args  []any
	)
	add := func(format string, value any) {
		args = append(args, value)
		where += fmt.Sprintf(format, len(args)+1) // $1 is the commune
	}
	if filter.Year != 0 {
		add(" AND nam = $%d", filter.Year)
	}
	if filter.DocumentType != "" {
		add(" AND loai_van_ban = $%d", filter.DocumentType)
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		where += fmt.Sprintf(" AND (trich_yeu ILIKE $%d OR noi_nhan ILIKE $%d)",
			len(args)+1, len(args)+1)
	}
	return where, args
}

var outgoingDocumentCursor = store.NewMoc[domain.OutgoingDocument](OutgoingDocumentSorts,
	map[string]func(domain.OutgoingDocument) page.Key{
		"number":     func(d domain.OutgoingDocument) page.Key { return page.IntKey(int64(d.IssuedNo)) },
		"created_at": func(d domain.OutgoingDocument) page.Key { return page.TimeKey(d.CreatedAt) },
	})

// --- the write path ---------------------------------------------------------------------------

// ByIDForUpdate reads one live entry inside the transaction and holds it until the transaction ends.
// `FOR UPDATE` for the same reason as on the incoming register: every write is a read-decide-write.
func (s *OutgoingDocumentStore) ByIDForUpdate(ctx context.Context, tx *store.ScopedTx,
	id string) (domain.OutgoingDocument, error) {

	const stmt = `SELECT ` + outgoingDocumentColumns + ` FROM van_ban_di ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	d, err := scanOutgoingDocument(
		tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OutgoingDocument{}, ErrOutgoingDocumentNotFound
	}
	if err != nil {
		return domain.OutgoingDocument{}, fmt.Errorf("van_ban_di: đọc văn bản để sửa: %w", err)
	}
	return d, nil
}

// DocumentTypeInUse is the same check the incoming register makes, returning ITS OWN sentinel.
func (s *OutgoingDocumentStore) DocumentTypeInUse(ctx context.Context, tx *store.ScopedTx, code string) error {
	const stmt = `SELECT 1 FROM loai_van_ban
		WHERE tenant_id = $1 AND ma = $2 AND dang_dung AND deleted_at IS NULL`

	var one int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), code).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrOutgoingDocumentTypeNotInUse
	}
	if err != nil {
		return fmt.Errorf("van_ban_di: kiểm loại văn bản: %w", err)
	}
	return nil
}

// insertOutgoingDocument — `so_di` is a parameter and comes from NumberSeriesStore.IssueNumber in
// this same transaction, under the counter's row lock. There is no other caller and no other source.
const insertOutgoingDocument = `INSERT INTO van_ban_di
	(tenant_id, id, so_di, nam, ngay_van_ban, loai_van_ban, trich_yeu, noi_nhan,
	 nguoi_ky, nguoi_tao_ma)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

func (s *OutgoingDocumentStore) Insert(ctx context.Context, tx *store.ScopedTx, d domain.OutgoingDocument) error {
	_, err := tx.Exec(ctx, insertOutgoingDocument, string(tx.TenantID()),
		d.ID, d.IssuedNo, d.Year, d.DocumentDate, d.DocumentType, d.Summary, d.Recipient,
		emptyToNil(d.Signer), d.CreatedBy)
	if err != nil {
		return fmt.Errorf("van_ban_di: chèn: %w", err)
	}
	return nil
}

// updateOutgoingDocument — `so_di` and `nam` APPEAR NOWHERE IN THIS STATEMENT. An issued outgoing
// number is on paper outside this commune; the trigger refuses to change it underneath, and its
// absence here is what makes that floor unreachable from this service.
const updateOutgoingDocument = `UPDATE van_ban_di
	SET ngay_van_ban = $3, loai_van_ban = $4, trich_yeu = $5, noi_nhan = $6, nguoi_ky = $7,
	    cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *OutgoingDocumentStore) Update(ctx context.Context, tx *store.ScopedTx, d domain.OutgoingDocument) error {
	res, err := tx.Exec(ctx, updateOutgoingDocument, string(tx.TenantID()),
		d.ID, d.DocumentDate, d.DocumentType, d.Summary, d.Recipient, emptyToNil(d.Signer))
	if err != nil {
		return fmt.Errorf("van_ban_di: cập nhật: %w", err)
	}
	return requireOneOutgoingRow(res, "cập nhật")
}

// softDeleteOutgoingDocument writes all THREE columns rule 7, invariant 1 names, in one statement.
//
// THE NUMBER IS NOT RETURNED TO THE SERIES — and on this register that sentence is the whole point.
// A document was issued under that number and has left the building; removing the row from the
// commune's screens cannot unsend it.
const softDeleteOutgoingDocument = `UPDATE van_ban_di
	SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *OutgoingDocumentStore) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error {
	res, err := tx.Exec(ctx, softDeleteOutgoingDocument, string(tx.TenantID()), id, by, reason)
	if err != nil {
		return fmt.Errorf("van_ban_di: xoá mềm: %w", err)
	}
	return requireOneOutgoingRow(res, "xoá mềm")
}

// requireOneOutgoingRow is a SECOND COPY of requireOneIncomingRow on purpose, and the difference is
// the error it returns: that one answers ErrIncomingDocumentNotFound, which the handler maps to
// "Không tìm thấy văn bản đến này." Sharing it would put a sentence about the incoming register in
// front of a clerk who was working in the outgoing one — and those are two different books on two
// different screens.
func requireOneOutgoingRow(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("van_ban_di: %s: đọc số dòng: %w", op, err)
	}
	if n == 0 {
		return ErrOutgoingDocumentNotFound
	}
	return nil
}
