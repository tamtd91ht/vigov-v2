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
//  1. ResolveMiniApp — which app, which mode, which bound commune. Synchronous.
//  2. The account's remembered commune (main app only), read before any transaction: the commune
//     decides WHICH scoped transaction to open, so it cannot be read inside one.
//  3. domain.ChonXa, then GetTenant with that commune IN CONTEXT: still active?
//  4. ONE transaction in identity, scoped to that commune: account (row locked), identity by phone,
//     remembered commune and the revocation of the old sessions, the new session, and every audit
//     entry. Nothing is written to any other service; no event is published.
//
// Platform unreachable at 1 or 3 → ErrCauNenTang (UNAVAILABLE). NEVER "use the commune it
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
	AppID       string
	MaZalo      string
	GoiYXa      string
	DaXacNhanXa bool
	SoDaXacThuc string
	IP          string
	ThietBi     string
}

// KetQuaMoPhienCau is OpenCitizenSessionResponse. Token/Sid/HetHan are empty exactly when Xa is.
type KetQuaMoPhienCau struct {
	Token  string
	Sid    string
	HetHan time.Time
	Xa     tenant.ID
	TenXa  string
	DaCoSo bool
	CheDo  domain.CheDoApp
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
	yc.GoiYXa = strings.TrimSpace(yc.GoiYXa)
	yc.SoDaXacThuc = strings.TrimSpace(yc.SoDaXacThuc)

	// --- INVALID_ARGUMENT: wiring faults in the caller (the contract's list) -----------------
	switch {
	case yc.AppID == "", strings.TrimSpace(yc.MaZalo) == "":
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: thiếu app_id hoặc zalo_user_id", ErrCauYeuCauSai)
	case yc.GoiYXa != "" && !tenant.ID(yc.GoiYXa).Valid():
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: tenant_hint không phải ULID", ErrCauYeuCauSai)
	case yc.DaXacNhanXa && yc.GoiYXa == "":
		return KetQuaMoPhienCau{}, fmt.Errorf("%w: %w", ErrCauYeuCauSai, domain.ErrXacNhanKhongCoGoiY)
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
		GoiY:          yc.GoiYXa,
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
				if err := ghiVetCau(ctxXa, tx, dd.ID, yc.IP, HanhDongLienKetDinhDanh, tk.ID, map[string]any{
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
		if err := ghiVetCau(ctxXa, tx, chuThe, yc.IP, HanhDongMoPhienCongDan, sid, map[string]any{
			"app_id":             yc.AppID,
			"che_do":             tenCheDo(cheDo),
			"nguon_xa":           string(ungVien.Nguon),
			"da_co_so":           congDan != "",
			"tai_khoan_zalo":     tk.ID,
			"tai_khoan_zalo_moi": taiKhoanMoi,
		}); err != nil {
			return err
		}

		kq = KetQuaMoPhienCau{
			Token: token, Sid: sid, HetHan: hetHan,
			Xa: xa, TenXa: t.Name, DaCoSo: congDan != "", CheDo: cheDo,
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
func chuTheVet(congDanID, taiKhoanZaloID string) string {
	if congDanID != "" {
		return congDanID
	}
	return taiKhoanZaloID
}

func ghiVetCau(ctx context.Context, tx *store.ScopedTx, chuThe, ip, hanhDong, doiTuong string, noiDung map[string]any) error {
	delta, err := json.Marshal(noiDung)
	if err != nil {
		return fmt.Errorf("cầu phiên: dựng nội dung vết: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{
		Actor:   audit.Actor{ID: chuThe, Kind: "citizen", IP: ip},
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
