package http

// Routes for the petitions service.
//
// EVERY route declares its permission explicitly. The global guard only rejects users who are
// not signed in; it does not check permissions. A route with no declaration is callable by
// every staff role, and nothing reports it — see rule 5.
//
// Four declarations are available, and there is no fifth:
//
//	authz.RequirePermission(checker, "feedback.read")   // the normal case
//	authz.CitizenOnly()                                     // citizen paths, isolated by identity
//	authz.AnyAuthenticated("<why any account needs this>")  // reason mandatory
//	authz.Public("<why this is public>")                    // reason mandatory
//
// EVERY route also carries an @-annotation block IMMEDIATELY above the statement — no blank line
// between. `tools/apidoc` reads it and generates kb/20-contracts/openapi.json, the type contract
// the admin web builds against. Without it the web types every response by hand.
//
//	// @summary  <one line, Vietnamese — a person reads it>
//	// @screen   <file in docs/ui-ux/ §section>   design intent, the one part no tool can derive
//	// @request  <Go type>                        omit when the route takes no body
//	// @reply    <status> <Go type|->             one line per status the handler REALLY returns
//
// The permission and the idempotency mode are NOT annotated: apidoc reads them from the authz.* /
// idem.* calls below, so there is no second copy to drift (rule 9).

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The catalogue readers are INTERFACES, not the concrete stores.
//
// WHY: these routes carry the isolation rule this service exists to keep — one commune's catalogue
// never reaching another commune's caller — and that has to be testable without a PostgreSQL, or it
// gets tested once and then never again. There is no PostgreSQL in this build environment at all
// (VIGOV_TEST_DSN unset), so a test that needs one is a test that does not run.
//
// *petstore.LoaiNhiemVuStore and *petstore.MucUuTienNhiemVuStore satisfy these as they are; nothing
// was changed to accommodate them.
type (
	// LoaiNhiemVuDanhMuc is the commune's task-type catalogue, for GET /api/v1/task-types.
	//
	// NO page.Request PARAMETER: this route returns the whole list on purpose. The reason is on
	// petstore.LoaiNhiemVuStore.DanhSach, and the bound that replaces the missing `limit` is
	// petstore.TranDanhMucLoaiNhiemVu.
	LoaiNhiemVuDanhMuc interface {
		DanhSach(ctx context.Context) ([]domain.LoaiNhiemVu, error)
	}

	// MucUuTienDanhMuc is the commune's task-priority scale, for GET /api/v1/task-priorities.
	//
	// A SECOND, NARROW INTERFACE RATHER THAN ONE SHARED "catalogue reader", although the two have
	// the same method shape and the same arity. A shared reader would have to be told WHICH
	// catalogue to read, which means a table name or a key travelling as an argument — and the
	// handler that passes the wrong one produces a perfectly valid response listing the wrong
	// catalogue. Two interfaces make that call impossible to express, and each route depends on
	// exactly the one list it renders: a dependency wider than the work requires is how the next
	// person justifies using it for something else.
	MucUuTienDanhMuc interface {
		DanhSach(ctx context.Context) ([]domain.MucUuTienNhiemVu, error)
	}

	// TrangThaiNhiemVuDoc is the commune's OVERRIDES of the seven task-status labels (migration
	// 0010), for GET /api/v1/task-statuses. It returns only the rows that exist; the handler merges
	// them over domain.MacDinhTrangThaiNhiemVu, so a code with no row is the default, never missing.
	TrangThaiNhiemVuDoc interface {
		DanhSach(ctx context.Context) ([]domain.NhanTrangThaiNhiemVu, error)
	}

	// PhieuPhanAnhDoc is the read side of the petition register, for
	// GET /api/v1/citizen-reports/{maTraCuu}.
	//
	// READ ONLY, and the two write methods on *petstore.PhieuPhanAnhStore are deliberately NOT
	// in this interface. A route depending on a wider surface than it uses is how the next
	// person justifies writing through it — and the reasons the two write ROUTES are missing
	// (below) are not reasons this interface should carry the methods anyway.
	PhieuPhanAnhDoc interface {
		TheoMaTraCuu(ctx context.Context, ma string) (domain.PhieuPhanAnh, error)
	}

	// VetXemNguoiGui records ONE disclosure of a reporter's full name and phone number.
	//
	// IT IS A DEPENDENCY OF A READ ROUTE, which looks wrong and is not: rule 6, invariant 7 audits
	// reading FULL personal data, and ADR 0030 makes that the condition attached to the
	// `feedback.unmask` key rather than a follow-up task. The handler cannot write the entry
	// itself — audit.Write takes a *store.ScopedTx and there is no overload that writes outside a
	// transaction — so the use case owns it and the route depends on the use case.
	//
	// THE INTERFACE TAKES NO TRANSACTION AND RETURNS ONLY AN ERROR, and that shape is the
	// contract: an implementation either committed the entry or it did not, and the route treats
	// "did not" as "do not disclose". There is no third answer for a caller to interpret.
	VetXemNguoiGui interface {
		GhiVet(ctx context.Context, maTraCuu string, nguoi audit.Actor) error
	}

	// NhanLinhVucDanhMuc answers the LABEL each field code shows. In production it is
	// app.EffectiveFieldLabels: every tier-1 code with the commune's wording, else the platform
	// default (ADR 0026 + 0060), never filtered by on/off or retired. A code absent from the answer
	// is one tier 1 does not know — the screen then has no label for it. The error may be
	// app.ErrFieldCatalogueUnavailable, which read routes answer 503 (writeFieldCatalogueError).
	NhanLinhVucDanhMuc interface {
		DanhSach(ctx context.Context) ([]domain.NhanLinhVuc, error)
	}

	// PhieuPhanAnhDanhSach is the paginated read of the register, for GET /api/v1/citizen-reports.
	//
	// A THIRD INTERFACE OVER THE SAME STORE, and not a method added to PhieuPhanAnhDoc, because the
	// two answer different questions and one of them carries the restricted-field decision. A list
	// route that could reach TheoMaTraCuu would be a list route somebody could make read one
	// petition WITHOUT the `can-bo` filter — which is exactly the path that filter exists to close.
	PhieuPhanAnhDanhSach interface {
		DanhSach(ctx context.Context, loc petstore.LocPhieu, yc page.Request) (
			page.Result[domain.PhieuPhanAnh], error)
		// The total and the heat map under the SAME filter (09/10/2026) — on THIS interface, the
		// NhiemVuDanhSach precedent, so neither can be wired to a reader other than the list's.
		CountCitizenReports(ctx context.Context, loc petstore.LocPhieu) (int, error)
		CitizenReportPoints(ctx context.Context, loc petstore.LocPhieu) ([]domain.CitizenReportPoint, error)
	}

	// NhiemVuDoc is the read side of ONE task, for GET /api/v1/tasks/{ma}.
	//
	// READ ONLY, and separate from the list below for the same reason PhieuPhanAnhDoc is separate
	// from PhieuPhanAnhDanhSach: a route depending on a wider surface than it uses is how the next
	// person justifies reaching through it. The write methods this store will grow in the next pass
	// have no business being reachable from a GET.
	NhiemVuDoc interface {
		TheoMa(ctx context.Context, ma string) (domain.NhiemVu, error)

		// §5.4's document block (migration 0009). A SECOND METHOD AND NOT A WIDER TheoMa, because
		// the two surfaces need different things: the register LIST renders no documents, and
		// folding the block into the single-task read would have been the shape that put it there
		// too. It is keyed by the task's INTERNAL id, which the caller has from the row it just
		// read.
		VanBanCuaNhiemVu(ctx context.Context, nhiemVuID string) ([]domain.NhiemVuVanBan, error)

		// §5.9's progress log (migration 0006), one page. ON THIS INTERFACE AND NOT A FIELD OF ITS
		// OWN, for the reason VanBanCuaNhiemVu is: it is keyed by the task's INTERNAL id, which the
		// handler has only after TheoMa — so the soft-delete and commune checks of that read cannot be
		// skipped by calling this one with a register number. (The petition logbook has a separate
		// NhatKyPhieuDoc because its read belongs to a different interface than the list; here the
		// single-task read already is the narrow one.)
		NhatKyCuaNhiemVu(ctx context.Context, nhiemVuID string, yc page.Request) (
			page.Result[domain.NhatKyNhiemVu], error)
	}

	// NhiemVuDanhSach is the paginated read of the task register, for GET /api/v1/tasks, and its
	// per-status count, for GET /api/v1/task-counts.
	//
	// ONE INTERFACE FOR BOTH because they are one question asked two ways — which tasks match these
	// filters, and how many per status — under one permission, over one filter struct. Splitting
	// them would let the two be wired to different readers, and a count that came from somewhere
	// other than the list is the drift the route exists to prevent.
	NhiemVuDanhSach interface {
		DanhSach(ctx context.Context, loc petstore.LocNhiemVu, yc page.Request) (
			page.Result[domain.NhiemVu], error)
		CountByStatus(ctx context.Context, loc petstore.LocNhiemVu) (
			map[domain.TrangThaiNhiemVu]int, error)

		// DocumentsForTasks is §5.4's document block for a WHOLE PAGE in one statement, keyed by
		// the tasks' INTERNAL ids — the list under `include=documents` (§4.3's Sổ theo dõi). Every
		// asked id comes back with a non-nil slice. On this interface and not on NhiemVuDoc because
		// it is the list's projection, bound to the same page the list just read.
		DocumentsForTasks(ctx context.Context, taskIDs []string) (map[string][]domain.NhiemVuVanBan, error)
	}

	// TaskFilterIdentity is what the task list's two identity-backed filters ask identity, for
	// GET /api/v1/tasks and GET /api/v1/task-counts. *identityclient.Client satisfies it as it is.
	//
	//	DueSoonCutoff  `soon=true` — the commune's own "sắp đến hạn" threshold, walked in WORKING
	//	               hours by identity (ADR 0007, 0029). Never a number held here.
	//	StaffOrgUnits  `scope=related` — the caller's OWN live unit(s), by the session's business code.
	//	               It NARROWS a list; it grants nothing and must never guard a write (rule 5).
	//
	// NEVER PER ROW, and neither answer is cached (rule 1: a process cache keyed without the commune
	// serves one commune's answer to another). StaffOrgUnits is one call per request; DueSoonCutoff is
	// one per task-priority level plus the default row (ADR 0079 lô 2 Q4 b — each level has its own
	// threshold), or one when the request names a `priority`. The scale is bounded at
	// petstore.TranDanhMucMucUuTien.
	TaskFilterIdentity interface {
		DueSoonCutoff(ctx context.Context, kind identityv1.WorkKind, linhVuc string, asOf time.Time) (
			time.Time, error)
		StaffOrgUnits(ctx context.Context, staffCode string) ([]string, error)
	}

	// TaskRegisterExporter is the Sổ theo dõi export (§4.3), for GET /api/v1/tasks/register-export.
	// *app.TaskRegisterExport satisfies it. ITS OWN INTERFACE because it WRITES — the audit entry
	// every export leaves — and a read interface must stay unable to write.
	TaskRegisterExporter interface {
		Export(ctx context.Context, req app.RegisterExportRequest, actor audit.Actor,
			render func(app.TaskRegisterData) ([]byte, error)) ([]byte, int, error)
	}

	// TaskImporter is the spreadsheet import (§8), for POST /api/v1/tasks/imports.
	// *app.TaskImport satisfies it. It WRITES — every task, its trail, one batch entry — in one
	// transaction, so it is its own interface.
	TaskImporter interface {
		Import(ctx context.Context, sheet [][]string, dryRun bool, actor audit.Actor) (app.TaskImportResult, error)
	}

	// DeNghiLuiHanChoDuyetDoc is the approval queue of extension requests (§5.8), for
	// GET /api/v1/task-extensions. *petstore.DeNghiLuiHanStore satisfies it.
	//
	// READ ONLY, AND ITS OWN INTERFACE rather than a method on the task readers: the rows are the
	// extension table's (joined to their task), and the decision path — which WRITES that table —
	// must stay unreachable from a GET.
	DeNghiLuiHanChoDuyetDoc interface {
		ChoDuyet(ctx context.Context, loc petstore.LocDeNghiChoDuyet, yc page.Request) (
			page.Result[domain.DeNghiLuiHanChoDuyet], error)

		// CountPending is the same queue's size under the same filter, for
		// GET /api/v1/task-extension-counts. On THIS interface so the count cannot be wired to a
		// reader other than the one the queue pages through.
		CountPending(ctx context.Context, loc petstore.LocDeNghiChoDuyet) (int, error)

		// TaskHistory is ONE task's requests, every status, newest first, with the decider's note —
		// GET /api/v1/tasks/{ma}/extensions. Keyed by the task's INTERNAL id, which the handler has only
		// after NhiemVuDoc.TheoMa, so that read's soft-delete and commune checks cannot be skipped. On
		// this interface because it is the same read-only reader over the same table; it does NOT
		// share the queue's predicate builder, so the queue and its badge are untouched.
		TaskHistory(ctx context.Context, taskID string, yc page.Request) (
			page.Result[domain.DeNghiLuiHan], error)
	}

	// GhiNhiemVuUseCase is the six STAFF acts on a task, each of which opens a transaction and
	// writes the business change, the timeline row and the audit entry inside it (rule 6,
	// invariant 3).
	//
	// ONE INTERFACE FOR THE SIX, unlike the two catalogue writers above, and the reason is the
	// opposite of theirs: those are DIFFERENT CATALOGUES, so one shared interface would need to be
	// told which table to write. These six are six acts on ONE record — they share the locking
	// read, the tree walks and the timeline — and splitting them would be six wiring lines that can
	// disagree about which use case instance, and therefore which transaction boundary, is in play.
	//
	// THE PERMISSIONS ARE NOT IN THE INTERFACE, deliberately: they are declared per route below, at
	// the one place rbac_guard and tools/apidoc both read.
	//
	// ⚠ ONLY DoiTrangThai TAKES app.QuyenDuyetHoanThanh, AND THE ASYMMETRY IS THE DEFENCE. It is
	// the caller's answer to "does this account hold `task.approve`", and it can only ever NARROW —
	// no value of it lets anybody do something they could not otherwise do. A second method growing
	// the same parameter would be a second act that could be signed off by the wrong key.
	GhiNhiemVuUseCase interface {
		Tao(ctx context.Context, yc app.YeuCauTaoNhiemVu, nguoi audit.Actor) (domain.NhiemVu, error)
		Sua(ctx context.Context, ma string, sua petstore.SuaNhiemVu, nguoi audit.Actor) (
			domain.NhiemVu, error)
		// `update` is "does the caller hold `task.update`" — a37ec96's commune-wide "full" case, which
		// together with being the assignee decides who may move the status at all (the route is gated
		// by `task.read`). `duyet` then narrows the moves that need `task.approve`. The optional handover
		// carries its own `task.assign` fact INSIDE yc (yc.AssignRight), read only when yc.Handover is set.
		DoiTrangThai(ctx context.Context, ma string, yc app.YeuCauDoiTrangThai, nguoi audit.Actor,
			duyet app.QuyenDuyetHoanThanh, update app.TaskUpdateRight) (domain.NhiemVu, error)
		Xoa(ctx context.Context, ma, lyDo string, nguoi audit.Actor) error
		DeNghiLuiHan(ctx context.Context, ma string, yc app.YeuCauDeNghiLuiHan, nguoi audit.Actor) (
			domain.DeNghiLuiHan, error)
		QuyetDinhLuiHan(ctx context.Context, ma, deNghiID string, yc app.YeuCauQuyetDinhLuiHan,
			nguoi audit.Actor) (domain.DeNghiLuiHan, error)
		// Reassign is the seventh act, `task.assign` (owner decision 28/09/2026): the same row handed to
		// another unit/person (the lead unit and the monitor ARE unit and assignee since ADR 0065 NV5).
		// See internal/app/task_assignment.go.
		Reassign(ctx context.Context, ma string, req app.TaskAssignmentRequest, nguoi audit.Actor) (
			domain.NhiemVu, error)
		// AddLogEntry is the manual timeline entry (§5.9, vigov-require a37ec96). It takes the ONE fact
		// "does the caller hold `task.update`", which only ever WIDENS to people who may already edit
		// every task of the commune; who else may write is decided on the row (app/task_log_entry.go).
		// `attachments` are completed uploads of the same officer for the same task (migration 0021),
		// linked in the entry's transaction; the reply carries them. `mentions` are staff business codes
		// of colleagues named in the entry (ADR 0086 kind 21), checked against identity by the use case.
		AddLogEntry(ctx context.Context, ma, text string, attachments, mentions []string, nguoi audit.Actor,
			update app.TaskUpdateRight) (domain.NhatKyNhiemVu, []domain.TaskLogAttachment, error)
	}

	// TaskAttachmentActs is §5.9's `📎 Đính kèm` — upload a file through this service, hand out a
	// download link, remove (ADR 0052 §Sửa đổi 09/10/2026). *app.TaskAttachments satisfies it. ITS OWN
	// INTERFACE and not more methods on GhiNhiemVuUseCase: its dependencies are the object store, the
	// scanner and platform's limits, any of which may be absent in a deployment while the six task acts
	// keep working.
	TaskAttachmentActs interface {
		uploadLimiter
		Upload(ctx context.Context, ma string, req app.AttachmentUploadRequest, body app.UploadBody,
			nguoi audit.Actor, update app.TaskUpdateRight) (domain.StoredFile, error)
		DownloadLink(ctx context.Context, ma, id string, reader audit.Actor) (app.AttachmentDownload, error)
		Remove(ctx context.Context, ma, id, reason string, nguoi audit.Actor, update app.TaskUpdateRight) error
	}

	// TaskLogAttachmentReader reads the attachments of ONE PAGE of timeline entries in one statement,
	// for GET /api/v1/tasks/{ma}/log-entries. *petstore.StoredFileStore satisfies it.
	TaskLogAttachmentReader interface {
		AttachmentsByLogEntries(ctx context.Context, logEntryIDs []string) (
			map[string][]domain.TaskLogAttachment, error)
	}

	// BienBanDanhSach is the paginated read of the meeting-minutes register, for
	// GET /api/v1/meetings — each meeting already carrying its conclusions and their task counters.
	//
	// ONE METHOD, AND THE COUNTERS ARE INSIDE IT rather than a second method the handler would call
	// and add up. §7.5's badge is a SUM over the conclusions of one meeting, and a handler holding
	// two halves of that sum is a handler that can pair the wrong ones: the aggregation belongs to
	// the query that already visits both tables, and domain.BienBanHop.TienDoNhiemVu is the only
	// arithmetic left above it.
	//
	// THE WHOLE READ SURFACE OF THE REGISTER, all three methods on one store and all three under
	// `task.read`: the page of cards, ONE meeting with everything (GET /api/v1/meetings/{id}), and the
	// live tasks split from one conclusion (…/conclusions/{stt}/tasks). One interface because they
	// are three views of one record with one permission; the write acts stay on GhiBienBanUseCase.
	BienBanDanhSach interface {
		DanhSach(ctx context.Context, yc page.Request) (page.Result[domain.BienBanHop], error)
		TheoID(ctx context.Context, id string) (domain.BienBanHop, error)
		NhiemVuCuaKetLuan(ctx context.Context, bienBanID string, thuTu int) ([]domain.NhiemVu, error)
	}

	// GhiBienBanUseCase is the three STAFF acts on the meeting register (§3, §4): record the
	// minutes, append a conclusion, split a conclusion into a task.
	//
	// ONE INTERFACE FOR THE THREE, for the reason GhiNhiemVuUseCase gives: they are three acts on
	// ONE record, they share the ordinal rule and the lock it is read under, and splitting them
	// would be three wiring lines that can disagree about which use case instance — and therefore
	// which transaction boundary — is in play.
	//
	// ⚠ THE THIRD METHOD RETURNS A TASK AND NOT A CONCLUSION, and that asymmetry IS the flow: the
	// split does not write into this register at all. It resolves the conclusion and hands the work
	// to the task register's own create use case, so everything a task is — the minted number, the
	// cycle check, the deadline fixed once, the timeline row, the audit entry in one transaction —
	// stays in one place (app.TachKetLuanThanhNhiemVu).
	//
	// ⚠ IT TAKES app.YeuCauTaoNhiemVu WITH NO `nguon_giao` / `nguon_id` SUPPLIED BY THE HANDLER.
	// The pair is set from the conclusion named in the PATH, which is what makes §3's locked "Nguồn
	// giao" field a property of the server rather than a field somebody has to remember to check.
	GhiBienBanUseCase interface {
		TaoBienBan(ctx context.Context, yc app.YeuCauTaoBienBan, nguoi audit.Actor) (
			domain.BienBanHop, error)
		ThemKetLuan(ctx context.Context, bienBanID string, yc app.YeuCauThemKetLuan,
			nguoi audit.Actor) (domain.KetLuanHop, error)
		TachKetLuanThanhNhiemVu(ctx context.Context, bienBanID string, thuTu int,
			yc app.YeuCauTaoNhiemVu, nguoi audit.Actor) (domain.NhiemVu, error)

		// The lifecycle acts (user decisions 25/09/2026) — internal/app/bien_ban_hop_sua.go.
		SuaBienBan(ctx context.Context, id string, yc app.YeuCauSuaBienBan, nguoi audit.Actor) (
			domain.BienBanHop, error)
		XoaBienBan(ctx context.Context, id, lyDo string, nguoi audit.Actor) error
		KyBienBan(ctx context.Context, id string, yc app.YeuCauKyBienBan, nguoi audit.Actor) (
			domain.BienBanHop, error)
		SuaKetLuan(ctx context.Context, bienBanID string, thuTu int, yc app.YeuCauSuaKetLuan,
			nguoi audit.Actor) (domain.KetLuanHop, error)
		XoaKetLuan(ctx context.Context, bienBanID string, thuTu int, lyDo string, nguoi audit.Actor) error
		DanhDauKhongPhatSinh(ctx context.Context, bienBanID string, thuTu int, nguoi audit.Actor) (
			domain.KetLuanHop, error)
		BoDanhDauKhongPhatSinh(ctx context.Context, bienBanID string, thuTu int, nguoi audit.Actor) error
	}

	// XuLyPhieuPhanAnh is the four STAFF acts, each of which opens a transaction and writes the
	// business change, the audit entry and the notification obligation inside it (rule 6, invariant
	// 3; rule 10, invariant 5).
	//
	// ONE INTERFACE FOR THE FOUR, unlike the two catalogue writers above, and the reason is the
	// opposite of theirs: those two are DIFFERENT CATALOGUES, so one shared interface would need to
	// be told which table to write. These four are four acts on ONE record, they share the locking
	// read and the outbox, and splitting them would be four wiring lines that can disagree about
	// which use case instance — and therefore which transaction boundary — is in play.
	//
	// THE PERMISSIONS ARE NOT IN THE INTERFACE, deliberately: they are declared per route below, at
	// the one place rbac_guard and tools/apidoc both read.
	//
	// ALL FOUR TAKE app.QuyenXemHanChe, AND THE UNIFORMITY IS THE DEFENCE. It is the caller's answer
	// to "does this account hold `feedback.restricted`", and the use case refuses the ACT on a
	// petition in `can-bo` when it is false (app.duocChamPhieuHanChe). A method here without that
	// parameter would be a write path that cannot refuse — which is exactly what all four were until
	// 2026-09-23, while the two READ paths had refused from the day they were written.
	XuLyPhieuPhanAnh interface {
		ChotLinhVuc(ctx context.Context, ma string, yc app.YeuCauChotLinhVuc, nguoi audit.Actor,
			hanChe app.QuyenXemHanChe) (domain.PhieuPhanAnh, error)
		PhanCong(ctx context.Context, ma string, yc app.YeuCauPhanCong, nguoi audit.Actor,
			hanChe app.QuyenXemHanChe) (domain.PhieuPhanAnh, error)
		// TienTrangThai TAKES THE COMMUNE-WIDE RIGHT AS AN ARGUMENT AND Dong DOES NOT — see
		// app.QuyenXuLyCaXa. The holding rule of 2026-09-23 widened the WORKING path only, and the
		// asymmetry in this interface is what makes the closing path impossible to widen by accident.
		// The restricted fact beside it is a DIFFERENT KIND of argument: it can only ever narrow.
		//
		// `ghiChu` on TienTrangThai, Dong and KhongTiepNhan (and GhiChu on the three request structs) is
		// the OPTIONAL internal note stored on the act's logbook row (migration 0013). Free text: it
		// grants nothing, and it is neither of the two permission facts.
		//
		// `attachmentIDs` (09/10/2026) on the three string-signature acts, and `Attachments` on the three
		// request structs: optional completed log attachments linked to the act's timeline row in its
		// transaction (app/act_attachments.go).
		TienTrangThai(ctx context.Context, ma, ghiChu string, nguoi audit.Actor,
			quyen app.QuyenXuLyCaXa, hanChe app.QuyenXemHanChe, attachmentIDs ...string) (domain.PhieuPhanAnh, error)
		Dong(ctx context.Context, ma, ketQua, ghiChu string, nguoi audit.Actor,
			hanChe app.QuyenXemHanChe, attachmentIDs ...string) (domain.PhieuPhanAnh, error)
		// The two terminal branches (user decisions 24-25/09/2026). Same uniformity: both take the
		// restricted fact, so neither can end a report about a member of staff for a colleague.
		KhongTiepNhan(ctx context.Context, ma, lyDo, ghiChu string, nguoi audit.Actor,
			hanChe app.QuyenXemHanChe, attachmentIDs ...string) (domain.PhieuPhanAnh, error)
		ChuyenCapTren(ctx context.Context, ma string, yc app.YeuCauChuyenCapTren, nguoi audit.Actor,
			hanChe app.QuyenXemHanChe) (domain.PhieuPhanAnh, error)
		// The manual internal note (POST …/log-entries). It takes the restricted fact like every act,
		// and its own commune-wide fact — ANY of resolve/assign/classify — which only ever widens to
		// the officers who already work on petitions commune-wide (app.duocGhiChu decides).
		// `attachmentIDs` are the caller's completed log attachments, linked in the note's
		// transaction (optional; nil writes the note alone).
		GhiChuNoiBo(ctx context.Context, ma, ghiChu string, attachmentIDs []string, nguoi audit.Actor,
			quyen app.QuyenGhiChuCaXa, hanChe app.QuyenXemHanChe) (
			domain.NhatKyPhanAnh, []domain.PetitionLogAttachment, error)
		// SetPublication is the staff moderation of the public page (PUT …/publication, ADR 0050 point
		// 8). It takes the restricted fact like every act, so a colleague cannot even hide a report
		// about a member of staff; it moves no lifecycle status.
		SetPublication(ctx context.Context, ma, status string, nguoi audit.Actor,
			hanChe app.QuyenXemHanChe) (domain.PhieuPhanAnh, error)
	}

	// NhatKyPhieuDoc is the READ of one petition's processing logbook, for
	// GET /api/v1/citizen-reports/{maTraCuu}/log-entries.
	//
	// KEYED BY THE PETITION'S INTERNAL id, which the handler has only AFTER it read the petition
	// through PhieuPhanAnhDoc — so the soft-delete and restricted-field checks of that read cannot be
	// skipped by calling this one directly with a lookup code. *petstore.PhieuPhanAnhStore satisfies it.
	NhatKyPhieuDoc interface {
		NhatKyCuaPhieu(ctx context.Context, phieuID string, yc page.Request) (
			page.Result[domain.NhatKyPhanAnh], error)
	}
)

