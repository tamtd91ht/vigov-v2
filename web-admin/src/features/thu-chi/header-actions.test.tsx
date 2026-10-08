// @vitest-environment jsdom
//
// jsdom: the prototype's flow — press, pick, load at once — checked by what goes over the wire (path,
// `year`, the `file` part, a key per pick), the spinner while it uploads, and the toast the clerk reads.
// Permission gating is UX (rule 5); the DENIED cases are checked, not only the allowed one.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { BudgetImportOut } from "@/lib/api/budget-import";

import { SheetSelectionBar } from "./bang-thu-chi"; // vi-name-ok: existing component under test
import {
  BAD_WORKBOOK,
  BudgetImportButton,
  IMPORT_BUTTON,
  NO_FISCAL_SHEET,
  NOT_IMPORTED,
} from "./budget-import-button";

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (s: string) => toastSuccess(s),
    error: (s: string) => toastError(s),
  },
}));

beforeAll(() => {
  (
    globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }
  ).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
  toastSuccess.mockReset();
  toastError.mockReset();
});

function bar(
  canRecord: boolean,
  canConfirm: boolean,
  sheetState: "ready" | "missing" = "ready",
): string {
  return renderToStaticMarkup(
    <SheetSelectionBar
      year={2026}
      anchorYear={2026}
      onYearChange={() => {}}
      kind="chi"
      onKindChange={() => {}}
      sheetState={sheetState}
      canRecord={canRecord}
      canConfirm={canConfirm}
      sheetLock={null}
      busy={false}
      onCreate={() => {}}
      onEdit={() => {}}
      onRemove={() => {}}
      onImported={() => {}}
    />,
  );
}

describe("`Nạp từ Excel` gating (budget.update)", () => {
  it("with budget.update: the button and its hidden .xlsx picker, beside `Lập bảng` when the sheet is missing", () => {
    const html = bar(true, false, "missing");
    expect(html).toContain(IMPORT_BUTTON);
    expect(html).toContain('accept=".xlsx"');
    expect(html).toContain("Lập bảng");
  });

  it("DENIED — read only: no `Nạp từ Excel`, no picker, no action", () => {
    const html = bar(false, false);
    expect(html).not.toContain(IMPORT_BUTTON);
    expect(html).not.toContain('type="file"');
    expect(html).not.toContain("Gỡ");
  });

  it("DENIED — budget.confirm without budget.update: `Gỡ` (an imported sheet included) but no `Nạp từ Excel`", () => {
    const html = bar(false, true);
    expect(html).not.toContain(IMPORT_BUTTON);
    expect(html).toContain('aria-label="Gỡ bảng"');
  });
});

const CHI = {
  kind: "chi",
  line_count: 59,
} as BudgetImportOut["sheets"][number];
const THU = {
  kind: "thu",
  line_count: 31,
} as BudgetImportOut["sheets"][number];

type Call = { url: string; init: RequestInit };

function stub(res: () => Response | Promise<Response>): Call[] {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return res();
    }),
  );
  return calls;
}

function mount(onImported = vi.fn()) {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  let n = 0;
  act(() =>
    r.render(
      <BudgetImportButton
        year={2026}
        onImported={onImported}
        mintKey={() => `khoa-${++n}`}
      />,
    ),
  );
  return onImported;
}

async function pick(name = "bao-cao.xlsx") {
  const input = host!.querySelector<HTMLInputElement>('input[type="file"]')!;
  Object.defineProperty(input, "files", {
    value: [new File(["x"], name)],
    configurable: true,
  });
  await act(async () => {
    input.dispatchEvent(new Event("change", { bubbles: true }));
  });
  for (let i = 0; i < 5; i++) await act(async () => {});
}

function button(): HTMLButtonElement {
  return host!.querySelector<HTMLButtonElement>("button")!;
}

