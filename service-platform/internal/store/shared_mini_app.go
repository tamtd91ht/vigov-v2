package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/service-platform/internal/domain"
)

// SharedMiniAppStore is the operator console's path to the platform's SHARED Mini App — the
// `mini_app` row with che_do = 'chinh' (migration 0006; owner 04/10/2026: the operator declares the
// shared app's App ID there, and the QR links are built from it).
//
// A RAW HANDLE, like UploadPolicyStore: the row has NO commune by 0006's CHECK
// (mini_app_xa_khi_va_chi_khi_rieng), so core/store.Scoped cannot address it. Its trail therefore
// goes to platform_audit_log — the trail of platform-wide configuration (migration 0008) — in the
// same transaction as the row (rule 6 invariant 3). A commune's audit_log is the wrong place: the act
// belongs to no commune, and filing it under one would put a platform change on that commune's
// record.
//
// ONE RUNNING SHARED APP. 0006 deliberately left "how many main apps" unconstrained and reported it;
// the owner's decision of 04/10 ("the platform's shared app") is one. It is held HERE, by the
// transaction-scoped advisory lock every write takes, not by a partial unique index — adding that
// index is DDL on a table that holds citizen-session bindings, which is a separate migration task
// (reported as a follow-up). Directory.MiniApp — the resolver — is unchanged: a 'chinh' row resolves
// to "main app, no commune", and a switched-off one to "no such app".
type SharedMiniAppStore struct {
	db *sql.DB
}

func NewSharedMiniAppStore(db *sql.DB) *SharedMiniAppStore { return &SharedMiniAppStore{db: db} }

var (
	// ErrNoSharedMiniApp — no running 'chinh' row: the shared app has not been declared (or was
	// switched off by hand). QR links cannot be built.
	ErrNoSharedMiniApp = errors.New("shared mini app: chưa khai báo App ID của Mini App dùng chung")
	// ErrSharedMiniAppAmbiguous — more than one running 'chinh' row. Not reachable through this
	// store (every write holds sharedAppLock), so it is a row written by hand. Refused rather than
	// guessed: a QR printed with the wrong one of two App IDs lives for years.
	ErrSharedMiniAppAmbiguous = errors.New("shared mini app: có hơn một Mini App dùng chung đang chạy")
	// ErrCommuneNoPrimaryHost — the commune holds no primary domain, so there is nothing for `d=`.
	ErrCommuneNoPrimaryHost = errors.New("operator registry: xã chưa có tên miền chính")
)

// ActionSharedMiniAppChanged is the platform_audit_log verb of a declaration or a replacement of the
// shared app's App ID. before = {app_id} of the row switched off (null on the first declaration),
// after = {app_id} of the row inserted.
const ActionSharedMiniAppChanged = "shared_mini_app.changed"

// sharedAppLock serialises every write of a 'chinh' row: two concurrent declarations would each see
// "one running row" and each insert their own. Operator writes are a handful a month.
const sharedAppLock = `SELECT pg_advisory_xact_lock(hashtext('platform.operator.shared_mini_app'))`

type sharedQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// runningShared reads every running 'chinh' row (normally zero or one).
func runningShared(ctx context.Context, q sharedQuerier, forUpdate bool) ([]domain.SharedMiniApp, error) {
	stmt := `SELECT app_id, tao_luc, tao_boi FROM mini_app
		WHERE che_do = 'chinh' AND dang_hoat_dong AND deleted_at IS NULL
		ORDER BY tao_luc, app_id`
	if forUpdate {
		stmt += ` FOR UPDATE`
	}
	// No tenant filter: a 'chinh' row has no commune by 0006's CHECK (see the type comment).
	rows, err := q.QueryContext(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("shared mini app: read: %w", err)
	}
	defer rows.Close()
	var out []domain.SharedMiniApp
	for rows.Next() {
		var a domain.SharedMiniApp
		if err := rows.Scan(&a.AppID, &a.CreatedAt, &a.CreatedBy); err != nil {
			return nil, fmt.Errorf("shared mini app: scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("shared mini app: read: %w", err)
	}
	return out, nil
}

func onlyOne(apps []domain.SharedMiniApp) (domain.SharedMiniApp, error) {
	switch len(apps) {
	case 0:
		return domain.SharedMiniApp{}, ErrNoSharedMiniApp
	case 1:
		return apps[0], nil
	}
	return domain.SharedMiniApp{}, ErrSharedMiniAppAmbiguous
}

// SharedMiniApp returns the one running shared app.
func (s *SharedMiniAppStore) SharedMiniApp(ctx context.Context) (domain.SharedMiniApp, error) {
	apps, err := runningShared(ctx, s.db, false)
	if err != nil {
		return domain.SharedMiniApp{}, err
	}
	return onlyOne(apps)
}

// DeclareSharedMiniApp makes appID the platform's shared app, in ONE transaction: insert a new
// 'chinh' row, switch the previously running one off (never delete it — it decided which app every
// earlier citizen session came through, rule 7), and append ActionSharedMiniAppChanged with the
// reason. All or nothing: half of it — old off, new missing — is a platform with no shared app.
//
// changed=false, nothing written, when appID already IS the running shared app. Refused, nothing
// written: appID has ANY mini_app row — a commune's app, an earlier shared app, soft-deleted
// included (ErrMiniAppTaken; the key keeps every row, so an App ID is never re-registered); more than
// one running shared row (ErrSharedMiniAppAmbiguous — a hand-written state an operator must not
// paper over).
//
// TAKES EFFECT AT ONCE: Directory.MiniApp has no cache, so the next citizen bridge call with the old
// App ID is refused, and the new one resolves as the main app.
func (s *SharedMiniAppStore) DeclareSharedMiniApp(ctx context.Context, appID, reason string,
	by domain.OperatorActor) (out domain.SharedMiniApp, changed bool, err error) {
	if err := by.Validate(); err != nil {
		return domain.SharedMiniApp{}, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.SharedMiniApp{}, false, fmt.Errorf("shared mini app: begin: %w", err)
	}
	defer func() {
		if err != nil || !changed {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, sharedAppLock); err != nil {
		return domain.SharedMiniApp{}, false, fmt.Errorf("shared mini app: lock: %w", err)
	}
	running, err := runningShared(ctx, tx, true)
	if err != nil {
		return domain.SharedMiniApp{}, false, err
	}
	if len(running) > 1 {
		return domain.SharedMiniApp{}, false, ErrSharedMiniAppAmbiguous
	}
	var before any
	if len(running) == 1 {
		if running[0].AppID == appID {
			return running[0], false, nil
		}
		before = map[string]any{"app_id": running[0].AppID}
	}
	if err = appIDFree(ctx, tx, appID); err != nil {
		return domain.SharedMiniApp{}, false, err
	}
	// The reason is the note of the new row too: it is why this App ID became the shared app.
	err = tx.QueryRowContext(ctx,
		`INSERT INTO mini_app (app_id, che_do, tenant_id, ghi_chu, tao_boi, cap_nhat_boi)
		 VALUES ($1, 'chinh', NULL, $2, $3, $3)
		 RETURNING app_id, tao_luc, tao_boi`,
		appID, reason, by.Code).Scan(&out.AppID, &out.CreatedAt, &out.CreatedBy)
	if err != nil {
		if isUniqueViolation(err) {
			err = ErrMiniAppTaken
			return domain.SharedMiniApp{}, false, err
		}
		return domain.SharedMiniApp{}, false, fmt.Errorf("shared mini app: insert: %w", err)
	}
	if len(running) == 1 {
		var res sql.Result
		res, err = tx.ExecContext(ctx,
			`UPDATE mini_app SET dang_hoat_dong = false, cap_nhat_luc = now(), cap_nhat_boi = $2
			  WHERE app_id = $1 AND che_do = 'chinh' AND dang_hoat_dong AND deleted_at IS NULL`,
			running[0].AppID, by.Code)
		if err != nil {
			return domain.SharedMiniApp{}, false, fmt.Errorf("shared mini app: switch off: %w", err)
		}
		var n int64
		if n, err = res.RowsAffected(); err != nil {
			return domain.SharedMiniApp{}, false, fmt.Errorf("shared mini app: switch off: %w", err)
		}
		if n != 1 {
			err = fmt.Errorf("shared mini app: switch off: %d rows changed, want 1", n)
			return domain.SharedMiniApp{}, false, err
		}
	}
	var beforeJSON any
	if before != nil {
		beforeJSON = string(delta(before))
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO platform_audit_log (actor, actor_ip, action, subject, before, after, reason)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7)`,
		by.Code, by.IP, ActionSharedMiniAppChanged, "MiniApp "+appID,
		beforeJSON, string(delta(map[string]any{"app_id": appID, "che_do": string(domain.CheDoChinh)})), reason); err != nil {
		return domain.SharedMiniApp{}, false, fmt.Errorf("shared mini app: trail: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return domain.SharedMiniApp{}, false, fmt.Errorf("shared mini app: commit: %w", err)
	}
	return out, true, nil
}

// CommuneLaunchHost reads what a QR link needs about one commune: whether it is active and its
// PRIMARY host. The id comes from the path, ULID-checked by the caller. ErrCommuneNotFound for an
// unknown commune; ErrCommuneNoPrimaryHost when it holds no primary row.
//
// The primary row only (la_chinh), never "the first host": after a merger a commune may hold the
// absorbed commune's old host, and a QR naming that would suggest the wrong authority (ADR 0047
// decision 4).
func (s *SharedMiniAppStore) CommuneLaunchHost(ctx context.Context, communeID string) (host string, active bool, err error) {
	var primary sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT t.dang_hoat_dong, d.host
		  FROM tenant t LEFT JOIN tenant_domain d ON d.tenant_id = t.id AND d.la_chinh
		 WHERE t.id = $1`, communeID).Scan(&active, &primary)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", false, ErrCommuneNotFound
	case err != nil:
		return "", false, fmt.Errorf("shared mini app: commune host: %w", err)
	}
	if !primary.Valid || primary.String == "" {
		return "", active, ErrCommuneNoPrimaryHost
	}
	return primary.String, active, nil
}