// Deps are everything the routes need. Kept explicit so wiring stays in cmd/server.
// THE PERMISSION ON THE SIX CATALOGUE WRITE ROUTES BELOW IS `admin.lookup`, WRITTEN OUT AT EVERY CALL SITE.
//
// A CONSTANT WOULD READ BETTER AND IS DELIBERATELY NOT USED: tools/apidoc resolves the key from the
// authz.RequirePermission call and refuses anything that is not a string literal there — "khóa
// quyền không phải hằng chuỗi — không ghi vào hợp đồng được". A route whose key it cannot read is a
// route absent from kb/20-contracts/openapi.json, which is the contract the admin web builds
// against (ADR 0014). Six literals that a whole-repository scan checks beat one constant the
// generator cannot see.
//
// `admin.lookup` — "Quản lý danh mục" — IS THE KEY THE SPECIFICATION ALREADY NAMES FOR THIS EXACT
// SCREEN, checked rather than assumed: docs/ui-ux/14-cau-hinh.md:109 lists it in the permission
// matrix, and §5 of that same file (:148-182) is the `Danh mục` tab these routes serve — the one
// with `+ Thêm mục`, the `✎` / `Tắt` / `🗑` actions and the `Nguồn` column. The key is seeded at
// service-identity/migrations/0001_init.sql:274, so a commune administrator can actually tick it.
//
// NO NEW KEY WAS INVENTED, and that is rule 5, invariant 3c: a key no migration seeds is a right
// nobody can grant, so the route would answer 403 to every account forever while the tests stayed
// green. Had `admin.lookup` not existed, the correct move is a finding for open question #27 — not
// an INSERT. tools/check_quyen.py scans the whole repository against the `quyen` table on every
// `make check`.
//
// WHY THE WRITE ROUTES ARE GUARDED WHILE THE READ ROUTE IS AnyAuthenticated: they answer different
// questions. Reading the list is what fills a box on nearly every screen, so a configuration
// permission there would empty those screens for everybody who is not an administrator (the
// argument accepted for GET /api/v1/org-units and for all eight catalogue reads). Changing the list
// is administration of the commune's own configuration, which is precisely what this key is for.

// The WRITE halves of the two reference catalogues this service owns. Two interfaces, two
// obligations, visible at the point of use — see the note on the Deps fields.
type GhiLoaiNhiemVu interface {
	Them(ctx context.Context, yc app.YeuCauThemLoaiNhiemVu, nguoi audit.Actor) (domain.LoaiNhiemVu, error)
	Sua(ctx context.Context, id string, yc app.YeuCauSuaLoaiNhiemVu, nguoi audit.Actor) (domain.LoaiNhiemVu, error)
	Xoa(ctx context.Context, id, lyDo string, nguoi audit.Actor) error
}

type GhiMucUuTien interface {
	Them(ctx context.Context, yc app.YeuCauThemMucUuTien, nguoi audit.Actor) (domain.MucUuTienNhiemVu, error)
	Sua(ctx context.Context, id string, yc app.YeuCauSuaMucUuTien, nguoi audit.Actor) (domain.MucUuTienNhiemVu, error)
	Xoa(ctx context.Context, id, lyDo string, nguoi audit.Actor) error
}

// GhiTrangThaiNhiemVu is the ONE write on task-status wording. There is no Them and no Xoa, and that
// absence is open question #21's answer (ADR 0035 §C): the code list is closed and has no `Tắt`.
type GhiTrangThaiNhiemVu interface {
	Sua(ctx context.Context, ma string, yc app.YeuCauSuaNhanTrangThai, nguoi audit.Actor) (
		domain.TrangThaiHienThi, error)
}

type Deps struct {
	// Checker decides permissions. IT IS NOW LOAD-BEARING: GET /api/v1/citizen-reports/{maTraCuu}
	// declares authz.RequirePermission, and the handler consults the Checker TWICE more — once
	// for the restricted field, once for `feedback.unmask`. A nil one is therefore in the panic
	// switch in Register — see the note there, which predicted exactly this route.
	Checker authz.Checker

	LoaiNhiemVu LoaiNhiemVuDanhMuc
	MucUuTien   MucUuTienDanhMuc

	// The WRITE half of each catalogue, and a second interface per catalogue rather than three
	// more methods on the read one — on purpose. The read is a store call; each of these three
	// opens a TRANSACTION and writes an audit entry inside it (rule 6, invariant 3). Behind one
	// interface a future caller would reach for whichever method was nearest and could end up
	// writing the row outside a transaction, which is the exact defect core/audit was shaped to
	// make impossible.
	GhiLoaiNhiemVu GhiLoaiNhiemVu
	GhiMucUuTien   GhiMucUuTien
	// The Excel imports of the same two catalogues (ADR 0059 §3). See CatalogueImporting.
	TaskTypeImports     CatalogueImporting
	TaskPriorityImports CatalogueImporting

	// The task-status wording: a read of the overrides, and the one write that opens a transaction
	// and audits inside it. Two fields for the reason the catalogue pairs above give.
	TrangThaiNhiemVu    TrangThaiNhiemVuDoc
	GhiTrangThaiNhiemVu GhiTrangThaiNhiemVu

	Phieu       PhieuPhanAnhDoc
	NhanLinhVuc NhanLinhVucDanhMuc

	// PetitionFields is the field catalogue's configuration: tier 1 from platform merged with the
	// commune's tier-2 overrides, and the one edit that opens a transaction and audits inside it.
	PetitionFields PetitionFieldCatalogue

	// The register list and the four staff acts. THE LIST IS SEPARATE FROM `Phieu` on purpose — see
	// PhieuPhanAnhDanhSach — and `XuLyPhieu` is given *store.DB rather than a transaction because
	// opening one is precisely what it is for (rule 6, invariant 3).
	DanhSachPhieu PhieuPhanAnhDanhSach
	XuLyPhieu     XuLyPhieuPhanAnh

	// StaffIntake books a petition on a citizen's behalf ("Nhập hộ phản ánh", docs/ui-ux/09 §11) — a
	// write that opens its transaction and audits inside it (app.StaffIntake), hence its own field.
	StaffIntake StaffIntakeBooker

	// The processing logbook's READ (migration 0013). Its one write, the manual note, is on
	// XuLyPhieu because it opens a transaction and audits inside it, like the six acts.
	NhatKyPhieu NhatKyPhieuDoc

	// The TASK register — two read paths over one store, see NhiemVuDoc.
	NhiemVu         NhiemVuDoc
	DanhSachNhiemVu NhiemVuDanhSach

	// Identity's two answers behind `soon=true` and `scope=related` on the task list and counts.
	TaskFilterIdentity TaskFilterIdentity

	// The Sổ theo dõi export — reads, resolves names, renders, and audits (app.TaskRegisterExport).
	TaskRegisterExport TaskRegisterExporter

	// The spreadsheet import — checks every row, then books them all in one transaction (app.TaskImport).
	TaskImport TaskImporter

	// The approval queue of extension requests (§5.8) — a read over `de_nghi_lui_han` joined to its
	// task. The two extension WRITES stay on GhiNhiemVu below.
	DeNghiChoDuyet DeNghiLuiHanChoDuyetDoc

	// The SIX WRITE acts on a task (migrations 0006 and 0008). A THIRD field rather than methods on
	// either interface above, and for the reason the catalogue pair states: a read is a store call,
	// while each of these opens a TRANSACTION and writes an audit entry inside it. Behind one
	// interface a future caller would reach for whichever method was nearest, and could end up
	// writing the row outside a transaction — the exact defect core/audit was shaped to prevent.
	GhiNhiemVu GhiNhiemVuUseCase

	// PetitionTasks books a task FROM a petition (POST /api/v1/citizen-reports/{maTraCuu}/tasks,
	// 30/09/2026) — through GhiNhiemVu's own create path, with the petition checked and its timeline
	// written in the same transaction (app.PetitionTaskCreation).
	PetitionTasks PetitionTaskCreator

	// CitizenLetterTasks books a task FROM a citizen letter (POST /api/v1/citizen-letter-tasks, ADR 0085) —
	// documents asked first, then GhiNhiemVu's own create path (app.CitizenLetterTaskCreation).
	CitizenLetterTasks CitizenLetterTaskCreator

	// §5.9's attachments: the three acts (app.TaskAttachments — built even when object storage, the
	// scanner or platform's limits are not configured; its routes then answer 503 and nothing else
	// changes), and the batched read the timeline renders them from.
	TaskAttachments    TaskAttachmentActs
	TaskLogAttachments TaskLogAttachmentReader

	// PetitionPhotos lists a petition's citizen scene photos with signed links for "TRƯỚC KHI XỬ LÝ"
	// (app.StaffPetitionPhotos — built even without object storage; its route then answers 503).
	PetitionPhotos StaffPetitionPhotos
	// VerificationPhotos is "SAU KHI XỬ LÝ": staff's upload, completion and audited list
	// (app.StaffVerificationPhotos — built even without object storage; its routes then answer 503).
	VerificationPhotos StaffVerificationPhotoActs
	// §8.7's `📎 Đính kèm` on the petition log: the three acts (app.PetitionLogAttachments) and the
	// batched read the timeline renders them from. STAFF-ONLY — nothing on the citizen Deps reaches them.
	PetitionLogAttachments       PetitionLogAttachmentActs
	PetitionLogAttachmentsReader PetitionLogAttachmentReader

	// The MEETING MINUTES register — one read path in this pass (migration 0007). It is the ORIGIN
	// of the tasks above: a conclusion is split into a task and the task keeps a permanent back-link
	// to it through `nguon_giao`/`nguon_id`, which is why both registers live in this service.
	DanhSachBienBan BienBanDanhSach

	// The THREE WRITE acts on the meeting register. A SEPARATE field from the read one, and for the
	// reason every pair above states: a read is a store call, while recording minutes opens a
	// TRANSACTION and writes an audit entry inside it. Behind one interface a future caller would
	// reach for whichever method was nearest — and could end up writing the row outside a
	// transaction, the exact defect core/audit was shaped to prevent.
	GhiBienBan GhiBienBanUseCase

	// Vet writes the trail for a full-view read. Required, not optional — see the panic switch.
	Vet VetXemNguoiGui

	// THE LEADERSHIP OVERVIEW (/tong-quan) — summary.go. The two summaries are store reads (counts
	// over one register, one statement each); the queue is a use case because its `critical` flag is
	// asked of identity's working-hours calendar.
	TaskSummary          TaskSummaryReader
	CitizenReportSummary CitizenReportSummaryReader
	OverdueQueue         OverdueQueueReader
	// The /phan-anh statistics read (09/10/2026) — a use case, it asks identity's calendar
	// (app.CitizenReportBreakdown). routes_citizen_report_figures.go refuses it nil.
	CitizenReportBreakdown CitizenReportBreakdownReader

	// MERGING DUPLICATE PETITIONS (ADR 0087) — routes_citizen_report_merge.go refuses all three nil. The
	// two acts open a transaction (the XuLyPhieu instance in production); the two reads do not — the
	// detail's link codes are a store read, the suspected-duplicate search a use case over the store.
	CitizenReportMerge  CitizenReportMerging
	MergeLinks          CitizenReportMergeLinks
	DuplicateCandidates CitizenReportDuplicateCandidates

	// ResidentialUnitNames names the thôn / tổ dân phố a petition carries (ADR 0088) on the staff detail,
	// list and write responses — *identityclient.Client in production. nil is NOT refused at construction
	// (most routes never need it); a read that needs a name then answers 503, never a blank name.
	ResidentialUnitNames app.ResidentialUnitNamer

	// AuditLog reads this service's own `audit_log` for the "Xem nhật ký hệ thống" screen (ADR 0054),
	// withholding `can-bo` petitions' entries from a reader without `feedback.restricted` (§4).
	// *audit.Log in production. Refused at construction when missing.
	AuditLog AuditLogReader

	// SystemMessages is "Lời hệ thống" for the sentences this service raises (migration 0020): the
	// three configuration routes, AND the read every configurable refusal branch sends its sentence
	// through (refusalMessageKeys). *app.SystemMessages in production. Refused at construction when
	// missing — a nil one would panic inside a refusal, on the path meant to explain a mistake.
	SystemMessages SystemMessageService

	// UploadSlots is the PROCESS-WIDE cap on uploads in flight (httpx.UploadSlotsPerPod, ADR 0052
	// §Sửa đổi 09/10/2026) — the SAME value DepsCongDan.UploadSlots holds, built once in cmd/server: a
	// cap per mux would let the two surfaces hold twice the uploads the pod is sized for. Refused at
	// construction when missing.
	UploadSlots *httpx.UploadSlots

	Log *slog.Logger
}

