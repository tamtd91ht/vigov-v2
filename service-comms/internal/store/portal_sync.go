package store

// THE PORTAL SYNC TABLES — migration 0013 (`portal_sync_settings`, `portal_categories`,
// `portal_sync_runs`) and the sync's own INSERT into `noi_dung_mini_app`. SQL, and nothing else.
//
// THE SAME FOUR THINGS HOLD AS IN noi_dung_mini_app.go: the commune is $1 from the context and never a
// parameter; nothing here opens a transaction (internal/app does, and writes the audit entry inside
// it); every value is bound; reads exclude soft-deleted rows — except ExistingPortalItems, which exists
// precisely to SEE them (a removed article is never re-imported, 0006:343-358).
//
// THE SEALED KEY leaves the database on exactly two reads — SettingsWithKey (to open it for a call) and
// SettingsForUpdate (to keep it on a save that sends none). The screen's read, Settings, selects the
// row without the column; a row exists only with a key (the column is NOT NULL), so "key set" is "row
// exists".
//
// NO audit.Write IN THIS FILE — the actor and the business verb do not exist at the SQL layer; the use
// case writes the entry in the transaction it passes here (rule 6, invariant 3).

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

var (
	// ErrPortalSyncSettingsNotFound — the commune has not configured its portal.
	ErrPortalSyncSettingsNotFound = errors.New("portal_sync_settings: xã chưa cấu hình đồng bộ Cổng")
	// ErrPortalItemExists — the article's portal id is already held in this commune (a concurrent
	// writer won `UNIQUE (tenant_id, nguon_id_ngoai)`). Counted as skipped, never retried as an update.
	ErrPortalItemExists = errors.New("noi_dung_mini_app: tin Cổng này đã có trong xã")
	// ErrPortalSyncRunFinished — the run row is already finished; the one fill happened (0013 trigger).
	ErrPortalSyncRunFinished = errors.New("portal_sync_runs: lượt đồng bộ đã kết thúc")
	// ErrPortalCategoriesTooMany — past PortalCategoriesMax; refused rather than truncated.
	ErrPortalCategoriesTooMany = errors.New("portal_categories: vượt trần chuyên mục Cổng")
)

// PortalCategoriesMax bounds one commune's `portal_categories` read. §3's sample portal has ~60
// categories; a row exists only for one selected at least once.
const PortalCategoriesMax = domain.PortalSelectionMax

// existingBatch bounds one IN (...) of ExistingPortalItems.
const existingBatch = 200

// PortalSyncStore is the only path to the three 0013 tables and to the sync's item INSERT.
type PortalSyncStore struct {
	db *store.DB
}

func NewPortalSyncStore(db *store.DB) *PortalSyncStore { return &PortalSyncStore{db: db} }

// portalSettingsCols IS READ BY POSITION in scanPortalSettings.
const portalSettingsCols = `provider, api_url, publish_mode, interval_hours, window_days, ` +
	`max_items_per_run, keep_source_credit, is_enabled, last_run_at, updated_at, updated_by`

func scanPortalSettings(scan func(...any) error, extra ...any) (domain.PortalSyncSettings, error) {
	var (
		s    domain.PortalSyncSettings
		last sql.NullTime
	)
	dest := append([]any{&s.Provider, &s.APIURL, &s.PublishMode, &s.IntervalHours, &s.WindowDays,
		&s.MaxItemsPerRun, &s.KeepSourceCredit, &s.IsEnabled, &last, &s.UpdatedAt, &s.UpdatedBy}, extra...)
	if err := scan(dest...); err != nil {
		return domain.PortalSyncSettings{}, err
	}
	s.LastRunAt = nullTimeUTC(last)
	s.UpdatedAt = s.UpdatedAt.UTC()
	s.APIKeySet = true // the column is NOT NULL: a row is a row with a key
	return s, nil
}

