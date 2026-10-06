// @vitest-environment jsdom
//
// jsdom for this file: the dialogs are driven by clicks and typed values, and what matters is WHICH
// request each press sends and that the cards are re-read after it — events a markup string cannot show.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { finance_fundingSourceOut, finance_fundingSourcesOut } from "@/lib/api/schema.gen";

import { GRANTED_AMOUNT_MISSING, SOURCE_NAME_MISSING } from "./funding-source-dialogs";
import { FundingSourceCards, FundingSourceProgress } from "./funding-source-progress";

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

/** Let pending fetch promises resolve and React commit what they set. */
async function settle(): Promise<void> {
  for (let i = 0; i < 5; i++) {
    await act(async () => {
      await Promise.resolve();
    });
  }
}

function source(over: Partial<finance_fundingSourceOut> = {}): finance_fundingSourceOut {
  return {
    id: "01JSRC1",
    name: "Ngân sách xã, phường",
    order: 1,
    year: 2026,
    granted_amount: 9_200_000_000,
    allocated_amount: 70_000_000,
    project_count: 2,
    disbursed_amount: 13_200_000,
    unallocated_amount: 9_130_000_000,
    overallocated_amount: 0,
    allocated_ratio: 76,
    disbursed_of_allocated_ratio: 1886,
    disbursed_of_granted_ratio: 14,
    ...over,
  };
}

function reply(items: finance_fundingSourceOut[], unattributed = 0): finance_fundingSourcesOut {
  return { year: 2026, items, unattributed_disbursed_amount: unattributed };
}

function cards(data: finance_fundingSourcesOut, canManage = true, onManage = () => {}): HTMLDivElement {
  return mount(<FundingSourceCards data={data} canManage={canManage} onOpenSource={() => {}} onManage={onManage} />);
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

type Call = { url: string; method: string; body: unknown; headers: Record<string, string> };

/** A fake server: `handler` answers by method + path; every call is recorded. */
function stubServer(handler: (c: Call) => Response): Call[] {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const c: Call = {
        url,
        method: init?.method ?? "GET",
        body: typeof init?.body === "string" ? JSON.parse(init.body) : undefined,
        headers: (init?.headers ?? {}) as Record<string, string>,
      };
      calls.push(c);
      return handler(c);
    }),
  );
  return calls;
}

const listGets = (calls: Call[]) => calls.filter((c) => c.method === "GET" && c.url.startsWith("/api/v1/funding-sources?"));

