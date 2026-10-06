// @vitest-environment jsdom
//
// jsdom for this file: ticking rows, the order the DELETE calls leave in, the per-project result and
// the re-read after it are events — a markup string cannot show them.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type { finance_duAnRa } from "@/lib/api/schema.gen";

import { BangDuAn } from "./bang-du-an";
import { PROJECT_REMOVE_NOTE, ProjectRemoveDialog } from "./ghi-du-an";
import { deleteProjectsInTurn, type SelectedProject } from "./project-bulk-delete";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  if (!("ResizeObserver" in globalThis)) {
    (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
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
  for (let i = 0; i < 8; i++) {
    await act(async () => {
      await Promise.resolve();
    });
  }
}

function buttonByText(el: ParentNode, text: string): HTMLButtonElement | undefined {
  return [...el.querySelectorAll<HTMLButtonElement>("button")].find((b) => b.textContent?.trim() === text);
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

type Call = { url: string; method: string; body: unknown; rawBody: unknown };

function stubServer(handler: (c: Call) => Response | Promise<Response>): Call[] {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const c: Call = {
        url,
        method: init?.method ?? "GET",
        body: typeof init?.body === "string" ? JSON.parse(init.body) : undefined,
        rawBody: init?.body,
      };
      calls.push(c);
      return handler(c);
    }),
  );
  return calls;
}

function project(id: string, code: string, name: string, planned = 100_000_000): finance_duAnRa {
  return {
    id,
    code,
    year: 2026,
    category_id: "",
    name,
    planned_amount: planned,
    approved_amount: planned,
    disbursed_amount: 0,
    remaining_amount: planned,
    disbursed_ratio: 0,
    delay_score: null,
    is_delayed: false,
    disbursement_deadline: "2026-12-31",
    delay_threshold: 1000,
    delay_threshold_source: "mac_dinh",
  };
}

const PROJECTS = [
  project("01JP1", "DA01", "Bê tông hoá đường ngõ xóm tổ 6", 100_000_000),
  project("01JP2", "DA02", "Nhà văn hoá thôn Đông", 250_000_000),
  project("01JP3", "DA03", "Kè chống sạt lở suối Cạn", 900_000_000),
];

const LOCKED = "Dự án này có chứng từ đã khoá nên chưa xoá được. Hãy mở khoá chứng từ trước.";

const listGets = (calls: Call[]) =>
  calls.filter((c) => c.method === "GET" && c.url.startsWith("/api/v1/investment-projects?"));
const deletes = (calls: Call[]) => calls.filter((c) => c.method === "DELETE");

/** Lists PROJECTS for 2026; DA02's delete is refused 409, the others answer 204. */
function server(): Call[] {
  return stubServer((c) => {
    if (c.method === "GET") return json({ items: PROJECTS, year: 2026, delay_threshold: 1000, delay_threshold_source: "mac_dinh" }, 200);
    if (c.url.endsWith("/01JP2")) return json({ code: "project_has_locked_vouchers", message: LOCKED, trace_id: "t" }, 409);
    return new Response(null, { status: 204 });
  });
}

async function register(canDelete: boolean): Promise<HTMLDivElement> {
  const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} canDelete={canDelete} />);
  await settle();
  return el;
}

const rowBoxes = (el: ParentNode) =>
  [...el.querySelectorAll<HTMLInputElement>('input[type="checkbox"][aria-label^="Chọn dự án "]')];
const selectAll = (el: ParentNode) =>
  el.querySelector<HTMLInputElement>('input[type="checkbox"][aria-label="Chọn tất cả dự án đang hiện"]');

