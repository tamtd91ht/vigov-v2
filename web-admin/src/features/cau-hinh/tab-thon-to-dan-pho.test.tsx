import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_thonToDanPhoRa } from "@/lib/api/schema.gen";

import {
  ADD_BUTTON,
  CONFIRM_RETIRE_BUTTON,
  NAME_REQUIRED,
  REACTIVATE_BUTTON,
  RETIRE_BUTTON,
  SAVE_FAILED,
  draftForCreate,
  draftFromUnit,
  openCreate,
} from "./residential-unit-form";
import { ResidentialUnitForm, ResidentialUnitsView } from "./tab-thon-to-dan-pho";
import type { ResidentialUnitActions } from "./tab-thon-to-dan-pho";

/**
 * The Thôn / Tổ dân phố tab reaches the page as decided: spec `04-thon-to-dan-pho.md` in the shared
 * table pattern (`config-ui.tsx`), write buttons only with `admin.org` (allowed AND denied), out-of-use
 * units listed and marked with `Dùng lại`, no delete anywhere, no code box, and the server's refusal
 * shown in place. `react-dom/server` in Node, no jsdom.
 */

const NOOP: ResidentialUnitActions = {
  add: () => {},
  edit: () => {},
  askRetire: () => {},
  confirmRetire: () => {},
  cancelRetire: () => {},
  reactivate: () => {},
  reload: () => {},
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

const UNITS = [unit(), unit({ id: "01JB", code: "thon-cu", name: "Thôn Cũ", active: false, type_code: "", type_label: "" })];

function view(
  canWrite: boolean,
  retiring: identity_thonToDanPhoRa | null = null,
  rowError: { id: string; message: string } | null = null,
  units: readonly identity_thonToDanPhoRa[] = UNITS,
) {
  return renderToStaticMarkup(
    <ResidentialUnitsView
      load={{ phase: "ready", units }}
      canWrite={canWrite}
      actions={NOOP}
      sending={false}
      form={null}
      retiring={retiring}
      rowError={rowError}
    />,
  );
}

describe("list and write buttons", () => {
  it("with admin.org: import, add, edit on each row; Ngừng dùng on the active one, Dùng lại on the other", () => {
    const html = view(true);
    expect(html).toContain(ADD_BUTTON);
    expect(html).toContain("Nhập từ Excel");
    expect(html).toContain('aria-label="Sửa Thôn Bình An"');
    expect(html).toContain('title="Sửa"');
    expect(html).toContain('aria-label="Ngừng dùng Thôn Bình An"');
    expect(html).toContain('aria-label="Dùng lại Thôn Cũ"');
    expect(html).not.toContain('aria-label="Ngừng dùng Thôn Cũ"');
  });

  it("the import button comes before the add button (spec 02: Nhập từ Excel heads the tab)", () => {
    const html = view(true);
    expect(html.indexOf("Nhập từ Excel")).toBeLessThan(html.indexOf(ADD_BUTTON));
  });

  it("DENIED (no admin.org): the list is still there, NO button at all, and no read-only notice", () => {
    const html = view(false);
    expect(html).toContain("Thôn Bình An");
    expect(html).toContain("Thôn Cũ");
    expect(html).not.toContain("<button");
    expect(html).not.toContain("chỉ xem được");
    // No action column either: 7 heads, not 8.
    expect(html.match(/<th /g)).toHaveLength(7);
  });

  it("no delete anywhere — the only way out of use is Ngừng dùng; no note about it beside the import", () => {
    const html = view(true);
    expect(html).not.toMatch(/Xoá|🗑/);
    expect(html).not.toContain("Không có thao tác xoá");
  });

  it("spec columns, the actions head is empty (no 'Thao tác' word on screen)", () => {
    const html = view(true);
    for (const head of ["Tên", "Mã", "Loại", "Trưởng thôn / Tổ trưởng", "Số hộ", "Nhân khẩu", "Trạng thái"]) {
      expect(html).toContain(`<th scope="col">${head}</th>`);
    }
    expect(html).not.toContain('<th scope="col">Thao tác</th>');
  });

  it("name navy, code in <code>, null count and missing type are '—', numbers vi-VN", () => {
    const html = view(false);
    expect(html).toContain('<td class="text-navy font-medium">Thôn Bình An</td>');
    expect(html).toContain('<code class="text-[11.5px]">thon-binh-an</code>');
    expect(html).toContain("<td>1.132</td>");
    expect(html).not.toContain("Chưa nhập");
    expect(html).not.toContain("Chưa phân loại");
    expect(html).toContain("<td>—</td>");
  });

  it("status says Đang dùng / Ngừng dùng, never 'Đã tắt'", () => {
    const html = view(false);
    expect(html).toContain("Đang dùng");
    expect(html).toContain("Ngừng dùng");
    expect(html).not.toContain("Đã tắt");
  });

  it("empty list: the spec's sentence inside the table", () => {
    expect(view(true, null, null, [])).toContain("Chưa khai báo thôn hoặc tổ dân phố nào.");
  });

  it("loading: the shared skeleton, no table", () => {
    const html = renderToStaticMarkup(
      <ResidentialUnitsView
        load={{ phase: "loading" }}
        canWrite={false}
        actions={NOOP}
        sending={false}
        form={null}
        retiring={null}
        rowError={null}
      />,
    );
    expect(html).toContain("h-11 w-full");
    expect(html).not.toContain("<table");
  });

  it("loading with admin.org: the import row stays, the add button waits (prototype ConfigWorkspace.tsx:99)", () => {
    const html = renderToStaticMarkup(
      <ResidentialUnitsView
        load={{ phase: "loading" }}
        canWrite={true}
        actions={NOOP}
        sending={false}
        form={null}
        retiring={null}
        rowError={null}
      />,
    );
    expect(html).toContain("Nhập từ Excel");
    expect(html).not.toContain(ADD_BUTTON);
  });

  it("Ngừng dùng asks once, saying old records keep the name", () => {
    const html = view(true, UNITS[0]!);
    expect(html).toContain(CONFIRM_RETIRE_BUTTON);
    expect(html).toContain("mọi hồ sơ, phản ánh đã lập ở đây vẫn giữ nguyên tên địa bàn");
    expect(view(true)).not.toContain(CONFIRM_RETIRE_BUTTON);
  });

  it("a refused toggle: the server's sentence under that row, verbatim", () => {
    const html = view(true, null, { id: "01JB", message: "Không tìm thấy thôn / tổ dân phố." });
    expect(html).toContain('role="alert" class="text-danger m-0 text-[12px] font-medium">Không tìm thấy thôn / tổ dân phố.</p>');
  });

  it("no legacy class names in the new markup", () => {
    const html = view(true);
    for (const legacy of ["bang-danh-muc", "o-thao-tac", "ma-muc", "cum-nut", "ghi-chu", "thong-bao-loi"]) {
      expect(html).not.toContain(legacy);
    }
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

describe("the add / edit form — the spec's grey row", () => {
  it("create: no code box (the server derives it), the spec's fields, 'Thêm' and 'Huỷ'", () => {
    const html = form(openCreate(() => "k"));
    expect(html).toMatch(/^<form[^>]*class="[^"]*bg-background[^"]*sm:grid-cols-\[1fr_12rem_8rem_8rem_auto\]/);
    expect(html).not.toContain('id="o-ma-thon"');
    expect(html).toContain('placeholder="Ví dụ: Tổ dân phố 5"');
    expect(html).toContain('<option value="" selected="">— Chọn loại —</option><option value="thon">Thôn</option>');
    expect(html).toContain('<option value="" selected="">Chưa có</option><option value="CB-002">');
    expect(html).toMatch(/id="o-so-ho"[^>]*type="number"[^>]*min="0"|type="number"[^>]*min="0"[^>]*id="o-so-ho"/);
    expect(html).toContain('id="o-nhan-khau"');
    expect(html).toContain('id="o-thu-tu-thon"');
    expect(html).toContain(">Thêm</button>");
    expect(html).toContain(">Huỷ</button>");
    expect(html).not.toContain("<h4");
  });

  it("edit: same row prefilled, Loại still editable, 'Lưu'; a null count opens as an empty box", () => {
    const html = form({ kind: "edit", unit: unit() });
    expect(html).toContain('value="Thôn Bình An"');
    expect(html).not.toMatch(/<select[^>]*id="o-loai-thon"[^>]*disabled/);
    expect(html).toContain(">Lưu</button>");
    expect(html).not.toContain('id="o-ma-thon"');
    expect(html).toMatch(/id="o-so-ho"[^>]* value=""/);
  });

  it("the server's refusal is said in place after the spec's sentence; a local error verbatim", () => {
    const taken = "Xã đã có một thôn / tổ dân phố cùng tên.";
    expect(form(openCreate(() => "k"), taken)).toContain(`${SAVE_FAILED} ${taken}</p>`);
    expect(form(openCreate(() => "k"), "", NAME_REQUIRED)).toContain(`role="alert" class="text-danger m-0 text-[12px] font-medium">${NAME_REQUIRED}</p>`);
  });

  it("no legacy class names in the form", () => {
    const html = form(openCreate(() => "k"));
    for (const legacy of ["form-danh-muc", "o-nhap", "cum-nut", "ghi-chu", "thong-bao-loi"]) {
      expect(html).not.toContain(legacy);
    }
  });
});
