// @vitest-environment jsdom
//
// jsdom: choosing a file and pressing Nhập are events; what is checked is what leaves the browser —
// a multipart part `file` on both POSTs, no key on the check, ONE key on the import.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { CitizenContactImportDialog } from "./citizen-contact-import-dialog";
import { IMPORT_FILE_INPUT_ID, IMPORT_TEMPLATE_BUTTON, IMPORT_TITLE } from "./citizen-directory-words";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

type Call = { url: string; init: RequestInit | undefined };
const calls: Call[] = [];
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

function serve(commitStatus: 201 | 400 = 201) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url === "/api/v1/citizen-contact-import-previews") return json({ valid: true, row_count: 2, errors: [] });
      if (url === "/api/v1/citizen-contact-imports") {
        return commitStatus === 201
          ? json({ batch_id: "01JBATCH", row_count: 2 }, 201)
          : json({ code: "import_invalid", message: "Tệp có lỗi.", trace_id: "", errors: [{ row: 3, column: "Số điện thoại", message: "Số này đã có trong danh bạ." }] }, 400);
      }
      return json({ code: "not_found", message: "Không có tuyến này.", trace_id: "" }, 404);
    }),
  );
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  calls.length = 0;
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

const settle = () => act(async () => {});

async function mount(onImported = () => {}, onClose = () => {}): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => {
    root!.render(<CitizenContactImportDialog onClose={onClose} onImported={onImported} />);
  });
  return host;
}

/** Put a file in the hidden input and fire `change`, as the picker would. */
async function choose(el: HTMLElement, file: File) {
  const input = el.querySelector<HTMLInputElement>(`#${IMPORT_FILE_INPUT_ID}`)!;
  const list = { 0: file, length: 1, item: (i: number) => (i === 0 ? file : null) } as unknown as FileList;
  Object.defineProperty(input, "files", { value: list, configurable: true });
  await act(async () => {
    input.dispatchEvent(new Event("change", { bubbles: true }));
  });
  await settle();
  await settle();
}

const buttonNamed = (el: HTMLElement, text: string) => [...el.querySelectorAll("button")].find((b) => b.textContent === text) ?? null;
const FILE = new File(["PK"], "danh-ba-nguoi-dan.xlsx");

describe("Nhập danh bạ người dân từ Excel", () => {
  it("the title and the template link; nothing sent until a file is chosen", async () => {
    serve();
    const el = await mount();
    expect(el.textContent).toContain(IMPORT_TITLE);
    expect(buttonNamed(el, IMPORT_TEMPLATE_BUTTON)).not.toBeNull();
    expect(buttonNamed(el, "Nhập")!.disabled).toBe(true);
    expect(calls).toHaveLength(0);
  });

  it("choosing a file POSTs the preview as FormData with part `file`, no key; a clean check enables `Nhập 2 người dân`", async () => {
    serve();
    const el = await mount();
    await choose(el, FILE);
    const preview = calls.find((c) => c.url === "/api/v1/citizen-contact-import-previews");
    expect(preview).toBeDefined();
    expect(preview!.init?.method).toBe("POST");
    expect(preview!.init?.body).toBeInstanceOf(FormData);
    expect((preview!.init?.body as FormData).get("file")).toBeInstanceOf(Blob);
    expect((preview!.init?.headers as Record<string, string> | undefined)?.["Idempotency-Key"]).toBeUndefined();
    expect(el.textContent).toContain("2 dòng hợp lệ, sẵn sàng nhập");
    expect(buttonNamed(el, "Nhập 2 người dân")!.disabled).toBe(false);
  });

  it("Nhập POSTs the import as FormData WITH an Idempotency-Key; 201 → onImported and the dialog closes", async () => {
    serve();
    const onImported = vi.fn();
    const onClose = vi.fn();
    const el = await mount(onImported, onClose);
    await choose(el, FILE);
    await act(async () => buttonNamed(el, "Nhập 2 người dân")!.click());
    await settle();
    const commit = calls.find((c) => c.url === "/api/v1/citizen-contact-imports");
    expect(commit).toBeDefined();
    expect(commit!.init?.method).toBe("POST");
    expect(commit!.init?.body).toBeInstanceOf(FormData);
    expect((commit!.init?.body as FormData).get("file")).toBeInstanceOf(Blob);
    const key = (commit!.init?.headers as Record<string, string>)["Idempotency-Key"];
    expect(key).toMatch(/^[0-9a-f-]{36}$/);
    expect(onImported).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("400 import_invalid: the refused rows replace the clean check — row and column, never a value — and Nhập is held", async () => {
    serve(400);
    const onImported = vi.fn();
    const el = await mount(onImported);
    await choose(el, FILE);
    await act(async () => buttonNamed(el, "Nhập 2 người dân")!.click());
    await settle();
    expect(onImported).not.toHaveBeenCalled();
    expect(el.textContent).toContain("Số điện thoại");
    expect(el.textContent).toContain("Số này đã có trong danh bạ.");
    expect(buttonNamed(el, "Nhập")!.disabled).toBe(true);
  });

  it("a file that is not .xlsx is refused BEFORE anything is sent", async () => {
    serve();
    const el = await mount();
    await choose(el, new File(["x"], "danh-ba.csv"));
    expect(calls).toHaveLength(0);
    expect(el.querySelector('[role="alert"]')?.textContent).toContain(".xlsx");
  });
});