// Settings reads the commune's row for the screen — without the key.
func (s *PortalSyncStore) Settings(ctx context.Context) (domain.PortalSyncSettings, error) {
	rows, err := s.db.For(ctx).Query(ctx, portalSettingsCols, "portal_sync_settings", "")
	if err != nil {
		return domain.PortalSyncSettings{}, fmt.Errorf("portal_sync_settings: read: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.PortalSyncSettings{}, fmt.Errorf("portal_sync_settings: read: %w", err)
		}
		return domain.PortalSyncSettings{}, ErrPortalSyncSettingsNotFound
	}
	st, err := scanPortalSettings(rows.Scan)
	if err != nil {
		return domain.PortalSyncSettings{}, fmt.Errorf("portal_sync_settings: scan: %w", err)
	}
	return st, nil
}

// SettingsWithKey reads the row AND the sealed key, for a call to the portal.
func (s *PortalSyncStore) SettingsWithKey(ctx context.Context) (domain.PortalSyncSettings, []byte, error) {
	rows, err := s.db.For(ctx).Query(ctx, portalSettingsCols+`, api_key_sealed`, "portal_sync_settings", "")
	if err != nil {
		return domain.PortalSyncSettings{}, nil, fmt.Errorf("portal_sync_settings: read key: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.PortalSyncSettings{}, nil, fmt.Errorf("portal_sync_settings: read key: %w", err)
		}
		return domain.PortalSyncSettings{}, nil, ErrPortalSyncSettingsNotFound
	}
	var sealed []byte
	st, err := scanPortalSettings(rows.Scan, &sealed)
	if err != nil {
		return domain.PortalSyncSettings{}, nil, fmt.Errorf("portal_sync_settings: scan key: %w", err)
	}
	return st, sealed, nil
}

// SettingsForUpdate reads and LOCKS the row with its key. found=false when there is none yet. Two
// administrators saving at once would otherwise both decide "address unchanged, keep the key" against
// the old row (the mail_settings argument, store/mail_settings.go).
func (s *PortalSyncStore) SettingsForUpdate(ctx context.Context, tx *store.ScopedTx) (
	domain.PortalSyncSettings, []byte, bool, error) {

	const stmt = `SELECT ` + portalSettingsCols + `, api_key_sealed FROM portal_sync_settings ` +
		`WHERE tenant_id = $1 FOR UPDATE`
	var sealed []byte
	st, err := scanPortalSettings(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID())).Scan, &sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PortalSyncSettings{}, nil, false, nil
	}
	if err != nil {
		return domain.PortalSyncSettings{}, nil, false, fmt.Errorf("portal_sync_settings: read for update: %w", err)
	}
	return st, sealed, true, nil
}

// upsertPortalSettings — `tenant_id` is in no SET list (the trigger refuses a change too) and
// `last_run_at` is not either: a configuration save does not say when a run started.
const upsertPortalSettings = `INSERT INTO portal_sync_settings
	(tenant_id, provider, api_url, api_key_sealed, publish_mode, interval_hours, window_days,
	 max_items_per_run, keep_source_credit, is_enabled, updated_by)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	ON CONFLICT (tenant_id) DO UPDATE SET
	provider = EXCLUDED.provider, api_url = EXCLUDED.api_url, api_key_sealed = EXCLUDED.api_key_sealed,
	publish_mode = EXCLUDED.publish_mode, interval_hours = EXCLUDED.interval_hours,
	window_days = EXCLUDED.window_days, max_items_per_run = EXCLUDED.max_items_per_run,
	keep_source_credit = EXCLUDED.keep_source_credit, is_enabled = EXCLUDED.is_enabled,
	updated_by = EXCLUDED.updated_by, updated_at = now()`

// UpsertSettings writes the whole row. sealed is the FINAL sealed key, never empty. by is a staff
// BUSINESS CODE (rule 6, invariant 8).
func (s *PortalSyncStore) UpsertSettings(ctx context.Context, tx *store.ScopedTx, st domain.PortalSyncSettings,
	sealed []byte, by string) error {

	if len(sealed) == 0 {
		return errors.New("portal_sync_settings: upsert without a sealed key")
	}
	if _, err := tx.Exec(ctx, upsertPortalSettings, string(tx.TenantID()), st.Provider, st.APIURL, sealed,
		st.PublishMode, st.IntervalHours, st.WindowDays, st.MaxItemsPerRun, st.KeepSourceCredit,
		st.IsEnabled, by); err != nil {
		return fmt.Errorf("portal_sync_settings: upsert: %w", err)
	}
	return nil
}

