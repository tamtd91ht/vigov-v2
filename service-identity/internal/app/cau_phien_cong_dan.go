package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
	"github.com/vihat/vigov/service-identity/internal/store/crosstenant"
)

// CauPhienCongDan is CitizenSessionBridgeService.OpenCitizenSession — ViGov issuing a citizen
// session for the Zalo Mini App, after vihat-miniapp verified the Zalo token (ADR 0045).
//
// # THE ORDER IS THE CONSISTENCY DECISION (strong — transaction-boundaries.json,
// phat_hanh_phien_cong_dan_qua_cau_mini_app)
//
//  0. The request's shape, before any platform call: `tenant_hint` (GoiYXa) is retired and refused
//     when non-empty; `commune_host_hint` (GoiYTenMien) must have domain.HopLeTenMienXa's shape in
//     EVERY mode, and must be present when the citizen confirmed.
//  1. ResolveMiniApp — which app, which mode, which bound commune. Synchronous.
//     1b. Main app with a confirmation only: ResolveHost on the domain (ADR 0047). Synchronous. From
//     here on only the ULID exists — the domain is never stored: not in the session, the remembered
//     commune, nor the audit entry (rule 1, forbidden #5), because a merger re-points a domain to
//     the successor and a stored domain would re-attribute every record written before. A dedicated
//     app never asks: the app IS the commune.
//  2. The account's remembered commune (main app only), read before any transaction: the commune
//     decides WHICH scoped transaction to open, so it cannot be read inside one.
//  3. domain.ChonXa, then GetTenant with that commune IN CONTEXT: still active?
//  4. ONE transaction in identity, scoped to that commune: account (row locked), identity by phone,
//     remembered commune and the revocation of the old sessions, the new session, and every audit
//     entry. Nothing is written to any other service; no event is published.
//
// Platform unreachable at 1, 1b or 3 → ErrCauNenTang (UNAVAILABLE); at 1b in particular an outage is
// NEVER ErrCauXaKhongHoatDong, or a platform incident reads to the caller as "no such commune". NEVER "use the commune it
// remembered last time": serving under a commune the registry could not confirm is exactly the
// failure the strong consistency was chosen to prevent.
//
// # NO COMMUNE MEANS NO SESSION — AND NO WRITE
//
// The rule says "no commune" for a main-app open with nothing confirmed or remembered. The owner's
// decision (2026-09-25): do NOT issue a session with an empty tenant_id — open question #25's table
// for trails that belong to no commune is not built, so such a session could not be audited in its
// own transaction. The answer is then an empty tenant_id and NO token, and NOTHING is written: not
// the account, not the identity. A verified phone arriving on such a call is therefore not stored;
// the Mini App asks for the phone only when the citizen submits something, which needs a commune.
//
// # NOT IDEMPOTENT, BY CONTRACT
//
// Every call that issues writes a new session and its entry. A retry after a timeout may leave one
// extra session whose token nobody holds; it expires by TTL (citizen_session_bridge.proto). No
// idempotency key: the token cannot be guessed, and two entries for two openings are the truth.
//
// # NOTHING PERSONAL IS LOGGED OR AUDITED
//
// Not the phone number, not the Zalo user id (rule 3). The audit "who" is the citizen identity's id,
// or the Zalo account's id while no phone is verified (ADR 0045 §Ghi vết). app_id is not personal
// data and is logged and audited.
type CauPhienCongDan struct {
	db       *store.DB
	nenTang  NenTangCauPhien
	taiKhoan TaiKhoanZaloKho
	dinhDanh DinhDanhKho
	phien    PhienCauKho
	thoiHan  time.Duration
	log      *slog.Logger
}

// NenTangCauPhien is the platform, as the bridge reads it. *platformclient.Directory satisfies it.
type NenTangCauPhien interface {
	MiniApp(ctx context.Context, appID string) (platformclient.MiniApp, bool, error)
	// XaTheoHost is ResolveHost with the outage kept apart from "no commune holds it" (see
	// platformclient.XaTheoHost). Unknown, platform-reserved and malformed are all ok=false.
	XaTheoHost(ctx context.Context, host string) (tenant.Tenant, bool, error)
	XaTrongNguCanh(ctx context.Context) (tenant.Tenant, bool, error)
}

