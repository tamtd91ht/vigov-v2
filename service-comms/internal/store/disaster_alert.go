package store

// Disaster alerts — migrations/0026_disaster_alerts.sql.
//
// FOUR THINGS HOLD ACROSS EVERY METHOD, external_contact.go's four:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from Scoped / tx.TenantID(), never a parameter.
//  2. `tenant_id`, `id`, `created_at` AND `created_by` APPEAR IN NO UPDATE (the trigger refuses them too).
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS (rule 7, invariant 2) — the staff list, the detail, the
//     public read alike.
//  4. NOTHING HERE OPENS A TRANSACTION. The use case opens it and writes the audit entry inside.
//
// THE THREE JSONB LISTS travel as JSON of the domain types — the element shape was checked by
// domain.NormalizeDisasterAlert before any statement here runs; the database holds only the array floor.
//
// PERSONAL DATA (0026): the contact names and phones may be an officer's own. Nothing here logs a row or
// puts a value into an error message.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// DisasterAlertStore reads and writes disaster_alerts through Scoped / ScopedTx only (rule 1, inv. 5).
type DisasterAlertStore struct {
	db *store.DB
}

func NewDisasterAlertStore(db *store.DB) *DisasterAlertStore {
	return &DisasterAlertStore{db: db}
}

// PublicDisasterAlertCeiling bounds the public read: the alerts in force at one moment in one commune.
// 0026 expects "a handful per commune per season"; twenty at once is already a commune posting one per
// hamlet. Past it the read serves the NEWEST twenty and the handler logs — see InForce.
const PublicDisasterAlertCeiling = 20

var (
	// ErrDisasterAlertNotFound — no LIVE row with that id in THIS commune. 404, the same answer for
	// another commune's row, a deleted one and no such row.
	ErrDisasterAlertNotFound = errors.New("disaster_alert: không tồn tại trong xã này")

	// ErrTooManyAlertsInForce — the public read found more than the ceiling in force. Returned BESIDE
	// the newest PublicDisasterAlertCeiling rows, never instead of them: during a disaster an empty
	// banner is the worst answer, so the handler serves what it has and logs this.
	ErrTooManyAlertsInForce = errors.New("disaster_alert: số cảnh báo đang hiệu lực vượt trần")
)

// SortDisasterAlerts is the staff list's one sort: newest start first. `effective_from` is NOT NULL, as
// a cursor column must be (core/page).
var SortDisasterAlerts = page.NewAllowlist(page.Desc,
	page.Col("effective_from", "effective_from", page.KindTime),
)

var anchorDisasterAlert = store.NewMoc[domain.DisasterAlert](SortDisasterAlerts,
	map[string]func(domain.DisasterAlert) page.Key{
		"effective_from": func(a domain.DisasterAlert) page.Key { return page.TimeKey(a.EffectiveFrom) },
	})

// disasterAlertColumns IS READ BY POSITION in scanDisasterAlert. Adjacent TEXT columns (title, level,
// headline, detail) and adjacent JSONB columns: a swap compiles and prints a headline as a level or the
// contacts as evacuation points — the fake driver in disaster_alert_test.go addresses rows BY NAME.
const disasterAlertColumns = `id, kind, title, level, headline, detail, ` +
	`effective_from, effective_to, ended_at, ended_by, ` +
	`danger_zone, evacuation_points, emergency_contacts, ` +
	`created_at, created_by, updated_at, updated_by`

func scanDisasterAlert(scan func(...any) error) (domain.DisasterAlert, error) {
	var (
		a                                    domain.DisasterAlert
		kind                                 string
		level, headline, endedBy             sql.NullString
		effectiveTo, endedAt                 sql.NullTime
		dangerZone, evacuationPts, contactsJ []byte
	)
	if err := scan(&a.ID, &kind, &a.Title, &level, &headline, &a.Detail,
		&a.EffectiveFrom, &effectiveTo, &endedAt, &endedBy,
		&dangerZone, &evacuationPts, &contactsJ,
		&a.CreatedAt, &a.CreatedBy, &a.UpdatedAt, &a.UpdatedBy); err != nil {
		return domain.DisasterAlert{}, err
	}
	a.Kind = domain.DisasterKind(kind)
	a.Level, a.Headline, a.EndedBy = level.String, headline.String, endedBy.String
	a.EffectiveFrom = a.EffectiveFrom.UTC()
	a.EffectiveTo = nullTimeUTC(effectiveTo)
	a.EndedAt = nullTimeUTC(endedAt)
	a.CreatedAt, a.UpdatedAt = a.CreatedAt.UTC(), a.UpdatedAt.UTC()
	// The decoder's message is NOT wrapped in: it may quote the stored value, which may hold a phone.
	if json.Unmarshal(dangerZone, &a.DangerZone) != nil ||
		json.Unmarshal(evacuationPts, &a.EvacuationPoints) != nil ||
		json.Unmarshal(contactsJ, &a.EmergencyContacts) != nil {
		return domain.DisasterAlert{}, fmt.Errorf("disaster_alert: danh sách JSONB của %s không đọc được", a.ID)
	}
	if a.DangerZone == nil {
		a.DangerZone = []domain.DangerZonePoint{}
	}
	if a.EvacuationPoints == nil {
		a.EvacuationPoints = []domain.EvacuationPoint{}
	}
	if a.EmergencyContacts == nil {
		a.EmergencyContacts = []domain.EmergencyContact{}
	}
	return a, nil
}

