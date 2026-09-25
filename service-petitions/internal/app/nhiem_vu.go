package app

// The SIX STAFF acts on a task — create, edit, move, soft delete, ask for more time, decide on it.
//
// Until this file a commune could READ its task register and change nothing in it: §7's "Giao việc
// mới" had no server behind it, and a task that arrived from a meeting conclusion sat where it
// landed with a deadline running against it.
//
// # WHY THIS LAYER EXISTS
//
// Rule 6, invariant 3 requires the audit entry to share a transaction with the business write, and
// core/audit.Write takes a *store.ScopedTx with no overload that writes outside one. Opening that
// transaction is this layer's job. The handler translates HTTP and nothing else; the store knows
// SQL and nothing else.
//
// THE SECOND REASON IS THE TREE. ADR 0037 made the sub-task tree UNLIMITED IN DEPTH, and three of
// its four decisions are rules about the RELATIONSHIP BETWEEN ROWS — which a CHECK constraint
// cannot express, because a CHECK sees one row:
//
//	decision 2  a child's deadline is its OWN         this layer simply never copies the parent's
//	decision 3  no soft delete while children live    Xoa, on the tree read under the lock
//	decision 4  no `hoan-thanh` while work remains    duyetCaCay, RECURSIVE, under the lock
//	+ the fifth rule, in no specification             kiemChuTrinh, walking UPWARD, under the lock
//
// Every one of them is decided INSIDE the transaction, on rows read `FOR UPDATE`. Outside it, two
// officers acting at once both read the old tree and both decide against it — and the pair that
// costs the most is completing a parent whose last child was finished by nobody.
//
// ---------------------------------------------------------------------------
// THE DATABASE IS THE FLOOR AND THIS LAYER IS THE SENTENCE. Said once, here:
//
//	the seven statuses               migration 0006, `nhiem_vu_trang_thai_hop_le`
//	`ma` and `han_ban_dau` immutable         0006, trigger `nhiem_vu_bat_bien`
//	hard removal refused outright            0006, same trigger
//	one pending extension per task           0006, UNIQUE (tenant_id, nhiem_vu_id, moc_cho_duyet)
//	the request AS FILED is immutable        0006, trigger `de_nghi_lui_han_bat_bien`
//	the progress log is append-only          0006, trigger `nhat_ky_nhiem_vu_chi_them`
//	a task is its own parent -> refused      0008, CHECK `nhiem_vu_khong_tu_lam_cha`
//
// Those hold against every writer — this service, a psql prompt, an import job written next year.
// What they cannot do is explain themselves, and they cannot express a rule that spans two rows at
// all. This layer refuses FIRST, in Vietnamese. A drift between the two is therefore a worse error
// message, never a hole.
//
// ---------------------------------------------------------------------------
// ⚠ WHERE A TASK'S DEADLINE COMES FROM, BECAUSE THE ANSWER IS NOT THE PETITION REGISTER'S
//
// A petition's deadline is COMPUTED: the citizen is promised a date the authority derives from its
// own SLA table and its own calendar, so `xu_ly_phan_anh.go` asks identity for it and REFUSES the
// act when the commune has not configured the hours.
//
// A TASK'S DEADLINE IS TYPED BY THE PERSON GIVING THE WORK OUT. §7.1 puts "Hạn hoàn thành" on the
// form as a date field, not marked required, beside "Lãnh đạo giao việc" — it is one officer
// committing to another, not the authority committing to a citizen. So this file performs NO
// working-hours arithmetic and does not call identity at all; what reaches `han_xu_ly` is the
// instant the form carried, stored ONCE at the act that fixes it (rule 10, invariant 2; ADR 0028).
//
// THAT IS NOT A WAY ROUND RULE 10, FORBIDDEN #2. Nothing below derives an instant from another
// instant: there is no duration added to a date anywhere in this file, so there is no arithmetic
// here that could walk through a night, a weekend, `ngay_nghi_le` or `ngay_lam_bu` while looking
// like it respected the unit. The moment a deadline has to be DERIVED for a task — §8's Excel
// import with no date in the column, or splitting a meeting conclusion whose sentence says "báo cáo
// trước ngày 20/8" — the answer is identity's ResolveDeadlines with WORK_KIND_NHIEM_VU, which
// EXISTS and is implemented (proto/vigov/identity/v1/identity.proto, enum `WorkKind`;
// service-identity/internal/grpc/sla.go:234 maps it to `loai_viec = 'nhiem-vu'`). Neither of those
// paths is in this pass.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// KhoNhiemVuGhi is the write half of the task register as this layer needs it, declared at the
// point of use.
//
// EVERY METHOD TAKES THE TRANSACTION. That is what makes it impossible to move a task in one
// transaction and record the trail in another: there is no signature here that would let you. It is
// also what makes the properties worth proving — that a refusal writes nothing, that the entry and
// the timeline row share the transaction, that the rows are read under a lock — provable without a
// PostgreSQL, of which there is none reachable from this build environment (VIGOV_TEST_DSN unset).
//
// *petstore.NhiemVuStore satisfies it as it is.
type KhoNhiemVuGhi interface {
	TheoMaDeSua(ctx context.Context, tx *store.ScopedTx, ma string) (domain.NhiemVu, error)
	TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.NhiemVu, error)

	// The two tree reads. ONE LEVEL EACH, and the recursion is this layer's — see duyetCaCay.
	ConTrucTiep(ctx context.Context, tx *store.ScopedTx, chaID string) ([]domain.NhiemVuTomTat, error)
	ChaCua(ctx context.Context, tx *store.ScopedTx, id string) (string, error)

	SoLonNhatDaCap(ctx context.Context, tx *store.ScopedTx) (int, error)
	MaDaDung(ctx context.Context, tx *store.ScopedTx, ma string) (bool, error)

	Tao(ctx context.Context, tx *store.ScopedTx, n domain.NhiemVu) error
	Sua(ctx context.Context, tx *store.ScopedTx, id string, n domain.NhiemVu) error
	DoiTrangThai(ctx context.Context, tx *store.ScopedTx, id string,
		tu, sang domain.TrangThaiNhiemVu, ngayHoanThanh time.Time) error
	DoiHanXuLy(ctx context.Context, tx *store.ScopedTx, id string, hanCu, hanMoi time.Time) error
	XoaMem(ctx context.Context, tx *store.ScopedTx, id, nguoiMa, lyDo string, luc time.Time) error

	GhiNhatKy(ctx context.Context, tx *store.ScopedTx, e domain.NhatKyNhiemVu) error
	NhatKyGanNhat(ctx context.Context, tx *store.ScopedTx, nhiemVuID string, n int) (
		[]domain.MocNhatKy, error)

	// §5.4's document block (migration 0009). FOUR METHODS ON THIS INTERFACE AND NOT A FIFTH
	// INTERFACE, unlike KhoDeNghiLuiHan below, and the difference is what the rows ARE: a line is a
	// FIELD VALUE of the task, written by the same acts that write the task itself, while an
	// extension request is a record with its own lifecycle decided by a different person under a
	// different permission.
	//
	// EVERY ONE OF THEM TAKES THE TRANSACTION, including the read. A block read outside the
	// transaction would be a block another officer could change before the diff computed against it
	// was applied — and the diff is what decides which lines are REMOVED.
	VanBanCuaNhiemVuDeSua(ctx context.Context, tx *store.ScopedTx, nhiemVuID string) (
		[]domain.NhiemVuVanBan, error)
	ThuTuVanBanLonNhat(ctx context.Context, tx *store.ScopedTx, nhiemVuID string,
		nhom domain.NhomVanBanNhiemVu) (int, error)
	ThemVanBan(ctx context.Context, tx *store.ScopedTx, v domain.NhiemVuVanBan) error
	SuaVanBan(ctx context.Context, tx *store.ScopedTx, v domain.NhiemVuVanBan) error
	XoaMemVanBan(ctx context.Context, tx *store.ScopedTx, nhiemVuID, id, nguoiMa string,
		luc time.Time) error
}