describe("FundingSourceCards — the three bars", () => {
  it("granted > 0: three bars, 'còn … chưa phân bổ', no 'vượt nguồn'", () => {
    const el = cards(reply([source()]));
    const card = el.querySelector("[data-funding-source]")!;
    expect(card.querySelectorAll("[data-meter]")).toHaveLength(3);
    expect(card.textContent).toContain("2 dự án");
    expect(card.textContent).toContain("Đã phân bổ 70 triệu đồng / tổng 9,2 tỷ đồng");
    expect(card.textContent).toContain("còn 9,13 tỷ đồng chưa phân bổ");
    expect(card.textContent).toContain("tổng nguồn");
    // Ratios are the server's hundredths of a percent, drawn as they come.
    expect(card.textContent).toContain("18,86%");
    expect(card.querySelector("[data-overallocated]")).toBeNull();
  });

  it("granted = 0: ONLY bar 2, and the card says the figure is missing", () => {
    const el = cards(
      reply([
        source({
          granted_amount: 0,
          unallocated_amount: 0,
          allocated_ratio: null,
          disbursed_of_granted_ratio: null,
        }),
      ]),
    );
    const card = el.querySelector("[data-funding-source]")!;
    expect(card.querySelectorAll("[data-meter]")).toHaveLength(1);
    expect(card.textContent).toContain("chưa nhập số được giao");
    expect(card.textContent).toContain("đã phân bổ");
    expect(card.textContent).not.toContain("tổng nguồn");
    expect(card.textContent).not.toContain("chưa phân bổ");
  });

  it("overallocation: red 'vượt nguồn' with the server's figure; the ratio above 100% is NOT clamped", () => {
    const el = cards(
      reply([
        source({
          granted_amount: 50_000_000,
          allocated_amount: 70_000_000,
          unallocated_amount: 0,
          overallocated_amount: 20_000_000,
          allocated_ratio: 14000,
        }),
      ]),
    );
    const over = el.querySelector("[data-overallocated]")!;
    expect(over.textContent).toContain("vượt nguồn 20 triệu đồng");
    expect(over.className).toContain("text-danger-600");
    expect(el.textContent).not.toContain("chưa phân bổ");
    expect(el.textContent).toContain("140,00%");
  });

  it("a null ratio is '—', never '0%'", () => {
    const el = cards(reply([source({ allocated_amount: 0, disbursed_of_allocated_ratio: null })]));
    const meters = [...el.querySelectorAll("[data-meter]")].map((m) => m.textContent);
    expect(meters[1]).toContain("—");
    expect(meters[1]).not.toContain("0%");
  });

  it("unattributed > 0: the orange note with the server's amount; 0: no note", () => {
    const el = cards(reply([source()], 3_400_000_000));
    const note = el.querySelector("[data-unattributed]")!;
    expect(note.textContent).toBe(
      "Còn 3,4 tỷ đồng đã chi nhưng chưa ghi rút từ nguồn nào — thuộc các dự án chưa khai phân bổ nguồn vốn.",
    );
    expect(note.className).toContain("text-warning-600");

    act(() => root?.unmount());
    host?.remove();
    expect(cards(reply([source()], 0)).querySelector("[data-unattributed]")).toBeNull();
  });
});

describe("FundingSourceCards — empty catalogue and the permission gate", () => {
  it("empty catalogue WITH budget.update: friendly empty state + '+ Thêm nguồn vốn' that opens management", () => {
    const onManage = vi.fn();
    const el = cards(reply([]), true, onManage);
    expect(el.textContent).toContain("Chưa khai báo nguồn vốn nào");
    act(() => buttonByText(el, "Thêm nguồn vốn")!.click());
    expect(onManage).toHaveBeenCalledOnce();
  });

  it("empty catalogue WITHOUT budget.update: the empty state, and NO add button", () => {
    const el = cards(reply([]), false);
    expect(el.textContent).toContain("Chưa khai báo nguồn vốn nào");
    expect(buttonByText(el, "Thêm nguồn vốn")).toBeUndefined();
  });

  it("the empty state still says the unattributed amount", () => {
    const el = cards(reply([], 1_000_000));
    expect(el.querySelector("[data-unattributed]")).not.toBeNull();
  });
});