// List reads one page of the commune's live alerts — every state (upcoming, in force, past, ended) —
// newest start first. The staff tab shows them all; whether one is in force is derived from its times.
func (s *DisasterAlertStore) List(ctx context.Context, req page.Request) (page.Result[domain.DisasterAlert], error) {
	// Scoped: QueryPage reads through db.For(ctx).Query — tenant_id is $1.
	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: disasterAlertColumns,
		Table:   "disaster_alerts",
		Filter:  `AND deleted_at IS NULL`,
	}, req, anchorDisasterAlert, func(rows *sql.Rows) (domain.DisasterAlert, string, error) {
		a, err := scanDisasterAlert(rows.Scan)
		if err != nil {
			return domain.DisasterAlert{}, "", fmt.Errorf("disaster_alert: đọc dòng: %w", err)
		}
		return a, a.ID, nil
	})
}

// ByID reads one live alert of the commune.
func (s *DisasterAlertStore) ByID(ctx context.Context, id string) (domain.DisasterAlert, error) {
	// Scoped: db.For(ctx).Query binds the commune as $1.
	rows, err := s.db.For(ctx).Query(ctx, disasterAlertColumns, "disaster_alerts",
		`AND id = $2 AND deleted_at IS NULL`, id)
	if err != nil {
		return domain.DisasterAlert{}, fmt.Errorf("disaster_alert: đọc: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.DisasterAlert{}, fmt.Errorf("disaster_alert: duyệt: %w", err)
		}
		return domain.DisasterAlert{}, ErrDisasterAlertNotFound
	}
	a, err := scanDisasterAlert(rows.Scan)
	if err != nil {
		return domain.DisasterAlert{}, fmt.Errorf("disaster_alert: đọc dòng: %w", err)
	}
	return a, rows.Err()
}

