package app

// The use cases behind the WRITE surface of the commune's working calendar — the three tables of
// migration 0006, under Cấu hình → Thời hạn xử lý (14-cau-hinh.md §8:318).
//
// WHY THIS LAYER EXISTS: rule 6, invariant 3 requires the audit entry to share a transaction with
// the business write, and core/audit.Write takes a *store.ScopedTx with no overload that writes
// outside one. Opening that transaction is this layer's job. The handler translates HTTP and
// nothing else; the store knows SQL and nothing else.
//
// =================================================================================================
// WHY ONE TYPE FOR THREE TABLES, WHERE THE READ SIDE HAS THREE STORES.
//
// The three tables answer one question together — "was this authority working at this instant" —
// and the write path has to enforce rules that SPAN them, which no per-table type could hold:
//
//	a date in BOTH ngay_nghi_le AND ngay_lam_bu    the commune is recorded as closed and as working
//	                                               on one day. Refused, no winner picked
//	                                               (migration 0006:254)
//	a swap day whose WEEKDAY already has sessions  ADR 0007 decision 9 — REPLACE and ADD are both
//	                                               defensible, they differ by exactly the overlap,
//	                                               and nothing decides between them
//
// Splitting this into three types would mean three copies of the same three store handles, and the
// first turn that added a rule to one copy would leave the other two behind. The NARROWING happens
// where it belongs — at the routes: three separate interfaces in internal/http/routes.go, so the
// holiday handler still cannot reach the weekly calendar's writer.
//
// =================================================================================================
// NOTHING HERE RECOMPUTES A DEADLINE THAT HAS ALREADY BEEN ISSUED, AND THAT IS THE LOAD-BEARING
// PROPERTY OF THE WHOLE FILE.
//
// Rule 10, invariant 2: each deadline is computed ONCE, at the act that fixes it, and stored —
// `han_tiep_nhan` when the row is created, `han_xu_ly_xong` when the field is settled (ADR 0028).
// Those columns live on the petition and on the document, in `service-petitions` and
// `service-documents`, which this service cannot write to at all (rule 2, forbidden #2), and
// 14-cau-hinh.md:289 says the same from the specification's side: *"Thay đổi chỉ áp dụng cho hồ sơ
// tiếp nhận sau thời điểm lưu."*
//
// THAT IS NOT MERELY "WE HAPPEN NOT TO". Changing a commune's office hours and then recomputing
// would silently move a commitment already made to a named citizen who has already been told the
// old one — and would change every on-time/overdue figure of every past reporting period. If a
// later turn finds any path that recomputes a stored deadline from these tables, that is a defect
// to report, not a feature to finish.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The three stores, declared at the point of use.
//
// EVERY METHOD TAKES THE TRANSACTION — the reads as well as the writes. That is not symmetry for
// its own sake: every decision here is read-decide-write (does this overlap, is this date already a
// holiday, does the commune already have this seed row), and a read outside the transaction is a
// check with a gap in the middle through which a concurrent write slips.
type (
	KhoLichLamViecGhi interface {
		DanhSachDeGhi(ctx context.Context, tx *store.ScopedTx) ([]idstore.CaLamViecDeGhi, error)
		TheoIDDeGhi(ctx context.Context, tx *store.ScopedTx, id string) (domain.CaLamViec, error)
		Chen(ctx context.Context, tx *store.ScopedTx, c domain.CaLamViec) error
		CapNhat(ctx context.Context, tx *store.ScopedTx, c domain.CaLamViec) error
		XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error
	}

	KhoNgayNghiLeGhi interface {
		TheoNamDeGhi(ctx context.Context, tx *store.ScopedTx, nam int) ([]idstore.NgayNghiLeDeGhi, error)
		TheoIDDeGhi(ctx context.Context, tx *store.ScopedTx, id string) (domain.NgayNghiLe, error)
		Chen(ctx context.Context, tx *store.ScopedTx, n domain.NgayNghiLe) error
		CapNhat(ctx context.Context, tx *store.ScopedTx, n domain.NgayNghiLe) error
		XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error
	}

	KhoNgayLamBuGhi interface {
		TheoNamDeGhi(ctx context.Context, tx *store.ScopedTx, nam int) ([]idstore.CaLamBuDeGhi, error)
		TheoIDDeGhi(ctx context.Context, tx *store.ScopedTx, id string) (domain.CaLamBu, error)
		Chen(ctx context.Context, tx *store.ScopedTx, c domain.CaLamBu) error
		CapNhat(ctx context.Context, tx *store.ScopedTx, c domain.CaLamBu) error
		XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error
	}
)

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system writes (`dang_nhap`, `them_can_bo`, `sua_thoi_han_xu_ly`): an inspection reads these
// strings, and a function name would tell them nothing.
//
// SEEDING HAS ITS OWN VERB, SEPARATE FROM "them_…". Setting a commune's calendar up and adding one
// session to a live one are two different acts with two different consequences, and in a ledger
// that is never deleted they must not be one string — otherwise telling "the commune was set up"
// from "somebody moved the Friday closing time" requires opening and interpreting every delta.
const (
	HanhViThemCaLamViec = "them_ca_lam_viec"
	HanhViSuaCaLamViec  = "sua_ca_lam_viec"
	HanhViXoaCaLamViec  = "xoa_ca_lam_viec"
	HanhViGieoLichTuan  = "gieo_lich_lam_viec_mac_dinh"

	HanhViThemNgayNghiLe = "them_ngay_nghi_le"
	HanhViSuaNgayNghiLe  = "sua_ngay_nghi_le"
	HanhViXoaNgayNghiLe  = "xoa_ngay_nghi_le"
	HanhViGieoNgayNghiLe = "gieo_ngay_nghi_le_mac_dinh"

	HanhViThemNgayLamBu = "them_ngay_lam_bu"
	HanhViSuaNgayLamBu  = "sua_ngay_lam_bu"
	HanhViXoaNgayLamBu  = "xoa_ngay_lam_bu"
)