// KhoDeNghiLuiHan is the extension-request table.
//
// A SEPARATE INTERFACE FROM KhoNhiemVuGhi AND NOT FOUR MORE METHODS ON IT, because the two have
// different obligations: everything above changes the task itself, while these four record who
// asked for more time and what the answer was. Behind one interface a later caller would reach for
// whichever method was nearest — and the one it would reach for is the one that moves a deadline
// without a decision row beside it.
type KhoDeNghiLuiHan interface {
	Tao(ctx context.Context, tx *store.ScopedTx, d domain.DeNghiLuiHan) error
	DangChoDuyet(ctx context.Context, tx *store.ScopedTx, nhiemVuID string) (bool, error)
	TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, nhiemVuID, id string) (
		domain.DeNghiLuiHan, error)
	QuyetDinh(ctx context.Context, tx *store.ScopedTx, id string, sang domain.TrangThaiDeNghi,
		nguoiDuyetMa string, luc time.Time) error
}

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes (`dang_nhap`, `vao_so_van_ban_den`, `phan_loai_phan_anh`) — an inspection
// reads these strings, and a function name would tell them nothing.
//
// SIX VERBS AND NOT ONE `sua_nhiem_vu`, because they are six different administrative acts under
// four different permissions. `quyet_dinh_lui_han_nhiem_vu` in particular is the one an inspection
// searches for by name: it is the act that MOVED A DEADLINE, and the entry carries both the old and
// the new one.
const (
	HanhViTaoNhiemVu         = "tao_nhiem_vu"
	HanhViSuaNhiemVu         = "sua_nhiem_vu"
	HanhViChuyenTrangNhiemVu = "chuyen_trang_thai_nhiem_vu"
	HanhViXoaNhiemVu         = "xoa_nhiem_vu"
	HanhViDeNghiLuiHan       = "de_nghi_lui_han_nhiem_vu"
	HanhViQuyetDinhLuiHan    = "quyet_dinh_lui_han_nhiem_vu"
)

// QuyenDuyetHoanThanh is the answer to ONE question, asked at the edge and carried down as a FACT
// rather than as a decision: does this principal hold `task.approve` — "Duyệt hoàn thành"?
//
// # WHY IT IS A SECOND KEY ON A ROUTE THAT ALREADY DECLARES ONE
//
// §6 names both: `task.update` ("Cập nhật tiến độ") for moving work along, and `task.approve`
// ("Duyệt hoàn thành") for the step that declares it finished. Rule 5, invariant 3b says these
// rights are not a Cartesian product, and this is exactly such a pair: an officer reports progress
// on their own work, and somebody else signs it off. A route can declare ONE permission, so the
// gate is the broader key and this layer holds the narrower one — the same shape the petition path
// uses for its holding rule.
//
// IT IS A NAMED TYPE AND NOT A BARE bool, because a bare bool at a call site reads as nothing at
// all, and the wrong literal there lets every holder of `task.update` sign off their own work.
//
// THE DECISION IS NOT MADE WHERE THIS VALUE IS PRODUCED. The handler may only answer "does this
// account hold the key"; whether the act is allowed is duocHoanThanh's, below, inside the
// transaction — which is what keeps the rule provable without an HTTP request and impossible to
// bypass by adding a second caller.
type QuyenDuyetHoanThanh bool

// ErrKhongDuocDuyetHoanThanh refuses the final step to somebody who may update a task but may not
// declare it finished.
var ErrKhongDuocDuyetHoanThanh = errors.New(
	"nhiem_vu: hoàn thành nhiệm vụ cần quyền duyệt hoàn thành, không chỉ quyền cập nhật tiến độ")

// GhiNhiemVu owns the six staff acts.
type GhiNhiemVu struct {
	db     *store.DB
	kho    KhoNhiemVuGhi
	deNghi KhoDeNghiLuiHan

	// sinhID is injected so a test can pin every generated id. In production it is ulid.Moi.
	sinhID func() (string, error)

	// nay is the clock every recorded instant comes from — `thoi_diem` on the timeline,
	// `ngay_hoan_thanh`, `deleted_at`, `duyet_luc`.
	//
	// A SEAM AND NOT time.Now() AT THE CALL SITE, for a reason about evidence rather than
	// convenience: `ngay_hoan_thanh` is the instant §11.3's on-time ratio compares against
	// `han_ban_dau`, so a test that cannot pin it cannot assert whether a task was finished on time.
	// In production this is nil and nayHoac returns the real clock.
	nay func() time.Time
}

func NewGhiNhiemVu(db *store.DB, kho KhoNhiemVuGhi, deNghi KhoDeNghiLuiHan) *GhiNhiemVu {
	return &GhiNhiemVu{db: db, kho: kho, deNghi: deNghi, sinhID: ulid.Moi}
}

// nayHoac is the clock, UTC. `TIMESTAMPTZ` stores an instant rather than a wall reading, so the
// location changes nothing that is stored — it is fixed so a value read back in a test compares
// equal without a location dance.
func (uc *GhiNhiemVu) nayHoac() time.Time {
	if uc.nay == nil {
		return time.Now().UTC()
	}
	return uc.nay().UTC()
}

// --- the tree walks (ADR 0037) -----------------------------------------------------------------

// duyetCaCay collects EVERY descendant of a task, at every depth.
//
// # BREADTH-FIRST WITH A VISITED SET, AND THE VISITED SET IS NOT AN OPTIMISATION
//
// ADR 0037 decision 1 made the tree unlimited in depth, which is what makes `A → B → C → A`
// representable at all. A cycle that reached the table — written before the write path existed, by
// an import, or through a defect — would make a naive walk run FOR EVER while holding row locks on
// a government register. The visited set means this function TERMINATES on such data and the
// ceiling means it terminates quickly; the cycle is then refused at the write path so it cannot be
// created in the first place (kiemChuTrinh).
//
// ⚠ IT IS RECURSIVE OVER THE WHOLE TREE AND NOT ONE LEVEL. Migration 0008 spells out why in its own
// words: "DUYỆT ĐỆ QUY CẢ CÂY chứ không chỉ một tầng con". A one-level check would let a parent be
// completed while a GRANDCHILD is still open — and the completion figure that reaches leadership
// would count it.
//
// SOFT-DELETED CHILDREN ARE NOT IN IT, because ConTrucTiep excludes them (rule 7, invariant 2): a
// task removed from the register does not hold its parent open.
func (uc *GhiNhiemVu) duyetCaCay(ctx context.Context, tx *store.ScopedTx, gocID string) (
	[]domain.NhiemVuTomTat, error) {

	var (
		ra      []domain.NhiemVuTomTat
		daTham  = map[string]bool{gocID: true}
		hangCho = []string{gocID}
	)
	for len(hangCho) > 0 {
		cha := hangCho[0]
		hangCho = hangCho[1:]

		con, err := uc.kho.ConTrucTiep(ctx, tx, cha)
		if err != nil {
			return nil, err
		}
		for _, c := range con {
			if daTham[c.ID] {
				// Already seen on this walk: the data holds a cycle. Skipping it is what makes the
				// walk terminate; the ceiling below is what makes a WIDE cycle terminate too.
				continue
			}
			daTham[c.ID] = true
			ra = append(ra, c)
			hangCho = append(hangCho, c.ID)

			if len(ra) > domain.TranDuyetCayNhiemVu {
				return nil, domain.ErrCayNhiemVuQuaLon
			}
		}
	}
	return ra, nil
}

// kiemChuTrinh IS THE FIFTH RULE — the one in no specification, and the one migration 0008 names as
// "luật dễ quên nhất".
//
// # WHAT IT WALKS, AND WHY UPWARD
//
// Setting `nhiem_vu_cha_id = X` on task T closes a cycle exactly when T is already an ANCESTOR of
// X. So the walk starts at X and follows `nhiem_vu_cha_id` upward looking for T. `A → B → C → A` is
// found on the third hop; the schema's `CHECK (nhiem_vu_cha_id IS DISTINCT FROM id)` finds only the
// first, because a CHECK sees one row and can never see the row above it.
//
// IT RUNS INSIDE THE CALLER'S TRANSACTION, on rows another officer cannot move underneath it. A
// check made before the transaction opens decides against a tree that may no longer exist by the
// time the UPDATE lands — and the window is exactly long enough for the other officer's re-parent
// to commit.
//
// ⚠ IT RUNS ON CREATE TOO, where a cycle is impossible by construction — a row that does not exist
// yet has no descendants. That is deliberate and is not wasted work: it is what detects a cycle
// that is ALREADY in the data before a new child is hung off it, and a child attached under an
// existing cycle is a row whose every future completion check would hit ErrCayNhiemVuQuaLon with no
// explanation. One uniform check also means there is no second, laxer path to reach for.
//
// THE VISITED SET IS THE SECOND WALL. Without it, a pre-existing cycle above X would make this loop
// run until the hop ceiling instead of being named; with it the answer is the honest one — this
// parent sits in a cycle, refuse.
func (uc *GhiNhiemVu) kiemChuTrinh(ctx context.Context, tx *store.ScopedTx, id, chaID string) error {
	daTham := map[string]bool{}
	hienTai := chaID

	for i := 0; i < domain.TranBacCayNhiemVu; i++ {
		if hienTai == "" {
			return nil // reached a root: no cycle
		}
		if hienTai == id {
			return domain.ErrChuTrinhCayNhiemVu
		}
		if daTham[hienTai] {
			// The ancestry ALREADY holds a cycle, above the row being edited. Attaching anything to
			// it would make every later tree walk depend on the safety net rather than on the rule.
			return domain.ErrChuTrinhCayNhiemVu
		}
		daTham[hienTai] = true

		tiep, err := uc.kho.ChaCua(ctx, tx, hienTai)
		if err != nil {
			return err
		}
		hienTai = tiep
	}
	return domain.ErrCayNhiemVuQuaLon
}

