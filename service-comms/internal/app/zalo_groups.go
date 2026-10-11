package app

// THE STAFF SIDE OF ZALO GROUP CHATS (ADR 0092 #1–#3, #8) — behind the commune's admin.lookup routes
// (Cấu hình → Kênh Zalo, ADR 0074 #6; no new `quyen` key):
//
//	IssueGroupPairingCode  a one-time code for ONE scope (an org unit, or the whole commune) and its kinds
//	Groups                 the commune's live groups, with unit and issuer names — never a chat_id
//	EditGroup              scope and/or kinds of a live group, before/after in the entry
//	UnlinkGroup            end a live group (soft, with a reason) — the ONLY way a group is unlinked
//
// THE SCOPE IS CHECKED AGAINST THE COMMUNE TODAY: the unit must be a live unit of THIS commune (identity's
// ResolveLiveOrgUnits — a client-supplied id), and every kind a group kind AND switched on in the commune's
// channel settings (ADR 0092 #2). The sender re-checks the kind at send ('loai-tat').
//
// Every write and its audit entry share ONE transaction (rule 6, invariant 3). No entry, error or log line
// carries a code, a chat_id or a message text.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
)

// OrgUnits is identity's org-unit half. *identityclient.Client satisfies it.
type OrgUnits interface {
	LiveOrgUnits(ctx context.Context, ids []string) (map[string]struct{}, error)
	OrgUnitNames(ctx context.Context, ids []string) (map[string]identityclient.OrgUnitName, error)
}

// ErrOrgUnitsUnavailable — identity could not be asked about org units. Never read as "not live" or as
// "no name" (identityclient.LiveOrgUnits).
var ErrOrgUnitsUnavailable = errors.New("zalo: org units unavailable")

// GroupPairingCodeInput is one POST zalo-group-pairing-codes. OrgUnitID "" = the whole commune.
type GroupPairingCodeInput struct {
	OrgUnitID string
	Kinds     []string
}

// GroupPairingCodeView is the one-time group code — returned ONCE, stored only as a hash.
type GroupPairingCodeView struct {
	Code      string
	ExpiresAt time.Time
}

// ZaloGroupView is one line of GET zalo-groups. No chat_id (ADR 0092 §Hệ quả).
type ZaloGroupView struct {
	ID        string
	OrgUnitID string // "" = the whole commune
	// OrgUnitName "" when identity holds no name for the id; OrgUnitRemoved: the unit left the org chart —
	// the group then receives nothing unit-scoped until its scope is edited.
	OrgUnitName    string
	OrgUnitRemoved bool
	Kinds          []string
	LinkedAt       time.Time
	LinkedBy       string
	LinkedByName   string
}

// ZaloGroupPatch is one PATCH zalo-groups/{id}. OrgUnitSet=false keeps the scope; OrgUnitID "" with
// OrgUnitSet means the whole commune. Kinds nil keeps the kinds.
type ZaloGroupPatch struct {
	OrgUnitSet bool
	OrgUnitID  string
	Kinds      []string
}

// ZaloGroups owns the staff side of group chats.
type ZaloGroups struct {
	db     *store.DB
	groups *docstore.ZaloGroupStore
	links  *docstore.ZaloLinkStore // the commune's channel settings
	bots   ZaloBotSource
	units  OrgUnits
	names  StaffNames
	log    *slog.Logger

	newID    func() (string, error)
	now      func() time.Time
	drawCode func() (string, error)
}

// NewZaloGroups panics on a missing dependency — at construction, not on the first request.
func NewZaloGroups(db *store.DB, groups *docstore.ZaloGroupStore, links *docstore.ZaloLinkStore, bots ZaloBotSource,
	units OrgUnits, names StaffNames, log *slog.Logger) *ZaloGroups {

	if db == nil || groups == nil || links == nil || bots == nil || units == nil || names == nil {
		panic("app: NewZaloGroups thiếu phụ thuộc")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ZaloGroups{db: db, groups: groups, links: links, bots: bots, units: units, names: names, log: log,
		newID: ulid.Moi, now: time.Now, drawCode: drawPairingCode}
}

func (uc *ZaloGroups) clock() time.Time { return uc.now().UTC().Truncate(time.Microsecond) }

// normalizeUnitShape reads a client's org-unit id: "" = the whole commune, else a ULID.
func normalizeUnitShape(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	unit, ok := domain.NormalizeOrgUnitID(raw)
	if !ok {
		return "", domain.ErrZaloGroupOrgUnitShape
	}
	return unit, nil
}

// checkUnitLive asks identity whether unit is a live unit of THIS commune. "" passes (commune-wide).
func (uc *ZaloGroups) checkUnitLive(ctx context.Context, unit string) error {
	if unit == "" {
		return nil
	}
	live, err := uc.units.LiveOrgUnits(ctx, []string{unit})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrOrgUnitsUnavailable, err)
	}
	if _, ok := live[unit]; !ok {
		return domain.ErrZaloGroupOrgUnitNotLive
	}
	return nil
}