describe("deleteProjectsInTurn — the pure run", () => {
  it("one call at a time, in order, and never stops at a refusal", async () => {
    let inFlight = 0;
    let maxInFlight = 0;
    const order: string[] = [];
    const softDelete = async (id: string): Promise<KetQua<null>> => {
      inFlight += 1;
      maxInFlight = Math.max(maxInFlight, inFlight);
      order.push(id);
      await new Promise((r) => setTimeout(r, 1));
      inFlight -= 1;
      return id === "01JP2" ? { ok: false, thongBao: LOCKED } as KetQua<null> : { ok: true, duLieu: null };
    };
    const progress: number[] = [];

    const results = await deleteProjectsInTurn(PROJECTS as SelectedProject[], softDelete, (n) => progress.push(n));

    expect(maxInFlight).toBe(1);
    expect(order).toEqual(["01JP1", "01JP2", "01JP3"]);
    expect(progress).toEqual([1, 2, 3]);
    expect(results).toEqual([
      { code: "DA01", ok: true },
      { code: "DA02", ok: false, message: LOCKED },
      { code: "DA03", ok: true },
    ]);
  });
});

describe("Project register — selection gated on budget.update", () => {
  it("without the key: no checkbox column, no select-all, no bar", async () => {
    server();
    const el = await register(false);
    expect(el.querySelectorAll("tbody tr")).toHaveLength(3);
    expect(el.querySelectorAll('input[type="checkbox"]:not([disabled])')).toHaveLength(0);
    expect(selectAll(el)).toBeNull();
    expect(el.querySelector("[data-bulk-bar]")).toBeNull();
  });

  it("with the key: one box per row and a select-all; the bar appears once something is ticked", async () => {
    server();
    const el = await register(true);
    expect(rowBoxes(el)).toHaveLength(3);
    expect(selectAll(el)).not.toBeNull();
    expect(el.querySelector("[data-bulk-bar]")).toBeNull();

    act(() => rowBoxes(el)[1]!.click());
    expect(el.querySelector("[data-bulk-bar]")?.textContent).toContain("Đã chọn 1 dự án");
    expect(selectAll(el)!.checked).toBe(false);
    expect(selectAll(el)!.indeterminate).toBe(true);
  });

  it("select-all ticks every row on screen, and a second press unticks them all", async () => {
    server();
    const el = await register(true);

    act(() => selectAll(el)!.click());
    expect(rowBoxes(el).every((b) => b.checked)).toBe(true);
    expect(el.querySelector("[data-bulk-bar]")?.textContent).toContain("Đã chọn 3 dự án");

    act(() => selectAll(el)!.click());
    expect(rowBoxes(el).some((b) => b.checked)).toBe(false);
    expect(el.querySelector("[data-bulk-bar]")).toBeNull();
  });

  it("select-all with a search keyword selects only the rows the keyword leaves on screen", async () => {
    server();
    const el = await register(true);
    typeInto(el.querySelector<HTMLInputElement>("#tim-du-an")!, "DA03");
    expect(rowBoxes(el)).toHaveLength(1);

    act(() => selectAll(el)!.click());
    expect(el.querySelector("[data-bulk-bar]")?.textContent).toContain("Đã chọn 1 dự án");
  });
});

