// @vitest-environment jsdom
//
// jsdom for this file: the funding list is typed into, rows are added and removed, and the PATCH body
// that leaves the browser is the thing under test — events and a captured request, not a markup string.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { toast } from "sonner";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

// Outcomes are toasts (ADR 0068 lần 6 #4).
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import type {
  finance_duAnRa,
  finance_fundingSourceOut,
  finance_fundingStatusOut,
  finance_hangMucRa,
} from "@/lib/api/schema.gen";

import { FundingChip } from "./bang-du-an";
import { ProjectFundingBlock, ThongTinDuAn } from "./chi-tiet-du-an";
import { FormDuAn, ProjectEditPanel, type FundingCatalogue } from "./ghi-du-an";
import { FORM_DU_AN_TRONG } from "./nhan-ghi-giai-ngan";
import { PEOPLE_LOADING } from "./project-people";

/**
 * Project funding allocations on the Giải ngân screens (8245698b): the §9 list in the add / edit form,
 * the §7.2 chip on the list, the §8 per-source block on the detail page.
 *
 * THE MOST EXPENSIVE CASE HERE IS THE PATCH BODY. `funding_allocations` absent means "unchanged", a
 * list means FULL replacement — so an edit that sends the unchanged list back is a write nobody asked
 * for, and one that drops the field when the clerk removed every row silently keeps money attributed.
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

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

/** Sets a controlled field's value the way a user does, so React's handler runs. */
function enter(el: HTMLInputElement | HTMLSelectElement, value: string): void {
  const proto = el instanceof HTMLSelectElement ? HTMLSelectElement.prototype : HTMLInputElement.prototype;
  act(() => {
    Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
    el.dispatchEvent(new Event(el instanceof HTMLSelectElement ? "change" : "input", { bubbles: true }));
  });
}

function buttonByText(el: HTMLElement, text: string): HTMLButtonElement {
  const b = [...el.querySelectorAll("button")].find((x) => x.textContent?.trim() === text);
  if (b === undefined) throw new Error(`no button "${text}"`);
  return b;
}

function source(id: string, name: string, over: Partial<finance_fundingSourceOut> = {}): finance_fundingSourceOut {
  return {
    id,
    name,
    order: 1,
    year: 2026,
    granted_amount: 9_000_000_000,
    allocated_amount: 4_000_000_000,
    project_count: 2,
    disbursed_amount: 0,
    unallocated_amount: 5_000_000_000,
    overallocated_amount: 0,
    allocated_ratio: 4444,
    disbursed_of_allocated_ratio: 0,
    disbursed_of_granted_ratio: 0,
    ...over,
  };
}

const SOURCES = [source("S1", "Ngân sách tỉnh"), source("S2", "Ngân sách xã")];
const READY: FundingCatalogue = { phase: "ready", items: SOURCES };

const CATEGORIES: finance_hangMucRa[] = [
  { id: "01JHM1", code: "chuyen-tiep", label: "Chuyển tiếp", is_default: true, active: true, order: 1, source: "he-thong", tier: 1 },
];

function addForm(catalogue: FundingCatalogue, save = vi.fn()) {
  return mount(
    <FormDuAn
      tieuDeForm="Thêm dự án"
      budgetYear={2026}
      giaTriDau={FORM_DU_AN_TRONG}
      danhMuc={CATEGORIES}
      fundingCatalogue={catalogue}
      dangGui={false}
      loi={null}
      huy={() => {}}
      luu={save}
    />,
  );
}

function sourceSelects(el: HTMLElement): HTMLSelectElement[] {
  return [...el.querySelectorAll<HTMLSelectElement>('select[id^="nguon-von-du-an-"]')];
}
function amountInputs(el: HTMLElement): HTMLInputElement[] {
  return [...el.querySelectorAll<HTMLInputElement>('input[id^="so-tien-nguon-von-"]')];
}
function summaryState(el: HTMLElement): string | null {
  return el.querySelector("[data-allocation-state]")?.getAttribute("data-allocation-state") ?? null;
}
function submitButton(el: HTMLElement): HTMLButtonElement {
  return el.querySelector<HTMLButtonElement>('button[type="submit"]')!;
}

