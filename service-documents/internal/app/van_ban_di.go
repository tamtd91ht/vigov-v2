package app

// The use cases behind the WRITE surface of SỔ VĂN BẢN ĐI.
//
// ⚠ NO SPECIFICATION EXISTS FOR THIS REGISTER — see migration 0004 and domain.VanBanDi. What is
// built here is the smallest register that is actually usable: issue a number, record what went out,
// correct a mistake, remove an entry with a reason. Nothing more was invented: there is no
// lifecycle, no approval chain and no deadline, because no source in this repository names one.
//
// THE STRUCTURE IS van_ban_den.go's, AND THE SHARED HELPERS ARE ITS: `coNguoiThucHienVanBan`,
// `bocVanBan`, `chonChuoi`, `chonNgay`. One copy, because the two registers must answer the same
// question the same way — a second copy of "who is acting" or "what may leave in an error" is a
// second answer, and the drifted one is the one that leaks a document summary into a log line.
//
// WHAT IS DIFFERENT, AND IT IS ONE THING: THE DEADLINE. An incoming document carries a commitment
// the commune must meet, computed from its SLA. An outgoing document does not — it is the commune's
// own act, finished at the moment it is issued. So there is no identity call on this path, and
// `van_ban_di` has no deadline column. IF THE CUSTOMER LATER WANTS "văn bản đi phải phát hành trong
// N giờ sau khi ký", that is a new SLA work kind and a new column, not a number added here.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// KhoVanBanDi is the outgoing register's write half, declared at the point of use. Every method
// takes the transaction, for the reason stated on KhoVanBanDen.
type KhoVanBanDi interface {
	TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.VanBanDi, error)
	LoaiVanBanConDung(ctx context.Context, tx *store.ScopedTx, ma string) error
	Chen(ctx context.Context, tx *store.ScopedTx, v domain.VanBanDi) error
	CapNhat(ctx context.Context, tx *store.ScopedTx, v domain.VanBanDi) error
	XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error
}

// The business verbs written into the trail. `cap_so_van_ban_di` rather than `them_van_ban_di`, and
// the difference is what an inspection searches for: the event worth naming is that THIS COMMUNE
// ISSUED A NUMBER, which is the act that cannot be undone.
const (
	HanhViCapSoVanBanDi = "cap_so_van_ban_di"
	HanhViSuaVanBanDi   = "sua_van_ban_di"
	HanhViGoVanBanDi    = "go_van_ban_di"
)

// YeuCauCapSoVanBanDi is one outgoing document as it arrives from the handler.
//
// THERE IS NO `SoDi` FIELD AND THERE MUST NEVER BE ONE, and on this register that sentence is the
// heaviest one in the file: the number goes on paper, under a seal, out of the building. A client
// that could name it could put a number that is already on a signed document onto a second one.
//
// THERE IS NO `Nam` FIELD: the year is the year of the act — see CapSo.
type YeuCauCapSoVanBanDi struct {
	NgayVanBan time.Time
	LoaiVanBan string
	TrichYeu   string
	NoiNhan    string
	NguoiKy    string
}

// YeuCauSuaVanBanDi is a PARTIAL edit: a nil pointer means "leave this alone". `NguoiKy` is optional
// and its empty value is meaningful — "this entry does not record a signer" is a statement — so a
// dialog editing only the recipient must not wipe it.
type YeuCauSuaVanBanDi struct {
	NgayVanBan *time.Time
	LoaiVanBan *string
	TrichYeu   *string
	NoiNhan    *string
	NguoiKy    *string
}

// VanBanDi owns issuing, correcting and removing one commune's outgoing documents.
type VanBanDi struct {
	db    *store.DB
	kho   KhoVanBanDi
	daySo KhoDaySo

	sinhID func() (string, error)
	nay    func() time.Time
}

func NewVanBanDi(db *store.DB, kho KhoVanBanDi, daySo KhoDaySo) *VanBanDi {
	return &VanBanDi{db: db, kho: kho, daySo: daySo, sinhID: ulid.Moi}
}

func (uc *VanBanDi) nayHoac() time.Time {
	if uc.nay == nil {
		return time.Now().UTC()
	}
	return uc.nay().UTC()
}

