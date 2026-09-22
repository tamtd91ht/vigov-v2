package app

// The use cases behind the WRITE surface of the commune's processing-deadline table
// (migration 0008, ADR 0029).
//
// WHY THIS LAYER EXISTS: rule 6, invariant 3 requires the audit entry to share a transaction with
// the business write, and core/audit.Write takes a *store.ScopedTx with no overload that writes
// outside one. Opening that transaction is this layer's job. The handler translates HTTP and
// nothing else; the store knows SQL and nothing else.
//
// =================================================================================================
// TWO OPERATIONS, AND THE SECOND ONE IS THE EXPENSIVE ONE TO GET WRONG.
//
//	Sua          change the five hour figures of ONE existing row.
//	GieoMacDinh  write the rows of domain.BoGieoSLA() that this commune does not have yet.
//
// GieoMacDinh IS IDEMPOTENT IN THE ONLY SENSE THAT MATTERS: pressing the button twice writes no
// duplicate row AND OVERWRITES NO FIGURE THE COMMUNE HAS EDITED. The second half is the one worth
// stating, because the obvious implementation — an UPSERT — satisfies the first and destroys the
// second. A commune that changed 40 working hours to 24, and then had somebody press "seed
// defaults" and silently get 40 back, is worse off than a commune with no button at all: the
// number on the screen is one nobody chose, and nothing reports that it moved.
//
// SO IT ONLY EVER INSERTS. There is no path from this file to store.CapNhatGio, and the store
// offers no UPSERT to reach for.
//
// =================================================================================================
// NEITHER OPERATION TOUCHES A DEADLINE ALREADY ISSUED, AND NOTHING HERE RECOMPUTES ONE.
//
// 14-cau-hinh.md:289 — *"Thay đổi chỉ áp dụng cho hồ sơ tiếp nhận sau thời điểm lưu"* — and
// rule 10, invariant 2: each deadline is computed ONCE, at the act that fixes it, and stored
// (`han_tiep_nhan` when the row is created, `han_xu_ly_xong` when the field is settled, ADR 0028).
// Those columns live on the petition and on the document, in `service-petitions` and
// `service-documents`, which this service cannot write to at all (rule 2, forbidden #2).
//
// THAT IS NOT MERELY "WE HAPPEN NOT TO". A recomputation would move a commitment already made to a
// named citizen, after the fact, with the citizen having already been told the old one. If a later
// turn finds any path that recomputes a stored deadline from this table, that is a defect to report,
// not a feature to finish.

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

// KhoSLA is the store, declared at the point of use.
//
// EVERY MUTATING METHOD TAKES THE TRANSACTION, which is what makes it impossible to write the row
// in one transaction and the audit entry in another: there is no signature here that would let you.
// DanhSachDeGhi takes it too, and that is not symmetry for its own sake — the seeding decision is
// read-decide-write, and a read outside the transaction is a check with a gap in the middle through
// which a second seeding run inserts the same row.
type KhoSLA interface {
	DanhSachDeGhi(ctx context.Context, tx *store.ScopedTx) ([]domain.DongSLA, error)
	TheoIDDeGhi(ctx context.Context, tx *store.ScopedTx, id string) (domain.DongSLA, error)
	CapNhatGio(ctx context.Context, tx *store.ScopedTx, d domain.DongSLA) error
	Chen(ctx context.Context, tx *store.ScopedTx, d domain.DongSLA) error
}

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system writes (`dang_nhap`, `them_can_bo`): an inspection reads these strings, and a function
// name would tell them nothing.
//
// TWO VERBS AND NOT ONE `cau_hinh_sla`. Seeding a commune's first table and changing a figure on a
// live one are two different acts with two different consequences, and in a ledger that is never
// deleted they must not be one string — otherwise telling "the commune was set up" from "somebody
// shortened the security deadline" requires opening and interpreting every delta.
const (
	HanhViSuaSLA  = "sua_thoi_han_xu_ly"
	HanhViGieoSLA = "gieo_thoi_han_xu_ly_mac_dinh"
)

// ErrKhongCoGiDeGieo is not an error the handler turns into a failure.
//
// IT DOES NOT EXIST, DELIBERATELY — and this comment stands where it would have been. A second
// seeding run that finds every row present is a SUCCESS: the commune's table is in exactly the
// state the caller asked for. Turning it into an error would make the screen show a failure for a
// button that worked, and would push a client towards retrying. KetQuaGieo carries the counts
// instead, so the caller can say "15 dòng đã có sẵn" without anything having gone wrong.

// SLA owns the write surface of one commune's deadline table.
type SLA struct {
	db  *store.DB
	kho KhoSLA

	// Injected so a test can pin it. In production: ulid.Moi.
	sinhID func() (string, error)
}

