// @vitest-environment jsdom
//
// jsdom for this file: the checkbox is ticked and unticked, and the POST body that leaves the browser
// is the thing under test — events and a captured request, not a markup string.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { toast } from "sonner";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

// Outcomes are toasts (ADR 0068 lần 6 #4).
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import type { finance_hangMucRa } from "@/lib/api/schema.gen";

import { KhoiThemDuAn } from "./ghi-du-an";
import { CAU_THIEU_MA_DU_AN } from "./nhan-ghi-giai-ngan";

/**
 * §9 `☑ Tự sinh mã` (server 9f0a0187: `code` absent or blank → the next DA-number of the commune).
 *
 * THE EXPENSIVE CASE IS THE UNTICKED BOX LEFT BLANK. The server reads a blank `code` as "issue one", so
 * a form that sent it would hand a clerk who chose to type a code an auto-issued one — and an issued
 * code is never taken back (rule 7). The form refuses locally instead.
 */

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

const CATEGORIES: finance_hangMucRa[] = [
  { id: "01JHM1", code: "chuyen-tiep", label: "Chuyển tiếp", is_default: true, active: true, order: 1, source: "he-thong", tier: 1 },
];

type Captured = { posts: Record<string, unknown>[] };

/** Fake server: an empty funding catalogue on GET, and the POST answered with `postReply`. */
function stubServer(postReply: () => Response): Captured {
  const captured: Captured = { posts: [] };
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      if ((init?.method ?? "GET") === "GET" && url.startsWith("/api/v1/funding-sources?year=2026")) {
        return new Response(JSON.stringify({ year: 2026, items: [], unattributed_disbursed_amount: 0 }), { status: 200 });
      }
      if (init?.method === "POST" && url === "/api/v1/investment-projects") {
        captured.posts.push(JSON.parse(String(init.body)) as Record<string, unknown>);
        return postReply();
      }
      return new Response("unexpected", { status: 500 });
    }),
  );
  return captured;
}

const CREATED = () =>
  new Response(
    JSON.stringify({
      id: "01JDA9",
      code: "DA03",
      year: 2026,
      category_id: "01JHM1",
      name: "Nhà văn hoá thôn 2",
      planned_amount: 500_000_000,
      approved_amount: 500_000_000,
      approved_amount_set: false,
      disbursement_deadline: "2026-12-31",
      funding_allocated_total: 0,
    }),
    { status: 201 },
  );

function enter(el: HTMLInputElement | HTMLSelectElement, value: string): void {
  const proto = el instanceof HTMLSelectElement ? HTMLSelectElement.prototype : HTMLInputElement.prototype;
  act(() => {
    Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
    el.dispatchEvent(new Event(el instanceof HTMLSelectElement ? "change" : "input", { bubbles: true }));
  });
}

/** Mounts the header button, opens the dialog, fills the three required fields other than the code. */
async function openFilled(onSaved = vi.fn()): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<KhoiThemDuAn nam={2026} danhMuc={CATEGORIES} coGhi daGhiXong={onSaved} />));
  act(() => host!.querySelector<HTMLButtonElement>('button[aria-haspopup="dialog"]')!.click());
  await act(async () => {}); // the funding catalogue arrives
  enter(host.querySelector<HTMLSelectElement>("#hang-muc-du-an")!, "01JHM1");
  enter(host.querySelector<HTMLInputElement>("#ten-du-an")!, "Nhà văn hoá thôn 2");
  enter(host.querySelector<HTMLInputElement>("#ke-hoach-von-du-an")!, "500000000");
  return host;
}

async function submit(el: HTMLElement): Promise<void> {
  await act(async () => {
    el.querySelector<HTMLButtonElement>('dialog button[type="submit"]')!.click();
  });
  await act(async () => {});
}

function autoBox(el: HTMLElement): HTMLInputElement {
  return el.querySelector<HTMLInputElement>("#tu-sinh-ma-du-an")!;
}

describe("Thêm dự án — opening focus (brief §3.4, prototype autoFocus)", () => {
  function openWith(danhMuc: finance_hangMucRa[]): HTMLDivElement {
    stubServer(CREATED);
    host = document.createElement("div");
    document.body.append(host);
    const r = createRoot(host);
    root = r;
    act(() => r.render(<KhoiThemDuAn nam={2026} danhMuc={danhMuc} coGhi daGhiXong={vi.fn()} />));
    act(() => host!.querySelector<HTMLButtonElement>('button[aria-haspopup="dialog"]')!.click());
    return host;
  }

  it("the dialog opens with the focus on `Hạng mục`", () => {
    openWith(CATEGORIES);
    expect(document.activeElement?.id).toBe("hang-muc-du-an");
  });

  it("empty catalogue → the select is disabled and is not forced to take the focus", () => {
    const el = openWith([]);
    expect(el.querySelector<HTMLSelectElement>("#hang-muc-du-an")!.disabled).toBe(true);
    expect(document.activeElement?.id).not.toBe("hang-muc-du-an");
  });
});

describe("§9 Tự sinh mã — what leaves the browser", () => {
  it("default: checked, code box disabled, and the POST carries NO `code`", async () => {
    const captured = stubServer(CREATED);
    const onSaved = vi.fn();
    const el = await openFilled(onSaved);

    expect(autoBox(el).checked).toBe(true);
    const code = el.querySelector<HTMLInputElement>("#ma-du-an")!;
    expect(code.disabled).toBe(true);
    expect(code.placeholder).toBe("Hệ thống sẽ tự sinh");

    await submit(el);
    expect(captured.posts).toHaveLength(1);
    expect(captured.posts[0]).not.toHaveProperty("code");
    expect(captured.posts[0]).toMatchObject({ year: 2026, category_id: "01JHM1", planned_amount: 500_000_000 });
    expect(onSaved).toHaveBeenCalledOnce();
  });

  it("unchecked: a blank code is refused at the form (nothing sent); a typed one is sent", async () => {
    const captured = stubServer(CREATED);
    const el = await openFilled();

    act(() => autoBox(el).click());
    expect(autoBox(el).checked).toBe(false);
    const code = el.querySelector<HTMLInputElement>("#ma-du-an")!;
    expect(code.disabled).toBe(false);

    await submit(el);
    expect(captured.posts).toHaveLength(0);
    expect(el.querySelector('[role="alert"]')?.textContent).toBe(CAU_THIEU_MA_DU_AN);

    enter(code, " DA-07 ");
    await submit(el);
    expect(captured.posts).toHaveLength(1);
    expect(captured.posts[0]!.code).toBe("DA-07");
  });

  it("409 code_taken: the server's sentence shows, the dialog and the typed code stay", async () => {
    const sentence = "Mã dự án này đã được dùng, kể cả bởi dự án đã rút khỏi danh sách. Hãy chọn mã khác.";
    stubServer(() => new Response(JSON.stringify({ code: "code_taken", message: sentence }), { status: 409 }));
    const onSaved = vi.fn();
    const el = await openFilled(onSaved);

    act(() => autoBox(el).click());
    enter(el.querySelector<HTMLInputElement>("#ma-du-an")!, "DA01");
    await submit(el);

    expect(toast.error).toHaveBeenCalledWith(sentence);
    expect(el.querySelector<HTMLInputElement>("#ma-du-an")!.value).toBe("DA01");
    expect(onSaved).not.toHaveBeenCalled();
  });
});
