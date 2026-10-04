package app

// The commune's map frame — ADR 0072 §"Sửa đổi 04/10/2026", H3, and §"Sửa đổi 04/10/2026 (lần 2)"
// K2–K6; migrations/0016_map_frame.sql, 0017.
// Same structure as mail_settings.go: the read is a store call; the save is read-decide-write under a
// row lock with its audit entry in the SAME transaction (rule 6, invariant 3).
//
// THE EFFECTIVE FRAME (K3). The commune's own row WINS when it is enabled; the platform default is then
// only ATTACHED, best-effort (ownView) — a platform outage must never take the map away from a commune
// that set its own frame. Without one (no row, or "Về mặc định") the platform default IS the frame. No
// frame from either is H3's "Chưa đặt". On THAT path a platform that cannot be asked is an ERROR (503),
// never "Chưa đặt" and never a guessed frame — with one exception, Unimplemented, see defaultFrame.
//
// NO PERSONAL DATA (0016 §PERSONAL DATA): a commune's centre is a public place, not a home coordinate,
// so the audit delta carries the values themselves, before and after.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// MapFrameRepo is the store, declared at the point of use. The writes take the transaction, so there
// is no signature that writes the row outside the one its entry is in.
type MapFrameRepo interface {
	Get(ctx context.Context) (domain.MapFrame, error)
	LockCommuneFrame(ctx context.Context, tx *store.ScopedTx) error
	ForUpdate(ctx context.Context, tx *store.ScopedTx) (domain.MapFrame, bool, error)
	Upsert(ctx context.Context, tx *store.ScopedTx, f domain.MapFrame, by string) (domain.MapFrame, error)
	Disable(ctx context.Context, tx *store.ScopedTx, by string) (bool, error)
}

// MapFrameDefaults is the platform's default frame of the commune in ctx (ADR 0072 K1/K3), declared at
// the point of use. *platformclient.Directory satisfies it.
type MapFrameDefaults interface {
	MapFrameDefault(ctx context.Context) (platformclient.MapFrameDefault, bool, error)
}

// The business verbs written into the trail — VALUES (ADR 0011).
const (
	ActionSaveMapFrame  = "luu_khung_ban_do"
	ActionResetMapFrame = "ve_khung_ban_do_mac_dinh"
)

// ErrMapFrameDefaultUnavailable — the commune has no own frame and the platform could not be asked for
// its default. NOT "Chưa đặt": telling staff the commune has no frame while the truth is unknown would
// send them to set one for nothing, or hide a default that exists.
var ErrMapFrameDefaultUnavailable = errors.New("map_frame: không đọc được khung mặc định từ nền tảng")

// MapFrameDefaultView is the platform default and its bounds — computed by the SAME function as the
// commune's (domain.MapFrame.Bounds), so the two boxes cannot disagree on the geometry.
type MapFrameDefaultView struct {
	Frame  domain.MapFrame
	Bounds domain.MapFrameBounds
}

// MapFrameView is what the map page and the form read. Configured false is H3's "Chưa đặt" (K3: no own
// frame AND no default): the page draws no map, only the guidance sentence and — for a holder of
// `admin.lookup` — the form.
//
// Default is the platform default whenever it could be read and exists. With an own frame enabled it is
// best-effort: nil there means "none, or could not be read" — the reply is still the commune's frame.
type MapFrameView struct {
	Frame      domain.MapFrame
	Bounds     domain.MapFrameBounds
	Configured bool
	Source     string // domain.MapFrameSourceCommune | domain.MapFrameSourceDefault; "" when not configured
	Default    *MapFrameDefaultView
}

// MapFrameInput is one save as the form sends it, before rounding and validation. NoticeVersion is the
// version of the legal notice the official acknowledged (ADR 0072 K4/K6); "" = not acknowledged.
type MapFrameInput struct {
	CenterLat, CenterLng, RadiusKm float64
	NoticeVersion                  string
}

// MapFrames owns reading and saving one commune's map frame.
type MapFrames struct {
	db       *store.DB
	repo     MapFrameRepo
	defaults MapFrameDefaults
	log      *slog.Logger
}

func NewMapFrames(db *store.DB, repo MapFrameRepo, defaults MapFrameDefaults, log *slog.Logger) *MapFrames {
	if defaults == nil {
		// Refuse at construction: without it every commune lacking an own frame would read "Chưa đặt" —
		// the platform default silently ignored — or panic on the first read.
		panic("comms/app: MapFrames thiếu nguồn khung mặc định của nền tảng")
	}
	if log == nil {
		log = slog.Default()
	}
	return &MapFrames{db: db, repo: repo, defaults: defaults, log: log}
}

func mapFrameView(f domain.MapFrame) MapFrameView {
	return MapFrameView{Frame: f, Bounds: f.Bounds(), Configured: true, Source: domain.MapFrameSourceCommune}
}