describe("§9 funding list — add form", () => {
  it("no row yet: the spec's empty sentence, and `+ Thêm nguồn vốn` is live", () => {
    const el = addForm(READY);
    expect(el.textContent).toContain(
      "Chưa gắn nguồn nào. Xã theo dõi kế hoạch vốn theo hạng mục thì để trống cũng được.",
    );
    expect(buttonByText(el, "Thêm nguồn vốn").disabled).toBe(false);
    expect(summaryState(el)).toBeNull();
  });

  it("each source is selectable once: a used source leaves the other rows' selects; + disabled when all used", () => {
    const el = addForm(READY);
    act(() => buttonByText(el, "Thêm nguồn vốn").click());
    enter(sourceSelects(el)[0]!, "S1");
    act(() => buttonByText(el, "Thêm nguồn vốn").click());

    expect([...sourceSelects(el)[1]!.options].map((o) => o.value)).toEqual(["", "S2"]);
    // The first row still offers its own source.
    expect([...sourceSelects(el)[0]!.options].map((o) => o.value)).toContain("S1");
    // Two rows, two sources: nothing left to add.
    expect(buttonByText(el, "Thêm nguồn vốn").disabled).toBe(true);
  });

  it("per-row hint is the server's figure for the year: còn X chưa phân bổ trên tổng Y", () => {
    const el = addForm(READY);
    act(() => buttonByText(el, "Thêm nguồn vốn").click());
    enter(sourceSelects(el)[0]!, "S1");
    // Spec 04 verbatim, short amounts.
    expect(el.textContent).toContain("Nguồn này còn 5 tỷ chưa phân bổ trên tổng 9 tỷ được giao.");
  });

  it("live line: thiếu → khớp → vượt as the clerk types; vượt disables saving and says so", () => {
    const save = vi.fn();
    const el = addForm(READY, save);
    enter(el.querySelector<HTMLInputElement>("#ke-hoach-von-du-an")!, "100.000.000");
    act(() => buttonByText(el, "Thêm nguồn vốn").click());
    enter(sourceSelects(el)[0]!, "S1");

    enter(amountInputs(el)[0]!, "60000000");
    expect(summaryState(el)).toBe("short");
    expect(el.textContent).toContain("còn thiếu 40.000.000 đ chưa gắn nguồn");
    expect(submitButton(el).disabled).toBe(false);

    enter(amountInputs(el)[0]!, "100000000");
    expect(summaryState(el)).toBe("match");
    expect(el.textContent).toContain("· khớp");

    enter(amountInputs(el)[0]!, "120000000");
    expect(summaryState(el)).toBe("over");
    expect(el.textContent).toContain("vượt 20.000.000 đ, không lưu được");
    expect(submitButton(el).disabled).toBe(true);
    // Even a submit that gets through (Enter in a field) does not reach `luu`.
    act(() => {
      el.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(save).not.toHaveBeenCalled();
  });

  it("removing a row gives its source back to the others", () => {
    const el = addForm(READY);
    act(() => buttonByText(el, "Thêm nguồn vốn").click());
    enter(sourceSelects(el)[0]!, "S1");
    act(() => el.querySelector<HTMLButtonElement>('button[aria-label="Bỏ nguồn vốn Ngân sách tỉnh"]')!.click());
    expect(sourceSelects(el)).toHaveLength(0);
    expect(el.textContent).toContain("Chưa gắn nguồn nào.");
  });

  it("empty catalogue for the year: the whole `Nguồn vốn` block is absent (spec 04: only when the commune has sources)", () => {
    const el = addForm({ phase: "ready", items: [] });
    expect([...el.querySelectorAll("button")].some((b) => b.textContent?.trim() === "Thêm nguồn vốn")).toBe(false);
    expect(el.querySelector("[data-funding-allocations]")).toBeNull();
  });

  it("catalogue failed to load: the server's sentence, no editable row", () => {
    const el = addForm({ phase: "error", message: "Không kết nối được máy chủ. Vui lòng thử lại." });
    expect(el.textContent).toContain("Chưa tải được danh mục nguồn vốn: Không kết nối được máy chủ.");
    expect(sourceSelects(el)).toHaveLength(0);
  });
});

/* ── Edit panel: what leaves the browser ─────────────────────────────────────────────────────── */

const PROJECT: finance_duAnRa = {
  id: "01JDA1",
  code: "DA01",
  year: 2026,
  category_id: "01JHM1",
  name: "Bê tông hoá đường thôn Hà Lam",
  planned_amount: 100_000_000,
  approved_amount: 100_000_000,
  disbursed_amount: 30_000_000,
  remaining_amount: 70_000_000,
  disbursed_ratio: 3000,
  delay_score: null,
  is_delayed: false,
  disbursement_deadline: "2026-12-31",
  delay_threshold: 1000,
  delay_threshold_source: "mac-dinh",
  funding_status: { status: "chua-du", source_count: 2, allocated_total: 90_000_000, shortfall_amount: 10_000_000 },
  funding_allocations: [
    { funding_source_id: "S1", name: "Ngân sách tỉnh", amount: 60_000_000, disbursed_amount: 30_000_000, disbursed_ratio: 5000 },
    { funding_source_id: "S2", name: "Ngân sách xã", amount: 30_000_000, disbursed_amount: 0, disbursed_ratio: 0 },
  ],
  unallocated_plan_amount: 10_000_000,
};

type Captured = { patch: Record<string, unknown> | null };

/** Fake server: the year's catalogue on GET, and the PATCH answered with `patchReply`. */
function stubServer(patchReply: () => Response): Captured {
  const captured: Captured = { patch: null };
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      if ((init?.method ?? "GET") === "GET" && url.startsWith("/api/v1/funding-sources?year=2026")) {
        return new Response(JSON.stringify({ year: 2026, items: SOURCES, unattributed_disbursed_amount: 0 }), { status: 200 });
      }
      if (init?.method === "PATCH" && url === "/api/v1/investment-projects/01JDA1") {
        captured.patch = JSON.parse(String(init.body)) as Record<string, unknown>;
        return patchReply();
      }
      return new Response("unexpected", { status: 500 });
    }),
  );
  return captured;
}

