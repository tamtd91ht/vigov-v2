import { renderToStaticMarkup } from "react-dom/server";
import type { ReactElement } from "react";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { STAFF_IMPORT_TARGET } from "@/features/cau-hinh/excel-import-targets";
import { ONCE_WARNING, SAVED_CLOSE_BUTTON } from "@/features/cau-hinh/staff-import-result";
import { commitImport, downloadImportTemplate, previewImport } from "@/lib/api/excel-import";
import type { StaffImportCreatedRow, StaffImportPlannedRow } from "@/lib/api/staff-import";

import {
  CHECKING_FILE,
  CHOOSE_FILE_BUTTON,
  DIRECTORY_IMPORT_DESCRIPTION,
  DIRECTORY_IMPORT_TITLE,
  DIRECTORY_TEMPLATE_BUTTON,
  FILE_UNREADABLE_TOAST,
  IMPORT_FAILED_TOAST,
  ROWS_REFUSED_TOAST,
  TEMPLATE_FAILED_TOAST,
  StaffDirectoryImportDialog,
  StaffDirectoryImportView,
  rowCounts,
} from "./staff-directory-import-dialog";

/**
 * The Danh bạ tab's import dialog: the prototype's presentation (`ExcelImportDialog.tsx:104-230`) over
 * the unchanged staff-import flow. The container is driven for real through a minimal hook runner
 * (`useState`, `useReducer`), the shape of `cau-hinh/staff-import.test.tsx`; the network is mocked, so
 * what is checked is what the dialog SENDS and what it SHOWS.
 *
 * Fake data only: `.invalid` addresses, a password that is obviously not real.
 */

const H = vi.hoisted(() => ({
  current: null as null | { slots: { value: unknown }[]; i: number },
}));

vi.mock("react", async (original) => {
  const R = await original<typeof import("react")>();
  const slot = () => {
    const c = H.current;
    if (c === null) throw new Error("hook called outside a render");
    return { c, i: c.i++ };
  };
  return {
    ...R,
    useState: (init: unknown) => {
      const { c, i } = slot();
      if (c.slots[i] === undefined) c.slots[i] = { value: typeof init === "function" ? (init as () => unknown)() : init };
      const s = c.slots[i] as { value: unknown };
      return [s.value, (v: unknown) => (s.value = typeof v === "function" ? (v as (o: unknown) => unknown)(s.value) : v)];
    },
    useReducer: (reducer: (s: unknown, e: unknown) => unknown, init: unknown) => {
      const { c, i } = slot();
      if (c.slots[i] === undefined) c.slots[i] = { value: init };
      const s = c.slots[i] as { value: unknown };
      return [s.value, (e: unknown) => (s.value = reducer(s.value, e))];
    },
  };
});

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

vi.mock("@/lib/api/excel-import", async (original) => ({
  ...(await original<typeof import("@/lib/api/excel-import")>()),
  previewImport: vi.fn(),
  commitImport: vi.fn(),
  downloadImportTemplate: vi.fn(),
}));

const preview = vi.mocked(previewImport);
const commit = vi.mocked(commitImport);

type ViewProps = Parameters<typeof StaffDirectoryImportView>[0];

function run(props: Parameters<typeof StaffDirectoryImportDialog>[0]) {
  const state = { slots: [] as { value: unknown }[], i: 0 };
  let view: ReactElement<ViewProps> | null = null;
  const render = () => {
    state.i = 0;
    H.current = state;
    try {
      const dialog = StaffDirectoryImportDialog(props) as ReactElement<{ children: ReactElement<ViewProps> }>;
      view = dialog.props.children;
    } finally {
      H.current = null;
    }
    return view.props;
  };
  return { render, html: () => renderToStaticMarkup(view!) };
}

const tick = () => new Promise((r) => setTimeout(r, 0));

const PLANNED: StaffImportPlannedRow[] = [2, 3].map((row) => ({
  row,
  full_name: `Cán bộ ${row}`,
  email: "",
  position: "",
  org_unit_code: "",
  org_unit_name: "",
  role_code: "",
  role_name: "",
  office_phone: "",
  mobile: "",
  issues_account: false,
}));

