package domain

import "time"

// Meeting minutes and their conclusions — `bien_ban_hop` / `ket_luan_hop`, entities `Meeting` and
// `MeetingConclusion` (migration 0007).
//
// THIS FILE IMPORTS THE STANDARD LIBRARY AND NOTHING ELSE (rule 4 of the service pattern). The two
// counting rules below are what the screen's badges mean, and they are expressed as pure functions
// precisely so they can be tested without a database.
//
// # THE ONE THING THAT IS NOT IN THIS FILE: THE BACK-LINK
//
// A task born from a conclusion carries `NhiemVu.NguonGiao == NguonKetLuanHop` and
// `NhiemVu.NguonID == KetLuanHop.ID` — the blurred pair 0006 declared and 0007 deliberately did NOT
// duplicate with a `ket_luan_id` column of its own. There is therefore no `Tasks []NhiemVu` field
// here and no method that walks from a conclusion to its tasks: that walk is a QUERY (the counts
// below arrive already aggregated from the store), and a slice of tasks hanging off a conclusion
// would be a second place for the same fact to be wrong.

// The two statuses of minutes (user decision 25/09/2026), the closed list of migration 0012's
// `bien_ban_hop_trang_thai_hop_le`. migrations/bien_ban_vong_doi_test.go holds the two lists equal.
//
// There is no transition back: once `da-ky`, the database refuses any edit of the business columns
// (trigger `bien_ban_hop_da_ky_bat_bien`), and a correction is SUPPLEMENTARY minutes.
const (
	TrangThaiBienBanDuThao = "du-thao"
	TrangThaiBienBanDaKy   = "da-ky"
)

// BienBanHop is one meeting's minutes as the business sees it — `docs/ui-ux/04-bien-ban-hop.md` §5.
//
// IT CARRIES NO CITIZEN PERSONAL DATA. Everybody named on it is a member of staff, by business code
// (rule 6, invariant 8). Two fields hold free text a clerk typed — the minutes body and each
// conclusion's content — and a commune's minutes do quote cases: they are rendered on a staff
// screen and never logged, never put in a file name (rule 3, forbidden #1 and #4).
type BienBanHop struct {
	ID string

	// TenCuocHop is §4's "Tên cuộc họp", the bold line of the card.
	TenCuocHop string

	// NgayHop is a CALENDAR DAY, not an instant: the form collects a date and the card renders
	// `5/8/2026`. The column is DATE, so the time-of-day part of this value carries no information
	// and must not be rendered — see migration 0007 for why that is the opposite choice from the
	// task register's two deadlines.
	NgayHop time.Time

	// SoHieu is the reference number of the written minutes, `31/BB-UBND`. EMPTY IS ORDINARY — §4
	// marks it optional and the card drops the segment when it is missing.
	//
	// It is NOT an issued number in rule 7, invariant 3's sense: it is typed by hand and restarts
	// each year, which is why no unique key guards it.
	SoHieu  string
	DiaDiem string

	// ChuTriMa is a STAFF BUSINESS CODE (`CB-2026-7K3M9Q`), never an internal id (rule 6, invariant
	// 8). §5 models it as a uuid; the rule wins, and 0007 states why.
	ChuTriMa string

	// NoiDung is §4's "Nội dung biên bản" — the minutes IN FULL — and ThanhPhan is its "Thành phần
	// tham dự", staff codes and/or free text.
	//
	// ⚠ NEITHER IS POPULATED BY THE REGISTER LIST. `GET /api/v1/meetings` does not select them and
	// the card of §2 does not draw them, so an empty value here means "this read did not ask for it",
	// NOT "this meeting has none". The detail read (`GET /api/v1/meetings/{id}`) fills both.
	//
	// ⚠ ThanhPhan IS A LIST OF STAFF. Migration 0007 says so on the column: the day somebody writes a
	// reporter's name and number into it, this table has become a personal-data store (rule 3). And
	// NoiDung is free text a clerk typed which routinely quotes a case — it may not travel into a log
	// line, an error message or a file name (rule 3, forbidden #1 and #4).
	NoiDung   string
	ThanhPhan []string

	NguoiTaoMa string
	TaoLuc     time.Time

	// --- the lifecycle and the fields of migration 0012 (user decisions 25/09/2026) ---------------

	// TrangThai is TrangThaiBienBanDuThao or TrangThaiBienBanDaKy. Every read that fills this struct
	// selects it; the column is NOT NULL DEFAULT 'du-thao', so an empty value here means the read did
	// not ask, never "no status".
	TrangThai string

	// KyLuc and KyBoiMa record the signing act. BOTH ZERO ON A DRAFT, both set on signed minutes —
	// the CHECK `bien_ban_hop_ky_du_truong` holds them together. KyBoiMa is a STAFF BUSINESS CODE
	// (rule 6, invariant 8).
	KyLuc   time.Time
	KyBoiMa string

	// ThuKyMa is the secretary, a STAFF BUSINESS CODE. Empty is ordinary: small meetings name none.
	ThuKyMa string

	// TbSoKyHieu and TbNgay are the conclusion notice (Thông báo kết luận), TRANSCRIBED from the
	// office clerk's register — never minted here (Decree 30/2020, Art. 15). Both or neither (CHECK
	// `bien_ban_hop_thong_bao_du_truong`). TbNgay is a CALENDAR DAY, like NgayHop.
	TbSoKyHieu string
	TbNgay     time.Time

	// BoSungChoID is the original these SUPPLEMENTARY minutes correct. Empty on ordinary minutes.
	BoSungChoID string

	// DuocBoSungBoi lists the LIVE supplementary minutes pointing at THIS one, oldest first.
	//
	// ⚠ nil MEANS "NOT READ ON THIS SURFACE", NOT "NOBODY SUPPLEMENTED IT" — the same split
	// NhiemVu.VanBan makes. The register LIST does not read it; the detail read always fills a
	// non-nil slice, empty when there is none.
	DuocBoSungBoi []string

	// KetLuan holds the meeting's conclusions IN THE ORDER THE CIRCLES ARE DRAWN — by ThuTu, which
	// is the order the store reads them in. Empty is a real, supported state: §7.3 says a meeting
	// with no conclusions is still saved ("nhập nháp trước, bổ sung sau").
	//
	// ⚠ ONLY LIVE CONCLUSIONS ARE IN IT (rule 7, invariant 2). A soft-deleted conclusion is out of
	// this slice, therefore out of SoKetLuan and out of TienDoNhiemVu — which is what the screen
	// must show, and the reason both counts are derived from it rather than stored.
	KetLuan []KetLuanHop
}

