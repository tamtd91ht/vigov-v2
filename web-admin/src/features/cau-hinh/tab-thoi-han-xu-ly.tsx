"use client";

import { AlertTriangle, Pencil, Plus, Trash2 } from "lucide-react";
import {
  Fragment,
  useCallback,
  useEffect,
  useState,
  type KeyboardEvent,
} from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { BUSY_DELETING, BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";

import { usePhien } from "@/features/phien/phien-hien-tai";
import type { KetQua } from "@/lib/api/goi";
import type {
  identity_danhSachSLARa,
  identity_dongSLARa,
} from "@/lib/api/schema.gen";
import {
  FIELD_ROW_KINDS,
  UNASSIGNED_HOLD_KEY,
  addSlaFieldRow,
  gieoThoiHanMacDinh,
  layThoiHanXuLy,
  readDocumentTypeOptions,
  readPetitionFieldOptions,
  readTaskPriorityOptions,
  removeSlaFieldRow,
  suaThoiHanXuLy,
  type FieldRowKind,
  type SlaFieldCatalogues,
} from "@/lib/api/thoi-han-xu-ly";
import { cn } from "@/lib/cn";

import { khoiCanhBao, tinhTrangBang } from "./chua-cau-hinh";
import {
  ConfigField,
  ConfigFormRow,
  ConfigLoading,
  ConfigTable,
  RowActions,
  SMALL_BUTTON_CLASS,
  formInputCls,
  formSelectCls,
} from "./config-ui";
import {
  ADDED_SLA,
  ADD_ACKNOWLEDGE_LABEL,
  ADD_FIELD_LABEL,
  ADD_FIELD_PLACEHOLDER,
  ADD_KIND_LABEL,
  ADD_RESOLVE_LABEL,
  ADD_SLA_BUTTON,
  ADD_SUBMIT,
  BUSY_ADDING,
  DA_LUU_THOI_HAN,
  NUT_HUY,
  NUT_LUU,
  NUT_XOA,
  O_LY_DO_XOA,
  PETITION_FIELDS_NOT_READABLE,
  REMOVED_SLA,
  REMOVE_SLA_TITLE,
  SLA_BANNER_APPLIES,
  SLA_BANNER_DUE_SOON_COLUMN,
  SLA_BANNER_DUE_SOON_LEAD,
  SLA_BANNER_DUE_SOON_REST,
  SLA_BANNER_HOLIDAYS,
  SLA_BANNER_LEAD,
  SLA_BANNER_UNIT,
  UNASSIGNED_HOLD_OFF,
  afterHoursCell,
  cauGieoThoiHan,
  hoursCell,
  nhanLinhVuc,
  nhanLoaiViec,
} from "./nhan-thoi-han";
import { quyetDinhGhiThoiHan, slaFieldLabelReadDecision } from "./quyen-tab";
// vi-name-ok: imports the existing exports of sua-thoi-han.ts unchanged (rule 12 invariant 3)
import {
  COT_GIO,
  NHAN_COT,
  banTuDong,
  changeAddKind,
  composeAdd,
  composeRemoveReason,
  newAddDraft,
  soanSua,
  type AddDraft,
  type BanNhapGio,
  type KhoaGio,
} from "./sua-thoi-han";
import { KhoiChuaKhai } from "./working-calendar-tab";

/**
 * Tab "Thời hạn xử lý" — spec `08-thoi-han-xu-ly.md` (ADR 0079), prototype `SlaTable.tsx`: the banner,
 * the add button, the table edited IN PLACE (`SlaTable.tsx:200-217`), nothing below it but one note.
 *
 * THE THREE CALENDAR TABLES LEFT THIS TAB (owner, 08/10/2026, ADR 0079 D1/D2) for "Lịch làm việc"
 * (`working-calendar-tab.tsx`), and this tab is gated on `admin.sla` by the frame: its one read,
 * `GET /api/v1/sla`, declares that key (`sla.go` says why), so without it there is nothing to show.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VIỆC SỐ MỘT CỦA MÀN NÀY KHÔNG PHẢI MỘT BẢNG ĐẸP. Nó là làm cho một xã mới biết mình đang thiếu
 * gì và bấm được nút gieo.
 *
 * Chuỗi phía sau `POST /api/v1/incoming-documents` đi qua `identity.ResolveDeadlines`, và hàm ấy
 * từ chối khi bảng thời hạn rỗng, từ chối khi lịch làm việc rỗng. Một xã vừa nhận hệ thống vì thế
 * KHÔNG vào sổ được văn bản đến và KHÔNG nhận được phản ánh — và lỗi ấy hiện ra ở một màn hình
 * khác hẳn màn hình sửa được nó. Khối `KhoiChuaKhai` là thứ thay cho một bước hướng dẫn ban đầu
 * không tồn tại; ở tab này nó chỉ nói về bảng thời hạn, ở tab Lịch làm việc thì về lịch tuần.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * "+ Thêm thời hạn cho một lĩnh vực" AND "Xoá thời hạn riêng" ARE LIVE (ADR 0079 lô 2 Q4): `POST
 * /api/v1/sla` and `DELETE /api/v1/sla/{id}`. The server checks the field code against the list's
 * owner (`service-identity/internal/app/sla_field_rule.go`); the select here only offers that list.
 * The add row toggles above the table, never a modal (spec 02); removal is a reason typed in the row.
 *
 * ẨN NÚT LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP: các tuyến ghi khai `RequirePermission("admin.sla")` và
 * kiểm trên TỪNG yêu cầu (luật 5, cấm #1).
 *
 * KHÔNG MỘT PHÉP CỘNG GIỜ LÀM VIỆC NÀO Ở ĐÂY. `identity` sở hữu bảng và sở hữu phép cộng (ADR 0007).
 */

/** Spec 08's pencil title — also the start of each pencil's accessible name. */
const EDIT_TITLE = "Sửa thời hạn";

/** Columns that count from a moment (a missed deadline, a hold) and so read "sau {n} giờ". */
const AFTER_COLUMNS: ReadonlySet<KhoaGio> = new Set<KhoaGio>([
  "escalate_leader_hours",
  "escalate_president_hours",
  UNASSIGNED_HOLD_KEY,
]);

const EMPTY_DRAFT: BanNhapGio = {
  acknowledge_hours: "",
  resolve_hours: "",
  due_soon_hours: "",
  escalate_leader_hours: "",
  escalate_president_hours: "",
  unassigned_hold_hours: "",
};

/** Marks a number box the browser could not parse; `soanSua` refuses it like any other non-number. */
const BAD_NUMBER = "?";

/**
 * What a `type="number"` box holds. A box the browser cannot parse ("-", "1e") reports `value === ""`,
 * and an empty sixth box MEANS "không báo" (`null`) — so a typo there would silently turn a report off
 * with "Đã lưu" on screen. `badInput` keeps it a refusal instead.
 */
function readNumberBox(el: HTMLInputElement): string {
  return el.validity.badInput ? BAD_NUMBER : el.value;
}

/** Các thao tác mà một dòng, nút Thêm hoặc khối cảnh báo có thể yêu cầu. */
export type ThaoTacThoiHan = {
  readonly gieoThoiHan: () => void;
  readonly suaThoiHan: (d: identity_dongSLARa) => void;
  /** Opens the add row, or closes it when open (spec 08: the button toggles). */
  readonly toggleAdd: () => void;
  /** Opens the reason step of one non-default row. */
  readonly startRemove: (d: identity_dongSLARa) => void;
};

/** No field list read yet — the select then offers only its placeholder. */
export const NO_CATALOGUES: SlaFieldCatalogues = {
  "phan-anh": null,
  "van-ban-den": null,
  "nhiem-vu": null,
};

/** The add row above the table. Two error slots, as for `InlineEdit`. */
export type AddForm = {
  readonly draft: AddDraft;
  readonly localError: string;
  readonly serverError: string;
  readonly busy: boolean;
  readonly setDraft: (d: AddDraft) => void;
  readonly onSubmit: () => void;
  readonly onCancel: () => void;
};

/** The row whose removal reason is being typed (rule 7: the server keeps the row and requires why). */
export type InlineRemove = {
  readonly rowId: string;
  readonly reason: string;
  readonly localError: string;
  readonly serverError: string;
  readonly busy: boolean;
  readonly setReason: (reason: string) => void;
  readonly onConfirm: () => void;
  readonly onCancel: () => void;
};

/**
 * Code → label for ONE row's kind. Per kind, never merged: a document-type code and a priority code may
 * coincide, and the wrong label on a commitment row is worse than the raw code. A kind with no list
 * (`don-thu`) or a failed read gives an empty map — `nhanLinhVuc` then shows the code.
 */
function labelsFor(catalogues: SlaFieldCatalogues, workKind: string): ReadonlyMap<string, string> {
  const kq = (FIELD_ROW_KINDS as readonly string[]).includes(workKind)
    ? catalogues[workKind as FieldRowKind]
    : null;
  return kq !== null && kq.ok ? new Map(kq.duLieu.map((o) => [o.code, o.label])) : new Map();
}

/** Lượt đọc của tab. `null` là CHƯA đọc xong — khác hẳn "đọc xong và rỗng". */
export type DuLieuTab = {
  readonly thoiHan: KetQua<identity_danhSachSLARa> | null;
};

/**
 * The row being edited in place. Two error slots, never merged: the local one says "a box is wrong",
 * the server one says "the request was refused" — one overwriting the other hides half of what to fix.
 */
export type InlineEdit = {
  readonly rowId: string;
  readonly draft: BanNhapGio;
  readonly localError: string;
  readonly serverError: string;
  readonly busy: boolean;
  readonly setDraft: (d: BanNhapGio) => void;
  readonly onSave: () => void;
  readonly onCancel: () => void;
};

/* ---- vỏ đọc dữ liệu ------------------------------------------------------------------------ */

export function TabThoiHanXuLy() {
  /** Tăng sau mỗi lần ghi thành công để ĐỌC LẠI từ máy chủ, không vá mảng tại chỗ. */
  const [lanDoc, datLanDoc] = useState(0);
  const [thoiHan, datThoiHan] = useState<KetQua<identity_danhSachSLARa> | null>(
    null,
  );

  const [editing, setEditing] = useState<identity_dongSLARa | null>(null);
  const [draft, setDraft] = useState<BanNhapGio>(EMPTY_DRAFT);
  const [localError, setLocalError] = useState("");
  const [serverError, setServerError] = useState("");
  const [seedError, setSeedError] = useState("");
  const [busy, setBusy] = useState(false);

  const phien = usePhien();
  /**
   * BA TRẠNG THÁI, KHÔNG HAI: chưa đọc xong phiên thì chưa vẽ nút ghi nào. "Chưa biết" không được
   * hành xử như "có quyền", và cũng không được hành xử như "thiếu quyền".
   */
  const coQuyenGhi = phien !== null && quyetDinhGhiThoiHan(phien).hien;

  /**
   * The three field lists — labels for the "Lĩnh vực" column (SLA-03) and the options of the add row.
   * Read ONCE per screen; not re-read after a write: no SLA write changes them.
   *
   * Document types and task priorities are `any-authenticated`. The petition fields declare
   * `admin.lookup`, which this tab (`admin.sla`) does not imply: an account without it is not sent a
   * request bound to answer 403 — its column keeps the raw code and the add row says why the list is
   * empty. "Session not read yet" reads nothing and says nothing (three states, not two).
   */
  const sessionKnown = phien !== null;
  const canReadFieldLabels =
    phien !== null && slaFieldLabelReadDecision(phien).hien;
  const [catalogues, setCatalogues] = useState<SlaFieldCatalogues>(NO_CATALOGUES);
  useEffect(() => {
    let bo = false;
    void Promise.all([readDocumentTypeOptions(), readTaskPriorityOptions()]).then(([documents, tasks]) => {
      if (!bo) setCatalogues((c) => ({ ...c, "van-ban-den": documents, "nhiem-vu": tasks }));
    });
    return () => {
      bo = true;
    };
  }, []);
  useEffect(() => {
    if (!canReadFieldLabels) return;
    let bo = false;
    void readPetitionFieldOptions().then((kq) => {
      if (!bo) setCatalogues((c) => ({ ...c, "phan-anh": kq }));
    });
    return () => {
      bo = true;
    };
  }, [canReadFieldLabels]);
  // Derived, not stored: the session decides it, and a second copy in state could disagree with it.
  const shownCatalogues: SlaFieldCatalogues =
    sessionKnown && !canReadFieldLabels
      ? { ...catalogues, "phan-anh": { ok: false, thongBao: PETITION_FIELDS_NOT_READABLE } }
      : catalogues;

  /** The open add row and the key it holds for every retry of this one add (`Idempotency-Key`). */
  const [adding, setAdding] = useState<{ draft: AddDraft; key: string } | null>(null);
  const [addLocalError, setAddLocalError] = useState("");
  const [addServerError, setAddServerError] = useState("");

  const [removing, setRemoving] = useState<{ row: identity_dongSLARa; reason: string } | null>(null);
  const [removeLocalError, setRemoveLocalError] = useState("");
  const [removeServerError, setRemoveServerError] = useState("");

  useEffect(() => {
    let bo = false;
    layThoiHanXuLy().then((kq) => {
      if (!bo) datThoiHan(kq);
    });
    return () => {
      bo = true;
    };
  }, [lanDoc]);

  const cancelRemove = useCallback(() => {
    setRemoving(null);
    setRemoveLocalError("");
    setRemoveServerError("");
  }, []);

  const startEdit = useCallback(
    (d: identity_dongSLARa) => {
      // One row in one mode at a time: a reason step left open on another row would be a second
      // pending act the officer no longer sees.
      cancelRemove();
      setEditing(d);
      setDraft(banTuDong(d));
      setLocalError("");
      setServerError("");
    },
    [cancelRemove],
  );

  const cancelEdit = useCallback(() => {
    setEditing(null);
    setDraft(EMPTY_DRAFT);
    setLocalError("");
    setServerError("");
  }, []);

  /** Success → toast + READ AGAIN; refusal → the server's sentence AS WRITTEN, in place. */
  const seed = useCallback(() => {
    if (busy) return;
    setSeedError("");
    setBusy(true);
    void gieoThoiHanMacDinh().then((kq) => {
      setBusy(false);
      if (!kq.ok) {
        setSeedError(kq.thongBao);
        return;
      }
      toast.success(cauGieoThoiHan(kq.duLieu.seeded, kq.duLieu.kept));
      datLanDoc((n) => n + 1);
    });
  }, [busy]);

  const save = useCallback(() => {
    if (editing === null || busy) return;
    setServerError("");
    const soan = soanSua(editing, draft);
    if (!soan.ok) {
      setLocalError(soan.loi);
      return;
    }
    setLocalError("");
    setBusy(true);
    void suaThoiHanXuLy(editing.id, soan.than).then((kq) => {
      setBusy(false);
      if (!kq.ok) {
        setServerError(kq.thongBao);
        return;
      }
      setEditing(null);
      setDraft(EMPTY_DRAFT);
      toast.success(DA_LUU_THOI_HAN);
      datLanDoc((n) => n + 1);
    });
  }, [busy, draft, editing]);

  const toggleAdd = useCallback(() => {
    setAddLocalError("");
    setAddServerError("");
    // A fresh key per opening: one opening is one intended row, and every retry of it replays.
    setAdding((open) =>
      open !== null ? null : { draft: newAddDraft(), key: crypto.randomUUID() },
    );
  }, []);

  const submitAdd = useCallback(() => {
    if (adding === null || busy) return;
    setAddServerError("");
    const rows = thoiHan !== null && thoiHan.ok ? thoiHan.duLieu.items : [];
    const composed = composeAdd(adding.draft, rows);
    if (!composed.ok) {
      setAddLocalError(composed.error);
      return;
    }
    setAddLocalError("");
    setBusy(true);
    void addSlaFieldRow(composed.body, adding.key).then((kq) => {
      setBusy(false);
      if (!kq.ok) {
        setAddServerError(kq.thongBao);
        return;
      }
      setAdding(null);
      toast.success(ADDED_SLA);
      datLanDoc((n) => n + 1);
    });
  }, [adding, busy, thoiHan]);

  const startRemove = useCallback(
    (d: identity_dongSLARa) => {
      cancelEdit();
      setRemoving({ row: d, reason: "" });
      setRemoveLocalError("");
      setRemoveServerError("");
    },
    [cancelEdit],
  );

  const confirmRemove = useCallback(() => {
    if (removing === null || busy) return;
    setRemoveServerError("");
    const composed = composeRemoveReason(removing.reason);
    if (!composed.ok) {
      setRemoveLocalError(composed.error);
      return;
    }
    setRemoveLocalError("");
    setBusy(true);
    void removeSlaFieldRow(removing.row.id, composed.reason).then((kq) => {
      setBusy(false);
      if (!kq.ok) {
        // 409 `default_sla_rule` included: the server's sentence is spec 08's, verbatim.
        setRemoveServerError(kq.thongBao);
        return;
      }
      setRemoving(null);
      toast.success(REMOVED_SLA);
      datLanDoc((n) => n + 1);
    });
  }, [busy, removing]);

  return (
    <ManThoiHanXuLy
      du={{ thoiHan }}
      coQuyenGhi={coQuyenGhi}
      catalogues={shownCatalogues}
      thaoTac={{ gieoThoiHan: seed, suaThoiHan: startEdit, toggleAdd, startRemove }}
      loiMayChuNgoaiForm={seedError}
      dangGui={busy}
      add={
        adding === null
          ? null
          : {
              draft: adding.draft,
              localError: addLocalError,
              serverError: addServerError,
              busy,
              setDraft: (d) => setAdding((open) => (open === null ? null : { ...open, draft: d })),
              onSubmit: submitAdd,
              onCancel: toggleAdd,
            }
      }
      remove={
        removing === null
          ? null
          : {
              rowId: removing.row.id,
              reason: removing.reason,
              localError: removeLocalError,
              serverError: removeServerError,
              busy,
              setReason: (reason) =>
                setRemoving((open) => (open === null ? null : { ...open, reason })),
              onConfirm: confirmRemove,
              onCancel: cancelRemove,
            }
      }
      edit={
        editing === null
          ? null
          : {
              rowId: editing.id,
              draft,
              localError,
              serverError,
              busy,
              setDraft,
              onSave: save,
              onCancel: cancelEdit,
            }
      }
    />
  );
}

/* ---- phần trình bày ------------------------------------------------------------------------ */

/**
 * Toàn bộ phần nhìn thấy được của tab, THUẦN TRÌNH BÀY.
 *
 * XUẤT RA để bài kiểm kết xuất được bằng `react-dom/server` mà không cần trình duyệt giả lập: mọi
 * QUYẾT ĐỊNH (câu nào hiện, nút nào có) phải được canh ở chỗ nó ra tới trang, không chỉ trong module
 * thuần.
 */
export function ManThoiHanXuLy({
  du,
  coQuyenGhi,
  catalogues = NO_CATALOGUES,
  thaoTac,
  loiMayChuNgoaiForm,
  dangGui,
  edit = null,
  add = null,
  remove = null,
}: {
  du: DuLieuTab;
  coQuyenGhi: boolean;
  /** The three field lists: labels of the "Lĩnh vực" column, options of the add row. Omitted = none. */
  catalogues?: SlaFieldCatalogues;
  thaoTac: ThaoTacThoiHan;
  /** Lỗi của nút gieo — the one action that opens no row. */
  loiMayChuNgoaiForm: string;
  dangGui: boolean;
  /** The row being edited in place, or `null`. */
  edit?: InlineEdit | null;
  /** The open add row, or `null`. */
  add?: AddForm | null;
  /** The row whose removal reason is being typed, or `null`. */
  remove?: InlineRemove | null;
}) {
  // Only the SLA table here; the weekly-hours half of the block lives on "Lịch làm việc" (ADR 0079 D2).
  const khoi = khoiCanhBao(tinhTrangBang(du.thoiHan), "chuaBiet");
  const kq = du.thoiHan;
  const rows = kq !== null && kq.ok ? kq.duLieu.items : [];

  return (
    <section className="space-y-3" aria-labelledby="tieu-de-thoi-han">
      <h2 id="tieu-de-thoi-han" className="an-thi-giac">
        Thời hạn xử lý
      </h2>
      <KhoiChuaKhai
        khoi={khoi}
        coQuyenGhi={coQuyenGhi}
        dangGui={dangGui}
        onSeedSla={thaoTac.gieoThoiHan}
      />

      {/* Spec 08 "Hộp giải thích" (`SlaTable.tsx:68-78`). `border-solid`: preflight is off here, so a
          bare `border` draws nothing. */}
      <div className="border-brand/25 bg-brand/8 text-navy flex gap-2.5 rounded-[10px] border border-solid p-3 text-[12.5px]">
        <AlertTriangle
          aria-hidden="true"
          focusable="false"
          className="text-brand mt-0.5 size-4 shrink-0"
        />
        <span>
          {SLA_BANNER_LEAD}
          <b>{SLA_BANNER_UNIT}</b>
          {SLA_BANNER_HOLIDAYS}
          {SLA_BANNER_APPLIES}
          <br />
          {SLA_BANNER_DUE_SOON_LEAD}
          <b>{SLA_BANNER_DUE_SOON_COLUMN}</b>
          {SLA_BANNER_DUE_SOON_REST}
        </span>
      </div>

      {coQuyenGhi && (
        // NO RE-SEED BUTTON once the table has rows (owner, 08/10/2026: "Bỏ hết, đúng prototype"). The
        // only seed button is the empty table's, inside `KhoiChuaKhai`.
        <div className="flex justify-end">
          {/* `SlaTable.tsx:80-87`: toggles the add row. */}
          <Button
            type="button"
            variant="primary"
            size="sm"
            className={SMALL_BUTTON_CLASS}
            aria-expanded={add !== null}
            aria-controls={add !== null ? ADD_FORM_ID : undefined}
            onClick={thaoTac.toggleAdd}
          >
            <Plus aria-hidden="true" focusable="false" strokeWidth={1.8} />
            {ADD_SLA_BUTTON}
          </Button>
        </div>
      )}

      {coQuyenGhi && add !== null && (
        <AddRow add={add} catalogues={catalogues} />
      )}

      {loiMayChuNgoaiForm !== "" && (
        <p role="alert" className="text-danger m-0 text-[12.5px]">
          {loiMayChuNgoaiForm}
        </p>
      )}

      {kq === null && <ConfigLoading label="Đang tải bảng thời hạn xử lý…" />}
      {kq !== null && !kq.ok && (
        // NGUYÊN VĂN câu máy chủ viết.
        <ErrorState
          role="alert"
          title="Chưa tải được bảng này"
          message={kq.thongBao}
          className="py-6"
        />
      )}

      {/* SAI SÓT CỦA CHÍNH DỮ LIỆU XÃ, máy chủ suy ra từ đúng những dòng nó vừa trả về. Tuyến vẫn trả
          200 có chủ ý: đây là màn hình SỬA nó, nên phải mở được kể cả khi đang hỏng. */}
      {kq !== null &&
        kq.ok &&
        kq.duLieu.problems.map((v, i) => (
          <p
            role="alert"
            className="text-danger m-0 text-[12.5px]"
            key={`${v.kind}-${v.work_kind}-${i}`}
          >
            {v.message}
          </p>
        ))}

      {rows.length > 0 && (
        <SlaTable
          rows={rows}
          coQuyenGhi={coQuyenGhi}
          catalogues={catalogues}
          thaoTac={thaoTac}
          dangGui={dangGui}
          edit={edit}
          remove={remove}
        />
      )}
    </section>
  );
}

/** One figure, read-only: "{n} giờ", "sau {n} giờ", or "—" for an unset sixth figure. */
function FigureText({
  column,
  value,
}: {
  column: KhoaGio;
  value: number | null;
}) {
  if (value === null)
    return (
      <>
        <span aria-hidden="true">—</span>
        <span className="sr-only">{UNASSIGNED_HOLD_OFF}</span>
      </>
    );
  return (
    <>{AFTER_COLUMNS.has(column) ? afterHoursCell(value) : hoursCell(value)}</>
  );
}

/**
 * The SLA table (`SlaTable.tsx:96-126`) plus the sixth figure "Giữ chưa phân công" before the actions
 * (ADR 0079 decision 3). GIỮ NGUYÊN THỨ TỰ MÁY CHỦ TRẢ VỀ: sắp lại theo tên lĩnh vực sẽ tách dòng mặc
 * định khỏi nhóm của nó.
 */
function SlaTable({
  rows,
  coQuyenGhi,
  catalogues,
  thaoTac,
  dangGui,
  edit,
  remove,
}: {
  rows: readonly identity_dongSLARa[];
  coQuyenGhi: boolean;
  catalogues: SlaFieldCatalogues;
  thaoTac: ThaoTacThoiHan;
  dangGui: boolean;
  edit: InlineEdit | null;
  remove: InlineRemove | null;
}) {
  const columnCount = 2 + COT_GIO.length + (coQuyenGhi ? 1 : 0);

  return (
    // FITS 1144px WITHOUT SCROLLING THE ACTIONS AWAY: `.data-table thead th` is `nowrap` (legacy layer,
    // so a utility beats it), and the six long figure headings alone overran the content width. Headings
    // wrap; each figure column keeps 4.5rem so "Tiếp nhận" does not collapse to one word per line. Rough
    // budget, 16px cell padding included: kind ~166 + field ~186 + 6 × 88 + actions ~86 ≈ 970px read,
    // ≈ 1050px while editing (6 × (80px input + 16)). Body cells stay `nowrap` (ConfigTable).
    <div className="[&_th]:min-w-[4.5rem] [&_th]:whitespace-normal">
      <ConfigTable
        label="Thời hạn xử lý"
        caption="Số giờ làm việc cho từng loại việc và lĩnh vực của đơn vị"
      >
        <thead>
          <tr>
            <th scope="col">Loại việc</th>
            <th scope="col">Lĩnh vực</th>
            {COT_GIO.map((c) => (
              <th scope="col" key={c}>
                {NHAN_COT[c]}
              </th>
            ))}
            {coQuyenGhi && (
              <th scope="col">
                <span className="an-thi-giac">Thao tác</span>
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {rows.map((d) => {
            const kindText = nhanLoaiViec(d.work_kind);
            const fieldText = nhanLinhVuc(
              d.field,
              d.is_default,
              labelsFor(catalogues, d.work_kind),
            );
            const editing =
              coQuyenGhi && edit !== null && edit.rowId === d.id ? edit : null;
            // Never on the default row, even if a stale state names it: the server refuses it anyway.
            const removing =
              coQuyenGhi && editing === null && !d.is_default && remove !== null && remove.rowId === d.id
                ? remove
                : null;
            const rowErrors =
              editing !== null
                ? [editing.localError, editing.serverError]
                : removing !== null
                  ? [removing.localError, removing.serverError]
                  : [];
            return (
              <Fragment key={d.id}>
                <tr>
                  <td className="text-navy font-medium">{kindText}</td>
                  <td className={d.is_default ? "text-ink-muted" : undefined}>
                    {fieldText}
                  </td>
                  {COT_GIO.map((c) => (
                    <td
                      key={c}
                      className={
                        c === "resolve_hours" ? "font-semibold" : undefined
                      }
                    >
                      {editing === null ? (
                        <FigureText column={c} value={d[c]} />
                      ) : (
                        <HoursInput column={c} edit={editing} />
                      )}
                    </td>
                  ))}
                  {coQuyenGhi && (
                    <td>
                      <RowActions>
                        {editing !== null ? (
                          <>
                            <Button
                              type="button"
                              variant="primary"
                              size="sm"
                              disabled={editing.busy}
                              aria-busy={editing.busy}
                              onClick={editing.onSave}
                            >
                              <BusyLabel
                                busy={editing.busy}
                                label={NUT_LUU}
                                busyText={BUSY_SAVING}
                              />
                            </Button>
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              disabled={editing.busy}
                              onClick={editing.onCancel}
                            >
                              {NUT_HUY}
                            </Button>
                          </>
                        ) : removing !== null ? (
                          <RemoveStep
                            remove={removing}
                            rowName={`${kindText} — ${fieldText}`}
                          />
                        ) : (
                          <>
                            <Button
                              type="button"
                              variant="secondary"
                              size="sm"
                              title={EDIT_TITLE}
                              aria-label={`${EDIT_TITLE} ${kindText} — ${fieldText}`}
                              disabled={dangGui}
                              onClick={() => thaoTac.suaThoiHan(d)}
                            >
                              <Pencil
                                aria-hidden="true"
                                focusable="false"
                                className="size-3.5"
                              />
                            </Button>
                            {/* Hidden on the default row (spec 08): every field without its own row
                              falls back to it. */}
                            {!d.is_default && (
                              <Button
                                type="button"
                                variant="secondary"
                                size="sm"
                                className="text-danger"
                                title={REMOVE_SLA_TITLE}
                                aria-label={`${REMOVE_SLA_TITLE} ${kindText} — ${fieldText}`}
                                disabled={dangGui}
                                onClick={() => thaoTac.startRemove(d)}
                              >
                                <Trash2
                                  aria-hidden="true"
                                  focusable="false"
                                  className="size-3.5"
                                />
                              </Button>
                            )}
                          </>
                        )}
                      </RowActions>
                    </td>
                  )}
                </tr>
                {rowErrors.some((e) => e !== "") && (
                  <tr>
                    <td colSpan={columnCount}>
                      <div className="space-y-1 whitespace-normal">
                        {rowErrors
                          .filter((e) => e !== "")
                          .map((e, i) => (
                            <p
                              key={i}
                              role="alert"
                              className="text-danger m-0 text-[12.5px]"
                            >
                              {e}
                            </p>
                          ))}
                      </div>
                    </td>
                  </tr>
                )}
              </Fragment>
            );
          })}
        </tbody>
      </ConfigTable>
    </div>
  );
}

/** One figure in edit mode (`SlaTable.tsx:200-214`): Enter saves, Esc cancels. */
function HoursInput({ column, edit }: { column: KhoaGio; edit: InlineEdit }) {
  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      edit.onSave();
    } else if (e.key === "Escape") {
      e.preventDefault();
      edit.onCancel();
    }
  };
  return (
    <input
      type="number"
      min={0}
      name={column}
      aria-label={`${NHAN_COT[column]} (giờ)`}
      className={cn(controlClass, "h-8 w-20 text-[12.5px]")}
      value={edit.draft[column]}
      disabled={edit.busy}
      onChange={(e) =>
        edit.setDraft({
          ...edit.draft,
          [column]: readNumberBox(e.currentTarget),
        })
      }
      onKeyDown={onKeyDown}
    />
  );
}

/** The id of the add row — the add button's `aria-controls`. */
const ADD_FORM_ID = "sla-add-row";

/**
 * The reason step of one row's removal, inside the action cell: compact (spec 08 draws only a trash
 * button), but never a one-click delete — the server keeps the row and records why (rule 7). Enter
 * confirms, Esc cancels. Refusals show in the row below, like the edit's.
 */
function RemoveStep({ remove, rowName }: { remove: InlineRemove; rowName: string }) {
  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      remove.onConfirm();
    } else if (e.key === "Escape") {
      e.preventDefault();
      remove.onCancel();
    }
  };
  return (
    <>
      <input
        name="reason"
        autoFocus
        aria-label={`${O_LY_DO_XOA} — ${rowName}`}
        aria-invalid={remove.localError !== ""}
        placeholder={O_LY_DO_XOA}
        className={cn(controlClass, "h-8 w-44 text-[12.5px]")}
        value={remove.reason}
        disabled={remove.busy}
        onChange={(e) => remove.setReason(e.currentTarget.value)}
        onKeyDown={onKeyDown}
      />
      <Button
        type="button"
        variant="danger"
        size="sm"
        disabled={remove.busy}
        aria-busy={remove.busy}
        onClick={remove.onConfirm}
      >
        <BusyLabel busy={remove.busy} label={NUT_XOA} busyText={BUSY_DELETING} />
      </Button>
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={remove.busy}
        onClick={remove.onCancel}
      >
        {NUT_HUY}
      </Button>
    </>
  );
}