// InForce reads the alerts residents are shown at `now`: live, not ended, started, and not past their end
// — [effective_from, effective_to) — newest start first, at most PublicDisasterAlertCeiling. `now` is a
// parameter rather than the database's now() so one clock decides the whole reply and a test can pin it.
//
// Past the ceiling it returns the newest PublicDisasterAlertCeiling rows AND ErrTooManyAlertsInForce.
func (s *DisasterAlertStore) InForce(ctx context.Context, now time.Time) ([]domain.DisasterAlert, error) {
	// Scoped: db.For(ctx).Query binds the commune as $1; the partial index disaster_alerts_public
	// (tenant_id, effective_from) WHERE deleted_at IS NULL serves it.
	rows, err := s.db.For(ctx).Query(ctx, disasterAlertColumns, "disaster_alerts",
		`AND deleted_at IS NULL AND ended_at IS NULL AND effective_from <= $2 `+
			`AND (effective_to IS NULL OR effective_to > $2) `+
			`ORDER BY effective_from DESC, id DESC LIMIT $3`, now.UTC(), PublicDisasterAlertCeiling+1)
	if err != nil {
		return nil, fmt.Errorf("disaster_alert: đọc cảnh báo đang hiệu lực: %w", err)
	}
	defer rows.Close()

	out := make([]domain.DisasterAlert, 0, 4)
	for rows.Next() {
		a, err := scanDisasterAlert(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("disaster_alert: đọc dòng: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("disaster_alert: duyệt: %w", err)
	}
	if len(out) > PublicDisasterAlertCeiling {
		return out[:PublicDisasterAlertCeiling], ErrTooManyAlertsInForce
	}
	return out, nil
}

// ByIDForUpdate reads one LIVE row and locks it until the transaction ends (read-decide-write, the
// reason on ExternalContactStore.ByIDForUpdate).
func (s *DisasterAlertStore) ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.DisasterAlert, error) {
	const stmt = `SELECT ` + disasterAlertColumns + ` FROM disaster_alerts ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	a, err := scanDisasterAlert(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DisasterAlert{}, ErrDisasterAlertNotFound
	}
	if err != nil {
		return domain.DisasterAlert{}, fmt.Errorf("disaster_alert: đọc dòng để sửa: %w", err)
	}
	return a, nil
}

// alertLists renders the three JSONB lists. Nil slices become `[]` — the columns are NOT NULL arrays.
func alertLists(a domain.DisasterAlert) (zone, evac, contacts []byte, err error) {
	if a.DangerZone == nil {
		a.DangerZone = []domain.DangerZonePoint{}
	}
	if a.EvacuationPoints == nil {
		a.EvacuationPoints = []domain.EvacuationPoint{}
	}
	if a.EmergencyContacts == nil {
		a.EmergencyContacts = []domain.EmergencyContact{}
	}
	if zone, err = json.Marshal(a.DangerZone); err != nil {
		return nil, nil, nil, fmt.Errorf("disaster_alert: mã hoá vùng nguy hiểm: %w", err)
	}
	if evac, err = json.Marshal(a.EvacuationPoints); err != nil {
		return nil, nil, nil, fmt.Errorf("disaster_alert: mã hoá điểm sơ tán: %w", err)
	}
	if contacts, err = json.Marshal(a.EmergencyContacts); err != nil {
		return nil, nil, nil, fmt.Errorf("disaster_alert: mã hoá số khẩn cấp: %w", err)
	}
	return zone, evac, contacts, nil
}

const insertDisasterAlert = `INSERT INTO disaster_alerts
	(tenant_id, id, kind, title, level, headline, detail, effective_from, effective_to,
	 danger_zone, evacuation_points, emergency_contacts, created_by, updated_by)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13)`

// Insert adds one alert. `by` is the actor's BUSINESS CODE (rule 6, invariant 8).
func (s *DisasterAlertStore) Insert(ctx context.Context, tx *store.ScopedTx, a domain.DisasterAlert, by string) error {
	zone, evac, contacts, err := alertLists(a)
	if err != nil {
		return err
	}
	// Scoped: tenant_id is $1 from tx.TenantID().
	if _, err := tx.Exec(ctx, insertDisasterAlert, string(tx.TenantID()), a.ID, string(a.Kind), a.Title,
		nullText(a.Level), nullText(a.Headline), a.Detail, a.EffectiveFrom.UTC(), zeroTimeAsNull(a.EffectiveTo),
		zone, evac, contacts, by); err != nil {
		return fmt.Errorf("disaster_alert: chèn: %w", err)
	}
	return nil
}

// updateDisasterAlert — the fields staff may correct while the alert is live and not ended. `AND
// ended_at IS NULL`: an ended alert is a historical record (0026's guard refuses it underneath).
const updateDisasterAlert = `UPDATE disaster_alerts SET kind = $3, title = $4, level = $5, headline = $6,
	detail = $7, effective_from = $8, effective_to = $9, danger_zone = $10, evacuation_points = $11,
	emergency_contacts = $12, updated_at = now(), updated_by = $13
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND ended_at IS NULL`

func (s *DisasterAlertStore) Update(ctx context.Context, tx *store.ScopedTx, a domain.DisasterAlert, by string) error {
	zone, evac, contacts, err := alertLists(a)
	if err != nil {
		return err
	}
	// Scoped: tenant_id is $1 from tx.TenantID().
	res, err := tx.Exec(ctx, updateDisasterAlert, string(tx.TenantID()), a.ID, string(a.Kind), a.Title,
		nullText(a.Level), nullText(a.Headline), a.Detail, a.EffectiveFrom.UTC(), zeroTimeAsNull(a.EffectiveTo),
		zone, evac, contacts, by)
	if err != nil {
		return fmt.Errorf("disaster_alert: cập nhật: %w", err)
	}
	return disasterAlertOneRow(res, "cập nhật")
}

// endDisasterAlert — staff end the alert early: when and who, both in one statement (0026's
// `ended_complete`). `AND ended_at IS NULL`: a second end cannot overwrite who ended it.
const endDisasterAlert = `UPDATE disaster_alerts SET ended_at = now(), ended_by = $3, updated_at = now(),
	updated_by = $3 WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND ended_at IS NULL`

func (s *DisasterAlertStore) End(ctx context.Context, tx *store.ScopedTx, id, by string) error {
	// Scoped: tenant_id is $1 from tx.TenantID().
	res, err := tx.Exec(ctx, endDisasterAlert, string(tx.TenantID()), id, by)
	if err != nil {
		return fmt.Errorf("disaster_alert: kết thúc: %w", err)
	}
	return disasterAlertOneRow(res, "kết thúc")
}

// softDeleteDisasterAlert writes all three columns of rule 7 invariant 1 in one statement. An ENDED alert
// may still be soft deleted (0026's guard lets exactly these columns move).
const softDeleteDisasterAlert = `UPDATE disaster_alerts SET deleted_at = now(), deleted_by = $3,
	delete_reason = $4, updated_at = now(), updated_by = $3
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// SoftDelete retires one alert. There is no hard delete in this package; the trigger refuses one.
func (s *DisasterAlertStore) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error {
	// Scoped: tenant_id is $1 from tx.TenantID().
	res, err := tx.Exec(ctx, softDeleteDisasterAlert, string(tx.TenantID()), id, by, reason)
	if err != nil {
		return fmt.Errorf("disaster_alert: xoá mềm: %w", err)
	}
	return disasterAlertOneRow(res, "xoá mềm")
}

func disasterAlertOneRow(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("disaster_alert: %s: đọc số dòng: %w", op, err)
	}
	if n == 0 {
		return ErrDisasterAlertNotFound
	}
	return nil
}
