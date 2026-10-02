"use client";

import { Layers, MapPinned, Pencil, Plus, Power, PowerOff, Trash2, X } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { CardHeader, CardTitle } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Field } from "@/components/ui/field";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";
import { BUSY_DELETING, BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";

import { usePhien } from "@/features/phien/phien-hien-tai";
import { layLoaiTaiNguyenBanDo } from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import { createMapField, deleteMapField, listMapFields, updateMapField } from "@/lib/api/map-field-schemas";
import type { comms_loaiTaiNguyenRa, comms_mapFieldSchemaOut } from "@/lib/api/schema.gen";

import {
  CHOICE_TYPE,
  NO_CHANGE,
  VALUE_TYPES,
  addOption,
  createBody,
  editDraft,
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

/** §6's footnote, minus its first sentence: no Excel import of assets exists to "keep columns". */
export const MAP_FIELD_NOTE =
  "Trường đã xoá thì dữ liệu cũ vẫn còn trong hồ sơ, chỉ không hiện lên trên biểu mẫu nữa. Trường " +
  "đang tắt vẫn nằm trong danh sách và bật lại được.";

/** Said once, plainly: these fields describe a form whose records do not exist yet. */
export const NO_REGISTER_NOTE =
  "Bản đồ kinh tế số hiện chưa lưu hồ sơ tài nguyên; các trường khai ở đây sẽ là biểu mẫu của hồ sơ " +
  "ấy khi phân hệ được đưa vào dùng.";

/** Empty state when the commune has no asset type: fields hang off a type, so types come first. */
export const NO_ASSET_TYPES =
  "Đơn vị chưa có nhóm tài nguyên bản đồ nào, nên chưa khai được trường. Hãy thêm nhóm ở tab Danh " +
  "mục (mục Loại tài nguyên bản đồ) trước, rồi quay lại đây.";

export const NO_FIELDS = "Nhóm này chưa có trường tuỳ biến nào.";
export const READ_ONLY_NOTE =
  "Tài khoản của bạn chỉ xem được các trường. Việc thêm, sửa, tắt và xoá trường cần quyền Quản lý danh mục.";

export const FIELD_CODE_HINT =
  "Chữ thường không dấu, số và dấu gạch dưới, bắt đầu bằng chữ — ví dụ: legal_form. Mã trường đã " +
  "lưu thì không sửa được, và mã của trường đã xoá không được dùng lại.";
export const VALUE_TYPE_FIXED_HINT = "Kiểu dữ liệu không đổi được sau khi thêm: dữ liệu đã ghi theo kiểu cũ.";
export const OPTIONS_HINT =
  "Giá trị của lựa chọn đã lưu thì không đổi và không bỏ được — chỉ sửa được nhãn, hoặc thêm lựa chọn mới.";
export const DELETE_FIELD_EXPLANATION =
  "Xoá trường thì trường không còn trên biểu mẫu và không bật lại được. Dữ liệu đã ghi theo trường " +
  "này vẫn được giữ, và mã trường không bao giờ được cấp lại trong nhóm này — muốn tạm ẩn thì bấm Tắt.";

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
 * "Cấu hình → Trường bản đồ" (§6). Visible with `asset.read` (the read key); write controls with
 * `admin.lookup` (the write key). Both are convenience — the server checks each on every request.
 *
 * ONE TYPE AT A TIME, chosen in `Nhóm tài nguyên` as §6 draws it — and one call per type, because
 * the contract declares `asset_type_code` required.
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
  const [done, setDone] = useState("");
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
      <p className="thong-bao-loi" role="alert">
        {readDecision.thongBao}
      </p>
    ) : (
      <p className="trang-thai-rong">
        Tài khoản của bạn không có quyền xem bản đồ tài nguyên, nên tab này không hiển thị.
      </p>
    );
  }

  function start(form: OpenForm, d: MapFieldDraft) {
    setOpen(form);
    setDraft(d);
    setReason("");
    setFormError("");
    setDone("");
    setRowError("");
  }

  function finish(sentence: string) {
    setOpen(null);
    setDone(sentence);
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
      return finish(`Đã xoá trường ${open.row.label}.`);
    }
    if (open.kind === "create") {
      const b = createBody(draft);
      if (b.kind !== "send") return setFormError(b.kind === "error" ? b.message : NO_CHANGE);
      setSending(true);
      // The key minted when the form OPENED: a retry after an error is the same add, not a second one.
      const r = await createMapField(b.body, open.idempotencyKey);
      setSending(false);
      if (!r.ok) return setFormError(r.thongBao);
      return finish(`Đã thêm trường ${r.duLieu.label}.`);
    }
    const b = updateBody(open.row, draft);
    if (b.kind !== "send") return setFormError(b.kind === "error" ? b.message : NO_CHANGE);
    setSending(true);
    const r = await updateMapField(open.row.id, b.body);
    setSending(false);
    if (!r.ok) return setFormError(r.thongBao);
    return finish(`Đã lưu trường ${r.duLieu.label}.`);
  }

  async function toggle(row: comms_mapFieldSchemaOut) {
    setRowError("");
    setDone("");
    const r = await updateMapField(row.id, { is_active: !row.is_active });
    if (!r.ok) return setRowError(r.thongBao);
    finish(r.duLieu.is_active ? `Đã bật trường ${r.duLieu.label}.` : `Đã tắt trường ${r.duLieu.label}.`);
  }

  const formNode =
    open === null ? null : open.kind === "delete" ? (
      <MapFieldDeleteForm
        label={open.row.label}
        reason={reason}
        setReason={setReason}
        error={formError}
        sending={sending}
        onSubmit={() => void submit()}
        onCancel={() => setOpen(null)}
      />
    ) : (
      <MapFieldForm
        mode={open.kind}
        draft={draft}
        setDraft={setDraft}
        error={formError}
        sending={sending}
        onSubmit={() => void submit()}
        onCancel={() => setOpen(null)}
      />
    );

  return (
    <section
      className="tab-danh-muc m-0 min-w-0 overflow-hidden rounded-card border border-line bg-surface shadow-sm"
      aria-labelledby="tieu-de-truong-ban-do"
    >
      <CardHeader className="m-0">
        <div className="min-w-0 flex-1 basis-64">
          <CardTitle as="h2" id="tieu-de-truong-ban-do" className="flex items-center gap-2">
            <MapPinned aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-brand-600" />
            {MAP_FIELD_TITLE}
          </CardTitle>
          <p className="ghi-chu m-0 mt-1 text-[13px] text-ink-500">{MAP_FIELD_NOTE}</p>
        </div>
      </CardHeader>
      <div className="flex min-w-0 flex-col gap-3 p-4 [&>*]:my-0">
      <Notice tone="info" className="ghi-chu">
        {NO_REGISTER_NOTE}
      </Notice>
      {writeDecision !== null && !writeDecision.hien && <Notice tone="neutral">{READ_ONLY_NOTE}</Notice>}

      {types === null && (
        <>
          <p role="status" className="an-thi-giac">
            Đang tải nhóm tài nguyên…
          </p>
          <SkeletonRows rows={3} className="rounded-xl border border-line" />
        </>
      )}
      {types !== null && !types.ok && (
        <ErrorState role="alert" title="Chưa tải được nhóm tài nguyên" message={types.thongBao} />
      )}
      {types !== null && types.ok && typeItems.length === 0 && <EmptyState icon={Layers} title={NO_ASSET_TYPES} />}

      {typeItems.length > 0 && (
        <>
          <div className="flex flex-wrap items-end justify-between gap-3">
          <Field label={ASSET_TYPE_LABEL} htmlFor="o-nhom-tai-nguyen" kind="select" icon={Layers} className="min-w-[220px] flex-[0_1_320px]">
            <select
              id="o-nhom-tai-nguyen"
              value={chosenType}
              onChange={(e) => {
                setChosenType(e.target.value);
                setFields(null);
                setOpen(null);
                setDone("");
              }}
            >
              {typeItems.map((t) => (
                <option key={t.id} value={t.code}>
                  {t.active ? t.label : `${t.label} (đang tắt)`}
                </option>
              ))}
            </select>
          </Field>

          {canWrite && (
            <p className="m-0">
              <Button
                type="button"
                variant="primary"
                icon={<Plus aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                onClick={() => start({ kind: "create", idempotencyKey: crypto.randomUUID() }, newDraft(chosenType))}
              >
                {ADD_FIELD_BUTTON}
              </Button>
            </p>
          )}
          </div>
          {done !== "" && (
            <p role="status" className="text-sm font-medium text-success-600">
              {done}
            </p>
          )}
          {rowError !== "" && (
            <p className="thong-bao-loi" role="alert">
              {rowError}
            </p>
          )}
          {open !== null && open.kind === "create" && formNode}

          {fields === null && (
            <>
              <p role="status" className="an-thi-giac">
                Đang tải các trường…
              </p>
              <SkeletonRows rows={3} className="rounded-xl border border-line" />
            </>
          )}
          {fields !== null && !fields.ok && (
            <ErrorState role="alert" title="Chưa tải được các trường" message={fields.thongBao} />
          )}
          {fields !== null && fields.ok && (
            <MapFieldTable
              rows={fields.duLieu.items}
              canWrite={canWrite}
              openRowId={open !== null && open.kind !== "create" ? open.row.id : null}
              openForm={open !== null && open.kind !== "create" ? formNode : null}
              onEdit={(row) => start({ kind: "edit", row }, editDraft(row))}
              onToggle={(row) => void toggle(row)}
              onDelete={(row) => start({ kind: "delete", row }, editDraft(row))}
            />
          )}
        </>
      )}
      </div>
    </section>
  );
}

