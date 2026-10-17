package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The directory reads behind ResolveNoticeAudience and RevealNoticeRecipientPhones (ADR 0099 #5). The
// predicates are the contract's (identity.proto, "THE PREDICATE"); the selection over candidates —
// skip, count, merge duplicates by number — is domain.SelectNoticeRecipients.
//
// THE COMMUNE IS $1 IN EVERY STATEMENT, bound by Scoped / ScopedTx; every joined or sub-queried table
// binds `tenant_id = $1` itself (rule 1). NOTHING HERE LOGS: every phone read is personal data.
// ONE STATEMENT PER READ, never one per row (skills/load-data-once).

// ErrNoticeAudienceKind — a store read asked for an audience of the other directory. A wiring fault.
var ErrNoticeAudienceKind = errors.New("đối tượng gửi tin: loại đối tượng không thuộc danh bạ này")

// noticeCitizenAudienceClause is the CITIZEN_* predicate, live rows only. Placeholders start at $2;
// the returned args fill them. The caller appends its own after.
func noticeCitizenAudienceClause(a domain.NoticeAudience) (string, []any, error) {
	switch a.Kind {
	case domain.NoticeAudienceCitizenAll:
		return citizenContactLiveFilter, nil, nil
	case domain.NoticeAudienceCitizenGroup:
		// A LIVE membership in a LIVE group of THIS commune: both tables bind $1 themselves, so a
		// group id of another commune matches nobody (identity.proto, not an error).
		return citizenContactLiveFilter + ` AND EXISTS (SELECT 1 FROM citizen_contact_group m` +
			` JOIN citizen_group g ON g.tenant_id = $1 AND g.id = m.group_id AND g.deleted_at IS NULL` +
			` WHERE m.tenant_id = $1 AND m.contact_id = citizen_contact.id AND m.deleted_at IS NULL` +
			` AND m.group_id = ANY($2))`, []any{a.GroupIDs}, nil
	case domain.NoticeAudienceCitizenTag:
		// ANY of the tags, compared in the stored form (domain.CheckNoticeAudience normalised them).
		return citizenContactLiveFilter + ` AND tags && $2::text[]`, []any{a.Tags}, nil
	}
	return "", nil, ErrNoticeAudienceKind
}

// NoticeAudienceCounts counts a CITIZEN_* audience: live contacts that are active (the recipients —
// a live number is unique per commune, `citizen_contact_phone_live_unique`, so nothing merges) and
// live contacts switched off (`is_active = false`, skipped). A contact always has a number
// (`citizen_contact_phone_digits`), so there is no "no phone" count here.
func (s *CitizenDirectoryStore) NoticeAudienceCounts(ctx context.Context, a domain.NoticeAudience) (active, inactive int, err error) {
	clause, args, err := noticeCitizenAudienceClause(a)
	if err != nil {
		return 0, 0, err
	}
	rows, err := s.db.For(ctx).Query(ctx,
		`count(*) FILTER (WHERE is_active), count(*) FILTER (WHERE NOT is_active)`,
		"citizen_contact", clause, args...)
	if err != nil {
		return 0, 0, fmt.Errorf("đối tượng gửi tin: đếm người dân: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, 0, fmt.Errorf("đối tượng gửi tin: đếm người dân: %w", err)
		}
		return 0, 0, errors.New("đối tượng gửi tin: đếm người dân: không có dòng kết quả")
	}
	var a64, i64 int64
	if err := rows.Scan(&a64, &i64); err != nil {
		return 0, 0, fmt.Errorf("đối tượng gửi tin: đếm người dân: đọc: %w", err)
	}
	return int(a64), int(i64), rows.Err()
}

// NoticeAudiencePage reads up to `limit` ACTIVE contacts of a CITIZEN_* audience whose id is after
// `after` ("" = from the start), ordered by id — the keyset the page token carries. The caller asks for
// one more than it hands out, to know whether a next page exists.
func (s *CitizenDirectoryStore) NoticeAudiencePage(ctx context.Context, a domain.NoticeAudience, after string, limit int) ([]domain.NoticeCandidate, error) {
	if limit <= 0 {
		return nil, nil
	}
	clause, args, err := noticeCitizenAudienceClause(a)
	if err != nil {
		return nil, err
	}
	n := len(args) + 2 // the next free placeholder: $1 is the commune
	clause += fmt.Sprintf(` AND is_active AND id > $%d ORDER BY id LIMIT $%d`, n, n+1)
	args = append(args, after, limit)

	rows, err := s.db.For(ctx).Query(ctx, `id, phone`, "citizen_contact", clause, args...)
	if err != nil {
		return nil, fmt.Errorf("đối tượng gửi tin: đọc trang người dân: %w", err)
	}
	return scanNoticeCandidates(rows, false)
}

// NoticeRevealCandidates reads the contacts behind `refs` inside the caller's transaction, whose
// audit entry it must write before the numbers leave (rule 6, invariants 3 and 7). Live rows only;
// `is_active` travels as Inactive so domain.SelectNoticeRecipients re-applies the predicate NOW.
// Group / tag membership is NOT re-checked: the audience was fixed at POST (identity.proto).
func (s *CitizenDirectoryStore) NoticeRevealCandidates(ctx context.Context, tx *store.ScopedTx, refs []string) ([]domain.NoticeCandidate, error) {
	if tx == nil {
		return nil, ErrThieuGiaoDich
	}
	if len(refs) == 0 {
		return nil, nil
	}
	// ScopedTx.Query binds tenant_id = $1 from the transaction's commune (rule 1, invariant 5).
	rows, err := tx.Query(ctx, `id, phone, NOT is_active`, "citizen_contact",
		citizenContactLiveFilter+` AND id = ANY($2)`, refs)
	if err != nil {
		return nil, fmt.Errorf("đối tượng gửi tin: đọc số người dân để gửi: %w", err)
	}
	return scanNoticeCandidates(rows, true)
}

// noticeStaffCols: the ref (business code — never the internal id), the mobile as typed (migration
// 0009 §2), and LOCKED = an account that exists and is switched off. A directory-only row
// (`co_tai_khoan = false`) is not locked: `dang_hoat_dong` says nothing there (0009:100-101).
const noticeStaffCols = `ma, di_dong_ca_nhan, (co_tai_khoan AND NOT dang_hoat_dong)`

// NoticeAudienceCandidates reads EVERY live staff row of a STAFF_* audience in one statement. The whole
// audience, not a page: duplicates among staff are real (two rows may hold one typed mobile) and are
// merged over the whole audience before paging (identity.proto). A commune's staff is hundreds.
//
// STAFF_UNIT: DIRECT members of a LIVE unit of THIS commune — sub-units are not walked, and a unit id
// of another commune or a removed unit matches nobody.
func (s *CanBoStore) NoticeAudienceCandidates(ctx context.Context, a domain.NoticeAudience) ([]domain.NoticeCandidate, error) {
	var tail string
	var args []any
	switch a.Kind {
	case domain.NoticeAudienceStaffAll:
		tail = `AND deleted_at IS NULL`
	case domain.NoticeAudienceStaffUnit:
		tail = `AND deleted_at IS NULL AND bo_phan_id = ANY($2) AND EXISTS (SELECT 1 FROM bo_phan b` +
			` WHERE b.tenant_id = $1 AND b.id = nguoi_dung.bo_phan_id AND b.` + liveOrgUnit + `)`
		args = []any{a.OrgUnitIDs}
	default:
		return nil, ErrNoticeAudienceKind
	}
	rows, err := s.db.For(ctx).Query(ctx, noticeStaffCols, "nguoi_dung", tail, args...)
	if err != nil {
		return nil, fmt.Errorf("đối tượng gửi tin: đọc cán bộ: %w", err)
	}
	return scanNoticeCandidates(rows, true)
}

// NoticeRevealCandidates reads the staff rows behind `codes` inside the caller's transaction. Live
// rows only; LOCKED travels as Inactive and the mobile as typed, so domain.SelectNoticeRecipients
// re-applies the predicate NOW. Unit membership is NOT re-checked (identity.proto).
func (s *CanBoStore) NoticeRevealCandidates(ctx context.Context, tx *store.ScopedTx, codes []string) ([]domain.NoticeCandidate, error) {
	if tx == nil {
		return nil, ErrThieuGiaoDich
	}
	if len(codes) == 0 {
		return nil, nil
	}
	// ScopedTx.Query binds tenant_id = $1 from the transaction's commune (rule 1, invariant 5).
	rows, err := tx.Query(ctx, noticeStaffCols, "nguoi_dung", `AND deleted_at IS NULL AND ma = ANY($2)`, codes)
	if err != nil {
		return nil, fmt.Errorf("đối tượng gửi tin: đọc số cán bộ để gửi: %w", err)
	}
	return scanNoticeCandidates(rows, true)
}

// StaffCodeExists reports whether `code` is a `nguoi_dung.ma` of this commune, SOFT-DELETED INCLUDED:
// the person who pressed send stays the actor of the disclosure their campaign causes, even if they
// were removed from the directory before the job ran (identity.proto, RevealNoticeRecipientPhones).
func (s *CanBoStore) StaffCodeExists(ctx context.Context, tx *store.ScopedTx, code string) (bool, error) {
	if tx == nil {
		return false, ErrThieuGiaoDich
	}
	if strings.TrimSpace(code) == "" {
		return false, nil
	}
	// ScopedTx.Query binds tenant_id = $1 from the transaction's commune (rule 1, invariant 5).
	rows, err := tx.Query(ctx, `1`, "nguoi_dung", `AND ma = $2 LIMIT 1`, code)
	if err != nil {
		return false, fmt.Errorf("đối tượng gửi tin: kiểm mã cán bộ gửi: %w", err)
	}
	defer rows.Close()
	found := rows.Next()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("đối tượng gửi tin: kiểm mã cán bộ gửi: %w", err)
	}
	return found, nil
}

// scanNoticeCandidates reads (ref, phone[, inactive]) rows. rows.Err() is checked: a connection lost
// halfway ends the loop exactly like a complete read, and a short audience would be sent as whole.
func scanNoticeCandidates(rows *sql.Rows, withInactive bool) ([]domain.NoticeCandidate, error) {
	defer rows.Close()
	var out []domain.NoticeCandidate
	for rows.Next() {
		var c domain.NoticeCandidate
		dest := []any{&c.Ref, &c.Phone}
		if withInactive {
			dest = append(dest, &c.Inactive)
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("đối tượng gửi tin: đọc dòng: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("đối tượng gửi tin: duyệt dòng: %w", err)
	}
	return out, nil
}