/**
 * The grey add row (spec 08, `SlaTable.tsx:294-409`). Only the two figures the spec shows are typed;
 * the other four come from the kind's default row (`composeAdd`). The field select offers the ACTIVE
 * entries of the kind's own list — the server refuses a switched-off code (`sla_field_unknown`).
 */
function AddRow({ add, catalogues }: { add: AddForm; catalogues: SlaFieldCatalogues }) {
  const list = catalogues[add.draft.workKind];
  const options = list !== null && list.ok ? list.duLieu.filter((o) => o.active) : [];
  const listError = list !== null && !list.ok ? list.thongBao : "";
  const fieldMissing = add.localError !== "" && add.draft.field === "";

  return (
    <ConfigFormRow
      id={ADD_FORM_ID}
      columns="sm:grid-cols-[14rem_1fr_8rem_8rem_auto]"
      aria-label={ADD_SLA_BUTTON}
      onSubmit={(e) => {
        e.preventDefault();
        add.onSubmit();
      }}
      onKeyDown={(e) => {
        // Esc closes like `Huỷ` — except while sending, so a send in flight stays in view.
        if (e.key === "Escape" && !add.busy) add.onCancel();
      }}
    >
      <ConfigField label={ADD_KIND_LABEL} htmlFor="sla-add-kind">
        <select
          id="sla-add-kind"
          name="work_kind"
          autoFocus
          className={formSelectCls}
          value={add.draft.workKind}
          disabled={add.busy}
          onChange={(e) => {
            const kind = FIELD_ROW_KINDS.find((k) => k === e.currentTarget.value);
            if (kind !== undefined) add.setDraft(changeAddKind(add.draft, kind));
          }}
        >
          {FIELD_ROW_KINDS.map((k) => (
            <option key={k} value={k}>
              {nhanLoaiViec(k)}
            </option>
          ))}
        </select>
      </ConfigField>

      <ConfigField label={ADD_FIELD_LABEL} htmlFor="sla-add-field">
        <select
          id="sla-add-field"
          name="field"
          className={formSelectCls}
          value={add.draft.field}
          disabled={add.busy}
          aria-invalid={fieldMissing}
          onChange={(e) => add.setDraft({ ...add.draft, field: e.currentTarget.value })}
        >
          <option value="">{ADD_FIELD_PLACEHOLDER}</option>
          {options.map((o) => (
            <option key={o.code} value={o.code}>
              {o.label}
            </option>
          ))}
        </select>
      </ConfigField>

      <ConfigField label={ADD_ACKNOWLEDGE_LABEL} htmlFor="sla-add-acknowledge">
        <input
          id="sla-add-acknowledge"
          name="acknowledge_hours"
          type="number"
          min={1}
          step={1}
          className={formInputCls}
          value={add.draft.acknowledgeHours}
          disabled={add.busy}
          onChange={(e) => add.setDraft({ ...add.draft, acknowledgeHours: readNumberBox(e.currentTarget) })}
        />
      </ConfigField>

      <ConfigField label={ADD_RESOLVE_LABEL} htmlFor="sla-add-resolve">
        <input
          id="sla-add-resolve"
          name="resolve_hours"
          type="number"
          min={1}
          step={1}
          className={formInputCls}
          value={add.draft.resolveHours}
          disabled={add.busy}
          onChange={(e) => add.setDraft({ ...add.draft, resolveHours: readNumberBox(e.currentTarget) })}
        />
      </ConfigField>

      <div className="flex gap-2">
        <Button type="submit" variant="primary" disabled={add.busy} aria-busy={add.busy}>
          <BusyLabel busy={add.busy} label={ADD_SUBMIT} busyText={BUSY_ADDING} />
        </Button>
        <Button type="button" variant="outline" disabled={add.busy} onClick={add.onCancel}>
          {NUT_HUY}
        </Button>
      </div>

      {/* The list's own read failure (or why it cannot be read), then the two error slots apart. */}
      {listError !== "" && (
        <p className="text-ink-muted col-span-full m-0 text-[12.5px]">{listError}</p>
      )}
      {add.localError !== "" && (
        <p role="alert" className="text-danger col-span-full m-0 text-[12.5px]">
          {add.localError}
        </p>
      )}
      {add.serverError !== "" && (
        <p role="alert" className="text-danger col-span-full m-0 text-[12.5px]">
          {add.serverError}
        </p>
      )}
    </ConfigFormRow>
  );
}