// TaiKhoanZaloKho is *crosstenant.TaiKhoanZaloStore.
type TaiKhoanZaloKho interface {
	Doc(ctx context.Context, appID, maZalo string) (crosstenant.TaiKhoanZalo, bool, error)
	TimHoacTao(ctx context.Context, tx *sql.Tx, appID, maZalo string) (crosstenant.TaiKhoanZalo, bool, error)
	TroToiDinhDanh(ctx context.Context, tx *sql.Tx, taiKhoanID, congDanID string) error
	NhoXaCuaGiaoDich(ctx context.Context, tx *store.ScopedTx, taiKhoanID string) error
}

// DinhDanhKho is *crosstenant.DinhDanhStore.
type DinhDanhKho interface {
	TimHoacTao(ctx context.Context, tx *sql.Tx, soDienThoai string) (crosstenant.DinhDanh, bool, error)
}

// PhienCauKho is the part of *idstore.PhienCongDanStore the bridge writes through.
type PhienCauKho interface {
	TaoQuaCau(ctx context.Context, tx *store.ScopedTx, p idstore.PhienCauMoi) (string, string, time.Time, error)
	ThuHoiCuaTaiKhoanZalo(ctx context.Context, tx *store.ScopedTx, taiKhoanID, lyDo string) (int64, error)
}

func NewCauPhienCongDan(db *store.DB, nenTang NenTangCauPhien, taiKhoan TaiKhoanZaloKho,
	dinhDanh DinhDanhKho, phien PhienCauKho, thoiHan time.Duration, log *slog.Logger) *CauPhienCongDan {
	switch {
	case db == nil, nenTang == nil, taiKhoan == nil, dinhDanh == nil, phien == nil:
		panic("app: NewCauPhienCongDan thiếu phụ thuộc — cầu phiên sẽ panic ở lời mở app đầu tiên")
	case thoiHan <= 0:
		// config.Load already refuses this; checked again because a direct caller never passes there.
		panic("app: NewCauPhienCongDan cần CITIZEN_SESSION_TTL dương")
	}
	if log == nil {
		log = slog.Default()
	}
	return &CauPhienCongDan{db: db, nenTang: nenTang, taiKhoan: taiKhoan, dinhDanh: dinhDanh,
		phien: phien, thoiHan: thoiHan, log: log}
}

// YeuCauMoPhienCau is OpenCitizenSessionRequest, field for field. Everything in it is a CLAIM BY
// THE BRIDGE CALLER (ADR 0045 §Tin cậy). MaZalo and SoDaXacThuc are personal data (rule 3).
type YeuCauMoPhienCau struct {
	AppID  string
	MaZalo string
	// GoiYXa is the RETIRED `tenant_hint` (ADR 0047 decision 4). Carried only so that a non-empty
	// value can be REFUSED: a caller still wired to it would otherwise confirm a hint that is
	// silently dropped. It never selects a commune.
	GoiYXa string
	// GoiYTenMien is `commune_host_hint`: a commune's domain, a lookup key and never a commune
	// reference. Not personal data.
	GoiYTenMien string
	DaXacNhanXa bool
	SoDaXacThuc string
	IP          string
	ThietBi     string

	// RequireOwnAppOf is set ONLY by the in-process own-app sign-in (own_app_sign_in.go, ADR 0066),
	// which already resolved the App ID and checked THAT commune's secret. Mo then refuses
	// (ErrCauAppChuaSanSang) unless its own ResolveMiniApp still answers "own app of this commune":
	// a binding moved in between must not open a session in a commune whose secret nobody checked.
	// Empty on the gRPC bridge — the caller there is vihat-miniapp and the field is not on the wire.
	RequireOwnAppOf tenant.ID
	// AccountOnly is set ONLY by the own-app sign-in when the client sent no phoneToken (ADR 0080
	// decision 6): the session is owned by the Zalo account alone. Mo then issues it with NO citizen
	// identity even when this account was linked to one by an earlier verified phone — that identity
	// was proven by a phone exchange signed with the app's secret, and this open carries no such
	// proof (internal/zalo: AccountID takes no secret). Carrying the old link over would hand
	// "Phản ánh của tôi" to whoever holds an accessToken, which ADR 0080 stop condition #4 forbids.
	// Requires SoDaXacThuc == "". Empty/false on the gRPC bridge — not on the wire.
	AccountOnly bool
	// There is no demo-identity input any more (owner decision 05/10/2026, ADR 0066 §Sửa đổi): every
	// own-app session is opened from a phone verified by Zalo. Sessions opened under the old fixed
	// identity keep their audit entries (`danh_tinh_demo: true`) untouched — history, not input.
}

