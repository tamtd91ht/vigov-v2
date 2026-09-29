package app

// The use cases behind the WRITE surface of the commune's residential units (thôn / tổ dân phố),
// under `admin.org` — user decision 2026-09-29, ADR 0059 §2:
//
//	Create   add a unit; a blank code is derived from the name
//	Update   rename, reclassify, change head / counts / rank, and TAKE OUT OF USE or back into use
//	         (`active`) — a partial edit
//
// THERE IS NO DELETE, NO MERGE AND NO SPLIT, deliberately. Taking a unit out of use writes
// `dang_dung = false` and nothing else (the user: "hồ sơ giữ nguyên, đơn vị chỉ ẩn khỏi ô chọn"): no
// record pointing at the unit is touched, the unit stays on the list and keeps labelling old records,
// and its code stays issued for good (rule 7, invariant 3). Merging or splitting two units rewrites
// archival records — rule 1 stop condition #3, a question for the user, never a route.
//
// WHY THIS LAYER EXISTS: rule 6, invariant 3 requires the audit entry to share a transaction with the
// business write, and core/audit.Write takes a *store.ScopedTx. Opening it is this layer's job.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// ResidentialUnitRepo is the store, declared at the point of use. EVERY METHOD TAKES THE TRANSACTION.
// *idstore.ThonToDanPhoStore satisfies it.
type ResidentialUnitRepo interface {
	LockRegister(ctx context.Context, tx *store.ScopedTx) error
	UnitSnapshot(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingResidentialUnit, error)
	UsableType(ctx context.Context, tx *store.ScopedTx, code string) (string, error)
	EligibleHeadByCode(ctx context.Context, tx *store.ScopedTx, code string) (domain.HeadStaffChoice, error)
	LockUnit(ctx context.Context, tx *store.ScopedTx, id string) (domain.ThonToDanPho, error)
	InsertUnit(ctx context.Context, tx *store.ScopedTx, u domain.ThonToDanPho) error
	UpdateUnit(ctx context.Context, tx *store.ScopedTx, u domain.ThonToDanPho) error
}

// The business verbs written into the trail. English constants, Vietnamese values — the house enum
// convention (ActionDeleteOrgUnit). TAKING OUT OF USE AND BACK INTO USE ARE THEIR OWN VERBS, so "who
// retired Thôn Bình An, and when" is one query on the trail rather than a scan of every edit's delta.
const (
	ActionCreateResidentialUnit     = "them_thon_to_dan_pho"
	ActionUpdateResidentialUnit     = "sua_thon_to_dan_pho"
	ActionDeactivateResidentialUnit = "ngung_dung_thon_to_dan_pho"
	ActionReactivateResidentialUnit = "dung_lai_thon_to_dan_pho"
)

// maxDerivedCodeTries is how many suffixes a derived code tries (`-2` … `-100`) before answering 409 —
// the org chart's tranHauToMa.
const maxDerivedCodeTries = 100

// ResidentialUnits owns the write surface of one commune's residential units.
type ResidentialUnits struct {
	db   *store.DB
	repo ResidentialUnitRepo

	// Injected so a test can pin it. In production: ulid.Moi.
	newID func() (string, error)
}

func NewResidentialUnits(db *store.DB, repo ResidentialUnitRepo) *ResidentialUnits {
	return &ResidentialUnits{db: db, repo: repo, newID: ulid.Moi}
}

// CreateResidentialUnit is the create request.
//
//	Name        required; trimmed; case kept as typed
//	Code        "" = derived from Name, `-2`, `-3`… when taken. NON-EMPTY = used exactly or 409
//	TypeCode    "" = not classified; else a live, in-use type of this commune, else 400
//	HeadCode    "" = no head; else the STAFF CODE of a live, unlocked member of staff, else 400
//	Households  nil = not entered (never 0)
//	Population  nil = not entered (never 0)
//	Order       0 by default
//
// THERE IS NO `Active`: a unit just created is in use; creating one out of use is POST then PATCH, and
// the second request is the one whose trail says somebody took it out of use.
type CreateResidentialUnit struct {
	Name       string
	Code       string
	TypeCode   string
	HeadCode   string
	Households *int
	Population *int
	Order      int
}

