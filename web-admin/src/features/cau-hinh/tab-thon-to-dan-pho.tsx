"use client";

import { Pencil, Plus, PowerOff, RotateCcw } from "lucide-react";
import { useCallback, useEffect, useId, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ErrorState } from "@/components/ui/error-state";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";

import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi";
import { layLoaiDonViDanCu } from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import type {
  identity_danhBaChonNguoiRa,
  identity_danhSachLoaiDonViDanCuRa,
  identity_thonToDanPhoRa,
} from "@/lib/api/schema.gen";
import { layDanhSachThonToDanPho } from "@/lib/api/thon-to-dan-pho";
import { QUYEN_QUAN_LY_SO_DO, quyetDinhTheoKhoa } from "@/lib/quyen";
import { usePhien } from "@/features/phien/phien-hien-tai";

import { ConfigDialog } from "./config-dialog";
import { ConfigImportButton } from "./config-import-button";
import {
  ConfigField,
  ConfigLoading,
  ConfigTable,
  EmptyRow,
  RowActions,
  SMALL_BUTTON_CLASS,
  StatusBadge,
  formInputCls,
  formSelectCls,
} from "./config-ui";
import { RESIDENTIAL_UNIT_IMPORT_TARGET } from "./excel-import-targets";
import { DIALOG_FOOTER_CLASS } from "./org-unit-dialog-classes";
import { NUT_HUY, NUT_LUU } from "./nhan-so-do";
import { THON_RONG, loaiDonVi, nhanLoaiDonVi } from "./nhan-thon";
import {
  ADD_BUTTON,
  CONFIRM_RETIRE_BUTTON,
  CREATE_TITLE,
  EDIT_BUTTON,
  EMPTY_CELL,
  FIELD_HEAD,
  FIELD_HOUSEHOLDS,
  FIELD_NAME,
  FIELD_ORDER,
  FIELD_POPULATION,
  FIELD_TYPE,
  HEAD_NONE,
  NAME_PLACEHOLDER,
  REACTIVATE_BUTTON,
  RESIDENTIAL_UNIT_API,
  RETIRE_BUTTON,
  SAVE_FAILED,
  TYPE_NONE,
  draftForCreate,
  draftFromUnit,
  editButtonLabel,
  editTitle,
  headOptions,
  openCreate,
  reactivateButtonLabel,
  retireButtonLabel,
  retireConfirmSentence,
  submitResidentialUnitForm,
  toggleResidentialUnitActive,
  typeOptions,
  type PickerOption,
  type ResidentialUnitDraft,
  type ResidentialUnitFormOpen,
} from "./residential-unit-form";

/**
 * Tab "Thôn / Tổ dân phố" — spec `04-thon-to-dan-pho.md` in the shared table pattern of spec 02
 * (`config-ui.tsx`, ADR 0079), prototype `HamletTable.tsx`: import and add buttons on one row, the
 * framed table. Add / edit open a DIALOG like every other Cấu hình add/edit (bug sheet row 32 — it
 * replaced spec 02's grey row above the table).
 *
 * WHAT THE SPEC DROPS AND THIS TAB KEEPS (ADR 0079 decision 3, "Giữ, trình bày theo spec"): the
 * `Trưởng thôn / Tổ trưởng` picker and column, the `Thứ tự` box, and `Loại` editable after create —
 * the server serves all three. They are fields of the same dialog.
 *
 * TAB RIÊNG, KHÔNG NHẬP VÀO TAB DANH MỤC, dù cả hai cùng đọc một tuyến kiểu danh sách: một thôn
 * không phải một mục danh mục. Hợp đồng nói ra điều đó bằng chính tên trường (`name` chứ không
 * phải `label`), máy chủ nói ra bằng hai con số chỉ một địa bàn có thật mới có (số hộ, nhân
 * khẩu), và đặc tả tách đôi thành hai tab.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 * DANH SÁCH HIỆN CHO MỌI TÀI KHOẢN, CHỈ NÚT GHI MỚI ẨN — cùng khuôn tab Sơ đồ tổ chức, và vì cùng một
 * lý do ở máy chủ: `GET /api/v1/residential-units` khai `any-authenticated` (tên địa bàn có ở bộ lọc
 * phản ánh), còn hai tuyến ghi và ba tuyến nhập khai `RequirePermission("admin.org")`. Ẩn nút là TIỆN
 * DỤNG, không phải biện pháp: máy chủ kiểm khoá trên TỪNG yêu cầu (luật 5, cấm #1).
 *
 * KHÔNG CÓ XOÁ, CÓ CHỦ Ý (ADR 0059 §2, ADR 0068 §15). The prototype's Trash2 position is "Ngừng dùng"
 * (`active: false`); the unit stays listed and records pointing at it keep printing its name
 * (`residential_unit_write.go:20-24`). Ngừng dùng asks once; Dùng lại does not — it only puts a unit
 * back in the pickers.
 *
 * ĐỌC LẠI SAU MỖI LẦN GHI, KHÔNG VÁ TẠI CHỖ: thứ tự và nhãn loại là của máy chủ.
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 */

