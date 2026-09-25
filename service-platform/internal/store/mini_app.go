package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/service-platform/internal/domain"
)

// ErrKhongCoMiniApp — the App ID names no ACTIVE, not-deleted registered app.
//
// Unknown, switched off and soft-deleted are deliberately one answer: each means "this app grants
// nothing", and telling them apart to a caller would only describe the registry's history.
var ErrKhongCoMiniApp = errors.New("directory: không có mini app đang hoạt động ứng với app_id này")

// truyVanMiniApp reads one app and, for a dedicated app, the commune it is bound to.
//
// LEFT JOIN because a main app has no commune. The commune is returned WITH its state and is not
// filtered on `dang_hoat_dong`: an inactive bound commune is an ordinary answer the caller refuses,
// not "no such app" — collapsing the two would send an operator looking for a missing row when
// the real cause is a merger (ADR 0045 §Chế độ).
//
// No join to tenant_succession, on purpose: a dedicated app never follows a successor on its own.
const truyVanMiniApp = `
	SELECT m.app_id, m.che_do,
	       COALESCE(t.id, ''), COALESCE(t.ten, ''), COALESCE(t.tinh_thanh, ''),
	       COALESCE(t.dang_hoat_dong, false)
	FROM mini_app m
	LEFT JOIN tenant t ON t.id = m.tenant_id
	WHERE m.app_id = $1 AND m.dang_hoat_dong AND m.deleted_at IS NULL`

// MiniApp resolves one App ID. It belongs on Directory — the sanctioned unscoped reader — for the
// same reason ByHostErr does: this lookup is what ESTABLISHES the commune of a dedicated app's
// session, so it cannot be scoped by one. It returns one app or none, never a list.
func (d *Directory) MiniApp(ctx context.Context, appID string) (domain.MiniApp, error) {
	if err := domain.KiemAppID(appID); err != nil {
		return domain.MiniApp{}, fmt.Errorf("directory: %w", err)
	}

	var (
		out   domain.MiniApp
		cheDo string
		xa    domain.Tenant
	)
	// @cross-tenant: registry lookup that answers "which commune" for one App ID — it runs before
	// any commune is known and reads only mini_app + tenant metadata (ADR 0003, ADR 0045).
	err := d.db.QueryRowContext(ctx, truyVanMiniApp, appID).
		Scan(&out.AppID, &cheDo, &xa.ID, &xa.Ten, &xa.TinhThanh, &xa.DangHoatDong)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return domain.MiniApp{}, ErrKhongCoMiniApp
	case err != nil:
		// The App ID is not personal data and not a secret (platform.proto), so it may name the
		// failing lookup.
		return domain.MiniApp{}, fmt.Errorf("directory: truy vấn mini app %q: %w", appID, err)
	}

	out.CheDo = domain.CheDoMiniApp(cheDo)
	switch out.CheDo {
	case domain.CheDoChinh:
		if xa.ID != "" {
			return domain.MiniApp{}, fmt.Errorf("directory: mini app %q chế độ chính lại gắn xã", appID)
		}
	case domain.CheDoRieng:
		if xa.ID == "" {
			return domain.MiniApp{}, fmt.Errorf("directory: mini app %q chế độ riêng không gắn xã", appID)
		}
		out.Xa = &xa
	default:
		// The CHECK constraint makes this unreachable. If it is reached, the schema and this code
		// disagree, and guessing a mode here would guess a commune.
		return domain.MiniApp{}, fmt.Errorf("directory: mini app %q có chế độ lạ %q", appID, cheDo)
	}
	return out, nil
}