const OK_REPLY = () => new Response(JSON.stringify({ id: "01JDA1" }), { status: 200 });

async function mountEdit(onSaved = vi.fn()): Promise<HTMLDivElement> {
  const el = mount(
    <ProjectEditPanel duAn={PROJECT} danhMuc={CATEGORIES} people={PEOPLE_LOADING} onClose={() => {}} onSaved={onSaved} />,
  );
  await act(async () => {}); // the catalogue arrives
  return el;
}

async function submit(el: HTMLElement): Promise<void> {
  await act(async () => {
    submitButton(el).click();
  });
  await act(async () => {});
}

describe("§8 edit — funding_allocations in the PATCH", () => {
  it("pre-fills one row per saved allocation, raw amounts", async () => {
    stubServer(OK_REPLY);
    const el = await mountEdit();
    expect(sourceSelects(el).map((s) => s.value)).toEqual(["S1", "S2"]);
    expect(amountInputs(el).map((i) => i.value)).toEqual(["60000000", "30000000"]);
    expect(summaryState(el)).toBe("short");
  });

  it("UNCHANGED set: the PATCH carries no funding_allocations (absent = leave as is)", async () => {
    const captured = stubServer(OK_REPLY);
    const el = await mountEdit();
    enter(el.querySelector<HTMLInputElement>("#ten-du-an")!, "Tên mới");
    await submit(el);
    expect(captured.patch).toEqual({ name: "Tên mới" });
    expect(captured.patch).not.toHaveProperty("funding_allocations");
  });

  it("every row removed: the PATCH carries [] (remove all), not an absent field", async () => {
    const captured = stubServer(OK_REPLY);
    const el = await mountEdit();
    act(() => el.querySelector<HTMLButtonElement>('button[aria-label="Bỏ nguồn vốn Ngân sách tỉnh"]')!.click());
    act(() => el.querySelector<HTMLButtonElement>('button[aria-label="Bỏ nguồn vốn Ngân sách xã"]')!.click());
    await submit(el);
    expect(captured.patch).toEqual({ funding_allocations: [] });
  });

  it("one amount changed: the FULL list is sent (replacement)", async () => {
    const captured = stubServer(OK_REPLY);
    const el = await mountEdit();
    enter(amountInputs(el)[1]!, "40000000");
    await submit(el);
    expect(captured.patch).toEqual({
      funding_allocations: [
        { funding_source_id: "S1", amount: 60_000_000 },
        { funding_source_id: "S2", amount: 40_000_000 },
      ],
    });
  });

  it("409 from the server: its Vietnamese sentence shows, without the backticked field tag", async () => {
    stubServer(
      () =>
        new Response(
          JSON.stringify({
            code: "source_has_disbursements",
            message:
              "`funding_allocations`: nguồn vốn muốn gỡ đã có chứng từ giải ngân của dự án này. Hãy chuyển các chứng từ ấy sang nguồn khác hoặc gỡ chúng trước, rồi mới gỡ nguồn vốn.",
          }),
          { status: 409 },
        ),
    );
    const onSaved = vi.fn();
    const el = await mountEdit(onSaved);
    act(() => el.querySelector<HTMLButtonElement>('button[aria-label="Bỏ nguồn vốn Ngân sách tỉnh"]')!.click());
    await submit(el);
    expect(toast.error).toHaveBeenCalledWith(
      "Nguồn vốn muốn gỡ đã có chứng từ giải ngân của dự án này. Hãy chuyển các chứng từ ấy sang nguồn khác hoặc gỡ chúng trước, rồi mới gỡ nguồn vốn.",
    );
    expect(onSaved).not.toHaveBeenCalled();
  });
});

