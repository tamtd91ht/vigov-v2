package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/core/audit"
	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// RegistryWriter is the operator console's WRITE side of the commune registry (ADR 0048 §01/10 #4).
//
// EVERY METHOD IS ONE TRANSACTION, AND THE AUDIT ENTRY IS INSIDE IT (rule 6 invariant 3; rule 2
// invariant 6). The entry goes to THIS service's audit_log with tenant_id = the TARGET commune
// (ADR 0048 §Thiết kế #6) — for a create, the commune just created.
//
// THE TARGET COMMUNE TRAVELS IN THE CONTEXT (rule 1 invariant 4): the handler puts the commune of
// the path — or the new ULID of a create — into ctx with tenant.Into, and corestore.For reads it from
// there. It is never taken from a body or a header.
//
// NOTHING HERE DELETES, and every UPDATE names its row (rule 7). No route removes a domain or moves
// one to another commune (ADR 0048 §01/10 #5) — there is no method for either.
type RegistryWriter struct {
	db *corestore.DB
}

// NewRegistryWriter builds the writer over the scoped handle.
func NewRegistryWriter(db *corestore.DB) *RegistryWriter { return &RegistryWriter{db: db} }

var (
	ErrProvinceNotFound   = errors.New("operator registry: không có tỉnh này trong danh mục")
	ErrDomainTaken        = errors.New("operator registry: tên miền đã thuộc một xã")
	ErrDuplicateName      = errors.New("operator registry: đã có xã trùng tên trong tỉnh")
	ErrCommuneInactive    = errors.New("operator registry: xã đã ngừng hoạt động")
	ErrDomainNotInCommune = errors.New("operator registry: tên miền không thuộc xã này")
	ErrMiniAppTaken       = errors.New("operator registry: App ID đã có trong sổ mini_app")
	// ErrCommuneSucceeded — reactivation refused: tenant_succession names this commune as a
	// predecessor (a merged or split unit). See SetActivation.
	ErrCommuneSucceeded = errors.New("operator registry: xã đã được kế thừa bởi đơn vị khác — không mở lại")
	// ErrMiniAppNotInCommune — the App ID is not a not-deleted `rieng` row of THIS commune. Unknown,
	// soft-deleted, a main app, and a row bound to ANOTHER commune are one answer on purpose: the
	// operator console must not learn which commune holds an App ID through a refusal, and no path
	// here can move a row across communes (ADR 0070 #3).
	ErrMiniAppNotInCommune = errors.New("operator registry: App ID không phải Mini App riêng của xã này")
	// ErrMiniAppInactive — a replacement names an old App ID that is switched off but not removed: a
	// row written before 05/10/2026, when "gỡ" only switched off. The console removes it instead.
	ErrMiniAppInactive = errors.New("operator registry: App ID đã tắt")
	// ErrMiniAppAlreadyRunning — attaching would leave the commune with two running dedicated apps
	// (ADR 0070 #1). The console uses the replacement route instead.
	ErrMiniAppAlreadyRunning = errors.New("operator registry: xã đã có một Mini App riêng đang chạy")
	// ErrMiniAppRemoved — the App ID to bind has a SOFT-DELETED row (removed or replaced, by any
	// commune). Its own error, not ErrMiniAppTaken, so the operator reads WHY it can never be bound
	// again (ADR 0070 §Sửa đổi 05/10/2026 #2: the key keeps removed App IDs). It names no commune.
	ErrMiniAppRemoved = errors.New("operator registry: App ID đã bị gỡ trước đây, không gắn lại được")
)

// Audit actions written to audit_log.action. VALUES ARE VIETNAMESE snake_case, as every commune
// audit_log in this repository writes them (ADR 0011 for values; rule 12 covers identifiers). Two of
// them are not new: `tao_xa` and `gan_mini_app` are what the Jenkins stages wrote for the same acts
// (deploy/Jenkinsfile), so one query finds every creation whichever channel did it.
const (
	ActionCreateCommune    = "tao_xa"
	ActionAddDomain        = "them_ten_mien"
	ActionSetPrimaryDomain = "dat_ten_mien_chinh"
	ActionCorrectName      = "sua_ten_xa"
	ActionDeactivate       = "ngung_hoat_dong_xa"
	ActionReactivate       = "mo_lai_hoat_dong_xa"
	ActionAttachMiniApp    = "gan_mini_app"
	// `tat_mini_app` is what the Jenkins stage `doi-app-id-thang-binh` wrote for the same act. Since
	// 05/10/2026 the act it names is a SOFT DELETE; the delta says so (`da_xoa_mem`, removalDelta).
	// The verb is kept so one query still finds every removal, whichever channel and date did it.
	// `bat_lai_mini_app` is no longer written: reactivation was withdrawn (ADR 0070 §Sửa đổi
	// 05/10/2026 #2). Rows written before that keep it; the operator log still labels them.
	ActionDeactivateMiniApp = "tat_mini_app"
	// The two secret-forward verbs are NEW (ADR 0073 §Hệ quả): this service only relays the secret to
	// identity, whose operator trail stays the primary record (ADR 0070 bổ sung #7); these rows exist
	// so the operator log screen sees the act. Metadata only — RecordMiniAppSecretForward.
	ActionSetMiniAppSecret    = "dat_khoa_mini_app"
	ActionRetireMiniAppSecret = "thu_hoi_khoa_mini_app"
)