// KetLuanHop is one numbered conclusion of a meeting (§2, §5).
//
// THE TWO COUNTERS ARE READ FROM THE DATABASE, NOT STORED IN IT. They are `count(*)` over the tasks
// whose blurred pair points at this conclusion, computed on every read (migration 0007 says why no
// counter column exists). A zero pair means "Chưa tách thành nhiệm vụ nào", which §2 renders as its
// own sentence rather than as `0/0`.
type KetLuanHop struct {
	ID        string
	BienBanID string

	// ThuTu is the number in the circle, from 1. §7.2: numbering is continuous within one meeting
	// and new conclusions are APPENDED, never renumbered — so a gap after a removal is correct and
	// this value is never recomputed from a slice index.
	ThuTu int

	NoiDung string
	TaoLuc  time.Time

	// SoNhiemVu is `y` and SoNhiemVuXong is `x` in §2's `{x}/{y} nhiệm vụ đã hoàn thành`.
	//
	// ⚠ `x` COUNTS TASKS IN `hoan-thanh` AND NOTHING ELSE. It is not "not overdue", not "approved by
	// a leader" (§5.4's two tick boxes change no status), and not a percentage: `tien_do = 100` on a
	// task still in `dang-thuc-hien` is an officer's own report and does not finish anything.
	//
	// BOTH COUNT ONLY LIVE TASKS. A soft-deleted task leaves both the numerator and the denominator,
	// which is the only reading under which the fraction stays true after a removal.
	SoNhiemVu     int
	SoNhiemVuXong int

	// SoNhiemVuTreHan counts the live tasks that are NOT `hoan-thanh` AND late by NhiemVu.TreHan's
	// definition — aggregated in SQL by the store (the one SQL spelling of TreHan,
	// store.dieuKienTreHan). A count and not a flag: it is read, never written (rule 10, invariant 3).
	SoNhiemVuTreHan int

	// KhongPhatSinh is the human-set mark "không phát sinh nhiệm vụ" (migration 0012, decision 4).
	// The ONLY stored input to TrangThai below; nothing can derive a person's judgement.
	KhongPhatSinh bool
}

// TrangThaiKetLuan is a conclusion's DERIVED status (user decision 4, 25/09/2026). It is never
// stored: migration 0012 refuses a status column on `ket_luan_hop` for rule 10, invariant 3's reason
// — a stored copy is wrong the moment a task moves, and the stale copy is the one a report reads.
type TrangThaiKetLuan string

const (
	KetLuanChuaGiao     TrangThaiKetLuan = "chua-giao"
	KetLuanDangThucHien TrangThaiKetLuan = "dang-thuc-hien"
	KetLuanQuaHan       TrangThaiKetLuan = "qua-han"
	KetLuanHoanThanh    TrangThaiKetLuan = "hoan-thanh"
)