/* ---- presentational parts, exported for the tests ------------------------------------------- */

/**
 * The field list. A TABLE, as §6 draws it, inside a horizontal scroll region — at 320px seven
 * columns do not fit, and a table read in rows keeps "which field is required" attached to its row.
 * The open edit/delete form renders in a full-width row right under the row it is about.
 */
export function MapFieldTable({
  rows,
  canWrite,
  openRowId,
  openForm,
  onEdit,
  onToggle,
  onDelete,
}: {
  rows: readonly comms_mapFieldSchemaOut[];
  canWrite: boolean;
  openRowId: string | null;
  openForm: ReactNode;
  onEdit: (row: comms_mapFieldSchemaOut) => void;
  onToggle: (row: comms_mapFieldSchemaOut) => void;
  onDelete: (row: comms_mapFieldSchemaOut) => void;
}) {
  if (rows.length === 0) return <EmptyState icon={MapPinned} title={NO_FIELDS} />;
  const cols = canWrite ? 7 : 6;
  return (
    <TableScroll sticky aria-label="Các trường tuỳ biến">
      <table className={`bang-danh-muc ${DATA_TABLE_CLASS}`}>
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
          {rows.map((r) => (
            <FieldRow
              key={r.id}
              row={r}
              canWrite={canWrite}
              cols={cols}
              form={openRowId === r.id ? openForm : null}
              onEdit={onEdit}
              onToggle={onToggle}
              onDelete={onDelete}
            />
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}

function FieldRow({
  row,
  canWrite,
  cols,
  form,
  onEdit,
  onToggle,
  onDelete,
}: {
  row: comms_mapFieldSchemaOut;
  canWrite: boolean;
  cols: number;
  form: ReactNode;
  onEdit: (row: comms_mapFieldSchemaOut) => void;
  onToggle: (row: comms_mapFieldSchemaOut) => void;
  onDelete: (row: comms_mapFieldSchemaOut) => void;
}) {
  return (
    <>
      <tr>
        <td>{row.label}</td>
        <td className="ma-muc">{row.field_code}</td>
        <td>{valueTypeLabel(row.value_type)}</td>
        <td>{row.is_required ? "Có" : "—"}</td>
        <td>{row.sort_order}</td>
        <td>
          {/* Tone by the CODE (`is_active`); icon + word, never colour alone. */}
          <Badge tone={row.is_active ? "success" : "neutral"}>{row.is_active ? "Đang dùng" : "Đang tắt"}</Badge>
        </td>
        {canWrite && (
          <td>
            <span className="cum-nut flex flex-wrap items-center gap-1.5">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                aria-label={`Sửa trường ${row.label}`}
                icon={<Pencil aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                onClick={() => onEdit(row)}
              >
                Sửa
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                aria-label={`${row.is_active ? "Tắt" : "Bật"} trường ${row.label}`}
                icon={row.is_active ? <PowerOff aria-hidden="true" focusable="false" strokeWidth={1.8} /> : <Power aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                onClick={() => onToggle(row)}
              >
                {row.is_active ? "Tắt" : "Bật"}
              </Button>
              <Button
                type="button"
                variant="danger"
                size="sm"
                aria-label={`Xoá trường ${row.label}`}
                icon={<Trash2 aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                onClick={() => onDelete(row)}
              >
                Xoá
              </Button>
            </span>
          </td>
        )}
      </tr>
      {form !== null && (
        <tr>
          <td colSpan={cols} className="whitespace-normal">
            {form}
          </td>
        </tr>
      )}
    </>
  );
}

/**
 * Create / edit form. On EDIT the code and the value type are text, not inputs (a box that edits
 * them promises what the server refuses), and each existing option's VALUE is a read-only input —
 * only its label and the rows added here are editable.
 */
export function MapFieldForm({
  mode,
  draft,
  setDraft,
  error,
  sending,
  onSubmit,
  onCancel,
}: {
  mode: "create" | "edit";
  draft: MapFieldDraft;
  setDraft: (d: MapFieldDraft) => void;
  error: string;
  sending: boolean;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const title = mode === "create" ? "Thêm trường" : `Sửa trường ${draft.label}`;
  return (
    <form
      className="form-danh-muc grid min-w-0 gap-4 sm:grid-cols-2 [&>*]:m-0 [&>.cum-nut]:col-span-full [&>.thong-bao-loi]:col-span-full [&>h4]:col-span-full [&>.ghi-chu]:col-span-full [&>fieldset]:col-span-full"
      aria-label={title}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
      onKeyDown={(e) => {
        if (e.key === "Escape" && !sending) onCancel();
      }}
    >
      <h4 className="text-[15px] font-semibold text-ink-900">{title}</h4>

      <div className="o-nhap">
        <label htmlFor="o-nhan-truong">Nhãn hiển thị</label>
        <input id="o-nhan-truong" name="label" value={draft.label} onChange={(e) => setDraft({ ...draft, label: e.target.value })} />
      </div>

      {mode === "create" ? (
        <>
          <div className="o-nhap">
            <label htmlFor="o-ma-truong">Mã trường</label>
            <input
              id="o-ma-truong"
              name="field_code"
              value={draft.fieldCode}
              onChange={(e) => setDraft({ ...draft, fieldCode: e.target.value })}
              aria-describedby="giai-thich-ma-truong"
            />
            <p className="ghi-chu" id="giai-thich-ma-truong">
              {FIELD_CODE_HINT}
            </p>
          </div>
          <div className="o-nhap">
            <label htmlFor="o-kieu-truong">Kiểu dữ liệu</label>
            <select
              id="o-kieu-truong"
              name="value_type"
              value={draft.valueType}
              onChange={(e) => setDraft({ ...draft, valueType: e.target.value })}
              aria-describedby="giai-thich-kieu-truong"
            >
              {VALUE_TYPES.map((t) => (
                <option key={t.value} value={t.value}>
                  {t.label}
                </option>
              ))}
            </select>
            <p className="ghi-chu" id="giai-thich-kieu-truong">
              {VALUE_TYPE_FIXED_HINT}
            </p>
          </div>
        </>
      ) : (
        <p className="ghi-chu">
          Mã trường <span className="ma-muc">{draft.fieldCode}</span> · Kiểu dữ liệu {valueTypeLabel(draft.valueType)}.
          Hai thông tin này không sửa được.
        </p>
      )}

      {draft.valueType === CHOICE_TYPE && <OptionsEditor draft={draft} setDraft={setDraft} />}

      <div className="o-nhap self-end">
        <label className="inline-flex min-h-10 cursor-pointer items-center gap-2">
          <input
            type="checkbox"
            name="is_required"
            checked={draft.isRequired}
            onChange={(e) => setDraft({ ...draft, isRequired: e.target.checked })}
          />{" "}
          Bắt buộc nhập
        </label>
      </div>

      <div className="o-nhap">
        <label htmlFor="o-thu-tu-truong">Thứ tự</label>
        <input
          id="o-thu-tu-truong"
          name="sort_order"
          inputMode="numeric"
          value={draft.sortOrder}
          onChange={(e) => setDraft({ ...draft, sortOrder: e.target.value })}
        />
      </div>

      {error !== "" && (
        <p className="thong-bao-loi" role="alert">
          {error}
        </p>
      )}

      <div className="cum-nut flex flex-wrap justify-end gap-2">
        <Button type="submit" variant="primary" disabled={sending} aria-busy={sending}>
          <BusyLabel busy={sending} label="Lưu" busyText={BUSY_SAVING} />
        </Button>
        <Button type="button" variant="secondary" onClick={onCancel} disabled={sending}>
          Huỷ
        </Button>
      </div>
    </form>
  );
}

function OptionsEditor({ draft, setDraft }: { draft: MapFieldDraft; setDraft: (d: MapFieldDraft) => void }) {
  return (
    <fieldset className="o-nhap m-0 flex min-w-0 flex-col gap-2 rounded-xl border border-line p-3 [&>*]:my-0">
      <legend className="px-1 text-xs font-semibold text-ink-700">Các lựa chọn</legend>
      <p className="ghi-chu text-[13px] text-ink-500">{OPTIONS_HINT}</p>
      {draft.options.map((o, i) => (
        <div className="cum-nut grid grid-cols-1 items-center gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]" key={i}>
          <label htmlFor={`o-gia-tri-${i}`} className="an-thi-giac">{`Giá trị lựa chọn ${i + 1}`}</label>
          <input
            id={`o-gia-tri-${i}`}
            name={`option_value_${i}`}
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
            value={o.label}
            placeholder="Nhãn hiển thị"
            onChange={(e) => setDraft(setOptionLabel(draft, i, e.target.value))}
          />
          {!o.locked && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              aria-label={`Bỏ lựa chọn ${i + 1}`}
              icon={<X aria-hidden="true" focusable="false" strokeWidth={1.8} />}
              onClick={() => setDraft(removeOption(draft, i))}
            >
              Bỏ
            </Button>
          )}
        </div>
      ))}
      <Button
        type="button"
        variant="secondary"
        size="sm"
        className="self-start"
        icon={<Plus aria-hidden="true" focusable="false" strokeWidth={1.8} />}
        onClick={() => setDraft(addOption(draft))}
      >
        Thêm lựa chọn
      </Button>
    </fieldset>
  );
}

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
  const title = deleteFieldTitle(label);
  return (
    // The existing confirm form, framed as the shared ConfirmDialog (spec v2 §7).
    <ConfirmDialog
      as="form"
      className="form-danh-muc m-0"
      aria-label={title}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
      onKeyDown={(e) => {
        if (e.key === "Escape" && !sending) onCancel();
      }}
      title={title}
      titleAs="h4"
      tone="danger"
      icon={Trash2}
      actions={
        <>
          <Button type="submit" variant="danger" disabled={sending} aria-busy={sending}>
            <BusyLabel busy={sending} label="Xác nhận xoá" busyText={BUSY_DELETING} />
          </Button>
          <Button type="button" variant="secondary" onClick={onCancel} disabled={sending}>
            Huỷ
          </Button>
        </>
      }
    >
      <p className="ghi-chu m-0">{DELETE_FIELD_EXPLANATION}</p>
      <div className="o-nhap m-0">
        <label htmlFor="o-ly-do-xoa-truong">Lý do xoá</label>
        <textarea id="o-ly-do-xoa-truong" name="reason" required value={reason} onChange={(e) => setReason(e.target.value)} />
      </div>
      {error !== "" && (
        <p className="thong-bao-loi m-0" role="alert">
          {error}
        </p>
      )}
    </ConfirmDialog>
  );
}