/* ── §7.2 chip ────────────────────────────────────────────────────────────────────────────────── */

function chip(status: finance_fundingStatusOut | undefined, names?: string[]): string {
  return renderToStaticMarkup(<FundingChip status={status} names={names} />);
}

describe("§7.2 funding chip — the server's state, three variants", () => {
  it("chua-gan-nguon → orange 'Chưa gắn nguồn'", () => {
    const html = chip({ status: "chua-gan-nguon", source_count: 0, allocated_total: 0, shortfall_amount: 100 });
    expect(html).toContain("Chưa gắn nguồn");
    expect(html).toContain("text-tangerine");
  });

  it("chua-du → 'Thiếu {shortfall}' short (spec 02 §8), full đồng on hover, plus the source names", () => {
    const html = chip(
      { status: "chua-du", source_count: 1, allocated_total: 60_000_000, shortfall_amount: 40_000_000 },
      ["Ngân sách tỉnh"],
    );
    expect(html).toContain("Thiếu 40 triệu");
    expect(html).toContain('title="40.000.000 đ"');
    expect(html).toContain("Ngân sách tỉnh");
  });

  it("du → 'Đủ · N nguồn' in green, names joined", () => {
    const html = chip(
      { status: "du", source_count: 3, allocated_total: 100, shortfall_amount: 0 },
      ["Ngân sách tỉnh", "Ngân sách xã", "Xã hội hoá"],
    );
    expect(html).toContain("Đủ · 3 nguồn");
    expect(html).toContain("text-leaf");
    expect(html).toContain("Ngân sách tỉnh, Ngân sách xã, Xã hội hoá");
  });

  it("an unknown state shows the raw string; an absent status is '—'", () => {
    expect(chip({ status: "la", source_count: 0, allocated_total: 0, shortfall_amount: 0 })).toContain(">la<");
    expect(chip(undefined)).toContain("—");
  });
});

