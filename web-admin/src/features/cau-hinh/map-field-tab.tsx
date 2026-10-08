"use client";

import { Layers, Pencil, Plus, Trash2, X } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import type { KeyboardEvent, ReactNode } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { BUSY_DELETING, BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";

import { usePhien } from "@/features/phien/phien-hien-tai";
import { layLoaiTaiNguyenBanDo } from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import { createMapField, deleteMapField, listMapFields, updateMapField } from "@/lib/api/map-field-schemas";
import type { comms_loaiTaiNguyenRa, comms_mapFieldSchemaOut } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import {
  ConfigField,
  ConfigFormRow,
  ConfigLoading,
  ConfigTable,
  EmptyRow,
  RowActions,
  SMALL_BUTTON_CLASS,
  StatusBadge,
  formInputCls,
  formSelectCls,
  selectCls,
} from "./config-ui";
import {
  CHOICE_TYPE,
  VALUE_TYPES,
  addOption,
  createBody,
  editDraft,
  fieldCodeFromLabel,
  newDraft,
  removeOption,
  setOptionLabel,
  setOptionValue,
  updateBody,
  valueTypeLabel,
} from "./map-field-form";
import type { MapFieldDraft } from "./map-field-form";
import { mapFieldTabDecision, mapFieldWriteDecision } from "./quyen-tab";
import { kiemLyDoXoa } from "./tang-danh-muc";

/* ---- wording ---------------------------------------------------------------------------------- */

export const MAP_FIELD_TITLE = "Trường bản đồ";
export const ASSET_TYPE_LABEL = "Nhóm tài nguyên";
// No leading "+": the button draws a lucide `Plus` beside the words (ADR 0068 §2).
export const ADD_FIELD_BUTTON = "Thêm trường";
export const LABEL_PLACEHOLDER = "Ví dụ: Số phòng học kiên cố";

/**
 * Spec 06 §4's footnote MINUS ITS FIRST SENTENCE ("Cột trong tệp Excel nhập vào chỉ được giữ lại khi có
 * trường tương ứng ở đây."): no Excel import of map ASSETS exists — the contract has no
 * `/map-assets/import*` route, and `map-asset-type-import.ts` imports the asset-TYPE catalogue, not
 * assets — so that sentence would describe a behaviour nobody can find.
 */
export const MAP_FIELD_NOTE =
  "Trường đã xoá thì dữ liệu cũ vẫn còn trong hồ sơ, chỉ không hiện ra trên biểu mẫu nữa.";

/** Empty state when the commune has no asset type: fields hang off a type, so types come first. */
export const NO_ASSET_TYPES =
  "Đơn vị chưa có nhóm tài nguyên bản đồ nào, nên chưa khai được trường. Hãy thêm nhóm ở tab Danh " +
  "mục (mục Loại tài nguyên bản đồ) trước, rồi quay lại đây.";

// The prototype's sentence (`AssetFieldTable.tsx:114`).
export const NO_FIELDS = "Nhóm này chưa khai báo trường riêng nào.";

export const FIELD_ADDED = "Đã thêm trường vào biểu mẫu.";
export const FIELD_SAVED = "Đã lưu.";
export const FIELD_DELETED = "Đã xoá trường.";

export const OPTIONS_HINT =
  "Giá trị của lựa chọn đã lưu thì không đổi và không bỏ được — chỉ sửa được nhãn, hoặc thêm lựa chọn mới.";

export function deleteFieldTitle(label: string): string {
  return `Xoá trường ${label}`;
}

/* ---- the tab ---------------------------------------------------------------------------------- */

type Loaded<T> = KetQua<T> | null;

type OpenForm =
  | { readonly kind: "create"; readonly idempotencyKey: string }
  | { readonly kind: "edit"; readonly row: comms_mapFieldSchemaOut }
  | { readonly kind: "delete"; readonly row: comms_mapFieldSchemaOut };

/**
 * "Cấu hình → Trường bản đồ" — spec Cấu hình 06, prototype `AssetFieldTable.tsx`. Visible with
 * `asset.read` (the read key); write controls with `admin.lookup` (the write key). Both are
 * convenience — the server checks each on every request.
 *
 * ONE TYPE AT A TIME, chosen in `Nhóm tài nguyên` — and one call per type, because the contract
 * declares `asset_type_code` required. The groups are the COMMUNE'S catalogue (GET /map-asset-types,
 * ADR 0072), never the spec's eleven hard-coded keys.
 *
 * Kept against the prototype (ADR 0079): delete asks for a REASON (rule 7 — the server requires it);
 * edit also changes Bắt buộc, Thứ tự and the options of a `chon` field (decision 3).
 */
export function MapFieldTab() {
  const phien = usePhien();
  const readDecision = phien === null ? null : mapFieldTabDecision(phien);
  const writeDecision = phien === null ? null : mapFieldWriteDecision(phien);
  const canWrite = writeDecision !== null && writeDecision.hien;

  const [types, setTypes] = useState<Loaded<{ items: readonly comms_loaiTaiNguyenRa[] }>>(null);
  const [chosenType, setChosenType] = useState("");
  const [fields, setFields] = useState<Loaded<{ items: readonly comms_mapFieldSchemaOut[] }>>(null);
  const [reload, setReload] = useState(0);

  const [open, setOpen] = useState<OpenForm | null>(null);
  const [draft, setDraft] = useState<MapFieldDraft>(newDraft(""));
  const [reason, setReason] = useState("");
  const [formError, setFormError] = useState("");
  const [sending, setSending] = useState(false);
  const [rowError, setRowError] = useState("");

  const readable = readDecision !== null && readDecision.hien;

  useEffect(() => {
    if (!readable) return;
    let gone = false;
    layLoaiTaiNguyenBanDo().then((r) => {
      if (!gone) setTypes(r);
    });
    return () => {
      gone = true;
    };
  }, [readable]);

  // The first type is chosen once the list arrives — adjusted during render, not in an effect.
  const typeItems = useMemo(() => (types !== null && types.ok ? types.duLieu.items : []), [types]);
  if (chosenType === "" && typeItems.length > 0) setChosenType(typeItems[0]!.code);

  useEffect(() => {
    if (chosenType === "") return;
    let gone = false;
    listMapFields(chosenType).then((r) => {
      if (!gone) setFields(r);
    });
    return () => {
      gone = true;
    };
  }, [chosenType, reload]);

  if (phien === null) return <p role="status">Đang kiểm tra quyền truy cập…</p>;
  if (readDecision !== null && !readDecision.hien) {
    return readDecision.vi === "khong-doc-duoc" ? (
      <InlineError>{readDecision.thongBao}</InlineError>
    ) : (
      <p className="text-ink-muted m-0 text-[12.5px]">
        Tài khoản của bạn không có quyền xem bản đồ tài nguyên, nên tab này không hiển thị.
      </p>
    );
  }

  function start(form: OpenForm, d: MapFieldDraft) {
    setOpen(form);
    setDraft(d);
    setReason("");
    setFormError("");
    setRowError("");
  }

  function close() {
    setOpen(null);
    setFormError("");
  }

  /** A done write: close the form, toast the sentence when there is one, READ AGAIN. */
  function finish(sentence: string | null) {
    setOpen(null);
    if (sentence !== null) toast.success(sentence);
    setReload((n) => n + 1);
  }

  async function submit() {
    if (open === null || sending) return;
    setFormError("");
    if (open.kind === "delete") {
      const checked = kiemLyDoXoa(reason);
      if (!checked.ok) {
        setFormError(checked.loi);
        return;
      }
      setSending(true);
      const r = await deleteMapField(open.row.id, checked.giaTri);
      setSending(false);
      if (!r.ok) return setFormError(r.thongBao);
      return finish(FIELD_DELETED);
    }
    if (open.kind === "create") {
      const b = createBody(draft);
      if (b.kind === "error") return setFormError(b.message);
      if (b.kind !== "send") return;
      setSending(true);
      // The key minted when the row OPENED: a retry after an error is the same add, not a second one.
      // A duplicate code comes back as the server's own sentence: `createMapField` does not ask for the
      // refusal's `code`, so `field_code_taken` cannot be told apart here (`lib/api` is out of reach).
      const r = await createMapField(b.body, open.idempotencyKey);
      setSending(false);
      if (!r.ok) return setFormError(r.thongBao);
      return finish(FIELD_ADDED);
    }
    const b = updateBody(open.row, draft);
    if (b.kind === "error") return setFormError(b.message);
    // Nothing changed: Enter on an untouched label simply leaves edit mode — there is nothing to save.
    if (b.kind === "unchanged") return close();
    setSending(true);
    const r = await updateMapField(open.row.id, b.body);
    setSending(false);
    if (!r.ok) return setFormError(r.thongBao);
    return finish(FIELD_SAVED);
  }

  async function toggle(row: comms_mapFieldSchemaOut) {
    setRowError("");
    const r = await updateMapField(row.id, { is_active: !row.is_active });
    if (!r.ok) return setRowError(r.thongBao);
    // No toast: the switch changing IS the confirmation (prototype `AssetFieldTable.tsx:236-238` toasts
    // only the error; ADR 0079 lô 6 #3). A refusal still shows, under the table.
    finish(null);
  }

  return (
    // Prototype `AssetFieldTable.tsx:59-131`: no card, no visible title — the group picker and "Thêm
    // trường" on one row, the add row, the table, the footnote UNDER the table.
    <section className="min-w-0 space-y-4" aria-labelledby="tieu-de-truong-ban-do">
      <h2 id="tieu-de-truong-ban-do" className="an-thi-giac">
        {MAP_FIELD_TITLE}
      </h2>

      {types === null && <ConfigLoading label="Đang tải nhóm tài nguyên…" />}
      {types !== null && !types.ok && (
        <ErrorState role="alert" title="Chưa tải được nhóm tài nguyên" message={types.thongBao} />
      )}
      {types !== null && types.ok && typeItems.length === 0 && <EmptyState icon={Layers} title={NO_ASSET_TYPES} />}

      {typeItems.length > 0 && (
        <>
          <MapFieldToolbar
            types={typeItems}
            chosenType={chosenType}
            canWrite={canWrite}
            onChoose={(code) => {
              setChosenType(code);
              setFields(null);
              setOpen(null);
              setRowError("");
            }}
            onAdd={() =>
              // The add button toggles its row (spec 02 "Bấm để bật/tắt hàng form").
              open !== null && open.kind === "create"
                ? close()
                : start({ kind: "create", idempotencyKey: crypto.randomUUID() }, newDraft(chosenType))
            }
          />

          {rowError !== "" && <InlineError>{rowError}</InlineError>}

          {open !== null && open.kind === "create" && (
            <MapFieldAddRow
              draft={draft}
              setDraft={setDraft}
              error={formError}
              sending={sending}
              onSubmit={() => void submit()}
              onCancel={close}
            />
          )}

          {fields === null && <ConfigLoading label="Đang tải các trường…" />}
          {fields !== null && !fields.ok && (
            <ErrorState role="alert" title="Chưa tải được các trường" message={fields.thongBao} />
          )}
          {fields !== null && fields.ok && (
            <MapFieldTable
              rows={fields.duLieu.items}
              canWrite={canWrite}
              editing={
                open !== null && open.kind === "edit"
                  ? {
                      rowId: open.row.id,
                      draft,
                      setDraft,
                      error: formError,
                      sending,
                      onSave: () => void submit(),
                      onCancel: close,
                    }
                  : null
              }
              deleting={
                open !== null && open.kind === "delete"
                  ? {
                      rowId: open.row.id,
                      form: (
                        <MapFieldDeleteForm
                          label={open.row.label}
                          reason={reason}
                          setReason={setReason}
                          error={formError}
                          sending={sending}
                          onSubmit={() => void submit()}
                          onCancel={close}
                        />
                      ),
                    }
                  : null
              }
              onEdit={(row) => start({ kind: "edit", row }, editDraft(row))}
              onToggle={(row) => void toggle(row)}
              onDelete={(row) => start({ kind: "delete", row }, editDraft(row))}
            />
          )}
        </>
      )}

      <p className="text-ink-muted text-[11.5px]">{MAP_FIELD_NOTE}</p>
    </section>
  );
}

/* ---- presentational parts, exported for the tests ------------------------------------------- */

/** A sentence the server wrote (or the screen's one local check), in place — spec's error type. */
function InlineError({ children }: { children: ReactNode }) {
  return (
    <p role="alert" className="text-danger m-0 text-[12px] font-medium">
      {children}
    </p>
  );
}

/**
 * An option's words. Inactive groups stay listed (fields of a switched-off group are still read and
 * edited here) under their plain label, as the prototype shows every group. The "(đang tắt)" suffix
 * appears ONLY when another group carries the same label: labels are not unique in the catalogue (the
 * table is UNIQUE on `(tenant_id, ma)` only, `service-comms/internal/store/loai_tai_nguyen_ban_do.go`),
 * and two identical lines in a picker are two lines nobody can tell apart.
 */
export function assetTypeOptionLabel(t: comms_loaiTaiNguyenRa, all: readonly comms_loaiTaiNguyenRa[]): string {
  const shared = all.some((o) => o.id !== t.id && o.label === t.label);
  return shared && !t.active ? `${t.label} (đang tắt)` : t.label;
}

/** The group picker and "Thêm trường" (prototype `AssetFieldTable.tsx:60-88`). */
export function MapFieldToolbar({
  types,
  chosenType,
  canWrite,
  onChoose,
  onAdd,
}: {
  types: readonly comms_loaiTaiNguyenRa[];
  chosenType: string;
  canWrite: boolean;
  onChoose: (code: string) => void;
  onAdd: () => void;
}) {
  return (
    <div className="flex flex-wrap items-end gap-3">
      <div className="block w-72 max-w-full">
        <label
          htmlFor="o-nhom-tai-nguyen"
          className="text-foreground m-0 text-[11.5px] leading-none font-medium select-none"
        >
          {ASSET_TYPE_LABEL}
        </label>
        <select
          id="o-nhom-tai-nguyen"
          className={cn(selectCls, "mt-1 w-full min-w-0 pr-8")}
          value={chosenType}
          onChange={(e) => onChoose(e.target.value)}
        >
          {types.map((t) => (
            <option key={t.id} value={t.code}>
              {assetTypeOptionLabel(t, types)}
            </option>
          ))}
        </select>
      </div>

      {canWrite && (
        <Button
          type="button"
          variant="primary"
          size="sm"
          className={cn(SMALL_BUTTON_CLASS, "ml-auto")}
          icon={<Plus aria-hidden="true" focusable="false" className="size-4" />}
          onClick={onAdd}
        >
          {ADD_FIELD_BUTTON}
        </Button>
      )}
    </div>
  );
}

/**
 * The grey add row (prototype `AssetFieldTable.tsx:319-372`): Nhãn hiển thị with the generated code
 * under it, Kiểu dữ liệu, Bắt buộc, Thêm / Huỷ. The code is not typed: it is the label's slug, shown
 * so staff know what the records will store. "Chọn trong danh sách" opens the options editor under the
 * row — the server refuses a `chon` field without options.
 */
export function MapFieldAddRow({
  draft,
  setDraft,
  error,
  sending,
  onSubmit,
  onCancel,
}: {
  draft: MapFieldDraft;
  setDraft: (d: MapFieldDraft) => void;
  error: string;
  sending: boolean;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const code = fieldCodeFromLabel(draft.label);
  return (
    <ConfigFormRow
      columns="sm:grid-cols-[1fr_14rem_9rem_auto]"
      aria-label="Thêm trường"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
      onKeyDown={(e) => {
        if (e.key === "Escape" && !sending) onCancel();
      }}
    >
      <ConfigField label="Nhãn hiển thị" htmlFor="o-nhan-truong">
        <input
          id="o-nhan-truong"
          name="label"
          autoFocus
          className={formInputCls}
          placeholder={LABEL_PLACEHOLDER}
          value={draft.label}
          onChange={(e) => setDraft({ ...draft, label: e.target.value })}
        />
        {draft.label.trim() !== "" && (
          <p className="text-ink-muted m-0 mt-1 text-[11px]">
            Mã trường: <code>{code === "" ? "—" : code}</code>
          </p>
        )}
      </ConfigField>
      <ConfigField label="Kiểu dữ liệu" htmlFor="o-kieu-truong">
        <select
          id="o-kieu-truong"
          name="value_type"
          className={formSelectCls}
          value={draft.valueType}
          onChange={(e) => setDraft({ ...draft, valueType: e.target.value })}
        >
          {VALUE_TYPES.map((t) => (
            <option key={t.value} value={t.value}>
              {t.label}
            </option>
          ))}
        </select>
      </ConfigField>
      <ConfigField label="Bắt buộc" htmlFor="o-bat-buoc-truong">
        <select
          id="o-bat-buoc-truong"
          name="is_required"
          className={formSelectCls}
          value={draft.isRequired ? "co" : "khong"}
          onChange={(e) => setDraft({ ...draft, isRequired: e.target.value === "co" })}
        >
          <option value="khong">Không</option>
          <option value="co">Có</option>
        </select>
      </ConfigField>
      <div className="flex gap-2">
        <Button type="submit" variant="primary" disabled={sending} aria-busy={sending}>
          <BusyLabel busy={sending} label="Thêm" busyText={BUSY_SAVING} />
        </Button>
        <Button type="button" variant="outline" onClick={onCancel} disabled={sending}>
          Huỷ
        </Button>
      </div>

      {draft.valueType === CHOICE_TYPE && (
        <div className="col-span-full">
          <OptionsEditor draft={draft} setDraft={setDraft} />
        </div>
      )}
      {error !== "" && (
        <div className="col-span-full">
          <InlineError>{error}</InlineError>
        </div>
      )}
    </ConfigFormRow>
  );
}

/** The row being edited in place — its draft lives with the tab, the inputs live in the row. */
export type RowEditing = {
  readonly rowId: string;
  readonly draft: MapFieldDraft;
  readonly setDraft: (d: MapFieldDraft) => void;
  readonly error: string;
  readonly sending: boolean;
  readonly onSave: () => void;
  readonly onCancel: () => void;
};

/** The row whose delete (reason step) is open, and that form. */
export type RowDeleting = { readonly rowId: string; readonly form: ReactNode };

/**
 * The field list (prototype `AssetFieldTable.tsx:94-124`) in the shared hairline frame, which scrolls
 * sideways at 320px. An edited row turns its label, Bắt buộc and Thứ tự cells into inputs; a `chon`
 * field's options, an error and the delete reason open in a full-width row right under it.
 */
export function MapFieldTable({
  rows,
  canWrite,
  editing,
  deleting,
  onEdit,
  onToggle,
  onDelete,
}: {
  rows: readonly comms_mapFieldSchemaOut[];
  canWrite: boolean;
  editing: RowEditing | null;
  deleting: RowDeleting | null;
  onEdit: (row: comms_mapFieldSchemaOut) => void;
  onToggle: (row: comms_mapFieldSchemaOut) => void;
  onDelete: (row: comms_mapFieldSchemaOut) => void;
}) {
  const cols = canWrite ? 7 : 6;
  return (
    <ConfigTable label="Các trường riêng của nhóm" caption="Các trường riêng khai cho nhóm tài nguyên đang chọn">
      <thead>
        <tr>
          <th scope="col">Nhãn hiển thị</th>
          <th scope="col">Mã trường</th>
          <th scope="col">Kiểu dữ liệu</th>
          <th scope="col">Bắt buộc</th>
          <th scope="col">Thứ tự</th>
          <th scope="col">Trạng thái</th>
          {canWrite && (
            <th scope="col">
              <span className="an-thi-giac">Thao tác</span>
            </th>
          )}
        </tr>
      </thead>
      <tbody>
        {rows.length === 0 && <EmptyRow colSpan={cols}>{NO_FIELDS}</EmptyRow>}
        {rows.map((r) => (
          <FieldRow
            key={r.id}
            row={r}
            canWrite={canWrite}
            cols={cols}
            editing={canWrite && editing !== null && editing.rowId === r.id ? editing : null}
            deleteForm={canWrite && deleting !== null && deleting.rowId === r.id ? deleting.form : null}
            onEdit={onEdit}
            onToggle={onToggle}
            onDelete={onDelete}
          />
        ))}
      </tbody>
    </ConfigTable>
  );
}

/** A compact control inside a table cell (prototype's in-place Input `h-8 text-[12.5px]`). */
const cellInputCls = cn(formInputCls, "mt-0 h-8");
const cellSelectCls = cn(selectCls, "h-8 pr-8");

function FieldRow({
  row,
  canWrite,
  cols,
  editing,
  deleteForm,
  onEdit,
  onToggle,
  onDelete,
}: {
  row: comms_mapFieldSchemaOut;
  canWrite: boolean;
  cols: number;
  editing: RowEditing | null;
  deleteForm: ReactNode;
  onEdit: (row: comms_mapFieldSchemaOut) => void;
  onToggle: (row: comms_mapFieldSchemaOut) => void;
  onDelete: (row: comms_mapFieldSchemaOut) => void;
}) {
  const e = editing;
  // Enter saves, Esc cancels, from any input of the row (prototype `AssetFieldTable.tsx:172-178`).
  const keys = (ev: KeyboardEvent) => {
    if (e === null || e.sending) return;
    if (ev.key === "Enter") {
      ev.preventDefault();
      e.onSave();
    } else if (ev.key === "Escape") e.onCancel();
  };
  const subRow = e !== null && (row.value_type === CHOICE_TYPE || e.error !== "");

  return (
    <>
      <tr>
        <td className="text-navy font-medium">
          {e !== null ? (
            <>
              <label htmlFor={`o-sua-nhan-${row.id}`} className="an-thi-giac">
                Nhãn hiển thị
              </label>
              <input
                id={`o-sua-nhan-${row.id}`}
                name="label"
                autoFocus
                className={cn(cellInputCls, "min-w-48")}
                value={e.draft.label}
                onChange={(ev) => e.setDraft({ ...e.draft, label: ev.target.value })}
                onKeyDown={keys}
              />
            </>
          ) : (
            row.label
          )}
        </td>
        <td>
          <code className="text-[11.5px]">{row.field_code}</code>
        </td>
        <td>{valueTypeLabel(row.value_type)}</td>
        <td>
          {e !== null ? (
            <>
              <label htmlFor={`o-sua-bat-buoc-${row.id}`} className="an-thi-giac">
                Bắt buộc
              </label>
              <select
                id={`o-sua-bat-buoc-${row.id}`}
                name="is_required"
                className={cellSelectCls}
                value={e.draft.isRequired ? "co" : "khong"}
                onChange={(ev) => e.setDraft({ ...e.draft, isRequired: ev.target.value === "co" })}
                onKeyDown={keys}
              >
                <option value="khong">Không</option>
                <option value="co">Có</option>
              </select>
            </>
          ) : row.is_required ? (
            "Có"
          ) : (
            "—"
          )}
        </td>
        <td>
          {e !== null ? (
            <>
              <label htmlFor={`o-sua-thu-tu-${row.id}`} className="an-thi-giac">
                Thứ tự
              </label>
              <input
                id={`o-sua-thu-tu-${row.id}`}
                name="sort_order"
                inputMode="numeric"
                className={cn(cellInputCls, "w-16")}
                value={e.draft.sortOrder}
                onChange={(ev) => e.setDraft({ ...e.draft, sortOrder: ev.target.value })}
                onKeyDown={keys}
              />
            </>
          ) : (
            row.sort_order
          )}
        </td>
        <td>
          <StatusBadge active={row.is_active} />
        </td>
        {canWrite && (
          <td>
            <RowActions>
              {e !== null ? (
                <>
                  <Button type="button" variant="primary" size="sm" disabled={e.sending} aria-busy={e.sending} onClick={e.onSave}>
                    <BusyLabel busy={e.sending} label="Lưu" busyText={BUSY_SAVING} />
                  </Button>
                  <Button type="button" variant="outline" size="sm" disabled={e.sending} onClick={e.onCancel}>
                    Huỷ
                  </Button>
                </>
              ) : (
                <>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    title="Sửa nhãn"
                    aria-label={`Sửa trường ${row.label}`}
                    onClick={() => onEdit(row)}
                  >
                    <Pencil aria-hidden="true" focusable="false" className="size-3.5" />
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    aria-label={`${row.is_active ? "Tắt" : "Bật"} trường ${row.label}`}
                    onClick={() => onToggle(row)}
                  >
                    {row.is_active ? "Tắt" : "Bật"}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="text-danger"
                    title="Xoá trường"
                    aria-label={deleteFieldTitle(row.label)}
                    onClick={() => onDelete(row)}
                  >
                    <Trash2 aria-hidden="true" focusable="false" className="size-3.5" />
                  </Button>
                </>
              )}
            </RowActions>
          </td>
        )}
      </tr>
      {subRow && e !== null && (
        <tr>
          <td colSpan={cols} className="whitespace-normal">
            <div className="space-y-2">
              {row.value_type === CHOICE_TYPE && <OptionsEditor draft={e.draft} setDraft={e.setDraft} />}
              {e.error !== "" && <InlineError>{e.error}</InlineError>}
            </div>
          </td>
        </tr>
      )}
      {deleteForm !== null && (
        <tr>
          <td colSpan={cols} className="whitespace-normal">
            {deleteForm}
          </td>
        </tr>
      )}
    </>
  );
}

/**
 * The options of a `chon` field. Each existing option's VALUE is read-only (locked: records store it);
 * its label can change, and new options can be appended and removed again before saving.
 */
function OptionsEditor({ draft, setDraft }: { draft: MapFieldDraft; setDraft: (d: MapFieldDraft) => void }) {
  const hasLocked = draft.options.some((o) => o.locked);
  return (
    <fieldset className="m-0 min-w-0 space-y-2 border-0 p-0">
      <legend className="text-foreground p-0 text-[11.5px] font-medium">Các lựa chọn</legend>
      {hasLocked && <p className="text-ink-muted m-0 text-[11px]">{OPTIONS_HINT}</p>}
      {draft.options.map((o, i) => (
        <div className="grid grid-cols-1 items-center gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]" key={i}>
          <label htmlFor={`o-gia-tri-${i}`} className="an-thi-giac">{`Giá trị lựa chọn ${i + 1}`}</label>
          <input
            id={`o-gia-tri-${i}`}
            name={`option_value_${i}`}
            className={cellInputCls}
            value={o.value}
            readOnly={o.locked}
            aria-readonly={o.locked}
            placeholder="Giá trị lưu, ví dụ: tnhh"
            onChange={(e) => setDraft(setOptionValue(draft, i, e.target.value))}
          />
          <label htmlFor={`o-nhan-lua-chon-${i}`} className="an-thi-giac">{`Nhãn lựa chọn ${i + 1}`}</label>
          <input
            id={`o-nhan-lua-chon-${i}`}
            name={`option_label_${i}`}
            className={cellInputCls}
            value={o.label}
            placeholder="Nhãn hiển thị"
            onChange={(e) => setDraft(setOptionLabel(draft, i, e.target.value))}
          />
          {o.locked ? (
            <span aria-hidden="true" />
          ) : (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className={SMALL_BUTTON_CLASS}
              aria-label={`Bỏ lựa chọn ${i + 1}`}
              icon={<X aria-hidden="true" focusable="false" className="size-3.5" />}
              onClick={() => setDraft(removeOption(draft, i))}
            >
              Bỏ
            </Button>
          )}
        </div>
      ))}
      <Button
        type="button"
        variant="outline"
        size="sm"
        className={SMALL_BUTTON_CLASS}
        icon={<Plus aria-hidden="true" focusable="false" className="size-3.5" />}
        onClick={() => setDraft(addOption(draft))}
      >
        Thêm lựa chọn
      </Button>
    </fieldset>
  );
}

/**
 * Delete's REASON STEP (rule 7: the server soft-deletes and requires `reason`; ADR 0079 "Giữ bất kể
 * spec"), compact: one grey row under the field's row — reason, Xoá / Huỷ. No explanatory sentence:
 * the prototype has none (owner, "Bỏ hết, đúng prototype", ADR 0079 lô 3 Q7).
 */
export function MapFieldDeleteForm({
  label,
  reason,
  setReason,
  error,
  sending,
  onSubmit,
  onCancel,
}: {
  label: string;
  reason: string;
  setReason: (s: string) => void;
  error: string;
  sending: boolean;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  return (
    <ConfigFormRow
      columns="sm:grid-cols-[minmax(0,1fr)_auto]"
      aria-label={deleteFieldTitle(label)}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
      onKeyDown={(e) => {
        if (e.key === "Escape" && !sending) onCancel();
      }}
    >
      <ConfigField label={`Lý do xoá trường ${label}`} htmlFor="o-ly-do-xoa-truong">
        {/* `required` is the browser's reminder, not the check: `kiemLyDoXoa` runs before sending. */}
        <input
          id="o-ly-do-xoa-truong"
          name="reason"
          required
          autoFocus
          className={formInputCls}
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          aria-invalid={error !== ""}
        />
      </ConfigField>
      <div className="flex gap-2">
        <Button type="submit" variant="danger" disabled={sending} aria-busy={sending}>
          <BusyLabel busy={sending} label="Xoá" busyText={BUSY_DELETING} />
        </Button>
        <Button type="button" variant="outline" onClick={onCancel} disabled={sending}>
          Huỷ
        </Button>
      </div>
      {error !== "" && (
        <div className="col-span-full">
          <InlineError>{error}</InlineError>
        </div>
      )}
    </ConfigFormRow>
  );
}
