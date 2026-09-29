package domain

// The SEED set for a commune's processing-deadline table — the rows POST /api/v1/sla/defaults
// writes into `sla` for a commune that has none (migration 0008, ADR 0029).
//
// =================================================================================================
// READ THIS BEFORE CONCLUDING THIS FILE BREAKS RULE 10, FORBIDDEN #3.
//
// Rule 10 forbids "a commune's SLA figures hardcoded in source". This is NOT that, and the
// difference is not a technicality — it is the difference between a number the commune OWNS and a
// number the software substitutes for one:
//
//	FORBIDDEN      a figure in source READ AT THE MOMENT A DEADLINE IS COMPUTED, standing in for an
//	               empty table. The commune never owns that number, cannot see it on any screen and
//	               cannot change it; the citizen is told a commitment the AUTHORITY never made.
//	NOT FORBIDDEN  a figure written ONCE into the commune's OWN ROWS by a deliberate administrative
//	               act, which then appears on Cấu hình → Thời hạn xử lý and which the commune edits
//	               from the first day. After the seed runs, the source is no longer the source: the
//	               row is.
//
// THE CONSEQUENCE THAT KEEPS THE TWO APART, AND IT MUST NOT BE SOFTENED: nothing on the
// deadline-computing path may ever read this table. An empty `sla` still answers FAILED_PRECONDITION
// through grpc.ResolveDeadlines and still makes `service-documents` answer 409 `sla_chua_cau_hinh`.
// This file is a way for the table to STOP BEING EMPTY; it is not a way to carry on while it is.
// A fallback wired into store.SLAStore or internal/grpc/sla.go re-creates exactly the defect
// migration 0008 §NOTHING IS SEEDED spends three paragraphs refusing.
//
// WHY IT IS NOT A MIGRATION EITHER — migration 0008 already answered that: a row carries
// `tenant_id`, the migration path has no commune in it (ADR 0013), and reading `platform`'s tenant
// registry from another service's migration is rule 2, forbidden #2. Seeding is therefore a
// PER-COMMUNE administrative act, which is what makes it a route.
//
// =================================================================================================
// WHERE THE NUMBERS COME FROM — docs/ui-ux/14-cau-hinh.md §8, THE TABLE AT :297-312, VERBATIM.
//
// Nothing here was invented, adjusted or rounded. They are ONE commune's prototype (migration 0008
// says so, and so does ADR 0029 §"Xã chưa cấu hình"), which is precisely why they are offered as a
// STARTING POINT a commune edits rather than as a standard. Every one of the five numbers on every
// row is a count of WORKING HOURS (ADR 0007, rule 10, invariant 4) — never wall-clock hours, and
// never days: the security field really is 2 working hours to acknowledge, and no count in days can
// say that.
//
// THE SPECIFICATION'S TABLE HAS 16 ROWS; THIS ONE HAS 15 OF THEM PLUS THE `don-thu` ROW (16 in all,
// see below). The specification row dropped is :308,
// `ve-sinh-moi-truong`, and the specification itself labels it *"mã cũ, không còn trong danh mục"*
// and notes at :314 that the screen renders the raw code. It is the SAME FIELD as `rac-thai` —
// identical label ("Rác thải – Vệ sinh môi trường") and identical five numbers (4/24/8/16/32) — so
// it is a rename that was never cleaned up, not a thirteenth field. Seeding it would manufacture,
// on day one, the exact defect ADR 0026 §2 and ADR 0024 cite it as evidence of: a live SLA row
// pointing at a code no catalogue holds.
//
// =================================================================================================
// THE FIELD CODES ARE THE TIER-1 CODE SET, NOT A TRANSLATION MADE HERE.
//
// §8 lists fields by LABEL; the codes are docs/ui-ux/09-phan-anh-nguoi-dan.md §5, the closed
// twelve-code set ADR 0026 places in service `platform`. The mapping is label-for-label and it is
// TOTAL: all twelve codes appear below exactly once, and every `phan-anh` row of §8 except the old
// one maps onto one of them. That totality is asserted in sla_gieo_test.go, because a mapping that
// is one short is a field a commune silently has no deadline for.
//
// WHY THIS DOES NOT NEED THE WRITE-TIME CHECK ADR 0026 STOP CONDITION #2 IS ABOUT, and this is the
// load-bearing sentence of the whole file: that stop condition guards a write path that accepts a
// FIELD CODE FROM A CLIENT and must therefore validate it against `platform`'s code set over a read
// path no ADR has chosen. NOTHING HERE TAKES A CODE FROM ANYBODY. This is a fixed list in source,
// and the edit route deliberately has no parameter for `linh_vuc` at all — it changes the five
// numbers of a row that already exists, by id. So no unvalidated code can enter the table through
// either route, and no cross-service read path is created.
//
// WHAT THAT LEAVES UNBUILT, SAID PLAINLY: the specification's `+ Thêm thời hạn cho một lĩnh vực`
// button (14-cau-hinh.md:293) has NO route, and cannot have one until ADR 0026 stop condition #2 is
// answered. A commune may edit the fifteen rows below and may not yet add a sixteenth.
//
// =================================================================================================
// THE TWO ESCALATION COLUMNS count from the MISSED DEADLINE (ADR 0029 §Bổ sung 29/09), and every row
// below has the chairman figure at twice the unit head's, so Y >= X holds for the whole seed set.
//
// THE SIXTH FIGURE, `unassigned_hold_hours`, IS 8 ON EVERY ROW — the user's suggested starting value
// (2026-09-29) for "working hours a unit may hold work unassigned before it is reported". The
// specification's table has no such column, so 8 is the user's number, not §8's. Like every figure
// here it is written into the commune's own rows and edited there; a NULL (a row the commune cleared)
// reports nothing, and nothing substitutes 8 for it.
//
// =================================================================================================
// `don-thu` (migration 0016, user decision 2026-09-29) GETS THE SHAPE OF `van-ban-den`, 8/40/24/24/48.
// The specification's §8 table predates the kind and the reference system seeds no rule for đơn thư
// (../vigov-require/apps/api/app/modules/org/data/default_config.json:674-755 — feedback, document and
// task only), so the nearest documented row — the same register's incoming documents — is the
// starting point, and the commune edits it.