// chuDeGieoLichTuan is the audit subject of a weekly seeding run.
//
// audit.Entry REQUIRES A SUBJECT and rule 6, invariant 2 wants the record the act was performed ON.
// A seeding run acts on the commune's WEEK, which has no business code — there is no `ma` column on
// `lich_lam_viec` and inventing one would be a second identifier for rows that already have ULIDs.
// The commune itself is already on every entry (`tenant_id`), so a constant naming WHAT was
// configured is the honest subject. Same decision as chuDeGieoSLA, for the same reason.
const chuDeGieoLichTuan = "cau-hinh-lich-lam-viec"

// Lich owns the write surface of one commune's working calendar.
type Lich struct {
	db   *store.DB
	tuan KhoLichLamViecGhi
	nghi KhoNgayNghiLeGhi
	bu   KhoNgayLamBuGhi

	// Injected so a test can pin it. In production: ulid.Moi.
	sinhID func() (string, error)
}

func NewLich(db *store.DB, tuan KhoLichLamViecGhi, nghi KhoNgayNghiLeGhi, bu KhoNgayLamBuGhi) *Lich {
	return &Lich{db: db, tuan: tuan, nghi: nghi, bu: bu, sinhID: ulid.Moi}
}

// KetQuaGieoLich is what one seeding run did.
//
// THREE COUNTS AND NOT A BOOLEAN, because the three outcomes are things a person needs told apart
// and the third one is the one nobody expects:
//
//	DaGieo  rows this run actually inserted.
//	DaCo    seed rows the commune ALREADY HAS — left exactly as they were, including any hours the
//	        commune edited. Also counts a row the commune SOFT-DELETED: they removed it on purpose,
//	        and putting it back would overwrite a decision (and would hit the unique key, which
//	        counts deleted rows — idstore.ErrCaLamViecTrungGioMo).
//	BoQua   seed rows SKIPPED because writing them would have BROKEN the calendar — a session
//	        overlapping one the commune already has, or a holiday on a date the commune has already
//	        declared a swap working day. See the note on GieoTuanMacDinh: this is the counter that
//	        keeps the seed button from being the thing that makes deadlines uncomputable.
type KetQuaGieoLich struct {
	DaGieo int
	DaCo   int
	BoQua  int
}

// ---------------------------------------------------------------------------------------------
// lich_lam_viec — the ordinary week.
// ---------------------------------------------------------------------------------------------

// YeuCauThemCa is one new weekly session.
//
// THE TIMES ARE ALREADY TYPED. The handler parses "07:30" into seconds since midnight with
// domain.DocGioTrongNgay, so no string reaches this layer and there is no second parser to drift
// from the first.
type YeuCauThemCa struct {
	Thu     int
	BatDau  domain.GioTrongNgay
	KetThuc domain.GioTrongNgay
	GhiChu  string
}

// YeuCauSuaCa is a PARTIAL edit: a nil pointer means "leave this alone".
//
// WHY POINTERS. With plain values this struct could not tell "the client did not mention this" from
// "the client sent zero", and zero is a legal weekday for nobody and a legal time for midnight — so
// a screen editing only the note would move every session to 00:00:00 on an invalid weekday, or
// have the whole edit refused.
type YeuCauSuaCa struct {
	Thu     *int
	BatDau  *domain.GioTrongNgay
	KetThuc *domain.GioTrongNgay
	GhiChu  *string
}

// ThemCa adds one session to the commune's ordinary week.
//
// =================================================================================================
// THE OVERLAP CHECK IS THE REASON THIS METHOD IS NOT THREE LINES. Migration 0006:109 states the
// obligation in writing: the schema's UNIQUE (tenant_id, thu, bat_dau) only stops two sessions
// STARTING at the same minute, and 07:30–11:30 against 09:00–12:00 passes every constraint while
// double-counting two hours. A deadline computed from a double count comes out EARLIER than the
// commune's real hours — the direction that reports an authority late when it was not.
//
// IT RUNS INSIDE THE TRANSACTION, AGAINST LIVE ROWS ONLY. A soft-deleted session is not a session
// (rule 7, invariant 2), and a check outside the transaction is a check with a gap through which a
// concurrent insert puts the overlapping row.
//
// ⚠ STATED GAP — THE OTHER DIRECTION OF ADR 0007 DECISION 9 IS NOT CHECKED HERE. Adding a session
// to a weekday that some FUTURE swap day happens to fall on turns that swap day into a
// configuration error (a swap day on a date whose weekday already has sessions). Checking it would
// mean scanning every year that holds a swap day, and there is no bound on which years those are.
// What holds instead is that nothing computes a wrong answer: domain.caCuaNgay REFUSES that state
// with LoiLamBuTrungNgayDaLam, loudly, naming the date. The commune is told; nothing is guessed.
// =================================================================================================
func (uc *Lich) ThemCa(ctx context.Context, yc YeuCauThemCa, nguoi NguoiThucHien) (domain.CaLamViec, error) {
	if err := nguoi.hopLe(); err != nil {
		return domain.CaLamViec{}, err
	}
	ghiChu, err := domain.ChuanHoaGhiChuCa(yc.GhiChu)
	if err != nil {
		return domain.CaLamViec{}, err
	}
	moi := domain.CaLamViec{Thu: yc.Thu, BatDau: yc.BatDau, KetThuc: yc.KetThuc, GhiChu: ghiChu}
	if err := domain.KiemTraCaLamViec(moi); err != nil {
		return domain.CaLamViec{}, err
	}
	if moi.ID, err = uc.sinhID(); err != nil {
		return domain.CaLamViec{}, fmt.Errorf("lich_lam_viec: sinh id: %w", err)
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.khongChongCaTuan(ctx, tx, moi); err != nil {
			return err
		}
		if err := uc.tuan.Chen(ctx, tx, moi); err != nil {
			return err
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). TenantID is left unset on purpose:
		// audit.Write fills it from the transaction, which took it from the context (rule 1,
		// invariant 4). Passing it here would be a second source for the one fact that decides
		// which commune the entry belongs to.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViThemCaLamViec,
			Subject: chuDeCaLamViec(moi),
			Delta:   deltaLich(map[string]any{"sau": tomTatCaLamViec(moi)}),
		})
	})
	if err != nil {
		return domain.CaLamViec{}, err
	}
	return moi, nil
}