// Register mounts the petitions routes.
//
// Add every new route with its permission declaration in the SAME statement, never on a nearby
// line: rbac_guard anchors to the statement, and so should a reader.
func Register(mux *http.ServeMux, d Deps) {
	// Refusing incomplete wiring HERE, at construction, not at request time: a route mounted
	// without its store would accept requests it cannot answer, and the first person to find out
	// would be a member of staff in front of a government screen. The process failing to start is
	// loud, happens before any commune is served, and names the missing piece.
	//
	// d.Checker IS NOW IN THE SWITCH, and that note used to say the opposite for a reason that
	// has expired: it said a nil Checker guarded nothing because no route declared a permission,
	// and it said THE FIRST RequirePermission ROUTE MUST ADD THE CASE. The petition read route
	// below is that route. authz.RequirePermission(nil, …) panics when somebody CALLS it — a
	// panic on a member of staff's screen — whereas this one happens before any commune is served
	// and names the missing piece.
	switch {
	case d.LoaiNhiemVu == nil:
		panic("petitions/http: thiếu kho loại nhiệm vụ — GET /api/v1/task-types sẽ panic khi có người gọi")
	case d.MucUuTien == nil:
		panic("petitions/http: thiếu kho mức ưu tiên — GET /api/v1/task-priorities sẽ panic khi có người gọi")
	case d.GhiLoaiNhiemVu == nil:
		panic("petitions/http: thiếu use case ghi danh mục loại nhiệm vụ — POST/PATCH/DELETE /api/v1/task-types sẽ panic khi có người gọi")
	case d.GhiMucUuTien == nil:
		panic("petitions/http: thiếu use case ghi danh mục mức ưu tiên — POST/PATCH/DELETE /api/v1/task-priorities sẽ panic khi có người gọi")
	case d.TaskTypeImports == nil:
		panic("petitions/http: thiếu use case nhập Excel loại nhiệm vụ — ba tuyến /api/v1/task-types/import* sẽ panic khi có người gọi")
	case d.TaskPriorityImports == nil:
		panic("petitions/http: thiếu use case nhập Excel mức ưu tiên — ba tuyến /api/v1/task-priorities/import* sẽ panic khi có người gọi")
	case d.TrangThaiNhiemVu == nil:
		panic("petitions/http: thiếu kho nhãn trạng thái nhiệm vụ — GET /api/v1/task-statuses sẽ panic khi có người gọi")
	case d.GhiTrangThaiNhiemVu == nil:
		panic("petitions/http: thiếu use case ghi nhãn trạng thái nhiệm vụ — PATCH /api/v1/task-statuses/{code} sẽ panic khi có người gọi")
	case d.Checker == nil:
		panic("petitions/http: thiếu Checker — GET /api/v1/citizen-reports/{maTraCuu} khai quyền feedback.read, " +
			"và một Checker rỗng sẽ panic lúc có cán bộ gọi chứ không phải lúc khởi động")
	case d.Phieu == nil:
		panic("petitions/http: thiếu kho phiếu phản ánh — GET /api/v1/citizen-reports/{maTraCuu} sẽ panic khi có người gọi")
	case d.NhanLinhVuc == nil:
		panic("petitions/http: thiếu kho nhãn lĩnh vực — GET /api/v1/citizen-reports/{maTraCuu} sẽ panic khi có người gọi")
	case d.PetitionFields == nil:
		panic("petitions/http: thiếu use case danh mục lĩnh vực — GET/PATCH /api/v1/citizen-report-fields sẽ panic khi có người gọi")
	case d.DanhSachPhieu == nil:
		panic("petitions/http: thiếu đường đọc danh sách phiếu — GET /api/v1/citizen-reports sẽ panic khi có người gọi")
	case d.XuLyPhieu == nil:
		// THE FOUR WRITE ROUTES AT ONCE. A nil here does not break a screen quietly: it breaks every
		// act a member of staff can perform on a petition, which means petitions come in and nothing
		// can be done with them — the exact state this service was in before these routes existed.
		panic("petitions/http: thiếu use case xử lý phiếu — bốn tuyến phân loại/phân công/chuyển trạng thái/đóng phiếu sẽ panic khi có người gọi")
	case d.StaffIntake == nil:
		panic("petitions/http: thiếu use case nhập hộ phản ánh — POST /api/v1/citizen-reports sẽ panic khi có người gọi")
	case d.NhatKyPhieu == nil:
		panic("petitions/http: thiếu đường đọc nhật ký xử lý phiếu — GET /api/v1/citizen-reports/{maTraCuu}/log-entries sẽ panic khi có người gọi")
	case d.NhiemVu == nil:
		panic("petitions/http: thiếu kho nhiệm vụ — GET /api/v1/tasks/{ma} sẽ panic khi có người gọi")
	case d.DanhSachNhiemVu == nil:
		panic("petitions/http: thiếu đường đọc danh sách nhiệm vụ — GET /api/v1/tasks sẽ panic khi có người gọi")
	case d.TaskFilterIdentity == nil:
		panic("petitions/http: thiếu đường hỏi identity cho bộ lọc nhiệm vụ — `soon=true` và `scope=related` " +
			"trên GET /api/v1/tasks sẽ panic khi có người gọi")
	case d.TaskRegisterExport == nil:
		panic("petitions/http: thiếu use case xuất sổ theo dõi — GET /api/v1/tasks/register-export sẽ panic khi có người gọi")
	case d.TaskImport == nil:
		panic("petitions/http: thiếu use case nhập nhiệm vụ từ Excel — POST /api/v1/tasks/imports sẽ panic khi có người gọi")
	case d.DeNghiChoDuyet == nil:
		panic("petitions/http: thiếu đường đọc hàng chờ duyệt lùi hạn — GET /api/v1/task-extensions sẽ panic khi có người gọi")
	case d.GhiNhiemVu == nil:
		// THE SIX WRITE ROUTES AT ONCE. A nil here does not break one screen: it breaks giao việc,
		// sửa, chuyển trạng thái, xoá and both halves of lùi hạn — which puts the task register back
		// in the state it was in before this pass, readable and unchangeable, while four other
		// subsystems stand on it.
		panic("petitions/http: thiếu use case ghi nhiệm vụ — sáu tuyến giao việc/sửa/chuyển trạng thái/xoá/lùi hạn sẽ panic khi có người gọi")
	case d.PetitionTasks == nil:
		panic("petitions/http: thiếu use case tạo nhiệm vụ từ phiếu — POST /api/v1/citizen-reports/{maTraCuu}/tasks sẽ panic khi có người gọi")
	case d.CitizenLetterTasks == nil:
		panic("petitions/http: thiếu use case chuyển đơn thư thành nhiệm vụ — POST /api/v1/citizen-letter-tasks sẽ panic khi có người gọi")
	case d.TaskAttachments == nil:
		panic("petitions/http: thiếu use case tệp đính kèm nhiệm vụ — các tuyến /api/v1/tasks/{ma}/attachments sẽ panic khi có người gọi")
	case d.PetitionPhotos == nil:
		panic("petitions/http: thiếu đường đọc ảnh hiện trường — GET /api/v1/citizen-reports/{maTraCuu}/photos sẽ panic khi có người gọi")
	case d.VerificationPhotos == nil:
		panic("petitions/http: thiếu use case ảnh sau xử lý — hai tuyến /api/v1/citizen-reports/{maTraCuu}/verification-photos sẽ panic khi có người gọi")
	case d.PetitionLogAttachments == nil:
		panic("petitions/http: thiếu use case tệp đính kèm nhật ký phiếu — các tuyến /api/v1/citizen-reports/{maTraCuu}/log-attachments sẽ panic khi có người gọi")
	case d.PetitionLogAttachmentsReader == nil:
		panic("petitions/http: thiếu đường đọc tệp đính kèm của nhật ký phiếu — GET /api/v1/citizen-reports/{maTraCuu}/log-entries sẽ panic khi có người gọi")
	case d.TaskLogAttachments == nil:
		panic("petitions/http: thiếu đường đọc tệp đính kèm của nhật ký — GET /api/v1/tasks/{ma}/log-entries sẽ panic khi có người gọi")
	case d.DanhSachBienBan == nil:
		panic("petitions/http: thiếu đường đọc danh sách biên bản họp — GET /api/v1/meetings sẽ panic khi có người gọi")
	case d.GhiBienBan == nil:
		// THE THREE WRITE ROUTES AT ONCE. A nil here leaves the meeting register readable and
		// unfillable — and with it §3, the ONE path by which a conclusion becomes a task and keeps a
		// back-link to where it came from.
		panic("petitions/http: thiếu use case ghi biên bản họp — ba tuyến nhập biên bản/thêm kết luận/tách nhiệm vụ sẽ panic khi có người gọi")
	case d.Vet == nil:
		// THE MOST DANGEROUS OF THE FIVE TO LEAVE OUT, because a nil here does not crash a screen:
		// it crashes the ONE path that discloses a citizen's name and number, and only when
		// somebody with `feedback.unmask` reads a petition. Refusing at startup is what keeps
		// "every full read leaves a trail" (rule 6, invariant 7) from depending on a wiring line
		// nobody re-reads.
		panic("petitions/http: thiếu đường ghi vết xem đầy đủ — quyền feedback.unmask mở họ tên và số " +
			"điện thoại người gửi, và luật 6 bất biến 7 không cho phép đọc đầy đủ mà không ghi vết")
	case d.TaskSummary == nil:
		panic("petitions/http: thiếu đường đếm tổng quan nhiệm vụ — GET /api/v1/task-summary sẽ panic khi có người gọi")
	case d.CitizenReportSummary == nil:
		panic("petitions/http: thiếu đường đếm tổng quan phản ánh — GET /api/v1/citizen-report-summary sẽ panic khi có người gọi")
	case d.OverdueQueue == nil:
		panic("petitions/http: thiếu use case hàng đợi quá hạn — GET /api/v1/overdue-tasks và " +
			"GET /api/v1/overdue-citizen-reports sẽ panic khi có người gọi")
	case d.AuditLog == nil:
		panic("petitions/http: thiếu bộ đọc nhật ký hệ thống — GET /api/v1/petitions-audit-entries sẽ panic khi có người gọi")
	case d.SystemMessages == nil:
		panic("petitions/http: thiếu use case lời hệ thống — các tuyến /api/v1/petitions-system-messages và " +
			"mọi câu từ chối xã được sửa lời sẽ panic khi có người gọi")
	case d.UploadSlots == nil:
		panic("petitions/http: thiếu bộ giới hạn lượt tải lên cùng lúc (httpx.UploadSlots) — ba tuyến tải tệp " +
			"của cán bộ không được phục vụ khi không có giới hạn (ADR 0052 §Sửa đổi 09/10/2026)")
	}

	h := NewHandler(d)

	// The two task catalogues' Excel imports — six routes, all `admin.lookup` (routes_catalogue_import.go).
	registerCatalogueImportRoutes(mux, d, h)
	// The /phan-anh statistics and the log-attachment removal — four routes (routes_citizen_report_figures.go).
	registerCitizenReportFigureRoutes(mux, d, h)
	// Merging duplicate petitions (ADR 0087) — three routes (routes_citizen_report_merge.go).
	registerCitizenReportMergeRoutes(mux, d, h)

	// --- the commune's task catalogues. TWO READ ROUTES, AND DELIBERATELY NO WRITE ROUTE --------
	//
	// `task-types` and `task-priorities` — English, plural, kebab-case, under /api/v1. The nouns
	// are the entities migration 0003 declares (`-- @entity: TaskType`, `-- @entity: TaskPriority`),
	// so the table, the Go type and the URL all say the same thing about the same data. Nothing is
	// translated on the spot here.
	//
	// The task-STATUS wording is not a 0003 catalogue: #21 (ADR 0035 §C) closed its code list, so it
	// is an override table (migration 0010) with a read and a single PATCH — see task-statuses below.
	//
	// AnyAuthenticated ON BOTH, AND THE REASON IS THE SHAPE OF THE DATA'S USE — the same reading
	// service-identity states on GET /api/v1/org-units and GET /api/v1/roles. These names fill the
	// picker on the task form and the filter bar of nearly every task screen, so requiring a
	// configuration permission would not protect anything: it would empty those pickers for
	// everybody who is not an administrator. There is nothing sensitive in the list of words an
	// authority uses to sort its own work.
	//
	// THE TRADE-OFF, STATED RATHER THAN GLOSSED: a commune's own catalogues are readable by every
	// signed-in account OF THAT COMMUNE. They are NOT readable across communes and cannot be —
	// store.Scoped.Query binds `tenant_id` from the context (rule 1, invariant 5), and the same
	// request carrying another commune's token is refused at the token layer before any query runs
	// (authz compares the token's commune with the one resolved from Host).
	//
	// NO idem.* DECLARATION on either: a GET changes no state, and declaring a duplicate-request
	// protection here would claim a protection with nothing to protect.

	// @summary  Danh mục loại nhiệm vụ của xã — dùng cho ô chọn loại trên biểu mẫu nhiệm vụ và bộ lọc; mỗi dòng kèm cờ chỉ đọc `requires_directive` (đúng với `theo-van-ban`)
	// @screen   14-cau-hinh §5
	// `requires_directive` is DERIVED from the code (domain.LoaiNhiemVu.RequiresDirective), never stored
	// and never writable — the write bodies below do not declare it.
	//
	// 500 covers two causes and says so honestly: an ordinary store failure, and the commune's
	// catalogue exceeding petstore.TranDanhMucLoaiNhiemVu — which this route REFUSES rather than
	// truncating, because a silently short list is a missing option in a form.
	//
	// 401 is authz.AnyAuthenticated's answer to "no session", and there is no 403 on this route
	// because there is no permission to fail.
	//
	// IT IS ALSO THE ANSWER TO A SESSION FROM ANOTHER COMMUNE, BUT NOT FOR THE REASON IT LOOKS.
	// AnyAuthenticated's own commune check cannot fail here: core/staffauth stamps the Host
	// commune onto the principal it builds (staffauth.go:157 and :241), so xacNhanXa compares a
	// value with itself. The comparison that decides happens inside identity, at
	// service-identity/internal/grpc/server.go:257, where the commune from `x-tenant-id` meets
	// the commune INSIDE the credential — the only place both values exist. A mismatch there
	// yields no principal, so this route answers 401 for ABSENCE, not for disagreement.
	//
	// The wall that is local and does hold alone: Scoped.Query binds `tenant_id` from the context
	// (rule 1, invariant 5), so neither query could read another commune's rows in any case.
	//
	// @reply    200 danhSachLoaiNhiemVuRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/task-types",
		authz.AnyAuthenticated("tên loại nhiệm vụ xuất hiện ở ô chọn trên biểu mẫu nhiệm vụ và ở bộ lọc của hầu hết màn hình nhiệm vụ — đòi một quyền cấu hình sẽ làm rỗng những ô đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: danh mục của xã lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachLoaiNhiemVu)))

	// THE ORDER OF `items` IS THE CONTRACT HERE, not a detail: priority is a SCALE, and the store
	// returns it ranked (ORDER BY thu_tu). That is why the reply carries no `order` field — one
	// fact, one representation (rule 9).
	//
	// @summary  Danh mục mức ưu tiên nhiệm vụ của xã, theo đúng thứ tự thang — dùng cho ô chọn và bộ lọc
	// @screen   14-cau-hinh §5
	// 500 covers an ordinary store failure and the commune's scale exceeding
	// petstore.TranDanhMucMucUuTien — REFUSED rather than truncated, because a truncated scale
	// loses the levels at one end of it and every task filed from it is ranked wrong.
	//
	// 401, and no 403, for the same reason as the route above.
	//
	// @reply    200 danhSachMucUuTienRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/task-priorities",
		authz.AnyAuthenticated("tên mức ưu tiên xuất hiện ở ô chọn trên biểu mẫu nhiệm vụ và ở bộ lọc của hầu hết màn hình nhiệm vụ — đòi một quyền cấu hình sẽ làm rỗng những ô đó cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: thang ưu tiên của xã lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachMucUuTien)))

	// --- the commune's wording and order for the seven task statuses (#21, ADR 0035 §C) ----------
	//
	// ALL SEVEN CODES, ALWAYS: the commune's override where it has one, the default
	// (domain.MacDinhTrangThaiNhiemVu) where it has not. Sorted by effective order, ties by default
	// order. AnyAuthenticated for the reason the two catalogue reads above give: these labels are the
	// Kanban column headers and the status filter of every task screen.
	//
	// 500 when the overrides cannot be read — REFUSED rather than answered with the defaults, which
	// would silently show a commune wording it replaced. 401, no 403: there is no permission to fail.
	//
	// @summary  Bảy trạng thái nhiệm vụ với nhãn và thứ tự của xã (mặc định nếu xã chưa sửa) — cột Kanban, bộ lọc, màn hình cấu hình
	// @screen   02-nhiem-vu §6
	// @reply    200 danhSachTrangThaiNhiemVuRa
	// @reply    401 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/task-statuses",
		authz.AnyAuthenticated("nhãn trạng thái là tiêu đề cột Kanban và giá trị bộ lọc trạng thái của mọi màn hình nhiệm vụ — đòi một quyền cấu hình sẽ làm mất tên cột cho mọi tài khoản không phải quản trị; đánh đổi đã chấp nhận: cách gọi trạng thái của xã lộ cho mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ, không chéo xã vì Scoped buộc tenant_id")(
			http.HandlerFunc(h.DanhSachTrangThaiNhiemVu)))

	// Re-word or re-order ONE status. `admin.lookup` — the same key as the six catalogue writes (see
	// the block above Register): this is the same `Danh mục` configuration of the commune.
	//
	// 404 for a code outside the seven — the list is closed (#21). 400 for a blank label, a label
	// over 100 characters (counted in characters, as the CHECK counts), an order below 1, or a body
	// naming `code` or `active` (no rename, no `Tắt`).
	//
	// idem.KhongCan: the use case writes nothing and audits nothing when neither field moves, and the
	// upsert stores absolute values — so the same request sent twice leaves one row, one entry.
	//
	// @summary  Sửa nhãn và/hoặc thứ tự hiển thị của một trạng thái nhiệm vụ trong xã (không thêm, xoá hay tắt mã)
	// @screen   14-cau-hinh §5
	// @request  suaTrangThaiNhiemVuVao
	// @reply    200 trangThaiNhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/task-statuses/{code}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("upsert ghi giá trị tuyệt đối và use case không ghi, không để vết khi không trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaTrangThaiNhiemVu))))

	// --- the commune's petition field catalogue (ADR 0026 tier 2 over ADR 0060 tier 1) ------------
	//
	// `admin.lookup` ON BOTH, including the read. It is the `Danh mục` configuration key the task
	// catalogues and task-status wording already use (see the block above Register). The read is NOT
	// AnyAuthenticated: it lists disabled and retired codes, which is the configuration screen's
	// business and no other screen's. Opening it to every account is rule 5 stop condition #1 and has
	// not been asked — the classification drop-down needs its own answer.
	//
	// 503 `field_catalogue_unavailable` when platform cannot be read and the cached answer is older
	// than 60 s — refused, never a built-in list and never raw codes (ADR 0060 §3).
	//
	// @summary  Danh mục lĩnh vực phản ánh của xã — đủ mọi mã nền tảng cấp, kèm nhãn, thứ tự, bật/tắt của xã (màn hình cấu hình)
	// @screen   14-cau-hinh
	// @reply    200 petitionFieldListOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/citizen-report-fields",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			http.HandlerFunc(h.ListPetitionFields)))

	// Re-word, re-order, or switch on/off ONE code for this commune. Sending the default label or
	// order goes back to inheriting it (stored as NULL / 0, migration 0022). Switching off hides the
	// code from the citizen's NEW-SUBMISSION form only; every read path keeps it (ADR 0026).
	//
	// 404 for a code tier 1 does not know. 400 for an empty body, a blank or over-100-character label,
	// an order outside 1..9999, or a body naming `code` (a code is never renamed).
	//
	// idem.KhongCan: the upsert stores absolute values and a no-op writes and audits nothing, so the
	// same request twice leaves one row and one entry.
	//
	// @summary  Sửa nhãn, thứ tự hiển thị hoặc bật/tắt một lĩnh vực phản ánh trong xã (không thêm, không xoá mã)
	// @screen   14-cau-hinh
	// @request  updatePetitionFieldIn
	// @reply    200 petitionFieldOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("PATCH /api/v1/citizen-report-fields/{code}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("upsert ghi giá trị tuyệt đối và use case không ghi, không để vết khi không trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.UpdatePetitionField))))

	// The fields the STAFF INTAKE modal offers ("Nhập hộ phản ánh", docs/ui-ux/09 §11; ADR 0028 Bổ sung
	// 2026-10-02 row 4): active on the platform and enabled by the commune, `can-bo` included — EXACTLY
	// the predicate POST /api/v1/citizen-reports checks the picked field against
	// (app.PetitionFieldCatalogue.StaffIntakeCatalogue / CheckStaffIntakeField), so a code the modal
	// shows is a code the write accepts. A route of its own rather than the configuration read above:
	// that one is `admin.lookup` and lists switched-off and retired codes, which the modal must not offer.
	//
	// `feedback.create` ("Tiếp nhận phản ánh", seeded at service-identity/migrations/0001_init.sql:298) —
	// the key of the write this list fills, so whoever may book may pick, and nobody else is handed a
	// list they cannot use. The citizen counterpart is GET /api/v1/my-citizen-report-fields; same shape.
	//
	// 401 no session AND a session of another commune (authz compares the commune before the key). 403
	// no `feedback.create`, or the key granted in another commune. 503 `field_catalogue_unavailable` when
	// platform cannot be read and the cached answer is older than 60 s — never a built-in list (ADR 0060 §3).
	// No idem.*: a GET changes no state.
	//
	// @summary  Lĩnh vực phản ánh xã đang nhận cho ô chọn của biểu mẫu cán bộ nhập hộ — mã đang dùng trên nền tảng và đang bật ở xã (gồm cả `can-bo`), theo thứ tự của xã
	// @screen   09-phan-anh-nguoi-dan §11
	// @reply    200 citizenFieldListOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/citizen-report-intake-fields",
		authz.RequirePermission(d.Checker, "feedback.create")(
			http.HandlerFunc(h.ListStaffIntakeFields)))

	// --- the petition register ---------------------------------------------------------------
	//
	// `citizen-reports` is the settled URL noun for `phan_anh`
	// (kb/00-foundation/ubiquitous-language.md §Tên tài nguyên trên URL). It is NOT `feedback`:
	// feedback means product suggestions, while a phản ánh is a class of administrative
	// submission with a processing deadline and somebody answerable for it. The permission keys
	// say `feedback.*` because they were fixed earlier, and that mismatch is a known, recorded
	// cost rather than a licence to spell the resource the same way.

	// NHẬP HỘ PHẢN ÁNH — staff intake (docs/ui-ux/09 §11). The two decisions this comment used to wait
	// for are taken: the modal wording was APPROVED by the user on 30/09/2026 (spec 09:248-249 — "Vì đã
	// biết lĩnh vực ngay, hạn xử lý được ấn định luôn", replacing "cùng thời hạn với phiếu gửi từ
	// Zalo", per ADR 0028 E/F). The residential unit (`residential_unit_id`, ADR 0088) is optional and
	// checked with identity's ResolveActiveResidentialUnits before anything is written. Scene photos are
	// out: that flow is bound to a citizen session. app.StaffIntake carries ADR 0028 E/F decision by
	// decision.
	//
	// `feedback.create` ("Tiếp nhận phản ánh"), seeded at service-identity/migrations/0001_init.sql:298
	// and named by spec 09 §14.6. The petition is linked to NO citizen account (ADR 0028 §Bổ sung
	// 2026-10-02): the officer hands over the lookup code in the 201.
	//
	// `channel` IS NOT A BODY FIELD although §11 draws a "Tiếp nhận qua kênh" select defaulting to `Cán
	// bộ, trưởng thôn nhập hộ`. Every row booked here is `can-bo-nhap-ho`: another value would change
	// which deadline shape applies (ADR 0028 E, stop condition #6), and nobody has said whether a call
	// logged as `zalo-oa` or `web-xa` from this modal gets the field-at-booking shape. A body naming it
	// is a 400.
	//
	// idem.Required(idem.MoKhiHong): a double click books a SECOND petition with a second code, which
	// can only be soft deleted (rule 7). MoKhiHong, the citizen intake's choice: refusing an officer
	// with a citizen on the telephone because a cache is down is worse than a rare visible duplicate.
	// A replay is told the code (idem.RecordCode).
	//
	// 400 `invalid_request` (shape, missing field/content, lengths, a system-decided field, coordinates
	// sent, the unit under `hamlet`/`thon_id`) · `field_not_offered` · `residential_unit_not_offered` ·
	// `clock_from_out_of_range` (ADR 0028 F3 — refused, never clamped). 401 no session AND a session of
	// another commune (authz compares the commune before the key). 403 no `feedback.create`. 409
	// `request_in_progress` (idem). 503 `intake_not_configured` (identity could not fix the resolve
	// deadline — no SLA table, calendar, or identity down) · `field_catalogue_unavailable` ·
	// `residential_unit_check_unavailable`. Nothing is written and no code is issued on any refusal.
	//
	// @summary  Cán bộ nhập hộ một phản ánh của người dân (gọi điện, ghé trụ sở, gặp trưởng thôn) — lĩnh vực bắt buộc, hạn xử lý ấn định ngay, không gắn tài khoản người dân, trả mã tra cứu
	// @screen   09-phan-anh-nguoi-dan §11
	// @request  staffIntakeIn
	// @reply    201 phieuPhanAnhRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports",
		authz.RequirePermission(d.Checker, "feedback.create")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.BookStaffIntake))))

	// @summary  Một phiếu phản ánh, tra theo mã tra cứu đã trả cho người dân
	// @screen   09-phan-anh-nguoi-dan §8
	// 404 is the single answer to four different causes — no such code, another commune's code, a
	// soft-deleted petition, and a petition in the restricted field `can-bo` read by somebody
	// without `feedback.restricted`. The reasoning for folding the last one in is on
	// Handler.khongTimThay: a 403 there would confirm to a colleague that a report about them
	// exists.
	//
	// 403 is RequirePermission's answer to an account without `feedback.read`. 401 is its answer
	// to no session AND to a session issued by another commune — those are one answer because
	// authz.xacNhanXa refuses before the permission is ever consulted (rule 5, invariant 3), so
	// no query runs and the response cannot differ by commune.
	//
	// A SECOND AND A THIRD PERMISSION ARE CONSULTED INSIDE THE HANDLER, and neither changes the
	// status code — which is why neither can be read off this block and both are stated here:
	//
	//	feedback.restricted  absent, on a `can-bo` petition -> 404, identical to an unknown code
	//	feedback.unmask      absent                         -> 200 with the reporter MASKED
	//	feedback.unmask      present                        -> 200 with the reporter in full,
	//	                                                        and one audit entry written first
	//
	// 500 therefore covers one more cause than it did: the audit entry for a full-view read
	// failing to commit. Rule 6 gives no other answer — no trail, no disclosure.
	//
	// 503 `field_catalogue_unavailable`: the petition carries a field and its label (commune wording,
	// else platform default) cannot be read — refused rather than showing a raw code (ADR 0060 §3).
	//
	// @reply    200 phieuPhanAnhRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/citizen-reports/{maTraCuu}",
		authz.RequirePermission(d.Checker, "feedback.read")(
			http.HandlerFunc(h.DocPhieuPhanAnh)))

	// ẢNH HIỆN TRƯỜNG — "TRƯỚC KHI XỬ LÝ" on the petition detail (docs/ui-ux/09 §8.4): the photos the
	// citizen attached (internal/app/petition_photo.go), each with a presigned GET of at most 15 minutes.
	//
	// `feedback.read` — THE KEY THE PETITION DETAIL ITSELF DECLARES, two statements up, seeded in
	// service-identity/migrations/0001_init.sql (rule 5, invariant 3c: NO KEY INVENTED). The owner's
	// words: "cán bộ có quyền đọc phiếu xem được". `feedback.restricted` is consulted inside, as on the
	// detail: a `can-bo` petition without it is the detail's own 404.
	//
	// AUDITED (rule 6, invariant 7): a photo cannot be masked, so every list that hands out links is a read
	// of full personal data — one entry per call, by the officer's business code, committed before the
	// reply (app.StaffPetitionPhotos.ListPhotos). `Cache-Control: no-store`.
	//
	// @summary  Ảnh hiện trường người dân gửi kèm một phiếu phản ánh, mỗi ảnh kèm liên kết xem có ký, sống tối đa 15 phút
	// @screen   09-phan-anh-nguoi-dan §8.4
	// 200 WITH `items: []` when the citizen attached none. 404 is the detail's four causes, one body.
	// 401 is no session AND a session of another commune (authz.xacNhanXa, before the key is consulted).
	// 503 `storage_not_configured`: no object store to sign links against.
	//
	// @reply    200 photoListOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/citizen-reports/{maTraCuu}/photos",
		authz.RequirePermission(d.Checker, "feedback.read")(
			http.HandlerFunc(h.ListPetitionPhotos)))

	// ẢNH SAU XỬ LÝ — "SAU KHI XỬ LÝ" on the petition detail (docs/ui-ux/09 §8.4), owner decisions of
	// 02/10/2026 (ADR 0047 row "Ảnh 'sau xử lý' của cán bộ — THAY G8"; ADR 0008 decision 3). ONE
	// multipart upload through this service (ADR 0052 §Sửa đổi 09/10/2026), the scene photo's steps for a
	// member of staff (internal/app/petition_verification_photo.go): at most 5 per petition per round
	// (platform's `petition-verification-photo` policy, migration 0027's floor), JPEG / PNG / WebP,
	// RE-ENCODED WITHOUT EXIF (the citizen sees it), private, class records. Answered with the STORED
	// photo; there is no completion route any more.
	//
	// `verification-photos`: "verification" is the glossary's English for nghiệm thu
	// (kb/00-foundation/ubiquitous-language.md, core/storage PurposePetitionVerificationPhoto) — the act
	// this photo evidences. A SIBLING of `photos`, not a `kind` on it: the two have different uploaders,
	// different keys, different counts, and the citizen's list must never grow a staff file by default.
	//
	// `feedback.resolve` ON THE WRITE — the key that guards the closing (open question #7), because this
	// photo IS the closing's evidence; `feedback.read` on the list, the detail's own key. Both seeded
	// (service-identity/migrations/0001_init.sql); NO KEY WAS INVENTED (rule 5, invariant 3c).
	// `feedback.restricted` narrows inside: a `can-bo` petition without it is the detail's 404.
	//
	// WHEN: every status but the three endings (domain.VerificationPhotoUploadOpen).
	//
	// THE BODY IS multipart/form-data (core/httpx.ReadUpload): text fields FIRST — `size` (required, the
	// byte count, ≤ the policy's cap) and `content_type` (optional; else the file part's Content-Type;
	// image/jpeg · image/png · image/webp) — then exactly ONE part named `file`, then nothing. The part's
	// file name is ignored. At most httpx.UploadSlotsPerPod uploads per pod at once, httpx.UploadReadTimeout
	// per upload.
	//
	// idem.Required(idem.MoKhiHong): a replay answers the FILE ID (`{"code": "<id>", "replayed": true}`),
	// never a second file.
	//
	// @summary  Cán bộ tải lên MỘT ảnh sau xử lý cho phiếu phản ánh qua service (multipart) — quét mã độc, mã hoá lại bỏ toàn bộ EXIF, lưu vào kho hồ sơ
	// @screen   09-phan-anh-nguoi-dan §8.4
	// 401 is no session AND a session of another commune (authz compares the commune before the key).
	// 403: no `feedback.resolve`. 404: unknown, another commune's, soft-deleted, or `can-bo` without
	// `feedback.restricted` — one body. 400 `invalid_upload` (broken envelope, file not the declared size)
	// · `invalid_request` (type outside the policy). 408 `upload_timeout`. 409 `petition_state` (ended,
	// also when it ended while the photo was processed — the clean copy then STAYS in the records bucket
	// with a `rejected` row, never listed) · `photo_limit` · `photo_state` · `upload_changed` ·
	// `request_in_progress`. 413 `file_too_large`. 415 `unsupported_media_type`. 422 `photo_rejected`.
	// 503 `upload_busy` (+ Retry-After) · `storage_not_configured` · `upload_limits_unavailable` ·
	// `malware_scan_unavailable`: nothing stored, never stored unscanned.
	//
	// @multipart size content_type? file
	// @reply    201 photoOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    408 httpx.Error
	// @reply    409 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    422 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/verification-photos",
		authz.RequirePermission(d.Checker, "feedback.resolve")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.UploadVerificationPhoto))))

	// DANH SÁCH ẢNH SAU XỬ LÝ — the `photos` list's twin: signed links of at most 15 minutes, AUDITED
	// (`xem_anh_sau_xu_ly`, by the officer's business code, committed before the reply), `no-store`.
	//
	// @summary  Ảnh sau xử lý cán bộ đã tải cho một phiếu phản ánh, mỗi ảnh kèm liên kết xem có ký, sống tối đa 15 phút
	// @screen   09-phan-anh-nguoi-dan §8.4
	// 200 WITH `items: []` when none. 404 is the detail's four causes, one body.
	// 503 `storage_not_configured`: no object store to sign links against.
	//
	// @reply    200 photoListOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/citizen-reports/{maTraCuu}/verification-photos",
		authz.RequirePermission(d.Checker, "feedback.read")(
			http.HandlerFunc(h.ListVerificationPhotos)))

	// --- THE STAFF PROCESSING PATH. SEVEN ROUTES (the two branches added 25/09/2026) --------------
	//
	// A citizen could file a petition and look it up, and no member of staff could classify it,
	// assign it, move it or close it. Every petition that arrived stayed where it landed, with a
	// deadline running and a person watching. The reasoning for each route, its URL noun and the two
	// findings raised for open question #27 are on internal/http/xu_ly_phan_anh.go.
	//
	// ONE THING IS TRUE OF ALL FOUR WRITE ROUTES AND IS SAID HERE ONCE (rule 9, invariant 2): a
	// petition in the field `can-bo` — a report ABOUT a member of staff — is refused to a caller
	// without `feedback.restricted`, and the answer is the 404 each block already declares, identical
	// to an unknown code. THE ACT IS REFUSED, not merely the body: nothing is written, no audit entry
	// is filed and no event is recorded. The decision is app.duocChamPhieuHanChe's, inside the
	// transaction; the route permissions below are unchanged and no key was added.

	// @summary  Danh sách phiếu phản ánh của xã — phân trang theo con trỏ, lọc theo trạng thái · lĩnh vực · thôn · bộ phận · kênh · trễ hạn · đánh giá thấp (`rating_max=1..5`: dân chấm không quá n sao) · phạm vi (`scope=mine`: phiếu đang giao cho chính người gọi, mã lấy từ phiên)
	// @screen   09-phan-anh-nguoi-dan §2, §4
	// `feedback.read` and NOT AnyAuthenticated, unlike the two catalogue reads above: this is a
	// commune's register of what its citizens have reported — names, numbers, addresses and the free
	// text of a complaint (rule 3). The specification gives it its own key for that reason.
	//
	// TWO THINGS DECIDED INSIDE THE HANDLER THAT NO STATUS CODE SHOWS, so they are stated here:
	//
	//	feedback.restricted  absent -> petitions in the field `can-bo` are NOT in the page, NOT in
	//	                     the cursor, and therefore not in any count derived by paging
	//	feedback.unmask      irrelevant -> the reporter is masked on this route WHATEVER the caller
	//	                     holds, because rule 6 invariant 7 wants one audit entry per disclosure
	//	                     and a list cannot produce an honest one
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// 503 `field_catalogue_unavailable`: the field labels (commune wording, else platform default)
	// cannot be read — refused rather than showing raw codes (ADR 0060 §3).
	//
	// @reply    200 page.Result[phieuPhanAnhRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/citizen-reports",
		authz.RequirePermission(d.Checker, "feedback.read")(
			http.HandlerFunc(h.DanhSachPhieu)))

	// PHÂN LOẠI — the act that ISSUES THE COMMUNE'S PROMISE, which is why it has a key of its own.
	//
	// `feedback.classify` AND NOT `feedback.assign` (ADR 0030, and rule 5 invariant 3b: these rights
	// are not a Cartesian product). Settling the field is what fixes `han_xu_ly_xong` — being handed
	// the work is not being allowed to promise on behalf of the authority.
	//
	// idem.KhongCan, AND IT IS THE HONEST DECLARATION RATHER THAN THE COMFORTABLE ONE: the act is
	// idempotent by the shape of the lifecycle, not by luck. The UPDATE carries
	// `trang_thai = 'da-tiep-nhan'`, so a second identical request matches no row and answers 409 —
	// one classification, one deadline, one audit entry. idem.Required would hide the second attempt
	// instead of telling the officer their screen is stale.
	//
	// 409 COVERS TWO VERY DIFFERENT THINGS and the code in the body tells them apart:
	// `petition_state` (somebody classified it first) and `sla_chua_cau_hinh` (the commune has not
	// configured the hours for this field — the ordinary answer in every commune today).
	//
	// @summary  Chốt lĩnh vực cho phiếu phản ánh — hành vi ẤN ĐỊNH hạn xử lý xong theo cấu hình của xã
	// @screen   09-phan-anh-nguoi-dan §8.2
	// @request  phanLoaiVao
	// @reply    200 phieuPhanAnhRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/classification",
		authz.RequirePermission(d.Checker, "feedback.classify")(
			idem.KhongCan("câu UPDATE mang `trang_thai = 'da-tiep-nhan'`, nên lần gửi thứ hai không khớp dòng nào và trả 409 — đúng một lần chốt lĩnh vực, đúng một hạn, đúng một vết")(
				http.HandlerFunc(h.PhanLoaiPhieu))))

	// PHÂN CÔNG — who inside the authority is answerable for this petition.
	//
	// idem.KhongCan, and here the reason is different from the route above: a second identical
	// assignment writes the same department and the same officer onto the same row, so the ROW ends
	// in one state. It does file a second audit entry, and that is CORRECT rather than a duplicate —
	// somebody performed the act twice, and an append-only ledger records acts (rule 6, invariant 4).
	//
	// @summary  Chuyển phiếu phản ánh cho một bộ phận xử lý, kèm cán bộ phụ trách nếu đã biết
	// @screen   09-phan-anh-nguoi-dan §8.5
	// @request  phanCongVao
	// @reply    200 phieuPhanAnhRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/assignment",
		authz.RequirePermission(d.Checker, "feedback.assign")(
			idem.KhongCan("phân công lại cùng một bộ phận để lại đúng một dòng ở đúng một trạng thái; vết thứ hai là một hành vi CÓ THẬT của con người, không phải bản sao")(
				http.HandlerFunc(h.PhanCongPhieu))))

	// CHUYỂN TRẠNG THÁI — one step along the main flow, and the target is deliberately not a
	// parameter. See the handler.
	//
	// ⚠ `feedback.read` GUARDS THIS ROUTE AND `feedback.resolve` STILL GUARDS THE CLOSING ROUTE BELOW.
	// That difference is the holding rule the owner decided on 2026-09-23 ("theo require"), and it is
	// not a relaxation of rule 5: the declaration here is an explicit, seeded key, so an account
	// without `feedback.read` is refused at this gate before anything is read. The condition that
	// actually decides — `feedback.resolve` OR being the officer this petition was assigned to — lives
	// in app.duocTienTrangThai, inside the transaction, on the row read under the lock. A hamlet leader
	// handed one petition can move it; giving them `feedback.resolve` instead would let them close the
	// neighbouring hamlet's petitions too.
	//
	// NO KEY WAS INVENTED (rule 5, invariant 3c): `feedback.read` is seeded at
	// service-identity/migrations/0001_init.sql. The older note here raised the missing "move the work
	// along" key as a finding for open question #27; that finding still stands — this change answers
	// WHO may advance a petition, not whether advancing deserves a key of its own.
	//
	// idem.KhongCan: the UPDATE carries the expected status, so a double click advances the petition
	// exactly one step and the second request answers 409.
	//
	// ⚠ NO @request LINE, AND THE ROUTE STILL ACCEPTS AN OPTIONAL BODY `{"note": "…"}` (tienTrangThaiVao,
	// migration 0013). tools/apidoc publishes every @request as `requestBody.required: true`
	// (tools/apidoc/openapi.go, the `t.Request != ""` branch) and has no optional-body form, so
	// declaring it would tell every generated client that this bodiless call is now invalid. The note is
	// therefore absent from the contract on THIS route only — a tools/apidoc gap, reported.
	//
	// @summary  Chuyển phiếu phản ánh sang bước kế tiếp của luồng chính (máy trạng thái quyết định bước nào)
	// @screen   09-phan-anh-nguoi-dan §8.2
	// @reply    200 phieuPhanAnhRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/status",
		authz.RequirePermission(d.Checker, "feedback.read")(
			idem.KhongCan("câu UPDATE mang trạng thái đang chờ, nên bấm hai lần vẫn chỉ tiến đúng một bước và lần thứ hai trả 409")(
				http.HandlerFunc(h.TienTrangThaiPhieu))))

	// ĐÓNG PHIẾU — the act rule 10, invariant 6 governs: it records a RESULT THE CITIZEN CAN READ,
	// and the server refuses a closing without one. "Đã xử lý" alone is not a result.
	//
	// `closure` IS THE SETTLED URL NOUN for `dong_phieu` (kb/00-foundation/ubiquitous-language.md) —
	// not translated on the spot.
	//
	// idem.KhongCan for the same mechanical reason as the two above: the UPDATE carries
	// `trang_thai = 'cho-dan-xac-nhan'`, so one closing, one result, one entry.
	//
	// @summary  Đóng phiếu phản ánh kèm kết quả xử lý người dân đọc được
	// @screen   09-phan-anh-nguoi-dan §8.2
	// @request  dongPhieuVao
	// @reply    200 phieuPhanAnhRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/closure",
		authz.RequirePermission(d.Checker, "feedback.resolve")(
			idem.KhongCan("câu UPDATE mang `trang_thai = 'cho-dan-xac-nhan'`, nên lần gửi thứ hai không khớp dòng nào — đúng một lần đóng, đúng một kết quả, đúng một vết")(
				http.HandlerFunc(h.DongPhieu))))

	// KHÔNG TIẾP NHẬN — the first of the two terminal branches (user decisions 24-25/09/2026).
	//
	// `rejection` IS THE USER'S URL NOUN (25/09/2026, kb/00-foundation/ubiquitous-language.md), not
	// translated on the spot. `feedback.classify` because the branch leaves only from
	// `dang-phan-loai`: it is an outcome of classification (ADR 0030). No key was invented.
	//
	// idem.KhongCan for closure's mechanical reason: the UPDATE carries `trang_thai = 'dang-phan-loai'`,
	// so a second identical request matches no row and answers 409 — one refusal, one reason, one
	// entry. Migration 0011's trigger additionally refuses rewriting the reason once set.
	//
	// @summary  Không tiếp nhận phiếu phản ánh — kèm lý do người dân đọc được
	// @screen   09-phan-anh-nguoi-dan §8.2
	// @request  khongTiepNhanVao
	// @reply    200 phieuPhanAnhRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/rejection",
		authz.RequirePermission(d.Checker, "feedback.classify")(
			idem.KhongCan("câu UPDATE mang `trang_thai = 'dang-phan-loai'`, nên lần gửi thứ hai không khớp dòng nào và trả 409 — đúng một lần từ chối, đúng một lý do, đúng một vết")(
				http.HandlerFunc(h.KhongTiepNhanPhieu))))

	// CHUYỂN CẤP TRÊN — the second terminal branch: the petition leaves the commune for another body,
	// which is NAMED, with a reason the citizen reads.
	//
	// `referral` IS THE USER'S URL NOUN (25/09/2026). Same key and same idempotency reasoning as
	// `rejection` above. ⚠ ONLY FROM `dang-phan-loai`: a referral of work already begun is undecided
	// with the owner and is refused 409, not built.
	//
	// @summary  Chuyển phiếu phản ánh lên/sang cơ quan có thẩm quyền — kèm lý do và cơ quan tiếp nhận
	// @screen   09-phan-anh-nguoi-dan §8.2
	// @request  chuyenCapTrenVao
	// @reply    200 phieuPhanAnhRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/referral",
		authz.RequirePermission(d.Checker, "feedback.classify")(
			idem.KhongCan("câu UPDATE mang `trang_thai = 'dang-phan-loai'`, nên lần gửi thứ hai không khớp dòng nào và trả 409 — đúng một lần chuyển, đúng một cơ quan nhận, đúng một vết")(
				http.HandlerFunc(h.ChuyenCapTrenPhieu))))

	// CÔNG KHAI — staff moderation of the public page (ADR 0050 point 8, SRS M4.3.1 P0). The
	// requirement's `POST /{id}/moderation` (`router.py:349-361`), respelled for this surface:
	// `publication` is the nominalisation rest_api_guard itself names for "publish", matching the column
	// `publication_status` (migration 0017), and PUT because the body is an absolute state — a
	// sub-resource you set, like PUT …/no-task-marker.
	//
	// `feedback.assign`, the requirement's key (`router.py:352`), seeded at
	// service-identity/migrations/0001_init.sql. NO KEY WAS INVENTED (rule 5, invariant 3c).
	//
	// IT MOVES NO LIFECYCLE STATUS (user decision 28/09/2026) and owes the citizen no message: it is
	// not a transition and not in ADR 0041's table. A `can-bo` petition is refused `cong-khai` with 409
	// `never_public`; to a caller without `feedback.restricted` it is 404, as on every write route.
	//
	// @summary  Đặt trạng thái công khai của phiếu phản ánh — cho hiện công khai hoặc ẩn khỏi trang công khai (không đổi trạng thái xử lý)
	// @screen   09-phan-anh-nguoi-dan §14.4
	// @request  publicationIn
	// @reply    200 phieuPhanAnhRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/citizen-reports/{maTraCuu}/publication",
		authz.RequirePermission(d.Checker, "feedback.assign")(
			idem.KhongCan("PUT đặt một trạng thái tuyệt đối; đã đúng giá trị thì use case không ghi gì, nên gửi lại để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SetPublication))))

	// --- THE PROCESSING LOGBOOK (migration 0013). ONE READ, ONE WRITE --------------------------------
	//
	// STAFF-INTERNAL, AND ONLY EVER ON THIS MUX. Routing history and staff notes never reach the
	// citizen (rule 4, forbidden #5; rule 10, invariant 7): routes_cong_dan.go mounts nothing of this,
	// and GET /api/v1/my-citizen-reports/{maTraCuu} carries no field of it.
	//
	// `feedback.read` ON BOTH, and on the write it is the GATE, not the answer — the same shape as
	// `…/status`. The condition that decides a note is app.duocGhiChu, on the row read FOR UPDATE: the
	// officer the petition is assigned to, OR a holder of feedback.resolve / feedback.assign /
	// feedback.classify (owner's decision, 2026-09-26). No key was invented (rule 5, invariant 3c).
	//
	// 404 is the one answer for no such code, another commune's code, a soft-deleted petition AND a
	// `can-bo` petition without `feedback.restricted` — see Handler.khongTimThay.

	// @summary  Nhật ký xử lý của một phiếu phản ánh — mới nhất trước, phân trang theo con trỏ
	// @screen   09-phan-anh-nguoi-dan §8.7
	// 401 is RequirePermission's answer to no session AND to a session of another commune (authz
	// compares the commune before the key). 403: no `feedback.read`. NO idem.* DECLARATION: a GET
	// changes no state.
	//
	// @reply    200 page.Result[nhatKyPhieuRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/citizen-reports/{maTraCuu}/log-entries",
		authz.RequirePermission(d.Checker, "feedback.read")(
			http.HandlerFunc(h.DocNhatKyPhieu)))

	// GHI CHÚ NỘI BỘ — one manual note on the timeline, allowed on every status, final ones included.
	//
	// idem.Required(idem.MoKhiHong), THE SHAPE OF POST /api/v1/tasks AND NOT THE KhongCan OF THE ACTS
	// ABOVE, and the reason is the table: those acts are bound to a status, so a second click matches
	// no row and answers 409; a note is bound to nothing, and the table it lands in is APPEND-ONLY — a
	// double submit would leave a duplicate entry that nobody can ever remove (rule 7). MoKhiHong: a
	// cache outage lets a note through (a visible duplicate, corrected by writing another note)
	// rather than refusing an officer mid-case.
	//
	// 403 is TWO causes and says so: no `feedback.read` at the gate, or — inside the use case — not the
	// assignee and none of the three commune-wide keys (the sentence of the `…/status` route).
	//
	// @summary  Ghi chú nội bộ vào nhật ký xử lý phiếu phản ánh (không đổi trạng thái)
	// @screen   09-phan-anh-nguoi-dan §8.7
	// @request  ghiChuPhieuVao
	// @reply    201 nhatKyPhieuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/log-entries",
		authz.RequirePermission(d.Checker, "feedback.read")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.GhiChuPhieu))))

	// 📎 ĐÍNH KÈM (§8.7 :197) — ONE multipart upload through this service (ADR 0052 §Sửa đổi 09/10/2026),
	// answered with the STORED file; then the file rides on the next note (`attachments` on the POST
	// above). internal/app/petition_log_attachment.go has the whole flow. STAFF-ONLY (owner decision B,
	// 02/10/2026): no citizen route reads these files.
	//
	// `log-attachments` AND NOT `attachments`: a petition carries THREE kinds of file (the citizen's
	// scene photos, staff's verification photos, these), and a bare `attachments` would not say which.
	//
	// `feedback.read` AT THE GATE, the note route's shape: whether this person may upload onto THIS
	// petition is the note rule (app.duocGhiChu — the assignee, or a holder of feedback.resolve /
	// feedback.assign / feedback.classify), decided on the locked row, because an upload exists only to
	// be attached to a note. Seeded keys only (rule 5, invariant 3c). Every status, like the note.
	//
	// THE LIMITS ARE PLATFORM'S (`petition-log-attachment`, ADR 0052 §10): 50 MB, PDF / JPEG / PNG today.
	// Sniffed, scanned, hashed and copied to the records key AS UPLOADED (not shown to a citizen, so no
	// EXIF re-encode — a re-encoded record would no longer be the record).
	//
	// THE BODY IS multipart/form-data (core/httpx.ReadUpload): text fields FIRST — `size` (required, the
	// byte count), `file_name` (optional; else the file part's own name — the name readers see, personal
	// data when it describes a case: stored, never logged) and `content_type` (optional; else the file
	// part's Content-Type) — then exactly ONE part named `file`, then nothing.
	//
	// @summary  Tải lên một tệp đính kèm cho nhật ký xử lý phiếu phản ánh qua service (multipart) — dò kiểu, quét mã độc, lưu vào kho hồ sơ
	// @screen   09-phan-anh-nguoi-dan §8.7
	// 401 is no session AND a session of another commune. 403: no `feedback.read`, or — inside — not the
	// assignee and none of the three commune-wide keys. 404: the petition's four causes. 400
	// `invalid_upload` · `invalid_request` (name, type). 408 `upload_timeout`. 409 `attachment_limit` ·
	// `attachment_state` · `upload_changed` · `request_in_progress`. 413 `file_too_large`. 415
	// `unsupported_media_type`. 422 `attachment_rejected`. 503 `upload_busy` (+ Retry-After) ·
	// `storage_not_configured` · `upload_limits_unavailable` · `malware_scan_unavailable`.
	//
	// @multipart size content_type? file_name? file
	// @reply    201 taskAttachmentOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    408 httpx.Error
	// @reply    409 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    422 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/log-attachments",
		authz.RequirePermission(d.Checker, "feedback.read")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.UploadPetitionLogAttachment))))

	// TẢI VỀ — `feedback.read`, the key of the timeline the file is shown on. A file on a log entry is
	// part of the timeline every staff reader sees; a file not yet attached only its uploader may fetch.
	// AUDITED (`tai_tep_nhat_ky_phan_anh`), UNLIKE THE TASK TWIN: a file on a petition's log is evidence
	// about one citizen's case and cannot be masked (rule 6, invariant 7). `no-store`.
	//
	// @summary  Liên kết tải về một tệp đính kèm của nhật ký xử lý phiếu (sống tối đa 15 phút)
	// @screen   09-phan-anh-nguoi-dan §8.7
	// @reply    200 taskAttachmentDownloadOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/citizen-reports/{maTraCuu}/log-attachments/{id}/download",
		authz.RequirePermission(d.Checker, "feedback.read")(
			http.HandlerFunc(h.PetitionLogAttachmentDownload)))

	// TẠO NHIỆM VỤ TỪ PHIẾU (user decision 30/09/2026) — the only door by which a `phan-anh` task is
	// booked: POST /api/v1/tasks refuses that source (commit 2d34eba4) because nothing there can check
	// the `source_id` it was sent.
	//
	// `task.create` AND `feedback.read`, BOTH seeded (service-identity/migrations/0001_init.sql:299 and
	// :308). The act is the creation of a task, so the task key; it is done FROM a petition the caller
	// must be able to read, so the read key — without it a task holder who cannot see the register could
	// probe lookup codes through this route. `feedback.restricted` is not a gate but a fact: a `can-bo`
	// petition without it is the 404 of an unknown code.
	//
	// 401 is RequirePermission's answer to no session AND to a session of another commune (the commune
	// is compared before either key). 404: unknown, another commune's, soft-deleted, or restricted
	// without the key — one answer (Handler.khongTimThay). 409, three petition codes (user decision
	// 30/09/2026, domain.PetitionAcceptsTask): `restricted_field_no_task` — a `can-bo` petition, to a
	// `feedback.restricted` holder; `petition_not_classified` — `da-tiep-nhan` / `dang-phan-loai`, not yet
	// classified and accepted; `petition_state` — closed (`da-dong`, `khong-tiep-nhan`,
	// `chuyen-cap-tren`). Plus, from the task register's own mapping (traLoiLoiNhiemVu), what the create
	// path can reach: `code_taken`, `task_tree` (parent gone, cycle, tree too deep) and `task_document` (a
	// document item naming a line id — there is no stored line on a task being created,
	// domain.SoSanhVanBan); and idem's `request_in_progress`. The 409 line below publishes exactly these.
	// 400 also answers a body that sends `source` or `source_id`, or `lead_unit` / `monitor` (ADR 0065
	// NV5: one role — the lead unit is `unit`, the monitor is `assignee`).
	//
	// idem.Required(idem.MoKhiHong), POST /api/v1/tasks' declaration and for its reason: nothing here
	// makes a second task from one petition a conflict (several tasks from one petition is legitimate),
	// so a double submit is indistinguishable from an intended second task without the key.
	//
	// @summary  Tạo nhiệm vụ từ một phiếu phản ánh — nguồn giao do máy chủ gắn theo phiếu, ghi cùng một dòng nhật ký phiếu
	// @screen   09-phan-anh-nguoi-dan §13
	// @request  petitionTaskIn
	// @reply    201 nhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error petition_state petition_not_classified restricted_field_no_task
	// @reply    409 httpx.Error code_taken task_tree task_document request_in_progress
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/citizen-reports/{maTraCuu}/tasks",
		authz.RequirePermission(d.Checker, "task.create")(
			authz.RequirePermission(d.Checker, "feedback.read")(
				idem.Required(idem.MoKhiHong)(
					http.HandlerFunc(h.CreateTaskFromPetition)))))

	// CHUYỂN ĐƠN THƯ THÀNH NHIỆM VỤ (ADR 0085 A; ADR 0084 #6; docs/ui-ux/05 §3.5). Its own noun because
	// `citizen-letters` belongs to service-documents and cannot be nested under (A1); the record created is
	// a task, and it is written HERE. Before anything is written, documents is asked whether the letter
	// exists in this commune and is not a denunciation (ResolveCitizenLetterForTask); one transaction then
	// books the task with `nguon_giao = don-thu`, `nguon_id = letter id`, the inherited deadline and the
	// `tao_nhiem_vu` entry naming the letter by number/year (transaction-boundaries `tao_nhiem_vu_tu_don_thu`).
	//
	// `task.create` AND `petition.read`, BOTH seeded (service-identity/migrations/0001_init.sql:308 and
	// :303; ADR 0085 A8). The act creates a task; it is done FROM a letter the caller must be able to read —
	// without the read key a task holder could probe letter ids through this route.
	//
	// 401 is RequirePermission's answer to no session AND to a session of another commune (compared before
	// either key). 404 `not_found`: unknown, soft-deleted or another commune's letter — documents' one
	// answer. 422 `denunciation_no_task` (a `to-cao` letter, C9) and `assignment_required` (no unit or
	// assignee in the dialog, and the letter holds none). 503 `citizen_letter_check_unavailable` (documents
	// not reachable / not wired / broke contract) and `assignee_check_unavailable` — nothing written in
	// either. 400 also answers `source`, `source_id`, `due_at`, `lead_unit` or `monitor` in the body. Plus
	// the task register's own refusals (traLoiLoiNhiemVu): `code_taken`, `task_tree`, `task_document`, and
	// idem's `request_in_progress`.
	//
	// idem.Required(idem.MoKhiHong), POST /api/v1/tasks' declaration (A9): one letter may become several
	// tasks, as in the prototype, so a double submit is indistinguishable from an intended second task
	// without the key.
	//
	// @summary  Chuyển đơn thư thành nhiệm vụ — nguồn, hạn và người giữ suy ra ở máy chủ theo đơn; đơn tố cáo bị từ chối
	// @screen   05-van-ban-don-thu §3.5
	// @request  citizenLetterTaskIn
	// @reply    201 nhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error code_taken task_tree task_document request_in_progress
	// @reply    422 httpx.Error denunciation_no_task assignment_required
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/citizen-letter-tasks",
		authz.RequirePermission(d.Checker, "task.create")(
			authz.RequirePermission(d.Checker, "petition.read")(
				idem.Required(idem.MoKhiHong)(
					http.HandlerFunc(h.CreateTaskFromCitizenLetter)))))

	// --- THE TASK REGISTER. TWO READ ROUTES AND SIX WRITE ROUTES ---------------------------------
	//
	// `tasks` is the settled URL noun for `nhiem_vu` (kb/00-foundation/ubiquitous-language.md:144),
	// not translated on the spot (ADR 0011).
	//
	// `task.read` ON BOTH READS, AND NOT AnyAuthenticated, unlike the two catalogue reads above. The
	// catalogues are the WORDS a commune sorts its work with and fill a picker on nearly every
	// screen; this is the work itself — who was told to do what, by when, and who has not. The
	// specification gives it a key of its own for that reason (§6), and the key is seeded at
	// service-identity/migrations/0001_init.sql:304. NO KEY WAS INVENTED (rule 5, invariant 3c).
	//
	// THE THREE REASONS THIS BLOCK ONCE GAVE FOR HAVING NO WRITE ROUTE ARE ANSWERED, and they are
	// marked answered rather than deleted: a reader who arrives through ADR 0029 §118 or through the
	// progress ledger needs to see that the reason MOVED, not that it vanished.
	//
	//	the sub-task tree        ANSWERED — ADR 0037 (2026-09-23) decided all four questions, and
	//	                         migration 0008 added the column. The three rules that do not fit in
	//	                         a CHECK live in internal/app, plus a FIFTH the specification never
	//	                         raised: a cycle across two or more rows.
	//	who approves an extension  ANSWERED — ADR 0038 (2026-09-23): the leader named ON THE RECORD,
	//	                         compared by staff business code, as a SECOND layer under
	//	                         `task.extend`. The empty-column case fails CLOSED.
	//	the deadline              NOT THE SAME QUESTION AS THE PETITION INTAKE'S. §7.1 puts "Hạn hoàn
	//	                         thành" on the form as a date a leader types, so creating a task
	//	                         derives no deadline and calls identity not at all — see the header of
	//	                         internal/app/nhiem_vu.go. (For completeness: identity's
	//	                         ResolveDeadlines does now cover `WORK_KIND_NHIEM_VU`, so the paths
	//	                         that WOULD need a computed deadline — §8's Excel import, splitting a
	//	                         meeting conclusion — have a contract to call when they are built.)
	//
	// §10's `giao-viec` — ANSWERED 28/09/2026: `task.assign` now has its own route,
	// POST /api/v1/tasks/{ma}/assignment, below. PATCH still cannot move `bo_phan_id` or
	// `nguoi_thuc_hien_ma` — folding it in would hand assignment to every holder of `task.update`.

	// @summary  Danh sách nhiệm vụ của xã — phân trang theo con trỏ, lọc theo phạm vi (`all` · `mine` · `related` · `assigned-by-me` = tôi tạo hoặc tôi là lãnh đạo giao việc) · trạng thái · chưa hoàn thành (`incomplete=true`, mọi trạng thái trừ `hoan-thanh`) · loại · khối · ưu tiên · bộ phận · người thực hiện · nguồn giao · trễ hạn · sắp đến hạn · việc con của một mã (`parent=NV19`) · chỉ việc gốc (`roots=true`); mỗi dòng kèm số lần đã lùi hạn (`extension_count`) và cờ đang có đề nghị lùi hạn chờ duyệt (`pending_extension`); sắp theo `created_at` · `code` · `due_at` (việc không có hạn luôn ở cuối) · `priority` (theo thứ tự danh mục mức ưu tiên của xã, việc không có mức ở cuối) · `title`
	// @screen   02-nhiem-vu §3, §4, §5.10
	// 400 covers a filter the server REFUSES rather than ignores.
	//
	// `soon=true` and `scope=related` ASK IDENTITY FIRST (taskFilterFromRequest), and neither falls
	// back to a page without the filter:
	//
	//	409 due_soon_not_configured  `soon=true` in a commune that has set no due-soon threshold
	//	503 task_filter_unavailable  identity did not answer either question
	//
	// `parent=<register number>` lists the DIRECT children of that task; a number matching nothing
	// in this commune is an empty page, the same answer a childless task gets. `sort=due_at` puts the
	// tasks WITHOUT a deadline LAST in both directions, and the cursor stays exact across that
	// boundary (petstore.SapXepNhiemVu, petstore.DanhSach). Every row carries `parent` as a register number,
	// `child_count`, `extension_count` (approved requests, soft-deleted included — migration 0016's
	// predicate) and `pending_extension`, resolved for the whole page in three statements at most.
	//
	// `roots=true` keeps only tasks with no parent (the prototype's `roots_only`); absent = every task.
	// Any other value is 400. It is one more field of the shared filter, so /task-counts and the
	// register export apply it too. With `parent=` it is an empty page — a child is never a root.
	//
	// 500 additionally covers `scope=mine` / `scope=related` / `scope=assigned-by-me` on a principal
	// with no staff business code — a wiring fault, refused rather than silently widened to the whole
	// register. `assigned-by-me` takes "me" from the session only (ADR 0071); no parameter names it.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @reply    200 page.Result[nhiemVuRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/tasks",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.DanhSachNhiemVu)))

	// XUẤT SỔ THEO DÕI (§4.3, :128) — the register as an .xlsx, columns in §4.3's order, under exactly
	// the filters and sort of GET /api/v1/tasks. `task.read`, the list's own key (user decision
	// 28/09/2026 — NO KEY INVENTED): the file shows what the list shows. It is NOT a personal-data
	// export under rule 3 invariant 4 — a task carries no citizen's data — but it DOES carry staff full
	// names, so every export writes ONE audit entry (`xuat_so_theo_doi_nhiem_vu`: who, the filters with
	// the free-text search recorded as present only, the sort, the row count, and that staff names were
	// included) BEFORE the file is sent; no trail, no file (app.TaskRegisterExport).
	//
	// 422 above 5.000 rows — refused, never truncated. 503 when identity cannot supply the names (no
	// file with blanks where names belong), or cannot resolve `soon` / `scope=related`. 409 as the list.
	//
	// NO idem.* DECLARATION: a GET, and a repeated export is a second, separately audited export —
	// which is the truth of what happened.
	//
	// @summary  Xuất Sổ theo dõi nhiệm vụ ra tệp Excel (.xlsx) — cùng bộ lọc và cách sắp với danh sách, cột đúng thứ tự §4.3; mỗi lần xuất được ghi vết
	// @screen   02-nhiem-vu §4.3
	// @reply    200 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    422 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/tasks/register-export",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.ExportTaskRegister)))

	// NHẬP TỪ EXCEL (§8) — `task.create`, the key of POST /api/v1/tasks: the import IS task creation,
	// row by row, through the same create path (app.createInTx). NO KEY INVENTED.
	//
	// multipart/form-data, field `file`, at most 2 MB (413 above it); zip parts, unzipped size and rows
	// scanned are bounded before any XML is parsed (task_import.go). `dry_run=true` checks everything —
	// identity included — and writes nothing. Every row is checked first; ONE refused row and nothing is
	// written (200, `committed: false`, the row report: row, column, sentence — never the cell's value).
	// All rows pass → ONE transaction books every task with an auto-issued number (201, `codes`).
	//
	// 400 not an .xlsx / not the template's heading row / no task row · 400 `import_retired_columns` a
	// file on the pre-30/09/2026 template, still carrying "Cơ quan chủ trì tham mưu" or "Chuyên viên theo
	// dõi" (ADR 0065 NV5 — refused whole, never read with those columns skipped) · 422 over 500 rows ·
	// 503 identity could not check the codes (nothing written).
	//
	// idem.Required(idem.DongKhiHong): the act ISSUES REGISTER NUMBERS, and a number once issued is never
	// issued again (rule 7, invariant 3) — a double submit would book the whole file twice, permanently.
	// A retry of the same key is told the first issued number.
	//
	// @summary  Nhập nhiệm vụ từ tệp Excel theo mẫu — kiểm mọi dòng trước, một dòng sai thì không nhập dòng nào; mã nhiệm vụ cấp tự động
	// @screen   02-nhiem-vu §8
	// @reply    200 taskImportResultOut
	// @reply    201 taskImportResultOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    413 httpx.Error
	// @reply    422 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/tasks/imports",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.ImportTasks))))

	// MẪU NHẬP (§8, "⬇ Tải mẫu nhiệm vụ") — the heading row and one invented example row. `task.create`,
	// the import's own key: the template is only useful to somebody who may import. A GET; no idem.*.
	//
	// @summary  Tải tệp Excel mẫu để nhập nhiệm vụ
	// @screen   02-nhiem-vu §8
	// @reply    200 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/tasks/import-template",
		authz.RequirePermission(d.Checker, "task.create")(
			http.HandlerFunc(h.ImportTemplate)))

	// SỐ LƯỢNG THEO TRẠNG THÁI (§4.1) — the real number over each Kanban column, under EXACTLY the
	// filters of GET /api/v1/tasks (one parser, one predicate builder; see task_counts.go). `task.read`,
	// the list's own key: the count reveals nothing the list does not, and a second key would let an
	// account see a board whose headers it may not read. NO KEY WAS INVENTED (rule 5, invariant 3c).
	//
	// A top-level `task-counts` and not `tasks/counts`, which `tasks/{ma}` would read as a task
	// numbered `counts`. ALL SEVEN codes, zeros included, in the codes' default order.
	//
	// 400, 409 and 503 are the list's own refusals, with the same sentences; paging parameters are
	// ignored. 401 is RequirePermission's answer to no session AND to a session of another commune.
	// NO idem.* DECLARATION: a GET changes no state. NO AUDIT ENTRY: counts, no personal data.
	//
	// @summary  Số nhiệm vụ theo từng trạng thái, cùng bộ lọc với danh sách nhiệm vụ — số thật trên đầu mỗi cột Kanban
	// @screen   02-nhiem-vu §4.1
	// @reply    200 taskCountsOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/task-counts",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.TaskCounts)))

	// @summary  Một nhiệm vụ, tra theo mã nhiệm vụ của xã (NV19) — kèm số lần đã lùi hạn và cờ đang có đề nghị lùi hạn chờ duyệt
	// @screen   02-nhiem-vu §5
	// 404 is the single answer to three causes — no such number, another commune's number, and a
	// soft-deleted task. Telling them apart tells a caller which numbers exist in a register they
	// are not reading.
	//
	// 401 is RequirePermission's answer to no session AND to a session issued by another commune:
	// authz.xacNhanXa refuses before the permission is consulted, so no query runs and the response
	// cannot differ by commune.
	//
	// THE `Theo văn bản` DOCUMENT BLOCK OF §5.4 IS IN THE REPLY (`documents`, migration 0009). The
	// progress log (§5.9) is NOT — it has its own paged route, …/{ma}/log-entries, below. The
	// PENDING extension requests (§5.8) are read commune-wide through GET /api/v1/task-extensions,
	// further down; the per-task history of every request, decided ones with their note, is
	// GET /api/v1/tasks/{ma}/extensions, beside the two extension writes.
	//
	// @reply    200 nhiemVuRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/tasks/{ma}",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.DocNhiemVu)))

	// NHẬT KÝ & TRAO ĐỔI (§5.9) — the READ half only. The URL noun `log-entries` is the petition
	// logbook's (GET /api/v1/citizen-reports/{maTraCuu}/log-entries), and so are the order (newest
	// first), the cursor paging and the row shape: two timelines that read alike.
	//
	// `task.read`, THE KEY OF THE TWO READS ABOVE (seeded at service-identity/migrations/
	// 0001_init.sql:304). The log is part of the task a reader of the register already sees. NO KEY
	// WAS INVENTED (rule 5, invariant 3c).
	//
	// The WRITE half (POST, manual entry) is declared right below.
	//
	// 404 is the single answer GET /api/v1/tasks/{ma} gives, for the same three causes — the task is
	// read first through the same reader, and the log only after. 401 is RequirePermission's answer to
	// no session AND to a session of another commune. NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Nhật ký & Trao đổi của một nhiệm vụ — mới nhất trước, phân trang theo con trỏ
	// @screen   02-nhiem-vu §5.9
	// @reply    200 page.Result[nhatKyNhiemVuRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/tasks/{ma}/log-entries",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.DocNhatKyNhiemVu)))

	// GHI NHẬT KÝ (§5.9) — one manual entry on the task's timeline, with the officer's completed uploads
	// as optional `attachments` (linked in the entry's transaction; the attachment routes are right
	// below). Allowed on every status, `hoan-thanh` included: it appends, it moves nothing.
	//
	// `task.read` AT THE GATE, AND THAT IS vigov-require a37ec96 RATHER THAN A RELAXATION (user decision
	// 28/09/2026). The answer to "may this person write here" depends on THIS task: the assignee and a
	// holder of the commune-wide `task.update` may; the assigning leader and the author may write the
	// log too (the monitor IS the assignee since ADR 0065 NV5) (a37ec96's "related" people); anybody else gets 403 from the use case, on the
	// row read FOR UPDATE (domain/task_participant.go). Gating on `task.update` would lock out exactly
	// the specialist a37ec96 was written for. `task.read` is seeded (service-identity/migrations/
	// 0001_init.sql:304); NO KEY WAS INVENTED (rule 5, invariant 3c). A TASKS-ONLY rule: ADR 0038 forbids
	// carrying the petition holder rule across, and this is not it.
	//
	// ⚠ ONE HALF OF a37ec96's "related" set IS MISSING: officers of the task's unit / lead unit. The
	// principal carries no unit, and identity's ResolveStaffOrgUnits says of its own answer that the
	// caller "must never use it in a guard" (identity.proto) — so they are refused (fail closed) until
	// the contract owner decides otherwise (domain/task_participant.go).
	//
	// idem.Required(idem.MoKhiHong), the petition note's declaration and for its reason: an entry is
	// bound to no status, so a double submit would leave a duplicate in an APPEND-ONLY table nobody can
	// ever remove (rule 7). MoKhiHong: a cache outage lets an entry through rather than refusing an
	// officer mid-work.
	//
	// ONE TRANSACTION: the timeline row, the audit entry (`ghi_nhat_ky_nhiem_vu`) and — when the body
	// names colleagues in `mentioned_staff_codes` — one staff-notice outbox row (ADR 0086 kind 21
	// `nhiem-vu.nhac-ten`, every named colleague but the author). The text is in the row only; the delta
	// carries its length and the named codes (rule 6, forbidden #4). A named code identity does not answer
	// as staff of THIS commune is 400; identity down is 503 and nothing is written.
	//
	// @summary  Ghi một dòng nhật ký vào Nhật ký & Trao đổi của nhiệm vụ — người thực hiện, người liên quan hoặc cán bộ có quyền cập nhật (không đổi trạng thái)
	// @screen   02-nhiem-vu §5.9
	// @request  taskLogEntryIn
	// @reply    201 nhatKyNhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/tasks/{ma}/log-entries",
		authz.RequirePermission(d.Checker, "task.read")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.AddTaskLogEntry))))

	// 📎 ĐÍNH KÈM (§5.9) — ONE multipart upload through this service (ADR 0052 §Sửa đổi 09/10/2026),
	// answered with the STORED file; then the file rides on the next log entry (`attachments` on the POST
	// above). internal/app/task_attachment.go has the whole flow.
	//
	// `task.read` AT THE GATE, for the log-entry route's reason: whether this person may upload onto
	// THIS task is the log-entry right (assignee / related person / `task.update`), decided on the row by
	// the use case — an upload exists only to be attached to an entry. `task.read` is seeded
	// (service-identity/migrations/0001_init.sql:304); NO KEY WAS INVENTED (rule 5, invariant 3c).
	//
	// THE LIMITS ARE PLATFORM'S (ADR 0052 §10): size, types and the per-task count are read from
	// ListUploadPolicies on every request. Not configured → 503 `storage_not_configured`, as is a missing
	// object store or scanner — the rest of the service keeps serving. Stat · sniff (the client's type is
	// never trusted) · the CURRENT policy · ClamAV · sha256 · copy to the private bucket at the records
	// key · `stored` and the trail in ONE transaction; infected, wrong type, too large, over the count →
	// 422 and the temp object deleted; scanner or platform down → 503, NEVER stored unscanned (ADR 0052 §9).
	//
	// THE BODY IS multipart/form-data (core/httpx.ReadUpload): text fields FIRST — `size` (required, the
	// byte count), `file_name` (optional; else the file part's own name — personal data when it
	// describes a case: stored, never logged) and `content_type` (optional; else the file part's
	// Content-Type) — then exactly ONE part named `file`, then nothing.
	//
	// idem.Required(idem.MoKhiHong): a replay answers the FILE ID (`{"code": "<id>", "replayed": true}`),
	// never a second file. MoKhiHong for the log entry's reason — a cache outage must not stop an
	// officer mid-work; the residue of a double submit is a second stored file the officer can remove
	// (`Gỡ`), never a lost one.
	//
	// @summary  Tải lên một tệp đính kèm cho nhật ký nhiệm vụ qua service (multipart) — dò kiểu, quét mã độc, lưu vào kho hồ sơ
	// @screen   02-nhiem-vu §5.9
	// @multipart size content_type? file_name? file
	// @reply    201 taskAttachmentOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    408 httpx.Error
	// @reply    409 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    422 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/tasks/{ma}/attachments",
		authz.RequirePermission(d.Checker, "task.read")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.UploadTaskAttachment))))

	// TẢI VỀ — `task.read`, the key of the timeline the file is shown on. A file on a log entry is part of
	// the task every reader sees; a file not yet attached only its uploader may fetch (domain.MayDownload).
	// Another commune's, another task's → 404, one answer.
	//
	// THE REPLY IS A LINK, NOT THE BYTES: a presigned GET on OBJECT_STORAGE_PUBLIC_ENDPOINT valid for
	// storage.MaxDownloadTTL, `attachment` for PDF, served from the object store's domain and never the
	// commune's (ADR 0052 §4). A bearer credential for its lifetime — `Cache-Control: no-store`, never
	// logged. NOT AUDITED: staff reading a staff file inside their own commune (see DownloadLink).
	//
	// @summary  Liên kết tải về một tệp đính kèm của nhiệm vụ (sống tối đa 15 phút)
	// @screen   02-nhiem-vu §5.9
	// @reply    200 taskAttachmentDownloadOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/tasks/{ma}/attachments/{id}/download",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.TaskAttachmentDownload)))

	// GỠ TỆP (user decision 07/10/2026) — a SOFT DELETE of the file's stored_file row with a mandatory
	// reason, plus a timeline line saying which file went and why, in ONE transaction with the audit
	// entry (`go_tep_nhiem_vu`). The object is NOT removed (ADR 0052 §6, §7; rule 7) and the link to its
	// log entry stays (append-only); every read path already skips a deleted row.
	//
	// `task.read` AT THE GATE, and who may actually remove is decided on the locked row
	// (domain.AttachmentRemovalRightFor): THE UPLOADER, OR A HOLDER OF `task.update` — attached to an
	// entry or not. A file the caller cannot see (another task's, another commune's, somebody else's
	// draft, unknown, already removed) is 404, one answer; a visible file without the right is 403; a file
	// under legal hold is 409. `task.read` and `task.update` are seeded (service-identity/migrations/
	// 0001_init.sql:304, :305); NO KEY WAS INVENTED (rule 5, invariant 3c).
	//
	// A BODY ON A DELETE, DELETE /api/v1/tasks/{ma}'s reason: the reason is mandatory, and the query
	// string would put free text about a government record into every access log and proxy cache.
	//
	// idem.KhongCan — the locked read sees only live rows, so a second removal is a 404 and cannot
	// overwrite who removed the file or why; it writes no second timeline line.
	//
	// @summary  Gỡ một tệp đính kèm khỏi nhiệm vụ (xoá mềm, lý do bắt buộc) — người đã tải lên hoặc cán bộ có quyền cập nhật nhiệm vụ
	// @screen   02-nhiem-vu §5.9
	// @request  taskAttachmentRemoveIn
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/tasks/{ma}/attachments/{id}",
		authz.RequirePermission(d.Checker, "task.read")(
			idem.KhongCan("gỡ lần hai một tệp đã gỡ trả 404: câu đọc khoá dòng chỉ thấy tệp chưa gỡ nên không ghi đè người gỡ, lý do, và không thêm dòng nhật ký thứ hai")(
				http.HandlerFunc(h.RemoveTaskAttachment))))

	// GIAO VIỆC MỚI (§7) — `task.create`, seeded at 0001_init.sql:301 ("Tạo nhiệm vụ").
	//
	// idem.Required(MoKhiHong), AND WHICH LAYER IS ACTUALLY PROTECTING THIS — the question
	// skills/rest-api-design §4 says to answer at the route. The real guard is
	// `UNIQUE (tenant_id, ma)`, which counts soft-deleted rows: two tasks carrying one register
	// number CANNOT EXIST, whatever happens to Redis. The idempotency key is the second, independent
	// layer — it is what stops a double-submitted form from producing two tasks with two DIFFERENT
	// minted numbers, which the unique key cannot see anything wrong with.
	//
	// MoKhiHong AND NOT DongKhiHong: with the unique key underneath, a cache outage cannot produce a
	// duplicate NUMBER, and refusing a commune mid-morning would pay with an outage for a risk that
	// is largely covered. THE RESIDUAL RISK IS STATED RATHER THAN GLOSSED: while the cache is down, a
	// double submit can mint NV20 and NV21 for one piece of work, and the spare is soft-deleted by
	// hand — a visible, recoverable nuisance, unlike a refused register.
	//
	// 409 COVERS TWO DIFFERENT THINGS and the code in the body tells them apart: `code_taken` (a
	// hand-typed number already issued, soft-deleted rows included) and `task_tree` (the named parent
	// is gone, or the move would close a cycle).
	//
	// 400 also answers a body that still sends `lead_unit` or `monitor` (ADR 0065 NV5, user decision
	// 30/09/2026): one role — the lead unit is `unit`, the monitoring officer is `assignee`.
	//
	// @summary  Giao việc mới — tạo một nhiệm vụ, tự sinh mã theo dãy NV của xã hoặc nhận mã tự nhập
	// @screen   02-nhiem-vu §7
	// @request  taoNhiemVuVao
	// @reply    201 nhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/tasks",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.TaoNhiemVu))))

	// SỬA THÔNG TIN (§5.4's ✎ Sửa) — `task.update`, seeded at 0001_init.sql:305 ("Cập nhật tiến độ").
	//
	// PATCH AND NOT PUT: four editable fields have a meaningful zero, so a full replacement cannot
	// tell "not mentioned" from "set to zero" — see suaNhiemVuVao.
	//
	// ⚠ IT CANNOT MOVE THE DEADLINE AND IT CANNOT REWRITE "Lãnh đạo giao việc". The first is the
	// extension flow's act, decided by the leader; the second IS the approver under ADR 0038, so a
	// holder of this key able to rewrite it could name themselves the approver of their own
	// extension requests. Both absences are enforced by petstore.SuaNhiemVu having no such field.
	//
	// idem.KhongCan, AND THE REASON IS A PROPERTY OF THE USE CASE RATHER THAN A HOPE: app.Sua
	// compares the row it read against the row it would write and, when nothing moved, writes
	// NOTHING — no UPDATE and no audit entry. Were that comparison removed, this declaration would
	// become a lie and the second request would file an entry saying nothing changed.
	//
	// OPTIMISTIC LOCKING, OPTIONAL (28/09/2026): a body carrying `expected_updated_at` — the task's
	// `updated_at` as the screen read it — is refused with 409 `task_changed` unless the row's
	// `cap_nhat_luc`, read under the lock, is that instant; absent keeps today's last-write-wins. A
	// body field rather than `If-Match` because tools/apidoc publishes body fields and no custom
	// request header (see suaNhiemVuVao). With the token, a double click's second request finds its
	// own first write and answers 409 `task_changed` — still no second write, so KhongCan holds.
	//
	// THE CODE IS NOT EDITABLE (ADR 0065 NV3, user decision 30/09/2026; migration 0024): a body that
	// sends `code` — any value, `null` included — is refused 400 `invalid_request`, and the trigger
	// refuses a change of `ma` underneath. The rename path of 28/09 is gone end to end.
	//
	// @summary  Sửa thông tin mô tả hoặc hạn của một nhiệm vụ — không đụng tới trạng thái hay phân công, không sửa được mã đã cấp (gửi `code` thì 400); sửa hạn là sửa cho đúng, không phải gia hạn: chưa có gia hạn được duyệt thì hạn ban đầu đi theo, đã có thì giữ nguyên; tuỳ chọn kèm `expected_updated_at` để chặn ghi đè (409 khi đã có người sửa)
	// @screen   02-nhiem-vu §5.4
	// @request  suaNhiemVuVao
	// @reply    200 nhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/tasks/{ma}",
		authz.RequirePermission(d.Checker, "task.update")(
			idem.KhongCan("app.Sua so dòng đọc được với dòng sắp ghi và KHÔNG ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaNhiemVu))))

	// CHUYỂN TRẠNG THÁI — `task.read` at the gate, the HOLDER RULE in the use case, `task.approve` for
	// the moves that need it.
	//
	// THE GATE IS `task.read` SINCE 28/09/2026, AND THAT IS vigov-require a37ec96 (user decision: "base
	// on require"). a37ec96 moved its progress/status route from `task.update` to `task.read` because
	// the answer depends on THIS task: the ASSIGNEE may move their own task without the commune-wide
	// key, a holder of `task.update` may move any task, and a related person (assigner, author) may
	// only write the log. Decided on the row read FOR UPDATE (app.DoiTrangThai,
	// domain.CheckMayChangeStatus); refused with 403 `ErrStatusNeedsHolder`. This WIDENS the route to
	// the assignee — deliberately; before, a specialist handed a task could not report on it at all.
	//
	// `task.approve` IS STILL ON TOP for three moves: the sign-off `cho-duyet` → `hoan-thanh`, the reopen
	// and the return from review (domain.NeedsApproval), consulted in the handler and decided in the use
	// case. SINCE 30/09/2026 (ADR 0065 NV1) review is optional: the assignee without the key may complete
	// straight from `dang-thuc-hien`, but work already at `cho-duyet` waits for a reviewer. Every
	// completion still needs the whole sub-tree finished (ADR 0037 decision 4), and a reopen of a
	// sub-task under a finished parent answers 409 `parent_completed` (NV2). NO KEY IS INVENTED:
	// `task.read`, `task.update` and `task.approve` are all seeded (0001_init.sql).
	//
	// THE SAME NARROWER KEY GUARDS ONE MORE MOVE: `cho-duyet` → `dang-thuc-hien`, "Trả lại để làm tiếp"
	// (owner decision 2026-09-27), which also requires a non-empty `note` as its reason. Decided in
	// app.DoiTrangThai on the locked row, because `dang-thuc-hien` is also the ordinary step from
	// `da-tiep-nhan` and only the CURRENT status tells the two apart. AND A THIRD since 28/09/2026: the
	// REOPEN `hoan-thanh` → `dang-thuc-hien` (require 52ec9b5), which clears `ngay_hoan_thanh` and keeps
	// the cleared instant in the timeline row and the audit entry (domain.NeedsApproval names all three).
	// Since P12 (28/09/2026) the reopen ALSO requires a non-empty `note` as its reason, like the return —
	// 400 without one, after the permission check.
	//
	// THE TARGET IS ON THE WIRE, unlike the petition path's `…/status`. The lifecycle (require
	// 52ec9b5's table, domain/nhiem_vu.go) BRANCHES at every state, so there is no single "next" for
	// the server to choose — what the server owns is the MAP, published per task as
	// `allowed_transitions`, and a move the map does not have is refused with a 409.
	//
	// "CẬP NHẬT VÀ GIAO VIỆC" (user decision 07/10/2026): the body may carry an OPTIONAL `handover` —
	// the shape of POST …/assignment's body — and OPTIONAL `attachments` (completed uploads of the same
	// officer for this task). Both ride in the move's ONE transaction (app/task_status_handover.go). The
	// handover lands the task in `status`, NOT `moi-giao` (the standalone assignment act is unchanged);
	// it is allowed to a holder of `task.assign` — asked here with Checker.Allows, a seeded key — OR the
	// task's current assignee, 403 otherwise; a finished task is refused 409 `task_state` as the
	// assignment act refuses it; identity refusing the unit/assignee is 400, identity down 503. The files
	// are linked to the move's own timeline row. Neither field = the move exactly as before.
	//
	// idem.KhongCan: the UPDATE carries the expected status, so a double click moves the task exactly
	// one step and the second request answers 409 — WITH a handover or files too: the second request is
	// refused (409, or 403 once the holder moved) before anything is written, and a file can be linked
	// only once.
	//
	// @summary  Chuyển trạng thái một nhiệm vụ theo vòng đời, kèm ghi nhật ký — hoàn thành cần mọi việc con đã xong, và cần quyền duyệt nếu việc đang chờ duyệt; trả lại để làm tiếp cần quyền duyệt và lý do; mở lại việc đã hoàn thành cần quyền duyệt, lý do, và việc cha chưa hoàn thành; tuỳ chọn kèm giao việc (`handover`, cho người có quyền giao nhiệm vụ hoặc người đang thực hiện — nhiệm vụ sang đúng trạng thái đã chọn, không về `moi-giao`) và tệp đính kèm (`attachments`) gắn vào dòng nhật ký của lần chuyển này, tất cả trong một giao dịch
	// @screen   02-nhiem-vu §6
	// @request  doiTrangThaiVao
	// @reply    200 nhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error parent_completed task_state task_tree
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/tasks/{ma}/status",
		authz.RequirePermission(d.Checker, "task.read")(
			idem.KhongCan("câu UPDATE mang trạng thái đang chờ, nên bấm hai lần vẫn chỉ chuyển đúng một bước và lần thứ hai trả 409")(
				http.HandlerFunc(h.DoiTrangThaiNhiemVu))))

	// GIAO LẠI / CHUYỂN TIẾP (§5.4, §10) — `task.assign`, seeded at service-identity/migrations/0001_init.sql:307 ("Giao nhiệm vụ").
	//
	// OWNER DECISION 28/09/2026: "Chuyển tiếp" is the SAME task handed to another unit or person, and
	// it is one act with "đổi bộ phận / người thực hiện". A change of unit or assignee sends the task
	// back to `moi-giao` so the new holder acknowledges it.
	//
	// ONE ROLE SINCE ADR 0065 NV5 (user decision 30/09/2026): "Cơ quan chủ trì tham mưu" IS `unit` and
	// "Chuyên viên theo dõi" IS `assignee`, so changing the monitoring officer is changing `assignee`
	// here. A body that still sends `lead_unit` or `monitor` is refused 400 `invalid_request`.
	// THE DEADLINE NEVER MOVES HERE — only an approved extension moves it (ADR 0038).
	//
	// `assignment` IS THE PETITION PATH'S NOUN (POST /api/v1/citizen-reports/{maTraCuu}/assignment) and
	// the one skills/rest-api-design §3 names for this very act. It has NO row in the URL table of
	// kb/00-foundation/ubiquitous-language.md — raised as a finding, not added here.
	//
	// 400 covers a body naming no field, an empty `unit`, `lead_unit` / `monitor`, a staff code identity does not accept in
	// this commune (ONE answer for unknown / another commune / locked), and `note` too long. 409 covers
	// a terminal task (`task_state`) and a request that changes nothing (`no_change`). 503: identity
	// could not be asked, so nothing was written.
	//
	// idem.KhongCan: the UPDATE carries the expected status, and a second identical request finds
	// every field already applied — the use case writes nothing and answers 409 `no_change`. One row,
	// one timeline entry, one audit entry.
	//
	// @summary  Giao lại một nhiệm vụ (chuyển tiếp) cho bộ phận/người khác — đổi bộ phận/người thực hiện thì về `moi-giao`, hạn giữ nguyên; chủ trì và theo dõi đã gộp vào bộ phận/người thực hiện
	// @screen   02-nhiem-vu §5.4
	// @request  taskAssignmentIn
	// @reply    200 nhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/tasks/{ma}/assignment",
		authz.RequirePermission(d.Checker, "task.assign")(
			idem.KhongCan("câu UPDATE mang trạng thái đang chờ và lần gửi thứ hai thấy mọi trường đã đúng nên không ghi gì, trả 409 — đúng một lần giao, đúng một dòng nhật ký, đúng một vết")(
				http.HandlerFunc(h.ReassignTask))))

	// XOÁ (§11.5) — `task.delete`, seeded at 0001_init.sql:302 ("Xoá nhiệm vụ khỏi sổ").
	//
	// THIS IS A SOFT DELETE AND THE METHOD IS THE ONLY THING THAT SAYS OTHERWISE. The row stays with
	// `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), its number stays taken
	// for ever, and its timeline survives — §11.5 asks for exactly that ("nên xoá mềm để giữ nhật
	// ký"). DELETE is still the right method: the resource is gone from every read path.
	//
	// ⚠ IT ANSWERS 409 WHILE THE TASK STILL HAS LIVE CHILDREN. That is ADR 0037 decision 3, and the
	// body says how many — refusing without saying what is in the way sends an officer hunting.
	//
	// A BODY ON A DELETE, and the alternative was worse: the reason is mandatory, and the query
	// string would put free text about a government record into every access log and proxy cache.
	//
	// idem.KhongCan — deleting an already-deleted task is a 404 either way, and the second request
	// cannot overwrite who deleted it or why: the UPDATE carries `AND deleted_at IS NULL`.
	//
	// @summary  Xoá mềm một nhiệm vụ khỏi sổ, kèm lý do bắt buộc — từ chối khi còn việc con chưa xoá
	// @screen   02-nhiem-vu §11
	// @request  xoaNhiemVuVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/tasks/{ma}",
		authz.RequirePermission(d.Checker, "task.delete")(
			idem.KhongCan("xoá một nhiệm vụ đã xoá cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.XoaNhiemVu))))

	// ĐỀ NGHỊ LÙI HẠN (§5.8) — `task.update`, NOT `task.extend`.
	//
	// ⚠ THAT CHOICE IS ADR 0038'S WHOLE POINT. `task.extend` is labelled "Duyệt gia hạn" in the
	// `quyen` table — it is the right to DECIDE. Guarding this route with it would mean only people
	// who can approve an extension may ask for one, which is the opposite of §5.8: the box sits on
	// the drawer of the officer doing the work.
	//
	// idem.KhongCan: at most one request may be pending per task —
	// `UNIQUE (tenant_id, nhiem_vu_id, moc_cho_duyet)` — so a double submit answers 409 rather than
	// filing a second request the leader could approve twice.
	//
	// @summary  Gửi đề nghị lùi hạn cho một nhiệm vụ — hạn mới phải muộn hơn hạn đang có, kèm lý do bắt buộc
	// @screen   02-nhiem-vu §5.8
	// @request  deNghiLuiHanVao
	// @reply    201 deNghiLuiHanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/tasks/{ma}/extensions",
		authz.RequirePermission(d.Checker, "task.update")(
			idem.KhongCan("mỗi nhiệm vụ chỉ có một đề nghị đang chờ duyệt — khoá duy nhất `(tenant_id, nhiem_vu_id, moc_cho_duyet)` làm lần gửi thứ hai trả 409 chứ không sinh đề nghị thứ hai")(
				http.HandlerFunc(h.DeNghiLuiHanNhiemVu))))

	// LỊCH SỬ LÙI HẠN (§5.8, user decision 07/10/2026) — every request of ONE task, every status,
	// newest first, each with the decider's note (`decision_note`, migration 0031; null before it).
	// The READ half of the two writes around it; the drawer shows it.
	//
	// `task.read`, THE KEY OF THE TASK READS (seeded at service-identity/migrations/0001_init.sql:304)
	// — the user's decision, and the key the queue already opened `reason` to. NO KEY WAS INVENTED
	// (rule 5, invariant 3c).
	//
	// 404 is the single answer GET /api/v1/tasks/{ma} gives, for the same three causes — the task is
	// read first through the same reader, and the requests only after. 401 is RequirePermission's answer
	// to no session AND to a session of another commune. Soft-deleted requests are excluded (rule 7).
	// The pending queue GET /api/v1/task-extensions and its count are NOT affected. NO idem.*
	// DECLARATION: a GET changes no state. NO AUDIT ENTRY.
	//
	// @summary  Lịch sử đề nghị lùi hạn của một nhiệm vụ — mọi trạng thái, mới nhất trước, kèm ghi chú của người quyết định, phân trang theo con trỏ
	// @screen   02-nhiem-vu §5.8
	// @reply    200 page.Result[deNghiLuiHanRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/tasks/{ma}/extensions",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.TaskExtensionHistory)))

	// QUYẾT ĐỊNH LÙI HẠN (§5.8, ADR 0038) — `task.extend`, seeded at 0001_init.sql:303.
	//
	// ⚠ THE KEY IS THE FIRST OF TWO LAYERS AND IS NOT THE WHOLE ANSWER. Rule 5 checks
	// `(tenant_id, role, permission)` and has no "which record" dimension, so holding `task.extend`
	// says only that this account may touch extensions AT ALL. Whether it may decide THIS one —
	// `Principal.Ma == nhiem_vu.lanh_dao_giao_viec_ma`, compared as two STAFF BUSINESS CODES — is
	// decided in domain.DuocDuyetLuiHan, inside the transaction, on the task row read under the lock.
	// Drop the gate and anybody may call the route; drop the second layer and every leader holding
	// the key decides every task in the commune.
	//
	// A TASK THAT NAMES NO LEADER IS REFUSED, not routed to `nguoi_tao_ma`. That is ADR 0038's open
	// question failing CLOSED, and the sentence names what is missing — the only thing the commune
	// can act on. 403 covers it, together with the independent rule that nobody decides their own
	// request.
	//
	// idem.KhongCan: the UPDATE carries `trang_thai = 'cho-duyet'`, so one request gets one decision
	// and the second attempt answers 409.
	//
	// @summary  Lãnh đạo giao việc duyệt hoặc từ chối một đề nghị lùi hạn — duyệt thì đổi hạn xử lý, hạn ban đầu giữ nguyên
	// @screen   02-nhiem-vu §5.8
	// @request  quyetDinhLuiHanVao
	// @reply    200 deNghiLuiHanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/tasks/{ma}/extensions/{deNghiID}/decision",
		authz.RequirePermission(d.Checker, "task.extend")(
			idem.KhongCan("câu UPDATE mang `trang_thai = 'cho-duyet'`, nên lần gửi thứ hai không khớp dòng nào — đúng một đề nghị, đúng một quyết định, đúng một vết")(
				http.HandlerFunc(h.QuyetDinhLuiHanNhiemVu))))

	// HÀNG CHỜ DUYỆT LÙI HẠN (§5.8) — the READ of every pending request of the commune. A top-level
	// resource like task-types / task-priorities: the queue spans every task, so it is not a sub-route
	// of one (decided by the project owner, 27/09/2026).
	//
	// `task.read`, THE KEY OF THE TASK READS ABOVE (seeded at service-identity/migrations/
	// 0001_init.sql:304), AND NOT `task.extend` — the project owner's decision, 27/09/2026. The list
	// decides nothing, but it IS the first GET that returns a request's reason text (`ly_do`); see the
	// header of de_nghi_lui_han_cho_duyet.go. WHO MAY DECIDE is
	// unchanged — `task.extend` at the decision route's gate plus ADR 0038's named-leader rule inside
	// it. NO KEY WAS INVENTED (rule 5, invariant 3c).
	//
	// `approver=me` narrows to the requests whose task names the CALLER as leader, resolved from the
	// session principal's staff code and never from the request; any other value is 400, and 500 covers
	// `approver=me` on a principal with no staff code — a wiring fault, refused rather than silently
	// widened to the whole commune. 401 is RequirePermission's answer to no session AND to a session of
	// another commune. NO idem.* DECLARATION: a GET changes no state.
	//
	// `task=<register number>` narrows to ONE task's pending requests (§5.8's block inside its drawer);
	// a number matching no live task of this commune is an empty page, the same answer as a task with
	// nothing pending. 400 when it is longer than an issued number can be. Combines with `approver=me`.
	//
	// @summary  Hàng chờ duyệt lùi hạn của xã — các đề nghị đang chờ, cũ nhất trước, phân trang theo con trỏ; `approver=me` chỉ lấy đề nghị mà mình là lãnh đạo giao việc; `task=NV19` chỉ lấy đề nghị của một nhiệm vụ
	// @screen   02-nhiem-vu §5.8
	// @reply    200 page.Result[deNghiChoDuyetRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/task-extensions",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.DanhSachDeNghiLuiHan)))

	// SỐ ĐỀ NGHỊ LÙI HẠN ĐANG CHỜ (ADR 0071, Sổ tay lãnh đạo — huy hiệu nhóm "Duyệt lùi hạn") — the size
	// of the queue above under EXACTLY its filters (one parser, pendingExtensionFilterFromRequest; one
	// predicate, petstore.pendingExtensionFilter). A sibling count route and not a `total` on the
	// queue: page.Result carries none by design, and task-counts is the precedent (task_counts.go).
	//
	// `task.read`, THE QUEUE'S OWN KEY: the number reveals nothing the queue does not, and a second key
	// would let an account see a badge over a list it may not read. NO KEY WAS INVENTED (rule 5,
	// invariant 3c). `approver=me` and `task=` behave as on the queue — the same 400s, the same 500 for
	// a principal with no staff code. Paging parameters are ignored. 401 is RequirePermission's answer
	// to no session AND to a session of another commune. NO idem.* DECLARATION: a GET. NO AUDIT ENTRY.
	//
	// @summary  Số đề nghị lùi hạn đang chờ duyệt, cùng bộ lọc với hàng chờ (`approver=me` · `task=NV19`) — số trên huy hiệu "Duyệt lùi hạn" của Sổ tay lãnh đạo
	// @screen   03-so-tay-lanh-dao §3
	// @reply    200 taskExtensionCountOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/task-extension-counts",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.TaskExtensionCount)))

	// --- THE LEADERSHIP OVERVIEW (/tong-quan, docs/ui-ux/01). FOUR READ ROUTES ---------------------
	//
	// TWO KEYS ON EACH, AND BOTH ARE REAL GUARDS: `report.read` — the overview is a report — AND the
	// module's own read key, `task.read` or `feedback.read`. A leader who may read reports but not the
	// petition register must not learn its figures from a tile, and an officer who may read the
	// register but not reports must not get the report through the back door. Both keys are seeded
	// (service-identity/migrations/0001_init.sql); none was invented (rule 5, invariant 3c).
	//
	// EXPRESSED AS TWO NESTED authz.RequirePermission, because core/authz has no "all of" guard and
	// adding one is outside this service. `report.read` IS THE INNER ONE ON PURPOSE: tools/apidoc
	// records the LAST RequirePermission it visits in a statement, which is the innermost, so the
	// contract names the overview's distinguishing key. The module key is not in openapi.json — a
	// known limit of the generator, reported, not papered over with a comment the generator reads.
	//
	// 401 is the answer to no session AND to a session of another commune (authz.xacNhanXa refuses
	// before any permission is read); 403 to an account missing EITHER key.
	//
	// NO idem.* DECLARATION: a GET changes no state. NO AUDIT ENTRY: counts and codes of one commune,
	// no personal data (rule 6, invariant 7).

	// @summary  Tổng quan nhiệm vụ của xã — số đang thực hiện · quá hạn · tạm dừng (hiện trạng) và hoàn thành · mẫu đúng hạn · đúng hạn trong kỳ [from, to)
	// @screen   01-tong-quan-dieu-hanh §4.1
	// 400: `from`/`to` missing, not RFC 3339, or from >= to. Counts only — the client divides and shows
	// a dash for an empty sample.
	//
	// @reply    200 taskSummaryOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/task-summary",
		authz.RequirePermission(d.Checker, "task.read")(
			authz.RequirePermission(d.Checker, "report.read")(
				http.HandlerFunc(h.TaskSummary))))

	// THE /bao-cao TABLE "Tình hình thực hiện theo bộ phận" — the task figures above, split by
	// `bo_phan_id`, counted live in this service like /tong-quan (ADR 0053 §1). The SAME two keys,
	// nested the same way, for the same reason (ADR 0053 §2). Every department with a task in scope is
	// listed — no ranking, no top-N; tasks with no department are one row with `org_unit_id: ""`.
	//
	// @summary  Tình hình thực hiện nhiệm vụ theo bộ phận trong kỳ [from, to) — mỗi bộ phận: số việc đang nắm trong kỳ · hoàn thành · mẫu đúng hạn · đúng hạn (theo kỳ) và quá hạn (hiện trạng)
	// @screen   13-bao-cao §5
	// 400: `from`/`to` missing, not RFC 3339, or from >= to. Counts only — names and zero rows come from
	// GET /api/v1/org-units.
	//
	// @reply    200 taskUnitSummaryOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/task-unit-summary",
		authz.RequirePermission(d.Checker, "task.read")(
			authz.RequirePermission(d.Checker, "report.read")(
				http.HandlerFunc(h.TaskUnitSummary))))

	// `feedback.restricted` absent -> the `can-bo` field is excluded from EVERY figure, as it is from the
	// register list the figures drill down into. It changes no status code.
	//
	// THREE MORE STOCK FIGURES (2026-10-02, docs/ui-ux/09 §3): `rating_sample` + `rating_sum` (the average
	// citizen rating, divided by the client), `low_rating` (1–2 stars, the `rating_max=2` list) and
	// `publication_pending` (`cho-duyet`, never `can-bo`). Optional on the wire; the five above unchanged.
	//
	// @summary  Tổng quan phản ánh của xã — số đang xử lý (hiện trạng) và tiếp nhận · mẫu đúng hạn · đúng hạn · trễ hạn trong kỳ [from, to); hiện trạng: số phiếu được dân chấm sao và tổng số sao, số phiếu bị đánh giá thấp (1–2 sao), số phiếu chờ kiểm duyệt công khai
	// @screen   01-tong-quan-dieu-hanh §4.5
	// @reply    200 citizenReportSummaryOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/citizen-report-summary",
		authz.RequirePermission(d.Checker, "feedback.read")(
			authz.RequirePermission(d.Checker, "report.read")(
				http.HandlerFunc(h.CitizenReportSummary))))

	// BOUNDED AT TEN AND NOT CURSOR-PAGINATED, with the reason: the panel is a top-ten by design
	// (§5, "tối đa 10 mục") and the full set is the register list with `metric=overdue`. `limit` above
	// ten is clamped to ten.
	//
	// 503 when identity cannot answer the working-hours question behind `critical` — refused rather than
	// answered with `critical: false` for every row.
	//
	// @summary  Nhiệm vụ quá hạn cần xử lý ngay — tối đa 10, trễ lâu nhất trước; mỗi dòng: mã, loại, hạn đã lỡ, có nghiêm trọng không
	// @screen   01-tong-quan-dieu-hanh §5
	// @reply    200 overdueQueueOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/overdue-tasks",
		authz.RequirePermission(d.Checker, "task.read")(
			authz.RequirePermission(d.Checker, "report.read")(
				http.HandlerFunc(h.OverdueTasks))))

	// Same shape as the task panel. A petition is overdue here when it is still unclassified past its
	// classification ceiling, or when its work is not done past its resolve deadline; `kind` says which.
	// NO CONTENT AND NO REPORTER on the wire. `feedback.restricted` absent -> `can-bo` excluded.
	//
	// `late_working_seconds` (optional, 2026-10-02): working time since the missed deadline, measured
	// by identity MeasureWorkingHours in ONE call for the page, up to the same `now` the overdue
	// predicate used. Identity or the calendar unavailable -> ABSENT on every row and the page is still
	// 200 (unlike `critical`, whose absence would be a guess and is therefore a 503). Never stored.
	//
	// @summary  Phản ánh quá hạn cần xử lý ngay — tối đa 10, trễ lâu nhất trước; mỗi dòng: mã tra cứu, lĩnh vực, hạn đã lỡ (phân loại hay xử lý xong), có nghiêm trọng không, số giây làm việc đã trễ (khi đo được)
	// @screen   01-tong-quan-dieu-hanh §5
	// @reply    200 overdueQueueOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/overdue-citizen-reports",
		authz.RequirePermission(d.Checker, "feedback.read")(
			authz.RequirePermission(d.Checker, "report.read")(
				http.HandlerFunc(h.OverdueCitizenReports))))

	// --- MEETING MINUTES AND THEIR CONCLUSIONS. ONE READ ROUTE AND THREE WRITE ROUTES -----------
	//
	// `meetings` IS NOT FROM kb/00-foundation/ubiquitous-language.md — that table has no row for
	// `bien_ban_hop` — and it is not translated on the spot either (ADR 0011 forbids both). It is
	// the noun the RUNNING sibling implementation serves at
	// `apps/api/app/modules/tasks/router.py:45`, read under the project owner's instruction of
	// 2026-09-23. The missing row is REPORTED as a finding for the session that owns that file,
	// never written from a route. The full reasoning is on internal/http/bien_ban_hop.go.
	//
	// `task.read` ON THE READ AND `task.create` ON THE THREE WRITES, and no `meeting.*` key is
	// invented: the `quyen` table has none (service-identity/migrations/0001_init.sql:273-305) and a
	// key no migration seeds is a right no administrator can grant — 403 to every account, forever,
	// with the tests still green (rule 5, invariant 3c). Everything this screen shows is either the
	// origin of a task or a count of tasks. Whether a commune wants "may record minutes" separated
	// from "may create tasks" is a question for the CUSTOMER — open question #27, whose `ask_before`
	// list names exactly this case — and it is reported as a finding. The full reasoning, including
	// what the shared key costs and why `document.create` would be worse, is on
	// internal/http/bien_ban_hop_ghi.go.
	//
	// THE LIFECYCLE ROUTES (edit, delete, signature, conclusion edit/delete, no-task marker) follow
	// the split route below; the decisions they carry are the user's of 25/09/2026 (migration 0012).
	//
	// ⚠ THE DEADLINE ON A SPLIT TASK IS TYPED, NOT DERIVED. §3 suggests a date when the conclusion's
	// sentence contains one ("báo cáo trước ngày 20/8") — that suggestion is the SCREEN's, and what
	// reaches `han_xu_ly` is whatever the person confirmed. Nothing on this path performs
	// working-hours arithmetic; that has one implementation, in identity (rule 10, forbidden #2).

	// @summary  Danh sách biên bản họp của xã — mỗi biên bản kèm các kết luận và bộ đếm nhiệm vụ đã tách / đã xong
	// @screen   04-bien-ban-hop §2, §5
	// NO FILTER AND NO SEARCH PARAMETER, because §2's screen has neither: one vertical list of
	// cards, newest first. The only query parameters are the page cursor's.
	//
	// 400 is page.Parse refusing a cursor or a sort key; the body never echoes what was sent.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @reply    200 page.Result[bienBanRa]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/meetings",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.DanhSachBienBan)))

	// MỘT BIÊN BẢN, ĐẦY ĐỦ — `task.read`, the list's key, for the list's reason (everything shown is
	// the origin of a task or a count of tasks; no `meeting.*` key exists and none is invented —
	// rule 5, invariant 3c; open question #27 for splitting it).
	//
	// 404 IS ONE BODY FOR THREE CAUSES — unknown id, another commune's id, soft-deleted minutes.
	// Telling them apart tells a caller which minutes exist in a register they are not reading.
	//
	// NO AUDIT ENTRY: no citizen personal data on the record, no cross-commune read (rule 6,
	// invariant 7). NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Một biên bản họp đầy đủ — nội dung, thành phần, thư ký, trạng thái ký, thông báo kết luận, biên bản bổ sung, kết luận kèm trạng thái suy ra
	// @screen   04-bien-ban-hop §2, §5
	// @reply    200 bienBanRa
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/meetings/{id}",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.DocBienBan)))

	// NHIỆM VỤ TÁCH TỪ MỘT KẾT LUẬN — `task.read`, the key of GET /api/v1/tasks, because the rows ARE
	// task-register rows (the register's own row shape, nhiemVuRa).
	//
	// THE WHOLE LIST, BOUNDED BY petstore.TranNhiemVuMotKetLuan AND REFUSED PAST IT (500), never
	// truncated: a short list is a task gone from its conclusion while the counter beside it still
	// counts it. Soft-deleted tasks, another commune's tasks and another conclusion's tasks are
	// excluded in the store's WHERE clause.
	//
	// 400 is a non-numeric or non-positive `{stt}`. 404 is ONE body for an unknown / other-commune /
	// removed meeting and an unknown / removed conclusion.
	//
	// @summary  Các nhiệm vụ còn hiệu lực tách từ một kết luận họp — hàng của sổ nhiệm vụ, theo thứ tự tách
	// @screen   04-bien-ban-hop §2
	// @reply    200 nhiemVuKetLuanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/meetings/{id}/conclusions/{stt}/tasks",
		authz.RequirePermission(d.Checker, "task.read")(
			http.HandlerFunc(h.NhiemVuCuaKetLuan)))

	// NHẬP BIÊN BẢN (§4) — `task.create`, seeded at 0001_init.sql:301 ("Tạo nhiệm vụ").
	//
	// idem.Required(MoKhiHong), AND WHICH LAYER IS ACTUALLY PROTECTING THIS — the question
	// skills/rest-api-design §4 says to answer at the route. THE HONEST ANSWER HERE IS: ONLY THE
	// IDEMPOTENCY KEY. Unlike POST /api/v1/tasks there is no unique key underneath — migration 0007
	// refuses one on `so_hieu`, which is typed by hand, optional, and restarts every year — so a
	// double-submitted form produces two meeting cards and nothing in the database objects.
	//
	// MoKhiHong AND NOT DongKhiHong, with the residual risk stated rather than glossed: recording
	// minutes is an INTAKE path, not one of the acts with legal consequence the skill reserves
	// DongKhiHong for (money, an issued document number, closing a commitment to a citizen). While
	// the cache is down a double submit leaves a duplicate card — visible, and one somebody must
	// live with until §6's delete route exists, because rule 7 forbids removing it with a command.
	// Refusing the whole register mid-morning would be the larger harm.
	//
	// @summary  Nhập một biên bản họp, kèm các kết luận đã gõ trên biểu mẫu — kết luận đánh số ① ② ③ theo thứ tự nhập
	// @screen   04-bien-ban-hop §4
	// @request  taoBienBanVao
	// @reply    201 bienBanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/meetings",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.TaoBienBan))))

	// THÊM MỘT KẾT LUẬN (§2's last row) — `task.create`.
	//
	// idem.Required(MoKhiHong) FOR THE SAME REASON AND WITH THE SAME GAP: the unique key
	// `(tenant_id, bien_ban_id, thu_tu)` does NOT protect this route, because the second request
	// mints the NEXT ordinal rather than colliding with the first. Two identical conclusions, ③ and
	// ④, are both perfectly valid to the database — which is exactly the shape an idempotency key
	// exists for.
	//
	// 404 COVERS THE MINUTES NOT EXISTING, another commune's minutes, and minutes that have been
	// removed. Telling them apart tells a caller which minutes exist in a register they are not
	// reading.
	//
	// @summary  Thêm một kết luận vào biên bản đã có — số thứ tự nối tiếp số ĐÃ CẤP, kể cả khi kết luận mang số đó đã bị xoá
	// @screen   04-bien-ban-hop §2, §7.2
	// @request  themKetLuanVao
	// @reply    201 ketLuanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/meetings/{id}/conclusions",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.ThemKetLuan))))

	// TÁCH KẾT LUẬN THÀNH NHIỆM VỤ (§3) — `task.create`, the same key POST /api/v1/tasks declares,
	// because this IS that act: it runs the same use case and mints from the same `NV…` series.
	//
	// ⚠ THE BODY IS THE FULL TASK FORM AND THE SERVER DERIVES NOTHING FROM THE CONCLUSION'S
	// SENTENCE. §3 pre-fills the "Giao việc mới" modal and waits for a person to confirm it; the
	// sibling implementation measured what guessing costs — right three times out of four, and the
	// fourth leaves a wrongly titled task in a register that cannot be deleted, only withdrawn.
	//
	// ⚠ `source` / `source_id` ARE NOT ON THE WIRE. §3 shows "Nguồn giao = Từ kết luận họp" as
	// locked, and here that is the pair being ABSENT from the request rather than validated on it:
	// the use case sets both from the conclusion named in the PATH, so there is no value a client
	// could send to point a task at a record of its choosing.
	//
	// idem.Required(MoKhiHong), the same declaration POST /api/v1/tasks carries and for the same
	// reason: `UNIQUE (tenant_id, ma)` cannot see anything wrong with two tasks minted under two
	// DIFFERENT numbers for one piece of work, and §3 permits a conclusion to be split many times —
	// so a duplicate is indistinguishable from an intended second split without the key.
	//
	// 409 COVERS TWO DIFFERENT THINGS, exactly as on POST /api/v1/tasks: `code_taken` and
	// `task_tree`. They are mapped by the task register's own function, so one act answers one way
	// whichever door it came through. 400 also answers `lead_unit` / `monitor` in the body, as there
	// (ADR 0065 NV5).
	//
	// @summary  Tách một kết luận họp thành một nhiệm vụ — nhiệm vụ giữ liên kết ngược về kết luận gốc qua cặp nguồn giao
	// @screen   04-bien-ban-hop §3
	// @request  tachKetLuanVao
	// @reply    201 nhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/meetings/{id}/conclusions/{stt}/task",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.TachKetLuanThanhNhiemVu))))

	// SỬA BIÊN BẢN NHÁP / GHI THÔNG BÁO KẾT LUẬN SAU KHI KÝ — `task.create`, the key of the other
	// write routes of this register.
	//
	// ONE ROUTE, TWO BEHAVIOURS BY STATE, and that is the least surprising shape rather than a second
	// URL for one field: on a DRAFT every field of the body is editable; on SIGNED minutes only
	// `notice`, and only while none is recorded (migration 0012's trigger allows exactly that). A field
	// sent back with the value it already holds is not a change; any real change answers 409.
	//
	// idem.KhongCan: app.SuaBienBan compares the row read under the lock with the row it would write
	// and writes NOTHING when nothing moved; the notice write carries `tb_so_ky_hieu IS NULL`.
	//
	// @summary  Sửa biên bản họp còn nháp; với biên bản đã ký chỉ ghi được số/ngày Thông báo kết luận, một lần
	// @screen   04-bien-ban-hop §4
	// @request  suaBienBanVao
	// @reply    200 bienBanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/meetings/{id}",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.KhongCan("app.SuaBienBan không ghi gì khi không trường nào đổi, và câu ghi thông báo mang `tb_so_ky_hieu IS NULL`, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaBienBan))))

	// XOÁ BIÊN BẢN NHÁP — `task.create`. Soft delete, reason required in the body. 409 when signed
	// (a correction is supplementary minutes) or while any conclusion still has a live task (the
	// count is in the sentence — handle the tasks first).
	//
	// @summary  Xoá mềm một biên bản họp còn nháp, kèm lý do — từ chối khi đã ký hoặc còn nhiệm vụ trỏ về kết luận
	// @screen   04-bien-ban-hop §7.1
	// @request  xoaBienBanVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/meetings/{id}",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.KhongCan("câu UPDATE mang `AND deleted_at IS NULL`, nên lần xoá thứ hai trả 404 và không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.XoaBienBan))))

	// KÝ BIÊN BẢN — `task.approve` ("Duyệt hoàn thành", seeded), NOT `task.create`: signing makes the
	// record final and locks it, and the account that typed the minutes is not thereby the one who
	// may sign them. Body optional: `{notice?}`. Minutes with no conclusions may be signed (§7.3).
	//
	// idem.KhongCan: the UPDATE carries `trang_thai = 'du-thao'`, so a second request answers 409 and
	// never records a second signer or instant.
	//
	// @summary  Ký biên bản họp — chuyển nháp sang đã ký, khoá nội dung và kết luận; có thể kèm Thông báo kết luận
	// @screen   04-bien-ban-hop §4
	// @request  kyBienBanVao
	// @reply    200 bienBanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/meetings/{id}/signature",
		authz.RequirePermission(d.Checker, "task.approve")(
			idem.KhongCan("câu UPDATE mang `trang_thai = 'du-thao'`, nên lần ký thứ hai trả 409 — đúng một chữ ký, đúng một vết")(
				http.HandlerFunc(h.KyBienBan))))

	// SỬA MỘT KẾT LUẬN — `task.create`. Draft only; 409 while a live task points at it (decision 3:
	// the tasks quote this sentence). The no-task marker does not lock the text.
	//
	// @summary  Sửa nội dung một kết luận của biên bản nháp — khoá khi kết luận đã tách thành nhiệm vụ
	// @screen   04-bien-ban-hop §2
	// @request  suaKetLuanVao
	// @reply    200 ketLuanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/meetings/{id}/conclusions/{stt}",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.KhongCan("app.SuaKetLuan không ghi gì khi nội dung không đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaKetLuan))))

	// XOÁ MỘT KẾT LUẬN — `task.create`. Draft only, reason required, 409 while a live task points at
	// it. The ordinal is never reissued (the high-water mark counts deleted rows).
	//
	// @summary  Xoá mềm một kết luận của biên bản nháp, kèm lý do — số thứ tự đã cấp không cấp lại
	// @screen   04-bien-ban-hop §7.1
	// @request  xoaBienBanVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/meetings/{id}/conclusions/{stt}",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.KhongCan("câu UPDATE mang `AND deleted_at IS NULL`, nên lần xoá thứ hai trả 404 và không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.XoaKetLuan))))

	// DẤU "KHÔNG PHÁT SINH NHIỆM VỤ" — `task.create`. PUT sets it (a state, so a sub-resource that PUT
	// puts and DELETE removes, the `lockout` pattern): draft only, 409 while a live task points at the
	// conclusion; already set is a no-op 200. While it is set, POST …/task answers 409.
	//
	// @summary  Đánh dấu một kết luận "không phát sinh nhiệm vụ" — tính là hoàn thành; từ chối khi đã có nhiệm vụ
	// @screen   04-bien-ban-hop §2
	// @reply    200 ketLuanRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/meetings/{id}/conclusions/{stt}/no-task-marker",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.KhongCan("PUT đặt một trạng thái tuyệt đối; đã có dấu thì use case không ghi gì, nên lần thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.DanhDauKhongPhatSinh))))

	// BỎ DẤU — `task.create`, draft only (after signing the mark is part of the record). Not set is a
	// no-op 204.
	//
	// @summary  Bỏ dấu "không phát sinh nhiệm vụ" của một kết luận thuộc biên bản nháp
	// @screen   04-bien-ban-hop §2
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/meetings/{id}/conclusions/{stt}/no-task-marker",
		authz.RequirePermission(d.Checker, "task.create")(
			idem.KhongCan("chưa có dấu thì use case không ghi gì, nên lần bỏ dấu thứ hai không để lại vết nào")(
				http.HandlerFunc(h.BoDanhDauKhongPhatSinh))))

	// --- the commune adds a task type of its own -------------------------------------------
	//
	// TIER 1 ONLY, AND THE STORE IS WHAT MAKES THAT TRUE: `nguon` and `ma_nguon_re_nhanh` are
	// LITERALS in the INSERT, not parameters, so there is no value any layer above could pass. The
	// handler additionally answers 400 to a body naming `source` or `tier` — not as the defence,
	// but so a client learns it may not decide provenance instead of watching the field vanish.
	//
	// idem.Required(MoKhiHong), AND WHICH LAYER IS ACTUALLY PROTECTING THIS — the question
	// skills/rest-api-design §4 says to answer at the route. The real guard is
	// `UNIQUE (tenant_id, ma)`, which counts soft-deleted rows: a second row with the same code
	// CANNOT EXIST, whatever happens to Redis. The idempotency key is the second, independent
	// layer — it is what stops a double-submitted form from producing one row and one confusing
	// 409 instead of one row and a replayed 201.
	//
	// MoKhiHong and not DongKhiHong for exactly that reason: with the unique key underneath, a
	// cache outage cannot produce a duplicate catalogue row, so refusing a commune administrator
	// mid-configuration would be paying with an outage for a risk that is already covered. The
	// legal-consequence cases the skill reserves DongKhiHong for — money, issued document numbers,
	// closing a commitment to a citizen — are not this.
	//
	// 409 AND NOT 403 for a full catalogue or a taken code: the caller holds `admin.lookup` and is
	// allowed to manage the list. What is refused is this value against the state of the data.
	//
	// @summary  Thêm một loại nhiệm vụ của riêng xã vào danh mục
	// @screen   14-cau-hinh §5
	// @request  themLoaiNhiemVuVao
	// @reply    201 loaiNhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/task-types",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.ThemLoaiNhiemVu))))

	// --- the commune edits one row --------------------------------------------------------------
	//
	// PATCH AND NOT PUT: three of the four editable fields have a meaningful zero, so a full
	// replacement cannot tell "not mentioned" from "set to zero" — see suaLoaiNhiemVuVao.
	//
	// WHAT EACH TIER ALLOWS IS DECIDED PER FIELD AND PER TRANSITION, not per route: relabel and
	// reorder at every tier, disable at tiers 1 and 2, and `ma` nowhere. The refusal is a 409
	// naming the tier, and the trigger refuses the same thing underneath (ADR 0024).
	//
	// idem.KhongCan, AND THE REASON IS A PROPERTY OF THE USE CASE RATHER THAN A HOPE: app.Sua
	// compares the row it read against the row it would write and, when nothing moved, writes
	// NOTHING — no UPDATE and no audit entry. So the same request sent twice leaves one row in one
	// state and one entry in the ledger. Were that comparison removed, this declaration would
	// become a lie and the second request would file a second entry saying nothing changed.
	//
	// @summary  Sửa nhãn, thứ tự, trạng thái dùng hoặc đặt mặc định cho một loại nhiệm vụ
	// @screen   14-cau-hinh §5
	// @request  suaLoaiNhiemVuVao
	// @reply    200 loaiNhiemVuRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/task-types/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaLoaiNhiemVu))))

	// --- the commune retires one of its own rows ------------------------------------------------
	//
	// THIS IS A SOFT DELETE AND THE METHOD IS THE ONLY THING THAT SAYS OTHERWISE. The row stays,
	// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), and its `ma`
	// stays taken forever — an issued code is never reissued, because task records hold it as
	// a value and nothing rewrites them. DELETE is still the right method: the resource is gone
	// from every read path, which is what the caller is asking for.
	//
	// TIER 1 ONLY. A `he-thong` row answers 409 and is told to use `Tắt` instead — the same
	// refusal the trigger makes, arriving first and in a sentence somebody can act on.
	//
	// A BODY ON A DELETE, and the alternative was worse: the reason is mandatory, and the query
	// string would put free text about a government record into every access log and proxy cache.
	//
	// idem.KhongCan — deleting an already-deleted row is a 404 either way, and the second request
	// cannot overwrite who deleted it or why: the UPDATE carries `AND deleted_at IS NULL`.
	//
	// @summary  Xoá mềm một mục danh mục do xã tự thêm, kèm lý do bắt buộc
	// @screen   14-cau-hinh §5
	// @request  xoaLoaiNhiemVuVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/task-types/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("xoá một dòng đã xoá cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.XoaLoaiNhiemVu))))
	// --- the commune adds a task priority of its own -------------------------------------------
	//
	// TIER 1 ONLY, AND THE STORE IS WHAT MAKES THAT TRUE: `nguon` and `ma_nguon_re_nhanh` are
	// LITERALS in the INSERT, not parameters, so there is no value any layer above could pass. The
	// handler additionally answers 400 to a body naming `source` or `tier` — not as the defence,
	// but so a client learns it may not decide provenance instead of watching the field vanish.
	//
	// idem.Required(MoKhiHong), AND WHICH LAYER IS ACTUALLY PROTECTING THIS — the question
	// skills/rest-api-design §4 says to answer at the route. The real guard is
	// `UNIQUE (tenant_id, ma)`, which counts soft-deleted rows: a second row with the same code
	// CANNOT EXIST, whatever happens to Redis. The idempotency key is the second, independent
	// layer — it is what stops a double-submitted form from producing one row and one confusing
	// 409 instead of one row and a replayed 201.
	//
	// MoKhiHong and not DongKhiHong for exactly that reason: with the unique key underneath, a
	// cache outage cannot produce a duplicate catalogue row, so refusing a commune administrator
	// mid-configuration would be paying with an outage for a risk that is already covered. The
	// legal-consequence cases the skill reserves DongKhiHong for — money, issued document numbers,
	// closing a commitment to a citizen — are not this.
	//
	// 409 AND NOT 403 for a full catalogue or a taken code: the caller holds `admin.lookup` and is
	// allowed to manage the list. What is refused is this value against the state of the data.
	//
	// @summary  Thêm một mức ưu tiên nhiệm vụ của riêng xã vào danh mục
	// @screen   14-cau-hinh §5
	// @request  themMucUuTienVao
	// @reply    201 mucUuTienRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/task-priorities",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.ThemMucUuTien))))

	// --- the commune edits one row --------------------------------------------------------------
	//
	// PATCH AND NOT PUT: three of the four editable fields have a meaningful zero, so a full
	// replacement cannot tell "not mentioned" from "set to zero" — see suaMucUuTienVao.
	//
	// WHAT EACH TIER ALLOWS IS DECIDED PER FIELD AND PER TRANSITION, not per route: relabel and
	// reorder at every tier, disable at tiers 1 and 2, and `ma` nowhere. The refusal is a 409
	// naming the tier, and the trigger refuses the same thing underneath (ADR 0024).
	//
	// idem.KhongCan, AND THE REASON IS A PROPERTY OF THE USE CASE RATHER THAN A HOPE: app.Sua
	// compares the row it read against the row it would write and, when nothing moved, writes
	// NOTHING — no UPDATE and no audit entry. So the same request sent twice leaves one row in one
	// state and one entry in the ledger. Were that comparison removed, this declaration would
	// become a lie and the second request would file a second entry saying nothing changed.
	//
	// @summary  Sửa nhãn, thứ tự, trạng thái dùng hoặc đặt mặc định cho một mức ưu tiên nhiệm vụ
	// @screen   14-cau-hinh §5
	// @request  suaMucUuTienVao
	// @reply    200 mucUuTienRa
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/task-priorities/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.Sua không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SuaMucUuTien))))

	// --- the commune retires one of its own rows ------------------------------------------------
	//
	// THIS IS A SOFT DELETE AND THE METHOD IS THE ONLY THING THAT SAYS OTHERWISE. The row stays,
	// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), and its `ma`
	// stays taken forever — an issued code is never reissued, because task records hold it as
	// a value and nothing rewrites them. DELETE is still the right method: the resource is gone
	// from every read path, which is what the caller is asking for.
	//
	// TIER 1 ONLY. A `he-thong` row answers 409 and is told to use `Tắt` instead — the same
	// refusal the trigger makes, arriving first and in a sentence somebody can act on.
	//
	// A BODY ON A DELETE, and the alternative was worse: the reason is mandatory, and the query
	// string would put free text about a government record into every access log and proxy cache.
	//
	// idem.KhongCan — deleting an already-deleted row is a 404 either way, and the second request
	// cannot overwrite who deleted it or why: the UPDATE carries `AND deleted_at IS NULL`.
	//
	// @summary  Xoá mềm một mục danh mục do xã tự thêm, kèm lý do bắt buộc
	// @screen   14-cau-hinh §5
	// @request  xoaMucUuTienVao
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/task-priorities/{id}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("xoá một dòng đã xoá cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.XoaMucUuTien))))

	// --- NHẬT KÝ HỆ THỐNG — this service's own audit log (ADR 0054) ------------------------------
	//
	// ONE OF FIVE ROUTES, one per service owning an `audit_log`; web-admin merges them (ADR 0054 §1,
	// §6). The noun carries the service name because tools/ingress routes by the first path segment
	// and five owners for one `audit-entries` segment is a STOP there (ADR 0054 §3).
	//
	// `admin.audit` — "Xem nhật ký hệ thống" — seeded at service-identity/migrations/0001_init.sql:280;
	// no key invented (rule 5, invariant 3c). Staff of the request's commune only; there is no operator
	// variant (ADR 0054 §2, ADR 0003:32).
	//
	// THE RESTRICTED FIELD (ADR 0054 §4, ADR 0030): entries about a `can-bo` petition are withheld
	// unless the reader ALSO holds `feedback.restricted`, asked of the Checker in the handler — never a
	// query parameter. The SQL is store.RestrictedPetitionAuditSubjects, built from the list's constant.
	//
	// A GET THAT WRITES: every call leaves one `xem_nhat_ky_he_thong` entry, in the same transaction
	// as the read (ADR 0054 §5) — so a page whose entry could not be written is a 500, never data.
	// NO idem.* DECLARATION: repeating the read is a second read, and it is recorded as one.
	//
	// @summary  Nhật ký hệ thống của phân hệ Tiếp dân – Nhiệm vụ — vết thao tác của xã, mới nhất trước, lọc theo thời gian · người · động từ · đối tượng
	// @screen   14-cau-hinh §12.1
	// @reply    200 page.Result[audit.EntryView]
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/petitions-audit-entries",
		authz.RequirePermission(d.Checker, "admin.audit")(
			http.HandlerFunc(h.ListAuditEntries)))

	// --- LỜI HỆ THỐNG — the sentences this service raises, reworded per commune (migration 0020) ----
	//
	// THE SAME THREE ROUTES AS service-finance's `finance-system-messages`, under this service's own
	// first segment: ADR 0024 splits the keys by the service that RAISES them, and tools/ingress refuses
	// two services on one first segment (ADR 0054 §3). The six `feedback.*` keys are listed here.
	//
	// `{code}` AND NOT `{key}`, and the response field is `code`: tools/apidoc's credential guard
	// refuses the word `key` in a response field, and the path parameter names the same thing.
	//
	// `override` IS A SUB-RESOURCE: the commune's own wording is a state PUT sets and DELETE removes,
	// and the message itself survives both — a shipped key cannot be deleted (14-cau-hinh §7).
	//
	// `admin.lookup` — "Quản lý danh mục" — ON ALL THREE, the key the requirement repository guards
	// this screen with (../vigov-require/docs/spec/04-api.md:41-44), seeded at
	// service-identity/migrations/0001_init.sql:274, and the key finance's half uses. NO KEY INVENTED
	// (rule 5, invariant 3c). READ IS NOT AnyAuthenticated: this list is the configuration screen; an
	// officer being refused gets the sentence inside the refusal, not from here.

	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Lời hệ thống của phân hệ Tiếp dân – Nhiệm vụ: câu mặc định, câu xã đang dùng và ai sửa lần cuối
	// @screen   14-cau-hinh §7
	// @reply    200 systemMessageListOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/petitions-system-messages",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			http.HandlerFunc(h.ListSystemMessages)))

	// PUT, a full replacement of the one field this resource has. Empty `text` is 400 and names the
	// DELETE below — an empty sentence is not a state this screen offers.
	//
	// idem.KhongCan: app.Reword writes nothing and files no entry when the text in force already
	// equals the text sent — so a retry leaves one row, one entry.
	//
	// @summary  Xã sửa lời một câu hệ thống của phân hệ Tiếp dân – Nhiệm vụ
	// @screen   14-cau-hinh §7
	// @request  rewordSystemMessageIn
	// @reply    200 systemMessageOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/petitions-system-messages/{code}/override",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("đặt lại đúng câu đang dùng không ghi gì và không để vết, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.RewordSystemMessage))))

	// "Khôi phục câu mặc định". A soft delete of the commune's live wording (rule 7, invariant 1);
	// the history stays on disk and in audit_log. 204 also when the commune is already on the
	// default — the state asked for holds, and nothing is written.
	//
	// idem.KhongCan: the second request finds no live wording and writes nothing.
	//
	// @summary  Khôi phục câu mặc định của phần mềm cho một câu hệ thống của phân hệ Tiếp dân – Nhiệm vụ
	// @screen   14-cau-hinh §7
	// @reply    204 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/petitions-system-messages/{code}/override",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("khôi phục khi xã đã dùng câu mặc định thì không còn dòng nào để gỡ và không ghi gì")(
				http.HandlerFunc(h.RestoreSystemMessage))))

	// --- ADR 0079 Q2 ("Làm đúng prototype", migration 0033) — the switch and the commune's sentences ---
	//
	// SAME KEY, `admin.lookup`, on all four: the same screen and the same act of administering the
	// commune's configuration as the three routes above (rule 5, invariant 3c — no key invented).

	// "Tắt / Bật" of a SHIPPED key, reworded or not (user decision 09/10/2026, which retired the 409
	// `no_commune_wording`): a PATCH of the override sub-resource's one other field. A switched-off
	// sentence is hidden where it is used, or the shipped default where the consumer must say something.
	// Every key here is a refusal, which must say something, so while off the refusal branches keep
	// sending the shipped sentence (domain.ResolveMessage CurrentText).
	//
	// idem.KhongCan: app.SetActive writes nothing and files no entry when the state already holds.
	//
	// @summary  Xã tắt hoặc bật một câu hệ thống (tắt thì ẩn nơi dùng, nơi bắt buộc có lời thì dùng lời gốc)
	// @screen   14-cau-hinh §7
	// @request  switchSystemMessageIn
	// @reply    200 systemMessageOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/petitions-system-messages/{code}/override",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("đặt lại đúng trạng thái đang có không ghi gì và không để vết, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.SwitchSystemMessage))))

	// A sentence the COMMUNE adds, group `phan-anh` or `chung` (ADR 0079 Q5a). Stored and managed
	// only — nothing resolves it anywhere (Q5b).
	//
	// idem.Required(MoKhiHong), as the catalogue creates: the real guard is UNIQUE (tenant_id,
	// message_key), which counts soft-deleted rows; the key turns a double-submit into a replayed 201
	// instead of a confusing 409. Not DongKhiHong: no legal consequence rides on a duplicate.
	//
	// @summary  Xã thêm một câu hệ thống của riêng xã (nhóm Phản ánh hoặc Dùng chung)
	// @screen   14-cau-hinh §7
	// @request  createCustomMessageIn
	// @reply    201 systemMessageOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/petitions-system-messages",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.CreateCustomMessage))))

	// Edit a commune sentence: words, description, switch. A shipped key answers 409 — it changes
	// through …/override.
	//
	// idem.KhongCan: app.EditCustom writes nothing and files no entry when no field moved.
	//
	// @summary  Sửa lời, mô tả hoặc tắt/bật một câu do xã tự thêm
	// @screen   14-cau-hinh §7
	// @request  editCustomMessageIn
	// @reply    200 systemMessageOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/petitions-system-messages/{code}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; app.EditCustom không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.EditCustomMessage))))

	// Soft delete of a commune sentence, reason required (rule 7, invariant 1). Its key stays taken
	// for ever; the deleted row is frozen by the trigger (0033). A shipped key answers 409.
	//
	// idem.KhongCan: the second request finds no live sentence (404) and cannot overwrite who deleted
	// it or why — the UPDATE carries `AND deleted_at IS NULL`.
	//
	// @summary  Xoá mềm một câu do xã tự thêm, kèm lý do bắt buộc
	// @screen   14-cau-hinh §7
	// @request  deleteCustomMessageIn
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/petitions-system-messages/{code}",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("xoá một câu đã xoá cho cùng một kết quả: câu UPDATE mang `AND deleted_at IS NULL` nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.DeleteCustomMessage))))
}