// Create adds one unit.
func (uc *ResidentialUnits) Create(ctx context.Context, req CreateResidentialUnit, actor NguoiThucHien) (domain.ThonToDanPho, error) {
	if err := actor.hopLe(); err != nil {
		return domain.ThonToDanPho{}, err
	}
	// Shape first, outside the transaction: a request that fails its shape holds no lock.
	u, err := shapeNewResidentialUnit(req)
	if err != nil {
		return domain.ThonToDanPho{}, err
	}
	typedCode := req.Code != ""

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.repo.LockRegister(ctx, tx); err != nil {
			return err
		}
		existing, err := uc.repo.UnitSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		live := 0
		taken := make(map[string]bool, len(existing))
		for _, e := range existing {
			taken[e.Code] = true
			if e.Deleted {
				continue
			}
			live++
			if domain.FoldResidentialUnitName(e.Name) == domain.FoldResidentialUnitName(u.Ten) {
				return idstore.ErrResidentialUnitNameTaken
			}
		}
		if live >= idstore.TranDanhSachThonToDanPho {
			// The list route REFUSES a list over its ceiling; one more unit would empty every address
			// picker in the commune.
			return ErrResidentialUnitListFull
		}
		if u.Ma, err = pickResidentialUnitCode(u.Ma, taken, typedCode); err != nil {
			return err
		}

		if u.LoaiMa != "" {
			if u.LoaiNhan, err = uc.repo.UsableType(ctx, tx, u.LoaiMa); err != nil {
				return err
			}
		}
		if req.HeadCode != "" {
			head, err := uc.repo.EligibleHeadByCode(ctx, tx, req.HeadCode)
			if err != nil {
				return err
			}
			u.HeadStaffID, u.HeadStaffCode, u.HeadStaffName = head.ID, head.Code, head.Name
		}

		if u.ID, err = uc.newID(); err != nil {
			return fmt.Errorf("thon_to_dan_pho: sinh id: %w", err)
		}
		if err := uc.repo.InsertUnit(ctx, tx, u); err != nil {
			return err
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). The subject is the unit's CODE; the
		// actor is the STAFF CODE (rule 6, invariant 8); TenantID is filled by audit.Write.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionCreateResidentialUnit,
			Subject: u.Ma,
			Delta:   residentialUnitDelta(map[string]any{"sau": residentialUnitTrail(u)}),
		})
	})
	if err != nil {
		return domain.ThonToDanPho{}, err
	}
	return u, nil
}

// ErrResidentialUnitListFull — the commune already holds TranDanhSachThonToDanPho live units. 409.
var ErrResidentialUnitListFull = errors.New("thon_to_dan_pho: danh sách thôn / tổ dân phố của xã đã tới trần")

func shapeNewResidentialUnit(req CreateResidentialUnit) (domain.ThonToDanPho, error) {
	name, err := domain.NormalizeResidentialUnitName(req.Name)
	if err != nil {
		return domain.ThonToDanPho{}, err
	}
	var code string
	if req.Code != "" {
		code, err = domain.NormalizeResidentialUnitCode(req.Code)
	} else {
		code, err = domain.DeriveResidentialUnitCode(name)
	}
	if err != nil {
		return domain.ThonToDanPho{}, err
	}
	for _, ref := range []string{req.TypeCode, req.HeadCode} {
		if err := domain.CheckResidentialUnitRef(ref); err != nil {
			return domain.ThonToDanPho{}, err
		}
	}
	for _, n := range []*int{req.Households, req.Population} {
		if err := domain.CheckResidentialUnitCount(n); err != nil {
			return domain.ThonToDanPho{}, err
		}
	}
	if err := domain.CheckResidentialUnitOrder(req.Order); err != nil {
		return domain.ThonToDanPho{}, err
	}
	return domain.ThonToDanPho{
		Ten: name, Ma: code, LoaiMa: req.TypeCode, SoHo: cloneCount(req.Households), NhanKhau: cloneCount(req.Population),
		DangDung: true, SortOrder: req.Order,
	}, nil
}