// SuaCa applies a partial edit to one weekly session.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING. Sending the values a row already has is not an event;
// recording it would fill a public authority's ledger with entries saying nothing changed, and
// those are the entries that bury the ones carrying legal weight. Same discipline as SLA.Sua, and
// it is also what makes the route genuinely idempotent.
func (uc *Lich) SuaCa(ctx context.Context, id string, yc YeuCauSuaCa, nguoi NguoiThucHien) (domain.CaLamViec, error) {
	if err := nguoi.hopLe(); err != nil {
		return domain.CaLamViec{}, err
	}
	if id == "" {
		return domain.CaLamViec{}, idstore.ErrCaLamViecKhongTonTai
	}

	var sau domain.CaLamViec
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// FOR UPDATE, inside the transaction. Two administrators editing one session otherwise both
		// read the old hours and the second write silently discards the first — on a table whose
		// values decide commitments to citizens.
		truoc, err := uc.tuan.TheoIDDeGhi(ctx, tx, id)
		if err != nil {
			return err
		}

		sau = truoc
		if yc.Thu != nil {
			sau.Thu = *yc.Thu
		}
		if yc.BatDau != nil {
			sau.BatDau = *yc.BatDau
		}
		if yc.KetThuc != nil {
			sau.KetThuc = *yc.KetThuc
		}
		if yc.GhiChu != nil {
			ghiChu, err := domain.ChuanHoaGhiChuCa(*yc.GhiChu)
			if err != nil {
				return err
			}
			sau.GhiChu = ghiChu
		}

		// THE RESULT IS VALIDATED, NOT THE REQUEST — the same reasoning as SLA.Sua. A row already
		// holding a bad value must not be committed again untouched by a screen that only meant to
		// change the note.
		if err := domain.KiemTraCaLamViec(sau); err != nil {
			return err
		}
		if sau == truoc {
			return nil
		}
		if err := uc.khongChongCaTuan(ctx, tx, sau); err != nil {
			return err
		}
		if err := uc.tuan.CapNhat(ctx, tx, sau); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViSuaCaLamViec,
			Subject: chuDeCaLamViec(truoc),
			Delta: deltaLich(map[string]any{
				"truoc": tomTatCaLamViec(truoc),
				"sau":   tomTatCaLamViec(sau),
			}),
		})
	})
	if err != nil {
		return domain.CaLamViec{}, err
	}
	return sau, nil
}

// XoaCa soft deletes one weekly session.
//
// THIS IS NOT A DELETE AND THE NAME IS THE ONLY PLACE THAT COULD SUGGEST OTHERWISE. The row stays,
// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), because a calendar
// row is the basis of commitments already issued: an inspection asking why a file received on 30/04
// was due on 05/05 reads the calendar as it stood that day (migration 0006:41).
func (uc *Lich) XoaCa(ctx context.Context, id, lyDoTho string, nguoi NguoiThucHien) error {
	return uc.xoaMotDong(ctx, id, lyDoTho, nguoi, HanhViXoaCaLamViec,
		func(tx *store.ScopedTx, lyDo string) (string, any, error) {
			truoc, err := uc.tuan.TheoIDDeGhi(ctx, tx, id)
			if err != nil {
				return "", nil, err
			}
			if err := uc.tuan.XoaMem(ctx, tx, truoc.ID, nguoi.Vet.ID, lyDo); err != nil {
				return "", nil, err
			}
			return chuDeCaLamViec(truoc), tomTatCaLamViec(truoc), nil
		})
}