// nameLock serialises every write that can create a duplicate commune name. There is NO unique
// index able to say "same name in the same province" (the comparison is domain.NameKey over a
// province key that also folds legacy spellings), so two concurrent creates would both pass the
// check without it. Operator writes are a handful a day, so one platform-wide transaction lock
// costs nothing. The DDL that would make the database own this is reported as a follow-up.
const nameLock = `SELECT pg_advisory_xact_lock(hashtext('platform.operator.tenant_name'))`

func (w *RegistryWriter) actor(a domain.OperatorActor) (audit.Actor, error) {
	if err := a.Validate(); err != nil {
		return audit.Actor{}, err
	}
	return audit.Actor{ID: a.Code, Kind: domain.AuditKindOperator, IP: a.IP}, nil
}

func delta(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		// Every value marshalled here is a map of strings and bools; Marshal cannot fail on it.
		panic("operator registry: delta: " + err.Error())
	}
	return b
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

// reservedHostConstraint is migration 0007's CHECK on tenant_domain.
const reservedHostConstraint = "tenant_domain_khong_danh_rieng"

// domainWriteError maps a failed tenant_domain INSERT/UPDATE to the registry's sentinels.
//
// WHY 23514 IS MAPPED AND NOT LEFT AS A 500: the Go check (domain.IsReservedCommuneHost) runs first,
// but the CHECK is the database's own copy of the rule, and it also fires on a path the Go check
// cannot see — an UPDATE touching one of the two frozen admin*.vigov.vn rows kept by rule 7 (0007's
// "CONSEQUENCE FOR THE TWO KEPT ROWS"), e.g. clearing la_chinh on one when the primary moves. That is
// the operator naming a reserved host, not an outage: a 500 would send them to the platform team for
// a refusal that is working as designed. Matched by CONSTRAINT NAME, so another CHECK violation
// (a different rule) still surfaces as the unexpected error it is.
func domainWriteError(err error, what string) error {
	var pg *pgconn.PgError
	switch {
	case isUniqueViolation(err):
		return ErrDomainTaken
	case errors.As(err, &pg) && pg.Code == "23514" && pg.ConstraintName == reservedHostConstraint:
		return domain.ErrCommuneHostReserved
	}
	return fmt.Errorf("operator registry: %s: %w", what, err)
}

// NewCommune is what a create needs. The id is NOT here: it is the commune in ctx.
type NewCommune struct {
	Name       string // validated + NFC by the caller
	ProvinceID string
	Host       string // domain.ParseCommuneHost output
}