function created(row: number, password?: string): StaffImportCreatedRow {
  return {
    row,
    id: `01J0000000000000000000000${row}`,
    code: `CB-00${row}00`,
    full_name: `Cán bộ ${row}`,
    login: password === undefined ? "" : `cb${row}@demo.invalid`,
    org_unit_code: "",
    role_code: "",
    account_issued: password !== undefined,
    ...(password === undefined ? {} : { temporary_password: password }),
  };
}

const FILE = new File(["x"], "danh-ba.xlsx");

/** The two footer buttons: the last flex row of the body. */
function footer(html: string): string[] {
  const tail = html.slice(html.lastIndexOf('<div class="flex justify-end gap-2">'));
  return [...tail.matchAll(/<button[^>]*>(.*?)<\/button>/g)].map((m) => m[1] ?? "");
}

beforeEach(() => {
  preview.mockResolvedValue({ ok: true, duLieu: { valid: true, rows: PLANNED, errors: [] } });
  commit.mockResolvedValue({ ok: true, created: [created(2), created(3)] });
});

afterEach(() => {
  vi.clearAllMocks();
});

describe("presentation — the prototype's words, verbatim", () => {
  it("title without icon, description, ghost template link, dashed box, two footer buttons", () => {
    const m = run({ onImported: vi.fn(), onClose: vi.fn() });
    m.render();
    const html = m.html();
    expect(html).toContain(`>${DIRECTORY_IMPORT_TITLE}</h2>`);
    expect(DIRECTORY_IMPORT_TITLE).toBe("Nhập danh bạ cán bộ từ Excel");
    expect(DIRECTORY_IMPORT_DESCRIPTION).toBe(
      "Tệp được kiểm trước và chưa ghi gì. Còn một dòng sai thì không dòng nào được nhận — sửa tệp rồi nhập lại.",
    );
    expect(html).toContain(DIRECTORY_IMPORT_DESCRIPTION);
    expect(DIRECTORY_TEMPLATE_BUTTON).toBe("Tải mẫu danh bạ");
    expect(html).toMatch(/<button[^>]*text-brand[^>]*>.*Tải mẫu danh bạ<\/button>/);
    expect(html).toContain("border-dashed");
    expect(CHOOSE_FILE_BUTTON).toBe("Chọn tệp .xlsx");
    expect(html).toContain(">Chọn tệp .xlsx</button>");
    expect(footer(html)).toEqual(["Đóng", "Nhập"]);
    // No separate "Kiểm tra tệp" button: choosing the file is the check.
    expect(html).not.toContain("Kiểm tra tệp");
  });
});