// GieoTuanMacDinh writes the weekly seed rows this commune does not have yet.
//
// =================================================================================================
// WHAT MAKES IT SAFE TO PRESS TWICE, in order:
//
//  1. The existing rows are read INSIDE the transaction. A read outside it is a check with a gap
//     through which a concurrent run inserts the same row.
//  2. A seed row is matched to an existing row by (thu, bat_dau) — THE UNIQUE KEY'S OWN PAIR, so
//     "already present" here means exactly what the database means by it. Matching on the closing
//     time as well would re-seed every session a commune had shortened, which is the defect this
//     method is shaped to avoid.
//  3. A row that is present is SKIPPED ENTIRELY — not updated, not compared, not counted as a
//     conflict. Whatever the commune put there stays. SOFT-DELETED ROWS COUNT AS PRESENT: the
//     commune removed that session on purpose, and the unique key counts it too.
//  4. A seed row that would OVERLAP a live session is skipped as well, and counted in BoQua. This
//     is the half a "just insert what is missing" implementation gets wrong: a commune that has
//     already entered Monday 08:00–12:00 would otherwise get Monday 07:30–11:30 written beside it,
//     and two overlapping sessions make every deadline uncomputable (LoiCaChongNhau). The seed
//     button must never be the thing that breaks the calendar.
//  5. If two runs still race, `UNIQUE (tenant_id, thu, bat_dau)` refuses the loser and the whole
//     transaction rolls back — no duplicate, and no half-seeded week.
//
// NOTHING IS WRITTEN AND NOTHING IS AUDITED WHEN THERE IS NOTHING TO SEED. Same reasoning as
// SLA.GieoMacDinh: an entry recording that a button did nothing buries the entries that carry legal
// weight.
//
// IT NEVER OVERWRITES AN HOUR THE COMMUNE HAS EDITED. There is no path from this method to
// tuan.CapNhat, and the store offers no UPSERT to reach for.
// =================================================================================================
func (uc *Lich) GieoTuanMacDinh(ctx context.Context, nguoi NguoiThucHien) (KetQuaGieoLich, error) {
	if err := nguoi.hopLe(); err != nil {
		return KetQuaGieoLich{}, err
	}

	var kq KetQuaGieoLich
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// Reset on every attempt: store.Tx does not retry today, but a counter that survived into a
		// second call would report a total rather than this run's work.
		kq = KetQuaGieoLich{}

		dangCo, err := uc.tuan.DanhSachDeGhi(ctx, tx)
		if err != nil {
			return err
		}
		co := make(map[khoaCaTuan]bool, len(dangCo))
		song := make([]domain.Khoang, 0, len(dangCo))
		for _, m := range dangCo {
			// THE KEY SET COUNTS DELETED ROWS; THE OVERLAP SET DOES NOT. See KetQuaGieoLich and
			// idstore.CaLamViecDeGhi for why those two answers differ.
			co[khoaCaTuan{m.Ca.Thu, m.Ca.BatDau}] = true
			if !m.DaXoa {
				song = append(song, domain.KhoangCuaCaLamViec(m.Ca))
			}
		}

		var daGieo []string
		for _, g := range domain.BoGieoCaLamViec() {
			moi := domain.CaLamViec{Thu: g.Thu, BatDau: g.BatDau, KetThuc: g.KetThuc, GhiChu: g.GhiChu}
			if co[khoaCaTuan{moi.Thu, moi.BatDau}] {
				kq.DaCo++
				continue
			}
			// THE SEED SET IS VALIDATED ON THE WAY IN, exactly like a typed figure. It is a fixed
			// list in source, so this can only fire if somebody edits that list badly — which is
			// precisely when it is worth catching, because the alternative is a constraint violation
			// at a commune's first configuration and no sentence saying which row.
			if err := domain.KiemTraCaLamViec(moi); err != nil {
				return fmt.Errorf("lich_lam_viec: bộ gieo có dòng không hợp lệ (%s %s): %w",
					domain.TenThuISO(moi.Thu), moi.BatDau.Chuoi(), err)
			}
			if err := domain.KhongChongCaNao(domain.KhoangCuaCaLamViec(moi), song); err != nil {
				kq.BoQua++
				continue
			}
			if moi.ID, err = uc.sinhID(); err != nil {
				return fmt.Errorf("lich_lam_viec: sinh id dòng gieo: %w", err)
			}
			if err := uc.tuan.Chen(ctx, tx, moi); err != nil {
				return err
			}
			// The row just written joins the overlap set, so two seed rows of one weekday are
			// checked against each other as well as against the commune's own.
			song = append(song, domain.KhoangCuaCaLamViec(moi))
			daGieo = append(daGieo, chuDeCaLamViec(moi))
			kq.DaGieo++
		}

		if kq.DaGieo == 0 {
			return nil
		}
		// ONE ENTRY FOR THE RUN rather than one per row: the act a person performed was "set this
		// commune's week up", and ten entries would describe ten acts that never happened.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViGieoLichTuan,
			Subject: chuDeGieoLichTuan,
			Delta: deltaLich(map[string]any{
				"da_gieo": kq.DaGieo,
				"da_co":   kq.DaCo,
				"bo_qua":  kq.BoQua,
				// The rows written, so an inspection can see WHICH hours this commune started from
				// without needing the source of the release that was deployed that day.
				"ca": daGieo,
			}),
		})
	})
	if err != nil {
		return KetQuaGieoLich{}, err
	}
	return kq, nil
}

// khoaCaTuan is the pair `UNIQUE (tenant_id, thu, bat_dau)` is built on. A struct key and not a
// joined string: a separator inside either half would make two different pairs collide.
type khoaCaTuan struct {
	thu    int
	batDau domain.GioTrongNgay
}

// khongChongCaTuan refuses a session that overlaps a LIVE session of the same weekday.
//
// THE ROW BEING WRITTEN IS EXCLUDED BY ID, which is what makes an edit possible at all: a row
// compared against itself overlaps itself perfectly.
func (uc *Lich) khongChongCaTuan(ctx context.Context, tx *store.ScopedTx, moi domain.CaLamViec) error {
	dangCo, err := uc.tuan.DanhSachDeGhi(ctx, tx)
	if err != nil {
		return err
	}
	song := make([]domain.Khoang, 0, len(dangCo))
	for _, m := range dangCo {
		if m.DaXoa || m.Ca.ID == moi.ID {
			continue
		}
		song = append(song, domain.KhoangCuaCaLamViec(m.Ca))
	}
	return domain.KhongChongCaNao(domain.KhoangCuaCaLamViec(moi), song)
}

// ---------------------------------------------------------------------------------------------
// ngay_nghi_le — the closure dates.
// ---------------------------------------------------------------------------------------------

// YeuCauThemNgayNghi is one new closure date.
type YeuCauThemNgayNghi struct {
	Ngay string // YYYY-MM-DD
	Ten  string
}

// YeuCauSuaNgayNghi is a PARTIAL edit — see YeuCauSuaCa for why the fields are pointers.
type YeuCauSuaNgayNghi struct {
	Ngay *string
	Ten  *string
}