// CreateCommune inserts the tenant row, its primary host and the `tao_xa` entry — the checks the
// Jenkins stage `tao-xa-thang-binh` made, in the same order, in one transaction.
//
// It seeds NOTHING of the commune's internal configuration (roles, SLA, hours, holidays, staff):
// that is the commune administrator's (ADR 0048 §01/10 #1).
func (w *RegistryWriter) CreateCommune(ctx context.Context, in NewCommune, by domain.OperatorActor) (domain.Commune, error) {
	act, err := w.actor(by)
	if err != nil {
		return domain.Commune{}, err
	}
	s := w.db.For(ctx)
	id := s.TenantID().String()
	var province string
	err = s.Tx(ctx, func(tx *corestore.ScopedTx) error {
		if _, err := tx.Exec(ctx, nameLock); err != nil {
			return fmt.Errorf("operator registry: lock: %w", err)
		}
		raw := tx.Underlying()
		// The catalogue has no commune column; an inactive province is not offered and not accepted.
		err := raw.QueryRowContext(ctx,
			`SELECT ten FROM tinh_thanh WHERE id = $1 AND dang_hoat_dong`, in.ProvinceID).Scan(&province)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrProvinceNotFound
		case err != nil:
			return fmt.Errorf("operator registry: province: %w", err)
		}
		if err := hostFree(ctx, raw, in.Host); err != nil {
			return err
		}
		if err := nameFree(ctx, raw, in.Name, province, ""); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO tenant (id, ten, tinh_thanh, dang_hoat_dong) VALUES ($1, $2, $3, true)`,
			id, in.Name, province); err != nil {
			return fmt.Errorf("operator registry: insert tenant: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO tenant_domain (host, tenant_id, la_chinh) VALUES ($1, $2, true)`,
			in.Host, id); err != nil {
			return domainWriteError(err, "insert domain")
		}
		// The delta keeps the Jenkins stage's shape: {ten, tinh_thanh, ten_mien}.
		return audit.Write(ctx, tx, audit.Entry{
			Actor: act, Action: ActionCreateCommune, Subject: in.Name,
			Delta: delta(map[string]any{"ten": in.Name, "tinh_thanh": province, "ten_mien": []string{in.Host}}),
		})
	})
	if err != nil {
		return domain.Commune{}, err
	}
	return domain.Commune{ID: id, Name: in.Name, Province: province, Active: true, Domains: []string{in.Host}}, nil
}

// hostFree refuses a host any commune already holds — active, inactive, or the two frozen
// admin*.vigov.vn rows (rule 7 keeps them; ParseCommuneHost already refuses those names).
func hostFree(ctx context.Context, raw *sql.Tx, host string) error {
	var one int
	// @cross-tenant: tenant_domain.host is globally unique by design (migration 0001) — "is this host
	// held by ANY commune" is the only question that keeps one host from naming two communes.
	err := raw.QueryRowContext(ctx, `SELECT 1 FROM tenant_domain WHERE host = $1`, host).Scan(&one)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("operator registry: host lookup: %w", err)
	}
	return ErrDomainTaken
}

// nameFree refuses a name equal (domain.NameKey) to another commune's in the same province
// (domain.ProvinceKey). exceptID is the commune being renamed, so a case-only correction of its own
// name is not a duplicate of itself. Inactive communes count: a merged commune keeps its name.
func nameFree(ctx context.Context, raw *sql.Tx, name, province, exceptID string) error {
	// @cross-tenant: the duplicate-name check reads the name and province of every commune — registry
	// metadata only, the same columns the operator list shows (ADR 0048 §30/09 #5).
	rows, err := raw.QueryContext(ctx, `SELECT id, ten, tinh_thanh FROM tenant`)
	if err != nil {
		return fmt.Errorf("operator registry: name lookup: %w", err)
	}
	defer rows.Close()
	want, wantProvince := domain.NameKey(name), domain.ProvinceKey(province)
	for rows.Next() {
		var id, ten, tinh string
		if err := rows.Scan(&id, &ten, &tinh); err != nil {
			return fmt.Errorf("operator registry: name lookup: %w", err)
		}
		if id != exceptID && domain.NameKey(ten) == want && domain.ProvinceKey(tinh) == wantProvince {
			return ErrDuplicateName
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("operator registry: name lookup: %w", err)
	}
	return nil
}

// current reads and LOCKS the target commune's row for the rest of the transaction.
type communeRow struct {
	name, province string
	active         bool
}

func lockCommune(ctx context.Context, tx *corestore.ScopedTx) (communeRow, error) {
	var c communeRow
	err := tx.Underlying().QueryRowContext(ctx,
		`SELECT ten, tinh_thanh, dang_hoat_dong FROM tenant WHERE id = $1 FOR UPDATE`,
		tx.TenantID().String()).Scan(&c.name, &c.province, &c.active)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return communeRow{}, ErrCommuneNotFound
	case err != nil:
		return communeRow{}, fmt.Errorf("operator registry: lock commune: %w", err)
	}
	return c, nil
}

