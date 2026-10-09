// @vitest-environment jsdom
//
// jsdom: the shared 672px import dialog (spec §1, user 09/10/2026) is behaviour — a file chosen, `Nhập`
// pressed, and the requests that follow read from the stubbed `fetch`. Target: Thôn / Tổ dân phố.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

const { ExcelImportDialog } = await import("./excel-import-dialog");
const { RESIDENTIAL_UNIT_IMPORT_TARGET } = await import("./excel-import-targets");

type Call = { method: string; url: string; key: string | null };
let calls: Call[] = [];
let previewBody: unknown;
let importReply: { status: number; body: unknown };

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

const ROW = {
  row: 2,
  code: "thon-binh-an",
  name: "Thôn Bình An",
  type_code: "thon",
  type_label: "Thôn",
  head_staff_code: "",
  head_staff_name: "",
  household_count: null,
  population_count: null,
  order: 1,
};

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  calls = [];
  importReply = { status: 201, body: { created: [ROW] } };
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit = {}) => {
      const headers = (init.headers ?? {}) as Record<string, string>;
      calls.push({ method: String(init.method ?? "GET"), url, key: headers["Idempotency-Key"] ?? null });
      if (url === RESIDENTIAL_UNIT_IMPORT_TARGET.routes.previews) return json(200, previewBody);
      if (url === RESIDENTIAL_UNIT_IMPORT_TARGET.routes.imports) return json(importReply.status, importReply.body);
      return json(404, { code: "not_found", message: "x" });
    }),
  );
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
});

async function settle() {
  for (let i = 0; i < 8; i++) await act(async () => {});
}

const imported = vi.fn();

function mount(): HTMLDivElement {
  imported.mockReset();
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<ExcelImportDialog target={RESIDENTIAL_UNIT_IMPORT_TARGET} onImported={imported} onClose={() => {}} />));
  return host;
}

function choose(el: HTMLElement) {
  const input = el.ownerDocument.querySelector<HTMLInputElement>('input[type="file"]')!;
  const file = new File(["x"], "thon.xlsx", { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" });
  Object.defineProperty(input, "files", { configurable: true, value: { item: () => file, length: 1, 0: file } });
  act(() => input.dispatchEvent(new Event("change", { bubbles: true })));
}

const submitButton = () => [...document.querySelectorAll("button")].find((b) => b.textContent === "Nhập")!;

describe("shared Excel import dialog — Nhập = check, then write only when clean", () => {
  it("the prototype's frame: title per target, one description, 672px, Tải tệp mẫu, Chọn tệp .xlsx, Nhập disabled until a file", () => {
    const el = mount();
    const dialog = document.querySelector("dialog")!;
    expect(dialog.className).toContain("max-w-[672px]");
    expect(dialog.textContent).toContain("Nhập thôn / tổ dân phố từ Excel");
    expect(dialog.textContent).toContain(
      "Tệp được kiểm trước và chưa ghi gì. Còn một dòng sai thì không dòng nào được nhận — sửa tệp rồi nhập lại.",
    );
    // The long per-target guidance is gone from the dialog.
    expect(dialog.textContent).not.toContain(RESIDENTIAL_UNIT_IMPORT_TARGET.explanation);
    expect(dialog.textContent).toContain("Tải tệp mẫu");
    expect(dialog.textContent).toContain("Chọn tệp .xlsx");
    expect(submitButton().disabled).toBe(true);
    choose(el);
    expect(submitButton().disabled).toBe(false);
  });

  it("errors in the preview → listed by row and column, and NOTHING is written", async () => {
    previewBody = {
      valid: false,
      units: [],
      errors: [{ row: 3, column: "Loại", message: "Loại không có trong danh mục." }],
    };
    const el = mount();
    choose(el);
    act(() => submitButton().click());
    await settle();
    expect(calls.map((c) => c.url)).toEqual([RESIDENTIAL_UNIT_IMPORT_TARGET.routes.previews]);
    const alert = document.querySelector('[role="alert"]')!;
    expect(alert.textContent).toContain(RESIDENTIAL_UNIT_IMPORT_TARGET.errorsHeading);
    expect(alert.textContent).toContain("3");
    expect(alert.textContent).toContain("Loại");
    expect(alert.textContent).toContain("Loại không có trong danh mục.");
    expect(imported).not.toHaveBeenCalled();
    // Nhập stays offered for the fixed file.
    expect(submitButton().disabled).toBe(false);
  });

  it("clean preview → the write follows in the same press, with an Idempotency-Key, and the result shows", async () => {
    previewBody = { valid: true, units: [ROW], errors: [] };
    const el = mount();
    choose(el);
    act(() => submitButton().click());
    await settle();
    expect(calls.map((c) => c.url)).toEqual([
      RESIDENTIAL_UNIT_IMPORT_TARGET.routes.previews,
      RESIDENTIAL_UNIT_IMPORT_TARGET.routes.imports,
    ]);
    expect(calls[1]!.key).toMatch(/.+/);
    expect(imported).toHaveBeenCalledTimes(1);
    expect(document.body.textContent).toContain("Đã nhập 1 thôn / tổ dân phố.");
    // Done: only Đóng is left.
    expect(submitButton()).toBeUndefined();
  });

  it("a write that got NO answer is retried under the SAME key, without a new preview (ADR 0059)", async () => {
    previewBody = { valid: true, units: [ROW], errors: [] };
    const el = mount();
    choose(el);
    type FetchFn = (url: string, init?: RequestInit) => Promise<Response>;
    const fetchMock = vi.mocked(globalThis.fetch as unknown as FetchFn);
    const real = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation(async (url: string, init: RequestInit = {}) => {
      if (url === RESIDENTIAL_UNIT_IMPORT_TARGET.routes.imports && calls.filter((c) => c.url === url).length === 0) {
        const headers = (init.headers ?? {}) as Record<string, string>;
        calls.push({ method: "POST", url, key: headers["Idempotency-Key"] ?? null });
        throw new TypeError("Failed to fetch");
      }
      return real(url, init);
    });
    act(() => submitButton().click());
    await settle();
    act(() => submitButton().click());
    await settle();
    const writes = calls.filter((c) => c.url === RESIDENTIAL_UNIT_IMPORT_TARGET.routes.imports);
    expect(writes).toHaveLength(2);
    expect(writes[1]!.key).toBe(writes[0]!.key);
    expect(calls.filter((c) => c.url === RESIDENTIAL_UNIT_IMPORT_TARGET.routes.previews)).toHaveLength(1);
    expect(imported).toHaveBeenCalledTimes(1);
  });
});
