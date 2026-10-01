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
// `van-hanh:<user>`. Refused when the commune is inactive, and when the App ID has ANY row,
// soft-deleted included: an App ID once bound is never silently re-bound (rule 7 invariant 3).
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
		var one int
		// @cross-tenant: mini_app.app_id is globally unique by design (migration 0006) and keeps
		// soft-deleted rows — "has this App ID EVER been registered" spans every commune.
		err = tx.Underlying().QueryRowContext(ctx, `SELECT 1 FROM mini_app WHERE app_id = $1`, appID).Scan(&one)
		switch {
		case err == nil:
			return ErrMiniAppTaken
		case !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("operator registry: mini app lookup: %w", err)
		}
		if err := tx.Underlying().QueryRowContext(ctx,
			`INSERT INTO mini_app (app_id, che_do, tenant_id, ghi_chu, tao_boi, cap_nhat_boi)
			 VALUES ($1, 'rieng', $2, $3, $4, $4)
			 RETURNING app_id, che_do, dang_hoat_dong, tao_luc, tao_boi`,
			appID, tx.TenantID().String(), note, by.Code).
			Scan(&out.AppID, &out.Mode, &out.Active, &out.CreatedAt, &out.CreatedBy); err != nil {
			if isUniqueViolation(err) {
				return ErrMiniAppTaken
			}
			return fmt.Errorf("operator registry: insert mini app: %w", err)
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