// KetQuaMoPhienCau is OpenCitizenSessionResponse. Token/Sid/HetHan are empty exactly when Xa is.
type KetQuaMoPhienCau struct {
	Token  string
	Sid    string
	HetHan time.Time
	Xa     tenant.ID
	TenXa  string
	// TenMienChinh is `commune_primary_host`: the commune's PRIMARY domain as GetTenant reports it
	// (tenant_domain.la_chinh; service-platform blanks a reserved platform address). "" when Xa is
	// "", and "" when the commune has no primary domain — never filled from GoiYTenMien, which after
	// a merger may be the absorbed commune's old domain (ADR 0047, decision 4). Response only: it is
	// written to no session row and no audit entry (ADR 0047, stop condition #1).
	TenMienChinh string
	DaCoSo       bool
	CheDo        domain.CheDoApp
}

// The error classes the gRPC handler maps to the contract's status table. Each message is generic:
// nothing the caller sent is echoed back.
var (
	ErrCauYeuCauSai       = errors.New("cầu phiên: yêu cầu không hợp lệ")             // INVALID_ARGUMENT
	ErrCauAppChuaSanSang  = errors.New("cầu phiên: ứng dụng chưa sẵn sàng")           // FAILED_PRECONDITION
	ErrCauXaKhongHoatDong = errors.New("cầu phiên: xã được xác nhận không hoạt động") // FAILED_PRECONDITION
	ErrCauNenTang         = errors.New("cầu phiên: không gọi được dịch vụ nền tảng")  // UNAVAILABLE
	ErrCauXungDot         = errors.New("cầu phiên: xã đã nhớ vừa đổi, gọi lại")       // ABORTED
)

// Actions written into audit_log — business verbs (rule 6), named after what the citizen did.
const (
	HanhDongMoPhienCongDan  = "mo_phien_cong_dan"
	HanhDongDoiXaDaNho      = "doi_xa_da_nho"
	HanhDongLienKetDinhDanh = "lien_ket_dinh_danh_zalo"

	lyDoThuHoiDoiXa = "đổi xã ở app chính (ADR 0045)"
)