describe("choosing a file runs the check at once", () => {
  it("one preview of the chosen file; 'Đang kiểm tệp…' while it runs; then the green line and 'Nhập 2 cán bộ'", async () => {
    let release: () => void = () => {};
    preview.mockImplementationOnce(
      () =>
        new Promise((r) => {
          release = () => r({ ok: true, duLieu: { valid: true, rows: PLANNED, errors: [] } });
        }),
    );
    const m = run({ onImported: vi.fn(), onClose: vi.fn() });
    m.render().onChooseFile(FILE);
    m.render();
    expect(preview).toHaveBeenCalledTimes(1);
    expect(preview).toHaveBeenCalledWith(STAFF_IMPORT_TARGET.routes, STAFF_IMPORT_TARGET.rowsField, FILE, "danh-ba.xlsx");
    expect(m.html()).toContain(CHECKING_FILE);
    expect(CHECKING_FILE).toBe("Đang kiểm tệp…");
    expect(m.html()).toContain(">danh-ba.xlsx</p>");

    release();
    await tick();
    m.render();
    const html = m.html();
    expect(html).not.toContain(CHECKING_FILE);
    expect(html).toContain("2 dòng hợp lệ, sẵn sàng nhập");
    // No table of the valid rows.
    expect(html).not.toContain("Cán bộ 2");
    expect(footer(html)).toEqual(["Đóng", "Nhập 2 cán bộ"]);
  });

  it("invalid file: red '{e} dòng sai trên tổng số {n} dòng', errors table headed Dòng · Cột · Vấn đề; import disabled", async () => {
    preview.mockResolvedValueOnce({
      ok: true,
      duLieu: {
        valid: false,
        rows: PLANNED,
        errors: [
          { row: 4, column: "Họ và tên", message: "Thiếu họ và tên." },
          { row: 4, column: "Bộ phận", message: "Không có bộ phận này." },
        ],
      },
    });
    const m = run({ onImported: vi.fn(), onClose: vi.fn() });
    m.render().onChooseFile(FILE);
    await tick();
    const props = m.render();
    const html = m.html();
    expect(html).toContain("1 dòng sai trên tổng số 3 dòng");
    expect(html).toMatch(/<th[^>]*>Dòng<\/th><th[^>]*>Cột<\/th><th[^>]*>Vấn đề<\/th>/);
    expect(html).toContain("Không có bộ phận này.");
    expect(html).toMatch(/<button[^>]*disabled[^>]*>Nhập 3 cán bộ<\/button>/);
    props.onImport();
    await tick();
    expect(commit).not.toHaveBeenCalled();
  });

  it("row counts: union of planned and refused rows; a whole-file error (row 0) is not a row", () => {
    expect(rowCounts(PLANNED, [{ row: 2, column: "", message: "x" }])).toEqual({ wrong: 1, total: 2 });
    expect(rowCounts([], [{ row: 0, column: "", message: "x" }])).toEqual({ wrong: 0, total: 0 });
  });
});