func NewSLA(db *store.DB, kho KhoSLA) *SLA {
	return &SLA{db: db, kho: kho, sinhID: ulid.Moi}
}

// YeuCauSuaSLA is a PARTIAL edit: a nil pointer means "leave this figure alone".
//
// WHY POINTERS AND NOT FIVE PLAIN INTS. With plain ints this struct could not tell "the client did
// not mention this figure" from "the client sent zero", and zero is refused by both
// domain.KiemTraGio and the CHECK constraint — so a screen editing only `gio_tiep_nhan` would send
// four zeros and have the whole edit refused, or, with the check removed, write four zeros into a
// commune's commitments. The specification's screen edits a whole row, but the contract must not
// depend on a client always sending all five.
//
// THERE IS NO `LoaiViec`, NO `LinhVuc` AND NO `ID` FIELD. What a row applies to is fixed when the
// row is created: a field here would let a screen silently re-point an existing commitment at
// another field, and it is also the parameter that would need the write-time code check ADR 0026
// stop condition #2 governs. The id is a separate argument because it names the row rather than
// changing it.
type YeuCauSuaSLA struct {
	GioTiepNhan   *int
	GioXuLyXong   *int
	GioSapDenHan  *int
	GioBaoLanhDao *int
	GioBaoChuTich *int
}

// Sua applies a partial edit to one deadline row.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING. Sending the figures a row already has is not an event;
// recording it would fill a public authority's ledger with entries saying nothing changed, and
// those are the entries that bury the ones carrying legal weight. Same discipline as
// DanhBaCanBo.Sua, and it is also what makes the route genuinely idempotent.
func (uc *SLA) Sua(ctx context.Context, id string, yc YeuCauSuaSLA,
	nguoi NguoiThucHien) (domain.DongSLA, error) {

	if err := nguoi.hopLe(); err != nil {
		return domain.DongSLA{}, err
	}
	if id == "" {
		return domain.DongSLA{}, idstore.ErrDongSLAKhongTonTai
	}

	var sau domain.DongSLA
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// FOR UPDATE, inside the transaction. Two administrators editing one row both read the old
		// figures otherwise, and the second write silently discards the first — on a table whose
		// values are commitments to citizens.
		truoc, err := uc.kho.TheoIDDeGhi(ctx, tx, id)
		if err != nil {
			return err
		}

		sau = truoc
		apDungSuaSLA(&sau, yc)

		// THE RESULT IS VALIDATED, NOT THE REQUEST. A row already holding a bad figure — restored
		// from elsewhere, written before this check existed — must not be committed again untouched
		// by a screen that only meant to change one number. See domain.KiemTraDongSLA.
		if err := domain.KiemTraDongSLA(sau); err != nil {
			return err
		}

		if sau == truoc {
			return nil
		}
		if err := uc.kho.CapNhatGio(ctx, tx, sau); err != nil {
			return err
		}

		// SAME TRANSACTION AS THE UPDATE (rule 6, invariant 3). TenantID is left unset on purpose:
		// audit.Write fills it from the transaction, which took it from the context (rule 1,
		// invariant 4). Passing it here would be a second source for the one fact that decides which
		// commune the entry belongs to.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViSuaSLA,
			Subject: chuDeSLA(truoc),
			Delta: deltaSLA(map[string]any{
				"truoc": soGioCuaDong(truoc),
				"sau":   soGioCuaDong(sau),
			}),
		})
	})
	if err != nil {
		return domain.DongSLA{}, err
	}
	return sau, nil
}

// apDungSuaSLA copies the figures the request actually mentioned onto the row that was read.
func apDungSuaSLA(d *domain.DongSLA, yc YeuCauSuaSLA) {
	if yc.GioTiepNhan != nil {
		d.GioTiepNhan = *yc.GioTiepNhan
	}
	if yc.GioXuLyXong != nil {
		d.GioXuLyXong = *yc.GioXuLyXong
	}
	if yc.GioSapDenHan != nil {
		d.GioSapDenHan = *yc.GioSapDenHan
	}
	if yc.GioBaoLanhDao != nil {
		d.GioBaoLanhDao = *yc.GioBaoLanhDao
	}
	if yc.GioBaoChuTich != nil {
		d.GioBaoChuTich = *yc.GioBaoChuTich
	}
}

// KetQuaGieo is what one seeding run did.
//
// TWO COUNTS AND NOT A BOOLEAN, because the two second-run cases are different things a person
// needs told apart: "the commune was already fully configured" (DaCo = 15, DaGieo = 0) and "the
// commune had some rows and the missing ones were added" (both non-zero). A boolean would answer
// neither, and the screen would have to guess.
type KetQuaGieo struct {
	// DaGieo is how many rows this run actually inserted.
	DaGieo int

	// DaCo is how many rows of the seed set the commune already had, and which were therefore LEFT
	// EXACTLY AS THEY WERE — including any figure the commune had edited.
	DaCo int
}

