import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_canBoChonNguoiRa } from "@/lib/api/schema.gen";

import { StaffCombobox, STAFF_COMBOBOX_HINT, STAFF_SEARCH_PLACEHOLDER, noMatchText } from "./staff-combobox";
import {
  COMBOBOX_CLOSED,
  comboboxKey,
  comboboxTyped,
  filterStaffOptions,
  foldForSearch,
  staffOptions,
  visibleOptions,
  type ComboboxState,
} from "./staff-combobox-logic";

/**
 * `StaffCombobox` — the type-to-search staff picker (#12, 28/09/2026).
 *
 * THE LIMIT, STATED: vitest runs in Node without a DOM (`vitest.config.mts`), so no case here
 * presses a real key or reads real focus. The keyboard contract is tested on `comboboxKey`, the
 * pure function the component's `onKeyDown` calls with nothing in between; the markup contract
 * (roles, label, ids) is tested on the rendered string.
 */

const DIRECTORY: readonly identity_canBoChonNguoiRa[] = [
  { code: "CB-2026-0000A1", full_name: "Nguyễn Thị Hoa", position: "Văn thư", department_id: "", email_masked: null },
  { code: "CB-2026-0000A2", full_name: "Trần Văn An", position: "Chủ tịch", department_id: "", email_masked: null },
  { code: "CB-2026-0000A3", full_name: "Đặng Quốc Dũng", position: "Chuyên viên", department_id: "", email_masked: null },
  { code: "CB-2026-0000A4", full_name: "Lê Nguyên Khôi", position: "", department_id: "", email_masked: null },
];

const OPTIONS = staffOptions(DIRECTORY, "");
const EMPTY = "— Chưa xác định —";

const names = (q: string) =>
  filterStaffOptions(OPTIONS, q).map((o) => o.label.split(" · ")[0]);

describe("filtering — any part of the full name, with or without diacritics", () => {
  it("'nguyen' finds 'Nguyễn' — and 'Nguyên' in the middle of another name", () => {
    expect(names("nguyen")).toEqual(["Nguyễn Thị Hoa", "Lê Nguyên Khôi"]);
  });

  it("a PART of the name, not only its start: 'hoa', 'van a', 'quoc'", () => {
    expect(names("hoa")).toEqual(["Nguyễn Thị Hoa"]);
    expect(names("van a")).toEqual(["Trần Văn An"]);
    expect(names("quoc")).toEqual(["Đặng Quốc Dũng"]);
  });

  it("`đ` folds to `d` — NFD alone leaves it, and 'dang' would miss 'Đặng'", () => {
    expect(foldForSearch("Đặng Quốc Dũng")).toBe("dang quoc dung");
    expect(names("dang dung")).toEqual(["Đặng Quốc Dũng"]);
  });

  it("typing WITH diacritics still matches, and case does not matter", () => {
    expect(names("THỊ HOA")).toEqual(["Nguyễn Thị Hoa"]);
    // Folding goes both ways: a typed `ễ` also finds `ê`. Names are recalled, not spelled.
    expect(names("Nguyễn")).toEqual(["Nguyễn Thị Hoa", "Lê Nguyên Khôi"]);
  });

  it("words in any order: 'hoa nguyen' finds 'Nguyễn Thị Hoa'", () => {
    expect(names("hoa nguyen")).toEqual(["Nguyễn Thị Hoa"]);
  });

  it("matches the NAME, not the position — 'chu' is not every `Chủ tịch`/`Chuyên viên`", () => {
    expect(names("chu")).toEqual([]);
  });

  it("no query: the 'nobody' line first, then everyone; a query drops the 'nobody' line", () => {
    expect(visibleOptions(OPTIONS, EMPTY, null).map((o) => o.value)).toEqual([
      "",
      ...DIRECTORY.map((d) => d.code),
    ]);
    expect(visibleOptions(OPTIONS, EMPTY, "hoa").map((o) => o.value)).toEqual(["CB-2026-0000A1"]);
  });

  it("a stored value missing from the directory keeps a line, matched by its code", () => {
    const withGone = staffOptions(DIRECTORY, "CB-2019-NGHIHUU");
    expect(withGone[0]?.value).toBe("CB-2019-NGHIHUU");
    expect(filterStaffOptions(withGone, "nghihuu").map((o) => o.value)).toEqual(["CB-2019-NGHIHUU"]);
  });
});