// MarkRunStarted records when the last run started — the scheduler's due-check, written in the SAME
// transaction as the run's row (0013). `updated_by` / `updated_at` are not touched: that is not a
// configuration change.
func (s *PortalSyncStore) MarkRunStarted(ctx context.Context, tx *store.ScopedTx, at time.Time) error {
	const stmt = `UPDATE portal_sync_settings SET last_run_at = $2 WHERE tenant_id = $1`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), at.UTC())
	if err != nil {
		return fmt.Errorf("portal_sync_settings: mark run started: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("portal_sync_settings: mark run started: %w", err)
	} else if n == 0 {
		return ErrPortalSyncSettingsNotFound
	}
	return nil
}

// ResealKey replaces the sealed key with one sealed under the api_url binding (R6), ONLY IF the row
// still holds apiURL and oldSealed — a compare-and-swap, so a configuration save that ran between the
// read and this write wins and nothing is overwritten. swapped=false is that case, not an error.
// `updated_by` / `updated_at` are not touched: re-sealing is not a configuration change by anyone; the
// use case files the system entry in the same transaction.
func (s *PortalSyncStore) ResealKey(ctx context.Context, tx *store.ScopedTx, apiURL string, oldSealed,
	newSealed []byte) (bool, error) {

	if len(newSealed) == 0 {
		return false, errors.New("portal_sync_settings: reseal without a sealed key")
	}
	const stmt = `UPDATE portal_sync_settings SET api_key_sealed = $4
		WHERE tenant_id = $1 AND api_url = $2 AND api_key_sealed = $3`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), apiURL, oldSealed, newSealed)
	if err != nil {
		return false, fmt.Errorf("portal_sync_settings: reseal: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("portal_sync_settings: reseal: %w", err)
	}
	return n == 1, nil
}

// --- categories ------------------------------------------------------------------------------------

const portalCategoryCols = `id, external_id, name, target_kind, is_selected, created_at, updated_at`

func scanPortalCategory(scan func(...any) error) (domain.PortalCategory, error) {
	var c domain.PortalCategory
	if err := scan(&c.ID, &c.ExternalID, &c.Name, &c.TargetKind, &c.IsSelected, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return domain.PortalCategory{}, err
	}
	c.CreatedAt, c.UpdatedAt = c.CreatedAt.UTC(), c.UpdatedAt.UTC()
	return c, nil
}

func collectPortalCategories(rows *sql.Rows) ([]domain.PortalCategory, error) {
	defer rows.Close()
	out := make([]domain.PortalCategory, 0, 16)
	for rows.Next() {
		c, err := scanPortalCategory(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("portal_categories: scan: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("portal_categories: read: %w", err)
	}
	if len(out) > PortalCategoriesMax {
		return nil, ErrPortalCategoriesTooMany
	}
	return out, nil
}

// Categories reads every category row of the commune (selected or not), ordered by name. LIMIT is the
// ceiling plus one so "too many" is detectable rather than silently cut.
func (s *PortalSyncStore) Categories(ctx context.Context) ([]domain.PortalCategory, error) {
	rows, err := s.db.For(ctx).Query(ctx, portalCategoryCols, "portal_categories",
		`ORDER BY name, external_id LIMIT $2`, PortalCategoriesMax+1)
	if err != nil {
		return nil, fmt.Errorf("portal_categories: read: %w", err)
	}
	return collectPortalCategories(rows)
}

// CategoriesForUpdate reads and LOCKS every category row of the commune, inside a selection save.
func (s *PortalSyncStore) CategoriesForUpdate(ctx context.Context, tx *store.ScopedTx) ([]domain.PortalCategory, error) {
	const stmt = `SELECT ` + portalCategoryCols + ` FROM portal_categories WHERE tenant_id = $1 ` +
		`ORDER BY external_id LIMIT $2 FOR UPDATE`
	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), PortalCategoriesMax+1)
	if err != nil {
		return nil, fmt.Errorf("portal_categories: read for update: %w", err)
	}
	return collectPortalCategories(rows)
}

