"use client";

import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";

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
export const ADD_FIELD_BUTTON = "+ Thêm trường";

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
    <section className="tab-danh-muc" aria-labelledby="tieu-de-truong-ban-do">
      <h2 id="tieu-de-truong-ban-do">{MAP_FIELD_TITLE}</h2>
      <p className="ghi-chu">{MAP_FIELD_NOTE}</p>
      <p className="ghi-chu">{NO_REGISTER_NOTE}</p>
      {writeDecision !== null && !writeDecision.hien && <p className="trang-thai-rong">{READ_ONLY_NOTE}</p>}

      {types === null && <p role="status">Đang tải nhóm tài nguyên…</p>}
      {types !== null && !types.ok && (
        <p className="thong-bao-loi" role="alert">
          {types.thongBao}
        </p>
      )}
      {types !== null && types.ok && typeItems.length === 0 && (
        <p className="trang-thai-rong">{NO_ASSET_TYPES}</p>
      )}

      {typeItems.length > 0 && (
        <>
          <div className="o-nhap">
            <label htmlFor="o-nhom-tai-nguyen">{ASSET_TYPE_LABEL}</label>
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
          </div>

          {canWrite && (
            <p>
              <button
                type="button"
                className="nut-phu"
                onClick={() => start({ kind: "create", idempotencyKey: crypto.randomUUID() }, newDraft(chosenType))}
              >
                {ADD_FIELD_BUTTON}
              </button>
            </p>
          )}
          {done !== "" && <p role="status">{done}</p>}
          {rowError !== "" && (
            <p className="thong-bao-loi" role="alert">
              {rowError}
            </p>
          )}
          {open !== null && open.kind === "create" && formNode}

          {fields === null && <p role="status">Đang tải các trường…</p>}
          {fields !== null && !fields.ok && (
            <p className="thong-bao-loi" role="alert">
              {fields.thongBao}
            </p>
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
  if (rows.length === 0) return <p className="trang-thai-rong">{NO_FIELDS}</p>;
  const cols = canWrite ? 7 : 6;
  return (
    <div className="bang-cuon" role="region" aria-label="Các trường tuỳ biến" tabIndex={0}>
      <table className="bang-danh-muc">
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
    </div>
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
          <span className={row.is_active ? "chip chip-hoat-dong" : "chip chip-ngung"}>
            {row.is_active ? "Đang dùng" : "Đang tắt"}
          </span>
        </td>
        {canWrite && (
          <td>
            <span className="cum-nut">
              <button type="button" className="nut-phu" aria-label={`Sửa trường ${row.label}`} onClick={() => onEdit(row)}>
                ✎ Sửa
              </button>
              <button
                type="button"
                className="nut-phu"
                aria-label={`${row.is_active ? "Tắt" : "Bật"} trường ${row.label}`}
                onClick={() => onToggle(row)}
              >
                {row.is_active ? "Tắt" : "Bật"}
              </button>
              <button
                type="button"
                className="nut-phu nut-xoa"
                aria-label={`Xoá trường ${row.label}`}
                onClick={() => onDelete(row)}
              >
                🗑 Xoá
              </button>
            </span>
          </td>
        )}
      </tr>
      {form !== null && (
        <tr>
          <td colSpan={cols}>{form}</td>
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
      className="form-danh-muc"
      aria-label={title}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
      onKeyDown={(e) => {
        if (e.key === "Escape" && !sending) onCancel();
      }}
    >
      <h4>{title}</h4>

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

      <div className="o-nhap">
        <label>
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

      <div className="cum-nut">
        <button type="submit" className="nut-chinh" disabled={sending}>
          Lưu
        </button>
        <button type="button" className="nut-phu" onClick={onCancel} disabled={sending}>
          Huỷ
        </button>
      </div>
    </form>
  );
}

function OptionsEditor({ draft, setDraft }: { draft: MapFieldDraft; setDraft: (d: MapFieldDraft) => void }) {
  return (
    <fieldset className="o-nhap">
      <legend>Các lựa chọn</legend>
      <p className="ghi-chu">{OPTIONS_HINT}</p>
      {draft.options.map((o, i) => (
        <div className="cum-nut" key={i}>
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
            <button
              type="button"
              className="nut-phu"
              aria-label={`Bỏ lựa chọn ${i + 1}`}
              onClick={() => setDraft(removeOption(draft, i))}
            >
              Bỏ
            </button>
          )}
        </div>
      ))}
      <button type="button" className="nut-phu" onClick={() => setDraft(addOption(draft))}>
        + Thêm lựa chọn
      </button>
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
    <form
      className="form-danh-muc"
      aria-label={title}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
      onKeyDown={(e) => {
        if (e.key === "Escape" && !sending) onCancel();
      }}
    >
      <h4>{title}</h4>
      <p className="ghi-chu">{DELETE_FIELD_EXPLANATION}</p>
      <div className="o-nhap">
        <label htmlFor="o-ly-do-xoa-truong">Lý do xoá</label>
        <textarea id="o-ly-do-xoa-truong" name="reason" required value={reason} onChange={(e) => setReason(e.target.value)} />
      </div>
      {error !== "" && (
        <p className="thong-bao-loi" role="alert">
          {error}
        </p>
      )}
      <div className="cum-nut">
        <button type="submit" className="nut-chinh nut-xoa" disabled={sending}>
          Xác nhận xoá
        </button>
        <button type="button" className="nut-phu" onClick={onCancel} disabled={sending}>
          Huỷ
        </button>
      </div>
    </form>
  );
}