// AddDomain adds a NON-primary host to the commune in ctx. Refused on an inactive commune: rule 7
// invariant 6 keeps a merged commune's registry unchanged, and its hosts do not resolve anyway.
func (w *RegistryWriter) AddDomain(ctx context.Context, host string, by domain.OperatorActor) error {
	act, err := w.actor(by)
	if err != nil {
		return err
	}
	s := w.db.For(ctx)
	return s.Tx(ctx, func(tx *corestore.ScopedTx) error {
		c, err := lockCommune(ctx, tx)
		if err != nil {
			return err
		}
		if !c.active {
			return ErrCommuneInactive
		}
		if err := hostFree(ctx, tx.Underlying(), host); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO tenant_domain (host, tenant_id, la_chinh) VALUES ($1, $2, false)`,
			host, tx.TenantID().String()); err != nil {
			return domainWriteError(err, "insert domain")
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: act, Action: ActionAddDomain, Subject: host,
			Delta: delta(map[string]any{"ten_mien": host, "la_chinh": false, "xa": c.name}),
		})
	})
}

// SetPrimaryDomain makes host the commune's canonical host. changed=false when it already was — no
// write and no entry, because nothing happened.
func (w *RegistryWriter) SetPrimaryDomain(ctx context.Context, host string, by domain.OperatorActor) (changed bool, err error) {
	act, err := w.actor(by)
	if err != nil {
		return false, err
	}
	s := w.db.For(ctx)
	err = s.Tx(ctx, func(tx *corestore.ScopedTx) error {
		c, err := lockCommune(ctx, tx)
		if err != nil {
			return err
		}
		if !c.active {
			return ErrCommuneInactive
		}
		raw := tx.Underlying()
		var isPrimary bool
		err = raw.QueryRowContext(ctx,
			`SELECT la_chinh FROM tenant_domain WHERE tenant_id = $1 AND host = $2`,
			tx.TenantID().String(), host).Scan(&isPrimary)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrDomainNotInCommune
		case err != nil:
			return fmt.Errorf("operator registry: domain lookup: %w", err)
		}
		if isPrimary {
			return nil
		}
		var before sql.NullString
		if err := raw.QueryRowContext(ctx,
			`SELECT host FROM tenant_domain WHERE tenant_id = $1 AND la_chinh`,
			tx.TenantID().String()).Scan(&before); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("operator registry: primary lookup: %w", err)
		}
		// Clear first, then set: tenant_domain_mot_chinh allows ONE primary per commune at any
		// statement boundary.
		if _, err := tx.Exec(ctx,
			`UPDATE tenant_domain SET la_chinh = false WHERE tenant_id = $1 AND la_chinh`,
			tx.TenantID().String()); err != nil {
			return domainWriteError(err, "clear primary")
		}
		if _, err := tx.Exec(ctx,
			`UPDATE tenant_domain SET la_chinh = true WHERE tenant_id = $1 AND host = $2`,
			tx.TenantID().String(), host); err != nil {
			return domainWriteError(err, "set primary")
		}
		changed = true
		return audit.Write(ctx, tx, audit.Entry{
			Actor: act, Action: ActionSetPrimaryDomain, Subject: host,
			Delta: delta(map[string]any{
				"truoc": map[string]any{"ten_mien_chinh": before.String},
				"sau":   map[string]any{"ten_mien_chinh": host},
			}),
		})
	})
	return changed, err
}

// CorrectName fixes a TYPO in the commune's name (ADR 0048 §01/10 #5). The reason is mandatory and
// is the only thing that tells a typo fix from a renaming of an administrative unit — which is a
// merger-class change this route must not be used for (rule 1 stop condition #3). changed=false when
// the name is identical.
func (w *RegistryWriter) CorrectName(ctx context.Context, name, reason string, by domain.OperatorActor) (changed bool, err error) {
	act, err := w.actor(by)
	if err != nil {
		return false, err
	}
	s := w.db.For(ctx)
	err = s.Tx(ctx, func(tx *corestore.ScopedTx) error {
		if _, err := tx.Exec(ctx, nameLock); err != nil {
			return fmt.Errorf("operator registry: lock: %w", err)
		}
		c, err := lockCommune(ctx, tx)
		if err != nil {
			return err
		}
		if !c.active {
			return ErrCommuneInactive
		}
		if c.name == name {
			return nil
		}
		if err := nameFree(ctx, tx.Underlying(), name, c.province, tx.TenantID().String()); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE tenant SET ten = $2, cap_nhat_luc = now() WHERE id = $1`,
			tx.TenantID().String(), name); err != nil {
			return fmt.Errorf("operator registry: update name: %w", err)
		}
		changed = true
		return audit.Write(ctx, tx, audit.Entry{
			Actor: act, Action: ActionCorrectName, Subject: name,
			Delta: delta(map[string]any{
				"truoc": map[string]any{"ten": c.name},
				"sau":   map[string]any{"ten": name},
				"ly_do": reason,
			}),
		})
	})
	return changed, err
}