// Get reads the EFFECTIVE frame (K3). NOT SET IS NOT AN ERROR: it is a designed state of the page (H3),
// so it is a view with Configured=false — mail_settings' convention for a singleton not yet saved.
func (uc *MapFrames) Get(ctx context.Context) (MapFrameView, error) {
	// Scoped in the store: MapFrameStore.Get reads through db.For(ctx).Query.
	f, err := uc.repo.Get(ctx)
	switch {
	case err == nil && f.Enabled:
		return uc.ownView(ctx, f), nil // the commune's own frame wins; the default is only attached
	case err == nil, errors.Is(err, commsstore.ErrMapFrameNotFound):
		return uc.defaultView(ctx)
	default:
		return MapFrameView{}, fmt.Errorf("map_frame: đọc cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
}

// ownView is the commune's own frame with the platform default ATTACHED, BEST-EFFORT — so web-admin can
// offer "Về mặc định" and show what it returns to (K3/K4). ANY failure to read it (Unimplemented
// included) only omits Default, with one Warn line: the frame in effect is the commune's own and is
// known, so a platform outage must never turn this read into a 503.
func (uc *MapFrames) ownView(ctx context.Context, f domain.MapFrame) MapFrameView {
	v := mapFrameView(f)
	d, ok, err := uc.defaultFrame(ctx)
	if err != nil {
		uc.log.WarnContext(ctx, "khung bản đồ: không đọc được khung mặc định để kèm theo — bỏ trường default",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		return v
	}
	if ok {
		v.Default = &d
	}
	return v
}

// defaultView is the view when the commune applies no own frame: the platform default, or not configured.
func (uc *MapFrames) defaultView(ctx context.Context) (MapFrameView, error) {
	d, ok, err := uc.defaultFrame(ctx)
	if err != nil {
		return MapFrameView{}, err
	}
	if !ok {
		return MapFrameView{}, nil
	}
	return MapFrameView{Frame: d.Frame, Bounds: d.Bounds, Configured: true,
		Source: domain.MapFrameSourceDefault, Default: &d}, nil
}

// defaultFrame asks the platform. ok=false with err=nil: no default.
//
// codes.Unimplemented IS "NO DEFAULT", NOT 503 — ROLLOUT SAFETY. The RPC is new (7abf9b47) and its server
// half lands in service-platform separately; a comms deployed first would otherwise answer 503 to every
// commune without an own frame — the map page broken where it used to say "Chưa đặt". Before the RPC
// existed "no default" was exactly the truth, so this is the old behaviour, not a guess. One Warn line
// per read keeps the window visible.
//
// THE VALUE IS NORMALISED WITH THE COMMUNE'S OWN RULE: platformclient already refuses out-of-contract
// values; rounding to the stored scales here keeps the default's bounds computed exactly as a commune
// row's would be. A value that fails is "no default" — fail closed, never clamped.
func (uc *MapFrames) defaultFrame(ctx context.Context) (MapFrameDefaultView, bool, error) {
	// The commune travels in ctx → x-tenant-id metadata (platformclient; rule 2, invariant 8).
	d, ok, err := uc.defaults.MapFrameDefault(ctx)
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			uc.log.WarnContext(ctx, "khung bản đồ: nền tảng chưa có GetMapFrameDefault — coi như chưa đặt khung mặc định",
				"xa", string(tenant.MustFrom(ctx)))
			return MapFrameDefaultView{}, false, nil
		}
		return MapFrameDefaultView{}, false, fmt.Errorf("%w: %w", ErrMapFrameDefaultUnavailable, err)
	}
	if !ok {
		return MapFrameDefaultView{}, false, nil
	}
	f, err := domain.NormalizeMapFrame(d.CenterLat, d.CenterLng, d.RadiusKm)
	if err != nil {
		uc.log.WarnContext(ctx, "khung bản đồ: khung mặc định của nền tảng không qua kiểm tra — coi như chưa đặt",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		return MapFrameDefaultView{}, false, nil
	}
	f.UpdatedAt = d.UpdatedAt
	return MapFrameDefaultView{Frame: f, Bounds: f.Bounds()}, true, nil
}

// Save rounds, validates, and writes the row and its audit entry in ONE transaction. A save that changes
// nothing writes and audits nothing — which is what makes the PUT idempotent. The same values onto a
// DISABLED row are a change: the save switches the commune's own frame back on.
//
// THE NOTICE IS CHECKED HERE, IN THE USE CASE, BEFORE ANY STATEMENT (K4; rule 5, forbidden #1): the form
// can be modified, so a box ticked only in the browser is no acknowledgement.
func (uc *MapFrames) Save(ctx context.Context, in MapFrameInput, actor audit.Actor) (MapFrameView, error) {
	if actor.ID == "" {
		return MapFrameView{}, ErrMissingActor
	}
	if err := domain.CheckMapFrameNotice(in.NoticeVersion); err != nil {
		return MapFrameView{}, err
	}
	after, err := domain.NormalizeMapFrame(in.CenterLat, in.CenterLng, in.RadiusKm)
	if err != nil {
		return MapFrameView{}, err
	}
	after.Enabled = true

	var saved domain.MapFrame
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of every statement.
		// The commune's lock FIRST: FOR UPDATE cannot lock a row that does not exist yet, so without it
		// two first saves both read "not found" and the second entry loses its `truoc`.
		if err := uc.repo.LockCommuneFrame(ctx, tx); err != nil {
			return err
		}
		before, found, err := uc.repo.ForUpdate(ctx, tx)
		if err != nil {
			return err
		}
		if found && before.Enabled && domain.SameMapFrame(before, after) {
			saved = before // nothing moved: no UPDATE, no entry
			return nil
		}
		if saved, err = uc.repo.Upsert(ctx, tx, after, actor.ID); err != nil {
			return err
		}

		delta := map[string]any{"sau": mapFrameValues(after)}
		if found {
			delta["truoc"] = mapFrameValues(before)
		}
		// SAME TRANSACTION AS THE UPSERT (rule 6, invariant 3).
		return writeMapFrameEntry(ctx, tx, actor, ActionSaveMapFrame, delta, in.NoticeVersion)
	})
	if err != nil {
		return MapFrameView{}, fmt.Errorf("map_frame: lưu cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	// After the commit, outside the transaction: the reply has GET's shape, default attached best-effort.
	return uc.ownView(ctx, saved), nil
}

// Reset is "Về mặc định" (ADR 0072 K4): the commune stops applying its own frame and the platform
// default applies. An UPDATE of is_enabled on the kept row, never a DELETE (rule 7), audited with the
// values it switches off. It needs the notice too: returning to the default moves the centre and radius
// the map shows — K4's "mỗi lần đổi tâm hoặc bán kính".
//
// IDEMPOTENT: no row, or a row already off → no write, no entry. The reply is the EFFECTIVE frame after
// the reset, read exactly as GET reads it — so when the platform cannot be asked this returns
// ErrMapFrameDefaultUnavailable even though the reset itself committed; a retry is then a no-op that
// returns the frame.
func (uc *MapFrames) Reset(ctx context.Context, noticeVersion string, actor audit.Actor) (MapFrameView, error) {
	if actor.ID == "" {
		return MapFrameView{}, ErrMissingActor
	}
	if err := domain.CheckMapFrameNotice(noticeVersion); err != nil {
		return MapFrameView{}, err
	}
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of every statement.
		// Same lock order as Save, so a reset and a save of one commune serialise.
		if err := uc.repo.LockCommuneFrame(ctx, tx); err != nil {
			return err
		}
		before, found, err := uc.repo.ForUpdate(ctx, tx)
		if err != nil {
			return err
		}
		if !found || !before.Enabled {
			return nil // already on the default: nothing to switch off, nothing to audit
		}
		changed, err := uc.repo.Disable(ctx, tx, actor.ID)
		if err != nil {
			return err
		}
		if !changed {
			// The row is locked and was read enabled: an UPDATE touching nothing means the two disagree,
			// and an entry for a change that did not happen would be a false record.
			return errors.New("map_frame: tắt khung riêng không đổi dòng nào dù dòng đang bật")
		}
		after := before
		after.Enabled = false
		delta := map[string]any{"truoc": mapFrameValues(before), "sau": mapFrameValues(after)}
		// SAME TRANSACTION AS THE UPDATE (rule 6, invariant 3).
		return writeMapFrameEntry(ctx, tx, actor, ActionResetMapFrame, delta, noticeVersion)
	})
	if err != nil {
		return MapFrameView{}, fmt.Errorf("map_frame: về mặc định cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	// Scoped in the store: the effective read goes through MapFrameStore.Get → db.For(ctx).Query.
	return uc.Get(ctx)
}

// writeMapFrameEntry writes one change's entry. The acknowledgement goes INTO the entry, beside the
// before/after values (K4 "Vết"; 0017's header: the trail, not a column, is its record).
func writeMapFrameEntry(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, action string,
	delta map[string]any, noticeVersion string) error {

	delta["notice_version"] = noticeVersion
	delta["notice_acknowledged"] = true
	b, err := json.Marshal(delta)
	if err != nil {
		return fmt.Errorf("map_frame: mã hoá delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{
		Actor:   actor,
		Action:  action,
		Subject: domain.MapFrameSubject,
		Delta:   b,
	})
}

// mapFrameValues is the full picture for the trail. Every value, both sides: there are only four, and
// "the radius was 12.0 before" answers an inspection without a second lookup.
func mapFrameValues(f domain.MapFrame) map[string]any {
	return map[string]any{"center_lat": f.CenterLat, "center_lng": f.CenterLng, "radius_km": f.RadiusKm,
		"is_enabled": f.Enabled}
}