// nhanCha validates a proposed parent INSIDE the transaction: it must be a live task of this
// commune, and it must not close a cycle.
//
// THE PARENT IS READ `FOR UPDATE`, not merely checked for existence. Between "this parent is live"
// and the INSERT that hangs a child off it, the parent can be soft-deleted — and ADR 0037 decision
// 3 refuses that delete only while children EXIST, so a child created in that window is exactly the
// orphan the decision exists to prevent. Holding the parent's row closes it.
func (uc *GhiNhiemVu) nhanCha(ctx context.Context, tx *store.ScopedTx, id, chaID string) error {
	if _, err := uc.kho.TheoIDDeSua(ctx, tx, chaID); err != nil {
		if errors.Is(err, petstore.ErrNhiemVuKhongTonTai) {
			return domain.ErrChaKhongTonTai
		}
		return err
	}
	return uc.kiemChuTrinh(ctx, tx, id, chaID)
}

// duocHoanThanh is ADR 0037 DECISION 4, on the tree read under the lock.
//
// TWO REFUSALS IN ONE PLACE, AND THEY ARE DIFFERENT KINDS. The permission is about the ACCOUNT and
// answers 403; the unfinished children are about the RECORD and answer 409. Both are checked before
// anything is written, so a refused completion leaves no row, no timeline entry and no audit
// entry — the transaction rolls back with nothing in it.
func (uc *GhiNhiemVu) duocHoanThanh(ctx context.Context, tx *store.ScopedTx, n domain.NhiemVu,
	duyet QuyenDuyetHoanThanh) error {

	if !duyet {
		return ErrKhongDuocDuyetHoanThanh
	}
	con, err := uc.duyetCaCay(ctx, tx, n.ID)
	if err != nil {
		return err
	}
	if chua := domain.ConChuaXong(con); len(chua) > 0 {
		return domain.LoiConChuaXong(chua)
	}
	return nil
}

// --- 1. giao việc mới (§7) ----------------------------------------------------------------------

// YeuCauTaoNhiemVu is "Giao việc mới" as it arrives from the handler.
//
// # THE ABSENCES ARE THE DESIGN
//
// There is no `TrangThai`: a new task starts at `moi-giao` and nowhere else — a client choosing its
// own starting state could file work that is already "hoàn thành". There is no `HanBanDau`: it is
// written from `HanXuLy` by the INSERT, once, and never again. There is no `NguoiTaoMa`: that is
// the acting principal, and a request that could name its own author is a request that can forge
// the trail.
type YeuCauTaoNhiemVu struct {
	// Ma is the register number, and TuSinhMa is §7.1's `Tự sinh mã` checkbox — DEFAULT ON, so the
	// ordinary case mints `NV01, NV02…` and the clerk types nothing.
	Ma       string
	TuSinhMa bool

	Loai      string
	Khoi      string
	TieuDe    string
	MoTa      string
	MucUuTien string

	NguonGiao string
	NguonID   string

	BoPhanID            string
	NguoiThucHienMa     string
	LanhDaoGiaoViecMa   string
	CoQuanChuTriID      string
	ChuyenVienTheoDoiMa string

	// HanXuLy is "Hạn hoàn thành" as the form carried it. ZERO MEANS NO DEADLINE, which §4.1 renders
	// as `Hạn —` and §7.1 permits by not marking the field required.
	//
	// ⚠ IT IS STORED, NOT DERIVED, AND IT IS FIXED HERE FOR EVER. `han_ban_dau` takes the same value
	// in the same statement and the trigger refuses every later change to it, so a task created
	// without a deadline can never be given one — see the report, where that consequence is raised
	// rather than worked around.
	HanXuLy time.Time

	// NhiemVuChaID makes this a sub-task of §5.10. EMPTY IS A ROOT TASK.
	//
	// ⚠ ADR 0037 DECISION 2 IS ENFORCED BY THIS STRUCT HAVING NOTHING TO DO WITH THE PARENT'S
	// DEADLINE. A child's deadline is whatever the form carried in HanXuLy above — there is no line
	// anywhere in this file that reads the parent's `han_xu_ly`, and that absence IS the rule.
	NhiemVuChaID string

	// VanBan is §7.2's three dynamic lists, on the create form. OPTIONAL, and empty is the ordinary
	// case: §7.3 removes the whole block for a `co-ban` task, and even a `theo-van-ban` task is
	// often filed with nothing in it.
	//
	// ⚠ AN ITEM CARRYING AN `ID` IS REFUSED HERE. The task does not exist yet, so it has no lines,
	// and a client naming a line id on a create is either confused or choosing internal ids. The
	// refusal is not written out as a branch: domain.SoSanhVanBan against an EMPTY stored block
	// produces exactly it, which is why create and edit share one rule rather than two that can
	// disagree.
	VanBan []domain.VanBanNhiemVuVao
}

// Tao books one task. Permission: `task.create`.
//
// # THE NUMBER IS MINTED INSIDE THE TRANSACTION
//
// §7.1 mints `NV01, NV02…` PER COMMUNE, and migration 0006 explains why there is no sequence: a
// PostgreSQL sequence is per-table, so one would hand commune B the number after commune A's and
// the second commune's register would start at NV58. The high-water mark is read and the row is
// inserted in one transaction, against `UNIQUE (tenant_id, ma)` — which is what actually guarantees
// it. See petstore.SoLonNhatDaCap for what that does and does not buy under concurrency.
func (uc *GhiNhiemVu) Tao(ctx context.Context, yc YeuCauTaoNhiemVu, nguoi audit.Actor) (
	domain.NhiemVu, error) {
	return uc.TaoTuNguon(ctx, yc, nguoi, nil)
}

// KiemNguonTrongGiaoDich is a precondition on the SOURCE RECORD of a task, run INSIDE the transaction
// that books the task, before anything is written. A non-nil error refuses the whole act: no row, no
// timeline entry, no audit entry.
type KiemNguonTrongGiaoDich func(ctx context.Context, tx *store.ScopedTx) error