// SetActivation switches the commune on or off, with a mandatory reason. changed=false when it was
// already in that state. Returns the commune's hosts so the caller can drop them from its cache.
//
// WHAT DEACTIVATING DOES, AND WHEN: ByHost refuses an inactive commune, so its hosts stop resolving
// — immediately in this process once the caller forgets them, and within one TENANT_CACHE_TTL (30 s
// by default) in every other service's cached directory. Nothing is deleted (rule 7 invariant 6).
func (w *RegistryWriter) SetActivation(ctx context.Context, active bool, reason string, by domain.OperatorActor) (changed bool, hosts []string, err error) {
	act, err := w.actor(by)
	if err != nil {
		return false, nil, err
	}
	s := w.db.For(ctx)
	err = s.Tx(ctx, func(tx *corestore.ScopedTx) error {
		c, err := lockCommune(ctx, tx)
		if err != nil {
			return err
		}
		rows, err := tx.Underlying().QueryContext(ctx,
			`SELECT host FROM tenant_domain WHERE tenant_id = $1 ORDER BY host`, tx.TenantID().String())
		if err != nil {
			return fmt.Errorf("operator registry: hosts: %w", err)
		}
		for rows.Next() {
			var h string
			if err := rows.Scan(&h); err != nil {
				rows.Close()
				return fmt.Errorf("operator registry: hosts: %w", err)
			}
			hosts = append(hosts, h)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("operator registry: hosts: %w", err)
		}
		if active {
			// A MERGED OR SPLIT COMMUNE IS NEVER SWITCHED BACK ON HERE. tenant_succession naming this
			// commune as a predecessor (tu_id) is the administrative record that another unit took
			// over its territory and its work (migration 0003, on the authority in `can_cu`). Rule 7
			// invariant 6 keeps such a commune INACTIVE with its data unchanged; reactivating it would
			// make two communes answer for one territory and send citizens' new petitions to an
			// authority that no longer exists. Undoing a reorganisation is a merger-class decision
			// (rule 1 stop condition #3, skills/admin-unit-merge) — taken by the owner from a
			// document, never by a toggle. Checked before the "already active" no-op so the answer
			// does not depend on the row's current state. Deactivation is unaffected.
			var one int
			err := tx.Underlying().QueryRowContext(ctx,
				`SELECT 1 FROM tenant_succession WHERE tu_id = $1 LIMIT 1`, tx.TenantID().String()).Scan(&one)
			switch {
			case err == nil:
				return ErrCommuneSucceeded
			case !errors.Is(err, sql.ErrNoRows):
				return fmt.Errorf("operator registry: succession lookup: %w", err)
			}
		}
		if c.active == active {
			return nil
		}
		if _, err := tx.Exec(ctx,
			`UPDATE tenant SET dang_hoat_dong = $2, cap_nhat_luc = now() WHERE id = $1`,
			tx.TenantID().String(), active); err != nil {
			return fmt.Errorf("operator registry: update activation: %w", err)
		}
		changed = true
		action := ActionDeactivate
		if active {
			action = ActionReactivate
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: act, Action: action, Subject: c.name,
			Delta: delta(map[string]any{
				"truoc": map[string]any{"dang_hoat_dong": c.active},
				"sau":   map[string]any{"dang_hoat_dong": active},
				"ly_do": reason,
			}),
		})
	})
	return changed, hosts, err
}

// AttachMiniApp registers the commune's OWN Mini App (che_do 'rieng') — the Jenkins stage
// `gan-mini-app-thang-binh`, with `tao_boi` = the operator's business code instead of
// `van-hanh:<user>`. Refused when the commune is inactive, when the App ID has ANY row,
// soft-deleted included: an App ID once bound is never silently re-bound (rule 7 invariant 3) — and
// when the commune already has a RUNNING dedicated app: attaching beside it would leave two apps
// serving one commune, which ADR 0070 #1 rules out. Changing the App ID is ReplaceMiniApp.
func (w *RegistryWriter) AttachMiniApp(ctx context.Context, appID, note string, by domain.OperatorActor) (domain.CommuneMiniApp, error) {
	act, err := w.actor(by)
	if err != nil {
		return domain.CommuneMiniApp{}, err
	}
	s := w.db.For(ctx)
	var out domain.CommuneMiniApp
	err = s.Tx(ctx, func(tx *corestore.ScopedTx) error {
		c, err := lockCommune(ctx, tx)
		if err != nil {
			return err
		}
		if !c.active {
			return ErrCommuneInactive
		}
		if err := appIDFree(ctx, tx.Underlying(), appID); err != nil {
			return err
		}
		if err := noOtherRunningApp(ctx, tx, ""); err != nil {
			return err
		}
		out, err = insertOwnMiniApp(ctx, tx, appID, note, by.Code)
		if err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: act, Action: ActionAttachMiniApp, Subject: "MiniApp " + appID,
			Delta: delta(map[string]any{"app_id": appID, "che_do": string(domain.CheDoRieng), "xa": c.name}),
		})
	})
	if err != nil {
		return domain.CommuneMiniApp{}, err
	}
	return out, nil
}