/** Ba nhánh rời nhau — cùng khuôn với `danh-ba-can-bo.tsx`. Không nhánh nào suy ra được từ nhánh khác. */
export type ResidentialUnitsLoad =
  | { readonly phase: "loading" }
  | { readonly phase: "error"; readonly message: string }
  | { readonly phase: "ready"; readonly units: readonly identity_thonToDanPhoRa[] };

/** What the tab's buttons call. */
export type ResidentialUnitActions = {
  readonly add: () => void;
  readonly edit: (u: identity_thonToDanPhoRa) => void;
  readonly askRetire: (u: identity_thonToDanPhoRa) => void;
  readonly confirmRetire: () => void;
  readonly cancelRetire: () => void;
  readonly reactivate: (u: identity_thonToDanPhoRa) => void;
  /** Read the list again — after an Excel import, whose rows exist only on the server. */
  readonly reload: () => void;
};

const NAME_INPUT_ID = "o-ten-thon";
const ADD_BUTTON_ID = "nut-them-thon";

export function TabThonToDanPho() {
  const [load, setLoad] = useState<ResidentialUnitsLoad>({ phase: "loading" });
  const [reads, setReads] = useState(0);

  const [open, setOpen] = useState<ResidentialUnitFormOpen | null>(null);
  const [draft, setDraft] = useState<ResidentialUnitDraft>(draftForCreate());
  const [localError, setLocalError] = useState("");
  const [serverError, setServerError] = useState("");
  const [sending, setSending] = useState(false);

  const [retiring, setRetiring] = useState<identity_thonToDanPhoRa | null>(null);
  /** A toggle refused by the server, said under the row it was pressed on. */
  const [rowError, setRowError] = useState<{ id: string; message: string } | null>(null);

  const [types, setTypes] = useState<KetQua<identity_danhSachLoaiDonViDanCuRa> | null>(null);
  const [directory, setDirectory] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);

  const phien = usePhien();
  /** Ba trạng thái: chưa đọc xong phiên thì chưa vẽ nút ghi nào. */
  const decision = phien === null ? null : quyetDinhTheoKhoa(phien, QUYEN_QUAN_LY_SO_DO);
  const canWrite = decision !== null && decision.hien;

  /**
   * ĐỌC LẠI theo `reads`. KHÔNG ĐỆM QUA LẦN MỞ MÀN HÌNH: danh sách địa bàn là dữ liệu của MỘT đơn vị,
   * và một bản đệm sống lâu hơn yêu cầu, trong một tiến trình phục vụ nhiều tên miền, là đúng hình dạng
   * của một lần dữ liệu đơn vị này hiện trên màn hình đơn vị khác (luật 1, cấm #1).
   */
  useEffect(() => {
    let dropped = false;
    layDanhSachThonToDanPho().then((kq) => {
      if (dropped) return;
      setLoad(kq.ok ? { phase: "ready", units: kq.duLieu.items } : { phase: "error", message: kq.thongBao });
    });
    return () => {
      dropped = true;
    };
  }, [reads]);

  // The two pickers' sources, read only for an account that can write — nobody else opens the form.
  useEffect(() => {
    if (!canWrite) return;
    let dropped = false;
    layLoaiDonViDanCu().then((kq) => {
      if (!dropped) setTypes(kq);
    });
    layDanhBaChonNguoi().then((kq) => {
      if (!dropped) setDirectory(kq);
    });
    return () => {
      dropped = true;
    };
  }, [canWrite]);

  // Focus is the dialog's: `Tên` when it opens (`initialFocusId`), the opener when it closes (`ModalDialog`).

  const clearMessages = useCallback(() => {
    setLocalError("");
    setServerError("");
    setRowError(null);
  }, []);

  /** A done write: the sentence as a toast, then READ AGAIN. */
  const done = useCallback((sentence: string) => {
    toast.success(sentence);
    setReads((n) => n + 1);
  }, []);

  const actions: ResidentialUnitActions = {
    // Opens the create dialog. (The toggle branch is kept from the grey row; with the modal open the
    // button sits behind the backdrop, so it no longer fires.)
    add: () => {
      clearMessages();
      setRetiring(null);
      if (open !== null && open.kind === "create") {
        setOpen(null);
        return;
      }
      setOpen(openCreate(() => crypto.randomUUID()));
      setDraft(draftForCreate());
    },
    edit: (u) => {
      clearMessages();
      setRetiring(null);
      setOpen({ kind: "edit", unit: u });
      setDraft(draftFromUnit(u));
    },
    askRetire: (u) => {
      clearMessages();
      setOpen(null);
      setRetiring(u);
    },
    cancelRetire: () => setRetiring(null),
    confirmRetire: () => {
      if (retiring === null || sending) return;
      const u = retiring;
      setSending(true);
      void toggleResidentialUnitActive(u, RESIDENTIAL_UNIT_API).then((r) => {
        setSending(false);
        setRetiring(null);
        if (r.kind === "done") done(r.sentence);
        else if (r.kind === "serverError") setRowError({ id: u.id, message: r.message });
      });
    },
    reactivate: (u) => {
      if (sending) return;
      clearMessages();
      setOpen(null);
      setRetiring(null);
      setSending(true);
      void toggleResidentialUnitActive(u, RESIDENTIAL_UNIT_API).then((r) => {
        setSending(false);
        if (r.kind === "done") done(r.sentence);
        else if (r.kind === "serverError") setRowError({ id: u.id, message: r.message });
      });
    },
    reload: () => setReads((n) => n + 1),
  };

  const submit = () => {
    if (open === null || sending) return;
    setLocalError("");
    setServerError("");
    setSending(true);
    void submitResidentialUnitForm(open, draft, RESIDENTIAL_UNIT_API).then((r) => {
      setSending(false);
      if (r.kind === "localError") {
        setLocalError(r.message);
        return;
      }
      if (r.kind === "serverError") {
        // The form KEEPS what was typed and KEEPS the create key: the next press is a retry of the
        // SAME add, never a second unit.
        setServerError(r.message);
        return;
      }
      setOpen(null);
      done(r.sentence);
    });
  };

  const current = open !== null && open.kind === "edit" ? open.unit : null;
  const form =
    open === null ? null : (
      <ResidentialUnitForm
        // A new key per row: a different row remounts the dialog, so its opening focus and the draft
        // both belong to the row now open.
        key={open.kind === "edit" ? open.unit.id : "create"}
        open={open}
        draft={draft}
        setDraft={setDraft}
        typeChoices={typeOptions(
          types !== null && types.ok ? types.duLieu.items : [],
          current === null ? null : { code: current.type_code, label: current.type_label },
        )}
        headChoices={headOptions(
          directory !== null && directory.ok ? directory.duLieu.items : [],
          current === null ? null : { code: current.head_staff_code, name: current.head_staff_name },
        )}
        typesError={types !== null && !types.ok ? types.thongBao : ""}
        directoryError={directory !== null && !directory.ok ? directory.thongBao : ""}
        localError={localError}
        serverError={serverError}
        sending={sending}
        onSubmit={submit}
        onCancel={() => {
          setOpen(null);
          setLocalError("");
          setServerError("");
        }}
      />
    );

  return (
    // No card, no visible title: the tab IS the section, as in the prototype.
    <section className="min-w-0" aria-labelledby="tieu-de-thon">
      <h2 id="tieu-de-thon" className="an-thi-giac">
        Thôn / Tổ dân phố
      </h2>
      {decision !== null && !decision.hien && decision.vi === "khong-doc-duoc" && (
        <InlineError>{decision.thongBao}</InlineError>
      )}
      <ResidentialUnitsView
        load={load}
        canWrite={canWrite}
        actions={actions}
        sending={sending}
        form={form}
        retiring={retiring}
        rowError={rowError}
      />
    </section>
  );
}