// pickResidentialUnitCode — a typed code is taken exactly or refused; a derived one takes the first
// free candidate of base, base-2, base-3, … — "free" counting soft-deleted rows and units out of use,
// because the unique key counts them (rule 7, invariant 3).
func pickResidentialUnitCode(base string, taken map[string]bool, typed bool) (string, error) {
	if typed {
		if taken[base] {
			return "", idstore.ErrResidentialUnitCodeTaken
		}
		return base, nil
	}
	for n := 1; n <= maxDerivedCodeTries; n++ {
		if c := domain.ResidentialUnitCodeCandidate(base, n); !taken[c] {
			return c, nil
		}
	}
	return "", idstore.ErrResidentialUnitCodeTaken
}

// OptionalCount is a count field of a PARTIAL edit, which has THREE states: absent (leave alone),
// null (clear — "not entered"), a number. A *int cannot hold three.
type OptionalCount struct {
	Set   bool
	Value *int
}

// UpdateResidentialUnit is a PARTIAL edit: nil / unset = leave alone.
//
//	TypeCode  "" = clear the classification
//	HeadCode  "" = clear the head
//	Active    false = TAKE OUT OF USE, true = back into use
//
// THERE IS NO Code. The code is an issued identifier (rule 7, invariant 3); the HTTP layer refuses a
// body that names it rather than ignoring it.
type UpdateResidentialUnit struct {
	Name       *string
	TypeCode   *string
	HeadCode   *string
	Households OptionalCount
	Population OptionalCount
	Order      *int
	Active     *bool
}

// Update applies a partial edit to one unit.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING — what makes the route's idem.KhongCan declaration true.
// A type or a head is validated only when it CHANGES: a unit already filed under a type the commune
// has since retired, or headed by somebody since locked, may still be renamed.
func (uc *ResidentialUnits) Update(ctx context.Context, id string, req UpdateResidentialUnit, actor NguoiThucHien) (domain.ThonToDanPho, error) {
	if err := actor.hopLe(); err != nil {
		return domain.ThonToDanPho{}, err
	}
	if id == "" {
		return domain.ThonToDanPho{}, idstore.ErrResidentialUnitNotFound
	}
	var name string
	if req.Name != nil {
		var err error
		if name, err = domain.NormalizeResidentialUnitName(*req.Name); err != nil {
			return domain.ThonToDanPho{}, err
		}
	}
	for _, ref := range []*string{req.TypeCode, req.HeadCode} {
		if ref != nil {
			if err := domain.CheckResidentialUnitRef(*ref); err != nil {
				return domain.ThonToDanPho{}, err
			}
		}
	}
	for _, c := range []OptionalCount{req.Households, req.Population} {
		if c.Set {
			if err := domain.CheckResidentialUnitCount(c.Value); err != nil {
				return domain.ThonToDanPho{}, err
			}
		}
	}
	if req.Order != nil {
		if err := domain.CheckResidentialUnitOrder(*req.Order); err != nil {
			return domain.ThonToDanPho{}, err
		}
	}

	var after domain.ThonToDanPho
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.repo.LockRegister(ctx, tx); err != nil {
			return err
		}
		before, err := uc.repo.LockUnit(ctx, tx, id)
		if err != nil {
			return err
		}
		after = before
		after.SoHo, after.NhanKhau = cloneCount(before.SoHo), cloneCount(before.NhanKhau)

		if req.Name != nil && name != before.Ten {
			existing, err := uc.repo.UnitSnapshot(ctx, tx)
			if err != nil {
				return err
			}
			for _, e := range existing {
				if !e.Deleted && e.ID != before.ID &&
					domain.FoldResidentialUnitName(e.Name) == domain.FoldResidentialUnitName(name) {
					return idstore.ErrResidentialUnitNameTaken
				}
			}
			after.Ten = name
		}
		if req.TypeCode != nil && *req.TypeCode != before.LoaiMa {
			after.LoaiMa, after.LoaiNhan = *req.TypeCode, ""
			if after.LoaiMa != "" {
				if after.LoaiNhan, err = uc.repo.UsableType(ctx, tx, after.LoaiMa); err != nil {
					return err
				}
			}
		}
		if req.HeadCode != nil && *req.HeadCode != before.HeadStaffCode {
			after.HeadStaffID, after.HeadStaffCode, after.HeadStaffName = "", "", ""
			if *req.HeadCode != "" {
				head, err := uc.repo.EligibleHeadByCode(ctx, tx, *req.HeadCode)
				if err != nil {
					return err
				}
				after.HeadStaffID, after.HeadStaffCode, after.HeadStaffName = head.ID, head.Code, head.Name
			}
		}
		if req.Households.Set {
			after.SoHo = cloneCount(req.Households.Value)
		}
		if req.Population.Set {
			after.NhanKhau = cloneCount(req.Population.Value)
		}
		if req.Order != nil {
			after.SortOrder = *req.Order
		}
		if req.Active != nil {
			after.DangDung = *req.Active
		}

		moved := residentialUnitChanges(before, after)
		if len(moved) == 0 {
			return nil
		}
		if err := uc.repo.UpdateUnit(ctx, tx, after); err != nil {
			return err
		}
		action := ActionUpdateResidentialUnit
		switch {
		case before.DangDung && !after.DangDung:
			action = ActionDeactivateResidentialUnit
		case !before.DangDung && after.DangDung:
			action = ActionReactivateResidentialUnit
		}
		// BEFORE AND AFTER, ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). Same transaction.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  action,
			Subject: before.Ma,
			Delta: residentialUnitDelta(map[string]any{
				"truoc": pickTrailFields(residentialUnitTrail(before), moved),
				"sau":   pickTrailFields(residentialUnitTrail(after), moved),
			}),
		})
	})
	if err != nil {
		return domain.ThonToDanPho{}, err
	}
	return after, nil
}