// CapSo issues one outgoing document number and records what it was issued for.
//
// THE SHAPE IS VALIDATED BEFORE THE TRANSACTION OPENS, so a malformed request never holds the
// counter's row lock — that lock serialises every issue in the commune for that year.
//
// `nam` IS THE YEAR OF THE ACT. A number is given when the document is issued; a clerk who types
// last year's date onto today's document does not thereby insert a number into last year's closed
// series. ⚠ Administrative practice, not a line of any specification — reported as such.
func (uc *VanBanDi) CapSo(ctx context.Context, yc YeuCauCapSoVanBanDi,
	nguoi audit.Actor) (domain.VanBanDi, error) {

	bayGio := uc.nayHoac()

	moi, err := chuanHoaCapSo(yc, bayGio)
	if err != nil {
		return domain.VanBanDi{}, err
	}
	if err := coNguoiThucHienVanBan(nguoi); err != nil {
		return domain.VanBanDi{}, err
	}

	id, err := uc.sinhID()
	if err != nil {
		return domain.VanBanDi{}, fmt.Errorf("van_ban_di: sinh mã: %w", err)
	}
	moi.ID = id
	moi.Nam = bayGio.Year()
	moi.NguoiTaoMa = nguoi.ID

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// THE TYPE IS CHECKED BEFORE THE NUMBER IS TAKEN, for the reason stated on VanBanDen.Them: a
		// request that is going to be refused must not consume a number, because the counter only
		// ever moves forward and the hole would be permanent.
		if err := uc.kho.LoaiVanBanConDung(ctx, tx, moi.LoaiVanBan); err != nil {
			return err
		}
		so, err := uc.daySo.CapSo(ctx, tx, docstore.SoSachDi, moi.Nam)
		if err != nil {
			return err
		}
		moi.SoDi = so

		if err := uc.kho.Chen(ctx, tx, moi); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{"sau": tomTatVanBanDi(moi)})
		if err != nil {
			return fmt.Errorf("van_ban_di: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViCapSoVanBanDi,
			Subject: domain.MaVanBanDi(moi.Nam, moi.SoDi),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.VanBanDi{}, bocVanBan(ctx, "cấp số văn bản đi", err)
	}
	return moi, nil
}

// Sua corrects one issued entry. The number and the year are not reachable from here.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING — same reasoning as the incoming register, and it is
// what makes this route's `idem.KhongCan` declaration true rather than hopeful.
func (uc *VanBanDi) Sua(ctx context.Context, id string, yc YeuCauSuaVanBanDi,
	nguoi audit.Actor) (domain.VanBanDi, error) {

	if id == "" {
		return domain.VanBanDi{}, docstore.ErrKhongThayVanBanDi
	}
	if err := coNguoiThucHienVanBan(nguoi); err != nil {
		return domain.VanBanDi{}, err
	}
	bayGio := uc.nayHoac()

	var sau domain.VanBanDi
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}

		sau = truoc
		if err := apSuaVanBanDi(&sau, yc, bayGio); err != nil {
			return err
		}
		if khongDoiVanBanDi(truoc, sau) {
			return nil
		}
		if sau.LoaiVanBan != truoc.LoaiVanBan {
			if err := uc.kho.LoaiVanBanConDung(ctx, tx, sau.LoaiVanBan); err != nil {
				return err
			}
		}
		if err := uc.kho.CapNhat(ctx, tx, sau); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{
			"van_ban_id": sau.ID,
			"truoc":      tomTatDoiVanBanDi(truoc, sau, true),
			"sau":        tomTatDoiVanBanDi(truoc, sau, false),
		})
		if err != nil {
			return fmt.Errorf("van_ban_di: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViSuaVanBanDi,
			Subject: domain.MaVanBanDi(truoc.Nam, truoc.SoDi),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.VanBanDi{}, bocVanBan(ctx, "sửa văn bản đi", err)
	}
	return sau, nil
}

// Go soft deletes one entry, with a mandatory reason.
//
// THE NUMBER DOES NOT COME BACK, and here that is not a technical statement: the document was issued
// under that number and has left the commune. Removing the row takes it off the screen; it cannot
// take it off the paper, and a register that reissued the number would give two documents in the
// outside world one identity.
func (uc *VanBanDi) Go(ctx context.Context, id, lyDoTho string, nguoi audit.Actor) error {
	if id == "" {
		return docstore.ErrKhongThayVanBanDi
	}
	lyDo, err := domain.ChuanHoaChuoi(lyDoTho, domain.TranLyDoGo, domain.ErrThieuLyDoGo)
	if err != nil {
		return err
	}
	if err := coNguoiThucHienVanBan(nguoi); err != nil {
		return err
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := uc.kho.XoaMem(ctx, tx, truoc.ID, nguoi.ID, lyDo); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{
			"van_ban_id": truoc.ID,
			"truoc":      tomTatVanBanDi(truoc),
			"ly_do":      lyDo,
			"xoa_mem":    true,
		})
		if err != nil {
			return fmt.Errorf("van_ban_di: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViGoVanBanDi,
			Subject: domain.MaVanBanDi(truoc.Nam, truoc.SoDi),
			Delta:   delta,
		})
	})
	if err != nil {
		return bocVanBan(ctx, "gỡ văn bản đi", err)
	}
	return nil
}

// --- shape ---------------------------------------------------------------------------------------