// enabledKinds normalises kinds against the commune's settings now. A commune whose channel is off has no
// kind to give a group: ErrZaloChannelOff, the personal test message's refusal.
func (uc *ZaloGroups) enabledKinds(ctx context.Context, kinds []string) ([]string, error) {
	s, err := uc.links.ChannelSetting(ctx)
	if err != nil {
		return nil, fmt.Errorf("zalo: read the settings: %w", err)
	}
	if !s.Saved || !s.IsEnabled {
		return nil, ErrZaloChannelOff
	}
	return domain.NormalizeZaloGroupKinds(kinds, s)
}

// IssueGroupPairingCode issues a new one-time group code for one scope. Refused — before anything is
// written — when the unit is not live here, a kind is not a group kind or not switched on, the channel is
// off, or no bot can serve the commune (a code for a bot nobody can reach is a dead credential).
//
// NO previous code is cancelled: 0029 deliberately allows a member of staff several open codes (two groups
// prepared at once); each still expires in ten minutes and works once.
func (uc *ZaloGroups) IssueGroupPairingCode(ctx context.Context, in GroupPairingCodeInput, actor audit.Actor) (
	GroupPairingCodeView, error) {

	if actor.ID == "" {
		return GroupPairingCodeView{}, ErrNoActor
	}
	unit, err := normalizeUnitShape(in.OrgUnitID)
	if err != nil {
		return GroupPairingCodeView{}, err
	}
	kinds, err := uc.enabledKinds(ctx, in.Kinds)
	if err != nil {
		return GroupPairingCodeView{}, err
	}
	b, err := uc.bots.Active(ctx)
	if err != nil {
		return GroupPairingCodeView{}, err
	}
	if !b.Configured {
		return GroupPairingCodeView{}, ErrZaloBotNotConfigured
	}
	if err := uc.checkUnitLive(ctx, unit); err != nil {
		return GroupPairingCodeView{}, err
	}
	at := uc.clock()
	expires := at.Add(domain.PairingCodeTTL)
	var code string
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		id, err := uc.newID()
		if err != nil {
			return fmt.Errorf("zalo: new id: %w", err)
		}
		for i := 0; ; i++ {
			if code, err = uc.drawCode(); err != nil {
				return err
			}
			err = uc.groups.InsertGroupPairingCode(ctx, tx, id, actor.ID, unit, kinds, domain.PairingCodeHash(code), expires, at)
			if err == nil {
				break
			}
			if !errors.Is(err, docstore.ErrPairingCodeTaken) || i+1 >= maxCodeDraws {
				return err
			}
		}
		// The code is NOT in the entry: the trail is kept for years and the code is a credential.
		delta, err := json.Marshal(map[string]any{"ma_gan_nhom_id": id, "het_han": expires, "bo_phan": unit,
			"loai": kinds})
		if err != nil {
			return fmt.Errorf("zalo: encode delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: domain.ActionIssueZaloGroupPairingCode,
			Subject: "ma-gan-nhom-zalo/" + id, At: at, Delta: delta})
	})
	if err != nil {
		return GroupPairingCodeView{}, fmt.Errorf("zalo: issue a group pairing code: %w", err)
	}
	return GroupPairingCodeView{Code: code, ExpiresAt: expires}, nil
}

// Groups lists the commune's live groups with the unit's and the issuer's names. An identity OUTAGE is an
// error, never a page of blanks; a name identity does not hold is "" (rendered unknown).
func (uc *ZaloGroups) Groups(ctx context.Context) ([]ZaloGroupView, error) {
	links, err := uc.groups.LiveGroupLinks(ctx)
	if err != nil {
		return nil, fmt.Errorf("zalo: list groups: %w", err)
	}
	var unitIDs, codes []string
	for _, l := range links {
		if l.OrgUnitID != "" && !slices.Contains(unitIDs, l.OrgUnitID) {
			unitIDs = append(unitIDs, l.OrgUnitID)
		}
		if !slices.Contains(codes, l.LinkedBy) {
			codes = append(codes, l.LinkedBy)
		}
	}
	unitNames := map[string]identityclient.OrgUnitName{}
	for start := 0; start < len(unitIDs); start += identityclient.MaxLookupKeysPerCall {
		end := min(start+identityclient.MaxLookupKeysPerCall, len(unitIDs))
		got, err := uc.units.OrgUnitNames(ctx, unitIDs[start:end])
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrOrgUnitsUnavailable, err)
		}
		for k, v := range got {
			unitNames[k] = v
		}
	}
	staff := map[string]identityclient.TenCanBo{}
	for start := 0; start < len(codes); start += identityclient.TranMaMotLo {
		end := min(start+identityclient.TranMaMotLo, len(codes))
		got, err := uc.names.TenCanBoTheoMa(ctx, codes[start:end])
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrStaffNamesUnavailable, err)
		}
		for k, v := range got {
			staff[k] = v
		}
	}
	out := make([]ZaloGroupView, 0, len(links))
	for _, l := range links {
		v := ZaloGroupView{ID: l.ID, OrgUnitID: l.OrgUnitID, Kinds: l.Kinds, LinkedAt: l.LinkedAt, LinkedBy: l.LinkedBy,
			LinkedByName: staff[l.LinkedBy].HoTen}
		if n, ok := unitNames[l.OrgUnitID]; ok && l.OrgUnitID != "" {
			v.OrgUnitName = n.Name
			v.OrgUnitRemoved = n.Standing == identityv1.RecordStanding_RECORD_STANDING_REMOVED
		}
		out = append(out, v)
	}
	return out, nil
}

