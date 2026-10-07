// @vitest-environment jsdom
//
// jsdom: choosing a file, the automatic check, pressing `Nhập` twice under one key — events, which a
// markup string cannot show.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { toast } from "sonner";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

// Outcomes are toasts (ADR 0068 lần 6 #4).
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const api = vi.hoisted(() => ({
  previewDisbursementImport: vi.fn(),
  commitDisbursementImport: vi.fn(),
  downloadDisbursementImportTemplate: vi.fn(),
}));

vi.mock("@/lib/api/disbursement-import", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api/disbursement-import")>()),
  ...api,
}));

import {
  DROP_ZONE_LABEL,
  DUPLICATE_CAUTION,
  DisbursementImportButton,
  IMPORT_DESCRIPTION,
  IMPORT_TITLE,
  ROWS_STILL_WRONG,
  TEMPLATE_LINK,
} from "./disbursement-import-dialog";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  api.previewDisbursementImport.mockReset();
  api.commitDisbursementImport.mockReset();
  api.downloadDisbursementImportTemplate.mockReset();
  vi.mocked(toast.success).mockClear();
  vi.mocked(toast.error).mockClear();
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

/** Let the awaited mock promises settle and React commit. */
async function settle() {
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
}

function buttonByText(el: ParentNode, text: string): HTMLButtonElement {
  const b = [...el.querySelectorAll<HTMLButtonElement>("button")].find((x) => x.textContent?.trim() === text);
  if (b === undefined) throw new Error(`no button "${text}"`);
  return b;
}

/** Spec 05's confirm button: `Nhập`, then `Nhập {n} lần giải ngân` once a preview is read. */
function importButton(dialog: ParentNode): HTMLButtonElement {
  const b = [...dialog.querySelectorAll<HTMLButtonElement>("button")].find((x) => /^Nhập( \d+ lần giải ngân)?$/.test(x.textContent?.trim() ?? ""));
  if (b === undefined) throw new Error("no import button");
  return b;
}

function openDialog(onImported = vi.fn()) {
  const el = mount(<DisbursementImportButton canImport onImported={onImported} />);
  act(() => buttonByText(el, "Nhập giải ngân").click());
  const dialog = document.body.querySelector<HTMLElement>("dialog")!;
  return { el, dialog, onImported };
}

async function chooseFile(dialog: HTMLElement, name = "giai-ngan.xlsx") {
  const input = dialog.querySelector<HTMLInputElement>('input[type="file"]')!;
  const file = new File(["PK fake"], name);
  Object.defineProperty(input, "files", { value: [file], configurable: true });
  act(() => {
    input.dispatchEvent(new Event("change", { bubbles: true }));
  });
  await settle();
  return file;
}

const VALID = { ok: true, duLieu: { valid: true, rowCount: 3, totalAmount: 1234567890, errors: [] } };
const INVALID = {
  ok: true,
  duLieu: {
    valid: false,
    rowCount: 2,
    totalAmount: 7000000,
    errors: [
      { row: 4, column: "Mã dự án", message: "Không có dự án mã này trong năm." },
      { row: 0, column: "", message: "Thiếu cột Nguồn vốn." },
    ],
  },
};
const CREATED = [
  { row: 2, id: "01JA", project_code: "DA01" },
  { row: 3, id: "01JB", project_code: "DA01" },
  { row: 4, id: "01JC", project_code: "DA02" },
];

describe("Nhập giải ngân — permission gating", () => {
  it("without budget.update NOTHING is drawn — no button, no dialog", () => {
    const el = mount(<DisbursementImportButton canImport={false} onImported={() => {}} />);
    expect(el.textContent).not.toContain("Nhập giải ngân");
    expect(el.querySelector("button")).toBeNull();
    expect(document.body.querySelector("dialog")).toBeNull();
  });

  it("with budget.update the live button opens the §10 modal: title, description, link, drop zone, caution", () => {
    const { dialog } = openDialog();
    expect(dialog.textContent).toContain(IMPORT_TITLE);
    expect(dialog.textContent).toContain(IMPORT_DESCRIPTION);
    expect(dialog.textContent).toContain(TEMPLATE_LINK);
    expect(dialog.textContent).toContain(DROP_ZONE_LABEL);
    expect(dialog.textContent).toContain(DUPLICATE_CAUTION);
    expect(dialog.querySelector("[data-pending]")).toBeNull();
    // Nothing chosen yet: `Nhập` is off, and nothing was sent.
    expect(importButton(dialog).disabled).toBe(true);
    expect(api.previewDisbursementImport).not.toHaveBeenCalled();
  });
});