// GieoMacDinh writes the seed rows this commune does not have yet.
//
// =================================================================================================
// WHAT MAKES IT SAFE TO PRESS TWICE, in order:
//
//  1. The existing rows are read INSIDE the transaction. A read outside it is a check with a gap
//     through which a concurrent run inserts the same row.
//  2. A seed row is matched to an existing row by (loai_viec, linh_vuc) — THE UNIQUE KEY'S OWN
//     PAIR, so "already present" here means exactly what the database means by it. Matching on the
//     figures instead would re-seed every row a commune had edited, which is the defect this whole
//     method is shaped to avoid.
//  3. A row that is present is SKIPPED ENTIRELY. Not updated, not compared, not counted as a
//     conflict. Whatever the commune put there stays.
//  4. If two runs still race, `UNIQUE (tenant_id, loai_viec, linh_vuc_khoa)` refuses the loser and
//     the whole transaction rolls back — no duplicate, and no half-seeded table.
//
// NOTHING IS WRITTEN AND NOTHING IS AUDITED WHEN THERE IS NOTHING TO SEED. Same reasoning as
// Sua's no-op: an entry recording that a button did nothing buries the entries that carry legal
// weight.
// =================================================================================================
func (uc *SLA) GieoMacDinh(ctx context.Context, nguoi NguoiThucHien) (KetQuaGieo, error) {
	if err := nguoi.hopLe(); err != nil {
		return KetQuaGieo{}, err
	}

	var kq KetQuaGieo
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// Reset on every attempt: store.Tx does not retry today, but a counter that survives into a
		// second call would report a total rather than this run's work.
		kq = KetQuaGieo{}

		dangCo, err := uc.kho.DanhSachDeGhi(ctx, tx)
		if err != nil {
			return err
		}
		co := make(map[khoaDongSLA]bool, len(dangCo))
		for _, d := range dangCo {
			co[khoaDongSLA{d.LoaiViec, d.LinhVuc}] = true
		}

		for _, g := range domain.BoGieoSLA() {
			if co[khoaDongSLA{g.LoaiViec, g.LinhVuc}] {
				kq.DaCo++
				continue
			}

			moi := domain.DongSLA{
				LoaiViec:      g.LoaiViec,
				LinhVuc:       g.LinhVuc,
				GioTiepNhan:   g.GioTiepNhan,
				GioXuLyXong:   g.GioXuLyXong,
				GioSapDenHan:  g.GioSapDenHan,
				GioBaoLanhDao: g.GioBaoLanhDao,
				GioBaoChuTich: g.GioBaoChuTich,
			}
			// THE SEED SET IS VALIDATED ON THE WAY IN, exactly like a typed figure. It is a fixed
			// list in source, so this can only fire if somebody edits that list badly — which is
			// precisely when it is worth catching, because the alternative is a constraint violation
			// at a commune's first configuration and no sentence saying which row.
			if err := domain.KiemTraDongSLA(moi); err != nil {
				return fmt.Errorf("sla: bộ gieo có dòng không hợp lệ (%s/%s): %w",
					g.LoaiViec, nhanLinhVuc(g.LinhVuc), err)
			}
			if moi.ID, err = uc.sinhID(); err != nil {
				return fmt.Errorf("sla: sinh id dòng gieo: %w", err)
			}
			if err := uc.kho.Chen(ctx, tx, moi); err != nil {
				return err
			}
			kq.DaGieo++
		}

		if kq.DaGieo == 0 {
			return nil
		}

		// SAME TRANSACTION AS EVERY INSERT (rule 6, invariant 3). One entry for the run rather than
		// one per row: the act a person performed was "set this commune up", and fifteen entries
		// would describe fifteen acts that never happened.
		//
		// THE SUBJECT IS THE COMMUNE'S OWN CONFIGURATION, not a row id — see chuDeGieoSLA.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViGieoSLA,
			Subject: chuDeGieoSLA,
			Delta: deltaSLA(map[string]any{
				"da_gieo": kq.DaGieo,
				"da_co":   kq.DaCo,
				// The rows written, so an inspection can see WHICH figures this commune started from
				// without needing the source of the release that was deployed that day.
				"dong": moTaDaGieo(dangCo),
			}),
		})
	})
	if err != nil {
		return KetQuaGieo{}, err
	}
	return kq, nil
}

// khoaDongSLA is the pair the unique key is built on. A struct key and not a joined string: a
// separator inside a field code would make two different pairs collide, and the code set is not
// this service's to constrain.
type khoaDongSLA struct {
	loaiViec domain.LoaiViec
	linhVuc  string
}