describe("Xoá đã chọn — confirm, sequential DELETEs, per-row result, re-read", () => {
  it("lists name · code · plan, deletes one by one WITHOUT a reason, reports the refusal verbatim, re-reads", async () => {
    const calls = server();
    const el = await register(true);
    expect(listGets(calls)).toHaveLength(1);

    act(() => selectAll(el)!.click());
    act(() => buttonByText(el.querySelector("[data-bulk-bar]")!, "Xoá đã chọn")!.click());

    const dialog = el.querySelector("dialog")!;
    expect(dialog.textContent).toContain("Xoá 3 dự án khỏi danh sách?");
    const listed = [...dialog.querySelectorAll('ul[aria-label="Các dự án sẽ xoá"] li')].map((li) => li.textContent);
    expect(listed).toEqual([
      "Bê tông hoá đường ngõ xóm tổ 6DA01 · 100.000.000 đ",
      "Nhà văn hoá thôn ĐôngDA02 · 250.000.000 đ",
      "Kè chống sạt lở suối CạnDA03 · 900.000.000 đ",
    ]);
    // No reason is asked (user decision 06/10/2026).
    expect(dialog.querySelector("input, textarea")).toBeNull();
    expect(deletes(calls)).toHaveLength(0);

    act(() => buttonByText(dialog, "Xoá 3 dự án")!.click());
    await settle();

    expect(deletes(calls).map((c) => c.url)).toEqual([
      "/api/v1/investment-projects/01JP1",
      "/api/v1/investment-projects/01JP2",
      "/api/v1/investment-projects/01JP3",
    ]);
    for (const d of deletes(calls)) expect(d.body).toEqual({});

    const results = [...el.querySelectorAll('ul[aria-label="Kết quả xoá từng dự án"] li')];
    expect(results.map((li) => li.textContent)).toEqual([
      "DA01 — đã xoá.",
      `DA02 — chưa xoá: ${LOCKED}`,
      "DA03 — đã xoá.",
    ]);
    expect(el.querySelector('dialog [role="status"]')?.textContent).toContain("Đã xoá 2/3 dự án; 1 dự án chưa xoá được");
    // The list is read again, and the selection made on the old list is gone.
    expect(listGets(calls)).toHaveLength(2);
    expect(el.querySelector("[data-bulk-bar]")).toBeNull();

    act(() => buttonByText(el.querySelector("dialog")!, "Đóng")!.click());
    expect(el.querySelector("dialog")).toBeNull();
  });

  it("the second DELETE leaves only after the first has answered", async () => {
    let release: (() => void) | null = null;
    const calls = stubServer((c) => {
      if (c.method === "GET") return json({ items: PROJECTS.slice(0, 2), year: 2026, delay_threshold: 1000, delay_threshold_source: "mac_dinh" }, 200);
      if (c.url.endsWith("/01JP1")) return new Promise<Response>((r) => (release = () => r(new Response(null, { status: 204 }))));
      return new Response(null, { status: 204 });
    });
    const el = mount(<BangDuAn nam={2026} danhMuc={[]} reloadSignal={0} canDelete />);
    await settle();

    act(() => selectAll(el)!.click());
    act(() => buttonByText(el.querySelector("[data-bulk-bar]")!, "Xoá đã chọn")!.click());
    act(() => buttonByText(el.querySelector("dialog")!, "Xoá 2 dự án")!.click());
    await settle();

    expect(deletes(calls)).toHaveLength(1);
    expect(el.querySelector('dialog [role="status"]')?.textContent).toContain("Đang xoá 0/2");
    await act(async () => release!());
    await settle();
    expect(deletes(calls)).toHaveLength(2);
  });
});

describe("Gỡ dự án (detail page) — confirmation without a reason", () => {
  it("no reason field; confirming sends DELETE with `{}`; success closes and reports", async () => {
    const calls = stubServer(() => new Response(null, { status: 204 }));
    const onClose = vi.fn();
    const onRemoved = vi.fn();
    const el = mount(<ProjectRemoveDialog duAn={PROJECTS[0]!} onClose={onClose} onRemoved={onRemoved} />);

    expect(el.textContent).toContain("Gỡ dự án “Bê tông hoá đường ngõ xóm tổ 6”?");
    expect(el.textContent).toContain(PROJECT_REMOVE_NOTE);
    expect(el.querySelector("input, textarea")).toBeNull();
    const confirm = buttonByText(el, "Gỡ dự án")!;
    expect(confirm.disabled).toBe(false);

    act(() => confirm.click());
    await settle();

    expect(calls).toHaveLength(1);
    const [call] = calls;
    expect(call?.method).toBe("DELETE");
    expect(call?.url).toBe("/api/v1/investment-projects/01JP1");
    expect(call?.rawBody).toBe("{}");
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onRemoved).toHaveBeenCalledTimes(1);
  });

  it("a refusal stays on screen as the server's sentence; nothing reported removed", async () => {
    const sentence = "Tài khoản của bạn không có quyền xoá dự án.";
    stubServer(() => json({ code: "forbidden", message: sentence, trace_id: "t" }, 403));
    const onRemoved = vi.fn();
    const el = mount(<ProjectRemoveDialog duAn={PROJECTS[0]!} onClose={() => {}} onRemoved={onRemoved} />);

    act(() => buttonByText(el, "Gỡ dự án")!.click());
    await settle();

    expect(el.querySelector('[role="alert"]')?.textContent).toBe(sentence);
    expect(onRemoved).not.toHaveBeenCalled();
  });
});