describe("Nhập giải ngân — preview", () => {
  it("choosing a file checks it AT ONCE; a valid file shows count + full-đồng total and enables Nhập", async () => {
    api.previewDisbursementImport.mockResolvedValue(VALID);
    const { dialog } = openDialog();
    const file = await chooseFile(dialog);
    expect(api.previewDisbursementImport).toHaveBeenCalledWith(file, "giai-ngan.xlsx");
    // Spec 05: the green box and the count on the button.
    expect(dialog.querySelector("[data-preview-summary]")?.textContent).toContain("3 dòng hợp lệ, sẵn sàng nhập");
    expect(importButton(dialog).textContent).toBe("Nhập 3 lần giải ngân");
    expect(importButton(dialog).disabled).toBe(false);
  });

  it("an invalid file renders every error (row, column, message) and Nhập stays DISABLED", async () => {
    api.previewDisbursementImport.mockResolvedValue(INVALID);
    const { dialog } = openDialog();
    await chooseFile(dialog);
    // Two errors on two distinct rows (row 0 = the whole file).
    expect(dialog.querySelector("[data-preview-summary]")?.textContent).toContain("2 dòng sai trên tổng số 2 dòng");
    const rows = [...dialog.querySelectorAll("tbody tr")].map((tr) =>
      [...tr.querySelectorAll("td")].map((td) => td.textContent),
    );
    expect(rows).toEqual([
      ["4", "Mã dự án", "Không có dự án mã này trong năm."],
      ["Cả tệp", "—", "Thiếu cột Nguồn vốn."],
    ]);
    expect(importButton(dialog).disabled).toBe(true);
    act(() => importButton(dialog).click());
    expect(api.commitDisbursementImport).not.toHaveBeenCalled();
  });

  it("a file refusal (413/415) is the server's sentence, verbatim, and Nhập stays off", async () => {
    api.previewDisbursementImport.mockResolvedValue({ ok: false, thongBao: "Tệp lớn hơn 2 MB nên chưa nhận." });
    const { dialog } = openDialog();
    await chooseFile(dialog);
    expect(dialog.querySelector('[role="alert"]')?.textContent).toBe("Tệp lớn hơn 2 MB nên chưa nhận.");
    expect(importButton(dialog).disabled).toBe(true);
  });

  it("a valid file with zero vouchers offers nothing to import", async () => {
    api.previewDisbursementImport.mockResolvedValue({
      ok: true,
      duLieu: { valid: true, rowCount: 0, totalAmount: 0, errors: [] },
    });
    const { dialog } = openDialog();
    await chooseFile(dialog);
    expect(importButton(dialog).disabled).toBe(true);
  });
});