// chuDeGieoSLA is the audit subject of a seeding run.
//
// audit.Entry REQUIRES A SUBJECT and rule 6, invariant 2 wants the record the act was performed ON.
// A seeding run acts on the commune's deadline TABLE, which has no business code — there is no
// `ma` column on `sla` and inventing one would be a second identifier for rows that already have
// ULIDs. The commune itself is already on every entry (`tenant_id`), so a constant naming WHAT was
// configured is the honest subject: `cau-hinh-sla` reads, years later, as "the processing-deadline
// configuration", which is exactly what changed.
const chuDeGieoSLA = "cau-hinh-sla"

// chuDeSLA names the row an edit acted on.
//
// `<loai_viec>/<linh_vuc>` AND NOT THE ULID, for the same reason rule 6, invariant 8 puts a staff
// code rather than an internal id in `actor_id`: `phan-anh/an-ninh` tells somebody handling an
// inspection which commitment was changed, with no lookup still alive. A ULID names nothing, and
// the row it points at may by then have been replaced.
//
// NEITHER PART IS PERSONAL DATA (rule 3): a kind of work and a field code are the same strings the
// configuration screen prints.
func chuDeSLA(d domain.DongSLA) string {
	return string(d.LoaiViec) + "/" + nhanLinhVuc(d.LinhVuc)
}

// nhanLinhVuc renders the default row's empty field code as a word rather than as nothing.
//
// "mac-dinh" AND NOT "": an audit subject reading `phan-anh/` is a subject somebody has to know the
// convention to read, and the default row is the single most consequential row in the table — it is
// what ADR 0028 decision E reads for every petition a citizen sends.
func nhanLinhVuc(linhVuc string) string {
	if linhVuc == "" {
		return "mac-dinh"
	}
	return linhVuc
}

// soGioCuaDong is the before/after payload of an edit: THE FIVE FIGURES AND NOTHING ELSE.
//
// Rule 6, invariant 5 asks for before and after of the significant fields. The significant fields
// here are exactly the five that can change; `loai_viec` and `linh_vuc` cannot (the UPDATE does not
// name them) and repeating them in both halves would be two copies of an unchanging fact.
//
// NOTHING HERE IS PERSONAL DATA, so there is nothing to mask (rule 3, rule 6, forbidden #4): every
// value is a count of working hours in a commune's published policy.
func soGioCuaDong(d domain.DongSLA) map[string]any {
	return map[string]any{
		"gio_tiep_nhan":    d.GioTiepNhan,
		"gio_xu_ly_xong":   d.GioXuLyXong,
		"gio_sap_den_han":  d.GioSapDenHan,
		"gio_bao_lanh_dao": d.GioBaoLanhDao,
		"gio_bao_chu_tich": d.GioBaoChuTich,
	}
}

// moTaDaGieo names the rows a seeding run wrote — the seed set minus what was already there.
//
// IT IS COMPUTED FROM THE SAME `dangCo` THE LOOP DECIDED ON, rather than accumulated inside it, so
// the entry cannot describe a different set from the one that was written.
func moTaDaGieo(dangCo []domain.DongSLA) []string {
	co := make(map[khoaDongSLA]bool, len(dangCo))
	for _, d := range dangCo {
		co[khoaDongSLA{d.LoaiViec, d.LinhVuc}] = true
	}
	ra := make([]string, 0, len(domain.BoGieoSLA()))
	for _, g := range domain.BoGieoSLA() {
		if !co[khoaDongSLA{g.LoaiViec, g.LinhVuc}] {
			ra = append(ra, string(g.LoaiViec)+"/"+nhanLinhVuc(g.LinhVuc))
		}
	}
	return ra
}

// deltaSLA marshals the audit payload.
//
// A MARSHAL FAILURE PRODUCES AN EXPLICIT MARKER RATHER THAN A NIL DELTA. These payloads hold only
// ints and short ASCII strings, so it cannot fail today; if it ever does, an entry saying the delta
// could not be rendered is auditable and an entry with an empty delta silently is not.
func deltaSLA(v map[string]any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{"loi":"khong_dung_duoc_delta"}`)
	}
	return b
}

// LaLoiDauVaoSLA reports whether this is a refusal of what the client sent, as opposed to a failure.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400, following laLoiDauVaoCanBo: a default of
// "anything I do not recognise is the client's fault" turns a database outage into a 400, and a
// client that believes its input is wrong retries with different input forever while nobody is told
// the server is broken.
func LaLoiDauVaoSLA(err error) bool {
	return errors.Is(err, domain.ErrGioPhaiDuong) || errors.Is(err, domain.ErrGioQuaLon)
}