describe("FundingSourceProgress — read, gate, dialogs", () => {
  it("reads the SELECTED year and draws 'Quản lý nguồn vốn' only with budget.update", async () => {
    const calls = stubServer(() => json(reply([source()]), 200));
    const el = mount(<FundingSourceProgress year={2026} canManage={false} />);
    await settle();
    expect(calls[0]?.url).toBe("/api/v1/funding-sources?year=2026");
    expect(el.querySelector("[data-funding-source]")).not.toBeNull();
    expect(buttonByText(el, "Quản lý nguồn vốn")).toBeUndefined();

    act(() => root?.unmount());
    host?.remove();
    const el2 = mount(<FundingSourceProgress year={2026} canManage />);
    await settle();
    expect(buttonByText(el2, "Quản lý nguồn vốn")).toBeDefined();
  });

  it("a failed read shows the server's sentence and a retry, never empty cards", async () => {
    stubServer(() => json({ code: "forbidden", message: "Tài khoản chưa có quyền xem giải ngân.", trace_id: "t" }, 403));
    const el = mount(<FundingSourceProgress year={2026} canManage />);
    await settle();
    expect(el.querySelector('[role="alert"]')?.textContent).toBe("Tài khoản chưa có quyền xem giải ngân.");
    expect(el.querySelector("[data-funding-source]")).toBeNull();
  });

  it("clicking the allocated figure opens the source's projects, with a link to each project", async () => {
    const calls = stubServer((c) =>
      c.url.includes("/projects")
        ? json(
            {
              funding_source_id: "01JSRC1",
              name: "Ngân sách xã, phường",
              year: 2026,
              items: [
                {
                  id: "01JPRJ",
                  code: "DA01",
                  name: "Bê tông hoá đường trục chính",
                  planned_amount: 100_000_000,
                  allocated_amount: 40_000_000,
                  disbursed_amount: 10_000_000,
                  disbursed_ratio: 2500,
                },
              ],
              disbursed_without_allocation_amount: 0,
            },
            200,
          )
        : json(reply([source()]), 200),
    );
    const el = mount(<FundingSourceProgress year={2026} canManage={false} />);
    await settle();

    const allocated = el.querySelector<HTMLButtonElement>('[data-funding-source] button[aria-haspopup="dialog"]')!;
    expect(allocated.textContent).toBe("70 triệu đồng");
    act(() => allocated.click());
    await settle();

    expect(calls.some((c) => c.url === "/api/v1/funding-sources/01JSRC1/projects?year=2026")).toBe(true);
    const dialog = document.body.querySelector("dialog")!;
    expect(dialog.textContent).toContain("Ngân sách xã, phường");
    expect(dialog.textContent).toContain("40.000.000 đ");
    expect(dialog.textContent).toContain("25,00%");
    expect(dialog.querySelector("a")?.getAttribute("href")).toBe("/giai-ngan/du-an/01JPRJ");
  });
});

