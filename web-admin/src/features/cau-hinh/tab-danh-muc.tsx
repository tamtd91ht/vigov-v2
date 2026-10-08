"use client";

import { Pencil, Plus, Trash2, Upload } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type KeyboardEvent, type ReactNode } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { controlClass } from "@/components/ui/field";
import { PendingButton, PendingMarker } from "@/components/ui/pending-feature";
import { BUSY_DELETING, BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { cn } from "@/lib/cn";
import { suaMuc, themMuc, xoaMuc, type KhoaDanhMucGhi } from "@/lib/api/danh-muc"; // vi-name-ok: existing exports of danh-muc.ts, imported not declared (rule 12 inv 3)
import { docDanhMucNghiepVu, type BayDanhMuc } from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import type { petitions_danhSachTrangThaiNhiemVuRa } from "@/lib/api/schema.gen";
import { suaTrangThaiNhiemVu, layTrangThaiNhiemVu } from "@/lib/api/trang-thai-nhiem-vu";
import { QUYEN_QUAN_LY_DANH_MUC, quyetDinhTheoKhoa } from "@/lib/quyen";

import { ConfigImportButton } from "./config-import-button";
import {
  ConfigField,
  ConfigFormRow,
  ConfigLoading,
  ConfigTable,
  RowActions,
  SMALL_BUTTON_CLASS,
  StatusBadge,
  formInputCls,
  formSelectCls,
} from "./config-ui";
import {
  CAPITAL_PLAN_CATEGORY_IMPORT_TARGET,
  DOCUMENT_TYPE_IMPORT_TARGET,
  MAP_ASSET_TYPE_IMPORT_TARGET,
  RESIDENTIAL_UNIT_TYPE_IMPORT_TARGET,
  TASK_BLOC_IMPORT_TARGET,
  TASK_PRIORITY_IMPORT_TARGET,
  TASK_TYPE_IMPORT_TARGET,
  catalogueImportFor,
} from "./excel-import-targets";
import { PHAN_CHUA_DUNG } from "./nhan-cau-hinh";
import {
  CANH_BAO_XOA,
  NUT_HUY,
  NUT_LUU,
  NUT_TAT,
  NUT_THEM,
  NUT_XOA,
  O_LY_DO_XOA,
  O_NHAN,
  O_THU_TU,
  nhanMacDinh,
  nhanNutCuaDong,
} from "./nhan-danh-muc";
import {
  CATALOGUE_GROUPS,
  TASK_STATUS_GROUP,
  catalogueRows,
  codeFromLabel,
  effectiveShownGroup,
  groupsWithRows,
  nhomDanhMuc,
  type CatalogueRow,
  type NhomDanhMuc,
  type ShownGroup,
} from "./nhom-danh-muc";
import { choBatLai, kiemLyDoXoa, laMucGhi, thaoTacCuaMuc } from "./tang-danh-muc";
import { THU_TU_KHONG_PHAI_SO, TIEU_DE_NHOM_TRANG_THAI, thanSuaTrangThai } from "./trang-thai-nhiem-vu";

/**
 * Tab "Danh mục" — spec `05-danh-muc.md` (ADR 0079), prototype `LookupTable.tsx`: a group filter, a
 * grey add row, ONE table across every group.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 * THREE THINGS DECIDE WHETHER THIS SCREEN IS RIGHT, AND NONE OF THEM IS VISIBLE:
 *
 * 1. BUTTONS FOLLOW THE TIER, AND THE TIER COMES FROM THE SERVER (ADR 0024). Tier 3 has no "Tắt" and no
 *    bin; tier 2 has no bin. The rule lives in `tang-danh-muc.ts` (pure, tested), never as a branch
 *    between two `<td>`s. The real block is the database trigger; drawing the right buttons only keeps
 *    staff from pressing a 409. The per-row sentence explaining a missing button is gone (spec 05):
 *    the rule is still enforced by which buttons are drawn.
 *
 * 2. `source` AND `tier` NEVER GO UP. The server answers 400 if a body names them; the bodies are built
 *    field by field in `lib/api/danh-muc.ts`.
 *
 * 3. HIDING A BUTTON IS CONVENIENCE, NOT A CONTROL. Every write route declares
 *    `RequirePermission("admin.lookup")` and checks it on EVERY request (rule 5, forbidden #1).
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 *
 * THE TABLE SHOWS FOR EVERY ACCOUNT, ONLY THE WRITES ARE GATED: the read routes are
 * `any-authenticated` because catalogue labels appear in pickers and filters on almost every screen.
 *
 * DELETE KEEPS ITS REASON STEP (rule 7, owner decision): the server keeps the row and requires
 * `reason`. The step is compact — the row's actions turn into a "Lý do xoá" box, "Xoá" and "Huỷ".
 *
 * `Trạng thái nhiệm vụ` (decision #21, ADR 0035 §C) sits in the same table but only ever offers the
 * pencil: a commune renames and reorders the seven fixed codes, never adds, disables or deletes one.
 */
export function TabDanhMuc() {
  /** `null` = not read yet. The seven catalogues arrive in ONE round, so the tab has one loading phase. */
  const [catalogues, setCatalogues] = useState<BayDanhMuc | null>(null);
  const [taskStatuses, setTaskStatuses] = useState<KetQua<petitions_danhSachTrangThaiNhiemVuRa> | null>(null);
  /**
   * Bumped after every successful write to READ AGAIN, never to patch arrays in place: making one entry
   * the default clears the default of another in the same catalogue, and a local patch would show two.
   */
  const [reads, setReads] = useState(0);

  /** The filter button pressed. What is APPLIED is `shownGroup` below — see `effectiveShownGroup`. */
  const [chosenGroup, setChosenGroup] = useState<ShownGroup>(null);
  const [open, setOpen] = useState<OpenForm>(null);
  const [draft, setDraft] = useState<CatalogueDraft>(EMPTY_DRAFT);
  /** A check this screen made before sending — distinct from the server's sentence (`serverError`). */
  const [localError, setLocalError] = useState("");
  const [serverError, setServerError] = useState("");
  const [busy, setBusy] = useState(false);

  const session = usePhien();
  // THREE STATES, NOT TWO: no write button until the session is read — "not known yet" acts neither as
  // "allowed" nor as "refused".
  const writeDecision = session === null ? null : quyetDinhTheoKhoa(session, QUYEN_QUAN_LY_DANH_MUC);
  const canWrite = writeDecision !== null && writeDecision.hien;
  const sessionError =
    writeDecision !== null && !writeDecision.hien && writeDecision.vi === "khong-doc-duoc" ? writeDecision.thongBao : "";

  useEffect(() => {
    let dropped = false;
    docDanhMucNghiepVu().then((d) => {
      if (!dropped) setCatalogues(d);
    });
    layTrangThaiNhiemVu().then((r) => {
      if (!dropped) setTaskStatuses(r);
    });
    return () => {
      dropped = true;
    };
  }, [reads]);

  const groups = useMemo(() => (catalogues === null ? null : nhomDanhMuc(catalogues)), [catalogues]);
  const allRows = useMemo(
    () =>
      groups === null
        ? []
        : catalogueRows(groups, taskStatuses !== null && taskStatuses.ok ? taskStatuses.duLieu.items : [], null),
    [groups, taskStatuses],
  );
  const writable = useMemo(() => writableGroups(groups), [groups]);
  const shownGroup = effectiveShownGroup(allRows, chosenGroup);

  const close = useCallback(() => {
    setOpen(null);
    setDraft(EMPTY_DRAFT);
    setLocalError("");
    setServerError("");
  }, []);

  /**
   * One write: clear old messages, send, then either toast what was done and READ AGAIN, or show the
   * server's sentence AS WRITTEN — no branching on `code`, no `trace_id`.
   */
  const run = useCallback(function <T>(call: Promise<KetQua<T>>, sentence: string) {
    setLocalError("");
    setServerError("");
    setBusy(true);
    void call.then((r) => {
      setBusy(false);
      if (!r.ok) {
        setServerError(r.thongBao);
        return;
      }
      setOpen(null);
      setDraft(EMPTY_DRAFT);
      toast.success(sentence);
      setReads((n) => n + 1);
    });
  }, []);

  const actions: CatalogueActions = {
    toggleAdd: () => {
      if (open !== null && open.kind === "add") {
        close();
        return;
      }
      const initial = writable.find((g) => g.khoa === shownGroup) ?? writable[0];
      setOpen({ kind: "add", idempotencyKey: newIdempotencyKey() });
      setDraft({ ...EMPTY_DRAFT, group: initial === undefined ? "" : initial.khoa });
      setLocalError("");
      setServerError("");
    },
    edit: (row) => {
      setOpen({ kind: "edit", rowKey: row.key });
      setDraft({
        ...EMPTY_DRAFT,
        label: row.item.label,
        order: row.kind === "taskStatus" ? String(row.item.order) : laMucGhi(row.item) ? String(row.item.order) : "",
        isDefault: row.kind === "lookup" && row.item.is_default,
      });
      setLocalError("");
      setServerError("");
    },
    remove: (row) => {
      setOpen({ kind: "delete", rowKey: row.key });
      setDraft(EMPTY_DRAFT);
      setLocalError("");
      setServerError("");
    },
    setActive: (row, active) => {
      if (busy || row.kind !== "lookup" || row.group.ghi === null) return;
      close();
      run(suaMuc(row.group.ghi, row.item.id, { active }), SAVED);
    },
    makeDefault: (row) => {
      if (busy || row.kind !== "lookup" || row.group.ghi === null) return;
      close();
      run(suaMuc(row.group.ghi, row.item.id, { is_default: true }), defaultFromNow(row.item.label));
    },
    cancel: close,
    submit: () => {
      if (open === null || busy) return;

      if (open.kind === "add") {
        const label = draft.label.trim();
        if (label === "") {
          setLocalError(LABEL_REQUIRED);
          return;
        }
        const code = codeFromLabel(label);
        if (code === "") {
          setLocalError(LABEL_WITHOUT_LETTERS);
          return;
        }
        const group = writable.find((g) => g.khoa === draft.group);
        if (group === undefined || group.ghi === null) return;
        run(themMuc(group.ghi, { code, label, is_default: false }, open.idempotencyKey), ADDED);
        return;
      }

      const row = allRows.find((r) => r.key === open.rowKey);
      if (row === undefined) return;

      if (open.kind === "delete") {
        if (row.kind !== "lookup" || row.group.ghi === null) return;
        // THE ONE CHECK THIS SCREEN MAKES ITSELF (`tang-danh-muc.ts`): an empty reason is never sent.
        const reason = kiemLyDoXoa(draft.reason);
        if (!reason.ok) {
          setLocalError(reason.loi);
          return;
        }
        run(xoaMuc(row.group.ghi, row.item.id, reason.giaTri), DELETED);
        return;
      }

      const label = draft.label.trim();
      if (label === "") {
        setLocalError(LABEL_REQUIRED);
        return;
      }
      if (row.kind === "taskStatus") {
        const body = thanSuaTrangThai(row.item, { nhan: label, thuTu: draft.order });
        if (!body.ok) {
          setLocalError(body.loi);
          return;
        }
        run(suaTrangThaiNhiemVu(row.item.code, body.than), SAVED);
        return;
      }
      if (row.group.ghi === null) return;
      const order = orderValue(draft.order);
      if (!order.ok) {
        setLocalError(THU_TU_KHONG_PHAI_SO);
        return;
      }
      run(suaMuc(row.group.ghi, row.item.id, { label, order: order.value, is_default: draft.isDefault }), SAVED);
    },
  };

  /**
   * A new add attempt is a new request: changing the group or the label after a refusal must not reuse
   * the key of a body the server may already hold. Retrying the SAME body (after a network failure)
   * keeps the key — the first attempt may have reached the server.
   */
  const updateDraft = (next: CatalogueDraft) => {
    if (open !== null && open.kind === "add" && (next.label !== draft.label || next.group !== draft.group)) {
      setOpen({ kind: "add", idempotencyKey: newIdempotencyKey() });
    }
    setDraft(next);
  };

  const importGroup = importGroupFor(groups, shownGroup, session);

  return (
    <CatalogueView
      groups={groups}
      taskStatuses={taskStatuses}
      shownGroup={shownGroup}
      onShowGroup={setChosenGroup}
      canWrite={canWrite}
      sessionError={sessionError}
      importButton={catalogueImportButton(canWrite, importGroup, () => setReads((n) => n + 1))}
      open={open}
      draft={draft}
      setDraft={updateDraft}
      localError={localError}
      serverError={serverError}
      busy={busy}
      actions={actions}
    />
  );
}

/* ---- words of this tab (spec 05 / prototype `LookupTable.tsx`, verbatim) ---------------------------- */

export const ALL_GROUPS = "Tất cả";
const GROUP_FIELD = "Nhóm danh mục";
const COLOR_FIELD = "Màu";
const LABEL_PLACEHOLDER = "Ví dụ: Chợ và thương mại";
const ADD_SUBMIT = "Thêm";
const ENABLE = "Bật";
export const EDIT_LABEL = "Sửa nhãn";
export const DELETE_ENTRY = "Xoá mục";
export const SET_DEFAULT = "Đặt mặc định";
export const SET_DEFAULT_TITLE = "Đặt làm lựa chọn mặc định khi giao việc";
export const SOURCE_SYSTEM = "Hệ thống";
export const SOURCE_COMMUNE = "Xã tự thêm";
export const ADDED = "Đã thêm mục mới vào danh mục.";
export const SAVED = "Đã lưu.";
export const DELETED = "Đã xoá mục khỏi danh mục.";
export const LABEL_REQUIRED = "Nhãn hiển thị không được để trống.";
/** Not in the spec: a label of only symbols yields no code (`codeFromLabel`), refused before sending. */
export const LABEL_WITHOUT_LETTERS = "Nhãn hiển thị cần có ít nhất một chữ cái hoặc chữ số.";

export function defaultFromNow(label: string): string {
  return `"${label}" là lựa chọn mặc định từ giờ.`;
}

/** The one group with a "default when assigning" choice in the prototype (`LookupTable.tsx:32`). */
const GROUP_WITH_DEFAULT_BUTTON = "loaiNhiemVu";

/** The "?" of the colour field — its sentence is `PHAN_CHUA_DUNG`'s, never a second copy. */
const COLOR_PENDING = PHAN_CHUA_DUNG.find((p) => p.ten === "Màu của mục danh mục")!;

/**
 * One import button per group, bound to that group's target. A `Record` over every key, so a new
 * catalogue group fails `tsc` here instead of silently drawing no button.
 */
const IMPORT_BUTTONS: Record<KhoaDanhMucGhi, (onImported: () => void) => ReactNode> = {
  loaiTaiNguyenBanDo: (f) => <ConfigImportButton target={MAP_ASSET_TYPE_IMPORT_TARGET} onImported={f} />,
  hangMucKeHoachVon: (f) => <ConfigImportButton target={CAPITAL_PLAN_CATEGORY_IMPORT_TARGET} onImported={f} />,
  loaiVanBan: (f) => <ConfigImportButton target={DOCUMENT_TYPE_IMPORT_TARGET} onImported={f} />,
  loaiDonViDanCu: (f) => <ConfigImportButton target={RESIDENTIAL_UNIT_TYPE_IMPORT_TARGET} onImported={f} />,
  khoiNhiemVu: (f) => <ConfigImportButton target={TASK_BLOC_IMPORT_TARGET} onImported={f} />,
  loaiNhiemVu: (f) => <ConfigImportButton target={TASK_TYPE_IMPORT_TARGET} onImported={f} />,
  mucUuTienNhiemVu: (f) => <ConfigImportButton target={TASK_PRIORITY_IMPORT_TARGET} onImported={f} />,
};

/** The "?" of the tab-wide import — its sentence is `PHAN_CHUA_DUNG`'s. */
const COMMON_IMPORT_PENDING = PHAN_CHUA_DUNG.find((p) => p.ten === "Nhập Excel chung cho mọi nhóm danh mục")!;

/**
 * The ONE "Nhập từ Excel" row at the head of the tab (prototype `ConfigWorkspace.tsx:109-111`), always
 * drawn for an account holding `admin.lookup`:
 *   · a filtered group with an import route → the working button for THAT group;
 *   · "Tất cả", the eighth group, or a group without a route → the same button DISABLED with its "?"
 *     (ADR 0068 §14): the server imports per group only, there is no route taking every group at once.
 * Without the key: nothing at all — a "?" would announce a write the account may not do anyway.
 */
export function catalogueImportButton(
  canWrite: boolean,
  importGroup: KhoaDanhMucGhi | null,
  onImported: () => void,
): ReactNode {
  if (!canWrite) return null;
  if (importGroup !== null) return IMPORT_BUTTONS[importGroup](onImported);
  return (
    // FIXED 28px ROW, the height of the working sm `ConfigImportButton`: the legacy `nut-phu` min-height
    // (32px) reached the disabled button inside `PendingButton` (which takes no button class), so the
    // whole tab jumped ~5px when the filter switched between "Tất cả" and a group. The row is pinned to
    // `h-7`, the disabled button is forced back to `h-7` without the min-height, and the 18px "?" (also a
    // `<button>`, excluded by `data-pending-marker`) stays inside that box; nothing clips it.
    <div className="mb-3 flex h-7 items-center justify-end overflow-visible">
      <span className="inline-flex h-7 items-center [&_button:not([data-pending-marker])]:h-7 [&_button:not([data-pending-marker])]:min-h-0">
        <PendingButton
          info={COMMON_IMPORT_PENDING}
          variant="outline"
          size="sm"
          icon={<Upload aria-hidden="true" focusable="false" className="size-4" />}
        >
          Nhập từ Excel
        </PendingButton>
      </span>
    </div>
  );
}

/* ---- state ------------------------------------------------------------------------------------- */

/**
 * Which form is open — ONE for the whole tab: the add row, or one row in edit or delete. Two drafts at
 * once on a 320px screen are two drafts staff cannot both see.
 */
export type OpenForm =
  | { readonly kind: "add"; readonly idempotencyKey: string }
  | { readonly kind: "edit"; readonly rowKey: string }
  | { readonly kind: "delete"; readonly rowKey: string }
  | null;

/** The draft being typed. Strings, as a browser input returns them. */
export type CatalogueDraft = {
  readonly group: string;
  readonly label: string;
  readonly order: string;
  readonly isDefault: boolean;
  readonly reason: string;
};

export const EMPTY_DRAFT: CatalogueDraft = { group: "", label: "", order: "", isDefault: false, reason: "" };

export type CatalogueActions = {
  readonly toggleAdd: () => void;
  readonly edit: (row: CatalogueRow) => void;
  readonly remove: (row: CatalogueRow) => void;
  readonly setActive: (row: CatalogueRow, active: boolean) => void;
  readonly makeDefault: (row: CatalogueRow) => void;
  readonly submit: () => void;
  readonly cancel: () => void;
};

function writableGroups(groups: readonly NhomDanhMuc[] | null): readonly NhomDanhMuc[] {
  return groups === null ? [] : groups.filter((g) => g.ghi !== null);
}

/**
 * The group the top "Nhập từ Excel" button imports into, or `null` for no button: it acts on the
 * FILTERED group only — never "Tất cả", never the eighth group — and only when that group has an import
 * route AND this account holds the route's key (`catalogueImportFor`, fail closed). Convenience, not a
 * control: the server checks the key on all three import routes.
 */
export function importGroupFor(
  groups: readonly NhomDanhMuc[] | null,
  shown: ShownGroup,
  session: Parameters<typeof catalogueImportFor>[1],
): KhoaDanhMucGhi | null {
  const group = writableGroups(groups).find((g) => g.khoa === shown);
  if (group === undefined || group.ghi === null) return null;
  return catalogueImportFor(group.ghi.khoa, session) === null ? null : group.ghi.khoa;
}

/**
 * The order box: empty = UNCHANGED (never 0, which would move the entry to the top — and in the
 * priority group the order IS the scale). Not `parseInt`: `parseInt("3 chữ")` is 3, swallowing a typo.
 */
function orderValue(box: string): { ok: true; value: number | undefined } | { ok: false } {
  const clean = box.trim();
  if (clean === "") return { ok: true, value: undefined };
  const n = Number(clean);
  return Number.isInteger(n) ? { ok: true, value: n } : { ok: false };
}

/** `crypto.randomUUID` exists in every secure context — the same context the `Secure` session cookie needs. */
function newIdempotencyKey(): string {
  return crypto.randomUUID();
}

/* ---- presentation ------------------------------------------------------------------------------ */

/**
 * Everything visible on the tab, PURE PRESENTATION — exported so a test renders it with
 * `react-dom/server`: a decision tested only in a pure module can still fail to reach the page.
 */
export function CatalogueView({
  groups,
  taskStatuses,
  shownGroup: chosenGroup,
  onShowGroup,
  canWrite,
  sessionError,
  importButton,
  open,
  draft,
  setDraft,
  localError,
  serverError,
  busy,
  actions,
}: {
  groups: readonly NhomDanhMuc[] | null;
  taskStatuses: KetQua<petitions_danhSachTrangThaiNhiemVuRa> | null;
  shownGroup: ShownGroup;
  onShowGroup: (g: ShownGroup) => void;
  canWrite: boolean;
  /** The session could not be read: its sentence, and no write button. */
  sessionError: string;
  importButton: ReactNode;
  open: OpenForm;
  draft: CatalogueDraft;
  setDraft: (d: CatalogueDraft) => void;
  localError: string;
  serverError: string;
  busy: boolean;
  actions: CatalogueActions;
}) {
  const loading = groups === null || taskStatuses === null;
  const taskItems = taskStatuses !== null && taskStatuses.ok ? taskStatuses.duLieu.items : [];
  const allRows = groups === null ? [] : catalogueRows(groups, taskItems, null);
  // Applied here too (the tab already passes the effective value): the view must never narrow to a group
  // that has no filter button.
  const shownGroup = effectiveShownGroup(allRows, chosenGroup);
  const rows = groups === null ? [] : catalogueRows(groups, taskItems, shownGroup);
  const total = allRows.length;
  const present = groupsWithRows(allRows);
  const writable = writableGroups(groups);

  // A group that could not be read says so ABOVE the table, in the server's words: its rows are absent
  // for a reason, never "the commune has no entries".
  const readErrors: string[] = [];
  for (const g of groups ?? []) {
    if ((shownGroup === null || shownGroup === g.khoa) && g.trangThai.pha === "khongDocDuoc") {
      readErrors.push(`${g.nhan}: ${g.trangThai.thongBao}`);
    }
  }
  if (taskStatuses !== null && !taskStatuses.ok && (shownGroup === null || shownGroup === TASK_STATUS_GROUP)) {
    readErrors.push(`${TIEU_DE_NHOM_TRANG_THAI}: ${taskStatuses.thongBao}`);
  }

  return (
    <section className="space-y-4" aria-labelledby="tieu-de-danh-muc">
      {/* No card, no visible title: the tab IS the section (prototype). */}
      <h2 id="tieu-de-danh-muc" className="an-thi-giac">
        Danh mục
      </h2>

      {importButton}

      {/* While loading the placeholder replaces the whole workspace, filter row included (prototype
          `ConfigWorkspace.tsx:112` wraps `LookupTable` in `Panel loading`). */}
      {!loading && (
      <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Lọc theo nhóm danh mục">
        <FilterButton pressed={shownGroup === null} onClick={() => onShowGroup(null)}>
          {`${ALL_GROUPS} (${total})`}
        </FilterButton>
        {/* One button per group THAT HAS ROWS, as the prototype derives them (`LookupTable.tsx:68-71`). */}
        {CATALOGUE_GROUPS.filter((g) => present.has(g.khoa)).map((g) => (
          <FilterButton key={g.khoa} pressed={shownGroup === g.khoa} onClick={() => onShowGroup(g.khoa)}>
            {g.nhan}
          </FilterButton>
        ))}
        {present.has(TASK_STATUS_GROUP) && (
          <FilterButton pressed={shownGroup === TASK_STATUS_GROUP} onClick={() => onShowGroup(TASK_STATUS_GROUP)}>
            {TIEU_DE_NHOM_TRANG_THAI}
          </FilterButton>
        )}
        {canWrite && writable.length > 0 && (
          <Button
            type="button"
            variant="primary"
            size="sm"
            className={cn(SMALL_BUTTON_CLASS, "ml-auto")}
            icon={<Plus aria-hidden="true" focusable="false" className="size-4" />}
            aria-expanded={open !== null && open.kind === "add"}
            onClick={actions.toggleAdd}
          >
            {NUT_THEM}
          </Button>
        )}
      </div>
      )}

      {sessionError !== "" && <InlineError>{sessionError}</InlineError>}

      {!loading && canWrite && open !== null && open.kind === "add" && (
        <AddRow
          groups={writable}
          draft={draft}
          setDraft={setDraft}
          localError={localError}
          serverError={serverError}
          busy={busy}
          onSubmit={actions.submit}
          onCancel={actions.cancel}
        />
      )}

      {loading ? (
        <ConfigLoading label="Đang tải danh mục của đơn vị…" />
      ) : (
        <>
          {readErrors.map((e) => (
            <InlineError key={e}>{e}</InlineError>
          ))}
          {/* The error of an action that opens no form ("Tắt", "Bật", "Đặt mặc định"). */}
          {open === null && serverError !== "" && <InlineError>{serverError}</InlineError>}

          {(rows.length > 0 || readErrors.length === 0) && (
            <ConfigTable label="Danh mục của đơn vị" caption="Các mục danh mục của đơn vị, theo nhóm và theo thứ tự đơn vị đã sắp">
              <thead>
                <tr>
                  <th scope="col">Nhóm danh mục</th>
                  <th scope="col">Mã</th>
                  <th scope="col">{O_NHAN}</th>
                  <th scope="col">{O_THU_TU}</th>
                  <th scope="col">Nguồn</th>
                  <th scope="col">Trạng thái</th>
                  {canWrite && (
                    <th scope="col">
                      <span className="an-thi-giac">Thao tác</span>
                    </th>
                  )}
                </tr>
              </thead>
              {/* NO EMPTY ROW, NO EMPTY SENTENCE: the prototype draws none (`LookupTable.tsx:136-140`, owner
                  08/10/2026 "Bỏ hết, đúng prototype") — an empty group is the header alone. */}
              <tbody>
                {rows.map((row) => (
                  <CatalogueTableRow
                    key={row.key}
                    row={row}
                    canWrite={canWrite}
                    mode={open !== null && open.kind !== "add" && open.rowKey === row.key ? open.kind : "view"}
                    draft={draft}
                    setDraft={setDraft}
                    localError={localError}
                    serverError={serverError}
                    busy={busy}
                    actions={actions}
                  />
                ))}
              </tbody>
            </ConfigTable>
          )}
        </>
      )}
    </section>
  );
}

function FilterButton({ pressed, onClick, children }: { pressed: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <Button
      type="button"
      size="sm"
      variant={pressed ? "primary" : "outline"}
      className={SMALL_BUTTON_CLASS}
      aria-pressed={pressed}
      onClick={onClick}
    >
      {children}
    </Button>
  );
}

/** A sentence the server wrote (or the one check of this screen), in place, in the spec's error type. */
function InlineError({ children }: { children: ReactNode }) {
  return (
    <p role="alert" className="text-danger text-[12px] font-medium whitespace-normal">
      {children}
    </p>
  );
}

/** The two error regions of a form, never merged: a local check and a server refusal read differently. */
function FormErrors({ localError, serverError }: { localError: string; serverError: string }) {
  return (
    <>
      {localError !== "" && <InlineError>{localError}</InlineError>}
      {serverError !== "" && <InlineError>{serverError}</InlineError>}
    </>
  );
}

/** The spec's grey add row: group · label · colour ("?") · "Thêm" / "Huỷ". Enter in the label submits. */
export function AddRow({
  groups,
  draft,
  setDraft,
  localError,
  serverError,
  busy,
  onSubmit,
  onCancel,
}: {
  /** The groups that have a write route — the only ones an entry can be added to. */
  groups: readonly NhomDanhMuc[];
  draft: CatalogueDraft;
  setDraft: (d: CatalogueDraft) => void;
  localError: string;
  serverError: string;
  busy: boolean;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  return (
    <ConfigFormRow
      columns="sm:grid-cols-[16rem_1fr_6rem_auto]"
      aria-label="Thêm mục danh mục"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      <ConfigField label={GROUP_FIELD} htmlFor="catalogue-add-group">
        <select
          id="catalogue-add-group"
          name="group"
          className={formSelectCls}
          value={draft.group}
          onChange={(e) => setDraft({ ...draft, group: e.target.value })}
        >
          {groups.map((g) => (
            <option key={g.khoa} value={g.khoa}>
              {g.nhan}
            </option>
          ))}
        </select>
      </ConfigField>
      <ConfigField label={O_NHAN} htmlFor="catalogue-add-label">
        <input
          id="catalogue-add-label"
          name="label"
          className={formInputCls}
          placeholder={LABEL_PLACEHOLDER}
          value={draft.label}
          onChange={(e) => setDraft({ ...draft, label: e.target.value })}
          aria-invalid={localError !== ""}
        />
      </ConfigField>
      <ColorField />
      <div className="flex gap-2">
        <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
          <BusyLabel busy={busy} label={ADD_SUBMIT} busyText={BUSY_SAVING} />
        </Button>
        <Button type="button" variant="outline" onClick={onCancel} disabled={busy}>
          {NUT_HUY}
        </Button>
      </div>
      {(localError !== "" || serverError !== "") && (
        <div className="col-span-full">
          <FormErrors localError={localError} serverError={serverError} />
        </div>
      )}
    </ConfigFormRow>
  );
}

/**
 * The spec's "Màu" field, drawn DISABLED with its "?" (ADR 0068 §14, ADR 0079 #5): no catalogue stores
 * a colour yet. The "?" sits beside the label, never inside it — inside, its sentence would become part
 * of the control's accessible name. The disabled input has no `name`: it can never submit anything.
 */
function ColorField() {
  return (
    <div className="block min-w-0" data-pending="">
      <div className="flex items-center gap-1.5">
        <label htmlFor="catalogue-add-color" className="text-foreground m-0 text-[11.5px] leading-none font-medium">
          {COLOR_FIELD}
        </label>
        <PendingMarker info={COLOR_PENDING} />
      </div>
      <input
        id="catalogue-add-color"
        type="color"
        disabled
        defaultValue="#2fb1f9"
        className="border-line mt-1 h-9 w-full cursor-not-allowed rounded-md border border-solid bg-white p-1 opacity-50"
      />
    </div>
  );
}

/** Enter saves, Escape cancels — the in-place edit and the reason box (prototype `LookupTable.tsx:224`). */
function submitOrCancel(onSubmit: () => void, onCancel: () => void) {
  return (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      onSubmit();
    } else if (e.key === "Escape") {
      e.preventDefault();
      onCancel();
    }
  };
}

const ROW_INPUT_CLASS = cn(controlClass, "h-8 text-[12.5px]");

function SourceText({ source }: { source: string | null }) {
  if (source === "he-thong") return <span className="text-ink-muted text-[12px]">{SOURCE_SYSTEM}</span>;
  if (source === "don-vi") return <span className="text-[12px]">{SOURCE_COMMUNE}</span>;
  // An unknown value means the contract drifted from the CHECK constraint: shown as is, never guessed.
  return <span className="text-[12px]">{source === null || source.trim() === "" ? "—" : source}</span>;
}

/**
 * One line of the table. `mode` is the open form ON THIS ROW: "edit" turns the label (and order, and
 * "Mặc định") cells into inputs with "Lưu" / "Huỷ"; "delete" turns the actions into the reason step.
 */
export function CatalogueTableRow({
  row,
  canWrite,
  mode,
  draft,
  setDraft,
  localError,
  serverError,
  busy,
  actions,
}: {
  row: CatalogueRow;
  canWrite: boolean;
  mode: "view" | "edit" | "delete";
  draft: CatalogueDraft;
  setDraft: (d: CatalogueDraft) => void;
  localError: string;
  serverError: string;
  busy: boolean;
  actions: CatalogueActions;
}) {
  const label = row.item.label;
  const writeShaped = row.kind === "lookup" && laMucGhi(row.item);
  // Order is the contract's field — never the array position: the edit changes exactly this number.
  const order = row.kind === "taskStatus" ? row.item.order : laMucGhi(row.item) ? row.item.order : "—";
  // Task statuses are the seven fixed codes the software ships (#21), so their source is the system.
  const source = row.kind === "taskStatus" ? "he-thong" : laMucGhi(row.item) ? row.item.source : null;
  const active = row.kind === "taskStatus" ? true : row.item.active;
  const isDefault = row.kind === "lookup" && row.item.is_default;
  const keys = submitOrCancel(actions.submit, actions.cancel);
  const editing = canWrite && mode === "edit";
  const deleting = canWrite && mode === "delete";

  return (
    <tr>
      <td className="text-ink-muted">{row.kind === "lookup" ? row.group.nhan : TIEU_DE_NHOM_TRANG_THAI}</td>
      <td>
        <code className="text-[11.5px]">{row.item.code}</code>
      </td>
      <td className="text-navy font-medium">
        {editing ? (
          <div className="flex flex-col gap-1">
            <div className="flex items-center gap-3">
              <input
                className={cn(ROW_INPUT_CLASS, "w-56")}
                aria-label={nhanNutCuaDong(O_NHAN, label)}
                value={draft.label}
                onChange={(e) => setDraft({ ...draft, label: e.target.value })}
                onKeyDown={keys}
                aria-invalid={localError !== ""}
              />
              {/* Decision 3: "Mặc định" stays editable. Every catalogue's PATCH carries `is_default`. */}
              {writeShaped && (
                <label className="text-ink inline-flex items-center gap-1.5 text-[12px] font-normal">
                  <input
                    type="checkbox"
                    checked={draft.isDefault}
                    onChange={(e) => setDraft({ ...draft, isDefault: e.target.checked })}
                  />
                  {nhanMacDinh(true)}
                </label>
              )}
            </div>
            <FormErrors localError={localError} serverError={serverError} />
          </div>
        ) : (
          <span className="inline-flex items-center gap-2">
            {label}
            {/* Not a status: the prototype's text-only pill (`LookupTable.tsx:243`), no tone icon. */}
            {isDefault && (
              <span className="bg-brand/12 text-brand border-brand/25 inline-flex h-5 w-fit shrink-0 items-center rounded-4xl border border-solid px-2 py-0.5 text-xs leading-none font-medium whitespace-nowrap">
                {nhanMacDinh(true)}
              </span>
            )}
          </span>
        )}
      </td>
      <td>
        {editing ? (
          // `inputMode="numeric"`, not `type="number"`: a number box changes value on a mouse wheel.
          <input
            className={cn(ROW_INPUT_CLASS, "w-16")}
            aria-label={nhanNutCuaDong(O_THU_TU, label)}
            inputMode="numeric"
            value={draft.order}
            onChange={(e) => setDraft({ ...draft, order: e.target.value })}
            onKeyDown={keys}
          />
        ) : (
          order
        )}
      </td>
      <td>
        <SourceText source={source} />
      </td>
      <td>
        <StatusBadge active={active} />
      </td>
      {canWrite && (
        <td>
          {editing ? (
            <RowActions>
              <Button type="button" variant="primary" size="sm" disabled={busy} aria-busy={busy} onClick={actions.submit}>
                <BusyLabel busy={busy} label={NUT_LUU} busyText={BUSY_SAVING} />
              </Button>
              <Button type="button" variant="outline" size="sm" disabled={busy} onClick={actions.cancel}>
                {NUT_HUY}
              </Button>
            </RowActions>
          ) : deleting ? (
            <div className="flex flex-col items-end gap-1">
              <RowActions>
                {/* Rule 7: the server keeps the row and requires why. The warning that the code stays
                    reserved rides on the box (hover + description) rather than as a notice. */}
                <input
                  className={cn(ROW_INPUT_CLASS, "w-48")}
                  aria-label={`${O_LY_DO_XOA} — mục ${label}`}
                  aria-describedby={`${row.key}-delete-note`}
                  title={CANH_BAO_XOA}
                  placeholder={O_LY_DO_XOA}
                  value={draft.reason}
                  onChange={(e) => setDraft({ ...draft, reason: e.target.value })}
                  onKeyDown={keys}
                  aria-invalid={localError !== ""}
                />
                <Button type="button" variant="danger" size="sm" disabled={busy} aria-busy={busy} onClick={actions.submit}>
                  <BusyLabel busy={busy} label={NUT_XOA} busyText={BUSY_DELETING} />
                </Button>
                <Button type="button" variant="outline" size="sm" disabled={busy} onClick={actions.cancel}>
                  {NUT_HUY}
                </Button>
              </RowActions>
              <span id={`${row.key}-delete-note`} className="an-thi-giac">
                {CANH_BAO_XOA}
              </span>
              <div className="max-w-80 text-right">
                <FormErrors localError={localError} serverError={serverError} />
              </div>
            </div>
          ) : (
            <RowButtons row={row} busy={busy} actions={actions} />
          )}
        </td>
      )}
    </tr>
  );
}

/**
 * The buttons of one row in view mode. WHICH buttons is decided in `tang-danh-muc.ts`, not here: a tier
 * number between two JSX tags is a second copy of the three-tier rule that no test reaches.
 */
function RowButtons({ row, busy, actions }: { row: CatalogueRow; busy: boolean; actions: CatalogueActions }) {
  const label = row.item.label;
  const pencil = (
    <Button
      type="button"
      variant="outline"
      size="sm"
      title={EDIT_LABEL}
      aria-label={nhanNutCuaDong(EDIT_LABEL, label)}
      disabled={busy}
      onClick={() => actions.edit(row)}
    >
      <Pencil aria-hidden="true" focusable="false" className="size-3.5" />
    </Button>
  );

  // #21: the eighth group renames and reorders, nothing else — the pencil only.
  if (row.kind === "taskStatus") return <RowActions>{pencil}</RowActions>;
  // No write route for the group, or a row that does not state its tier: no button (fail closed).
  if (row.group.ghi === null || !laMucGhi(row.item)) return null;

  const item = row.item;
  const allowed = thaoTacCuaMuc(item);
  const canDefault = row.group.khoa === GROUP_WITH_DEFAULT_BUTTON && allowed.doiNhan && item.active && !item.is_default;

  return (
    <RowActions>
      {allowed.doiNhan && pencil}
      {canDefault && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          title={SET_DEFAULT_TITLE}
          disabled={busy}
          onClick={() => actions.makeDefault(row)}
        >
          {SET_DEFAULT}
        </Button>
      )}
      {/* "Tắt" follows the tier; "Bật" does not — the trigger refuses only on → off at tier 3, so a tier-3
          row switched off must keep its way back (`choBatLai`). */}
      {allowed.tat && item.active && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          aria-label={nhanNutCuaDong(NUT_TAT, label)}
          disabled={busy}
          onClick={() => actions.setActive(row, false)}
        >
          {NUT_TAT}
        </Button>
      )}
      {choBatLai(item) && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          aria-label={nhanNutCuaDong(ENABLE, label)}
          disabled={busy}
          onClick={() => actions.setActive(row, true)}
        >
          {ENABLE}
        </Button>
      )}
      {allowed.xoa && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="text-danger"
          title={DELETE_ENTRY}
          aria-label={nhanNutCuaDong(DELETE_ENTRY, label)}
          disabled={busy}
          onClick={() => actions.remove(row)}
        >
          <Trash2 aria-hidden="true" focusable="false" className="size-3.5" />
        </Button>
      )}
    </RowActions>
  );
}