// TaoTuNguon is Tao with a source check inside the SAME transaction — the closure the meeting
// register's split needed once conclusions could be removed or marked "không phát sinh nhiệm vụ"
// (user decisions 25/09/2026). A check made before this transaction opened would decide against a
// conclusion that another clerk can delete or mark before the INSERT lands; running it here, under
// the lock it takes, serialises the two acts on the meeting row (app.kiemNguonKetLuan).
//
// ONE CREATE PATH STILL: Tao is this with no check, so the number, the tree rules, the deadline, the
// timeline row and the audit entry have exactly one implementation.
func (uc *GhiNhiemVu) TaoTuNguon(ctx context.Context, yc YeuCauTaoNhiemVu, nguoi audit.Actor,
	kiemNguon KiemNguonTrongGiaoDich) (domain.NhiemVu, error) {

	// A CONCLUSION SOURCE WITH NO SOURCE CHECK IS REFUSED. The HTTP handler of POST /api/v1/tasks
	// refuses it first; this is the same rule for any other caller of Tao, because the conclusion's
	// existence, its removal and its "không phát sinh" mark are checked ONLY by the split's closure
	// (kiemNguonKetLuan) — a `ket-luan-hop` task booked without it points at whatever id it was sent.
	if kiemNguon == nil && domain.NguonGiao(yc.NguonGiao) == domain.NguonKetLuanHop {
		return domain.NhiemVu{}, domain.ErrNguonKetLuanPhaiTach
	}

	moi, err := chuanHoaTaoNhiemVu(yc)
	if err != nil {
		return domain.NhiemVu{}, err
	}
	// THE DOCUMENT BLOCK IS CHECKED AND DECIDED BEFORE THE TRANSACTION OPENS, like every other field:
	// a rejected form must hold no row lock on a government register, and must not have written a row
	// that only a rollback takes back.
	//
	// THE DIFF IS THE SAME RULE THE EDIT PATH USES, against an EMPTY stored block — which is what
	// makes an item carrying a line id a refusal here without a branch saying so. It needs no
	// transaction precisely because a task that does not exist yet has nothing to compare against.
	vanBanVao, err := domain.KiemDanhSachVanBanNhiemVu(yc.VanBan)
	if err != nil {
		return domain.NhiemVu{}, err
	}
	thayDoiVB, err := domain.SoSanhVanBan(nil, vanBanVao)
	if err != nil {
		return domain.NhiemVu{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.NhiemVu{}, err
	}
	moi.NguoiTaoMa = nguoi.ID

	id, err := uc.sinhID()
	if err != nil {
		return domain.NhiemVu{}, fmt.Errorf("nhiem_vu: sinh mã nội bộ: %w", err)
	}
	moi.ID = id

	bayGio := uc.nayHoac()
	moi.TaoLuc = bayGio

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if kiemNguon != nil {
			// FIRST, before any lock of this register is taken: the source check locks the SOURCE's
			// row (a meeting), and taking it before the task tree's rows keeps one lock order.
			if err := kiemNguon(ctx, tx); err != nil {
				return err
			}
		}
		if moi.NhiemVuChaID != "" {
			// THE PARENT IS VALIDATED FIRST AND UNDER THE LOCK. A cycle cannot be closed by a row
			// that does not exist yet, and the check runs anyway — see kiemChuTrinh.
			if err := uc.nhanCha(ctx, tx, moi.ID, moi.NhiemVuChaID); err != nil {
				return err
			}
		}

		if yc.TuSinhMa {
			so, err := uc.kho.SoLonNhatDaCap(ctx, tx)
			if err != nil {
				return err
			}
			moi.Ma = domain.MaNhiemVuTiepTheo(so)
		} else {
			// THE UNIQUE KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE. It counts soft-deleted
			// rows, because an issued number is never reissued (rule 7, invariant 3).
			daDung, err := uc.kho.MaDaDung(ctx, tx, moi.Ma)
			if err != nil {
				return err
			}
			if daDung {
				return petstore.ErrMaNhiemVuDaTonTai
			}
		}

		if err := uc.kho.Tao(ctx, tx, moi); err != nil {
			return err
		}

		// §5.4'S DOCUMENT BLOCK, IN THE SAME TRANSACTION AS THE TASK IT BELONGS TO. A line is a
		// field value of the row above, so a task committed without its lines would be a record
		// missing part of what the clerk typed — and there is no second act that would put them
		// back.
		//
		// WHAT IS WRITTEN WAS DECIDED BEFORE THE TRANSACTION OPENED (see the top of this function);
		// `Them` is every item, `Sua` and `Xoa` are empty by construction.
		if err := uc.apDungVanBan(ctx, tx, moi.ID, thayDoiVB, nguoi.ID, bayGio); err != nil {
			return err
		}
		var err error
		if moi.VanBan, err = uc.kho.VanBanCuaNhiemVuDeSua(ctx, tx, moi.ID); err != nil {
			return err
		}

		// THE FIRST TIMELINE ROW, IN THE SAME TRANSACTION. §5.9's drawer renders "Chưa có ghi chép
		// nào." for an empty log, and a task that was given to somebody with no entry saying so
		// would render exactly that on the day it was created.
		if err := uc.ghiNhatKy(ctx, tx, moi, bayGio, nguoi.ID, "Giao việc mới: "+moi.Ma); err != nil {
			return err
		}

		// THE DELTA HAS NO "truoc", because there was nothing before. It names what was committed
		// to — the deadline above all, since that is the figure §11.3 measures against for ever.
		delta, err := json.Marshal(map[string]any{
			"sau": map[string]any{
				"ma":         moi.Ma,
				"loai":       moi.Loai,
				"trang_thai": string(moi.TrangThai),
				"nguon_giao": string(moi.NguonGiao),
				// THE RECORD THE WORK CAME FROM, and it is here because a task can now be born from a
				// MEETING CONCLUSION (§3 of chapter 04). `nguon_giao` alone says "from a meeting
				// conclusion"; without `nguon_id` the entry cannot say WHICH, and that link is the whole
				// reason the split route exists ("giữ liên kết ngược về kết luận gốc để truy vết được về
				// sau"). It is EMPTY for a directly-assigned task, which is most of them.
				"nguon_id":              moi.NguonID,
				"han_xu_ly":             lucRaVet(moi.HanXuLy),
				"han_ban_dau":           lucRaVet(moi.HanXuLy),
				"lanh_dao_giao_viec_ma": moi.LanhDaoGiaoViecMa,
				"nhiem_vu_cha_id":       moi.NhiemVuChaID,
			},
			"ma_tu_sinh": yc.TuSinhMa,
			// HOW MANY DOCUMENT LINES WERE FILED, AND NOT WHAT THEY SAY. `trich_yeu` is the subject
			// line of an administrative document and routinely names a citizen's case; `audit_log` is
			// append-only and never deleted (rule 6, invariant 4), so a copy there would be a second
			// permanent store of that text (rule 3, forbidden #5). The count is what an inspection
			// can act on — it says the block was filled in at creation — and the text itself lives on
			// rows the archival trigger already protects.
			"so_dong_van_ban": len(thayDoiVB.Them),
		})
		if err != nil {
			return fmt.Errorf("nhiem_vu: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViTaoNhiemVu,
			Subject: moi.Ma,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.NhiemVu{}, bocNhiemVu(ctx, "giao việc mới", err)
	}
	return moi, nil
}

// chuanHoaTaoNhiemVu validates and trims the request. IT RUNS BEFORE THE TRANSACTION OPENS: a
// rejected form must hold no row lock on a government register.
func chuanHoaTaoNhiemVu(yc YeuCauTaoNhiemVu) (domain.NhiemVu, error) {
	var moi domain.NhiemVu

	tieuDe, err := domain.KiemTieuDeNhiemVu(yc.TieuDe)
	if err != nil {
		return moi, err
	}
	moTa, err := domain.KiemVanBanTuyChon(yc.MoTa, domain.MoTaNhiemVuToiDa, domain.ErrMoTaNhiemVuQuaDai)
	if err != nil {
		return moi, err
	}

	// `loai` IS MANDATORY AND `muc_uu_tien` IS NOT — the schema says the same thing (NOT NULL versus
	// nullable), and §7.1 marks the type ✔ and the priority merely defaulted.
	//
	// ⚠ NEITHER IS CHECKED AGAINST THE COMMUNE'S CATALOGUE HERE, and that is not an omission: both
	// carry a REAL FOREIGN KEY into this service's own tables (migration 0006), so a code no commune
	// configured is refused by the database inside this transaction. What that costs is stated
	// rather than discovered — the catalogues are EMPTY in every commune until somebody fills them
	// (0003 seeds nothing), so NO TASK CAN BE CREATED until `loai_nhiem_vu` has rows, and the
	// refusal arrives as a constraint error rather than as a sentence naming the catalogue. It is
	// reported as a finding; a catalogue read invented here would be a second source for a list the
	// database already owns.
	if yc.Loai == "" {
		return moi, domain.ErrThieuLoaiNhiemVu
	}

	nguon := domain.NguonGiao(yc.NguonGiao)
	if yc.NguonGiao == "" {
		nguon = domain.NguonTrucTiep
	}
	if !nguon.HopLe() {
		return moi, domain.ErrNguonGiaoKhongHopLe
	}
	// `nhiem_vu_truc_tiep_khong_co_nguon` refuses a directly-assigned task that points at a record.
	// Dropping the id here turns a constraint error into the request the clerk actually made.
	nguonID := yc.NguonID
	if nguon == domain.NguonTrucTiep {
		nguonID = ""
	}

	ma := ""
	if !yc.TuSinhMa {
		if ma, err = domain.KiemMaNhiemVu(yc.Ma); err != nil {
			return moi, err
		}
	}

	moi = domain.NhiemVu{
		Ma:     ma,
		Loai:   yc.Loai,
		Khoi:   yc.Khoi,
		TieuDe: tieuDe,
		MoTa:   moTa,
		// A NEW TASK STARTS AT `moi-giao` AND NOWHERE ELSE. §6's Kanban calls that column "Chưa thực
		// hiện", and it is the only state §6 draws arrows out of and none into.
		TrangThai:           domain.MoiGiao,
		MucUuTien:           yc.MucUuTien,
		NguonGiao:           nguon,
		NguonID:             nguonID,
		BoPhanID:            yc.BoPhanID,
		NguoiThucHienMa:     yc.NguoiThucHienMa,
		LanhDaoGiaoViecMa:   yc.LanhDaoGiaoViecMa,
		CoQuanChuTriID:      yc.CoQuanChuTriID,
		ChuyenVienTheoDoiMa: yc.ChuyenVienTheoDoiMa,
		// ONE VALUE FOR BOTH CLOCKS, and the store writes them from this single field. ADR 0037
		// decision 2 lives in what is NOT here: nothing reads the parent's deadline.
		HanXuLy:      yc.HanXuLy,
		HanBanDau:    yc.HanXuLy,
		NhiemVuChaID: yc.NhiemVuChaID,
	}
	return moi, nil
}

// --- 2. edit (§5.4's ✎ Sửa) -----------------------------------------------------------------------

// Sua changes the descriptive fields of a task. Permission: `task.update`.
//
// WHAT IT MAY AND MAY NOT MOVE IS petstore.SuaNhiemVu's list, and the reasoning for each absence is
// there. The two that matter most: it cannot move `han_xu_ly` (that is the extension flow, decided
// by the leader named on the record) and it cannot rewrite `lanh_dao_giao_viec_ma` (that column IS
// the approver under ADR 0038, so a `task.update` holder able to rewrite it could name themselves
// the approver of their own extension requests).
//
// NOTHING IS WRITTEN WHEN NOTHING CHANGED — no UPDATE and no audit entry. That is what makes the
// route's `idem.KhongCan` declaration a property rather than a hope.
func (uc *GhiNhiemVu) Sua(ctx context.Context, ma string, sua petstore.SuaNhiemVu,
	nguoi audit.Actor) (domain.NhiemVu, error) {

	if err := chuanHoaSuaNhiemVu(&sua); err != nil {
		return domain.NhiemVu{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.NhiemVu{}, err
	}

	// ONE INSTANT FOR THE WHOLE ACT, read before the transaction opens. `deleted_at` on a removed
	// document line comes from it, and a clock read per statement would stamp the lines of one Save
	// with different instants — which reads, years later, as several edits rather than one.
	bayGio := uc.nayHoac()

	var sau domain.NhiemVu

	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoMaDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}

		// §5.4'S DOCUMENT BLOCK, READ AND DIFFED UNDER THE TASK'S LOCK.
		//
		// BOTH HALVES HAVE TO HAPPEN INSIDE THE TRANSACTION, and for different reasons. The READ
		// must, because the diff decides which lines are REMOVED and a block read earlier is a block
		// another officer can have changed since. The DIFF must, because it is what turns "the
		// client sent these three lines" into three statements — and a decision made against rows
		// nobody holds is a decision about a state that no longer exists.
		//
		// IT IS READ EVEN WHEN THE REQUEST DID NOT MENTION THE BLOCK, and the reason is the RESPONSE
		// rather than the diff: PATCH answers with the detail shape, and that shape always carries
		// `documents` as an array. Reading it only when the client sent it would make a PATCH of the
		// title answer with `documents: null` — which a client cannot tell from "this task has no
		// lines" without knowing what it happened to send.
		vanBanTruoc, err := uc.kho.VanBanCuaNhiemVuDeSua(ctx, tx, truoc.ID)
		if err != nil {
			return err
		}
		var thayDoiVB domain.ThayDoiVanBan
		if sua.VanBan != nil {
			if thayDoiVB, err = domain.SoSanhVanBan(vanBanTruoc, *sua.VanBan); err != nil {
				return err
			}
		}

		if !sua.CoGiDoi(truoc) && !thayDoiVB.CoGiDoi() {
			// NOTHING MOVED — neither a column of the task nor a line of its block. Writing an UPDATE
			// and an audit entry here would put a row in an append-only ledger saying an act happened
			// that changed nothing, and an inspection counting acts would count it.
			//
			// THE BLOCK IS STILL RETURNED, so the caller's response is the same shape whether or not
			// anything moved. A detail response that dropped `documents` on a no-op save would look
			// to a client exactly like a task whose lines had just been deleted.
			sau = truoc
			sau.VanBan = vanBanTruoc
			return nil
		}

		sau = sua.Apdung(truoc)

		if sau.NhiemVuChaID != truoc.NhiemVuChaID && sau.NhiemVuChaID != "" {
			// THE ONE PATH THAT CAN CLOSE A CYCLE. A task created under a parent has no descendants
			// to loop back through; MOVING an existing task under one of its own descendants is how
			// `A → B → C → A` is built, and this is where it is refused.
			if err := uc.nhanCha(ctx, tx, truoc.ID, sau.NhiemVuChaID); err != nil {
				return err
			}
		}

		if err := uc.kho.Sua(ctx, tx, truoc.ID, sau); err != nil {
			return err
		}

		// THE BLOCK IS APPLIED IN THE SAME TRANSACTION AS THE COLUMNS. §5.4 draws one `✎ Sửa` over
		// the whole block, so one Save is ONE administrative act: a commit that moved the title but
		// dropped the document lines would be half an act, with the audit entry claiming both.
		if err := uc.apDungVanBan(ctx, tx, truoc.ID, thayDoiVB, nguoi.ID, bayGio); err != nil {
			return err
		}
		if sau.VanBan, err = uc.kho.VanBanCuaNhiemVuDeSua(ctx, tx, truoc.ID); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{
			"truoc": map[string]any{
				"tieu_de":         truoc.TieuDe,
				"khoi":            truoc.Khoi,
				"muc_uu_tien":     truoc.MucUuTien,
				"tien_do":         truoc.TienDo,
				"nhiem_vu_cha_id": truoc.NhiemVuChaID,
				"so_dong_van_ban": len(vanBanTruoc),
			},
			"sau": map[string]any{
				"tieu_de":         sau.TieuDe,
				"khoi":            sau.Khoi,
				"muc_uu_tien":     sau.MucUuTien,
				"tien_do":         sau.TienDo,
				"nhiem_vu_cha_id": sau.NhiemVuChaID,
				"so_dong_van_ban": len(sau.VanBan),
			},
			// WHAT HAPPENED TO THE BLOCK, IN COUNTS AND IDS — NEVER IN TEXT. `trich_yeu` is the
			// subject line of an administrative document and routinely names a citizen's case, and
			// `audit_log` is append-only and never deleted (rule 6, invariant 4), so quoting it here
			// would be a second permanent store of that text (rule 3, forbidden #5).
			//
			// THE REMOVED LINES ARE NAMED BY ID AND THE OTHERS ARE NOT, and the asymmetry is what an
			// inspection actually needs: an added or edited line is still on the record and can be
			// read there, while a removed one has left every screen — the id is the only way back to
			// the row, which is still in the table carrying `deleted_at` and `deleted_by`.
			"van_ban": map[string]any{
				"them":   len(thayDoiVB.Them),
				"sua":    len(thayDoiVB.Sua),
				"xoa":    len(thayDoiVB.Xoa),
				"xoa_id": idVanBanDaGo(thayDoiVB.Xoa),
			},
		})
		if err != nil {
			return fmt.Errorf("nhiem_vu: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViSuaNhiemVu,
			Subject: truoc.Ma,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.NhiemVu{}, bocNhiemVu(ctx, "sửa nhiệm vụ", err)
	}
	return sau, nil
}

// chuanHoaSuaNhiemVu trims and bounds the fields that were actually sent. A nil pointer is "not
// mentioned" and is left alone — see petstore.SuaNhiemVu.
func chuanHoaSuaNhiemVu(sua *petstore.SuaNhiemVu) error {
	if sua.TieuDe != nil {
		s, err := domain.KiemTieuDeNhiemVu(*sua.TieuDe)
		if err != nil {
			return err
		}
		sua.TieuDe = &s
	}
	for _, ca := range []struct {
		truong **string
		tran   int
		loi    error
	}{
		{&sua.MoTa, domain.MoTaNhiemVuToiDa, domain.ErrMoTaNhiemVuQuaDai},
		{&sua.TomTatKetQua, domain.TomTatKetQuaToiDa, domain.ErrTomTatKetQuaQuaDai},
		{&sua.GhiChu, domain.GhiChuNhiemVuToiDa, domain.ErrGhiChuNhiemVuQuaDai},
	} {
		if *ca.truong == nil {
			continue
		}
		s, err := domain.KiemVanBanTuyChon(**ca.truong, ca.tran, ca.loi)
		if err != nil {
			return err
		}
		*ca.truong = &s
	}
	if sua.TienDo != nil {
		if err := domain.KiemTienDo(*sua.TienDo); err != nil {
			return err
		}
	}
	return nil
}

// --- 3. moving along the lifecycle (§6) --------------------------------------------------------

// YeuCauDoiTrangThai is one lifecycle move.
//
// THE TARGET IS A PARAMETER HERE AND IS NOT ONE ON THE PETITION PATH, and the difference is the
// shape of the two lifecycles. A petition has ONE main flow, so "advance" names the next step
// unambiguously and a target on the wire would let a client skip steps. §6's task lifecycle
// BRANCHES at every state — `tam-dung` and `chuyen-tiep` leave all four working states — so there
// is no single "next", and the server's job is to refuse the moves the diagram does not draw rather
// than to choose among three.
type YeuCauDoiTrangThai struct {
	TrangThai string

	// GhiChu is the officer's own line for the timeline. OPTIONAL: when it is empty the entry
	// carries a generated sentence naming the two statuses, because the timeline may not have a gap
	// and the schema refuses an empty entry — see domain.NoiDungChuyenTrangThai.
	GhiChu string
}

// DoiTrangThai moves the task. Route permission: `task.update`; the step INTO `hoan-thanh`
// additionally needs `task.approve` and an entirely finished sub-tree.
//
// # THE THREE CHECKS, IN THIS ORDER, INSIDE THE TRANSACTION
//
//  1. §6's SHAPE — is this move on the diagram at all, and for a paused task, is it the resume the
//     timeline supports.
//  2. THE PERMISSION for the final step (`task.approve`).
//  3. ADR 0037 DECISION 4 — the whole sub-tree, recursively.
//
// THE ORDER IS DELIBERATE. 2 before 3 keeps a caller who may not complete the task from learning,
// from the error message, which of its sub-tasks are still open — routing information about work
// they were just refused.
func (uc *GhiNhiemVu) DoiTrangThai(ctx context.Context, ma string, yc YeuCauDoiTrangThai,
	nguoi audit.Actor, duyet QuyenDuyetHoanThanh) (domain.NhiemVu, error) {

	moiTT := domain.TrangThaiNhiemVu(yc.TrangThai)
	if !moiTT.HopLe() {
		return domain.NhiemVu{}, domain.ErrTrangThaiNhiemVuKhongBiet
	}
	ghiChu, err := domain.KiemVanBanTuyChon(yc.GhiChu, domain.NoiDungNhatKyToiDa,
		domain.ErrNoiDungNhatKyQuaDai)
	if err != nil {
		return domain.NhiemVu{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.NhiemVu{}, err
	}

	bayGio := uc.nayHoac()
	var sau domain.NhiemVu

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoMaDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}

		// §6's "(trạng thái trước)" — DERIVED FROM THE TIMELINE, never from a column. Read only when
		// the task is actually paused: on every other path it is not a question, and asking anyway
		// would put a query on the common path to answer something nobody used.
		truocTamDung := domain.TrangThaiNhiemVu("")
		if truoc.TrangThai == domain.TamDung {
			nhatKy, err := uc.kho.NhatKyGanNhat(ctx, tx, truoc.ID, petstore.TranDocNhatKy)
			if err != nil {
				return err
			}
			if tt, co := domain.TrangThaiTruocTamDung(nhatKy); co {
				truocTamDung = tt
			}
		}

		if err := domain.ChuyenTrangThaiDuoc(truoc.TrangThai, moiTT, truocTamDung); err != nil {
			return err
		}

		// `ngay_hoan_thanh` IS SET ON EXACTLY ONE STEP. The schema ties it to the status with a
		// biconditional, and §11.3 measures the on-time ratio from it — stamping it on any other
		// step would put a task in the numerator that nobody finished.
		var xongLuc time.Time
		if moiTT == domain.HoanThanh {
			if err := uc.duocHoanThanh(ctx, tx, truoc, duyet); err != nil {
				return err
			}
			xongLuc = bayGio
		}

		if err := uc.kho.DoiTrangThai(ctx, tx, truoc.ID, truoc.TrangThai, moiTT, xongLuc); err != nil {
			return err
		}

		sau = truoc
		sau.TrangThai = moiTT
		sau.NgayHoanThanh = xongLuc

		noiDung := ghiChu
		if noiDung == "" {
			noiDung = domain.NoiDungChuyenTrangThai(truoc.TrangThai, moiTT)
		}
		if err := uc.ghiNhatKy(ctx, tx, sau, bayGio, nguoi.ID, noiDung); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{
			"truoc": map[string]any{"trang_thai": string(truoc.TrangThai)},
			"sau":   map[string]any{"trang_thai": string(moiTT)},
			// DERIVED AT THE INSTANT OF THE ACT AND RECORDED, never stored as a column (rule 10,
			// invariant 3). An entry states what was true at one moment; a column would be read as
			// the current truth later, which is the defect that rule forbids.
			"tre_han": sau.TreHan(bayGio),
		})
		if err != nil {
			return fmt.Errorf("nhiem_vu: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViChuyenTrangNhiemVu,
			Subject: truoc.Ma,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.NhiemVu{}, bocNhiemVu(ctx, "chuyển trạng thái", err)
	}
	return sau, nil
}

// --- 4. soft delete (ADR 0037 decision 3) ---------------------------------------------------------

// Xoa removes the task from every read path and keeps the record. Permission: `task.delete`.
//
// # IT REFUSES WHILE CHILDREN LIVE, AND SAYS HOW MANY
//
// ADR 0037 decision 3 chose refusal over the two alternatives and wrote down what each costs:
// cascading would remove dozens of rows on one click of a register that cannot be hard-deleted, and
// detaching the children would keep every row while losing the one thing the tree exists to hold —
// where this work came from. Refusal is the only option that never produces an orphan pointing at a
// parent that has vanished from every list.
//
// ONE LEVEL IS ENOUGH HERE, unlike decision 4. If the direct children are gone, the grandchildren
// went with them — because deleting each child was refused by this same rule until ITS children
// were gone. The recursion is in the history of the deletions rather than in this call.
func (uc *GhiNhiemVu) Xoa(ctx context.Context, ma, lyDoTho string, nguoi audit.Actor) error {
	lyDo, err := domain.KiemLyDoXoaNhiemVu(lyDoTho)
	if err != nil {
		return err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return err
	}

	bayGio := uc.nayHoac()

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.kho.TheoMaDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}

		con, err := uc.kho.ConTrucTiep(ctx, tx, n.ID)
		if err != nil {
			return err
		}
		if len(con) > 0 {
			return domain.LoiConChuaXoa(len(con))
		}

		// `deleted_by` IS THE STAFF BUSINESS CODE (rule 6, invariant 8). It is read years later by
		// somebody handling a complaint, and a ULID there names nobody.
		if err := uc.kho.XoaMem(ctx, tx, n.ID, nguoi.ID, lyDo, bayGio); err != nil {
			return err
		}

		// THE REASON TEXT IS NOT IN THE DELTA, AND ITS LENGTH IS. It is free text somebody typed
		// about a government record; `audit_log` is append-only and never deleted (rule 6, invariant
		// 4), so a copy there is a second permanent store of it. The reason itself lives in
		// `delete_reason` on a row the archival trigger already protects.
		delta, err := json.Marshal(map[string]any{
			"truoc":        map[string]any{"trang_thai": string(n.TrangThai), "da_xoa": false},
			"sau":          map[string]any{"trang_thai": string(n.TrangThai), "da_xoa": true},
			"do_dai_ly_do": len([]rune(lyDo)),
			"xoa_luc":      lucRaVet(bayGio),
		})
		if err != nil {
			return fmt.Errorf("nhiem_vu: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViXoaNhiemVu,
			Subject: n.Ma,
			Delta:   delta,
		})
	})
	if err != nil {
		return bocNhiemVu(ctx, "xoá nhiệm vụ", err)
	}
	return nil
}