// ThemNgayNghi adds one date the commune does not work.
//
// IT REFUSES A DATE THE COMMUNE HAS ALREADY DECLARED A SWAP WORKING DAY. That state — closed and
// working on one day — is a configuration error with no defensible winner (migration 0006:254): a
// silent precedence rule would make one of two rows a person can SEE on the configuration screen do
// nothing, and nobody would ever find out which. Both read routes already refuse a year containing
// it; refusing at the write is what keeps a commune from creating it by accident in the first place.
func (uc *Lich) ThemNgayNghi(ctx context.Context, yc YeuCauThemNgayNghi, nguoi NguoiThucHien) (domain.NgayNghiLe, error) {
	if err := nguoi.hopLe(); err != nil {
		return domain.NgayNghiLe{}, err
	}
	moi, err := uc.dungNgayNghi(domain.NgayNghiLe{Ngay: yc.Ngay, Ten: yc.Ten})
	if err != nil {
		return domain.NgayNghiLe{}, err
	}
	if moi.ID, err = uc.sinhID(); err != nil {
		return domain.NgayNghiLe{}, fmt.Errorf("ngay_nghi_le: sinh id: %w", err)
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.khongTrungLamBu(ctx, tx, moi.Ngay); err != nil {
			return err
		}
		if err := uc.nghi.Chen(ctx, tx, moi); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViThemNgayNghiLe,
			Subject: chuDeNgayNghiLe(moi),
			Delta:   deltaLich(map[string]any{"sau": tomTatNgayNghiLe(moi)}),
		})
	})
	if err != nil {
		return domain.NgayNghiLe{}, err
	}
	return moi, nil
}

// SuaNgayNghi applies a partial edit to one closure date.
func (uc *Lich) SuaNgayNghi(ctx context.Context, id string, yc YeuCauSuaNgayNghi, nguoi NguoiThucHien) (domain.NgayNghiLe, error) {
	if err := nguoi.hopLe(); err != nil {
		return domain.NgayNghiLe{}, err
	}
	if id == "" {
		return domain.NgayNghiLe{}, idstore.ErrNgayNghiLeKhongTonTai
	}

	var sau domain.NgayNghiLe
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.nghi.TheoIDDeGhi(ctx, tx, id)
		if err != nil {
			return err
		}
		sau = truoc
		if yc.Ngay != nil {
			sau.Ngay = *yc.Ngay
		}
		if yc.Ten != nil {
			sau.Ten = *yc.Ten
		}
		if sau, err = uc.dungNgayNghi(sau); err != nil {
			return err
		}
		if sau == truoc {
			return nil
		}
		// ONLY WHEN THE DATE ACTUALLY MOVED. Re-checking a date that did not change would refuse an
		// edit of the NAME on a row that is already in conflict — locking the commune out of the one
		// screen that can describe what went wrong.
		if sau.Ngay != truoc.Ngay {
			if err := uc.khongTrungLamBu(ctx, tx, sau.Ngay); err != nil {
				return err
			}
		}
		if err := uc.nghi.CapNhat(ctx, tx, sau); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViSuaNgayNghiLe,
			Subject: chuDeNgayNghiLe(truoc),
			Delta: deltaLich(map[string]any{
				"truoc": tomTatNgayNghiLe(truoc),
				"sau":   tomTatNgayNghiLe(sau),
			}),
		})
	})
	if err != nil {
		return domain.NgayNghiLe{}, err
	}
	return sau, nil
}

// XoaNgayNghi soft deletes one closure date.
func (uc *Lich) XoaNgayNghi(ctx context.Context, id, lyDoTho string, nguoi NguoiThucHien) error {
	return uc.xoaMotDong(ctx, id, lyDoTho, nguoi, HanhViXoaNgayNghiLe,
		func(tx *store.ScopedTx, lyDo string) (string, any, error) {
			truoc, err := uc.nghi.TheoIDDeGhi(ctx, tx, id)
			if err != nil {
				return "", nil, err
			}
			if err := uc.nghi.XoaMem(ctx, tx, truoc.ID, nguoi.Vet.ID, lyDo); err != nil {
				return "", nil, err
			}
			return chuDeNgayNghiLe(truoc), tomTatNgayNghiLe(truoc), nil
		})
}