// appIDFree refuses an App ID that has ANY row — any commune, the main app, soft-deleted included.
// A soft-deleted row is ErrMiniAppRemoved, every other row ErrMiniAppTaken.
func appIDFree(ctx context.Context, raw *sql.Tx, appID string) error {
	var deleted bool
	// @cross-tenant: mini_app.app_id is globally unique by design (migration 0006) and keeps
	// soft-deleted rows — "has this App ID EVER been registered" spans every commune.
	err := raw.QueryRowContext(ctx, `SELECT deleted_at IS NOT NULL FROM mini_app WHERE app_id = $1`, appID).Scan(&deleted)
	switch {
	case err == nil && deleted:
		return ErrMiniAppRemoved
	case err == nil:
		return ErrMiniAppTaken
	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("operator registry: mini app lookup: %w", err)
	}
	return nil
}

// noOtherRunningApp refuses when the commune in tx has an active, not-deleted `rieng` row other than
// exceptAppID. It holds against a concurrent write only because every caller has locked the tenant
// row first (lockCommune): every write that can switch a dedicated app on goes through that lock.
func noOtherRunningApp(ctx context.Context, tx *corestore.ScopedTx, exceptAppID string) error {
	var one int
	err := tx.Underlying().QueryRowContext(ctx,
		`SELECT 1 FROM mini_app
		  WHERE tenant_id = $1 AND che_do = 'rieng' AND dang_hoat_dong AND deleted_at IS NULL AND app_id <> $2
		  LIMIT 1`,
		tx.TenantID().String(), exceptAppID).Scan(&one)
	switch {
	case err == nil:
		return ErrMiniAppAlreadyRunning
	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("operator registry: running mini app lookup: %w", err)
	}
	return nil
}

func insertOwnMiniApp(ctx context.Context, tx *corestore.ScopedTx, appID, note, byCode string) (domain.CommuneMiniApp, error) {
	var out domain.CommuneMiniApp
	if err := tx.Underlying().QueryRowContext(ctx,
		`INSERT INTO mini_app (app_id, che_do, tenant_id, ghi_chu, tao_boi, cap_nhat_boi)
		 VALUES ($1, 'rieng', $2, $3, $4, $4)
		 RETURNING app_id, che_do, dang_hoat_dong, tao_luc, tao_boi`,
		appID, tx.TenantID().String(), note, byCode).
		Scan(&out.AppID, &out.Mode, &out.Active, &out.CreatedAt, &out.CreatedBy); err != nil {
		if isUniqueViolation(err) {
			return domain.CommuneMiniApp{}, ErrMiniAppTaken
		}
		return domain.CommuneMiniApp{}, fmt.Errorf("operator registry: insert mini app: %w", err)
	}
	return out, nil
}

// ownMiniAppRow is one `rieng` row of the commune in tx, soft-deleted or not, read under lock.
type ownMiniAppRow struct {
	active  bool
	deleted bool
}

// lockOwnMiniApp reads and LOCKS one `rieng` row of the commune in tx — SOFT-DELETED ROWS INCLUDED,
// so a caller can tell "already removed from this commune" from "never this commune's". The tenant
// filter is the point: a row of another commune is ErrMiniAppNotInCommune, exactly like an unknown
// App ID (see that error).
func lockOwnMiniApp(ctx context.Context, tx *corestore.ScopedTx, appID string) (ownMiniAppRow, error) {
	var row ownMiniAppRow
	err := tx.Underlying().QueryRowContext(ctx,
		`SELECT dang_hoat_dong, deleted_at IS NOT NULL FROM mini_app
		  WHERE app_id = $1 AND tenant_id = $2 AND che_do = 'rieng'
		  FOR UPDATE`,
		appID, tx.TenantID().String()).Scan(&row.active, &row.deleted)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ownMiniAppRow{}, ErrMiniAppNotInCommune
	case err != nil:
		return ownMiniAppRow{}, fmt.Errorf("operator registry: lock mini app: %w", err)
	}
	return row, nil
}