// EditGroup changes a live group's scope and/or kinds. Nothing that differs = nothing written, nothing
// recorded. Kinds are re-checked against the commune's settings only when sent: an edit of the scope alone
// does not fail because the commune switched a kind off since (the sender skips it, 'loai-tat').
func (uc *ZaloGroups) EditGroup(ctx context.Context, id string, p ZaloGroupPatch, actor audit.Actor) error {
	if actor.ID == "" {
		return ErrNoActor
	}
	if !p.OrgUnitSet && p.Kinds == nil {
		return domain.ErrZaloGroupNothingToChange
	}
	var (
		unit  string
		kinds []string
		err   error
	)
	if p.OrgUnitSet {
		if unit, err = normalizeUnitShape(p.OrgUnitID); err != nil {
			return err
		}
	}
	if p.Kinds != nil {
		if kinds, err = uc.enabledKinds(ctx, p.Kinds); err != nil {
			return err
		}
	}
	if p.OrgUnitSet {
		if err := uc.checkUnitLive(ctx, unit); err != nil {
			return err
		}
	}
	at := uc.clock()
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, err := uc.groups.LockLiveGroupLink(ctx, tx, id)
		if err != nil {
			return err
		}
		after := cur
		if p.OrgUnitSet {
			after.OrgUnitID = unit
		}
		if p.Kinds != nil {
			after.Kinds = kinds
		}
		if after.OrgUnitID == cur.OrgUnitID && slices.Equal(after.Kinds, cur.Kinds) {
			return nil
		}
		if err := uc.groups.UpdateGroupLinkScope(ctx, tx, cur.ID, after.OrgUnitID, after.Kinds, at); err != nil {
			return err
		}
		delta, err := json.Marshal(map[string]any{
			"truoc": map[string]any{"bo_phan": cur.OrgUnitID, "loai": cur.Kinds},
			"sau":   map[string]any{"bo_phan": after.OrgUnitID, "loai": after.Kinds},
		})
		if err != nil {
			return fmt.Errorf("zalo: encode delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: domain.ActionEditZaloGroup,
			Subject: domain.ZaloGroupSubject(cur.ID), At: at, Delta: delta})
	})
	if err != nil {
		return fmt.Errorf("zalo: edit a group: %w", err)
	}
	return nil
}

// UnlinkGroup ends a live group with the administrator's reason (ADR 0092 #8 — the only way a group is
// unlinked; /dung in a group does nothing). A group not live here is docstore.ErrZaloGroupNotFound.
func (uc *ZaloGroups) UnlinkGroup(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if actor.ID == "" {
		return ErrNoActor
	}
	reason, err := domain.NormalizeZaloGroupUnlinkReason(rawReason)
	if err != nil {
		return err
	}
	at := uc.clock()
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, err := uc.groups.LockLiveGroupLink(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := uc.groups.EndGroupLink(ctx, tx, cur.ID, actor.ID, reason, at); err != nil {
			return err
		}
		return writeGroupLinkEnded(ctx, tx, actor, cur, reason, "go_tren_vigov", at)
	})
	if err != nil {
		return fmt.Errorf("zalo: unlink a group: %w", err)
	}
	return nil
}

// writeGroupLinkEnded is the ket_thuc_nhom_zalo entry: which group, linked since when, its scope, why, from
// where. Never the chat_id.
func writeGroupLinkEnded(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, g domain.ZaloGroupLink,
	reason, source string, at time.Time) error {

	delta, err := json.Marshal(map[string]any{
		"lien_ket_id": g.ID, "lien_ket_tu": g.LinkedAt, "ly_do": reason, "nguon": source, "bot_ref": g.BotRef,
		"bo_phan": g.OrgUnitID, "loai": g.Kinds,
	})
	if err != nil {
		return fmt.Errorf("zalo: encode delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: domain.ActionEndZaloGroupLink,
		Subject: domain.ZaloGroupSubject(g.ID), At: at, Delta: delta})
}