func chuanHoaCapSo(yc YeuCauCapSoVanBanDi, bayGio time.Time) (domain.VanBanDi, error) {
	var v domain.VanBanDi
	var err error

	if err = domain.KiemNgayVanBanDi(yc.NgayVanBan, bayGio); err != nil {
		return domain.VanBanDi{}, err
	}
	if v.LoaiVanBan, err = domain.ChuanHoaChuoi(yc.LoaiVanBan, domain.TranLoaiVanBan,
		domain.ErrThieuLoaiVanBan); err != nil {
		return domain.VanBanDi{}, err
	}
	if v.TrichYeu, err = domain.ChuanHoaChuoi(yc.TrichYeu, domain.TranTrichYeu,
		domain.ErrThieuTrichYeu); err != nil {
		return domain.VanBanDi{}, err
	}
	if v.NoiNhan, err = domain.ChuanHoaChuoi(yc.NoiNhan, domain.TranNoiNhan,
		domain.ErrThieuNoiNhan); err != nil {
		return domain.VanBanDi{}, err
	}
	if v.NguoiKy, err = domain.ChuanHoaTuyChon(yc.NguoiKy, domain.TranNguoiKy); err != nil {
		return domain.VanBanDi{}, err
	}
	v.NgayVanBan = yc.NgayVanBan
	return v, nil
}

func apSuaVanBanDi(v *domain.VanBanDi, yc YeuCauSuaVanBanDi, bayGio time.Time) error {
	if yc.NgayVanBan != nil {
		if err := domain.KiemNgayVanBanDi(*yc.NgayVanBan, bayGio); err != nil {
			return err
		}
		v.NgayVanBan = *yc.NgayVanBan
	}
	if yc.LoaiVanBan != nil {
		s, err := domain.ChuanHoaChuoi(*yc.LoaiVanBan, domain.TranLoaiVanBan, domain.ErrThieuLoaiVanBan)
		if err != nil {
			return err
		}
		v.LoaiVanBan = s
	}
	if yc.TrichYeu != nil {
		s, err := domain.ChuanHoaChuoi(*yc.TrichYeu, domain.TranTrichYeu, domain.ErrThieuTrichYeu)
		if err != nil {
			return err
		}
		v.TrichYeu = s
	}
	if yc.NoiNhan != nil {
		s, err := domain.ChuanHoaChuoi(*yc.NoiNhan, domain.TranNoiNhan, domain.ErrThieuNoiNhan)
		if err != nil {
			return err
		}
		v.NoiNhan = s
	}
	if yc.NguoiKy != nil {
		s, err := domain.ChuanHoaTuyChon(*yc.NguoiKy, domain.TranNguoiKy)
		if err != nil {
			return err
		}
		v.NguoiKy = s
	}
	return nil
}

func khongDoiVanBanDi(truoc, sau domain.VanBanDi) bool {
	return truoc.NgayVanBan.Equal(sau.NgayVanBan) &&
		truoc.LoaiVanBan == sau.LoaiVanBan &&
		truoc.TrichYeu == sau.TrichYeu &&
		truoc.NoiNhan == sau.NoiNhan &&
		truoc.NguoiKy == sau.NguoiKy
}

// tomTatVanBanDi is the audit delta's view of one entry.
//
// ⚠ `noi_nhan` MAY NAME A CITIZEN (rule 3) — an outgoing document is often a reply to one person,
// and "Ông Nguyễn Văn A, thôn Bình Trị" is what a commune writes there. It is recorded for the same
// reason the incoming summary is: an entry that cannot say WHICH document was issued or removed is
// an entry nobody can use. What is guaranteed is narrower and stated rather than implied: nothing
// here logs it, and no error message carries it out to a client.
func tomTatVanBanDi(v domain.VanBanDi) map[string]any {
	return map[string]any{
		"van_ban_id":   v.ID,
		"so_di":        v.SoDi,
		"nam":          v.Nam,
		"ngay_van_ban": v.NgayVanBan.Format("2006-01-02"),
		"loai_van_ban": v.LoaiVanBan,
		"trich_yeu":    v.TrichYeu,
		"noi_nhan":     v.NoiNhan,
		"nguoi_ky":     v.NguoiKy,
	}
}

func tomTatDoiVanBanDi(truoc, sau domain.VanBanDi, ben bool) map[string]any {
	ra := map[string]any{}
	if !truoc.NgayVanBan.Equal(sau.NgayVanBan) {
		ra["ngay_van_ban"] = chonNgay(ben, truoc.NgayVanBan, sau.NgayVanBan)
	}
	if truoc.LoaiVanBan != sau.LoaiVanBan {
		ra["loai_van_ban"] = chonChuoi(ben, truoc.LoaiVanBan, sau.LoaiVanBan)
	}
	if truoc.TrichYeu != sau.TrichYeu {
		ra["trich_yeu"] = chonChuoi(ben, truoc.TrichYeu, sau.TrichYeu)
	}
	if truoc.NoiNhan != sau.NoiNhan {
		ra["noi_nhan"] = chonChuoi(ben, truoc.NoiNhan, sau.NoiNhan)
	}
	if truoc.NguoiKy != sau.NguoiKy {
		ra["nguoi_ky"] = chonChuoi(ben, truoc.NguoiKy, sau.NguoiKy)
	}
	return ra
}