function json(body: unknown, status: number): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("`Nạp từ Excel` — pick and load at once (prototype FiscalReportPanel.tsx:131-153)", () => {
  it("pressing opens the picker and calls nothing", () => {
    const calls = stub(() => json({}, 500));
    mount();
    const input = host!.querySelector<HTMLInputElement>('input[type="file"]')!;
    const click = vi.spyOn(input, "click");
    act(() => button().click());
    expect(click).toHaveBeenCalledTimes(1);
    expect(calls).toHaveLength(0);
  });

  it("a pick POSTs the file to imports?year= with a fresh key; success toasts and selects the first sheet", async () => {
    const calls = stub(() =>
      json(
        {
          valid: true,
          year: 2026,
          source_file: "x",
          sheets: [THU, CHI],
          warnings: [],
          errors: [],
        },
        201,
      ),
    );
    const onImported = mount();
    await pick();
    await pick("lan-2.xlsx");
    expect(calls[0]!.url).toBe("/api/v1/budget-sheets/imports?year=2026");
    expect(calls[0]!.init.method).toBe("POST");
    expect(new Headers(calls[0]!.init.headers).get("Idempotency-Key")).toBe(
      "khoa-1",
    );
    expect(new Headers(calls[1]!.init.headers).get("Idempotency-Key")).toBe(
      "khoa-2",
    );
    expect(((calls[0]!.init.body as FormData).get("file") as File).name).toBe(
      "bao-cao.xlsx",
    );
    expect(toastSuccess).toHaveBeenCalledWith(
      "Đã nạp: Thu ngân sách (31 khoản mục) · Chi ngân sách (59 khoản mục)",
    );
    expect(onImported).toHaveBeenCalledWith("thu");
  });

  it("spins and is disabled while uploading; the input is cleared so the same file can be picked again", async () => {
    let release: (r: Response) => void = () => {};
    stub(() => new Promise<Response>((r) => (release = r)));
    mount();
    const input = host!.querySelector<HTMLInputElement>('input[type="file"]')!;
    Object.defineProperty(input, "files", {
      value: [new File(["x"], "a.xlsx")],
      configurable: true,
    });
    await act(async () => {
      input.dispatchEvent(new Event("change", { bubbles: true }));
    });
    expect(button().disabled).toBe(true);
    expect(button().getAttribute("aria-busy")).toBe("true");
    expect(button().querySelector(".animate-spin")).not.toBeNull();
    expect(input.value).toBe("");
    await act(async () => release(json({ code: "x", replayed: true }, 201)));
    for (let i = 0; i < 5; i++) await act(async () => {});
    expect(button().disabled).toBe(false);
  });
});

describe("`Nạp từ Excel` — failure toasts", () => {
  const cases: [string, Response, string][] = [
    [
      "415 unsupported_file_type → bad_workbook",
      json({ code: "unsupported_file_type", message: "m", trace_id: "" }, 415),
      BAD_WORKBOOK,
    ],
    [
      "400 malformed_file → bad_workbook",
      json({ code: "malformed_file", message: "m", trace_id: "" }, 400),
      BAD_WORKBOOK,
    ],
    [
      "400 import_invalid, one file-level error → no_fiscal_sheet",
      json(
        {
          code: "import_invalid",
          message: "m",
          trace_id: "",
          errors: [
            { sheet: "", row: 0, column: "", message: "Không tìm thấy…" },
          ],
        },
        400,
      ),
      NO_FISCAL_SHEET,
    ],
    [
      "400 import_invalid → first error with where, and how many more",
      json(
        {
          code: "import_invalid",
          message: "m",
          trace_id: "",
          errors: [
            {
              sheet: "Chi NS",
              row: 14,
              column: "Dự toán năm",
              message: "Ô không phải số.",
            },
            { sheet: "Chi NS", row: 27, column: "", message: "x" },
            { sheet: "Thu NS", row: 0, column: "", message: "y" },
          ],
        },
        400,
      ),
      "Sheet Chi NS dòng 14 cột Dự toán năm: Ô không phải số. … và 2 lỗi khác",
    ],
    [
      "409 budget_period_closed → the server's sentence",
      json(
        {
          code: "budget_period_closed",
          message: "ngan_sach: kỳ tháng 9/2026 đã chốt (mã CK-2026-09)",
          trace_id: "",
        },
        409,
      ),
      "Kỳ tháng 9/2026 đã chốt (mã CK-2026-09)",
    ],
    [
      "409 budget_sheet_has_entries → the server's sentence",
      json(
        {
          code: "budget_sheet_has_entries",
          message: "ngan_sach: bảng chi năm 2026 đã có số liệu nhập tay",
          trace_id: "",
        },
        409,
      ),
      "Bảng chi năm 2026 đã có số liệu nhập tay",
    ],
    [
      "anything else → Chưa nạp được tệp.",
      json({ code: "forbidden", message: "m", trace_id: "" }, 403),
      NOT_IMPORTED,
    ],
  ];
  for (const [name, res, want] of cases) {
    it(name, async () => {
      stub(() => res);
      const onImported = mount();
      await pick();
      expect(toastError).toHaveBeenCalledWith(want);
      expect(toastSuccess).not.toHaveBeenCalled();
      expect(onImported).not.toHaveBeenCalled();
    });
  }
});