// softDeleteOwnMiniApp removes one not-deleted row the caller has locked with lockOwnMiniApp (ADR
// 0070 §Sửa đổi 05/10/2026 #2): deleted_at/deleted_by/delete_reason, and dang_hoat_dong = false as
// well. The resolver already refuses on either column (Directory.MiniApp), so the second is not what
// stops sign-ins; it is set so that no reader filtering on one column alone — a report, a future
// query — ever sees a removed App ID as running. The row is never deleted: it decided the commune of
// every earlier citizen session (rule 7), and its key keeps the App ID from ever being bound again.
func softDeleteOwnMiniApp(ctx context.Context, tx *corestore.ScopedTx, appID, reason, byCode string) error {
	res, err := tx.Exec(ctx,
		`UPDATE mini_app SET dang_hoat_dong = false, cap_nhat_luc = now(), cap_nhat_boi = $3,
		        deleted_at = now(), deleted_by = $3, delete_reason = $4
		  WHERE app_id = $1 AND tenant_id = $2 AND che_do = 'rieng' AND deleted_at IS NULL`,
		appID, tx.TenantID().String(), byCode, reason)
	if err != nil {
		return fmt.Errorf("operator registry: soft delete mini app: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("operator registry: soft delete mini app: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("operator registry: soft delete mini app: %d rows changed, want 1", n)
	}
	return nil
}

// removalDelta is the before/after of a removal, the same in both acts that remove: `da_xoa_mem`
// says in the trail itself that the row was soft-deleted, not merely switched off.
func removalDelta(appID string, wasActive bool, reason string) map[string]any {
	return map[string]any{
		"app_id": appID,
		"truoc":  map[string]any{"dang_hoat_dong": wasActive, "da_xoa_mem": false},
		"sau":    map[string]any{"dang_hoat_dong": false, "da_xoa_mem": true},
		"ly_do":  reason,
	}
}

// ReplaceMiniApp CHANGES the commune's dedicated App ID (ADR 0070 #1, §Sửa đổi 05/10/2026 #2): one
// transaction — insert the new `rieng` row, SOFT-DELETE the old one (never a hard delete: it decided
// the commune of every earlier citizen session, rule 7), and write `gan_mini_app` (with `thay_cho`)
// and `tat_mini_app` (before/after with `da_xoa_mem`, reason). All or nothing: half of it — old
// removed, new missing — is a commune whose citizens can open no app at all.
//
// Refused, writing nothing: commune unknown or inactive; old App ID not a not-deleted `rieng` row of
// THIS commune; old App ID switched off (a legacy state — remove it instead); new App ID already has
// any row, soft-deleted included (ErrMiniAppRemoved for those).
//
// Other running dedicated apps of the commune are not checked: a replacement never increases their
// number, and attach refuses to create a second one.
//
// WHAT THIS DOES NOT DO: the identity secret of either App ID, vihat-miniapp's environment, the
// citizen-app build — kb/30-indexes/transaction-boundaries.json `doi_app_id_mini_app_cua_xa`.
func (w *RegistryWriter) ReplaceMiniApp(ctx context.Context, oldAppID, newAppID, reason string, by domain.OperatorActor) (domain.CommuneMiniApp, error) {
	act, err := w.actor(by)
	if err != nil {
		return domain.CommuneMiniApp{}, err
	}
	s := w.db.For(ctx)
	var out domain.CommuneMiniApp
	err = s.Tx(ctx, func(tx *corestore.ScopedTx) error {
		c, err := lockCommune(ctx, tx)
		if err != nil {
			return err
		}
		if !c.active {
			return ErrCommuneInactive
		}
		old, err := lockOwnMiniApp(ctx, tx, oldAppID)
		if err != nil {
			return err
		}
		if old.deleted {
			// Removed earlier: the same answer as any App ID this commune does not hold now.
			return ErrMiniAppNotInCommune
		}
		if !old.active {
			return ErrMiniAppInactive
		}
		if err := appIDFree(ctx, tx.Underlying(), newAppID); err != nil {
			return err
		}
		// The reason is the note of the new row too: it is why this App ID was bound.
		out, err = insertOwnMiniApp(ctx, tx, newAppID, reason, by.Code)
		if err != nil {
			return err
		}
		if err := softDeleteOwnMiniApp(ctx, tx, oldAppID, reason, by.Code); err != nil {
			return err
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor: act, Action: ActionAttachMiniApp, Subject: "MiniApp " + newAppID,
			Delta: delta(map[string]any{"app_id": newAppID, "che_do": string(domain.CheDoRieng), "xa": c.name,
				"thay_cho": oldAppID, "ly_do": reason}),
		}); err != nil {
			return err
		}
		d := removalDelta(oldAppID, true, reason)
		d["thay_bang"] = newAppID
		return audit.Write(ctx, tx, audit.Entry{
			Actor: act, Action: ActionDeactivateMiniApp, Subject: "MiniApp " + oldAppID, Delta: delta(d),
		})
	})
	if err != nil {
		return domain.CommuneMiniApp{}, err
	}
	return out, nil
}