/** `id` of a row's edit button — focus returns there when the form closes. */
export function editButtonId(unitId: string): string {
  return `nut-sua-thon-${unitId}`;
}

/** A sentence in place, in the spec's error type. */
function InlineError({ children }: { children: ReactNode }) {
  return (
    <p role="alert" className="text-danger m-0 text-[12px] font-medium">
      {children}
    </p>
  );
}

/** Spec 04: `toLocaleString("vi-VN")`, empty → "—". `null` is "chưa nhập", never shown as 0. */
function countCell(n: number | null): string {
  return n === null ? EMPTY_CELL : n.toLocaleString("vi-VN");
}

/**
 * The tab's body: import, add, the open form, the list. PURE PRESENTATION and EXPORTED so the test
 * renders the allowed AND the denied case with `react-dom/server` — "no write button without
 * `admin.org`" is a fact of this JSX, not of a pure function.
 */
export function ResidentialUnitsView({
  load,
  canWrite,
  actions,
  sending,
  form,
  retiring,
  rowError,
}: {
  load: ResidentialUnitsLoad;
  canWrite: boolean;
  actions: ResidentialUnitActions;
  sending: boolean;
  /**
   * The open add/edit dialog, or `null`. A DIALOG, like every other Cấu hình add/edit (bug sheet row 32,
   * the customer's call over spec 02's "grey row above the table, never a modal").
   */
  form: ReactNode;
  retiring: identity_thonToDanPhoRa | null;
  rowError: { id: string; message: string } | null;
}) {
  return (
    <>
      {/* ONE right-aligned row: "Nhập từ Excel" then the add button (bug sheet row 28 — the prototype stacks
          them; the customer wants one line). The import draws its own `mb-3 flex justify-end` row;
          `[&>div]:mb-0` makes it an item of this one, which keeps the `mb-3` to the table. */}
      {canWrite && (
        <div className="mb-3 flex flex-wrap items-center justify-end gap-2 [&>div]:mb-0">
          <ConfigImportButton target={RESIDENTIAL_UNIT_IMPORT_TARGET} onImported={actions.reload} />
          {/* Not while loading: the prototype draws the add button only once the list has arrived
              (`ConfigWorkspace.tsx:99`). The import stays. */}
          {load.phase !== "loading" && (
            <Button
              type="button"
              variant="primary"
              size="sm"
              id={ADD_BUTTON_ID}
              className={SMALL_BUTTON_CLASS}
              icon={<Plus aria-hidden="true" focusable="false" className="size-4" />}
              onClick={actions.add}
            >
              {ADD_BUTTON}
            </Button>
          )}
        </div>
      )}

      <div className="flex min-w-0 flex-col gap-3">
        {/* The add / edit DIALOG (bug sheet row 32), mounted while open. */}
        {canWrite && form}

        {load.phase === "loading" && <ConfigLoading label="Đang tải danh sách địa bàn…" />}

        {/* LỖI: hiện đúng `message` của máy chủ, không diễn giải và không rẽ nhánh theo `code`. */}
        {load.phase === "error" && (
          <ErrorState role="alert" title="Chưa tải được danh sách thôn / tổ dân phố" message={load.message} />
        )}

        {/* TRẠNG THÁI RỖNG, KHÔNG PHẢI LỖI: máy chủ trả `items: []`, nói trong khung bảng. */}
        {load.phase === "ready" && (
          <UnitsTable
            units={load.units}
            canWrite={canWrite}
            actions={actions}
            sending={sending}
            retiring={retiring}
            rowError={rowError}
          />
        )}
      </div>
    </>
  );
}