// Mo opens a citizen session. See the type for the order and why.
func (uc *CauPhienCongDan) Mo(ctx context.Context, yc YeuCauMoPhienCau) (KetQuaMoPhienCau, error) {
	yc.AppID = strings.TrimSpace(yc.AppID)
	// GoiYXa is NOT trimmed: any non-empty value on the wire is the retired field in use.
	yc.GoiYTenMien = strings.TrimSpace(yc.GoiYTenMien)
	yc.SoDaXacThuc = strings.TrimSpace(yc.SoDaXacThuc)

	// --- INVALID_ARGUMENT: wiring faults in the caller (the contract's list) -----------------
	switch {
	case yc.AppID == "", strings.TrimSpace(yc.MaZalo) == "":
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: thiếu app_id hoặc zalo_user_id", ErrCauYeuCauSai)
	case yc.GoiYXa != "":
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: tenant_hint đã ngừng dùng (ADR 0047)", ErrCauYeuCauSai)
	case yc.GoiYTenMien != "" && !domain.HopLeTenMienXa(yc.GoiYTenMien):
		// Never repaired, and never echoed: the message does not carry the value.
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: commune_host_hint sai hình dạng", ErrCauYeuCauSai)
	case yc.DaXacNhanXa && yc.GoiYTenMien == "":
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: %w", ErrCauYeuCauSai, domain.ErrXacNhanKhongCoGoiY)
	case yc.AccountOnly && (yc.SoDaXacThuc != "" || yc.RequireOwnAppOf == ""):
		// An account-only open that carries a phone contradicts itself, and one that did not come
		// from the own-app sign-in has no business asking for it. Wiring faults, refused loudly.
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: account_only chỉ dành cho đăng nhập app riêng không số", ErrCauYeuCauSai)
	}

	// --- 1. which app ------------------------------------------------------------------------
	app, coApp, err := uc.nenTang.MiniApp(ctx, yc.AppID)
	if err != nil {
		uc.log.WarnContext(ctx, "cầu phiên: không tra được Mini App ở dịch vụ nền tảng — không phát phiên",
			"app_id", yc.AppID, "err", err)
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: %w", ErrCauNenTang, err)
	}
	if !coApp {
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: app_id chưa đăng ký", ErrCauAppChuaSanSang)
	}
	cheDo := sangCheDo(app.CheDo)
	if yc.RequireOwnAppOf != "" && (cheDo != domain.CheDoAppRieng || app.XaRieng != yc.RequireOwnAppOf) {
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: app không còn là app riêng của xã đã kiểm secret", ErrCauAppChuaSanSang)
	}

	// --- 1b. the confirmed domain → its commune (main app only) ------------------------------
	// Resolved ONLY where it counts. A dedicated app, or a main-app open without confirmation,
	// ignores the domain — and "ignored" means not even asked, so the platform is never told which
	// domain an unconfirmed QR carried.
	var goiY string
	if cheDo == domain.CheDoAppChinh && yc.DaXacNhanXa {
		t, co, err := uc.nenTang.XaTheoHost(ctx, yc.GoiYTenMien)
		if err != nil {
			// The domain is not personal data and has passed HopLeTenMienXa: safe to log.
			uc.log.WarnContext(ctx, "cầu phiên: không phân giải được tên miền xã ở dịch vụ nền tảng — không phát phiên",
				"app_id", yc.AppID, "host", yc.GoiYTenMien, "err", err)
			return KetQuaMoPhienCau{}, fmt.Errorf("%w: %w", ErrCauNenTang, err)
		}
		if !co || !t.Active {
			// Unclaimed, platform-reserved and deactivated: ONE answer, byte for byte, so the caller
			// cannot probe which domains exist (citizen_session_bridge.proto status table).
			return KetQuaMoPhienCau{}, ErrCauXaKhongHoatDong
		}
		goiY = string(t.ID)
	}

	// --- 2. the remembered commune (main app, not confirmed) ---------------------------------
	var xaDaNho tenant.ID
	if cheDo == domain.CheDoAppChinh && !yc.DaXacNhanXa {
		tk, co, err := uc.taiKhoan.Doc(ctx, yc.AppID, yc.MaZalo)
		if err != nil {
			return KetQuaMoPhienCau{}, fmt.Errorf("cầu phiên: đọc xã đã nhớ: %w", err)
		}
		if co {
			xaDaNho = tk.XaDaNho
		}
	}

	// --- 3. the commune, then whether it is still active -------------------------------------
	ungVien, err := domain.ChonXa(domain.DauVaoChonXa{
		CheDo:         cheDo,
		XaCuaAppRieng: string(app.XaRieng),
		GoiY:          goiY, // the resolved ULID, never the domain
		DaXacNhan:     yc.DaXacNhanXa,
		XaDaNho:       string(xaDaNho),
	})
	switch {
	case errors.Is(err, domain.ErrXacNhanKhongCoGoiY):
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: %w", ErrCauYeuCauSai, err)
	case err != nil:
		// Unknown mode, or a dedicated app with no active commune: configuration a human fixes.
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: %w", ErrCauAppChuaSanSang, err)
	}

	if ungVien.Xa == "" {
		return uc.khongCoXa(cheDo), nil
	}

	xa := tenant.ID(ungVien.Xa)
	ctxXa := tenant.Into(ctx, xa)
	t, coXa, err := uc.nenTang.XaTrongNguCanh(ctxXa)
	if err != nil {
		uc.log.WarnContext(ctx, "cầu phiên: không đọc được xã ở dịch vụ nền tảng — không phát phiên",
			"app_id", yc.AppID, "xa", string(xa), "err", err)
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: %w", ErrCauNenTang, err)
	}
	if !coXa || !t.Active {
		if ungVien.TuChoiNeuNgung {
			if ungVien.Nguon == domain.NguonXaAppRieng {
				return KetQuaMoPhienCau{}, fmt.Errorf("%w: xã gắn app không hoạt động", ErrCauAppChuaSanSang)
			}
			return KetQuaMoPhienCau{}, ErrCauXaKhongHoatDong
		}
		// The remembered commune was merged or dissolved: no commune, never a successor on our own.
		return uc.khongCoXa(cheDo), nil
	}

	// --- 4. one transaction ------------------------------------------------------------------
	var kq KetQuaMoPhienCau
	err = uc.db.For(ctxXa).Tx(ctxXa, func(tx *store.ScopedTx) error {
		tk, taiKhoanMoi, err := uc.taiKhoan.TimHoacTao(ctxXa, tx.Underlying(), yc.AppID, yc.MaZalo)
		if err != nil {
			return err
		}
		if ungVien.Nguon == domain.NguonXaNhoLai && tk.XaDaNho != xa {
			// A confirmed switch committed between step 2 and this lock. Issuing here would put a
			// session in the commune the citizen just left, after that switch revoked the old ones.
			return ErrCauXungDot
		}

		congDan := tk.CongDanID
		if yc.AccountOnly {
			// See the field: an account-only session never inherits the account's linked identity.
			// The link itself is left as it is — it is not this open's to change.
			congDan = ""
		}
		if yc.SoDaXacThuc != "" {
			dd, dinhDanhMoi, err := uc.dinhDanh.TimHoacTao(ctxXa, tx.Underlying(), yc.SoDaXacThuc)
			if err != nil {
				return err
			}
			if dd.ID != tk.CongDanID {
				if err := uc.taiKhoan.TroToiDinhDanh(ctxXa, tx.Underlying(), tk.ID, dd.ID); err != nil {
					return err
				}
				// Who linked: the citizen whose number was just verified. Before/after are identity
				// ids, never numbers (rule 6, invariant 5 with rule 3).
				if err := ghiVetCau(ctxXa, tx, chuTheVet(dd.ID, tk.ID), yc.IP, HanhDongLienKetDinhDanh, tk.ID, map[string]any{
					"truoc":             tk.CongDanID,
					"sau":               dd.ID,
					"dinh_danh_moi_tao": dinhDanhMoi,
					"app_id":            yc.AppID,
				}); err != nil {
					return err
				}
				congDan = dd.ID
			}
		}

		chuThe := chuTheVet(congDan, tk.ID)
		if ungVien.NhoXa && tk.XaDaNho != xa {
			if err := uc.taiKhoan.NhoXaCuaGiaoDich(ctxXa, tx, tk.ID); err != nil {
				return err
			}
			// "Mỗi lúc một phiên còn sống": the sessions of the commune being left end NOW, in the
			// same transaction as the new one (owner's answer to CÒN MỞ #3).
			daThuHoi, err := uc.phien.ThuHoiCuaTaiKhoanZalo(ctxXa, tx, tk.ID, lyDoThuHoiDoiXa)
			if err != nil {
				return err
			}
			if err := ghiVetCau(ctxXa, tx, chuThe, yc.IP, HanhDongDoiXaDaNho, tk.ID, map[string]any{
				"truoc":            string(tk.XaDaNho),
				"sau":              string(xa),
				"so_phien_thu_hoi": daThuHoi,
				"app_id":           yc.AppID,
			}); err != nil {
				return err
			}
		}

		sid, token, hetHan, err := uc.phien.TaoQuaCau(ctxXa, tx, idstore.PhienCauMoi{
			TaiKhoanZaloID: tk.ID,
			CongDanID:      congDan,
			ThoiHan:        uc.thoiHan,
			IP:             yc.IP,
			ThietBi:        yc.ThietBi,
		})
		if err != nil {
			return err
		}
		sessionDelta := map[string]any{
			"app_id":             yc.AppID,
			"che_do":             tenCheDo(cheDo),
			"nguon_xa":           string(ungVien.Nguon),
			"da_co_so":           congDan != "",
			"tai_khoan_zalo":     tk.ID,
			"tai_khoan_zalo_moi": taiKhoanMoi,
		}
		if yc.AccountOnly {
			// Recorded so a later inspection can tell "phone not shared at this open" (ADR 0080) from
			// "account never verified" — both read da_co_so=false otherwise.
			sessionDelta["chi_tai_khoan_zalo"] = true
		}
		if err := ghiVetCau(ctxXa, tx, chuThe, yc.IP, HanhDongMoPhienCongDan, sid, sessionDelta); err != nil {
			return err
		}

		kq = KetQuaMoPhienCau{
			Token: token, Sid: sid, HetHan: hetHan,
			Xa: xa, TenXa: t.Name, TenMienChinh: t.Host, DaCoSo: congDan != "", CheDo: cheDo,
		}
		return nil
	})
	if err != nil {
		// Nothing committed: no session, no trail, no remembered commune changed. Logged without
		// any personal data — the store errors carry none (rule 3).
		if !errors.Is(err, ErrCauXungDot) {
			uc.log.ErrorContext(ctx, "cầu phiên: giao dịch mở phiên thất bại",
				"app_id", yc.AppID, "xa", string(xa), "err", err)
		}
		return KetQuaMoPhienCau{}, fmt.Errorf("cầu phiên: mở phiên: %w", err)
	}
	return kq, nil
}