// InsertCategory records a category selected for the first time. by is a business code.
func (s *PortalSyncStore) InsertCategory(ctx context.Context, tx *store.ScopedTx, c domain.PortalCategory, by string) error {
	const stmt = `INSERT INTO portal_categories
		(tenant_id, id, external_id, name, target_kind, is_selected, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$7)`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), c.ID, c.ExternalID, c.Name, c.TargetKind,
		c.IsSelected, by); err != nil {
		return fmt.Errorf("portal_categories: insert: %w", err)
	}
	return nil
}

// UpdateCategory writes the three editable columns (0013 trigger: identity columns are frozen).
func (s *PortalSyncStore) UpdateCategory(ctx context.Context, tx *store.ScopedTx, c domain.PortalCategory, by string) error {
	const stmt = `UPDATE portal_categories
		SET name = $3, target_kind = $4, is_selected = $5, updated_by = $6, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), c.ID, c.Name, c.TargetKind, c.IsSelected, by)
	if err != nil {
		return fmt.Errorf("portal_categories: update: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return fmt.Errorf("portal_categories: update: no row (%v)", err)
	}
	return nil
}

// --- runs ------------------------------------------------------------------------------------------

// SortPortalSyncRuns — newest first, the one order the run history has (0013 index
// `portal_sync_runs_recent (tenant_id, started_at DESC, id)`).
var SortPortalSyncRuns = page.NewAllowlist(page.Desc,
	page.Col("started_at", "started_at", page.KindTime),
)

var mocPortalSyncRuns = store.NewMoc[domain.PortalSyncRun](SortPortalSyncRuns,
	map[string]func(domain.PortalSyncRun) page.Key{
		"started_at": func(r domain.PortalSyncRun) page.Key { return page.TimeKey(r.StartedAt) },
	})

const portalRunCols = `id, trigger_kind, actor, started_at, finished_at, outcome, fetched_count, ` +
	`imported_count, skipped_existing_count, skipped_deleted_count, failed_count, error_summary`

func scanPortalRun(scan func(...any) error) (domain.PortalSyncRun, error) {
	var (
		r                                  domain.PortalSyncRun
		finished                           sql.NullTime
		outcome                            sql.NullString
		fetched, imported, skE, skD, fails sql.NullInt64
		summary                            []byte
	)
	if err := scan(&r.ID, &r.TriggerKind, &r.Actor, &r.StartedAt, &finished, &outcome, &fetched,
		&imported, &skE, &skD, &fails, &summary); err != nil {
		return domain.PortalSyncRun{}, err
	}
	r.StartedAt = r.StartedAt.UTC()
	r.FinishedAt = nullTimeUTC(finished)
	r.Outcome = outcome.String
	r.Counts = domain.PortalRunCounts{Fetched: int(fetched.Int64), Imported: int(imported.Int64),
		SkippedExisting: int(skE.Int64), SkippedDeleted: int(skD.Int64), Failed: int(fails.Int64)}
	if len(summary) > 0 {
		if err := json.Unmarshal(summary, &r.Errors); err != nil {
			return domain.PortalSyncRun{}, fmt.Errorf("portal_sync_runs: error_summary: %w", err)
		}
	}
	return r, nil
}

// Runs reads one page of the commune's run history, newest first.
func (s *PortalSyncStore) Runs(ctx context.Context, req page.Request) (page.Result[domain.PortalSyncRun], error) {
	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: portalRunCols, Table: "portal_sync_runs",
	}, req, mocPortalSyncRuns, func(rows *sql.Rows) (domain.PortalSyncRun, string, error) {
		r, err := scanPortalRun(rows.Scan)
		if err != nil {
			return domain.PortalSyncRun{}, "", fmt.Errorf("portal_sync_runs: scan: %w", err)
		}
		return r, r.ID, nil
	})
}

// InsertRun records the start of a run. `started_at` is bound (one clock reading for the act), the
// finish columns are NULL by construction (0013 `finish_all_or_none`).
func (s *PortalSyncStore) InsertRun(ctx context.Context, tx *store.ScopedTx, r domain.PortalSyncRun) error {
	const stmt = `INSERT INTO portal_sync_runs (tenant_id, id, trigger_kind, actor, started_at)
		VALUES ($1,$2,$3,$4,$5)`
	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), r.ID, r.TriggerKind, r.Actor, r.StartedAt.UTC()); err != nil {
		return fmt.Errorf("portal_sync_runs: insert: %w", err)
	}
	return nil
}

// FinishRun is THE ONE FILL of a run's outcome (0013 `portal_sync_run_finish_once`). `AND finished_at
// IS NULL` makes a second fill touch nothing (ErrPortalSyncRunFinished) instead of meeting the trigger.
func (s *PortalSyncStore) FinishRun(ctx context.Context, tx *store.ScopedTx, r domain.PortalSyncRun) error {
	summary, err := marshalRunErrors(r.Errors)
	if err != nil {
		return err
	}
	const stmt = `UPDATE portal_sync_runs SET finished_at = $3, outcome = $4, fetched_count = $5,
		imported_count = $6, skipped_existing_count = $7, skipped_deleted_count = $8, failed_count = $9,
		error_summary = $10::jsonb
		WHERE tenant_id = $1 AND id = $2 AND finished_at IS NULL`
	c := r.Counts
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), r.ID, r.FinishedAt.UTC(), r.Outcome, c.Fetched,
		c.Imported, c.SkippedExisting, c.SkippedDeleted, c.Failed, string(summary))
	if err != nil {
		return fmt.Errorf("portal_sync_runs: finish: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("portal_sync_runs: finish: %w", err)
	} else if n == 0 {
		return ErrPortalSyncRunFinished
	}
	return nil
}