// GieoNgayNghiLeMacDinh writes the FIXED-DATE public holidays of one year that this commune does
// not have yet.
//
// =================================================================================================
// ⚠ IT SEEDS FOUR DAYS, NOT ELEVEN, AND THE ABSENCE IS THE DESIGN. domain.BoGieoNgayNghiLe owns the
// argument in full and it is not restated here (rule 9): Tết Nguyên đán and Giỗ Tổ Hùng Vương are
// LUNAR, and a hand-written lunar conversion that is one day out is a deadline counted through a
// day the office was shut; the day beside 02/9 is chosen by the Prime Minister each year between
// 01/9 and 03/9. The commune enters all three from the annual announcement.
//
// THE SCREEN OWES THE PERSON A SENTENCE SAYING SO. `seeded: 4` on its own reads as "done".
//
// THE YEAR IS A PARAMETER AND HAS NO DEFAULT. "This year" looks harmless and is not: on 31/12 the
// button would silently switch year at midnight, and a client that never sent the parameter would
// be seeding a different window every January with no line of code changing. Same refusal as
// docNamTruyVan on the read routes.
// =================================================================================================
//
// IDEMPOTENT IN THE SAME TWO SENSES AS THE WEEKLY SEED: a second run writes no duplicate (matched on
// the date, which is `UNIQUE (tenant_id, ngay)`'s own key, deleted rows included) and overwrites no
// name the commune edited (there is no path from here to nghi.CapNhat). A seed date the commune has
// already declared a SWAP WORKING DAY is skipped into BoQua rather than written — writing it would
// make both read routes refuse that whole year (domain.LoiNgayVuaNghiVuaLamBu).
func (uc *Lich) GieoNgayNghiLeMacDinh(ctx context.Context, nam int, nguoi NguoiThucHien) (KetQuaGieoLich, error) {
	if err := nguoi.hopLe(); err != nil {
		return KetQuaGieoLich{}, err
	}

	var kq KetQuaGieoLich
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		kq = KetQuaGieoLich{}

		dangCo, err := uc.nghi.TheoNamDeGhi(ctx, tx, nam)
		if err != nil {
			return err
		}
		co := make(map[string]bool, len(dangCo))
		for _, m := range dangCo {
			// Deleted rows count as present — the commune removed that day on purpose, and
			// `UNIQUE (tenant_id, ngay)` counts them too.
			co[m.Ngay.Ngay] = true
		}

		lamBu, err := uc.bu.TheoNamDeGhi(ctx, tx, nam)
		if err != nil {
			return err
		}
		laLamBu := make(map[string]bool, len(lamBu))
		for _, m := range lamBu {
			if !m.DaXoa {
				laLamBu[m.Ca.Ngay] = true
			}
		}

		var daGieo []string
		for _, g := range domain.BoGieoNgayNghiLe(nam) {
			moi := domain.NgayNghiLe{Ngay: g.Ngay, Ten: g.Ten}
			if co[moi.Ngay] {
				kq.DaCo++
				continue
			}
			if err := domain.KiemTraNgayNghiLe(moi); err != nil {
				return fmt.Errorf("ngay_nghi_le: bộ gieo có dòng không hợp lệ (%s): %w", moi.Ngay, err)
			}
			if laLamBu[moi.Ngay] {
				kq.BoQua++
				continue
			}
			if moi.ID, err = uc.sinhID(); err != nil {
				return fmt.Errorf("ngay_nghi_le: sinh id dòng gieo: %w", err)
			}
			if err := uc.nghi.Chen(ctx, tx, moi); err != nil {
				return err
			}
			daGieo = append(daGieo, chuDeNgayNghiLe(moi))
			kq.DaGieo++
		}

		if kq.DaGieo == 0 {
			return nil
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViGieoNgayNghiLe,
			Subject: chuDeGieoNgayNghiLe(nam),
			Delta: deltaLich(map[string]any{
				"nam":     nam,
				"da_gieo": kq.DaGieo,
				"da_co":   kq.DaCo,
				"bo_qua":  kq.BoQua,
				"ngay":    daGieo,
			}),
		})
	})
	if err != nil {
		return KetQuaGieoLich{}, err
	}
	return kq, nil
}

// dungNgayNghi normalises and validates one closure date, in one place, so the add path and the
// edit path cannot disagree about what a valid row is.
func (uc *Lich) dungNgayNghi(n domain.NgayNghiLe) (domain.NgayNghiLe, error) {
	ngay, err := domain.ChuanHoaNgay(n.Ngay)
	if err != nil {
		return domain.NgayNghiLe{}, err
	}
	ten, err := domain.ChuanHoaTenLich(n.Ten)
	if err != nil {
		return domain.NgayNghiLe{}, err
	}
	n.Ngay, n.Ten = ngay, ten
	return n, nil
}

// khongTrungLamBu refuses a date the commune has already declared a swap working day.
func (uc *Lich) khongTrungLamBu(ctx context.Context, tx *store.ScopedTx, ngay string) error {
	nam, err := namCuaNgay(ngay)
	if err != nil {
		return err
	}
	ds, err := uc.bu.TheoNamDeGhi(ctx, tx, nam)
	if err != nil {
		return err
	}
	for _, m := range ds {
		if !m.DaXoa && m.Ca.Ngay == ngay {
			// THE STORE'S OWN ERROR TYPE, so nothing downstream gains a second vocabulary for one
			// fault: the read routes answer 409 `calendar_conflict` on exactly this type.
			return &domain.LoiNgayVuaNghiVuaLamBu{Ngay: []string{ngay}}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------------------------
// ngay_lam_bu — the swap working days.
// ---------------------------------------------------------------------------------------------

// YeuCauThemLamBu is one new swap-day session.
type YeuCauThemLamBu struct {
	Ngay    string // YYYY-MM-DD
	BatDau  domain.GioTrongNgay
	KetThuc domain.GioTrongNgay
	Ten     string
}

// YeuCauSuaLamBu is a PARTIAL edit — see YeuCauSuaCa.
type YeuCauSuaLamBu struct {
	Ngay    *string
	BatDau  *domain.GioTrongNgay
	KetThuc *domain.GioTrongNgay
	Ten     *string
}

// ThemLamBu adds one session on a date the commune works although the week says otherwise.
//
// THREE REFUSALS, AND EACH ONE IS A STATE THE DEADLINE FUNCTION WOULD OTHERWISE MEET LATER:
//
//	the date is also a holiday        closed and working on one day, no winner (migration 0006:254)
//	the weekday already has sessions  ADR 0007 decision 9 — REPLACE or ADD, nothing decides
//	the session overlaps another      the same double count as the weekly calendar, on a table whose
//	                                  UNIQUE key also only stops two sessions STARTING together
//
// Meeting them at the write means the person who typed the row is told, on the screen, with the
// date named. Meeting them at the compute means a clerk somewhere is refused an entry into the
// register for a reason they did not cause.
func (uc *Lich) ThemLamBu(ctx context.Context, yc YeuCauThemLamBu, nguoi NguoiThucHien) (domain.CaLamBu, error) {
	if err := nguoi.hopLe(); err != nil {
		return domain.CaLamBu{}, err
	}
	moi, err := uc.dungLamBu(domain.CaLamBu{
		Ngay: yc.Ngay, BatDau: yc.BatDau, KetThuc: yc.KetThuc, Ten: yc.Ten,
	})
	if err != nil {
		return domain.CaLamBu{}, err
	}
	if moi.ID, err = uc.sinhID(); err != nil {
		return domain.CaLamBu{}, fmt.Errorf("ngay_lam_bu: sinh id: %w", err)
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.lamBuHopLeTrongLich(ctx, tx, moi); err != nil {
			return err
		}
		if err := uc.bu.Chen(ctx, tx, moi); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViThemNgayLamBu,
			Subject: chuDeCaLamBu(moi),
			Delta:   deltaLich(map[string]any{"sau": tomTatCaLamBu(moi)}),
		})
	})
	if err != nil {
		return domain.CaLamBu{}, err
	}
	return moi, nil
}