// khongCoXa is the ONE "no commune" answer: no token, no session, nothing written.
func (uc *CauPhienCongDan) khongCoXa(cheDo domain.CheDoApp) KetQuaMoPhienCau {
	return KetQuaMoPhienCau{CheDo: cheDo}
}

// chuTheVet is the audit "who" of a bridge session (ADR 0045 §Ghi vết): the citizen identity once a
// phone is verified, the Zalo account's own id before that. Never a phone number, never the Zalo id.
// A citizen has no staff code, so rule 6 invariant 8's `.Ma` does not apply (tools/check_audit_actor.py
// names the citizen case as the one where an id is the right value).
//
// THE KIND TRAVELS WITH THE ID (ADR 0080, core 434e72a3): a tai_khoan_zalo.id is written with
// audit.KindZaloAccount, never "citizen". The two ids are both ULIDs and indistinguishable on sight;
// a trail that labels an account id "citizen" is a trail whose citizen filter returns somebody who
// never verified a phone.
func chuTheVet(congDanID, taiKhoanZaloID string) audit.Actor {
	if congDanID != "" {
		return audit.Actor{ID: congDanID, Kind: authz.KindCitizen}
	}
	return audit.Actor{ID: taiKhoanZaloID, Kind: audit.KindZaloAccount}
}

func ghiVetCau(ctx context.Context, tx *store.ScopedTx, chuThe audit.Actor, ip, hanhDong, doiTuong string, noiDung map[string]any) error {
	delta, err := json.Marshal(noiDung)
	if err != nil {
		return fmt.Errorf("cầu phiên: dựng nội dung vết: %w", err)
	}
	chuThe.IP = ip
	return audit.Write(ctx, tx, audit.Entry{
		Actor:   chuThe,
		Action:  hanhDong,
		Subject: doiTuong,
		Delta:   delta,
	})
}

func sangCheDo(c platformclient.CheDoMiniApp) domain.CheDoApp {
	switch c {
	case platformclient.CheDoAppChinh:
		return domain.CheDoAppChinh
	case platformclient.CheDoAppRieng:
		return domain.CheDoAppRieng
	}
	return domain.CheDoAppKhongRo
}

func tenCheDo(c domain.CheDoApp) string {
	switch c {
	case domain.CheDoAppChinh:
		return "chinh"
	case domain.CheDoAppRieng:
		return "rieng"
	}
	return "khong_ro"
}