// marshalRunErrors encodes the summary, never null (0013: all finish columns set together).
func marshalRunErrors(es []domain.PortalRunError) ([]byte, error) {
	if es == nil {
		es = []domain.PortalRunError{}
	}
	b, err := json.Marshal(es)
	if err != nil {
		return nil, fmt.Errorf("portal_sync_runs: error_summary: %w", err)
	}
	return b, nil
}

// ReapStuckRuns finishes as `that-bai` every run of the commune still unfinished that started before
// `before` — a crashed pod's run. The caller holds the commune's run lock, so no LIVE run can be among
// them. Returns the ids finished (each is that run's one fill).
func (s *PortalSyncStore) ReapStuckRuns(ctx context.Context, tx *store.ScopedTx, before, at time.Time) ([]string, error) {
	summary, err := marshalRunErrors([]domain.PortalRunError{{Error: domain.PortalRunErrorInterrupted, Count: 1}})
	if err != nil {
		return nil, err
	}
	const stmt = `UPDATE portal_sync_runs SET finished_at = GREATEST($3, started_at), outcome = 'that-bai',
		fetched_count = 0, imported_count = 0, skipped_existing_count = 0, skipped_deleted_count = 0,
		failed_count = 0, error_summary = $4::jsonb
		WHERE tenant_id = $1 AND finished_at IS NULL AND started_at < $2
		RETURNING id`
	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), before.UTC(), at.UTC(), string(summary))
	if err != nil {
		return nil, fmt.Errorf("portal_sync_runs: reap: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("portal_sync_runs: reap: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("portal_sync_runs: reap: %w", err)
	}
	return ids, nil
}

// --- the sync's item write ---------------------------------------------------------------------------