describe("Nhập giải ngân — import", () => {
  it("a retry after a failed send REUSES the key; success re-reads the page once and says the count", async () => {
    api.previewDisbursementImport.mockResolvedValue(VALID);
    api.commitDisbursementImport
      .mockResolvedValueOnce({ ok: false, message: "Không kết nối được máy chủ.", errors: [] })
      .mockResolvedValueOnce({ ok: true, created: CREATED });
    const { dialog, onImported } = openDialog();
    const file = await chooseFile(dialog);

    act(() => importButton(dialog).click());
    await settle();
    expect(toast.error).toHaveBeenCalledWith("Không kết nối được máy chủ.");
    expect(onImported).not.toHaveBeenCalled();

    act(() => importButton(dialog).click());
    await settle();
    expect(api.commitDisbursementImport).toHaveBeenCalledTimes(2);
    const [first, second] = api.commitDisbursementImport.mock.calls as [Blob, string, string][];
    expect(first![0]).toBe(file);
    expect(typeof first![2]).toBe("string");
    expect(first![2]).not.toBe("");
    expect(second![2]).toBe(first![2]);

    expect(onImported).toHaveBeenCalledTimes(1);
    expect(toast.success).toHaveBeenCalledWith("Đã nhập 3 lần giải ngân.");
    // The attempt is over: the dialog closes, as spec 05 does.
    expect(document.body.querySelector("dialog")).toBeNull();
  });

  it("a NEW file is a new attempt: a new key", async () => {
    api.previewDisbursementImport.mockResolvedValue(VALID);
    api.commitDisbursementImport.mockResolvedValue({ ok: false, message: "Lỗi.", errors: [] });
    const { dialog } = openDialog();
    await chooseFile(dialog, "a.xlsx");
    act(() => importButton(dialog).click());
    await settle();
    await chooseFile(dialog, "b.xlsx");
    act(() => importButton(dialog).click());
    await settle();
    const keys = (api.commitDisbursementImport.mock.calls as [Blob, string, string][]).map((c) => c[2]);
    expect(keys).toHaveLength(2);
    expect(keys[1]).not.toBe(keys[0]);
  });

  it("400 import_invalid shows the server's sentence and its row table; nothing re-read", async () => {
    api.previewDisbursementImport.mockResolvedValue(VALID);
    api.commitDisbursementImport.mockResolvedValue({
      ok: false,
      message: "Tệp còn dòng sai.",
      errors: [{ row: 5, column: "Số tiền (đồng)", message: "Phải là số dương." }],
    });
    const { dialog, onImported } = openDialog();
    await chooseFile(dialog);
    act(() => importButton(dialog).click());
    await settle();
    // Row errors = spec 05's sentence as the toast; the rows stay in the red box.
    expect(toast.error).toHaveBeenCalledWith(ROWS_STILL_WRONG);
    expect(dialog.textContent).toContain("Phải là số dương.");
    expect(onImported).not.toHaveBeenCalled();
  });

  it("a replayed success (count unknown) still re-reads and never prints '0'", async () => {
    api.previewDisbursementImport.mockResolvedValue(VALID);
    api.commitDisbursementImport.mockResolvedValue({ ok: true, created: null });
    const { dialog, onImported } = openDialog();
    await chooseFile(dialog);
    act(() => importButton(dialog).click());
    await settle();
    expect(onImported).toHaveBeenCalledTimes(1);
    // The count comes from the preview of the same file — never "0".
    expect(toast.success).toHaveBeenCalledWith("Đã nhập 3 lần giải ngân.");
  });
});

describe("Nhập giải ngân — template", () => {
  it("downloads through the authenticated client and saves it under the server's name", async () => {
    const blob = new Blob(["PK"]);
    api.downloadDisbursementImportTemplate.mockResolvedValue({ ok: true, duLieu: blob });
    const createObjectURL = vi.fn(() => "blob:x");
    const revokeObjectURL = vi.fn();
    // jsdom has no object URLs: a subclass carries them, and `unstubAllGlobals` restores the real `URL`.
    vi.stubGlobal("URL", class extends URL {
      static override createObjectURL = createObjectURL;
      static override revokeObjectURL = revokeObjectURL;
    });
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) {
      expect(this.download).toBe("mau-nhap-giai-ngan.xlsx");
    });
    const { dialog } = openDialog();
    act(() => buttonByText(dialog, TEMPLATE_LINK).click());
    await settle();
    expect(api.downloadDisbursementImportTemplate).toHaveBeenCalledTimes(1);
    expect(createObjectURL).toHaveBeenCalledWith(blob);
    expect(click).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:x");
  });

  it("a refused template download shows the server's sentence", async () => {
    api.downloadDisbursementImportTemplate.mockResolvedValue({
      ok: false,
      thongBao: "Bạn không có quyền xem giải ngân.",
    });
    const { dialog } = openDialog();
    act(() => buttonByText(dialog, TEMPLATE_LINK).click());
    await settle();
    expect(toast.error).toHaveBeenCalledWith("Bạn không có quyền xem giải ngân.");
  });
});