describe("keyboard — ARIA combobox (`comboboxKey`)", () => {
  const all = visibleOptions(OPTIONS, EMPTY, null);

  it("ArrowDown opens on the current selection, then moves; Enter commits that value", () => {
    let s = comboboxKey(COMBOBOX_CLOSED, "ArrowDown", all, "CB-2026-0000A2").state;
    expect(s.open).toBe(true);
    expect(s.active).toBe(2);
    s = comboboxKey(s, "ArrowDown", all, "CB-2026-0000A2").state;
    expect(s.active).toBe(3);
    const r = comboboxKey(s, "Enter", all, "CB-2026-0000A2");
    expect(r.commit).toBe("CB-2026-0000A3");
    expect(r.state).toEqual(COMBOBOX_CLOSED);
    expect(r.handled).toBe(true);
  });

  it("ArrowUp stops at the first line; ArrowDown stops at the last — no silent wrap", () => {
    let s: ComboboxState = { open: true, query: null, active: 0 };
    s = comboboxKey(s, "ArrowUp", all, "").state;
    expect(s.active).toBe(0);
    s = { open: true, query: null, active: all.length - 1 };
    expect(comboboxKey(s, "ArrowDown", all, "").state.active).toBe(all.length - 1);
  });

  it("type then Enter picks the FIRST match — how an elderly clerk expects a search box to work", () => {
    const typed = comboboxTyped("nguyen", OPTIONS, EMPTY);
    expect(typed).toEqual({ open: true, query: "nguyen", active: 0 });
    const visible = visibleOptions(OPTIONS, EMPTY, typed.query);
    expect(comboboxKey(typed, "Enter", visible, "").commit).toBe("CB-2026-0000A1");
  });

  it("Escape closes and REVERTS the text; it never changes the value", () => {
    const typed = comboboxTyped("tran", OPTIONS, EMPTY);
    const r = comboboxKey(typed, "Escape", visibleOptions(OPTIONS, EMPTY, "tran"), "CB-2026-0000A1");
    expect(r.state).toEqual(COMBOBOX_CLOSED);
    expect(r.commit).toBeNull();
    expect(r.handled).toBe(true);
  });

  it("Enter is ALWAYS consumed, even closed — a stray Enter must not submit the Giao việc form", () => {
    const r = comboboxKey(COMBOBOX_CLOSED, "Enter", all, "");
    expect(r.handled).toBe(true);
    expect(r.commit).toBeNull();
  });

  it("Tab leaves without picking anything, and does not swallow the Tab", () => {
    const typed = comboboxTyped("hoa", OPTIONS, EMPTY);
    const r = comboboxKey(typed, "Tab", visibleOptions(OPTIONS, EMPTY, "hoa"), "");
    expect(r.commit).toBeNull();
    expect(r.handled).toBe(false);
    expect(r.state.open).toBe(false);
  });

  it("no match: nothing active, Enter commits nothing, and a sentence says so", () => {
    const typed = comboboxTyped("zzz", OPTIONS, EMPTY);
    expect(typed.active).toBe(-1);
    expect(comboboxKey(typed, "Enter", [], "").commit).toBeNull();
    expect(noMatchText(" zzz ")).toContain("“zzz”");
  });
});

