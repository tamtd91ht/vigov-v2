// @vitest-environment jsdom
//
// jsdom for this file: what matters is WHICH request each press sends, that the list is re-read and
// the page told after a write, and that nothing is drawn without `budget.update` — events a markup
// string cannot show.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { finance_hangMucRa } from "@/lib/api/schema.gen";

import { CategoryManagerButton } from "./category-manager-dialog";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

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

async function settle(): Promise<void> {
  for (let i = 0; i < 6; i++) {
    await act(async () => {
      await Promise.resolve();
    });
  }
}

function buttonByText(el: ParentNode, text: string): HTMLButtonElement | undefined {
  return [...el.querySelectorAll<HTMLButtonElement>("button")].find((b) => b.textContent?.trim() === text);
}

function buttonByLabel(el: ParentNode, label: string): HTMLButtonElement | undefined {
  return el.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`) ?? undefined;
}

function typeInto(input: HTMLInputElement, value: string): void {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
  act(() => {
    setter.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

function json(body: unknown, status: number): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

type Call = { url: string; method: string; body: unknown; headers: Headers };

function stubServer(handler: (c: Call) => Response): Call[] {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const c: Call = {
        url,
        method: init?.method ?? "GET",
        body: typeof init?.body === "string" ? JSON.parse(init.body) : undefined,
        headers: new Headers(init?.headers),
      };
      calls.push(c);
      return handler(c);
    }),
  );
  return calls;
}

function category(over: Partial<finance_hangMucRa> = {}): finance_hangMucRa {
  return {
    id: "01JHM1",
    code: "von-tra-no",
    label: "Vốn trả nợ",
    is_default: false,
    active: true,
    order: 1,
    source: "don-vi",
    tier: 1,
    ...over,
  };
}

const ROWS: finance_hangMucRa[] = [
  category(),
  category({ id: "01JHM2", code: "xay-dung-moi", label: "Công trình xây dựng mới", source: "he-thong", tier: 2, order: 2 }),
  category({ id: "01JHM3", code: "keo-dai", label: "Vốn kéo dài", active: false, order: 5 }),
];

const LIST = "/api/v1/capital-plan-categories";
const gets = (calls: Call[]) => calls.filter((c) => c.method === "GET" && c.url === LIST);

/** A server that lists ROWS and answers every write with `write`. */
function server(write: (c: Call) => Response = () => json(category(), 200)): Call[] {
  return stubServer((c) => (c.method === "GET" ? json({ items: ROWS }, 200) : write(c)));
}

async function openDialog(onChanged = () => {}): Promise<HTMLDivElement> {
  const el = mount(<CategoryManagerButton canManage onChanged={onChanged} />);
  act(() => buttonByText(el, "Hạng mục")!.click());
  await settle();
  return el;
}

function row(el: ParentNode, code: string): HTMLElement {
  return el.querySelector<HTMLElement>(`[data-category-row="${code}"]`)!;
}

describe("☰ Hạng mục — permission gating (budget.update)", () => {
  it("without the key: nothing is drawn, nothing is read", () => {
    const calls = server();
    const el = mount(<CategoryManagerButton canManage={false} onChanged={() => {}} />);
    expect(el.innerHTML).toBe("");
    expect(calls).toHaveLength(0);
  });

  it("with the key: the button, and the dialog only after it is pressed", async () => {
    const calls = server();
    const el = mount(<CategoryManagerButton canManage onChanged={() => {}} />);
    const button = buttonByText(el, "Hạng mục");
    expect(button).toBeDefined();
    expect(button!.getAttribute("aria-haspopup")).toBe("dialog");
    expect(el.querySelector("dialog")).toBeNull();
    expect(calls).toHaveLength(0);

    act(() => button!.click());
    await settle();
    expect(el.querySelector("dialog")).not.toBeNull();
    expect(gets(calls)).toHaveLength(1);
  });
});

describe("☰ Hạng mục — the list and the tier rules", () => {
  it("every row in the server's order, turned-off rows included; controls by tier", async () => {
    server();
    const el = await openDialog();

    expect([...el.querySelectorAll("[data-category-row]")].map((r) => r.getAttribute("data-category-row"))).toEqual([
      "von-tra-no",
      "xay-dung-moi",
      "keo-dai",
    ]);

    // Tier 1, active: Tắt + delete.
    const own = row(el, "von-tra-no");
    expect(buttonByLabel(own, "Tắt hạng mục Vốn trả nợ")).toBeDefined();
    expect(buttonByLabel(own, "Xoá hạng mục Vốn trả nợ")).toBeDefined();
    expect(own.textContent).not.toContain("Đi kèm phần mềm");

    // Tier 2: can turn off, cannot delete, badge says why.
    const shipped = row(el, "xay-dung-moi");
    expect(buttonByLabel(shipped, "Tắt hạng mục Công trình xây dựng mới")).toBeDefined();
    expect(buttonByLabel(shipped, "Xoá hạng mục Công trình xây dựng mới")).toBeUndefined();
    expect(shipped.textContent).toContain("Đi kèm phần mềm");

    // Turned off: Bật lại, no Tắt.
    const off = row(el, "keo-dai");
    expect(off.textContent).toContain("Đã tắt");
    expect(buttonByLabel(off, "Bật lại hạng mục Vốn kéo dài")).toBeDefined();
    expect(buttonByLabel(off, "Tắt hạng mục Vốn kéo dài")).toBeUndefined();
  });
});

describe("☰ Hạng mục — writes", () => {
  it("add: POST with code, label, order = last + 1 and an Idempotency-Key; then re-read and tell the page", async () => {
    const onChanged = vi.fn();
    const calls = server(() => json(category({ id: "01JHM9" }), 201));
    const el = await openDialog(onChanged);

    const add = buttonByText(el, "Thêm")!;
    expect(add.disabled).toBe(true);
    typeInto(el.querySelector<HTMLInputElement>("#ma-hang-muc-moi")!, " von-su-nghiep ");
    typeInto(el.querySelector<HTMLInputElement>("#ten-hang-muc-moi")!, "Vốn sự nghiệp có tính chất đầu tư");
    act(() => buttonByText(el, "Thêm")!.click());
    await settle();

    const post = calls.find((c) => c.method === "POST")!;
    expect(post.url).toBe(LIST);
    expect(post.body).toEqual({ code: "von-su-nghiep", label: "Vốn sự nghiệp có tính chất đầu tư", order: 6 });
    expect(post.headers.get("Idempotency-Key")).toBeTruthy();
    expect(gets(calls)).toHaveLength(2);
    expect(onChanged).toHaveBeenCalledTimes(1);
    expect(el.querySelector('[role="status"]')?.textContent).toContain("Đã thêm hạng mục");
    // Cleared for the next one.
    expect(el.querySelector<HTMLInputElement>("#ten-hang-muc-moi")!.value).toBe("");
  });

  it("rename: leaving the box sends PATCH with the label only", async () => {
    const onChanged = vi.fn();
    const calls = server();
    const el = await openDialog(onChanged);

    const input = el.querySelector<HTMLInputElement>("#nhan-hang-muc-01JHM1")!;
    act(() => input.focus());
    typeInto(input, "Vốn trả nợ đọng XDCB");
    act(() => input.blur());
    await settle();

    const patch = calls.find((c) => c.method === "PATCH")!;
    expect(patch.url).toBe(`${LIST}/01JHM1`);
    expect(patch.body).toEqual({ label: "Vốn trả nợ đọng XDCB" });
    expect(onChanged).toHaveBeenCalledTimes(1);
  });

  it("rename: an unchanged or blank label sends nothing", async () => {
    const calls = server();
    const el = await openDialog();
    const input = el.querySelector<HTMLInputElement>("#nhan-hang-muc-01JHM1")!;
    act(() => input.focus());
    typeInto(input, "   ");
    act(() => input.blur());
    await settle();
    expect(calls.filter((c) => c.method !== "GET")).toHaveLength(0);
    expect(input.value).toBe("Vốn trả nợ");
  });

  it("turn off / back on: PATCH with `active` only", async () => {
    const calls = server();
    const el = await openDialog();

    act(() => buttonByLabel(el, "Tắt hạng mục Vốn trả nợ")!.click());
    await settle();
    act(() => buttonByLabel(el, "Bật lại hạng mục Vốn kéo dài")!.click());
    await settle();

    const patches = calls.filter((c) => c.method === "PATCH");
    expect(patches.map((c) => [c.url, c.body])).toEqual([
      [`${LIST}/01JHM1`, { active: false }],
      [`${LIST}/01JHM3`, { active: true }],
    ]);
  });

  it("delete: asks a reason (the category route requires one); blank sends nothing; then DELETE with it", async () => {
    const onChanged = vi.fn();
    const calls = server(() => new Response(null, { status: 204 }));
    const el = await openDialog(onChanged);

    act(() => buttonByLabel(el, "Xoá hạng mục Vốn trả nợ")!.click());
    const form = el.querySelector<HTMLFormElement>('form[aria-label="Xoá hạng mục Vốn trả nợ"]')!;
    expect(form).not.toBeNull();

    act(() => buttonByText(form, "Xoá hạng mục")!.click());
    await settle();
    expect(calls.filter((c) => c.method === "DELETE")).toHaveLength(0);
    expect(form.querySelector('[role="alert"]')?.textContent).toContain("Vui lòng nhập lý do xoá");

    typeInto(form.querySelector<HTMLInputElement>("#ly-do-xoa-hang-muc-01JHM1")!, "Nhập trùng");
    act(() => buttonByText(form, "Xoá hạng mục")!.click());
    await settle();

    const del = calls.find((c) => c.method === "DELETE")!;
    expect(del.url).toBe(`${LIST}/01JHM1`);
    expect(del.body).toEqual({ reason: "Nhập trùng" });
    expect(onChanged).toHaveBeenCalledTimes(1);
  });

  it("a refusal (e.g. 403 until the route takes budget.update) is the server's sentence, verbatim; nothing re-read", async () => {
    const onChanged = vi.fn();
    const sentence = "Tài khoản của bạn không có quyền thực hiện thao tác này.";
    const calls = server(() => json({ code: "forbidden", message: sentence, trace_id: "t1" }, 403));
    const el = await openDialog(onChanged);

    act(() => buttonByLabel(el, "Tắt hạng mục Vốn trả nợ")!.click());
    await settle();

    expect(el.querySelector('dialog [role="alert"]')?.textContent).toBe(sentence);
    expect(onChanged).not.toHaveBeenCalled();
    expect(gets(calls)).toHaveLength(1);
  });
});