describe("Quản lý nguồn vốn — add and edit", () => {
  async function openManage(handler: (c: Call) => Response): Promise<{ el: HTMLDivElement; calls: Call[]; dialog: HTMLElement }> {
    const calls = stubServer(handler);
    const el = mount(<FundingSourceProgress year={2026} canManage />);
    await settle();
    act(() => buttonByText(el, "Quản lý nguồn vốn")!.click());
    const dialog = document.body.querySelector<HTMLElement>("dialog")!;
    return { el, calls, dialog };
  }

  const okList = () => json(reply([source()]), 200);

  it("add: POST {name, year, granted_amount} with an Idempotency-Key, then the cards are RE-READ", async () => {
    const { calls, dialog } = await openManage((c) =>
      c.method === "POST" ? json(source({ id: "01JNEW", name: "Nguồn xã hội hoá" }), 201) : okList(),
    );
    typeInto(dialog.querySelector<HTMLInputElement>("#ten-nguon-von")!, "  Nguồn xã hội hoá ");
    typeInto(dialog.querySelector<HTMLInputElement>("#von-duoc-giao-moi")!, "1.100.000.000");
    act(() => buttonByText(dialog, "Thêm nguồn vốn")!.click());
    await settle();

    const post = calls.find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/funding-sources");
    expect(post.body).toEqual({ name: "Nguồn xã hội hoá", year: 2026, granted_amount: 1_100_000_000 });
    expect(post.headers["Idempotency-Key"]).toMatch(/.+/);
    expect(listGets(calls)).toHaveLength(2);
    expect(dialog.textContent).toContain("Đã thêm nguồn vốn “Nguồn xã hội hoá”.");
  });

  it("add with a blank amount sends NO granted_amount; a blank name is refused before any request", async () => {
    const { calls, dialog } = await openManage((c) => (c.method === "POST" ? json(source(), 201) : okList()));
    act(() => buttonByText(dialog, "Thêm nguồn vốn")!.click());
    expect(dialog.querySelector('[role="alert"]')?.textContent).toBe(SOURCE_NAME_MISSING);
    expect(calls.filter((c) => c.method === "POST")).toHaveLength(0);

    typeInto(dialog.querySelector<HTMLInputElement>("#ten-nguon-von")!, "Ngân sách thành phố hỗ trợ");
    act(() => buttonByText(dialog, "Thêm nguồn vốn")!.click());
    await settle();
    expect(calls.find((c) => c.method === "POST")?.body).toEqual({ name: "Ngân sách thành phố hỗ trợ", year: 2026 });
  });

  it("409: the server's Vietnamese sentence, the typed name kept, the SAME key on the retry, no re-read", async () => {
    const sentence = "Danh mục nguồn vốn của xã đã đủ số nguồn tối đa nên chưa thêm được.";
    const { calls, dialog } = await openManage((c) =>
      c.method === "POST" ? json({ code: "funding_source_catalogue_full", message: sentence, trace_id: "t" }, 409) : okList(),
    );
    typeInto(dialog.querySelector<HTMLInputElement>("#ten-nguon-von")!, "Nguồn mới");
    act(() => buttonByText(dialog, "Thêm nguồn vốn")!.click());
    await settle();
    expect(dialog.querySelector('[role="alert"]')?.textContent).toBe(sentence);
    expect(dialog.querySelector<HTMLInputElement>("#ten-nguon-von")!.value).toBe("Nguồn mới");
    expect(listGets(calls)).toHaveLength(1);

    act(() => buttonByText(dialog, "Thêm nguồn vốn")!.click());
    await settle();
    const posts = calls.filter((c) => c.method === "POST");
    expect(posts).toHaveLength(2);
    expect(posts[1]!.headers["Idempotency-Key"]).toBe(posts[0]!.headers["Idempotency-Key"]);
  });

  it("edit: PUT this year's granted amount for the source, then the cards are RE-READ", async () => {
    const { calls, dialog } = await openManage((c) =>
      c.method === "PUT" ? json({ funding_source_id: "01JSRC1", year: 2026, granted_amount: 9_500_000_000 }, 200) : okList(),
    );
    expect(dialog.textContent).toContain("Xã đang có 1 nguồn vốn");
    act(() => buttonByText(dialog, "Sửa")!.click());
    const input = dialog.querySelector<HTMLInputElement>("#von-duoc-giao-01JSRC1")!;
    expect(input.value).toBe("9200000000");
    typeInto(input, "9.500.000.000");
    act(() => buttonByText(dialog, "Lưu")!.click());
    await settle();

    const put = calls.find((c) => c.method === "PUT")!;
    expect(put.url).toBe("/api/v1/funding-sources/01JSRC1/annual-amounts/2026");
    expect(put.body).toEqual({ granted_amount: 9_500_000_000 });
    expect(listGets(calls)).toHaveLength(2);
  });

  it("edit: a blank amount is refused (it would be read as 'granted nothing'); an unchanged one sends nothing", async () => {
    const { calls, dialog } = await openManage(okList);
    act(() => buttonByText(dialog, "Sửa")!.click());
    const input = dialog.querySelector<HTMLInputElement>("#von-duoc-giao-01JSRC1")!;
    typeInto(input, "");
    act(() => buttonByText(dialog, "Lưu")!.click());
    expect(dialog.querySelector('[role="alert"]')?.textContent).toBe(GRANTED_AMOUNT_MISSING);

    typeInto(input, "9.200.000.000");
    act(() => buttonByText(dialog, "Lưu")!.click());
    await settle();
    expect(calls.filter((c) => c.method === "PUT")).toHaveLength(0);
    expect(dialog.querySelector("#von-duoc-giao-01JSRC1")).toBeNull();
  });

  it("no delete and no rename control exists in the dialog", async () => {
    const { dialog } = await openManage(okList);
    for (const word of ["Xoá", "Gỡ", "Đổi tên"]) expect(buttonByText(dialog, word)).toBeUndefined();
  });
});