describe("markup — label above, ARIA roles, codes as values", () => {
  const html = renderToStaticMarkup(
    <StaffCombobox
      id="giao-lanh-dao"
      label="Lãnh đạo giao việc"
      emptyLabel={EMPTY}
      value="CB-2026-0000A2"
      directory={DIRECTORY}
      onChange={() => {}}
    />,
  );

  it("a visible `<label for>` precedes the input", () => {
    expect(html).toContain('<label for="giao-lanh-dao">Lãnh đạo giao việc</label>');
    expect(html.indexOf("<label")).toBeLessThan(html.indexOf("<input"));
  });

  it("input is `role=combobox`, collapsed, controlling the listbox, with the hint described", () => {
    expect(html).toMatch(/<input id="giao-lanh-dao" type="text" role="combobox" aria-expanded="false"/);
    expect(html).toContain('aria-controls="giao-lanh-dao-danh-sach"');
    expect(html).toContain('aria-autocomplete="list"');
    expect(html).toContain('aria-describedby="giao-lanh-dao-goi-y"');
    expect(html).toContain(STAFF_COMBOBOX_HINT);
  });

  it("closed: shows the selected label; the listbox is present but hidden", () => {
    expect(html).toContain('value="Trần Văn An · Chủ tịch"');
    expect(html).toMatch(/<ul id="giao-lanh-dao-danh-sach" role="listbox"[^>]* hidden=""/);
  });

  it("each option carries the business code `CB-…`, never an internal id", () => {
    for (const d of DIRECTORY) expect(html).toContain(`data-value="${d.code}"`);
    expect(html.split('role="option"').length - 1).toBe(DIRECTORY.length + 1);
  });

  it("disabled (directory loading): input disabled, and the hint says why in words", () => {
    const loading = renderToStaticMarkup(
      <StaffCombobox
        id="x"
        label="Người thực hiện"
        emptyLabel="Đang tải danh bạ cán bộ…"
        value=""
        directory={[]}
        disabled
        onChange={() => {}}
      />,
    );
    expect(loading).toMatch(/<input id="x"[^>]* disabled=""/);
    expect(loading).toContain('<p id="x-goi-y" class="goi-y-tim">Đang tải danh bạ cán bộ…</p>');
  });

  it("`compact` (07/10, prototype PersonPicker): one field, ⇕ toggle inside, keyboard line sr-only", () => {
    const box = (props: { compact?: boolean; disabled?: boolean }) =>
      renderToStaticMarkup(
        <StaffCombobox id="c" label="Người thực hiện" emptyLabel={EMPTY} value="" directory={DIRECTORY} onChange={() => {}} {...props} />,
      );
    const compact = box({ compact: true });
    // Same input contract: role, description, list control.
    expect(compact).toMatch(/<input id="c" type="text" role="combobox"[^>]* aria-describedby="c-goi-y"[^>]* class="pr-8"/);
    expect(compact).toContain(`<p id="c-goi-y" class="goi-y-tim sr-only">${STAFF_COMBOBOX_HINT}</p>`);
    // The toggle is still a button with its label, out of the tab order, inside the relative box.
    expect(compact).toMatch(/<div class="hop-tim-can-bo relative"><input [^>]*><button type="button" class="absolute inset-y-0 right-0 [^"]*" tabindex="-1" aria-label="Mở danh sách Người thực hiện" aria-controls="c-danh-sach" aria-expanded="false">/);
    expect(compact).toContain("lucide-chevrons-up-down");
    expect(compact).not.toContain("nut-mo-danh-sach");
    // Loading: the line says why, visibly, in both variants.
    expect(box({ compact: true, disabled: true })).toContain(`<p id="c-goi-y" class="goi-y-tim">${EMPTY}</p>`);
    // Default unchanged.
    const plain = box({});
    expect(plain).toContain('<div class="hop-tim-can-bo">');
    expect(plain).toContain('class="nut-phu nut-mo-danh-sach"');
    expect(plain).toContain(`<p id="c-goi-y" class="goi-y-tim">${STAFF_COMBOBOX_HINT}</p>`);
  });

  it("`placeholder` (07/10, prototype `Gõ tên để tìm…`): in the empty box; the empty choice stays in the list", () => {
    const box = (props: { value: string; disabled?: boolean; placeholder?: string }) =>
      renderToStaticMarkup(
        <StaffCombobox id="p" label="Người thực hiện" emptyLabel={EMPTY} directory={DIRECTORY} onChange={() => {}} {...props} />,
      );
    const input = (html: string) => /<input [^>]*>/.exec(html)?.[0] ?? "";
    const empty = box({ value: "", placeholder: STAFF_SEARCH_PLACEHOLDER });
    expect(STAFF_SEARCH_PLACEHOLDER).toBe("Gõ tên để tìm…");
    expect(input(empty)).toContain(`placeholder="${STAFF_SEARCH_PLACEHOLDER}"`);
    expect(empty).toMatch(new RegExp(`data-value="">(<span aria-hidden="true">✓ </span>)?${EMPTY}</li>`));
    // Without the prop: unchanged — the empty choice is the placeholder.
    expect(input(box({ value: "" }))).toContain(`placeholder="${EMPTY}"`);
    // Loading: says it is loading, not "type to search".
    expect(input(box({ value: "", disabled: true, placeholder: STAFF_SEARCH_PLACEHOLDER }))).toContain(
      `placeholder="${EMPTY}"`,
    );
    // A chosen person: no placeholder at all.
    expect(input(box({ value: "CB-2026-0000A2", placeholder: STAFF_SEARCH_PLACEHOLDER }))).not.toContain(
      "placeholder=",
    );
  });
});