// RemoveMiniApp removes one of the commune's dedicated apps ("Gỡ khỏi xã", ADR 0070 #2 as amended
// 05/10/2026): SOFT-DELETES its row and writes `tat_mini_app` in the same transaction, with the
// operator's mandatory reason as delete_reason. There is NO way back: reactivation was withdrawn
// (§Sửa đổi 05/10/2026 #2), and the key on app_id keeps the App ID from ever being bound again.
//
// changed=false when the row is ALREADY soft-deleted in THIS commune — no write, no entry. That
// repeat is how the console retries a secret retirement that failed after the first removal.
//
// Refused, writing nothing: commune unknown or inactive (rule 7 invariant 6 keeps a merged commune's
// registry unchanged); App ID not a `rieng` row of THIS commune.
//
// A row that is merely switched off (written before 05/10/2026, when "gỡ" only switched off) is
// soft-deleted like a running one; its trail entry says it was already off.
//
// Takes effect on the next citizen sign-in: the resolver (Directory.MiniApp) has no cache. Sessions
// already open live to their expiry (ADR 0070 §Hệ quả).
func (w *RegistryWriter) RemoveMiniApp(ctx context.Context, appID, reason string, by domain.OperatorActor) (changed bool, err error) {
	act, err := w.actor(by)
	if err != nil {
		return false, err
	}
	s := w.db.For(ctx)
	err = s.Tx(ctx, func(tx *corestore.ScopedTx) error {
		c, err := lockCommune(ctx, tx)
		if err != nil {
			return err
		}
		if !c.active {
			return ErrCommuneInactive
		}
		row, err := lockOwnMiniApp(ctx, tx, appID)
		if err != nil {
			return err
		}
		if row.deleted {
			return nil
		}
		if err := softDeleteOwnMiniApp(ctx, tx, appID, reason, by.Code); err != nil {
			return err
		}
		changed = true
		d := removalDelta(appID, row.active, reason)
		d["xa"] = c.name
		return audit.Write(ctx, tx, audit.Entry{
			Actor: act, Action: ActionDeactivateMiniApp, Subject: "MiniApp " + appID, Delta: delta(d),
		})
	})
	return changed, err
}

// RecordMiniAppSecretForward writes the commune's audit_log row for a Mini App secret act identity
// has ALREADY ACCEPTED (ADR 0073 §Hệ quả): `dat_khoa_mini_app` / `thu_hoi_khoa_mini_app`, actor the
// operator's VH- code, delta {app_id, phien_ban, ly_do[, tu_dong]}. NEVER the secret — this method is
// never handed it (domain.SecretForward has no field for it).
//
// WHY NOT ATOMIC WITH THE ACT, stated rather than hidden: the act is a write in identity's database,
// and two services cannot share a transaction (rule 2; core/audit's package comment). identity writes
// its own trail entry in ITS transaction with the secret, so the act is never untrailed. This row is
// the second, display copy, written only after identity answered ACCEPTED: a row before the answer
// could record an act that never happened, which is worse than a missing display copy. If THIS write
// fails, the act stands, identity's entry stands, and the caller reports it (logged) — it does not
// undo a secret already sealed elsewhere.
func (w *RegistryWriter) RecordMiniAppSecretForward(ctx context.Context, f domain.SecretForward, by domain.OperatorActor) error {
	act, err := w.actor(by)
	if err != nil {
		return err
	}
	action := ActionSetMiniAppSecret
	d := map[string]any{"app_id": f.AppID, "phien_ban": f.Version, "ly_do": f.Reason}
	if f.Retired {
		action = ActionRetireMiniAppSecret
		d["tu_dong"] = f.Automatic
	}
	return w.db.For(ctx).Tx(ctx, func(tx *corestore.ScopedTx) error {
		return audit.Write(ctx, tx, audit.Entry{
			Actor: act, Action: action, Subject: "MiniApp " + f.AppID, Delta: delta(d),
		})
	})
}
