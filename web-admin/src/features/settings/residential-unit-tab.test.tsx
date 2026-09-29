import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_thonToDanPhoRa } from "@/lib/api/schema.gen";

import {
  ADD_BUTTON,
  CONFIRM_RETIRE_BUTTON,
  NO_WRITE_PERMISSION,
  REACTIVATE_BUTTON,
  RETIRE_BUTTON,
  draftForCreate,
  draftFromUnit,
  openCreate,
} from "./residential-unit-form";
import { IMPORT_BUTTON } from "./excel-import-flow";
import { ResidentialUnitForm, ResidentialUnitsView } from "./residential-unit-tab";
import type { ResidentialUnitActions } from "./residential-unit-tab";

/**
 * The Thôn / Tổ dân phố tab reaches the page as decided: write buttons only with `admin.org` (allowed
 * AND denied), out-of-use units listed and marked with `Dùng lại`, no delete anywhere, the edit form
 * has no code box, and the server's refusal is shown verbatim. `react-dom/server` in Node, no jsdom.
 */

const NOOP: ResidentialUnitActions = {
  add: () => {},
  edit: () => {},
  askRetire: () => {},
  confirmRetire: () => {},
  cancelRetire: () => {},
  reactivate: () => {},
  openImport: () => {},
};

function unit(over: Partial<identity_thonToDanPhoRa> = {}): identity_thonToDanPhoRa {
  return {
    id: "01JA",
    code: "thon-binh-an",
    name: "Thôn Bình An",
    type_code: "thon",
    type_label: "Thôn",
    household_count: null,
    population_count: 1132,
    active: true,
    head_staff_code: "",
    head_staff_name: "",
    order: 0,
    ...over,
  };
}

const UNITS = [unit(), unit({ id: "01JB", code: "thon-cu", name: "Thôn Cũ", active: false })];

function view(
  canWrite: boolean,
  missingPermission = !canWrite,
  retiring: identity_thonToDanPhoRa | null = null,
  rowError: { id: string; message: string } | null = null,
) {
  return renderToStaticMarkup(
    <ResidentialUnitsView
      load={{ phase: "ready", units: UNITS }}
      canWrite={canWrite}
      missingPermission={missingPermission}
      actions={NOOP}
      sending={false}
      doneSentence=""
      topPanel={null}
      retiring={retiring}
      rowError={rowError}
    />,
  );
}

describe("list and write buttons", () => {
  it("with admin.org: add, import, edit on each row; Ngừng dùng on the active one, Dùng lại on the other", () => {
    const html = view(true);
    expect(html).toContain(ADD_BUTTON);
    expect(html).toContain(IMPORT_BUTTON);
    expect(html).toContain('aria-label="Sửa Thôn Bình An"');
    expect(html).toContain('aria-label="Ngừng dùng Thôn Bình An"');
    expect(html).toContain('aria-label="Dùng lại Thôn Cũ"');
    expect(html).not.toContain('aria-label="Ngừng dùng Thôn Cũ"');
    expect(html).not.toContain(NO_WRITE_PERMISSION);
  });

  it("DENIED (no admin.org): the list is still there, NO button at all, and the reason is said", () => {
    const html = view(false);
    expect(html).toContain("Thôn Bình An");
    expect(html).toContain("Thôn Cũ");
    expect(html).not.toContain("<button");
    expect(html).toContain(NO_WRITE_PERMISSION);
  });

  it("session not read yet: no button, and no 'missing permission' sentence either", () => {
    const html = view(false, false);
    expect(html).not.toContain("<button");
    expect(html).not.toContain(NO_WRITE_PERMISSION);
  });

  it("no delete anywhere — the only way out of use is Ngừng dùng", () => {
    const html = view(true);
    expect(html).not.toMatch(/Xoá|🗑/);
  });

  it("an out-of-use unit stays listed, marked; a null count is 'Chưa nhập', never 0", () => {
    const html = view(false);
    expect(html).toContain("Đã tắt");
    expect(html).toContain("<td>Chưa nhập</td>");
    expect(html).toContain("<td>1.132</td>");
  });

  it("Ngừng dùng asks once, saying old records keep the name", () => {
    const html = view(true, false, UNITS[0]!);
    expect(html).toContain(CONFIRM_RETIRE_BUTTON);
    expect(html).toContain("mọi hồ sơ, phản ánh đã lập ở đây vẫn giữ nguyên tên địa bàn");
    expect(view(true)).not.toContain(CONFIRM_RETIRE_BUTTON);
  });

  it("a refused toggle: the server's sentence under that row, verbatim", () => {
    const html = view(true, false, null, { id: "01JB", message: "Không tìm thấy thôn / tổ dân phố." });
    expect(html).toContain('role="alert">Không tìm thấy thôn / tổ dân phố.</p>');
  });

  it("the Ngừng dùng / Dùng lại words are the ones the user asked for", () => {
    expect(RETIRE_BUTTON).toBe("Ngừng dùng");
    expect(REACTIVATE_BUTTON).toBe("Dùng lại");
  });
});

function form(open: Parameters<typeof ResidentialUnitForm>[0]["open"], serverError = "", localError = "") {
  const draft = open.kind === "edit" ? draftFromUnit(open.unit) : draftForCreate();
  return renderToStaticMarkup(
    <ResidentialUnitForm
      open={open}
      draft={draft}
      setDraft={() => {}}
      typeChoices={[{ value: "thon", label: "Thôn" }]}
      headChoices={[{ value: "CB-002", label: "Trần Thị B — Công chức (CB-002)" }]}
      typesError=""
      directoryError=""
      localError={localError}
      serverError={serverError}
      sending={false}
      onSubmit={() => {}}
      onCancel={() => {}}
    />,
  );
}

describe("the add / edit form", () => {
  it("create: a code box, type and head pickers with a 'none' option each, three number boxes", () => {
    const html = form(openCreate(() => "k"));
    expect(html).toContain("Thêm thôn / tổ dân phố");
    expect(html).toContain('id="o-ma-thon"');
    expect(html).toContain('<option value="" selected="">Chưa phân loại</option><option value="thon">Thôn</option>');
    expect(html).toContain('<option value="" selected="">Chưa có</option><option value="CB-002">');
    expect(html).toContain('id="o-so-ho"');
    expect(html).toContain('id="o-nhan-khau"');
    // A blank count must stay blank: no `type="number"`.
    expect(html).not.toContain('type="number"');
  });

  it("edit: NO code box — the code is text, and a null count opens as an empty box", () => {
    const html = form({ kind: "edit", unit: unit() });
    expect(html).toContain("Sửa Thôn Bình An");
    expect(html).not.toContain('id="o-ma-thon"');
    expect(html).toContain("Mã thon-binh-an — mã đã cấp thì không đổi được.");
    expect(html).toMatch(/id="o-so-ho"[^>]* value=""/);
  });

  it("the server's refusal and a local error each reach the page verbatim", () => {
    const taken = "Mã này đã được cấp trong xã (kể cả cho đơn vị đã ngưng dùng — mã đã cấp không cấp lại). Hãy chọn mã khác.";
    expect(form(openCreate(() => "k"), taken)).toContain(`role="alert">${taken}</p>`);
    expect(form(openCreate(() => "k"), "", "Thứ tự phải là một số nguyên, ví dụ: 3.")).toContain(
      'role="alert">Thứ tự phải là một số nguyên, ví dụ: 3.</p>',
    );
  });
});
