import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { comms_mapFieldSchemaOut } from "@/lib/api/schema.gen";

import {
  SORT_ORDER_ERROR,
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
import { MapFieldForm, MapFieldTable, NO_FIELDS } from "./map-field-tab";

/**
 * "Trường bản đồ": the OPTION LOCK (an issued option value can be relabelled, never changed or
 * removed), PATCH bodies that never name the immutable fields, and write controls only with
 * `admin.lookup`.
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

  it("the edit form renders existing option values read-only, new ones editable", () => {
    const d = addOption(editDraft(ROW));
    const html = renderToStaticMarkup(
      <MapFieldForm mode="edit" draft={d} setDraft={() => {}} error="" sending={false} onSubmit={() => {}} onCancel={() => {}} />,
    );
    expect(html).toMatch(/<input id="o-gia-tri-0" readOnly=""[^>]*value="tnhh"\/>/);
    expect(html).toMatch(/<input id="o-gia-tri-1" readOnly=""[^>]*value="co-phan"\/>/);
    expect(html).not.toMatch(/<input id="o-gia-tri-2"[^>]*readOnly/);
    // Only the appended row can be removed.
    expect((html.match(/aria-label="Bỏ lựa chọn/g) ?? []).length).toBe(1);
    // Code and value type are text on edit, never inputs.
    expect(html).not.toContain('name="field_code"');
    expect(html).not.toContain('name="value_type"');
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
    expect(createBody({ ...newDraft("doanh-nghiep"), sortOrder: "3 chữ" })).toEqual({
      kind: "error",
      message: SORT_ORDER_ERROR,
    });
  });

  it("create: options only for `chon`", () => {
    const d = addOption({ ...newDraft("doanh-nghiep"), fieldCode: " revenue ", label: "Doanh thu", valueType: "so-thap-phan" });
    const b = createBody(d);
    expect(b.kind).toBe("send");
    if (b.kind !== "send") return;
    expect(b.body.options).toBeUndefined();
    expect(b.body.field_code).toBe("revenue");
  });

  it("six value types with the spec's labels; an unknown value is shown as sent", () => {
    expect(VALUE_TYPES.map((t) => t.label)).toEqual([
      "Văn bản",
      "Số nguyên",
      "Số thập phân",
      "Đúng/Sai",
      "Ngày",
      "Chọn trong danh sách",
    ]);
    expect(valueTypeLabel("la")).toBe("la");
  });
});

describe("field table", () => {
  const table = (canWrite: boolean, rows = [ROW]) =>
    renderToStaticMarkup(
      <MapFieldTable
        rows={rows}
        canWrite={canWrite}
        openRowId={null}
        openForm={null}
        onEdit={() => {}}
        onToggle={() => {}}
        onDelete={() => {}}
      />,
    );

  it("with admin.lookup: ✎ · Tắt · 🗑 on the row", () => {
    const html = table(true);
    expect(html).toContain('aria-label="Sửa trường Loại hình doanh nghiệp"');
    expect(html).toContain('aria-label="Tắt trường Loại hình doanh nghiệp"');
    expect(html).toContain('aria-label="Xoá trường Loại hình doanh nghiệp"');
    expect(html).toContain("Chọn trong danh sách");
  });

  it("DENIED: without admin.lookup the list shows, no button at all", () => {
    const html = table(false);
    expect(html).toContain("legal_form");
    expect(html).not.toContain("<button");
  });

  it("an inactive field offers Bật, and an empty type says so", () => {
    expect(table(true, [{ ...ROW, is_active: false }])).toContain('aria-label="Bật trường Loại hình doanh nghiệp"');
    expect(table(true, [])).toContain(NO_FIELDS);
  });
});