describe("import", () => {
  it("success without passwords → toast 'Đã nhập 2 cán bộ.', list re-read, dialog closes", async () => {
    const onImported = vi.fn();
    const onClose = vi.fn();
    const m = run({ onImported, onClose });
    m.render().onChooseFile(FILE);
    await tick();
    m.render().onImport();
    await tick();
    expect(commit).toHaveBeenCalledTimes(1);
    const key = commit.mock.calls[0]?.[3];
    expect(typeof key === "string" && key.length > 0).toBe(true);
    expect(onImported).toHaveBeenCalledTimes(1);
    expect(toast.success).toHaveBeenCalledWith("Đã nhập 2 cán bộ.");
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("success WITH temporary passwords → toast, re-read, but the dialog STAYS on the one-time passwords", async () => {
    commit.mockResolvedValueOnce({ ok: true, created: [created(2, "TEST-PASS-WORD-0001"), created(3)] });
    const onImported = vi.fn();
    const onClose = vi.fn();
    const m = run({ onImported, onClose });
    m.render().onChooseFile(FILE);
    await tick();
    m.render().onImport();
    await tick();
    m.render();
    const html = m.html();
    expect(toast.success).toHaveBeenCalledWith("Đã nhập 2 cán bộ.");
    expect(onImported).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();
    expect(html).toContain("TEST-PASS-WORD-0001");
    expect(html).toContain(ONCE_WARNING);
    expect(html).toContain(SAVED_CLOSE_BUTTON);
    // The upload box and the footer are gone: only the explicit button closes now.
    expect(html).not.toContain(CHOOSE_FILE_BUTTON);
  });

  it("refused import (not row errors) → the prototype's toast, NO red box, the green preview stays; a retry reuses the SAME key", async () => {
    commit.mockResolvedValueOnce({ ok: false, message: "Không kết nối được máy chủ (máy chủ nói).", errors: [] });
    const onClose = vi.fn();
    const m = run({ onImported: vi.fn(), onClose });
    m.render().onChooseFile(FILE);
    await tick();
    m.render().onImport();
    await tick();
    m.render();
    const html = m.html();
    expect(IMPORT_FAILED_TOAST).toBe("Không nhập được. Vui lòng thử lại.");
    expect(toast.error).toHaveBeenCalledWith(IMPORT_FAILED_TOAST);
    expect(html).not.toContain("Không kết nối được máy chủ (máy chủ nói).");
    expect(html).not.toContain('role="alert"');
    expect(html).toContain("2 dòng hợp lệ, sẵn sàng nhập");
    expect(onClose).not.toHaveBeenCalled();
    m.render().onImport();
    await tick();
    expect(commit.mock.calls[1]?.[3]).toBe(commit.mock.calls[0]?.[3]);
  });

  it("import refused for ROW errors → toast 'Tệp còn dòng sai, chưa ghi dòng nào.', the preview turns red with them, Nhập held", async () => {
    commit.mockResolvedValueOnce({
      ok: false,
      message: "Tệp có dòng sai (máy chủ nói).",
      errors: [{ row: 3, column: "Bộ phận", message: "Bộ phận vừa bị xoá." }],
    });
    const m = run({ onImported: vi.fn(), onClose: vi.fn() });
    m.render().onChooseFile(FILE);
    await tick();
    m.render().onImport();
    await tick();
    m.render();
    const html = m.html();
    expect(ROWS_REFUSED_TOAST).toBe("Tệp còn dòng sai, chưa ghi dòng nào.");
    expect(toast.error).toHaveBeenCalledWith(ROWS_REFUSED_TOAST);
    expect(html).not.toContain("Tệp có dòng sai (máy chủ nói).");
    expect(html).not.toContain("sẵn sàng nhập");
    expect(html).toContain("1 dòng sai trên tổng số 2 dòng");
    expect(html).toContain("Bộ phận vừa bị xoá.");
    expect(html).toMatch(/<button[^>]*disabled[^>]*>Nhập 2 cán bộ<\/button>/);
  });
});

describe("error paths per the prototype (`ExcelImportDialog.tsx:69, 121`)", () => {
  it("the check itself fails → toast 'Không đọc được tệp. Kiểm tra lại định dạng .xlsx.', no red box with the server's sentence", async () => {
    preview.mockResolvedValueOnce({ ok: false, thongBao: "Tệp không phải .xlsx (máy chủ nói)." });
    const m = run({ onImported: vi.fn(), onClose: vi.fn() });
    m.render().onChooseFile(FILE);
    await tick();
    m.render();
    const html = m.html();
    expect(FILE_UNREADABLE_TOAST).toBe("Không đọc được tệp. Kiểm tra lại định dạng .xlsx.");
    expect(toast.error).toHaveBeenCalledWith(FILE_UNREADABLE_TOAST);
    expect(html).not.toContain("máy chủ nói");
    expect(html).not.toContain('role="alert"');
    expect(footer(html)).toEqual(["Đóng", "Nhập"]);
    expect(html).toMatch(/<button[^>]*disabled[^>]*>Nhập<\/button>/);
  });

  it("the template download fails → toast 'Không tải được tệp mẫu.'", async () => {
    vi.mocked(downloadImportTemplate).mockResolvedValueOnce({ ok: false, thongBao: "Lỗi riêng của máy chủ." });
    const m = run({ onImported: vi.fn(), onClose: vi.fn() });
    m.render().onDownloadTemplate();
    await tick();
    expect(TEMPLATE_FAILED_TOAST).toBe("Không tải được tệp mẫu.");
    expect(toast.error).toHaveBeenCalledWith(TEMPLATE_FAILED_TOAST);
    expect(toast.error).not.toHaveBeenCalledWith("Lỗi riêng của máy chủ.");
  });

  it("a valid file with ZERO rows → green '0 dòng hợp lệ, sẵn sàng nhập', Nhập held, nothing sent", async () => {
    preview.mockResolvedValueOnce({ ok: true, duLieu: { valid: true, rows: [], errors: [] } });
    const m = run({ onImported: vi.fn(), onClose: vi.fn() });
    m.render().onChooseFile(FILE);
    await tick();
    const props = m.render();
    const html = m.html();
    expect(html).toContain("0 dòng hợp lệ, sẵn sàng nhập");
    expect(html).toContain("bg-leaf/8");
    expect(html).not.toContain("Tệp không có dòng nào để nhập.");
    expect(html).not.toContain('role="alert"');
    expect(html).toMatch(/<button[^>]*disabled[^>]*>Nhập 0 cán bộ<\/button>/);
    props.onImport();
    await tick();
    expect(commit).not.toHaveBeenCalled();
  });
});
