import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { comms_loaiTaiNguyenRa, comms_mapFieldSchemaOut } from "@/lib/api/schema.gen";

import {
  LABEL_NEEDS_ALNUM,
  SORT_ORDER_ERROR,
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
import {
  MAP_FIELD_NOTE,
  MapFieldAddRow,
  MapFieldDeleteForm,
  MapFieldTable,
  MapFieldToolbar,
  NO_FIELDS,
  assetTypeOptionLabel,
} from "./map-field-tab";
import type { RowDeleting, RowEditing } from "./map-field-tab";

/**
 * "Trường bản đồ" (spec Cấu hình 06, ADR 0079): the OPTION LOCK (an issued option value can be
 * relabelled, never changed or removed), the field code GENERATED from the label in the server's
 * shape, PATCH bodies that never name the immutable fields, and write controls only with
 * `admin.lookup` — the denied case rendered, not only the allowed one.
 */

const ROW: comms_mapFieldSchemaOut = {
  id: "01JMF1",
  asset_type_code: "doanh-nghiep",
  field_code: "legal_form",
  label: "Loại hình doanh nghiệp",
  value_type: "chon",
  options: [
    { value: "tnhh", label: "Công ty TNHH" },
    { value: "co-phan", label: "Công ty cổ phần" },
  ],
  is_required: false,
  sort_order: 1,
  is_active: true,
};

const SERVER_CODE_SHAPE = /^[a-z][a-z0-9_]*$/;

describe("field code generated from the label", () => {
  it("strips diacritics, đ → d, lowercases, joins with _ (the spec's example)", () => {
    expect(fieldCodeFromLabel("Số phòng học kiên cố")).toBe("so_phong_hoc_kien_co");
    expect(fieldCodeFromLabel("  Đường – nội đồng (km) ")).toBe("duong_noi_dong_km");
    expect(fieldCodeFromLabel("Hợp tác xã ĐỨC")).toBe("hop_tac_xa_duc");
  });

  it("always matches the server's `^[a-z][a-z0-9_]*$`, 1–64 — a leading digit gets `f_`", () => {
    expect(fieldCodeFromLabel("3 tầng")).toBe("f_3_tang");
    const long = fieldCodeFromLabel("Diện tích ".repeat(20));
    expect(long.length).toBeLessThanOrEqual(64);
    for (const c of ["3 tầng", "Diện tích ".repeat(20), "Ước tính ơ ư", "a"]) {
      expect(fieldCodeFromLabel(c)).toMatch(SERVER_CODE_SHAPE);
    }
  });

  it("a label with no letter or digit yields no code, and create refuses IN PLACE", () => {
    expect(fieldCodeFromLabel("— !!")).toBe("");
    expect(createBody({ ...newDraft("doanh-nghiep"), label: "— !!" })).toEqual({
      kind: "error",
      message: LABEL_NEEDS_ALNUM,
    });
  });

  it("REGRESSION: create sends the code generated from the label, never a typed one", () => {
    const d = { ...newDraft("doanh-nghiep"), fieldCode: "typed_code", label: " Số học sinh " };
    const b = createBody(d);
    expect(b.kind).toBe("send");
    if (b.kind !== "send") return;
    expect(b.body.field_code).toBe("so_hoc_sinh");
    expect(b.body.label).toBe("Số học sinh");
  });
});

describe("option lock", () => {
  it("existing options come in LOCKED; their value cannot change, their label can", () => {
    const d = editDraft(ROW);
    expect(d.options.every((o) => o.locked)).toBe(true);
    expect(setOptionValue(d, 0, "doi")).toBe(d);
    expect(setOptionLabel(d, 0, "TNHH").options[0]).toEqual({ value: "tnhh", label: "TNHH", locked: true });
  });

  it("an existing option cannot be removed; one added in this form can", () => {
    const d = editDraft(ROW);
    expect(removeOption(d, 0)).toBe(d);
    const added = setOptionValue(addOption(d), 2, "ho-kinh-doanh");
    expect(added.options[2]).toEqual({ value: "ho-kinh-doanh", label: "", locked: false });
    expect(removeOption(added, 2).options).toHaveLength(2);
  });

  it("PATCH options = the WHOLE list, existing first in their order, then appended", () => {
    let d = setOptionLabel(editDraft(ROW), 1, "Cổ phần");
    d = setOptionLabel(setOptionValue(addOption(d), 2, "hkd"), 2, "Hộ kinh doanh");
    const b = updateBody(ROW, d);
    expect(b).toEqual({
      kind: "send",
      body: {
        options: [
          { value: "tnhh", label: "Công ty TNHH" },
          { value: "co-phan", label: "Cổ phần" },
          { value: "hkd", label: "Hộ kinh doanh" },
        ],
      },
    });
  });
});

describe("bodies", () => {
  it("PATCH carries only what changed and never the immutable fields", () => {
    const d = { ...editDraft(ROW), label: "Loại hình", isRequired: true };
    const b = updateBody(ROW, d);
    expect(b).toEqual({ kind: "send", body: { label: "Loại hình", is_required: true } });
    if (b.kind !== "send") return;
    for (const k of ["field_code", "asset_type_code", "value_type"]) expect(b.body).not.toHaveProperty(k);
  });

  it("nothing changed → no request; a blanked Thứ tự is 'unchanged', not 0", () => {
    expect(updateBody(ROW, editDraft(ROW))).toEqual({ kind: "unchanged" });
    expect(updateBody(ROW, { ...editDraft(ROW), sortOrder: "" })).toEqual({ kind: "unchanged" });
  });

  it("a non-integer Thứ tự is a local error", () => {
    expect(updateBody(ROW, { ...editDraft(ROW), sortOrder: "3 chữ" })).toEqual({
      kind: "error",
      message: SORT_ORDER_ERROR,
    });
  });

  it("create: options only for `chon`", () => {
    const d = addOption({ ...newDraft("doanh-nghiep"), label: "Doanh thu", valueType: "so-thap-phan" });
    const b = createBody(d);
    expect(b.kind).toBe("send");
    if (b.kind !== "send") return;
    expect(b.body.options).toBeUndefined();
    expect(b.body.field_code).toBe("doanh_thu");
  });

  it("REGRESSION: six value types with spec 06's labels, the server's values unchanged", () => {
    expect(VALUE_TYPES.map((t) => t.label)).toEqual([
      "Chữ",
      "Số nguyên",
      "Số thập phân",
      "Có / Không",
      "Ngày",
      "Chọn trong danh sách",
    ]);
    expect(VALUE_TYPES.map((t) => t.value)).toEqual(["van-ban", "so-nguyen", "so-thap-phan", "dung-sai", "ngay", "chon"]);
    expect(valueTypeLabel("la")).toBe("la");
  });
});

/* ---- rendering ------------------------------------------------------------------------------- */

const NOOP = () => {};

function table(canWrite: boolean, rows = [ROW], editing: RowEditing | null = null, deleting: RowDeleting | null = null) {
  return renderToStaticMarkup(
    <MapFieldTable
      rows={rows}
      canWrite={canWrite}
      editing={editing}
      deleting={deleting}
      onEdit={NOOP}
      onToggle={NOOP}
      onDelete={NOOP}
    />,
  );
}

function editingOf(draft: MapFieldDraft, error = ""): RowEditing {
  return { rowId: ROW.id, draft, setDraft: NOOP, error, sending: false, onSave: NOOP, onCancel: NOOP };
}

describe("field table", () => {
  it("with admin.lookup: Pencil · Tắt · Trash2 on the row, spec 06's columns and cell styles", () => {
    const html = table(true);
    expect(html).toContain('aria-label="Sửa trường Loại hình doanh nghiệp"');
    expect(html).toContain('title="Sửa nhãn"');
    expect(html).toContain('aria-label="Tắt trường Loại hình doanh nghiệp"');
    expect(html).toContain('aria-label="Xoá trường Loại hình doanh nghiệp"');
    expect(html).toContain('title="Xoá trường"');
    expect(html).toContain("Chọn trong danh sách");
    expect(html).toContain('<td class="text-navy font-medium">Loại hình doanh nghiệp</td>');
    expect(html).toContain('<code class="text-[11.5px]">legal_form</code>');
    expect(html).toContain("Đang dùng");
  });

  it("DENIED: without admin.lookup the list shows, no button and no action column", () => {
    const html = table(false);
    expect(html).toContain("legal_form");
    expect(html).not.toContain("<button");
    expect(html).not.toContain("Thao tác");
  });

  it("DENIED: an edit or delete state passed without admin.lookup renders no input", () => {
    const html = table(false, [ROW], editingOf(editDraft(ROW)), { rowId: ROW.id, form: <form id="x" /> });
    expect(html).not.toContain("<input");
    expect(html).not.toContain('id="x"');
  });

  it("an inactive field offers Bật; an empty group says the prototype's sentence", () => {
    expect(table(true, [{ ...ROW, is_active: false }])).toContain('aria-label="Bật trường Loại hình doanh nghiệp"');
    expect(table(true, [])).toContain(NO_FIELDS);
  });

  it("edit in place: label, Bắt buộc and Thứ tự become inputs, Lưu / Huỷ replace the actions", () => {
    const html = table(true, [ROW], editingOf(editDraft(ROW)));
    expect(html).toMatch(/<input id="o-sua-nhan-01JMF1"[^>]*value="Loại hình doanh nghiệp"/);
    expect(html).toContain('id="o-sua-bat-buoc-01JMF1"');
    expect(html).toMatch(/<input id="o-sua-thu-tu-01JMF1"[^>]*value="1"/);
    expect(html).toContain(">Lưu<");
    expect(html).toContain(">Huỷ<");
    expect(html).not.toContain('aria-label="Sửa trường');
    // Code and value type are text on edit, never inputs.
    expect(html).not.toContain('name="field_code"');
    expect(html).not.toContain('name="value_type"');
  });

  it("edit of a `chon` field: existing option values read-only, new ones editable and removable", () => {
    const html = table(true, [ROW], editingOf(addOption(editDraft(ROW))));
    expect(html).toMatch(/<input id="o-gia-tri-0"[^>]*readOnly=""[^>]*value="tnhh"/);
    expect(html).toMatch(/<input id="o-gia-tri-1"[^>]*readOnly=""[^>]*value="co-phan"/);
    expect(html).not.toMatch(/<input id="o-gia-tri-2"[^>]*readOnly/);
    expect((html.match(/aria-label="Bỏ lựa chọn/g) ?? []).length).toBe(1);
  });

  it("a refused save shows the server's sentence in place, under the row", () => {
    const html = table(true, [{ ...ROW, value_type: "van-ban", options: [] }], editingOf(editDraft(ROW), "Nhãn quá dài."));
    expect(html).toContain('role="alert"');
    expect(html).toContain("Nhãn quá dài.");
  });
});

describe("add row", () => {
  const add = (draft: MapFieldDraft, error = "") =>
    renderToStaticMarkup(
      <MapFieldAddRow draft={draft} setDraft={NOOP} error={error} sending={false} onSubmit={NOOP} onCancel={NOOP} />,
    );

  it("spec 06 §2: label with the spec's placeholder, Kiểu dữ liệu, Bắt buộc, Thêm / Huỷ — no typed code", () => {
    const html = add(newDraft("doanh-nghiep"));
    expect(html).toContain("sm:grid-cols-[1fr_14rem_9rem_auto]");
    expect(html).toContain('placeholder="Ví dụ: Số phòng học kiên cố"');
    expect(html).toContain(">Kiểu dữ liệu<");
    expect(html).toContain('<option value="khong" selected="">Không</option>');
    expect(html).toContain(">Thêm<");
    expect(html).toContain(">Huỷ<");
    expect(html).not.toContain('name="field_code"');
    // No code line until something is typed.
    expect(html).not.toContain("Mã trường:");
  });

  it("the generated code shows under the label as soon as it has text", () => {
    const html = add({ ...newDraft("doanh-nghiep"), label: "Số phòng học kiên cố" });
    expect(html).toContain('<p class="text-ink-muted m-0 mt-1 text-[11px]">Mã trường: <code>so_phong_hoc_kien_co</code></p>');
  });

  it("the options editor appears only for Chọn trong danh sách", () => {
    expect(add(newDraft("doanh-nghiep"))).not.toContain("Các lựa chọn");
    expect(add({ ...newDraft("doanh-nghiep"), valueType: "chon" })).toContain("Các lựa chọn");
  });

  it("a form error is shown in place", () => {
    expect(add(newDraft("doanh-nghiep"), LABEL_NEEDS_ALNUM)).toContain(LABEL_NEEDS_ALNUM);
  });
});

describe("toolbar and footnote", () => {
  const TYPES: comms_loaiTaiNguyenRa[] = [
    { id: "1", code: "doanh-nghiep", label: "Doanh nghiệp", is_default: true, active: true, order: 1, source: "", tier: 0 },
    { id: "2", code: "truong-hoc", label: "Trường học", is_default: true, active: false, order: 2, source: "", tier: 0 },
    { id: "3", code: "dn-cu", label: "Doanh nghiệp", is_default: false, active: false, order: 3, source: "", tier: 0 },
  ];
  const bar = (canWrite: boolean) =>
    renderToStaticMarkup(
      <MapFieldToolbar types={TYPES} chosenType="doanh-nghiep" canWrite={canWrite} onChoose={NOOP} onAdd={NOOP} />,
    );

  it("with admin.lookup: Thêm trường on the right of the group picker", () => {
    const html = bar(true);
    expect(html).toContain("Nhóm tài nguyên");
    expect(html).toContain("ml-auto");
    expect(html).toContain("Thêm trường");
  });

  it("DENIED: without admin.lookup the picker shows, no Thêm trường", () => {
    const html = bar(false);
    expect(html).toContain('id="o-nhom-tai-nguyen"');
    expect(html).not.toContain("Thêm trường");
    expect(html).not.toContain("<button");
  });

  it("'(đang tắt)' only where an inactive group shares its label with another", () => {
    expect(assetTypeOptionLabel(TYPES[1]!, TYPES)).toBe("Trường học");
    expect(assetTypeOptionLabel(TYPES[2]!, TYPES)).toBe("Doanh nghiệp (đang tắt)");
    expect(assetTypeOptionLabel(TYPES[0]!, TYPES)).toBe("Doanh nghiệp");
  });

  it("the footnote never promises an Excel import of assets that does not exist", () => {
    expect(MAP_FIELD_NOTE).not.toContain("Excel");
    expect(MAP_FIELD_NOTE).toBe("Trường đã xoá thì dữ liệu cũ vẫn còn trong hồ sơ, chỉ không hiện ra trên biểu mẫu nữa.");
  });
});

describe("delete keeps its reason step (rule 7)", () => {
  it("a reason field, Xoá / Huỷ, and the server's refusal in place — no explanatory sentence (ADR 0079 lô 3 Q7)", () => {
    const html = renderToStaticMarkup(
      <MapFieldDeleteForm
        label="Loại hình doanh nghiệp"
        reason=""
        setReason={NOOP}
        error="Vui lòng nhập lý do xoá."
        sending={false}
        onSubmit={NOOP}
        onCancel={NOOP}
      />,
    );
    expect(html).toContain('aria-label="Xoá trường Loại hình doanh nghiệp"');
    expect(html).toContain('name="reason"');
    expect(html).toContain(">Xoá<");
    expect(html).toContain(">Huỷ<");
    expect(html).toContain("Vui lòng nhập lý do xoá.");
    // REGRESSION: the consequence sentence the prototype lacks is gone.
    expect(html).not.toContain("Xoá trường thì trường không còn trên biểu mẫu");
    expect(html).not.toContain("không bao giờ được cấp lại");
    expect(html).not.toContain("aria-describedby");
  });
});