/* ── §8 detail ────────────────────────────────────────────────────────────────────────────────── */

describe("§8 'Giải ngân theo nguồn vốn' block", () => {
  it("per source: disbursed / allocated · %, and the unallocated-plan note", () => {
    const html = renderToStaticMarkup(
      <ProjectFundingBlock allocations={PROJECT.funding_allocations} unallocatedPlanAmount={10_000_000} />,
    );
    expect(html).toContain("Giải ngân theo nguồn vốn");
    expect(html).toContain("30.000.000 đ / 60.000.000 đ");
    expect(html).toContain(">50%<");
    // The bar takes the 80/50/30 tier (spec 07 §Body 4): 50% → brand.
    expect(html).toContain("bg-brand");
    expect(html).toContain("Còn 10.000.000 đ của kế hoạch vốn năm chưa gắn nguồn nào.");
  });

  it("null ratio (allocated 0) is '—' over an empty bar, never '0%'; over 100% keeps its words", () => {
    const html = renderToStaticMarkup(
      <ProjectFundingBlock
        allocations={[
          { funding_source_id: "S1", name: "Ngân sách tỉnh", amount: 0, disbursed_amount: 0, disbursed_ratio: null },
          { funding_source_id: "S2", name: "Ngân sách xã", amount: 10, disbursed_amount: 12, disbursed_ratio: 12000 },
        ]}
        unallocatedPlanAmount={0}
      />,
    );
    expect(html).toMatch(/0 đ \/ 0 đ ·[^<]*<span[^>]*>—<\/span>/);
    expect(html).not.toContain(">0%<");
    expect(html).toContain("120%");
    expect(html).toContain("width:100%");
    expect(html).not.toContain("chưa gắn nguồn nào");
  });

  it("no allocation: no block (the header says it)", () => {
    expect(renderToStaticMarkup(<ProjectFundingBlock allocations={[]} unallocatedPlanAmount={100} />)).toBe("");
  });

  it("header line names the sources, or 'Chưa gắn nguồn vốn'", () => {
    const named = renderToStaticMarkup(<ThongTinDuAn duAn={PROJECT} />);
    // Spec 07: sources · officer — no budget year in the line (row D5).
    expect(named).toContain("Ngân sách tỉnh · Ngân sách xã<");
    expect(named).not.toContain("Năm ngân sách");
    const none = renderToStaticMarkup(
      <ThongTinDuAn duAn={{ ...PROJECT, funding_allocations: undefined, funding_source_names: undefined }} />,
    );
    expect(none).toContain("Chưa gắn nguồn vốn<");
  });
});

describe("§8 edit — the funding catalogue is read for the PROJECT's year", () => {
  it("a 2025 project's form asks `funding-sources?year=2025`, never the current year (spec 07 prototype bug not copied)", async () => {
    const urls: string[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) => {
        urls.push(url);
        return new Response(JSON.stringify({ year: 2025, items: SOURCES, unattributed_disbursed_amount: 0 }), { status: 200 });
      }),
    );
    mount(
      <ProjectEditPanel
        duAn={{ ...PROJECT, year: 2025 }}
        danhMuc={CATEGORIES}
        people={PEOPLE_LOADING}
        onClose={() => {}}
        onSaved={() => {}}
      />,
    );
    await act(async () => {});
    const reads = urls.filter((u) => u.startsWith("/api/v1/funding-sources"));
    expect(reads).toEqual(["/api/v1/funding-sources?year=2025"]);
  });
});