// ExistingPortalItems reports, for each portal id already held in this commune, whether the holder is
// SOFT-DELETED (true) or live (false). Ids not held are absent. DELIBERATELY NO `deleted_at` FILTER:
// a removed article keeps its id for ever, and the run counts it as skipped_deleted, never importing
// it again (ADR 0067 §2 "Ghi" #2).
func (s *PortalSyncStore) ExistingPortalItems(ctx context.Context, externalIDs []string) (map[string]bool, error) {
	out := make(map[string]bool, len(externalIDs))
	for start := 0; start < len(externalIDs); start += existingBatch {
		end := min(start+existingBatch, len(externalIDs))
		batch := externalIDs[start:end]
		marks := make([]string, len(batch))
		args := make([]any, len(batch))
		for i, id := range batch {
			marks[i] = "$" + strconv.Itoa(i+2) // $1 is the commune (Scoped.Query)
			args[i] = id
		}
		rows, err := s.db.For(ctx).Query(ctx, "nguon_id_ngoai, deleted_at IS NOT NULL", "noi_dung_mini_app",
			"AND nguon_id_ngoai IN ("+strings.Join(marks, ", ")+")", args...)
		if err != nil {
			return nil, fmt.Errorf("noi_dung_mini_app: read portal ids: %w", err)
		}
		for rows.Next() {
			var (
				id      string
				deleted bool
			)
			if err := rows.Scan(&id, &deleted); err != nil {
				rows.Close()
				return nil, fmt.Errorf("noi_dung_mini_app: scan portal ids: %w", err)
			}
			out[id] = deleted
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("noi_dung_mini_app: read portal ids: %w", err)
		}
	}
	return out, nil
}

// insertSyncedItem — THE SYNC'S OWN STATEMENT, and it has to be a different one from the staff
// compose (chenNoiDungMiniApp): `nguon` is the LITERAL 'dong-bo-cong' here and 'thu-cong' there, so no
// caller of either can choose an item's provenance. `nguon_id_ngoai` and `portal_category_id` are
// bound (0006's equivalence and 0013's "only for synced" CHECKs hold them to this literal).
// `danh_muc_id` is a literal NULL (§2 "Chế độ đăng" #5: a synced item is not filed in the commune's
// tree until staff file it). `da_sua_tay`, `luot_xem` and the soft-delete columns are absent: a row is
// born unedited, unread and not deleted.
//
// `nguon_url` ($9) is bound as given; the run passes NULL today — the portal API as recorded carries
// no article URL, and a guessed one would be a wrong link in an archival record.
const insertSyncedItem = `INSERT INTO noi_dung_mini_app
	(tenant_id, id, loai, danh_muc_id, tieu_de, tom_tat, noi_dung, ngay_dang, trang_thai, nguon,
	 nguon_url, nguon_id_ngoai, nguoi_tao_ma, published_at, cover_image_file_id, portal_category_id)
	VALUES ($1,$2,$3,NULL,$4,$5,$6,$7,$8,'dong-bo-cong',$9,$10,$11,$12,$13,$14)`

// InsertSyncedItem writes one imported article. A unique violation (the portal id already held — a
// race the per-commune lock makes unlikely) is ErrPortalItemExists; the transaction is then unusable
// and the caller rolls it back.
func (s *PortalSyncStore) InsertSyncedItem(ctx context.Context, tx *store.ScopedTx, n domain.NoiDungMiniApp,
	portalCategoryID string) error {

	_, err := tx.Exec(ctx, insertSyncedItem, string(tx.TenantID()), n.ID, string(n.Loai), n.TieuDe,
		rongThanhNull(n.TomTat), rongThanhNull(n.NoiDung), n.NgayDang.UTC(), string(n.TrangThai),
		rongThanhNull(n.NguonURL), n.NguonIDNgoai, n.NguoiTaoMa, zeroTimeAsNull(n.PublishedAt),
		rongThanhNull(n.CoverImageFileID), portalCategoryID)
	if isUniqueViolation(err) {
		return ErrPortalItemExists
	}
	if err != nil {
		return fmt.Errorf("noi_dung_mini_app: insert synced: %w", err)
	}
	return nil
}

// isUniqueViolation — SQLSTATE 23505. The constraint name is not compared: on a partitioned table it
// is the partition's (service-finance/internal/store/budget_period_close.go says the same).
func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}