// --- 5. asking for more time (§5.8) ----------------------------------------------------------------

// YeuCauDeNghiLuiHan is the "Đề nghị lùi hạn" block: a new date and a reason.
type YeuCauDeNghiLuiHan struct {
	HanMoi time.Time
	LyDo   string
}

// DeNghiLuiHan files a request. Permission: `task.update`.
//
// `task.update` AND NOT `task.extend`, AND THE DIFFERENCE IS ADR 0038'S WHOLE POINT. `task.extend`
// is labelled "Duyệt gia hạn" in the `quyen` table (service-identity/migrations/0001_init.sql:303) —
// it is the right to DECIDE. Guarding the filing route with it would mean only people who can
// approve an extension can ask for one, which is the opposite of §5.8: the box sits on the drawer
// of the officer doing the work.
//
// NOTHING ABOUT THE DEADLINE IS DERIVED. The requester types a date; approving it copies that date
// into `han_xu_ly` and nothing else.
func (uc *GhiNhiemVu) DeNghiLuiHan(ctx context.Context, ma string, yc YeuCauDeNghiLuiHan,
	nguoi audit.Actor) (domain.DeNghiLuiHan, error) {

	lyDo, err := domain.KiemLyDoLuiHan(yc.LyDo)
	if err != nil {
		return domain.DeNghiLuiHan{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.DeNghiLuiHan{}, err
	}

	id, err := uc.sinhID()
	if err != nil {
		return domain.DeNghiLuiHan{}, fmt.Errorf("de_nghi_lui_han: sinh mã nội bộ: %w", err)
	}

	bayGio := uc.nayHoac()
	var dn domain.DeNghiLuiHan

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.kho.TheoMaDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		if err := domain.DuocDeNghiLuiHan(n, yc.HanMoi); err != nil {
			return err
		}

		// THE UNIQUE KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE. Two pending requests
		// carrying two different dates is a leader with two buttons and no way to tell what
		// approving either one means.
		dangCho, err := uc.deNghi.DangChoDuyet(ctx, tx, n.ID)
		if err != nil {
			return err
		}
		if dangCho {
			return domain.ErrDaCoDeNghiChoDuyet
		}

		dn = domain.DeNghiLuiHan{
			ID:        id,
			NhiemVuID: n.ID,
			// THE REQUESTER IS THE ACTING PRINCIPAL'S BUSINESS CODE, never a field of the request. A
			// request that could name its own author is a request that can put somebody else's name
			// on an act (rule 6, invariant 8).
			NguoiDeNghiMa: nguoi.ID,
			HanMoi:        yc.HanMoi,
			LyDo:          lyDo,
			TrangThai:     domain.ChoDuyetLuiHan,
			ThoiDiem:      bayGio,
		}
		if err := uc.deNghi.Tao(ctx, tx, dn); err != nil {
			return err
		}

		// THE REASON TEXT IS NOT IN THE DELTA, ITS LENGTH IS — see Xoa. What the entry does carry is
		// the pair of dates, because "what was asked for, against what stood" is the question an
		// inspection asks when a deadline has slipped.
		delta, err := json.Marshal(map[string]any{
			"han_hien_tai": lucRaVet(n.HanXuLy),
			"han_de_nghi":  lucRaVet(yc.HanMoi),
			"do_dai_ly_do": len([]rune(lyDo)),
			"gui_toi":      n.LanhDaoGiaoViecMa,
		})
		if err != nil {
			return fmt.Errorf("de_nghi_lui_han: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViDeNghiLuiHan,
			Subject: n.Ma,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.DeNghiLuiHan{}, bocNhiemVu(ctx, "đề nghị lùi hạn", err)
	}
	return dn, nil
}

// --- 6. deciding it (ADR 0038) ---------------------------------------------------------------------

// YeuCauQuyetDinhLuiHan is the leader's answer. ONE BOOLEAN AND A NOTE.
//
// `Duyet` IS NOT A STATUS STRING, deliberately: a client sending `trang_thai` could send
// `cho-duyet` and file a "decision" that decides nothing, or invent a fourth code. Two outcomes
// exist and the wire carries exactly two.
type YeuCauQuyetDinhLuiHan struct {
	Duyet  bool
	GhiChu string
}

// QuyetDinhLuiHan approves or rejects a request. Route permission: `task.extend`; the second layer
// is ADR 0038's, below.
//
// # THE TWO LAYERS, AND THE ORDER THEY RUN IN
//
//	task.extend                        the GATE, in routes.go. An account with nothing to do with
//	                                   extensions is refused before this function is reached.
//	Ma == n.LanhDaoGiaoViecMa          HERE, on the task row read under the lock, by
//	                                   domain.DuocDuyetLuiHan.
//
// `nguoi.ID` HOLDS A STAFF BUSINESS CODE AND NOT AN INTERNAL id — internal/http.nguoiThucHien
// builds the actor from `authz.Principal.Ma` for exactly this reason (rule 6, invariant 8). That is
// what makes the comparison against `lanh_dao_giao_viec_ma` a comparison of two values from ONE
// vocabulary. An internal id here would never match, every leader would fall into the "not the
// named leader" branch, and nothing would turn red.
//
// # BOTH ROWS ARE READ UNDER THE LOCK, AND THE TASK IS READ FIRST
//
// The rule is about the TASK's leader and the REQUEST's author, so both rows have to be held: a
// re-assigned or already-decided row read before the transaction is a row the decision was not made
// against.
func (uc *GhiNhiemVu) QuyetDinhLuiHan(ctx context.Context, ma, deNghiID string,
	yc YeuCauQuyetDinhLuiHan, nguoi audit.Actor) (domain.DeNghiLuiHan, error) {

	ghiChu, err := domain.KiemVanBanTuyChon(yc.GhiChu, domain.NoiDungNhatKyToiDa,
		domain.ErrNoiDungNhatKyQuaDai)
	if err != nil {
		return domain.DeNghiLuiHan{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.DeNghiLuiHan{}, err
	}

	bayGio := uc.nayHoac()
	var sau domain.DeNghiLuiHan

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.kho.TheoMaDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		dn, err := uc.deNghi.TheoIDDeSua(ctx, tx, n.ID, deNghiID)
		if err != nil {
			return err
		}

		// ADR 0038, BOTH RULES, ON THE TWO LOCKED ROWS.
		if err := domain.DuocDuyetLuiHan(n, dn, nguoi.ID); err != nil {
			return err
		}

		sangTT := domain.TuChoiLuiHan
		if yc.Duyet {
			sangTT = domain.DaDuyetLuiHan
		}

		if yc.Duyet {
			// RE-CHECKED AGAINST THE DEADLINE AS IT STANDS NOW. Between filing and approval another
			// extension can have been approved, and a request that was a postponement when it was
			// written may no longer be one — approving it then would pull a commitment FORWARD
			// through a screen labelled "lùi hạn".
			if err := domain.DuocDeNghiLuiHan(n, dn.HanMoi); err != nil {
				return err
			}
			// `han_ban_dau` IS NOT TOUCHED — the store names one column, and the trigger refuses the
			// other. §5.8 promises exactly this on the screen: "Hạn gốc vẫn được giữ lại để báo cáo
			// đúng hạn không bị lùi theo".
			if err := uc.kho.DoiHanXuLy(ctx, tx, n.ID, n.HanXuLy, dn.HanMoi); err != nil {
				return err
			}
		}

		if err := uc.deNghi.QuyetDinh(ctx, tx, dn.ID, sangTT, nguoi.ID, bayGio); err != nil {
			return err
		}

		sau = dn
		sau.TrangThai = sangTT
		sau.NguoiDuyetMa = nguoi.ID
		sau.DuyetLuc = bayGio

		// THE TIMELINE RECORDS IT TOO, because §5.9's drawer is where an officer finds out their
		// request was answered. The task's status did not move, so the entry carries the status it
		// is still in.
		noiDung := ghiChu
		if noiDung == "" {
			noiDung = "Từ chối đề nghị lùi hạn."
			if yc.Duyet {
				noiDung = "Duyệt đề nghị lùi hạn."
			}
		}
		if err := uc.ghiNhatKy(ctx, tx, n, bayGio, nguoi.ID, noiDung); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{
			"truoc": map[string]any{
				"trang_thai_de_nghi": string(dn.TrangThai),
				"han_xu_ly":          lucRaVet(n.HanXuLy),
			},
			"sau": map[string]any{
				"trang_thai_de_nghi": string(sangTT),
				"han_xu_ly":          lucRaVet(hanSauQuyetDinh(n, dn, yc.Duyet)),
			},
			// THE ORIGINAL COMMITMENT, RESTATED IN THE ENTRY THAT MOVED THE CURRENT ONE. It is the
			// denominator of §11.3, and an inspection reading this entry should not have to open the
			// row to see that it did not move.
			"han_ban_dau_khong_doi": lucRaVet(n.HanBanDau),
			"nguoi_de_nghi_ma":      dn.NguoiDeNghiMa,
		})
		if err != nil {
			return fmt.Errorf("de_nghi_lui_han: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViQuyetDinhLuiHan,
			Subject: n.Ma,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.DeNghiLuiHan{}, bocNhiemVu(ctx, "quyết định lùi hạn", err)
	}
	return sau, nil
}

// hanSauQuyetDinh is the deadline the task carries once the decision lands — the requested one when
// it was approved, the unchanged one when it was not.
func hanSauQuyetDinh(n domain.NhiemVu, dn domain.DeNghiLuiHan, duyet bool) time.Time {
	if duyet {
		return dn.HanMoi
	}
	return n.HanXuLy
}

// --- §5.4's document block ------------------------------------------------------------------------

// apDungVanBan turns a decided domain.ThayDoiVanBan into statements, INSIDE the caller's
// transaction.
//
// ONE FUNCTION FOR CREATE AND FOR EDIT, so the two cannot disagree about what a block means. On a
// create the diff was computed against an empty stored block, so only the first loop does anything —
// which is the same code path, not a second one that happens to behave alike.
//
// # THE POSITION IS MINTED AGAINST THE STORE'S HIGH-WATER MARK, ONCE PER GROUP
//
// `ThuTuVanBanLonNhat` counts SOFT-DELETED rows on purpose (see its own comment), so a number a
// removed line still holds can never be handed out again. It is read once per group that actually
// receives a new line and then advanced in Go: reading it per line would be one round trip each, and
// re-reading it would answer the same number every time because the INSERTs of this transaction are
// not visible to a statement that has not run yet. A group nobody adds to is never queried at all.
//
// ⚠ NOTHING HERE RECOMPUTES `thu_tu` OF AN EXISTING LINE, and nothing here can: `SuaVanBan` does not
// name that column, and `td.Sua` carries the value read from the stored row. That is the rule
// migration 0009 spends a block on, and it is enforced in three places on purpose — the unique key
// underneath, the absent column in the UPDATE, and domain.SoSanhVanBan above.
func (uc *GhiNhiemVu) apDungVanBan(ctx context.Context, tx *store.ScopedTx, nhiemVuID string,
	td domain.ThayDoiVanBan, nguoiMa string, luc time.Time) error {

	moc := make(map[domain.NhomVanBanNhiemVu]int, 3)
	for _, v := range td.Them {
		lonNhat, daDoc := moc[v.Nhom]
		if !daDoc {
			var err error
			if lonNhat, err = uc.kho.ThuTuVanBanLonNhat(ctx, tx, nhiemVuID, v.Nhom); err != nil {
				return err
			}
		}
		thuTu := domain.ThuTuVanBanTiepTheo(lonNhat)
		moc[v.Nhom] = thuTu

		id, err := uc.sinhID()
		if err != nil {
			return fmt.Errorf("nhiem_vu_van_ban: sinh mã nội bộ: %w", err)
		}
		if err := uc.kho.ThemVanBan(ctx, tx, domain.NhiemVuVanBan{
			ID:        id,
			NhiemVuID: nhiemVuID,
			Nhom:      v.Nhom,
			SoKyHieu:  v.SoKyHieu,
			// A ZERO time.Time REACHES THE STORE AS SQL NULL, and it is the ordinary case: §7.2 has
			// one textarea and collects no document date at all today.
			NgayVanBan: v.NgayVanBan,
			TrichYeu:   v.TrichYeu,
			ThuTu:      thuTu,
		}); err != nil {
			return err
		}
	}

	for _, v := range td.Sua {
		if err := uc.kho.SuaVanBan(ctx, tx, v); err != nil {
			return err
		}
	}

	// SOFT, ALWAYS. `nguoiMa` is the acting principal's STAFF BUSINESS CODE (rule 6, invariant 8):
	// `deleted_by` is read years later by somebody handling a complaint, and a ULID names nobody.
	// There is no reason column here — migration 0009 sets out why: removing a line is an EDIT of the
	// task, and the reason for an edit lives on the audit entry of the act.
	for _, v := range td.Xoa {
		if err := uc.kho.XoaMemVanBan(ctx, tx, nhiemVuID, v.ID, nguoiMa, luc); err != nil {
			return err
		}
	}
	return nil
}

// idVanBanDaGo lists the internal ids of the lines an edit removed, for the audit delta.
//
// IDS AND NOT TEXT — see the note at the call site. A ULID names nobody on its own, which is exactly
// why it is safe here and exactly why rule 6, invariant 8 forbids it for the ACTOR: this is a
// pointer back to a row that still exists, not a claim about a person.
//
// IT RETURNS nil FOR AN EMPTY SET, so the delta carries `null` rather than `[]` when nothing was
// removed. One shape for "no lines were removed" beats two.
func idVanBanDaGo(xoa []domain.NhiemVuVanBan) []string {
	if len(xoa) == 0 {
		return nil
	}
	ra := make([]string, 0, len(xoa))
	for _, v := range xoa {
		ra = append(ra, v.ID)
	}
	return ra
}

// --- shared -------------------------------------------------------------------------------------

// ghiNhatKy appends one timeline entry for an act on this task.
//
// ONE FUNCTION FOR EVERY ACT THAT WRITES ONE, so the convention cannot differ between them: the
// entry carries THE STATE THE TASK IS IN AFTER the act, which is what domain.TrangThaiTruocTamDung
// reads when a paused task is resumed. An act that recorded the state BEFORE itself would make every
// resume read one row too far back — and no test of a single transition could show it.
//
// THE DEPARTMENT AND THE OFFICER ARE COPIED FROM THE TASK, so §5.9's "thông tin bộ phận/phụ trách"
// renders who was holding the work at that step. They are NOT personal data: both are staff
// identifiers (rule 3 is about the people a commune serves).
func (uc *GhiNhiemVu) ghiNhatKy(ctx context.Context, tx *store.ScopedTx, n domain.NhiemVu,
	luc time.Time, nguoiMa, noiDung string) error {

	noiDung, err := domain.KiemNoiDungNhatKy(noiDung)
	if err != nil {
		return err
	}
	id, err := uc.sinhID()
	if err != nil {
		return fmt.Errorf("nhat_ky_nhiem_vu: sinh mã nội bộ: %w", err)
	}
	return uc.kho.GhiNhatKy(ctx, tx, domain.NhatKyNhiemVu{
		ID:                   id,
		NhiemVuID:            n.ID,
		NguoiMa:              nguoiMa,
		ThoiDiem:             luc,
		TrangThaiTaiThoiDiem: n.TrangThai,
		BoPhanID:             n.BoPhanID,
		NguoiPhuTrachMa:      n.NguoiThucHienMa,
		NoiDung:              noiDung,
	})
}

// bocNhiemVu wraps a failure with the commune and the operation, and NOTHING ELSE.
//
// NOT THE REGISTER NUMBER AND NOT THE FREE TEXT. An error travels into centralised logging across
// every commune at once (rule 3, invariants 1 and 2); the commune is not personal data and is the
// one thing an operator can act on.
//
// The chain is kept with %w so the handler can still tell a refusal from a failure with errors.Is.
// A %v here would collapse "this task has already moved" and "the database is down" into one 500.
func bocNhiemVu(ctx context.Context, viec string, err error) error {
	return fmt.Errorf("nhiem_vu: %s cho xã %s: %w", viec, tenant.MustFrom(ctx), err)
}