func cloneCount(n *int) *int {
	if n == nil {
		return nil
	}
	v := *n
	return &v
}

// residentialUnitTrail is the before/after payload.
//
// THE HEAD IS HIS OR HER STAFF CODE, never the name: a name is personal data (rule 6, forbidden #4),
// the code is what the trail quotes for every person (rule 6, invariant 8). The counts are counts of a
// territory and name nobody (rule 3). null stays null — "not entered" is not 0.
func residentialUnitTrail(u domain.ThonToDanPho) map[string]any {
	return map[string]any{
		"ma": u.Ma, "ten": u.Ten, "loai": u.LoaiMa, "truong_thon": u.HeadStaffCode,
		"so_ho": u.SoHo, "nhan_khau": u.NhanKhau, "thu_tu": u.SortOrder, "dang_dung": u.DangDung,
	}
}

// residentialUnitChanges names the trail fields whose value differs between the two.
func residentialUnitChanges(before, after domain.ThonToDanPho) []string {
	var out []string
	eqInt := func(a, b *int) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }
	if before.Ten != after.Ten {
		out = append(out, "ten")
	}
	if before.LoaiMa != after.LoaiMa {
		out = append(out, "loai")
	}
	if before.HeadStaffCode != after.HeadStaffCode {
		out = append(out, "truong_thon")
	}
	if !eqInt(before.SoHo, after.SoHo) {
		out = append(out, "so_ho")
	}
	if !eqInt(before.NhanKhau, after.NhanKhau) {
		out = append(out, "nhan_khau")
	}
	if before.SortOrder != after.SortOrder {
		out = append(out, "thu_tu")
	}
	if before.DangDung != after.DangDung {
		out = append(out, "dang_dung")
	}
	return out
}

func pickTrailFields(m map[string]any, keys []string) map[string]any {
	out := make(map[string]any, len(keys))
	for _, k := range keys {
		out[k] = m[k]
	}
	return out
}

// residentialUnitDelta marshals the payload; a failure yields an explicit marker, never a nil delta.
func residentialUnitDelta(v map[string]any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{"loi":"khong_dung_duoc_delta"}`)
	}
	return b
}

// IsResidentialUnitInputError reports whether this is a refusal of what the client sent (400). Listed
// explicitly rather than defaulting to 400: a default would turn a database outage into a 400.
func IsResidentialUnitInputError(err error) bool {
	for _, e := range []error{
		domain.ErrResidentialUnitNameMissing, domain.ErrResidentialUnitNameTooLong, domain.ErrResidentialUnitNameControl,
		domain.ErrResidentialUnitCodeInvalid, domain.ErrResidentialUnitCodeTooLong, domain.ErrResidentialUnitCodeUnderived,
		domain.ErrResidentialUnitOrderRange, domain.ErrResidentialUnitCountRange, domain.ErrResidentialUnitRefTooLong,
	} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