// SuaLamBu applies a partial edit to one swap-day session.
func (uc *Lich) SuaLamBu(ctx context.Context, id string, yc YeuCauSuaLamBu, nguoi NguoiThucHien) (domain.CaLamBu, error) {
	if err := nguoi.hopLe(); err != nil {
		return domain.CaLamBu{}, err
	}
	if id == "" {
		return domain.CaLamBu{}, idstore.ErrNgayLamBuKhongTonTai
	}

	var sau domain.CaLamBu
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.bu.TheoIDDeGhi(ctx, tx, id)
		if err != nil {
			return err
		}
		sau = truoc
		if yc.Ngay != nil {
			sau.Ngay = *yc.Ngay
		}
		if yc.BatDau != nil {
			sau.BatDau = *yc.BatDau
		}
		if yc.KetThuc != nil {
			sau.KetThuc = *yc.KetThuc
		}
		if yc.Ten != nil {
			sau.Ten = *yc.Ten
		}
		if sau, err = uc.dungLamBu(sau); err != nil {
			return err
		}
		if sau == truoc {
			return nil
		}
		if err := uc.lamBuHopLeTrongLich(ctx, tx, sau); err != nil {
			return err
		}
		if err := uc.bu.CapNhat(ctx, tx, sau); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViSuaNgayLamBu,
			Subject: chuDeCaLamBu(truoc),
			Delta: deltaLich(map[string]any{
				"truoc": tomTatCaLamBu(truoc),
				"sau":   tomTatCaLamBu(sau),
			}),
		})
	})
	if err != nil {
		return domain.CaLamBu{}, err
	}
	return sau, nil
}

// XoaLamBu soft deletes one swap-day session.
func (uc *Lich) XoaLamBu(ctx context.Context, id, lyDoTho string, nguoi NguoiThucHien) error {
	return uc.xoaMotDong(ctx, id, lyDoTho, nguoi, HanhViXoaNgayLamBu,
		func(tx *store.ScopedTx, lyDo string) (string, any, error) {
			truoc, err := uc.bu.TheoIDDeGhi(ctx, tx, id)
			if err != nil {
				return "", nil, err
			}
			if err := uc.bu.XoaMem(ctx, tx, truoc.ID, nguoi.Vet.ID, lyDo); err != nil {
				return "", nil, err
			}
			return chuDeCaLamBu(truoc), tomTatCaLamBu(truoc), nil
		})
}

// dungLamBu normalises and validates one swap-day session, in one place.
func (uc *Lich) dungLamBu(c domain.CaLamBu) (domain.CaLamBu, error) {
	ngay, err := domain.ChuanHoaNgay(c.Ngay)
	if err != nil {
		return domain.CaLamBu{}, err
	}
	ten, err := domain.ChuanHoaTenLich(c.Ten)
	if err != nil {
		return domain.CaLamBu{}, err
	}
	c.Ngay, c.Ten = ngay, ten
	if err := domain.KiemTraCaLamBu(c); err != nil {
		return domain.CaLamBu{}, err
	}
	return c, nil
}

// lamBuHopLeTrongLich runs the three cross-table refusals a swap day owes, inside the transaction.
func (uc *Lich) lamBuHopLeTrongLich(ctx context.Context, tx *store.ScopedTx, moi domain.CaLamBu) error {
	nam, err := namCuaNgay(moi.Ngay)
	if err != nil {
		return err
	}

	// (1) the date is also a holiday.
	nghi, err := uc.nghi.TheoNamDeGhi(ctx, tx, nam)
	if err != nil {
		return err
	}
	for _, m := range nghi {
		if !m.DaXoa && m.Ngay.Ngay == moi.Ngay {
			return &domain.LoiNgayVuaNghiVuaLamBu{Ngay: []string{moi.Ngay}}
		}
	}

	// (2) ADR 0007 decision 9 — the weekday already has sessions in the ordinary week.
	thu, err := domain.ThuCuaNgay(moi.Ngay)
	if err != nil {
		return err
	}
	tuan, err := uc.tuan.DanhSachDeGhi(ctx, tx)
	if err != nil {
		return err
	}
	for _, m := range tuan {
		if !m.DaXoa && m.Ca.Thu == thu {
			return domain.LoiLamBuVaoNgayDaLamViec(moi.Ngay, thu)
		}
	}

	// (3) the session overlaps another swap-day session on the same date.
	bu, err := uc.bu.TheoNamDeGhi(ctx, tx, nam)
	if err != nil {
		return err
	}
	song := make([]domain.Khoang, 0, len(bu))
	for _, m := range bu {
		if m.DaXoa || m.Ca.ID == moi.ID {
			continue
		}
		song = append(song, domain.KhoangCuaCaLamBu(m.Ca))
	}
	return domain.KhongChongCaNao(domain.KhoangCuaCaLamBu(moi), song)
}

// ---------------------------------------------------------------------------------------------
// The pieces all three tables share.
// ---------------------------------------------------------------------------------------------