/**
 * Bảng địa bàn. Giữ nguyên thứ tự máy chủ trả về. KHÔNG LỌC BỎ địa bàn đã ngừng dùng: một hồ sơ đã
 * lập ở đó vẫn phải tra ra được tên của nó, và đây là chỗ duy nhất đưa nó vào dùng lại được.
 */
function UnitsTable({
  units,
  canWrite,
  actions,
  sending,
  retiring,
  rowError,
}: {
  units: readonly identity_thonToDanPhoRa[];
  canWrite: boolean;
  actions: ResidentialUnitActions;
  sending: boolean;
  retiring: identity_thonToDanPhoRa | null;
  rowError: { id: string; message: string } | null;
}) {
  const columns = canWrite ? 8 : 7;
  return (
    <>
      {canWrite && retiring !== null && <RetireDialog unit={retiring} sending={sending} actions={actions} />}
      <ConfigTable label="Danh sách thôn / tổ dân phố" caption="Danh sách thôn và tổ dân phố của đơn vị">
        <thead>
          <tr>
            <th scope="col">Tên</th>
            <th scope="col">Mã</th>
            <th scope="col">{FIELD_TYPE}</th>
            <th scope="col">{FIELD_HEAD}</th>
            <th scope="col">{FIELD_HOUSEHOLDS}</th>
            <th scope="col">{FIELD_POPULATION}</th>
            <th scope="col">Trạng thái</th>
            {/* The prototype's head is empty; the word stays for a screen reader only. */}
            {canWrite && (
              <th scope="col">
                <span className="an-thi-giac">Thao tác</span>
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {units.length === 0 && <EmptyRow colSpan={columns}>{THON_RONG}</EmptyRow>}
          {units.map((t) => {
            const loai = loaiDonVi(t.type_code, t.type_label);
            return (
              <UnitRows key={t.id}>
                <tr>
                  <td className="text-navy font-medium">{t.name}</td>
                  <td>
                    <code className="text-[11.5px]">{t.code}</code>
                  </td>
                  {/* Unclassified is "—" (spec 04). A code whose label left the catalogue still says so:
                      that is drift only an administrator can fix, not an empty cell (`nhan-thon.ts`). */}
                  <td>{loai.loai === "chuaPhanLoai" ? EMPTY_CELL : nhanLoaiDonVi(loai)}</td>
                  <td>{t.head_staff_code === "" ? EMPTY_CELL : t.head_staff_name}</td>
                  <td>{countCell(t.household_count)}</td>
                  <td>{countCell(t.population_count)}</td>
                  <td>
                    <StatusBadge active={t.active} />
                  </td>
                  {canWrite && (
                    <td>
                      {/* The prototype's Pencil + Trash2. Trash2's place is "Ngừng dùng" / "Dùng lại": a
                          hamlet is never deleted (ADR 0059 §2). The words are the title and the name. */}
                      <RowActions>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          id={editButtonId(t.id)}
                          title={EDIT_BUTTON}
                          aria-label={editButtonLabel(t.name)}
                          onClick={() => actions.edit(t)}
                          disabled={sending}
                        >
                          <Pencil aria-hidden="true" focusable="false" className="size-3.5" />
                        </Button>
                        {t.active ? (
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            className="text-danger"
                            title={RETIRE_BUTTON}
                            aria-label={retireButtonLabel(t.name)}
                            onClick={() => actions.askRetire(t)}
                            disabled={sending}
                          >
                            <PowerOff aria-hidden="true" focusable="false" className="size-3.5" />
                          </Button>
                        ) : (
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            title={REACTIVATE_BUTTON}
                            aria-label={reactivateButtonLabel(t.name)}
                            onClick={() => actions.reactivate(t)}
                            disabled={sending}
                          >
                            <RotateCcw aria-hidden="true" focusable="false" className="size-3.5" />
                          </Button>
                        )}
                      </RowActions>
                    </td>
                  )}
                </tr>
                {rowError !== null && rowError.id === t.id && (
                  <tr>
                    <td colSpan={columns} className="whitespace-normal">
                      <InlineError>{rowError.message}</InlineError>
                    </td>
                  </tr>
                )}
              </UnitRows>
            );
          })}
        </tbody>
      </ConfigTable>
    </>
  );
}

/**
 * "Ngừng dùng" asks once (ADR 0059, ADR 0068 §15), in a dialog over the list. The question is the
 * visible title; the dialog's own header only names it for assistive technology.
 */
function RetireDialog({
  unit,
  sending,
  actions,
}: {
  unit: identity_thonToDanPhoRa;
  sending: boolean;
  actions: ResidentialUnitActions;
}) {
  const question = `${RETIRE_BUTTON} ${unit.name}?`;
  return (
    <ConfigDialog
      title={question}
      hideHeader
      onDismiss={() => {
        if (!sending) actions.cancelRetire();
      }}
    >
      <ConfirmDialog
        icon={PowerOff}
        title={question}
        className="m-0 p-0 shadow-none"
        actions={
          <>
            <Button type="button" variant="primary" onClick={actions.confirmRetire} disabled={sending} aria-busy={sending}>
              <BusyLabel busy={sending} label={CONFIRM_RETIRE_BUTTON} busyText={BUSY_SAVING} />
            </Button>
            <Button type="button" variant="outline" onClick={actions.cancelRetire} disabled={sending}>
              {NUT_HUY}
            </Button>
          </>
        }
      >
        <p className="m-0">{retireConfirmSentence(unit.name)}</p>
      </ConfirmDialog>
    </ConfigDialog>
  );
}

/** A keyed fragment for one unit's row and the row under it (error). */
function UnitRows({ children }: { children: ReactNode }) {
  return <>{children}</>;
}

/**
 * The add / edit DIALOG (bug sheet row 32: "Đổi UI thêm/sửa thành dạng Popup đồng nhất với các chức năng
 * khác") — the same box as the org chart's `BieuMauBoPhan`: `ModalDialog` + header, a scrolling field
 * column, shadcn's muted footer with `Huỷ` · `Thêm`/`Lưu`. PURE PRESENTATION — every value in through
 * `draft`, every change out through `setDraft`; bodies are built in `residential-unit-form.ts`. EXPORTED
 * so the server's refusal is rendered in a test.
 *
 * THE FIELDS AND RULES ARE THE GREY ROW'S, UNCHANGED: Tên · Loại · Số hộ · Nhân khẩu, then (ADR 0079
 * decision 3) Trưởng thôn / Tổ trưởng and Thứ tự. Only the frame moved.
 *
 * NO CODE BOX: on create the server derives the code from the name (`residential_unit_write.go:32`);
 * on edit it refuses one with 400 `code_not_editable` (rule 7, invariant 3). The code is in the table.
 *
 * Opening focus goes to `Tên` (`initialFocusId`); closing returns it to the button that opened the box
 * (`ModalDialog`). Esc and ✕ ask `onCancel`, refused while a send is in flight — a send in flight must
 * not lose its form from view.
 */
export function ResidentialUnitForm({
  open,
  draft,
  setDraft,
  typeChoices,
  headChoices,
  typesError,
  directoryError,
  localError,
  serverError,
  sending,
  onSubmit,
  onCancel,
}: {
  open: ResidentialUnitFormOpen;
  draft: ResidentialUnitDraft;
  setDraft: (d: ResidentialUnitDraft) => void;
  typeChoices: readonly PickerOption[];
  headChoices: readonly PickerOption[];
  typesError: string;
  directoryError: string;
  localError: string;
  serverError: string;
  sending: boolean;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const titleId = useId();
  const editing = open.kind === "edit";
  const title = editing ? editTitle(open.unit.name) : CREATE_TITLE;
  return (
    <ModalDialog
      titleId={titleId}
      onDismiss={() => {
        if (!sending) onCancel();
      }}
      closeDisabled={sending}
      initialFocusId={NAME_INPUT_ID}
      className="sm:max-w-lg"
    >
      <ModalDialogHeader titleId={titleId} title={title} />
      <form
        className="m-0 flex min-h-0 min-w-0 flex-col gap-4"
        aria-label={title}
        onSubmit={(e) => {
          e.preventDefault();
          onSubmit();
        }}
      >
        <div className="min-h-0 space-y-4 overflow-y-auto">
          <ConfigField label={FIELD_NAME} htmlFor={NAME_INPUT_ID}>
            <input
              id={NAME_INPUT_ID}
              name="name"
              className={formInputCls}
              placeholder={NAME_PLACEHOLDER}
              value={draft.name}
              onChange={(e) => setDraft({ ...draft, name: e.target.value })}
              aria-invalid={localError !== ""}
            />
          </ConfigField>

          <ConfigField label={FIELD_TYPE} htmlFor="o-loai-thon">
            <select
              id="o-loai-thon"
              name="type"
              className={formSelectCls}
              value={draft.typeCode}
              onChange={(e) => setDraft({ ...draft, typeCode: e.target.value })}
            >
              <option value="">{TYPE_NONE}</option>
              {typeChoices.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </ConfigField>

          {/* `type="number" min=0` (spec 04): a blank box stays blank ("chưa nhập"), never 0; whatever
              is not a whole number ≥ 0 is sent as null (`residential-unit-form.ts`). Two columns from
              `sm`, one at 320px. */}
          <div className="grid gap-4 sm:grid-cols-2">
            <ConfigField label={FIELD_HOUSEHOLDS} htmlFor="o-so-ho">
              <input
                id="o-so-ho"
                name="households"
                type="number"
                min={0}
                step={1}
                className={formInputCls}
                value={draft.households}
                onChange={(e) => setDraft({ ...draft, households: e.target.value })}
              />
            </ConfigField>

            <ConfigField label={FIELD_POPULATION} htmlFor="o-nhan-khau">
              <input
                id="o-nhan-khau"
                name="population"
                type="number"
                min={0}
                step={1}
                className={formInputCls}
                value={draft.population}
                onChange={(e) => setDraft({ ...draft, population: e.target.value })}
              />
            </ConfigField>
          </div>

          <ConfigField label={FIELD_HEAD} htmlFor="o-truong-thon">
            <select
              id="o-truong-thon"
              name="head"
              className={formSelectCls}
              value={draft.headStaffCode}
              onChange={(e) => setDraft({ ...draft, headStaffCode: e.target.value })}
            >
              <option value="">{HEAD_NONE}</option>
              {headChoices.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </ConfigField>

          <ConfigField label={FIELD_ORDER} htmlFor="o-thu-tu-thon">
            <input
              id="o-thu-tu-thon"
              name="order"
              type="number"
              step={1}
              className={formInputCls}
              value={draft.order}
              onChange={(e) => setDraft({ ...draft, order: e.target.value })}
            />
          </ConfigField>

          {/* The pickers' own read failures, then TWO error regions: the local one (nothing was sent) and
              the server's refusal, after the spec's sentence. Merged, one overwrites the other. */}
          {typesError !== "" && <InlineError>{typesError}</InlineError>}
          {directoryError !== "" && <InlineError>{directoryError}</InlineError>}
          {localError !== "" && <InlineError>{localError}</InlineError>}
          {serverError !== "" && <InlineError>{`${SAVE_FAILED} ${serverError}`}</InlineError>}
        </div>

        <div className={DIALOG_FOOTER_CLASS}>
          <Button type="button" variant="outline" onClick={onCancel} disabled={sending}>
            {NUT_HUY}
          </Button>
          <Button type="submit" variant="primary" disabled={sending} aria-busy={sending}>
            <BusyLabel busy={sending} label={editing ? NUT_LUU : "Thêm"} busyText={BUSY_SAVING} />
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}