// GieoSLA is one row of the seed set. It is a DongSLA without an ID, because the ID is minted per
// commune at the moment the row is written and a fixed one here would be the same ULID in every
// commune's table.
type GieoSLA struct {
	LoaiViec LoaiViec

	// LinhVuc is empty for the default row of a kind of work — the same convention DongSLA uses,
	// folded to NULL by the store (migration 0008: the empty string is refused outright).
	LinhVuc string

	GioTiepNhan   int
	GioXuLyXong   int
	GioSapDenHan  int
	GioBaoLanhDao int
	GioBaoChuTich int

	// UnassignedHoldHours — see the file comment. 0 would seed NULL; every row below carries 8.
	UnassignedHoldHours int
}

// BoGieoSLA returns the seed set, in the order it is written.
//
// A FUNCTION RETURNING A FRESH SLICE, NOT AN EXPORTED VAR: a package-level slice is mutable by any
// caller in the process, and one commune's seeding run silently editing the set the next commune
// gets is the kind of defect that only shows up under load.
//
// THE ORDER IS THE SPECIFICATION'S, WHICH IS ALSO THE READ ORDER (`loai_viec`, then `linh_vuc` with
// the default first). It is not relied on for correctness — the unique key is what stops duplicates
// — but a stable order makes the audit entry's count and the screen's first load agree.
func BoGieoSLA() []GieoSLA {
	return []GieoSLA{
		// ---- van-ban-den ------------------------------------------------------------------------
		// 14-cau-hinh.md:297. ONE ROW ONLY, and that is the specification's own shape: an incoming
		// document has no `linh_vuc`, so the default row is the only row this kind of work can have.
		// It is the row `service-documents` reads for `han_tiep_nhan` on every entry into the
		// register — the read that answers 409 `sla_chua_cau_hinh` today.
		{LoaiViecVanBanDen, "", 8, 40, 24, 24, 48, 8},

		// ---- don-thu ----------------------------------------------------------------------------
		// ONE ROW, the default: a citizen letter carries no `linh_vuc` on this table. Figures copied from
		// the `van-ban-den` row above — see the file comment for why that is the starting point.
		{LoaiViecDonThu, "", 8, 40, 24, 24, 48, 8},

		// ---- phan-anh ---------------------------------------------------------------------------
		// The twelve tier-1 field codes (09-phan-anh-nguoi-dan.md §5) plus the default row, in the
		// specification's own order (:298-311, alphabetical by Vietnamese label).
		//
		// THE DEFAULT ROW IS LAST HERE AND FIRST ON READ, and it is the one row that may not be
		// omitted: ADR 0028 decision E reads `gio_tiep_nhan` from it for EVERY petition a citizen
		// sends, because at that instant nobody knows the field yet. A seed set without it would
		// leave the citizen channel with no deadline while looking configured.
		{LoaiViecPhanAnh, "an-ninh", 2, 16, 4, 8, 16, 8},            // An ninh trật tự
		{LoaiViecPhanAnh, "an-toan-thuc-pham", 2, 24, 6, 12, 24, 8}, // An toàn thực phẩm
		{LoaiViecPhanAnh, "can-bo", 8, 120, 24, 24, 48, 8},          // Thái độ / tác phong cán bộ
		{LoaiViecPhanAnh, "cap-thoat-nuoc", 4, 48, 8, 16, 32, 8},    // Cấp thoát nước
		{LoaiViecPhanAnh, "dien", 2, 24, 6, 12, 24, 8},              // Điện
		{LoaiViecPhanAnh, "giao-thong", 8, 168, 24, 24, 48, 8},      // Hạ tầng giao thông
		{LoaiViecPhanAnh, "khac", 8, 72, 24, 24, 48, 8},             // Khác
		{LoaiViecPhanAnh, "o-nhiem", 8, 72, 24, 24, 48, 8},          // Ô nhiễm (tiếng ồn, khí thải, nước thải)
		{LoaiViecPhanAnh, "rac-thai", 4, 24, 8, 16, 32, 8},          // Rác thải – Vệ sinh môi trường
		{LoaiViecPhanAnh, "trat-tu-do-thi", 6, 40, 12, 24, 48, 8},   // Trật tự đô thị – lấn chiếm vỉa hè
		{LoaiViecPhanAnh, "xay-dung", 4, 72, 12, 12, 24, 8},         // Xây dựng không phép
		{LoaiViecPhanAnh, "y-te-giao-duc", 8, 72, 24, 24, 48, 8},    // Y tế – Giáo dục
		{LoaiViecPhanAnh, "", 8, 56, 24, 24, 48, 8},                 // Mặc định cho mọi lĩnh vực
		//
		// 14-cau-hinh.md:308 — `ve-sinh-moi-truong`, 4/24/8/16/32 — IS DELIBERATELY ABSENT. It is the
		// old code of `rac-thai` above (same label, same five numbers) and the specification marks it
		// dead at :308 and :314. This comment stands in its place so nobody "completes" the list from
		// the specification without reading why it is fifteen rows and not sixteen.

		// ---- nhiem-vu ---------------------------------------------------------------------------
		// 14-cau-hinh.md:312. Note gio_sap_den_han (72) EXCEEDS gio_xu_ly_xong (40) on this row. That
		// is the customer's own configuration, not a typo to fix here, and migration 0008 cites this
		// very row as the reason there is no ordering CHECK between the columns: a constraint that
		// refuses real configuration is discovered on the day a commune is being set up.
		{LoaiViecNhiemVu, "", 8, 40, 72, 24, 48, 8},
	}
}