// xoaMotDong is the one soft-delete shape: check the actor, normalise the reason, then read the row
// and write it and its trail INSIDE ONE TRANSACTION.
//
// ONE FUNCTION FOR THREE TABLES because three copies of "read, delete, audit" is three places for
// the audit entry to drift out of the transaction — the exact defect rule 6, invariant 3 exists to
// prevent, and the one the previous system had everywhere.
//
// THE REASON IS IN THE ENTRY AS WELL AS IN THE COLUMN, and that is not the duplication rule 9
// forbids: the column is the current state of the row and can only ever hold the FIRST deletion,
// while the entry is the append-only record of the act. Neither can be derived from the other.
func (uc *Lich) xoaMotDong(ctx context.Context, id, lyDoTho string, nguoi NguoiThucHien, hanhVi string,
	lam func(tx *store.ScopedTx, lyDo string) (string, any, error)) error {

	if err := nguoi.hopLe(); err != nil {
		return err
	}
	if id == "" {
		// The caller's own not-found sentinel is produced by `lam` in every real case; this branch
		// exists so an empty path value never reaches a statement that would compare it against
		// whatever row happens to carry the empty string.
		return errors.New("lịch làm việc: thiếu mã dòng cần xoá")
	}
	lyDo, err := domain.ChuanHoaLyDoXoaLich(lyDoTho)
	if err != nil {
		return err
	}

	return uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		chuDe, truoc, err := lam(tx, lyDo)
		if err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  hanhVi,
			Subject: chuDe,
			Delta: deltaLich(map[string]any{
				"truoc":   truoc,
				"ly_do":   lyDo,
				"xoa_mem": true,
			}),
		})
	})
}

// namCuaNgay reads the year out of a `YYYY-MM-DD` date, so a cross-table check reads ONE year rather
// than the whole table.
//
// THE DATE HAS ALREADY BEEN NORMALISED by domain.ChuanHoaNgay before any caller reaches here, so
// this cannot fail on a real path; the error is returned rather than ignored because a silent zero
// would make the check read the year 0 and find nothing — a refusal that quietly stops refusing.
func namCuaNgay(ngay string) (int, error) {
	if len(ngay) < 4 {
		return 0, domain.ErrNgayKhongDocDuoc
	}
	nam, err := strconv.Atoi(ngay[:4])
	if err != nil {
		return 0, domain.ErrNgayKhongDocDuoc
	}
	return nam, nil
}

// The audit subjects. NONE OF THEM IS A ULID, for the same reason rule 6, invariant 8 puts a staff
// code rather than an internal id in `actor_id`: `lich-lam-viec/thu-2/07:30:00` tells somebody
// handling an inspection which session was changed, with no lookup still alive. A ULID names
// nothing, and the row it points at may by then have been removed.
//
// NONE OF THEM IS PERSONAL DATA (rule 3): a weekday, a date and a time of day say when an authority
// is open. They are printed on the noticeboard in the lobby.
func chuDeCaLamViec(c domain.CaLamViec) string {
	return "lich-lam-viec/thu-" + strconv.Itoa(c.Thu) + "/" + c.BatDau.Chuoi()
}

func chuDeNgayNghiLe(n domain.NgayNghiLe) string { return "ngay-nghi-le/" + n.Ngay }

func chuDeCaLamBu(c domain.CaLamBu) string { return "ngay-lam-bu/" + c.Ngay + "/" + c.BatDau.Chuoi() }

func chuDeGieoNgayNghiLe(nam int) string {
	return "cau-hinh-ngay-nghi-le/" + strconv.Itoa(nam)
}

// The audit deltas. THE TIMES ARE RENDERED AS HH:MM:SS AND NOT AS SECONDS, because an entry read
// years later by a person handling a complaint has to be readable without this source code.
func tomTatCaLamViec(c domain.CaLamViec) map[string]any {
	return map[string]any{
		"thu": c.Thu, "bat_dau": c.BatDau.Chuoi(), "ket_thuc": c.KetThuc.Chuoi(), "ghi_chu": c.GhiChu,
	}
}

func tomTatNgayNghiLe(n domain.NgayNghiLe) map[string]any {
	return map[string]any{"ngay": n.Ngay, "ten": n.Ten}
}

func tomTatCaLamBu(c domain.CaLamBu) map[string]any {
	return map[string]any{
		"ngay": c.Ngay, "bat_dau": c.BatDau.Chuoi(), "ket_thuc": c.KetThuc.Chuoi(), "ten": c.Ten,
	}
}

// deltaLich marshals the audit payload.
//
// A MARSHAL FAILURE PRODUCES AN EXPLICIT MARKER RATHER THAN A NIL DELTA. These payloads hold only
// ints and short strings, so it cannot fail today; if it ever does, an entry saying the delta could
// not be rendered is auditable and an entry with an empty delta silently is not.
func deltaLich(v map[string]any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{"loi":"khong_dung_duoc_delta"}`)
	}
	return b
}

// LaLoiXungDotLich reports whether this failure is a calendar that contradicts itself — a state the
// COMMUNE must fix, not a bad request and not a server fault.
//
// TWO TYPES, ONE ANSWER: a date that is both a holiday and a swap day, and a swap day on a weekday
// that already works. Both are ADR 0007's "the system refuses rather than picking a winner", both
// carry a sentence naming the date, and both map to 409.
func LaLoiXungDotLich(err error) bool {
	var vuaNghiVuaBu *domain.LoiNgayVuaNghiVuaLamBu
	var khongTinhDuoc *domain.LoiKhongTinhDuocHan
	var chongCa *domain.LoiCaChongCaKhac
	return errors.As(err, &vuaNghiVuaBu) || errors.As(err, &khongTinhDuoc) || errors.As(err, &chongCa)
}