// TrangThai derives the status, in the precedence the user fixed:
//
//	khong_phat_sinh set                   -> hoan-thanh (the mark "counts as done")
//	no live task                          -> chua-giao
//	any live task not done AND late       -> qua-han
//	any live task not done                -> dang-thuc-hien
//	every live task `hoan-thanh`          -> hoan-thanh
//
// "DONE" IS `hoan-thanh` AND NOTHING ELSE, the same reading SoNhiemVuXong has. ⚠ A task in
// `chuyen-tiep` or `tam-dung` therefore counts as NOT done, and a late one makes the conclusion
// `qua-han`. That is the literal reading of the decision, not a judgement: how those two states
// should count is an OPEN question for the customer (ledger service-petitions/
// bien-ban-hop-tang-du-lieu, "đếm nhiệm vụ chuyen-tiep/tam-dung"). Consequence worth knowing: a
// conclusion whose only task was forwarded (`chuyen-tiep` is terminal) never reaches `hoan-thanh`.
//
// THE MARK WINS EVEN IF LIVE TASKS EXIST. The use case refuses to set it while a live task points at
// the conclusion (decision 4), so the combination should not occur; if it does, the mark is the one
// act a person recorded, and the precedence above is the user's.
func (k KetLuanHop) TrangThai() TrangThaiKetLuan {
	switch {
	case k.KhongPhatSinh:
		return KetLuanHoanThanh
	case k.SoNhiemVu == 0:
		return KetLuanChuaGiao
	case k.SoNhiemVuTreHan > 0:
		return KetLuanQuaHan
	case k.SoNhiemVuXong < k.SoNhiemVu:
		return KetLuanDangThucHien
	default:
		return KetLuanHoanThanh
	}
}

// TienDoKetLuan is the card's main figure, `x/y kết luận hoàn thành` (decision 4; SRS.md:177 of the
// sibling). Done = TrangThai() == hoan-thanh, which includes the `khong_phat_sinh` mark.
func (b BienBanHop) TienDoKetLuan() (xong, tong int) {
	for _, k := range b.KetLuan {
		if k.TrangThai() == KetLuanHoanThanh {
			xong++
		}
	}
	return xong, len(b.KetLuan)
}

// LienKetKetLuanHop is the back-link a task born from a conclusion shows in its drawer: which
// meeting, which numbered conclusion. READ-ONLY context resolved by the task register's reads — it is
// not a second copy of the link, which stays the blurred pair `nguon_giao`/`nguon_id` (migration
// 0006).
type LienKetKetLuanHop struct {
	BienBanID  string
	TenCuocHop string
	ThuTu      int
}

// SoKetLuan is `n` in the card badge `{n} kết luận · {x}/{y} nhiệm vụ xong` (§2).
//
// A METHOD AND NOT A FIELD, for the reason migration 0007 refuses a counter column: a stored count
// is wrong from the instant a conclusion is added or removed, and the stale copy is the one a badge
// shows.
func (b BienBanHop) SoKetLuan() int { return len(b.KetLuan) }

// TienDoNhiemVu is the card header's `{x}/{y} nhiệm vụ xong` — §7.5 in one line: "Bộ đếm `x/y nhiệm
// vụ xong` ở header card = tổng trên tất cả kết luận của biên bản".
//
// THE SUM IS OVER CONCLUSIONS, AND THAT IS WHY IT CANNOT BE ONE QUERY ON THE MINUTES. A task points
// at a CONCLUSION, never at a meeting: `nhiem_vu.nguon_id` holds `ket_luan_hop.id`. The only way to
// the meeting's figure is through its conclusions, so a conclusion missing from KetLuan is a
// conclusion missing from this total — see the warning on that field.
//
// A MEETING WITH NO CONCLUSIONS RETURNS (0, 0), which the card shows as `0 kết luận` with no
// fraction. It is not an error and not "nothing done yet": it is minutes typed in draft (§7.3).
func (b BienBanHop) TienDoNhiemVu() (xong, tong int) {
	for _, k := range b.KetLuan {
		xong += k.SoNhiemVuXong
		tong += k.SoNhiemVu
	}
	return xong, tong
}

// ChuaTachNhiemVu reports the state §2 renders as `Chưa tách thành nhiệm vụ nào` instead of a
// fraction.
//
// IT ASKS ABOUT THE DENOMINATOR ONLY. `0/3` and `Chưa tách` are two different things on the screen:
// the first says three people were told to do something and none has finished, the second says the
// conclusion is still just a sentence in the minutes. Collapsing them would hide exactly the
// conclusions that need somebody's attention.
func (k KetLuanHop) ChuaTachNhiemVu() bool { return k.SoNhiemVu == 0 }

// DaXongToanBo reports whether every task split from this conclusion is finished.
//
// A CONCLUSION NOBODY HAS SPLIT IS NOT "DONE" — that is the `ok` half of the answer, and it is
// separate for the same reason domain.NhiemVu.HoanThanhDungHanBanDau returns one: a caller must not
// be able to fold "nothing was ever assigned" into "everything asked for was completed" by
// accident. `0 == 0` is true in Go and false in a commune.
func (k KetLuanHop) DaXongToanBo() (xong, daTach bool) {
	if k.SoNhiemVu == 0 {
		return false, false
	}
	return k.SoNhiemVuXong == k.SoNhiemVu, true
}
